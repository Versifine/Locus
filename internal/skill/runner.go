package skill

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/Versifine/locus/internal/body"
	"github.com/Versifine/locus/internal/world"
)

type BehaviorFunc func(bctx BehaviorCtx) error

type BehaviorExitReason string

const (
	BehaviorExitCompleted BehaviorExitReason = "completed"
	BehaviorExitCancelled BehaviorExitReason = "cancelled"
	BehaviorExitFailed    BehaviorExitReason = "failed"
	BehaviorExitPreempted BehaviorExitReason = "preempted"
)

type BehaviorEnd struct {
	Name   string
	RunID  uint64
	Reason BehaviorExitReason
	Err    error
}

type behaviorHandle struct {
	name     string
	runID    uint64
	priority int
	channels map[Channel]struct{}

	tickCh   chan world.Snapshot
	outputCh chan PartialInput
	cancel   context.CancelFunc

	stopReasonMu sync.Mutex
	stopReason   BehaviorExitReason
}

type BehaviorRunner struct {
	mu            sync.Mutex
	active        map[string]*behaviorHandle
	channelOwners map[Channel]string
	runSeq        atomic.Uint64

	send     func(string) error
	snapshot func() world.Snapshot
	blocks   BlockAccess
	endCh    chan BehaviorEnd
}

func NewBehaviorRunner(send func(string) error, snapshot func() world.Snapshot, blocks BlockAccess) *BehaviorRunner {
	return &BehaviorRunner{
		active:        make(map[string]*behaviorHandle),
		channelOwners: make(map[Channel]string),
		send:          send,
		snapshot:      snapshot,
		blocks:        blocks,
		endCh:         make(chan BehaviorEnd, 64),
	}
}

func (r *BehaviorRunner) Start(name string, fn BehaviorFunc, channels []Channel, priority int) bool {
	ok, _ := r.StartWithRunID(name, fn, channels, priority)
	return ok
}

func (r *BehaviorRunner) StartWithRunID(name string, fn BehaviorFunc, channels []Channel, priority int) (bool, uint64) {
	if r == nil || fn == nil || name == "" {
		return false, 0
	}

	r.mu.Lock()
	if _, exists := r.active[name]; exists {
		r.preemptLocked(name)
	}

	conflicts := r.findConflictsLocked(channels)
	for _, conflictName := range conflicts {
		owner := r.active[conflictName]
		if owner == nil {
			continue
		}
		if priority <= owner.priority {
			r.mu.Unlock()
			return false, 0
		}
	}

	for _, conflictName := range conflicts {
		r.preemptLocked(conflictName)
	}

	ctx, cancel := context.WithCancel(context.Background())
	h := &behaviorHandle{
		name:     name,
		runID:    r.runSeq.Add(1),
		priority: priority,
		channels: make(map[Channel]struct{}, len(channels)),
		tickCh:   make(chan world.Snapshot, 1),
		outputCh: make(chan PartialInput, 8),
		cancel:   cancel,
	}
	for _, ch := range channels {
		h.channels[ch] = struct{}{}
		r.channelOwners[ch] = name
	}
	r.active[name] = h
	r.mu.Unlock()

	bctx := BehaviorCtx{
		Ctx:        ctx,
		CancelFunc: cancel,
		Tick:       h.tickCh,
		Output:     h.outputCh,
		SendFunc:   r.send,
		SnapshotFn: r.snapshot,
		Blocks:     r.blocks,
	}

	go func(handle *behaviorHandle) {
		err := fn(bctx)
		r.cleanup(handle, ctx, err)
	}(h)

	return true, h.runID
}

func (r *BehaviorRunner) Tick(snap world.Snapshot) body.InputState {
	if r == nil {
		return body.InputState{}
	}

	r.mu.Lock()
	handles := make([]*behaviorHandle, 0, len(r.active))
	for _, h := range r.active {
		handles = append(handles, h)
	}
	r.mu.Unlock()

	sort.Slice(handles, func(i, j int) bool {
		if handles[i].priority == handles[j].priority {
			return handles[i].name < handles[j].name
		}
		return handles[i].priority > handles[j].priority
	})

	for _, h := range handles {
		pushLatestSnapshot(h.tickCh, snap)
	}

	out := body.InputState{}
	for _, h := range handles {
		partial, ok := drainLatestPartial(h.outputCh)
		if !ok {
			continue
		}
		applyPartialByChannels(&out, partial, h.channels)
	}

	return out
}

func (r *BehaviorRunner) Cancel(name string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	h := r.active[name]
	r.mu.Unlock()
	if h != nil {
		h.markStopReason(BehaviorExitCancelled)
		h.cancel()
	}
}

func (r *BehaviorRunner) CancelAll() {
	if r == nil {
		return
	}
	r.mu.Lock()
	handles := make([]*behaviorHandle, 0, len(r.active))
	for _, h := range r.active {
		handles = append(handles, h)
	}
	r.mu.Unlock()

	for _, h := range handles {
		h.markStopReason(BehaviorExitCancelled)
		h.cancel()
	}
}

