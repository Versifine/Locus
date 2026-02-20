package behaviors

import (
	"math"
	"strings"

	"github.com/Versifine/locus/internal/physics"
	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

const (
	defaultNearDist      = 0.9
	pathStuckTicks       = 10
	pathReplanCooldown   = 6
	lookAlignedThreshold = 3.0
	raycastStepSize      = 0.1
	raycastMaxSteps      = 64
)

func boolPtr(v bool) *bool { return &v }

func float32Ptr(v float32) *float32 { return &v }

func int32Ptr(v int32) *int32 { return &v }

func int8Ptr(v int8) *int8 { return &v }

func blockPosPtr(v skill.BlockPos) *physics.BlockPos {
	b := physics.BlockPos{X: v.X, Y: v.Y, Z: v.Z}
	return &b
}

func placeActionPtr(pos skill.BlockPos, face int) *physics.PlaceAction {
	a := physics.PlaceAction{Pos: physics.BlockPos{X: pos.X, Y: pos.Y, Z: pos.Z}, Face: face}
	return &a
}

func toBlockPos(pos world.Position) skill.BlockPos {
	return skill.BlockPos{
		X: int(math.Floor(pos.X)),
		Y: int(math.Floor(pos.Y)),
		Z: int(math.Floor(pos.Z)),
	}
}

func blockCenter(pos skill.BlockPos) skill.Vec3 {
	return skill.Vec3{X: float64(pos.X) + 0.5, Y: float64(pos.Y), Z: float64(pos.Z) + 0.5}
}

func blockTopCenter(pos skill.BlockPos) skill.Vec3 {
	return skill.Vec3{X: float64(pos.X) + 0.5, Y: float64(pos.Y) + 0.5, Z: float64(pos.Z) + 0.5}
}

func eyePos(pos world.Position) skill.Vec3 {
	return skill.Vec3{X: pos.X, Y: pos.Y + 1.62, Z: pos.Z}
}

func clickedBlockFromPlaceDest(dest skill.BlockPos, face int) skill.BlockPos {
	switch face {
	case 0: // bottom
		return skill.BlockPos{X: dest.X, Y: dest.Y + 1, Z: dest.Z}
	case 1: // top
		return skill.BlockPos{X: dest.X, Y: dest.Y - 1, Z: dest.Z}
	case 2: // north
		return skill.BlockPos{X: dest.X, Y: dest.Y, Z: dest.Z + 1}
	case 3: // south
		return skill.BlockPos{X: dest.X, Y: dest.Y, Z: dest.Z - 1}
	case 4: // west
		return skill.BlockPos{X: dest.X + 1, Y: dest.Y, Z: dest.Z}
	case 5: // east
		return skill.BlockPos{X: dest.X - 1, Y: dest.Y, Z: dest.Z}
	default:
		return dest
	}
}

func raycastClear(blocks skill.BlockAccess, from, to skill.Vec3, excludeBlock *skill.BlockPos) bool {
	_, blocked := raycastFirstSolid(blocks, from, to, excludeBlock)
	return !blocked
}

func raycastFirstSolid(blocks skill.BlockAccess, from, to skill.Vec3, excludeBlock *skill.BlockPos) (skill.BlockPos, bool) {
	if blocks == nil {
		return skill.BlockPos{}, false
	}

	dx := to.X - from.X
	dy := to.Y - from.Y
	dz := to.Z - from.Z
	dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
	if dist < 1e-6 {
		return skill.BlockPos{}, false
	}

	maxDist := raycastStepSize * float64(raycastMaxSteps)
	travel := dist
	if travel > maxDist {
		travel = maxDist
	}

	steps := int(math.Ceil(travel / raycastStepSize))
	if steps < 1 {
		steps = 1
	}

	invDist := 1.0 / dist
	dirX := dx * invDist
	dirY := dy * invDist
	dirZ := dz * invDist

	hasLast := false
	var last skill.BlockPos

	for i := 1; i <= steps; i++ {
		d := float64(i) * raycastStepSize
		if d > travel {
			d = travel
		}

		x := from.X + dirX*d
		y := from.Y + dirY*d
		z := from.Z + dirZ*d
		pos := skill.BlockPos{
			X: int(math.Floor(x)),
			Y: int(math.Floor(y)),
			Z: int(math.Floor(z)),
		}

		if hasLast && pos == last {
			continue
		}
		hasLast = true
		last = pos

		if excludeBlock != nil && pos == *excludeBlock {
			continue
		}
		if blocks.IsSolid(pos.X, pos.Y, pos.Z) {
			return pos, true
		}
	}

	return skill.BlockPos{}, false
}

func isAirAt(blocks skill.BlockAccess, pos skill.BlockPos) bool {
	if blocks == nil {
		return false
	}
	stateID, ok := blocks.GetBlockState(pos.X, pos.Y, pos.Z)
	if !ok {
		return false
	}
	if stateID == 0 {
		return true
	}
	name, ok := blocks.GetBlockNameByStateID(stateID)
	if !ok {
		return false
	}
	return isAirName(name)
}

func isAirName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "air", "cave air", "void air", "minecraft:air", "minecraft:cave_air", "minecraft:void_air", "cave_air", "void_air":
		return true
	default:
		return false
	}
}

func nearestApproach(pos skill.BlockPos, self world.Position, blocks skill.BlockAccess) (skill.BlockPos, bool) {
	if blocks == nil {
		return skill.BlockPos{}, false
	}

	candidates := make([]skill.BlockPos, 0, 20)
	offsets := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for dy := -1; dy <= 1; dy++ {
		for _, off := range offsets {
			candidate := skill.BlockPos{X: pos.X + off[0], Y: pos.Y + dy, Z: pos.Z + off[1]}
			if skill.IsWalkable(candidate, blocks) {
				candidates = append(candidates, candidate)
			}
		}
	}

	if len(candidates) == 0 {
		return skill.BlockPos{}, false
	}

	best := candidates[0]
	bestDist := sqDistancePos(self, best)
	for i := 1; i < len(candidates); i++ {
		d := sqDistancePos(self, candidates[i])
		if d < bestDist {
			best = candidates[i]
			bestDist = d
		}
	}
	return best, true
}

func sqDistancePos(self world.Position, target skill.BlockPos) float64 {
	v := blockCenter(target)
	dx := v.X - self.X
	dy := v.Y - self.Y
	dz := v.Z - self.Z
	return dx*dx + dy*dy + dz*dz
}

func absf64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func normalizeYaw(yaw float32) float32 {
	for yaw <= -180 {
		yaw += 360
	}
	for yaw > 180 {
		yaw -= 360
	}
	return yaw
}

func clampPitch(pitch float32) float32 {
	if pitch < -90 {
		return -90
	}
	if pitch > 90 {
		return 90
	}
	return pitch
}

func advanceWaypoint(path []skill.BlockPos, idx int, pos world.Position, near float64) int {
	if near <= 0 {
		near = defaultNearDist
	}
	for idx < len(path) {
		if !skill.IsNear(pos, blockCenter(path[idx]), near) {
			break
		}
		idx++
	}
	return idx
}
