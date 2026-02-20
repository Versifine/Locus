package agent

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Versifine/locus/internal/body"
	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/skill/behaviors"
	"github.com/Versifine/locus/internal/world"
)

const (
	loopTickInterval        = 50 * time.Millisecond
	thinkCooldown           = 800 * time.Millisecond
	thinkIdleInterval       = 6 * time.Second
	thinkEventThreshold     = 4
	thinkerDefaultTimeout   = 120 * time.Second
	episodeOpenTimeout      = 5 * time.Minute
	eventInChanBuffer       = 256
	thinkerActionChanSize   = 64
	waitForIdlePollTick     = 50 * time.Millisecond
	defaultWaitForIdleTime  = 10 * time.Second
	pendingEndTTLInTicks    = 12000
	pendingEndMaxEntries    = 256
	defaultHeadSpeedDegTick = 15.0
	headDoneThresholdDeg    = 1.0
)

type incomingEvent struct {
	name     string
	payload  any
	priority Priority
	tickID   uint64
}

type LoopAgent struct {
	bus           *event.Bus
	sender        MessageSender
	stateProvider StateProvider
	body          *body.Body
	runner        *skill.BehaviorRunner
	llmClient     thinkerClient

	attention     *Attention
	spatialMemory *SpatialMemory
	eventBuffer   *EventBuffer
	eventInCh     chan incomingEvent

	speakCh  chan string
	intentCh chan Intent

	toolExecutor   ToolExecutor
	toolDefs       []llm.ToolDefinition
	behaviorDeps   skill.BehaviorDeps
	thinkerTimeout time.Duration

	thinkerMu      sync.Mutex
	thinkerRunning bool
	thinkerCancel  context.CancelFunc
	lastThinkAt    time.Time
	idleSince      time.Time

	memoryStore *MemoryStore
	episodeLog  *EpisodeLog

	contextMu    sync.Mutex
	activePlayer string

	autoRuleMu       sync.Mutex
	autoRuleLastTick map[string]uint64

	episodeRunMu       sync.Mutex
	episodeByRunID     map[uint64]string
	pendingBehaviorEnd map[uint64]pendingBehaviorEnd

	thinkCtxMu        sync.Mutex
	thinkCtxStartTick uint64
	thinkCtxTrigger   string
	thinkCtxEvents    []BufferedEvent
	thinkCtxRuns      map[uint64]string

	headMu            sync.Mutex
	headTargetYaw     float32
	headTargetPitch   float32
	headCurrentYaw    float32
	headCurrentPitch  float32
	headInterpolating bool
	headSpeed         float32

	tickCounter atomic.Uint64
}

func NewLoopAgent(
	bus *event.Bus,
	sender MessageSender,
	stateProvider StateProvider,
	bodyController *body.Body,
	runner *skill.BehaviorRunner,
	llmClient thinkerClient,
	worldAccess BlockAccess,
	camera Camera,
) *LoopAgent {
	if camera.FOV <= 0 || camera.MaxDist <= 0 || camera.Width <= 0 || camera.Height <= 0 {
		camera = DefaultCamera()
	}

	a := &LoopAgent{
		bus:                bus,
		sender:             sender,
		stateProvider:      stateProvider,
		body:               bodyController,
		runner:             runner,
		llmClient:          llmClient,
		attention:          NewAttention(bus),
		spatialMemory:      NewSpatialMemory(),
		eventBuffer:        NewEventBuffer(100),
		eventInCh:          make(chan incomingEvent, eventInChanBuffer),
		speakCh:            make(chan string, thinkerActionChanSize),
		intentCh:           make(chan Intent, thinkerActionChanSize),
		toolDefs:           ToLLMTools(AllTools()),
		behaviorDeps:       behaviors.Deps(),
		thinkerTimeout:     thinkerDefaultTimeout,
		memoryStore:        NewMemoryStore(defaultMemoryCapacity),
		episodeLog:         NewEpisodeLog(defaultEpisodeCapacity),
		autoRuleLastTick:   map[string]uint64{},
		episodeByRunID:     map[uint64]string{},
		pendingBehaviorEnd: map[uint64]pendingBehaviorEnd{},
		thinkCtxRuns:       map[uint64]string{},
		headSpeed:          defaultHeadSpeedDegTick,
	}

	a.toolExecutor = ToolExecutor{
		SnapshotFn:    stateProvider.GetState,
		World:         worldAccess,
		Camera:        camera,
		SpatialMemory: a.spatialMemory,
		TickIDFn:      a.tickCounter.Load,
		SpeakChan:     a.speakCh,
		IntentChan:    a.intentCh,
		CancelAll:     runner.CancelAll,
		SetHead:       a.setHead,
		Recall:        a.recallMemory,
		Remember:      a.rememberMemory,
		WaitForIdle: func(ctx context.Context, timeout time.Duration) (map[string]any, error) {
			return a.waitForIdle(ctx, timeout)
		},
	}
	a.attention.SpatialMemory = a.spatialMemory

	a.subscribeEvents()
	return a
}