func (r *BehaviorRunner) BehaviorEnds() <-chan BehaviorEnd {
	if r == nil {
		return nil
	}
	return r.endCh
}

func (r *BehaviorRunner) Active() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	names := make([]string, 0, len(r.active))
	for name := range r.active {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *BehaviorRunner) ActiveCount() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.active)
}

func (r *BehaviorRunner) OwnsChannel(ch Channel) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.channelOwners[ch]
	return ok
}

func (r *BehaviorRunner) cleanup(handle *behaviorHandle, ctx context.Context, err error) {
	if handle == nil {
		return
	}

	reason := handle.getStopReason()
	if reason == "" {
		switch {
		case ctx != nil && ctx.Err() != nil:
			reason = BehaviorExitCancelled
		case err != nil:
			reason = BehaviorExitFailed
		default:
			reason = BehaviorExitCompleted
		}
	}

	r.mu.Lock()
	current := r.active[handle.name]
	if current != handle {
		r.mu.Unlock()
		r.emitBehaviorEnd(handle, reason, err)
		return
	}
	delete(r.active, handle.name)
	for ch, owner := range r.channelOwners {
		if owner == handle.name {
			delete(r.channelOwners, ch)
		}
	}
	r.mu.Unlock()

	r.emitBehaviorEnd(handle, reason, err)
}

func (r *BehaviorRunner) findConflictsLocked(channels []Channel) []string {
	set := make(map[string]struct{})
	for _, ch := range channels {
		if name, ok := r.channelOwners[ch]; ok {
			set[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (r *BehaviorRunner) preemptLocked(name string) {
	h := r.active[name]
	if h == nil {
		return
	}
	h.markStopReason(BehaviorExitPreempted)
	h.cancel()
	delete(r.active, name)
	for ch, owner := range r.channelOwners {
		if owner == name {
			delete(r.channelOwners, ch)
		}
	}
}

func (r *BehaviorRunner) emitBehaviorEnd(handle *behaviorHandle, reason BehaviorExitReason, err error) {
	if r == nil || handle == nil {
		return
	}
	evt := BehaviorEnd{
		Name:   handle.name,
		RunID:  handle.runID,
		Reason: reason,
		Err:    err,
	}
	r.endCh <- evt
}

func (h *behaviorHandle) markStopReason(reason BehaviorExitReason) {
	if h == nil || reason == "" {
		return
	}
	h.stopReasonMu.Lock()
	if h.stopReason == "" {
		h.stopReason = reason
	}
	h.stopReasonMu.Unlock()
}

func (h *behaviorHandle) getStopReason() BehaviorExitReason {
	if h == nil {
		return ""
	}
	h.stopReasonMu.Lock()
	reason := h.stopReason
	h.stopReasonMu.Unlock()
	return reason
}

func pushLatestSnapshot(ch chan world.Snapshot, snap world.Snapshot) {
	select {
	case ch <- snap:
		return
	default:
	}

	select {
	case <-ch:
	default:
	}

	select {
	case ch <- snap:
	default:
	}
}

func drainLatestPartial(ch chan PartialInput) (PartialInput, bool) {
	var (
		last PartialInput
		ok   bool
	)
	for {
		select {
		case p := <-ch:
			last = p
			ok = true
		default:
			return last, ok
		}
	}
}

func applyPartialByChannels(out *body.InputState, p PartialInput, channels map[Channel]struct{}) {
	if out == nil {
		return
	}

	if _, ok := channels[ChannelLegs]; ok {
		if p.Forward != nil {
			out.Forward = *p.Forward
		}
		if p.Backward != nil {
			out.Backward = *p.Backward
		}
		if p.Left != nil {
			out.Left = *p.Left
		}
		if p.Right != nil {
			out.Right = *p.Right
		}
		if p.Jump != nil {
			out.Jump = *p.Jump
		}
		if p.Sneak != nil {
			out.Sneak = *p.Sneak
		}
		if p.Sprint != nil {
			out.Sprint = *p.Sprint
		}
	}

	if _, ok := channels[ChannelHead]; ok {
		if p.Yaw != nil {
			out.Yaw = *p.Yaw
		}
		if p.Pitch != nil {
			out.Pitch = *p.Pitch
		}
	}

	if _, ok := channels[ChannelHands]; ok {
		if p.Attack != nil {
			out.Attack = *p.Attack
		}
		if p.Use != nil {
			out.Use = *p.Use
		}
		if p.AttackTarget != nil {
			v := *p.AttackTarget
			out.AttackTarget = &v
		}
		if p.BreakTarget != nil {
			v := *p.BreakTarget
			out.BreakTarget = &v
		}
		if p.BreakFinished != nil {
			out.BreakFinished = *p.BreakFinished
		}
		if p.PlaceTarget != nil {
			v := *p.PlaceTarget
			out.PlaceTarget = &v
		}
		if p.InteractTarget != nil {
			v := *p.InteractTarget
			out.InteractTarget = &v
		}
		if p.HotbarSlot != nil {
			v := *p.HotbarSlot
			out.HotbarSlot = &v
		}
	}
}
