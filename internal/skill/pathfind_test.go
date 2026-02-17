package skill

import "testing"

type gridBlocks struct {
	solids map[BlockPos]bool
}

func newGridBlocks() *gridBlocks {
	return &gridBlocks{solids: make(map[BlockPos]bool)}
}

func (g *gridBlocks) setSolid(x, y, z int) {
	g.solids[BlockPos{X: x, Y: y, Z: z}] = true
}

func (g *gridBlocks) GetBlockState(x, y, z int) (int32, bool) {
	if g.IsSolid(x, y, z) {
		return 1, true
	}
	return 0, true
}

func (g *gridBlocks) GetBlockNameByStateID(stateID int32) (string, bool) {
	if stateID == 0 {
		return "air", true
	}
	return "stone", true
}

func (g *gridBlocks) IsSolid(x, y, z int) bool {
	return g.solids[BlockPos{X: x, Y: y, Z: z}]
}

func makeFlatGround(g *gridBlocks, minX, maxX, minZ, maxZ int, y int) {
	for x := minX; x <= maxX; x++ {
		for z := minZ; z <= maxZ; z++ {
			g.setSolid(x, y, z)
		}
	}
}

func TestFindPathStraightLine(t *testing.T) {
	g := newGridBlocks()
	makeFlatGround(g, -2, 8, -2, 2, 0)

	from := BlockPos{X: 0, Y: 1, Z: 0}
	to := BlockPos{X: 5, Y: 1, Z: 0}
	path := FindPath(from, to, g, 64)

	if len(path) == 0 {
		t.Fatal("expected non-empty path")
	}
	if path[0] != from {
		t.Fatalf("path start=%+v want %+v", path[0], from)
	}
	if path[len(path)-1] != to {
		t.Fatalf("path end=%+v want %+v", path[len(path)-1], to)
	}
}

func TestFindPathAroundWall(t *testing.T) {
	g := newGridBlocks()
	makeFlatGround(g, -2, 8, -4, 4, 0)

	for z := -1; z <= 1; z++ {
		g.setSolid(2, 1, z)
		g.setSolid(2, 2, z)
	}

	from := BlockPos{X: 0, Y: 1, Z: 0}
	to := BlockPos{X: 4, Y: 1, Z: 0}
	path := FindPath(from, to, g, 64)
	if len(path) == 0 {
		t.Fatal("expected path around wall")
	}

	deviated := false
	for _, step := range path {
		if step.Z != 0 {
			deviated = true
			break
		}
	}
	if !deviated {
		t.Fatal("expected path to deviate around wall")
	}
}

func TestFindPathJumpOneBlock(t *testing.T) {
	g := newGridBlocks()
	makeFlatGround(g, -2, 4, -2, 2, 0)
	g.setSolid(1, 1, 0)
	g.setSolid(2, 1, 0)

	from := BlockPos{X: 0, Y: 1, Z: 0}
	to := BlockPos{X: 2, Y: 2, Z: 0}
	path := FindPath(from, to, g, 64)
	if len(path) == 0 {
		t.Fatal("expected path with jump")
	}

	jumped := false
	for _, step := range path {
		if step.Y == 2 {
			jumped = true
			break
		}
	}
	if !jumped {
		t.Fatalf("expected jump step, path=%v", path)
	}
}

func TestFindPathDropDown(t *testing.T) {
	g := newGridBlocks()
	makeFlatGround(g, -2, 4, -2, 2, 0)
	g.setSolid(0, 2, 0)
	g.setSolid(1, 2, 0)

	from := BlockPos{X: 0, Y: 3, Z: 0}
	to := BlockPos{X: 2, Y: 1, Z: 0}
	path := FindPath(from, to, g, 64)
	if len(path) == 0 {
		t.Fatal("expected path with drop")
	}
	if path[len(path)-1] != to {
		t.Fatalf("path end=%+v want %+v", path[len(path)-1], to)
	}
}

func TestFindPathReturnsPartialWhenTargetOutsideRadius(t *testing.T) {
	g := newGridBlocks()
	makeFlatGround(g, -2, 16, -2, 2, 0)

	from := BlockPos{X: 0, Y: 1, Z: 0}
	to := BlockPos{X: 10, Y: 1, Z: 0}
	path := FindPath(from, to, g, 2)
	if len(path) == 0 {
		t.Fatal("expected partial path")
	}
	last := path[len(path)-1]
	if last == to {
		t.Fatalf("expected partial path end, got full goal %+v", last)
	}
	if last.X != 2 {
		t.Fatalf("expected partial endpoint near radius edge, got %+v", last)
	}
}
