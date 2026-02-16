package skill

import (
	"context"
	"sort"
	"sync"

	"github.com/Versifine/locus/internal/body"
	"github.com/Versifine/locus/internal/world"
)

type BehaviorFunc func(bctx BehaviorCtx) error

type behaviorHandle struct {
	name     string
	priority int
	channels map[Channel]struct{}

	tickCh   chan world.Snapshot
	outputCh chan PartialInput
	cancel   context.CancelFunc
}

type BehaviorRunner struct {
	mu            sync.Mutex
	active        map[string]*behaviorHandle
	channelOwners map[Channel]string

	send     func(string) error
	snapshot func() world.Snapshot
}

func NewBehaviorRunner(send func(string) error, snapshot func() world.Snapshot) *BehaviorRunner {
	return &BehaviorRunner{
		active:        make(map[string]*behaviorHandle),
		channelOwners: make(map[Channel]string),
		send:          send,
		snapshot:      snapshot,
	}
}

func (r *BehaviorRunner) Start(name string, fn BehaviorFunc, channels []Channel, priority int) bool {
	if r == nil || fn == nil || name == "" {
		return false
	}

	r.mu.Lock()
	if _, exists := r.active[name]; exists {
		r.mu.Unlock()
		return false
	}

	conflicts := r.findConflictsLocked(channels)
	for _, conflictName := range conflicts {
		owner := r.active[conflictName]
		if owner == nil {
			continue
		}
		if priority <= owner.priority {
			r.mu.Unlock()
			return false
		}
	}

	for _, conflictName := range conflicts {
		owner := r.active[conflictName]
		if owner != nil {
			owner.cancel()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	h := &behaviorHandle{
		name:     name,
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
		ctx:      ctx,
		cancel:   cancel,
		tick:     h.tickCh,
		output:   h.outputCh,
		send:     r.send,
		snapshot: r.snapshot,
	}

	go func() {
		defer r.cleanup(name)
		_ = fn(bctx)
	}()

	return true
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
		h.cancel()
	}
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

func (r *BehaviorRunner) cleanup(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := r.active[name]
	if h == nil {
		return
	}
	delete(r.active, name)
	for ch, owner := range r.channelOwners {
		if owner == name {
			delete(r.channelOwners, ch)
		}
	}
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
