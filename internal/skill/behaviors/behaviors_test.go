package behaviors

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

type mockBlocks struct {
	mu     sync.RWMutex
	states map[skill.BlockPos]int32
}

func newMockBlocks() *mockBlocks {
	return &mockBlocks{states: make(map[skill.BlockPos]int32)}
}

func (m *mockBlocks) SetState(pos skill.BlockPos, state int32) {
	m.mu.Lock()
	m.states[pos] = state
	m.mu.Unlock()
}

func (m *mockBlocks) GetBlockState(x, y, z int) (int32, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[skill.BlockPos{X: x, Y: y, Z: z}]
	if !ok {
		return 0, true
	}
	return state, true
}

func (m *mockBlocks) GetBlockNameByStateID(stateID int32) (string, bool) {
	if stateID == 0 {
		return "air", true
	}
	return "stone", true
}

func (m *mockBlocks) IsSolid(x, y, z int) bool {
	state, _ := m.GetBlockState(x, y, z)
	return state != 0
}

func newFlatBlocks(minX, maxX, minZ, maxZ int, groundY int) *mockBlocks {
	b := newMockBlocks()
	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			b.SetState(skill.BlockPos{X: x, Y: groundY, Z: z}, 1)
		}
	}
	return b
}

type behaviorHarness struct {
	t      *testing.T
	ctx    context.Context
	cancel context.CancelFunc
	tickCh chan world.Snapshot
	outCh  chan skill.PartialInput
	doneCh chan error

	mu      sync.RWMutex
	current world.Snapshot
}

func startBehaviorHarness(t *testing.T, fn skill.BehaviorFunc, blocks skill.BlockAccess, initial world.Snapshot) *behaviorHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := &behaviorHarness{
		t:       t,
		ctx:     ctx,
		cancel:  cancel,
		tickCh:  make(chan world.Snapshot, 1),
		outCh:   make(chan skill.PartialInput, 4),
		doneCh:  make(chan error, 1),
		current: initial,
	}

	bctx := skill.BehaviorCtx{
		Ctx:    ctx,
		Tick:   h.tickCh,
		Output: h.outCh,
		Blocks: blocks,
		SnapshotFn: func() world.Snapshot {
			h.mu.RLock()
			defer h.mu.RUnlock()
			return h.current
		},
	}

	go func() {
		h.doneCh <- fn(bctx)
	}()

	return h
}

func (h *behaviorHarness) pullOutput() skill.PartialInput {
	h.t.Helper()
	select {
	case out := <-h.outCh:
		return out
	case <-time.After(time.Second):
		h.t.Fatal("timeout waiting behavior output")
		return skill.PartialInput{}
	}
}

func (h *behaviorHarness) pushSnapshot(snap world.Snapshot) {
	h.t.Helper()
	h.mu.Lock()
	h.current = snap
	h.mu.Unlock()

	select {
	case h.tickCh <- snap:
	case <-time.After(time.Second):
		h.t.Fatal("timeout pushing snapshot")
	}
}

func (h *behaviorHarness) waitDone() error {
	h.t.Helper()
	select {
	case err := <-h.doneCh:
		return err
	case <-time.After(time.Second):
		h.t.Fatal("timeout waiting behavior completion")
		return nil
	}
}

func TestIdleOutputsHeadAndLegs(t *testing.T) {
	h := startBehaviorHarness(t, Idle(), nil, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	out := h.pullOutput()
	if out.Yaw == nil {
		t.Fatal("expected yaw output")
	}
	h.cancel()
	if err := h.waitDone(); err != nil {
		t.Fatalf("idle returned error: %v", err)
	}
}

func TestGoToReachesTarget(t *testing.T) {
	blocks := newFlatBlocks(-2, 8, -2, 2, 0)
	h := startBehaviorHarness(t, GoTo(2, 1, 0, false), blocks, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})

	out1 := h.pullOutput()
	if out1.Forward == nil || !*out1.Forward {
		t.Fatal("expected forward movement on first tick")
	}
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 1, Y: 1, Z: 0}})

	_ = h.pullOutput()
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 2, Y: 1, Z: 0}})

	if err := h.waitDone(); err != nil {
		t.Fatalf("go_to returned error: %v", err)
	}
}

