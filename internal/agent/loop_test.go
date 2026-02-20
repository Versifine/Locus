package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

type loopTestSender struct {
	mu   sync.Mutex
	msgs []string
}

func (s *loopTestSender) SendMsgToServer(message string) error {
	s.mu.Lock()
	s.msgs = append(s.msgs, message)
	s.mu.Unlock()
	return nil
}

func (s *loopTestSender) list() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.msgs))
	copy(out, s.msgs)
	return out
}

type loopTestState struct{}

func (loopTestState) GetState() world.Snapshot {
	return world.Snapshot{Position: world.Position{X: 1, Y: 2, Z: 3}, Health: 20}
}

func TestShouldThinkUrgentAndCooldown(t *testing.T) {
	a := &LoopAgent{
		eventBuffer: NewEventBuffer(10),
		lastThinkAt: time.Now(),
		runner:      nil,
	}
	a.eventBuffer.Push("chat", nil, PriorityUrgent)

	if !a.shouldThink() {
		t.Fatal("urgent event should trigger think")
	}

	a.eventBuffer.DrainAll()
	a.lastThinkAt = time.Now()
	if a.shouldThink() {
		t.Fatal("cooldown should block immediate think")
	}

	a.lastThinkAt = time.Now().Add(-2 * thinkCooldown)
	if !a.shouldThink() {
		t.Fatal("cooldown elapsed should allow think when idle")
	}
}

func TestDrainThinkerActions(t *testing.T) {
	sender := &loopTestSender{}
	runner := skill.NewBehaviorRunner(nil, nil, nil)
	a := &LoopAgent{
		sender:   sender,
		runner:   runner,
		speakCh:  make(chan string, 2),
		intentCh: make(chan Intent, 2),
		behaviorDeps: skill.BehaviorDeps{
			Idle: func(durationMs int) skill.BehaviorFunc {
				_ = durationMs
				return func(bctx skill.BehaviorCtx) error {
					<-bctx.Done()
					return nil
				}
			},
		},
	}

	a.speakCh <- "hello"
	a.intentCh <- Intent{Action: "idle", Params: map[string]any{}}
	a.drainThinkerActions()

	msgs := sender.list()
	if len(msgs) != 1 || msgs[0] != "hello" {
		t.Fatalf("sender msgs=%v", msgs)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runner.ActiveCount() > 0 {
			runner.CancelAll()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected intent to start behavior")
}

func TestWaitForIdle(t *testing.T) {
	runner := skill.NewBehaviorRunner(nil, nil, nil)
	started := make(chan struct{}, 1)
	if !runner.Start("hold", func(bctx skill.BehaviorCtx) error {
		started <- struct{}{}
		<-bctx.Done()
		return nil
	}, []skill.Channel{skill.ChannelLegs}, 1) {
		t.Fatal("failed to start hold behavior")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting behavior start")
	}

	a := &LoopAgent{runner: runner, stateProvider: loopTestState{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(120 * time.Millisecond)
		runner.CancelAll()
	}()

	result, err := a.waitForIdle(ctx, time.Second)
	if err != nil {
		t.Fatalf("waitForIdle error: %v", err)
	}
	if result["status"] != "idle" {
		t.Fatalf("status=%v want idle", result["status"])
	}
}

func TestStartThinkerTimeout(t *testing.T) {
	client := &fakeThinkerClient{
		config: config.LLMConfig{SystemPrompt: "test"},
		blockFn: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	a := &LoopAgent{
		llmClient:      client,
		eventBuffer:    NewEventBuffer(10),
		toolDefs:       ToLLMTools(AllTools()),
		toolExecutor:   ToolExecutor{},
		thinkerTimeout: 80 * time.Millisecond,
	}

	a.startThinker(context.Background(), worldSnapshotForTest())
	time.Sleep(20 * time.Millisecond)

	a.thinkerMu.Lock()
	started := a.thinkerRunning
	a.thinkerMu.Unlock()
	if !started {
		t.Fatal("expected thinker to start")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		a.thinkerMu.Lock()
		running := a.thinkerRunning
		a.thinkerMu.Unlock()
		if !running {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("thinker should stop after timeout")
}

func TestHeadInterpolationProgressesByConfiguredSpeed(t *testing.T) {
	a := &LoopAgent{}
	a.syncHeadCurrent(0, 0)
	a.setHead(90, 0)

	yaw, pitch := a.interpolateHead()
	if yaw != 15 || pitch != 0 {
		t.Fatalf("first interpolate yaw/pitch=%.2f/%.2f want 15/0", yaw, pitch)
	}

	for i := 0; i < 5; i++ {
		yaw, pitch = a.interpolateHead()
	}
	if yaw != 90 || pitch != 0 {
		t.Fatalf("after six ticks yaw/pitch=%.2f/%.2f want 90/0", yaw, pitch)
	}
	if a.headInterpolating {
		t.Fatal("expected interpolation to complete at target")
	}
}

func TestHeadInterpolationWrapsAcross180(t *testing.T) {
	a := &LoopAgent{}
	a.syncHeadCurrent(170, 0)
	a.setHead(-170, 0)

	yaw, _ := a.interpolateHead()
	if yaw != -175 {
		t.Fatalf("wrapped first step yaw=%.2f want -175", yaw)
	}

	yaw, _ = a.interpolateHead()
	if yaw != -170 {
		t.Fatalf("wrapped second step yaw=%.2f want -170", yaw)
	}
}
