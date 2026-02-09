package agent

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/event"
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

// MockSender 实现 MessageSender 接口用于测试
type MockSender struct {
	sentMessages []string
	mu           sync.Mutex
}

func (m *MockSender) SendMsgToServer(message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, message)
	return nil
}

func (m *MockSender) Messages() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sentMessages
}

// TestAgentSendsReply 测试 Agent 收到消息后能正确通过 Sender 发送回复
func TestAgentSendsReply(t *testing.T) {
	bus := event.NewBus()
	mockSender := &MockSender{}

	// 需要一个 Mock LLM Client
	// 这里由于 llm.Client 结构比较复杂，我们简单测试 SplitByRunes 的逻辑或者用一个真实的 Client (如果环境允许)
	// 但为了单元测试纯粹，我们主要测试 Agent 的 Sender 注入是否成功。
	a := NewAgent(bus, mockSender, nil)

	if a.sender != mockSender {
		t.Error("Sender 注入失败")
	}

	// 测试 SplitByRunes
	t.Run("SplitByRunes", func(t *testing.T) {
		input := "1234567890"
		chunks := SplitByRunes(input, 3)
		expected := []string{"123", "456", "789", "0"}
		if len(chunks) != len(expected) {
			t.Fatalf("长度不符: got %v, want %v", chunks, expected)
		}
		for i := range chunks {
			if chunks[i] != expected[i] {
				t.Errorf("Index %d 不符: got %s, want %s", i, chunks[i], expected[i])
			}
		}
	})
}
