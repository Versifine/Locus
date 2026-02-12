package agent

import (
	"bufio"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

const (
	defaultMaxHistory = 20
	memoryTagPattern  = `(?is)<memory>(.*?)</memory>`
)

var memoryTagRegexp = regexp.MustCompile(memoryTagPattern)

type Agent struct {
	bus           *event.Bus
	sender        MessageSender
	stateProvider StateProvider
	client        *llm.Client
	maxHistory    int

	mu     sync.Mutex
	memory map[protocol.UUID][]string
}

type MessageSender interface {
	SendMsgToServer(message string) error
}

type StateProvider interface {
	GetState() world.Snapshot
}

func NewAgent(bus *event.Bus, sender MessageSender, stateProvider StateProvider, client *llm.Client) *Agent {
	maxH := defaultMaxHistory
	if client != nil && client.Config().MaxHistory > 0 {
		maxH = client.Config().MaxHistory
	}

	a := &Agent{
		bus:           bus,
		sender:        sender,
		stateProvider: stateProvider,
		client:        client,
		maxHistory:    maxH,
		memory:        make(map[protocol.UUID][]string),
	}
	bus.Subscribe(event.EventChat, a.ChatEventHandler)
	return a
}

func normalizeMemory(summary string) string {
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return ""
	}
	return strings.Join(strings.Fields(summary), " ")
}

func (a *Agent) appendMemory(uuid protocol.UUID, summary string) {
	summary = normalizeMemory(summary)
	if summary == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.memory[uuid] = append(a.memory[uuid], summary)
	if len(a.memory[uuid]) > a.maxHistory {
		a.memory[uuid] = a.memory[uuid][len(a.memory[uuid])-a.maxHistory:]
	}
}

func (a *Agent) getMemory(uuid protocol.UUID) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	memory := make([]string, len(a.memory[uuid]))
	copy(memory, a.memory[uuid])
	return memory
}

func buildRecentMemoryContent(memory []string) string {
	var builder strings.Builder
	for _, item := range memory {
		item = normalizeMemory(item)
		if item == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(item)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func buildMessages(systemPrompt string, memory []string, stateStr string, userMessage string) []llm.Message {
	messages := make([]llm.Message, 0, len(memory)+6)
	messages = append(messages, llm.Message{Role: "system", Content: systemPrompt})

	if len(memory) > 0 {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: "[以下是历史对话记录，其中涉及的位置、实体、状态等信息均已过时，仅供了解对话上下文]",
		})
		if recent := buildRecentMemoryContent(memory); recent != "" {
			messages = append(messages, llm.Message{
				Role:    "system",
				Content: "[近期记忆]\n" + recent,
			})
		}
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: "[历史记录结束。以下是当前真实状态，以此为准]",
		})
	}

	messages = append(messages, llm.Message{
		Role: "system",
		Content: "[当前状态（唯一可信数据源）]\n" + stateStr + "\n\n[回答约束]\n" +
			"- 只回答玩家这一次的问题，不要主动扩展未被询问的内容。\n" +
			"- 如果玩家是问候或闲聊，先正常简短回应，不要主动播报实体列表。\n" +
			"- 涉及数量或距离时，优先给结论，再补充必要细节。",
	})
	messages = append(messages, llm.Message{Role: "user", Content: userMessage})
	return messages
}

func extractMemory(response string) (chatReply, memory string) {
	response = strings.TrimSpace(response)
	if response == "" {
		return "", ""
	}

	matches := memoryTagRegexp.FindAllStringSubmatch(response, -1)
	if len(matches) > 0 {
		memory = normalizeMemory(matches[len(matches)-1][1])
	}

	chatReply = memoryTagRegexp.ReplaceAllString(response, "")

	lower := strings.ToLower(chatReply)
	if idx := strings.LastIndex(lower, "<memory>"); idx >= 0 {
		chatReply = chatReply[:idx]
	}
	chatReply = strings.ReplaceAll(chatReply, "</memory>", "")
	chatReply = strings.TrimSpace(chatReply)

	return chatReply, memory
}

func (a *Agent) handleSourcePlayer(evt *event.ChatEvent) {
	if a.client == nil {
		slog.Error("LLM client is nil")
		return
	}
	if a.stateProvider == nil {
		slog.Error("StateProvider is nil")
		return
	}

	// 1) build LLM messages using per-player recent memory summary
	memory := a.getMemory(evt.UUID)
	stateStr := a.stateProvider.GetState().String()
	messages := buildMessages(a.client.Config().SystemPrompt, memory, stateStr, evt.Message)

	response, err := a.client.Chat(evt.Ctx, messages)
	if err != nil {
		slog.Error("LLM chat error", "error", err)
		return
	}

	// 2) strip memory tags before sending to game, keep extracted summary for future turns
	chatReply, memorySummary := extractMemory(response)
	a.appendMemory(evt.UUID, memorySummary)

	// 3) split multiline response and send line by line
	scanner := bufio.NewScanner(strings.NewReader(chatReply))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		for _, l := range SplitByRunes(line, 250) {
			if err := a.sender.SendMsgToServer(l); err != nil {
				slog.Error("Failed to send message to server", "error", err)
			}
		}
	}
}

func SplitByRunes(s string, limit int) []string {
	if limit <= 0 {
		return nil
	}

	var chunks []string
	runes := []rune(s)
	for len(runes) > 0 {
		if len(runes) > limit {
			chunks = append(chunks, string(runes[:limit]))
			runes = runes[limit:]
			continue
		}
		chunks = append(chunks, string(runes))
		runes = nil
	}
	return chunks
}

func (a *Agent) ChatEventHandler(raw any) {
	chatEvent, ok := raw.(*event.ChatEvent)
	if !ok {
		slog.Error("Invalid event type for ChatEventHandler")
		return
	}
	if a.sender == nil {
		slog.Error("MessageSender is nil")
		return
	}

	slog.Info("Chat event", "username", chatEvent.Username, "uuid", chatEvent.UUID.String(), "message", chatEvent.Message, "source", chatEvent.Source.String())
	switch chatEvent.Source {
	case event.SourcePlayer:
		go a.handleSourcePlayer(chatEvent)
	case event.SourceSystem:
		// reserved for system messages
	case event.SourcePlayerCmd:
		// reserved for player command messages
	case event.SourcePlayerSend:
		// reserved for player send messages
	default:
		slog.Warn("Unknown chat event source", "source", chatEvent.Source.String())
	}
}
