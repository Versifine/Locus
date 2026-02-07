package agent

import (
	"log/slog"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/proxy"
)

type Agent struct {
	bus    *event.Bus
	server *proxy.Server
	client *llm.Client
}

func NewAgent(bus *event.Bus, server *proxy.Server) *Agent {
	a := &Agent{bus: bus, server: server}
	bus.Subscribe(event.EventChat, a.ChatEventHandler)
	return a
}

func (a *Agent) ChatEventHandler(raw any) {
	chatEvent, ok := raw.(*event.ChatEvent)
	if !ok {
		slog.Error("Invalid event type for ChatEventHandler")
		return
	}
	// 在这里处理聊天事件，例如记录日志或修改消息内容
	slog.Info("Chat event", "username", chatEvent.Username, "uuid", chatEvent.UUID.String(), "message", chatEvent.Message, "source", chatEvent.Source.String())
	switch chatEvent.Source {
	case event.SourcePlayer:
		// 处理玩家消息
		// 例如，可以将消息发送到 LLM 客户端进行处理
		if a.server != nil {
			a.server.SendMsgToServer("[Agent] 收到消息" + chatEvent.Message)
		}

	case event.SourceSystem:
		// 处理代理消息
	default:
		slog.Warn("Unknown chat event source", "source", chatEvent.Source.String())
	}
}
