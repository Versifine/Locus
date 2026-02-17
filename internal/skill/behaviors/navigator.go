package behaviors

import (
	"errors"

	"github.com/Versifine/locus/internal/skill"
	"github.com/Versifine/locus/internal/world"
)

type pathNavigator struct {
	path           []skill.BlockPos
	waypointIdx    int
	replanCooldown int
	stuckTicks     int
	lastPos        world.Position
	initialized    bool
	maxDist        int
	nearDist       float64

	hasLastPartialEnd bool
	lastPartialEnd    skill.BlockPos
	partialStallCount int
}

func newPathNavigator(maxDist int, nearDist float64) *pathNavigator {
	if maxDist <= 0 {
		maxDist = 64
	}
	if nearDist <= 0 {
		nearDist = defaultNearDist
	}
	return &pathNavigator{
		maxDist:  maxDist,
		nearDist: nearDist,
	}
}

func (n *pathNavigator) Tick(
	snap world.Snapshot,
	target skill.BlockPos,
	blocks skill.BlockAccess,
	sprint bool,
) (skill.PartialInput, bool, error) {
	if blocks == nil {
		return skill.PartialInput{}, false, errors.New("path navigator requires block access")
	}

	if !n.initialized {
		n.lastPos = snap.Position
		n.initialized = true
	}

	if skill.IsNear(snap.Position, blockCenter(target), n.nearDist) {
		return skill.PartialInput{}, true, nil
	}

	if n.replanCooldown > 0 {
		n.replanCooldown--
	}

	moved := absf64(snap.Position.X-n.lastPos.X) > 0.08 || absf64(snap.Position.Z-n.lastPos.Z) > 0.08
	if moved {
		n.stuckTicks = 0
	} else {
		n.stuckTicks++
	}
	n.lastPos = snap.Position

	needReplan := len(n.path) == 0 || n.waypointIdx >= len(n.path) || n.stuckTicks >= pathStuckTicks
	if !needReplan && n.waypointIdx < len(n.path) && !skill.IsWalkable(n.path[n.waypointIdx], blocks) {
		needReplan = true
	}

	if needReplan && n.replanCooldown == 0 {
		start := toBlockPos(snap.Position)
		result := skill.FindPathResult(start, target, blocks, n.maxDist)
		n.path = result.Path
		if len(n.path) == 0 {
			return skill.PartialInput{}, false, errors.New("path not found")
		}
		if !result.Complete {
			if n.recordPartial(n.path[len(n.path)-1], target) {
				return skill.PartialInput{}, false, errors.New("target unreachable")
			}
		} else {
			n.resetPartialTracking()
		}
		n.waypointIdx = 1
		n.stuckTicks = 0
		n.replanCooldown = pathReplanCooldown
	}

	n.waypointIdx = advanceWaypoint(n.path, n.waypointIdx, snap.Position, n.nearDist)
	if n.waypointIdx >= len(n.path) {
		if skill.IsNear(snap.Position, blockCenter(target), n.nearDist+0.4) {
			return skill.PartialInput{}, true, nil
		}
		n.path = nil
		n.waypointIdx = 0
		n.replanCooldown = 0
		return skill.PartialInput{}, false, nil
	}

	forward, yaw := skill.CalcWalkToward(snap.Position, blockCenter(n.path[n.waypointIdx]))
	partial := skill.PartialInput{Forward: boolPtr(forward), Yaw: float32Ptr(yaw)}
	if sprint {
		partial.Sprint = boolPtr(forward)
	}
	return partial, false, nil
}

func (n *pathNavigator) Invalidate() {
	if n == nil {
		return
	}
	n.path = nil
	n.waypointIdx = 0
	n.replanCooldown = 0
	n.resetPartialTracking()
}

func (n *pathNavigator) recordPartial(pathEnd, target skill.BlockPos) bool {
	if n == nil {
		return false
	}
	if !n.hasLastPartialEnd {
		n.hasLastPartialEnd = true
		n.lastPartialEnd = pathEnd
		n.partialStallCount = 0
		return false
	}

	prevEnd := n.lastPartialEnd
	prevDist := blockManhattanDistance(prevEnd, target)
	nextDist := blockManhattanDistance(pathEnd, target)
	n.lastPartialEnd = pathEnd
	n.hasLastPartialEnd = true

	if pathEnd == prevEnd || nextDist >= prevDist {
		n.partialStallCount++
	} else {
		n.partialStallCount = 0
	}

	return n.partialStallCount >= 3
}

func (n *pathNavigator) resetPartialTracking() {
	if n == nil {
		return
	}
	n.partialStallCount = 0
	n.hasLastPartialEnd = false
}

func blockManhattanDistance(a, b skill.BlockPos) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	dz := a.Z - b.Z
	if dz < 0 {
		dz = -dz
	}
	return dx + dy + dz
}
