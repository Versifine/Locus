package agent

import (
	"log/slog"

	"github.com/Versifine/locus/internal/event"
)

type Agent struct {
	bus *event.Bus
}

func NewAgent(bus *event.Bus) *Agent {
	bus.Subscribe(event.EventChat, ChatEventHandler)
	return &Agent{bus: bus}
}

func ChatEventHandler(raw any) {
	chatEvent, ok := raw.(*event.ChatEvent)
	if !ok {
		slog.Error("Invalid event type for ChatEventHandler")
		return
	}
	// 在这里处理聊天事件，例如记录日志或修改消息内容
	slog.Info("Chat event", "username", chatEvent.Username, "uuid", chatEvent.UUID.String(), "message", chatEvent.Message, "source", chatEvent.Source.String())
}
