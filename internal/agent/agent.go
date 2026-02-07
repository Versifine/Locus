package agent

import (
	"context"
	"log/slog"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/proxy"
)

type Agent struct {
	bus    *event.Bus
	server *proxy.Server
	client *llm.Client
	ctx    context.Context
}

func NewAgent(ctx context.Context, bus *event.Bus, server *proxy.Server, client *llm.Client) *Agent {
	a := &Agent{ctx: ctx, bus: bus, server: server, client: client}
	bus.Subscribe(event.EventChat, a.ChatEventHandler)
	return a
}
func (a *Agent) handleSourcePlayer(event *event.ChatEvent) {
	messages := []llm.Message{
		{
			Role:    "system",
			Content: event.Message},
	}
	response, err := a.client.Chat(a.ctx, messages)
	if err != nil {
		slog.Error("LLM chat error", "error", err)
		return
	}
	slog.Info("LLM response", "response", response)
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
	default:
		slog.Warn("Unknown chat event source", "source", chatEvent.Source.String())
	}
}
