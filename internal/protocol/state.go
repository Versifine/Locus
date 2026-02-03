package protocol

import "sync"

type State int

const (
	Handshaking State = iota
	Status
	Login
	Play
)

type ConnState struct {
	mu    sync.Mutex
	state State
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
