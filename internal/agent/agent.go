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

func (a *Agent) handleSourcePlayer(evt *event.ChatEvent) {
	// 1. 追加 user 消息到历史
	a.appendHistory(evt.UUID, llm.Message{Role: "user", Content: evt.Message})

	// 2. 组装 system + 历史 发给 LLM
	hist := a.getHistory(evt.UUID)
	messages := make([]llm.Message, 0, len(hist)+1)
	messages = append(messages, llm.Message{Role: "system", Content: a.client.Config().SystemPrompt})
	messages = append(messages, hist...)

	response, err := a.client.Chat(evt.Ctx, messages)
	if err != nil {
		slog.Error("LLM chat error", "error", err)
		return
	}

	// 3. 追加 assistant 回复到历史（存完整回复）
	a.appendHistory(evt.UUID, llm.Message{Role: "assistant", Content: response})

	// 4. 拆段发送到游戏
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
	runes := []rune(s) // 转为 rune 切片以正确处理中文/Emoji

	for len(runes) > 0 {
		if len(runes) > limit {
			// 还没切完，切一刀，剩下的继续循环
			chunks = append(chunks, string(runes[:limit]))
			runes = runes[limit:]
		} else {
			// 剩下的不足 limit，直接全部放入
			chunks = append(chunks, string(runes))
			runes = nil // 结束循环
		}
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
	// 在这里处理聊天事件，例如记录日志或修改消息内容
	slog.Info("Chat event", "username", chatEvent.Username, "uuid", chatEvent.UUID.String(), "message", chatEvent.Message, "source", chatEvent.Source.String())
	switch chatEvent.Source {
	case event.SourcePlayer:
		// 处理玩家消息
		// 例如，可以将消息发送到 LLM 客户端进行处理
		go a.handleSourcePlayer(chatEvent)

	case event.SourceSystem:
	// 处理系统消息
	case event.SourcePlayerCmd:
	// 处理玩家命令消息
	case event.SourcePlayerSend:
	// 处理玩家发送消息
	default:
		slog.Warn("Unknown chat event source", "source", chatEvent.Source.String())
	}
}
