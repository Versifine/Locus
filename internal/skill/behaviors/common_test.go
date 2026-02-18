package behaviors

import (
	"testing"

	"github.com/Versifine/locus/internal/skill"
)

type singleBlockAccess struct {
	state int32
	name  string
	solid bool
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
