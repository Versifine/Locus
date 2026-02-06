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
	mu        sync.RWMutex
	state     State
	threshold int
	username  string
	uuid      UUID
}

func (cs *ConnState) Username() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.username
}
func (cs *ConnState) SetUsername(username string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.username = username
}
func (cs *ConnState) UUID() UUID {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.uuid
}
func (cs *ConnState) SetUUID(uuid UUID) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.uuid = uuid
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
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.state
}

func (cs *ConnState) SetThreshold(t int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.threshold = t
}

func (cs *ConnState) GetThreshold() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.threshold
}
