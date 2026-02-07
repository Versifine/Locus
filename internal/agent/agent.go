package agent

import (
	"bufio"
	"log/slog"
	"strings"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/proxy"
)

type Agent struct {
	bus    *event.Bus
	server *proxy.Server
	client *llm.Client
}

func NewAgent(bus *event.Bus, server *proxy.Server, client *llm.Client) *Agent {
	a := &Agent{bus: bus, server: server, client: client}
	bus.Subscribe(event.EventChat, a.ChatEventHandler)
	return a
}
func (a *Agent) handleSourcePlayer(event *event.ChatEvent) {
	messages := []llm.Message{
		{Role: "system", Content: a.client.Config().SystemPrompt},
		{
			Role:    "user",
			Content: event.Message},
	}
	response, err := a.client.Chat(event.Ctx, messages)
	if err != nil {
		slog.Error("LLM chat error", "error", err)
		return
	}
	scanner := bufio.NewScanner(strings.NewReader(response))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines := SplitByRunes(line, 250)
		for _, l := range lines {
			a.server.SendMsgToServer(l)
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
	if a.server == nil {
		slog.Error("Proxy server is not initialized in Agent")
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
