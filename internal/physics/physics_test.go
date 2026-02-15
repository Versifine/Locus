package physics

import (
	"math"
	"testing"
)

type mockBlockStore struct {
	solid map[[3]int]bool
}

func newMockBlockStore() *mockBlockStore {
	return &mockBlockStore{solid: make(map[[3]int]bool)}
}

func (m *mockBlockStore) IsSolid(x, y, z int) bool {
	return m.solid[[3]int{x, y, z}]
}

func (m *mockBlockStore) setSolid(x, y, z int) {
	m.solid[[3]int{x, y, z}] = true
}

func addFloor(store *mockBlockStore, minX, maxX, minZ, maxZ, y int) {
	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			store.setSolid(x, y, z)
		}
	}
}

func approxEqual(t *testing.T, got, want, tol float64, field string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Fatalf("%s = %.8f, want %.8f (tol=%.8f)", field, got, want, tol)
	}
}

func TestPhysicsTick_FreeFallOneTick(t *testing.T) {
	store := newMockBlockStore()
	state := &PhysicsState{
		Position: Vec3{X: 0.0, Y: 10.0, Z: 0.0},
	}

	PhysicsTick(state, InputState{}, store)

	approxEqual(t, state.Position.Y, 10.0, 1e-9, "position.y")
	approxEqual(t, state.Velocity.Y, -0.0784, 1e-9, "velocity.y")
	if state.OnGround {
		t.Fatalf("onGround = true, want false")
	}
}

func TestPhysicsTick_CollisionAgainstWallStopsHorizontalMovement(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -2, 2, -2, 2, -1)
	store.setSolid(1, 0, 0)
	store.setSolid(1, 1, 0)

	state := &PhysicsState{
		Position: Vec3{X: 0.7, Y: 0.0, Z: 0.5},
		OnGround: true,
	}

	PhysicsTick(state, InputState{Right: true, Yaw: 0}, store)

	approxEqual(t, state.Position.X, 0.7, 1e-9, "position.x")
	approxEqual(t, state.Position.Y, 0.0, 1e-9, "position.y")
	if !state.OnGround {
		t.Fatalf("onGround = false, want true")
	}
}

func TestPhysicsTick_JumpReachesExpectedApex(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -4, 4, -4, 4, -1)

	state := &PhysicsState{
		Position: Vec3{X: 0.5, Y: 0.0, Z: 0.5},
		OnGround: true,
	}

	maxY := state.Position.Y
	for i := 0; i < 40; i++ {
		input := InputState{}
		if i == 0 {
			input.Jump = true
		}
		PhysicsTick(state, input, store)
		if state.Position.Y > maxY {
			maxY = state.Position.Y
		}
	}

	if maxY < 1.15 || maxY > 1.35 {
		t.Fatalf("jump apex = %.4f, want around 1.25", maxY)
	}
}

func TestPhysicsTick_CannotStepUpOneBlockWithoutJump(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -2, 4, -2, 2, -1)
	store.setSolid(1, 0, 0)

	state := &PhysicsState{
		Position: Vec3{X: 0.5, Y: 0.0, Z: 0.5},
		OnGround: true,
	}

	for i := 0; i < 20; i++ {
		PhysicsTick(state, InputState{Right: true, Yaw: 0}, store)
	}

	approxEqual(t, state.Position.Y, 0.0, 1e-9, "position.y")
	if state.Position.X > 0.7+1e-9 {
		t.Fatalf("position.x = %.6f, should be blocked by 1-block step", state.Position.X)
	}
}

func TestPhysicsTick_EntityPushMovesPlayerSideways(t *testing.T) {
	store := newMockBlockStore()
	addFloor(store, -2, 2, -2, 2, -1)

	state := &PhysicsState{
		Position: Vec3{X: 0.50, Y: 0.0, Z: 0.50},
		OnGround: true,
	}

	entities := []EntityCollider{
		{X: 0.62, Y: 0.0, Z: 0.50, Width: 0.6, Height: 1.8},
	}
	PhysicsTickWithEntities(state, InputState{}, store, entities)

	if state.Position.X >= 0.50 {
		t.Fatalf("position.x = %.6f, want < 0.50 due to push", state.Position.X)
	}
}
