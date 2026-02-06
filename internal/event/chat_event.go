package event

import (
	"log/slog"

	"github.com/Versifine/locus/internal/protocol"
)

type SourceType int

const (
	SourceSystem SourceType = iota
	SourcePlayer
	SourcePlayerSend
	SourcePlayerCmd
)

func (st SourceType) String() string {
	switch st {
	case SourceSystem:
		return "System"
	case SourcePlayer:
		return "Player"
	case SourcePlayerSend:
		return "PlayerSend"
	case SourcePlayerCmd:
		return "PlayerCmd"
	default:
		return "Unknown"
	}
}

type ChatEvent struct {
	Username string
	UUID     protocol.UUID
	Message  string
	Source   SourceType
}

func ChatEventHandler(event any) {
	chatEvent, ok := event.(*ChatEvent)
	if !ok {
		slog.Error("Invalid event type for ChatEventHandler")
		return
	}
	// 在这里处理聊天事件，例如记录日志或修改消息内容
	slog.Info("Chat event", "username", chatEvent.Username, "uuid", chatEvent.UUID.String(), "message", chatEvent.Message, "source", chatEvent.Source.String())
}

func NewChatEvent(username string, uuid protocol.UUID, message string, source SourceType) *ChatEvent {
	return &ChatEvent{
		Username: username,
		UUID:     uuid,
		Message:  message,
		Source:   source,
	}
}
