package event

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewBus 测试创建新的事件总线
func TestNewBus(t *testing.T) {
	bus := NewBus()
	if bus == nil {
		t.Fatal("NewBus() 返回 nil")
	}
	if bus.handlers == nil {
		t.Fatal("NewBus() handlers map 未初始化")
	}
}

// TestSubscribeAndPublish 测试订阅和发布事件
func TestSubscribeAndPublish(t *testing.T) {
	bus := NewBus()
	done := make(chan any, 1)
	bus.Subscribe("test", func(e any) {
		done <- e
	})

	bus.Publish("test", "hello")

	select {
	case received := <-done:
		if received != "hello" {
			t.Errorf("handler 收到 %v, 期望 %v", received, "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("handler 未被调用（超时）")
	}
}

// TestPublishNoSubscribers 测试发布无订阅者的事件不会 panic
func TestPublishNoSubscribers(t *testing.T) {
	bus := NewBus()
	// 不应 panic
	bus.Publish("nonexistent", "data")
}

// TestMultipleSubscribers 测试多个订阅者
func TestMultipleSubscribers(t *testing.T) {
	bus := NewBus()
	var count atomic.Int32
	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		bus.Subscribe("test", func(e any) {
			count.Add(1)
			wg.Done()
		})
	}

	bus.Publish("test", "data")

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if count.Load() != 3 {
			t.Errorf("handler 被调用 %d 次, 期望 3 次", count.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("handler 未全部完成（超时）")
	}
}

// TestMultipleEvents 测试不同事件名称互不干扰
func TestMultipleEvents(t *testing.T) {
	bus := NewBus()
	chatDone := make(chan struct{}, 1)
	var loginReceived atomic.Bool

	bus.Subscribe("chat", func(e any) {
		chatDone <- struct{}{}
	})
	bus.Subscribe("login", func(e any) {
		loginReceived.Store(true)
	})

	bus.Publish("chat", "msg")

	select {
	case <-chatDone:
	case <-time.After(time.Second):
		t.Fatal("chat handler 未被调用（超时）")
	}

	time.Sleep(50 * time.Millisecond)
	if loginReceived.Load() {
		t.Error("login handler 不应该被调用")
	}
}

// TestConcurrentSubscribeAndPublish 测试并发订阅和发布的线程安全性
func TestConcurrentSubscribeAndPublish(t *testing.T) {
	bus := NewBus()
	var count atomic.Int64

	// 先订阅一个handler
	bus.Subscribe("test", func(event any) {
		count.Add(1)
	})

	var wg sync.WaitGroup

	// 并发发布
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish("test", "data")
		}()
	}

	// 并发订阅
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Subscribe("test", func(event any) {
				count.Add(1)
			})
		}()
	}

	wg.Wait()
	// 等待异步 handler 执行完毕
	time.Sleep(200 * time.Millisecond)

	if count.Load() < 100 {
		t.Errorf("至少应该收到 100 次事件, 实际收到 %d 次", count.Load())
	}
}

// TestPublishEventData 测试事件数据正确传递
func TestPublishEventData(t *testing.T) {
	bus := NewBus()
	type testEvent struct {
		Name  string
		Value int
	}

	done := make(chan *testEvent, 1)
	bus.Subscribe("test", func(e any) {
		done <- e.(*testEvent)
	})

	sent := &testEvent{Name: "hello", Value: 42}
	bus.Publish("test", sent)

	select {
	case received := <-done:
		if received.Name != "hello" || received.Value != 42 {
			t.Errorf("收到 %+v, 期望 %+v", received, sent)
		}
	case <-time.After(time.Second):
		t.Fatal("handler 未收到事件（超时）")
	}
}