func TestGoToFailsWhenTargetUnreachable(t *testing.T) {
	blocks := newFlatBlocks(-2, 8, -2, 2, 0)
	for x := 1; x <= 3; x++ {
		for z := -2; z <= 2; z++ {
			blocks.SetState(skill.BlockPos{X: x, Y: 1, Z: z}, 1)
			blocks.SetState(skill.BlockPos{X: x, Y: 2, Z: z}, 1)
		}
	}

	h := startBehaviorHarness(t, GoTo(4, 1, 0, false), blocks, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	for i := 0; i < 80; i++ {
		select {
		case err := <-h.doneCh:
			if err == nil {
				t.Fatal("expected unreachable go_to error")
			}
			return
		case <-time.After(time.Second):
			t.Fatal("timeout waiting go_to tick or completion")
			return
		case <-h.outCh:
			h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
		}
	}

	err := h.waitDone()
	if err == nil {
		t.Fatal("expected unreachable go_to error")
	}
}

func TestGoToRecoversAfterPushBack(t *testing.T) {
	blocks := newFlatBlocks(-2, 8, -2, 2, 0)
	h := startBehaviorHarness(t, GoTo(3, 1, 0, false), blocks, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})

	out1 := h.pullOutput()
	if out1.Forward == nil || !*out1.Forward {
		t.Fatal("expected first tick forward")
	}
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 1, Y: 1, Z: 0}})

	out2 := h.pullOutput()
	if out2.Forward == nil || !*out2.Forward {
		t.Fatal("expected movement before disturbance")
	}
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0.2, Y: 1, Z: 0}})

	out3 := h.pullOutput()
	if out3.Forward == nil || !*out3.Forward {
		t.Fatal("expected behavior to recover and move toward target after push")
	}

	h.cancel()
	if err := h.waitDone(); err != nil {
		t.Fatalf("go_to recovery case returned error: %v", err)
	}
}

func TestFollowStopsWhenEntityMissing(t *testing.T) {
	blocks := newFlatBlocks(-2, 8, -2, 2, 0)
	fn := Follow(42, 2.5)
	h := startBehaviorHarness(t, fn, blocks, world.Snapshot{
		Position: world.Position{X: 0, Y: 1, Z: 0},
		Entities: []world.Entity{{EntityID: 42, X: 5, Y: 1, Z: 0}},
	})

	out := h.pullOutput()
	if out.Forward == nil || !*out.Forward {
		t.Fatal("expected follow to move toward target")
	}
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0.5, Y: 1, Z: 0}})

	if err := h.waitDone(); err != nil {
		t.Fatalf("follow returned error: %v", err)
	}
}

func TestFollowUsesPathAroundObstacle(t *testing.T) {
	blocks := newFlatBlocks(-2, 8, -4, 4, 0)
	for z := -1; z <= 1; z++ {
		blocks.SetState(skill.BlockPos{X: 1, Y: 1, Z: z}, 1)
		blocks.SetState(skill.BlockPos{X: 1, Y: 2, Z: z}, 1)
	}

	h := startBehaviorHarness(t, Follow(42, 2.5), blocks, world.Snapshot{
		Position: world.Position{X: 0, Y: 1, Z: 0},
		Entities: []world.Entity{{EntityID: 42, X: 5, Y: 1, Z: 0}},
	})

	out := h.pullOutput()
	if out.Forward == nil || !*out.Forward {
		t.Fatal("expected follow to move when obstacle blocks straight line")
	}
	if out.Yaw == nil {
		t.Fatal("expected follow yaw output")
	}
	if *out.Yaw <= -89 && *out.Yaw >= -91 {
		t.Fatalf("expected pathfinding yaw to deviate from straight chase, got %.2f", *out.Yaw)
	}

	h.cancel()
	if err := h.waitDone(); err != nil {
		t.Fatalf("follow pathfinding returned error: %v", err)
	}
}

func TestLookAtPosCompletesWhenAligned(t *testing.T) {
	target := skill.Vec3{X: 0, Y: 1, Z: 10}
	h := startBehaviorHarness(t, LookAtPos(target), nil, world.Snapshot{
		Position: world.Position{X: 0, Y: 1, Z: 0, Yaw: 90, Pitch: 0},
	})

	out := h.pullOutput()
	if out.Yaw == nil {
		t.Fatal("expected yaw output")
	}
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0, Yaw: 0, Pitch: 0}})

	if err := h.waitDone(); err != nil {
		t.Fatalf("look_at pos returned error: %v", err)
	}
}

func TestLookAtEntityTracksUntilGone(t *testing.T) {
	h := startBehaviorHarness(t, LookAtEntity(7), nil, world.Snapshot{
		Position: world.Position{X: 0, Y: 1, Z: 0},
		Entities: []world.Entity{{EntityID: 7, X: 0, Y: 1, Z: 6}},
	})

	out1 := h.pullOutput()
	if out1.Yaw == nil {
		t.Fatal("expected first look output")
	}

	h.pushSnapshot(world.Snapshot{
		Position: world.Position{X: 0, Y: 1, Z: 0},
		Entities: []world.Entity{{EntityID: 7, X: 6, Y: 1, Z: 0}},
	})
	out2 := h.pullOutput()
	if out2.Yaw == nil {
		t.Fatal("expected tracking output")
	}

	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	if err := h.waitDone(); err != nil {
		t.Fatalf("look_at entity returned error: %v", err)
	}
}

