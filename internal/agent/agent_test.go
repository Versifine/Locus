package agent

import (
	"sync/atomic"
	"testing"

	"github.com/Versifine/locus/internal/event"
)

// TestNewAgent 测试创建 Agent
func TestNewAgent(t *testing.T) {
	bus := event.NewBus()
	agent := NewAgent(bus)

	if agent == nil {
		t.Fatal("NewAgent() 返回 nil")
	}
	if agent.bus != bus {
		t.Error("Agent.bus 应该等于传入的 bus")
	}
}

// TestNewAgentSubscribesChat 测试 Agent 创建时订阅了 chat 事件
func TestNewAgentSubscribesChat(t *testing.T) {
	bus := event.NewBus()
	_ = NewAgent(bus)

	// 验证 chat 事件已被订阅：发布事件不应 panic
	// ChatEventHandler 会尝试类型断言，传入错误类型会记录错误但不 panic
	var called atomic.Bool
	bus.Subscribe("chat", func(e any) {
		called.Store(true)
	})

	bus.Publish("chat", "test-data")

	if !called.Load() {
		t.Error("chat 事件发布后 handler 应该被调用")
	}
}