func (a *LoopAgent) setHead(yaw, pitch float32) {
	if a == nil {
		return
	}
	a.headMu.Lock()
	if a.headSpeed <= 0 {
		a.headSpeed = defaultHeadSpeedDegTick
	}
	a.headTargetYaw = yaw
	a.headTargetPitch = pitch
	a.headInterpolating = true
	a.headMu.Unlock()
}

func (a *LoopAgent) syncHeadCurrent(yaw, pitch float32) {
	if a == nil {
		return
	}
	a.headMu.Lock()
	a.headCurrentYaw = yaw
	a.headCurrentPitch = pitch
	if !a.headInterpolating {
		a.headTargetYaw = yaw
		a.headTargetPitch = pitch
	}
	a.headMu.Unlock()
}

func (a *LoopAgent) interpolateHead() (yaw, pitch float32) {
	if a == nil {
		return 0, 0
	}
	a.headMu.Lock()
	defer a.headMu.Unlock()

	if a.headSpeed <= 0 {
		a.headSpeed = defaultHeadSpeedDegTick
	}

	if a.headInterpolating {
		a.headCurrentYaw = lerpAngle(a.headCurrentYaw, a.headTargetYaw, a.headSpeed)
		a.headCurrentPitch = lerpAngle(a.headCurrentPitch, a.headTargetPitch, a.headSpeed)
		if angleDiff(a.headCurrentYaw, a.headTargetYaw) < headDoneThresholdDeg &&
			angleDiff(a.headCurrentPitch, a.headTargetPitch) < headDoneThresholdDeg {
			a.headCurrentYaw = a.headTargetYaw
			a.headCurrentPitch = a.headTargetPitch
			a.headInterpolating = false
		}
	}

	return a.headCurrentYaw, a.headCurrentPitch
}

func (a *LoopAgent) SetInventory(inv InventoryProvider) {
	if a == nil {
		return
	}
	a.toolExecutor.Inventory = inv
}

func (a *LoopAgent) Start(ctx context.Context) error {
	if a == nil {
		return nil
	}
	if a.runner == nil || a.body == nil || a.stateProvider == nil {
		slog.Warn("LoopAgent missing dependencies")
		return nil
	}

	go a.forwardBehaviorEnds(ctx)
	initial := a.stateProvider.GetState()
	a.syncHeadCurrent(initial.Position.Yaw, initial.Position.Pitch)

	ticker := time.NewTicker(loopTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.cancelThinker()
			return nil
		case <-ticker.C:
			a.tick(ctx, a.stateProvider.GetState())
		}
	}
}

func (a *LoopAgent) tick(ctx context.Context, snap world.Snapshot) {
	if a == nil {
		return
	}
	tickID := a.tickCounter.Add(1)

	input := a.runner.Tick(snap)
	if a.runner.OwnsChannel(skill.ChannelHead) {
		a.syncHeadCurrent(snap.Position.Yaw, snap.Position.Pitch)
	} else {
		yaw, pitch := a.interpolateHead()
		input.Yaw = yaw
		input.Pitch = pitch
	}
	if err := a.body.Tick(input); err != nil {
		if !strings.Contains(err.Error(), "connection is not initialized") {
			slog.Warn("Agent loop body tick failed", "error", err)
		}
	}

	a.attention.Tick(snap, tickID)
	a.drainEvents()
	a.closeTimedOutEpisodes()
	a.cleanupPendingBehaviorEnds(tickID)

	if a.shouldThink() {
		a.startThinker(ctx, snap)
	}

	a.drainThinkerActions()

	if a.runner.ActiveCount() == 0 {
		if a.idleSince.IsZero() {
			a.idleSince = time.Now()
		}
	} else {
		a.idleSince = time.Time{}
	}
}

func (a *LoopAgent) drainEvents() {
	if a == nil {
		return
	}
	for {
		select {
		case evt := <-a.eventInCh:
			a.observeIncomingEvent(evt)
			a.eventBuffer.PushAt(evt.name, evt.payload, evt.priority, evt.tickID)
		default:
			return
		}
	}
}

func (a *LoopAgent) shouldThink() bool {
	if a == nil {
		return false
	}

	a.thinkerMu.Lock()
	running := a.thinkerRunning
	a.thinkerMu.Unlock()

	if running {
		if a.eventBuffer.HasUrgent() {
			a.cancelThinker()
		}
		return false
	}

	now := time.Now()
	if a.eventBuffer.HasUrgent() {
		return true
	}
	if a.runner.ActiveCount() == 0 && now.Sub(a.lastThinkAt) >= thinkCooldown {
		return true
	}
	if a.eventBuffer.Len() >= thinkEventThreshold && now.Sub(a.lastThinkAt) >= thinkCooldown {
		return true
	}
	if a.runner.ActiveCount() == 0 && !a.idleSince.IsZero() && now.Sub(a.idleSince) >= thinkIdleInterval {
		return true
	}

	return false
}

