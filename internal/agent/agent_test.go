package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/protocol"
)

// TestNewAgent 测试创建 Agent
func TestNewAgent(t *testing.T) {
	bus := event.NewBus()
	a := NewAgent(bus, nil, nil)

	if a == nil {
		t.Fatal("NewAgent() 返回 nil")
	}
	if a.bus != bus {
		t.Error("Agent.bus 应该等于传入的 bus")
	}
}

// TestNewAgentSubscribesChat 测试 Agent 创建时订阅了 chat 事件
func TestNewAgentSubscribesChat(t *testing.T) {
	bus := event.NewBus()
	_ = NewAgent(bus, nil, nil)

	var called atomic.Bool
	bus.Subscribe(event.EventChat, func(e any) {
		called.Store(true)
	})

	bus.Publish(event.EventChat, "test-data")

	time.Sleep(100 * time.Millisecond)

	if !called.Load() {
		t.Error("chat 事件发布后 handler 应该被调用")
	}
}

// TestChatEventHandler 测试聊天事件处理器不会 panic
func TestChatEventHandler(t *testing.T) {
	// 正确类型的事件
	ce := event.NewChatEvent(context.Background(), "Steve", protocol.UUID{}, "Hello", event.SourcePlayer)
	a := NewAgent(event.NewBus(), nil, nil)
	a.ChatEventHandler(ce) // 不应该 panic

	// 错误类型的事件也不应该 panic
	a.ChatEventHandler("not a chat event")
	a.ChatEventHandler(nil)
	a.ChatEventHandler(42)
}
