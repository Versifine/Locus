package behaviors

import (
	"errors"
	"strings"

	"github.com/Versifine/locus/internal/skill"
)

const mineReachDistance = 4.5
const mineEstimatedBreakTicks = 20
const mineSoftBlockBreakTicks = 4

func Mine(target skill.BlockPos, slot *int8, durationMs int) skill.BehaviorFunc {
	return func(bctx skill.BehaviorCtx) error {
		if bctx.Blocks == nil {
			return errors.New("mine requires block access")
		}

		snap := bctx.Snapshot()
		nav := newPathNavigator(32, 1.0)
		slotSent := false
		breakingTicks := 0
		lastBreakTarget := skill.BlockPos{}
		hasLastBreakTarget := false
		timedOut := durationCheck(durationMs)

		applyBreak := func(partial *skill.PartialInput, breakPos skill.BlockPos, yaw, pitch float32) {
			partial.Yaw = float32Ptr(yaw)
			partial.Pitch = float32Ptr(pitch)
			partial.Attack = boolPtr(true)
			partial.BreakTarget = blockPosPtr(breakPos)
			if !hasLastBreakTarget || breakPos != lastBreakTarget {
				lastBreakTarget = breakPos
				hasLastBreakTarget = true
				breakingTicks = 1
			} else {
				breakingTicks++
			}
			breakTicks := mineBreakTicksForBlock(bctx.Blocks, breakPos)
			if breakingTicks >= breakTicks {
				partial.BreakFinished = boolPtr(true)
				breakingTicks = 0
			}
		}

		for {
			if isAirAt(bctx.Blocks, target) {
				return nil
			}

			inRange := skill.IsNear(snap.Position, blockCenter(target), mineReachDistance)
			blockerPos, blocked := raycastFirstSolid(bctx.Blocks, eyePos(snap.Position), blockTopCenter(target), &target)
			hasLOS := !blocked
			partial := skill.PartialInput{}
			if slot != nil && !slotSent {
				partial.HotbarSlot = int8Ptr(*slot)
				slotSent = true
			}

			if inRange && hasLOS {
				yaw, pitch := skill.CalcLookAt(snap.Position, blockTopCenter(target))
				applyBreak(&partial, target, yaw, pitch)
			} else if inRange && blocked && isMineSoftOccluder(bctx.Blocks, blockerPos) {
				yaw, pitch := skill.CalcLookAt(snap.Position, blockTopCenter(blockerPos))
				applyBreak(&partial, blockerPos, yaw, pitch)
			} else {
				breakingTicks = 0
				hasLastBreakTarget = false
				approach, ok := nearestApproach(target, snap.Position, bctx.Blocks)
				if !ok {
					return errors.New("mine approach not found")
				}

				move, _, err := nav.Tick(snap, approach, bctx.Blocks, true)
				if err != nil {
					return err
				}
				partial.Forward = move.Forward
				partial.Yaw = move.Yaw
				partial.Jump = move.Jump
				partial.Sprint = move.Sprint
			}

			next, ok := skill.Step(bctx, partial)
			if !ok {
				return nil
			}
			snap = next
			if timedOut() {
				return nil
			}
		}
	}
}

func mineBreakTicksForBlock(blocks skill.BlockAccess, pos skill.BlockPos) int {
	if blocks == nil {
		return mineEstimatedBreakTicks
	}
	stateID, ok := blocks.GetBlockState(pos.X, pos.Y, pos.Z)
	if !ok || stateID == 0 {
		return mineEstimatedBreakTicks
	}
	name, ok := blocks.GetBlockNameByStateID(stateID)
	if !ok {
		return mineEstimatedBreakTicks
	}
	if isMineSoftBlockName(name) {
		return mineSoftBlockBreakTicks
	}
	return mineEstimatedBreakTicks
}

func isMineSoftOccluder(blocks skill.BlockAccess, pos skill.BlockPos) bool {
	if blocks == nil {
		return false
	}
	stateID, ok := blocks.GetBlockState(pos.X, pos.Y, pos.Z)
	if !ok || stateID == 0 {
		return false
	}
	name, ok := blocks.GetBlockNameByStateID(stateID)
	if !ok {
		return false
	}
	return isMineSoftBlockName(name)
}

func isMineSoftBlockName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.TrimPrefix(normalized, "minecraft:")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if strings.Contains(normalized, "leaves") {
		return true
	}
	switch normalized {
	case "vine",
		"cave_vines",
		"cave_vines_plant",
		"weeping_vines",
		"weeping_vines_plant",
		"twisting_vines",
		"twisting_vines_plant",
		"grass",
		"short_grass",
		"tall_grass",
		"fern",
		"large_fern",
		"cobweb":
		return true
	default:
		return false
	}
}
