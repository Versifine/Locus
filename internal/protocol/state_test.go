package protocol

import (
	"sync"
	"testing"
)

func TestConnStateDefault(t *testing.T) {
	cs := NewConnState()
	if cs.Get() != Handshaking {
		t.Errorf("默认状态 = %d, 期望 Handshaking(%d)", cs.Get(), Handshaking)
	}
	if cs.GetThreshold() != -1 {
		t.Errorf("默认阈值 = %d, 期望 -1", cs.GetThreshold())
	}
}

func TestConnStateSetGet(t *testing.T) {
	cs := NewConnState()

	states := []State{Handshaking, Status, Login, Play}
	for _, s := range states {
		cs.Set(s)
		if cs.Get() != s {
			t.Errorf("Set(%d) 后 Get() = %d", s, cs.Get())
		}
	}
}

func TestConnStateThreshold(t *testing.T) {
	cs := NewConnState()

	tests := []int{-1, 0, 256, 1024}
	for _, v := range tests {
		cs.SetThreshold(v)
		if cs.GetThreshold() != v {
			t.Errorf("SetThreshold(%d) 后 GetThreshold() = %d", v, cs.GetThreshold())
		}
	}
}

func TestConnStateConcurrency(t *testing.T) {
	cs := NewConnState()
	var wg sync.WaitGroup

	// 并发设置状态
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(s State) {
			defer wg.Done()
			cs.Set(s)
			_ = cs.Get()
		}(State(i % 4))
	}

	// 并发设置阈值
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			cs.SetThreshold(v)
			_ = cs.GetThreshold()
		}(i)
	}

	wg.Wait()

	// 只要不 panic/race 就算通过
	got := cs.Get()
	if got < Handshaking || got > Play {
		t.Errorf("最终状态 %d 不在合法范围内", got)
	}
}
