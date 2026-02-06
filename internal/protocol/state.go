package protocol

import "sync"

type State int

const (
	Handshaking State = iota
	Status
	Login
	Configuration // 1.20.2+ 新增的配置阶段
	Play
)

type ConnState struct {
	mu        sync.Mutex
	state     State
	threshold int
}

func NewConnState() *ConnState {
	return &ConnState{
		threshold: -1,
	}
}

func (cs *ConnState) Set(state State) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.state = state
}

func (cs *ConnState) Get() State {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.state
}

func (cs *ConnState) SetThreshold(t int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.threshold = t
}

func (cs *ConnState) GetThreshold() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.threshold
}
