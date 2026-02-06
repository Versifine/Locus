package event

import (
	"testing"

	"github.com/Versifine/locus/internal/protocol"
)

// TestSourceTypeString 测试 SourceType 的字符串表示
func TestSourceTypeString(t *testing.T) {
	tests := []struct {
		name     string
		source   SourceType
		expected string
	}{
		{"System", SourceSystem, "System"},
		{"Player", SourcePlayer, "Player"},
		{"PlayerSend", SourcePlayerSend, "PlayerSend"},
		{"PlayerCmd", SourcePlayerCmd, "PlayerCmd"},
		{"Unknown", SourceType(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.source.String()
			if got != tt.expected {
				t.Errorf("SourceType(%d).String() = %q, 期望 %q", tt.source, got, tt.expected)
			}
		})
	}
}

// TestNewChatEvent 测试创建聊天事件
func TestNewChatEvent(t *testing.T) {
	uuid := protocol.UUID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10}
	event := NewChatEvent("Steve", uuid, "Hello World", SourcePlayer)

	if event == nil {
		t.Fatal("NewChatEvent() 返回 nil")
	}
	if event.Username != "Steve" {
		t.Errorf("Username = %q, 期望 %q", event.Username, "Steve")
	}
	if event.UUID != uuid {
		t.Errorf("UUID = %v, 期望 %v", event.UUID, uuid)
	}
	if event.Message != "Hello World" {
		t.Errorf("Message = %q, 期望 %q", event.Message, "Hello World")
	}
	if event.Source != SourcePlayer {
		t.Errorf("Source = %v, 期望 %v", event.Source, SourcePlayer)
	}
}

// TestNewChatEventAllSources 测试所有事件来源类型
func TestNewChatEventAllSources(t *testing.T) {
	sources := []SourceType{SourceSystem, SourcePlayer, SourcePlayerSend, SourcePlayerCmd}

	for _, source := range sources {
		t.Run(source.String(), func(t *testing.T) {
			event := NewChatEvent("Player", protocol.UUID{}, "msg", source)
			if event.Source != source {
				t.Errorf("Source = %v, 期望 %v", event.Source, source)
			}
		})
	}
}

// TestSourceTypeIotaValues 测试 SourceType 枚举值的顺序
func TestSourceTypeIotaValues(t *testing.T) {
	if SourceSystem != 0 {
		t.Errorf("SourceSystem = %d, 期望 0", SourceSystem)
	}
	if SourcePlayer != 1 {
		t.Errorf("SourcePlayer = %d, 期望 1", SourcePlayer)
	}
	if SourcePlayerSend != 2 {
		t.Errorf("SourcePlayerSend = %d, 期望 2", SourcePlayerSend)
	}
	if SourcePlayerCmd != 3 {
		t.Errorf("SourcePlayerCmd = %d, 期望 3", SourcePlayerCmd)
	}
}