func (a *LoopAgent) startThinker(parent context.Context, snap world.Snapshot) {
	if a == nil || a.llmClient == nil {
		return
	}
	a.thinkerMu.Lock()
	if a.thinkerRunning {
		a.thinkerMu.Unlock()
		return
	}
	a.thinkerRunning = true
	timeout := a.thinkerTimeout
	if timeout <= 0 {
		timeout = thinkerDefaultTimeout
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	a.thinkerCancel = cancel
	a.thinkerMu.Unlock()

	events := a.eventBuffer.DrainAll()
	shortTerm := a.shortTermMemoryForPrompt()
	startTick := a.tickCounter.Load()
	a.setThinkContext(startTick, events)
	t := newThinker(a.llmClient, a.toolDefs, a.toolExecutor, a.runner)

	go func() {
		defer func() {
			a.clearThinkContext(startTick)
			cancel()
			a.thinkerMu.Lock()
			a.thinkerRunning = false
			a.thinkerCancel = nil
			a.lastThinkAt = time.Now()
			a.thinkerMu.Unlock()
		}()

		trace, err := t.think(ctx, snap, events, shortTerm)
		a.onThinkerFinished(startTick, events, trace, err)
		if err != nil && ctx.Err() == nil {
			slog.Warn("Thinker exited with error", "error", err)
		}
	}()
}

func (a *LoopAgent) cancelThinker() {
	if a == nil {
		return
	}
	a.thinkerMu.Lock()
	cancel := a.thinkerCancel
	a.thinkerMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *LoopAgent) drainThinkerActions() {
	if a == nil {
		return
	}
	for {
		select {
		case msg := <-a.speakCh:
			if a.sender != nil {
				if err := a.sender.SendMsgToServer(msg); err != nil {
					slog.Warn("failed to send thinker message", "error", err)
				}
			}
		case intent := <-a.intentCh:
			a.startIntent(intent)
		default:
			return
		}
	}
}

func (a *LoopAgent) startIntent(intent Intent) {
	if a == nil || a.runner == nil {
		return
	}
	fn, channels, priority, err := skill.MapIntentToBehavior(intent, a.behaviorDeps)
	if err != nil {
		slog.Warn("set_intent mapping failed", "action", intent.Action, "error", err)
		return
	}
	ok, runID := a.runner.StartWithRunID(intent.Action, fn, channels, priority)
	if !ok {
		slog.Warn("set_intent start rejected", "action", intent.Action)
		return
	}
	a.onBehaviorStartedFromIntent(intent, runID)
}

func (a *LoopAgent) waitForIdle(ctx context.Context, timeout time.Duration) (map[string]any, error) {
	if timeout <= 0 {
		timeout = defaultWaitForIdleTime
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(waitForIdlePollTick)
	defer ticker.Stop()

	for {
		if a.runner == nil || a.runner.ActiveCount() == 0 {
			snap := world.Snapshot{}
			if a.stateProvider != nil {
				snap = a.stateProvider.GetState()
			}
			return map[string]any{
				"status":   "idle",
				"position": []float64{snap.Position.X, snap.Position.Y, snap.Position.Z},
				"hp":       snap.Health,
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
			return map[string]any{"status": "timeout"}, nil
		case <-ticker.C:
		}
	}
}

func (a *LoopAgent) subscribeEvents() {
	if a == nil || a.bus == nil {
		return
	}

	a.bus.Subscribe(event.EventChat, func(raw any) {
		a.enqueueEvent(event.EventChat, raw, PriorityUrgent)
	})
	a.bus.Subscribe(event.EventDamage, func(raw any) {
		a.enqueueEvent(event.EventDamage, raw, PriorityUrgent)
	})
	a.bus.Subscribe(event.EventBehaviorEnd, func(raw any) {
		a.enqueueEvent(event.EventBehaviorEnd, raw, PriorityNormal)
	})
	a.bus.Subscribe(event.EventEntityAppear, func(raw any) {
		a.enqueueEvent(event.EventEntityAppear, raw, PriorityLow)
	})
	a.bus.Subscribe(event.EventEntityLeave, func(raw any) {
		a.enqueueEvent(event.EventEntityLeave, raw, PriorityLow)
	})
}

func (a *LoopAgent) enqueueEvent(name string, payload any, priority Priority) {
	if a == nil {
		return
	}
	select {
	case a.eventInCh <- incomingEvent{name: name, payload: payload, priority: priority, tickID: a.tickCounter.Load()}:
	default:
		if priority == PriorityUrgent {
			select {
			case <-a.eventInCh:
			default:
			}
			select {
			case a.eventInCh <- incomingEvent{name: name, payload: payload, priority: priority, tickID: a.tickCounter.Load()}:
			default:
			}
		}
	}
}

func (a *LoopAgent) forwardBehaviorEnds(ctx context.Context) {
	if a == nil || a.bus == nil || a.runner == nil {
		return
	}
	ch := a.runner.BehaviorEnds()
	if ch == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-ch:
			a.bus.Publish(event.EventBehaviorEnd, event.BehaviorEndEvent{
				Name:   evt.Name,
				RunID:  evt.RunID,
				Reason: string(evt.Reason),
			})
		}
	}
}
