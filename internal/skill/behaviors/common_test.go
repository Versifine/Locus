package behaviors

import (
	"testing"

	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

type singleBlockAccess struct {
	state int32
	name  string
	solid bool
}

type mapBlockAccess struct {
	solid map[skill.BlockPos]bool
}

func newMapBlockAccess() *mapBlockAccess {
	return &mapBlockAccess{solid: make(map[skill.BlockPos]bool)}
}

func (m *mapBlockAccess) SetSolid(pos skill.BlockPos, solid bool) {
	m.solid[pos] = solid
}

func (m *mapBlockAccess) GetBlockState(x, y, z int) (int32, bool) {
	if m.solid[skill.BlockPos{X: x, Y: y, Z: z}] {
		return 1, true
	}
	return 0, true
}

func (m *mapBlockAccess) GetBlockNameByStateID(stateID int32) (string, bool) {
	if stateID == 0 {
		return "air", true
	}
	return "stone", true
}

func (m *mapBlockAccess) IsSolid(x, y, z int) bool {
	return m.solid[skill.BlockPos{X: x, Y: y, Z: z}]
}

func (s singleBlockAccess) GetBlockState(x, y, z int) (int32, bool) {
	return s.state, true
}

func (s singleBlockAccess) GetBlockNameByStateID(stateID int32) (string, bool) {
	return s.name, true
}

func (s singleBlockAccess) IsSolid(x, y, z int) bool {
	return s.solid
}

func TestIsAirAtNonSolidNotAir(t *testing.T) {
	blocks := singleBlockAccess{state: 2, name: "Water", solid: false}
	if isAirAt(blocks, skill.BlockPos{X: 0, Y: 1, Z: 0}) {
		t.Fatal("water should not be treated as air")
	}
}

func TestIsAirAtDoesNotMisclassifyStairs(t *testing.T) {
	blocks := singleBlockAccess{state: 3, name: "Oak Stairs", solid: false}
	if isAirAt(blocks, skill.BlockPos{X: 0, Y: 1, Z: 0}) {
		t.Fatal("stairs should not be treated as air")
	}
}

func TestIsAirAtRecognizesAirNames(t *testing.T) {
	blocks := singleBlockAccess{state: 4, name: "Cave Air", solid: false}
	if !isAirAt(blocks, skill.BlockPos{X: 0, Y: 1, Z: 0}) {
		t.Fatal("cave air should be treated as air")
	}
}

func TestEyePosAddsPlayerEyeHeight(t *testing.T) {
	eye := eyePos(world.Position{X: 1.0, Y: 64.0, Z: 2.0})
	if eye.X != 1.0 || eye.Z != 2.0 {
		t.Fatalf("unexpected eye position X/Z: %+v", eye)
	}
	if eye.Y != 65.62 {
		t.Fatalf("unexpected eye Y: %.2f want 65.62", eye.Y)
	}
}

func TestRaycastClearBlockedBySolidBlock(t *testing.T) {
	blocks := newMapBlockAccess()
	blocks.SetSolid(skill.BlockPos{X: 1, Y: 2, Z: 0}, true)

	from := skill.Vec3{X: 0.2, Y: 2.62, Z: 0.2}
	to := skill.Vec3{X: 2.8, Y: 1.5, Z: 0.2}
	if raycastClear(blocks, from, to, nil) {
		t.Fatal("expected LOS to be blocked by solid block")
	}
}

func TestRaycastClearAllowsExcludedBlock(t *testing.T) {
	blocks := newMapBlockAccess()
	blocked := skill.BlockPos{X: 1, Y: 2, Z: 0}
	blocks.SetSolid(blocked, true)

	from := skill.Vec3{X: 0.2, Y: 2.62, Z: 0.2}
	to := skill.Vec3{X: 2.8, Y: 1.5, Z: 0.2}
	if !raycastClear(blocks, from, to, &blocked) {
		t.Fatal("expected LOS to pass when only excluded block is on path")
	}
}

func TestDurationCheckDisabledWhenNonPositive(t *testing.T) {
	check := durationCheck(0)
	for i := 0; i < 10; i++ {
		if check() {
			t.Fatal("durationCheck should remain false when duration_ms <= 0")
		}
	}
}

func TestDurationCheckRoundsUpToTicks(t *testing.T) {
	check := durationCheck(51)
	if check() {
		t.Fatal("first tick should not expire for 51ms")
	}
	if !check() {
		t.Fatal("second tick should expire for 51ms")
	}
}
