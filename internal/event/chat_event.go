package event

import (
	"context"

	"github.com/Versifine/locus/internal/protocol"
)

// EventChat 聊天事件名称常量
const EventChat = "chat"

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
	Ctx      context.Context
}

func NewChatEvent(ctx context.Context, username string, uuid protocol.UUID, message string, source SourceType) *ChatEvent {
	return &ChatEvent{
		Ctx:      ctx,
		Username: username,
		UUID:     uuid,
		Message:  message,
		Source:   source,
	}
}