func TestAttackCooldown(t *testing.T) {
	blocks := newFlatBlocks(-4, 8, -4, 4, 0)
	h := startBehaviorHarness(t, Attack(9), blocks, world.Snapshot{
		Position: world.Position{X: 0, Y: 1, Z: 0},
		Entities: []world.Entity{{EntityID: 9, X: 1, Y: 1, Z: 0}},
	})

	out1 := h.pullOutput()
	if out1.Attack == nil || !*out1.Attack {
		t.Fatal("expected first tick attack")
	}
	if out1.AttackTarget == nil || *out1.AttackTarget != 9 {
		t.Fatalf("unexpected attack target: %+v", out1.AttackTarget)
	}

	h.pushSnapshot(world.Snapshot{
		Position: world.Position{X: 0, Y: 1, Z: 0},
		Entities: []world.Entity{{EntityID: 9, X: 1, Y: 1, Z: 0}},
	})
	out2 := h.pullOutput()
	if out2.Attack != nil && *out2.Attack {
		t.Fatal("expected cooldown tick without attack")
	}

	h.cancel()
	if err := h.waitDone(); err != nil {
		t.Fatalf("attack returned error: %v", err)
	}
}

func TestMineSetsSlotAndBreakTarget(t *testing.T) {
	blocks := newFlatBlocks(-2, 4, -2, 2, 0)
	target := skill.BlockPos{X: 1, Y: 1, Z: 0}
	blocks.SetState(target, 1)
	slot := int8(2)

	h := startBehaviorHarness(t, Mine(target, &slot), blocks, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	out := h.pullOutput()
	if out.HotbarSlot == nil || *out.HotbarSlot != 2 {
		t.Fatalf("expected first tick slot switch, got %+v", out.HotbarSlot)
	}
	if out.Attack == nil || !*out.Attack || out.BreakTarget == nil {
		t.Fatal("expected mine attack output")
	}

	blocks.SetState(target, 0)
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	if err := h.waitDone(); err != nil {
		t.Fatalf("mine returned error: %v", err)
	}
}

func TestPlaceBlockWaitsForConfirmation(t *testing.T) {
	blocks := newFlatBlocks(-2, 4, -2, 2, 0)
	target := skill.BlockPos{X: 1, Y: 1, Z: 0}
	slot := int8(3)

	h := startBehaviorHarness(t, PlaceBlock(target, 1, &slot), blocks, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	out := h.pullOutput()
	if out.HotbarSlot == nil || *out.HotbarSlot != 3 {
		t.Fatalf("expected slot switch, got %+v", out.HotbarSlot)
	}
	if out.Use == nil || !*out.Use || out.PlaceTarget == nil {
		t.Fatal("expected place action output")
	}

	blocks.SetState(target, 1)
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	if err := h.waitDone(); err != nil {
		t.Fatalf("place block returned error: %v", err)
	}
}

func TestPlaceBlockTimeoutWithoutConfirmation(t *testing.T) {
	blocks := newFlatBlocks(-2, 4, -2, 2, 0)
	target := skill.BlockPos{X: 1, Y: 1, Z: 0}
	h := startBehaviorHarness(t, PlaceBlock(target, 1, nil), blocks, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})

	for i := 0; i < 140; i++ {
		select {
		case err := <-h.doneCh:
			if err == nil {
				t.Fatal("expected place_block timeout error")
			}
			return
		case <-time.After(time.Second):
			t.Fatal("timeout waiting place_block tick or completion")
			return
		case <-h.outCh:
			h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
		}
	}

	err := h.waitDone()
	if err == nil {
		t.Fatal("expected place_block timeout error")
	}
}

func TestUseItemKeepsUsingAndSlotOnlyOnce(t *testing.T) {
	slot := int8(1)
	h := startBehaviorHarness(t, UseItem(&slot), nil, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})

	out1 := h.pullOutput()
	if out1.Use == nil || !*out1.Use {
		t.Fatal("expected use on first tick")
	}
	if out1.HotbarSlot == nil || *out1.HotbarSlot != 1 {
		t.Fatal("expected first tick slot switch")
	}

	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	out2 := h.pullOutput()
	if out2.Use == nil || !*out2.Use {
		t.Fatal("expected use on second tick")
	}
	if out2.HotbarSlot != nil {
		t.Fatal("expected slot switch only on first tick")
	}

	h.cancel()
	if err := h.waitDone(); err != nil {
		t.Fatalf("use_item returned error: %v", err)
	}
}

func TestSwitchSlotSingleTick(t *testing.T) {
	h := startBehaviorHarness(t, SwitchSlot(4), nil, world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	out := h.pullOutput()
	if out.HotbarSlot == nil || *out.HotbarSlot != 4 {
		t.Fatalf("expected switch slot output, got %+v", out.HotbarSlot)
	}
	h.pushSnapshot(world.Snapshot{Position: world.Position{X: 0, Y: 1, Z: 0}})
	if err := h.waitDone(); err != nil {
		t.Fatalf("switch_slot returned error: %v", err)
	}
}
