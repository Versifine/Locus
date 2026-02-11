package agent

import (
	"bufio"
	"log/slog"
	"strings"
	"sync"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/protocol"
	"github.com/Versifine/locus/internal/world"
)

const defaultMaxHistory = 20

type Agent struct {
	bus           *event.Bus
	sender        MessageSender
	stateProvider StateProvider
	client        *llm.Client
	maxHistory    int

	mu      sync.Mutex
	history map[protocol.UUID][]llm.Message
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
		history:       make(map[protocol.UUID][]llm.Message),
	}
	bus.Subscribe(event.EventChat, a.ChatEventHandler)
	return a
}

func (a *Agent) appendHistory(uuid protocol.UUID, msg llm.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history[uuid] = append(a.history[uuid], msg)
	if len(a.history[uuid]) > a.maxHistory {
		a.history[uuid] = a.history[uuid][len(a.history[uuid])-a.maxHistory:]
	}
}

func (a *Agent) getHistory(uuid protocol.UUID) []llm.Message {
	a.mu.Lock()
	defer a.mu.Unlock()
	hist := make([]llm.Message, len(a.history[uuid]))
	copy(hist, a.history[uuid])
	return hist
}

func buildMessages(systemPrompt string, history []llm.Message, stateStr string, userMessage string) []llm.Message {
	messages := make([]llm.Message, 0, len(history)+3)
	messages = append(messages, llm.Message{Role: "system", Content: systemPrompt})

	// Keep prior turns, but not the current user message (already in history tail).
	if len(history) > 1 {
		messages = append(messages, history[:len(history)-1]...)
	}

	messages = append(messages, llm.Message{
		Role: "system",
		Content: "[当前状态]\n" + stateStr + "\n\n[回答约束]\n" +
			"- 只回答玩家这一次的问题，不要主动扩展未被询问的内容。\n" +
			"- 如果玩家是问候或闲聊，先正常简短回应，不要主动播报实体列表。\n" +
			"- 涉及数量或距离时，优先给结论，再补充必要细节。",
	})
	messages = append(messages, llm.Message{Role: "user", Content: userMessage})
	return messages
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

	// 1) append current user message into per-player history
	a.appendHistory(evt.UUID, llm.Message{Role: "user", Content: evt.Message})

	// 2) build LLM messages with a dedicated system state message
	hist := a.getHistory(evt.UUID)
	stateStr := a.stateProvider.GetState().String()
	messages := buildMessages(a.client.Config().SystemPrompt, hist, stateStr, evt.Message)

	response, err := a.client.Chat(evt.Ctx, messages)
	if err != nil {
		slog.Error("LLM chat error", "error", err)
		return
	}

	// 3) append assistant reply to history
	a.appendHistory(evt.UUID, llm.Message{Role: "assistant", Content: response})

	// 4) split multiline response and send line by line
	scanner := bufio.NewScanner(strings.NewReader(response))
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
