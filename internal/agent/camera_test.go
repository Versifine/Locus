package agent

import "testing"

type cameraTestBlocks struct {
	states map[[3]int]int32
	names  map[int32]string
}

func newCameraTestBlocks() *cameraTestBlocks {
	return &cameraTestBlocks{
		states: make(map[[3]int]int32),
		names: map[int32]string{
			0: "air",
			1: "stone",
			2: "gold_block",
		},
	}
}

func (b *cameraTestBlocks) set(x, y, z int, state int32) {
	b.states[[3]int{x, y, z}] = state
}

func (b *cameraTestBlocks) GetBlockState(x, y, z int) (int32, bool) {
	if state, ok := b.states[[3]int{x, y, z}]; ok {
		return state, true
	}
	return 0, true
}

func (b *cameraTestBlocks) GetBlockNameByStateID(stateID int32) (string, bool) {
	name, ok := b.names[stateID]
	return name, ok
}

func (b *cameraTestBlocks) IsSolid(x, y, z int) bool {
	state, _ := b.GetBlockState(x, y, z)
	return state != 0
}

func TestCameraVisibleSurfaceBlocksOcclusion(t *testing.T) {
	blocks := newCameraTestBlocks()
	blocks.set(0, 1, 3, 1)
	blocks.set(0, 1, 5, 2)

	camera := Camera{FOV: 70, MaxDist: 32, Width: 1, Height: 1}
	visible := camera.VisibleSurfaceBlocks(Vec3{X: 0.5, Y: 1.62, Z: 0.5}, 0, 0, blocks)

	if len(visible) != 1 {
		t.Fatalf("visible len=%d want 1", len(visible))
	}
	if visible[0].Pos != [3]int{0, 1, 3} {
		t.Fatalf("hit pos=%v want [0 1 3]", visible[0].Pos)
	}
}

func TestCameraVisibleSurfaceBlocksRespectsMaxDist(t *testing.T) {
	blocks := newCameraTestBlocks()
	blocks.set(0, 1, 20, 1)

	camera := Camera{FOV: 70, MaxDist: 8, Width: 1, Height: 1}
	visible := camera.VisibleSurfaceBlocks(Vec3{X: 0.5, Y: 1.62, Z: 0.5}, 0, 0, blocks)
	if len(visible) != 0 {
		t.Fatalf("visible len=%d want 0", len(visible))
	}
}
