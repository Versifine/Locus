package physics

import "math"

type PhysicsState struct {
	Position Vec3
	Velocity Vec3
	OnGround bool
}

type InputState struct {
	Forward        bool
	Backward       bool
	Left           bool
	Right          bool
	Jump           bool
	Sneak          bool
	Sprint         bool
	Attack         bool
	Use            bool
	AttackTarget   *int32
	BreakTarget    *BlockPos
	BreakFinished  bool
	PlaceTarget    *PlaceAction
	InteractTarget *int32
	HotbarSlot     *int8
	Yaw            float32
	Pitch          float32
}

type BlockPos struct {
	X int
	Y int
	Z int
}

type PlaceAction struct {
	Pos  BlockPos
	Face int
}

func PhysicsTick(state *PhysicsState, input InputState, blockStore BlockStore) {
	PhysicsTickWithEntities(state, input, blockStore, nil)
}

type EntityCollider struct {
	X      float64
	Y      float64
	Z      float64
	Width  float64
	Height float64
}

func PhysicsTickWithEntities(state *PhysicsState, input InputState, blockStore BlockStore, entities []EntityCollider) {
	if state == nil {
		return
	}

	state.OnGround = isStandingOnSolidBlock(state.Position, blockStore)

	moveX, moveZ := desiredMoveVector(input)
	friction := HorizontalDragBase
	if state.OnGround {
		friction *= DefaultGroundSlippery
	}

	accel := AirAcceleration
	if state.OnGround {
		accel = groundAcceleration(moveSpeedMultiplier(input), friction)
	}

	state.Velocity.X += moveX * accel
	state.Velocity.Z += moveZ * accel

	if state.OnGround && input.Jump {
		state.Velocity.Y = JumpInitialVelocity
	}

	if state.OnGround && input.Sneak {
		state.Velocity.X, state.Velocity.Z = clampSneakEdgeVelocity(
			state.Position,
			state.Velocity.X,
			state.Velocity.Z,
			blockStore,
		)
	}

	state.Position, state.Velocity = ResolveMovement(state.Position, state.Velocity, blockStore)
	state.Position = ApplyEntityPush(state.Position, blockStore, entities)
	state.OnGround = isStandingOnSolidBlock(state.Position, blockStore)

	state.Velocity.Y = (state.Velocity.Y - GravityAcceleration) * VerticalDrag
	state.Velocity.X *= friction
	state.Velocity.Z *= friction
	zeroResidualVelocity(&state.Velocity)
}

func desiredMoveVector(input InputState) (float64, float64) {
	var forward float64
	if input.Forward {
		forward += 1
	}
	if input.Backward {
		forward -= 1
	}

	var strafe float64
	if input.Right {
		strafe -= 1
	}
	if input.Left {
		strafe += 1
	}

	length := math.Sqrt(forward*forward + strafe*strafe)
	if length > 1 {
		forward /= length
		strafe /= length
	}

	yawRad := float64(input.Yaw) * math.Pi / 180.0
	worldX := forward*(-math.Sin(yawRad)) + strafe*math.Cos(yawRad)
	worldZ := forward*math.Cos(yawRad) + strafe*math.Sin(yawRad)

	return worldX, worldZ
}

func moveSpeedMultiplier(input InputState) float64 {
	speed := WalkBaseSpeed
	if input.Sprint {
		speed *= SprintSpeedMultiplier
	}
	if input.Sneak {
		speed *= SneakSpeedMultiplier
	}
	if speed < 0 {
		return 0
	}
	return speed
}

func groundAcceleration(speed, friction float64) float64 {
	if friction < CollisionAxisTolerance {
		return speed
	}
	return speed * (GroundAccelerationFactor / (friction * friction * friction))
}

func clampSneakEdgeVelocity(pos Vec3, velX, velZ float64, blockStore BlockStore) (float64, float64) {
	if blockStore == nil {
		return velX, velZ
	}
	adjustAxis := func(target float64, support func(delta float64) bool) float64 {
		v := target
		for !nearlyZero(v) && !support(v) {
			if math.Abs(v) <= SneakEdgeAdjustStep {
				v = 0
				break
			}
			if v > 0 {
				v -= SneakEdgeAdjustStep
			} else {
				v += SneakEdgeAdjustStep
			}
		}
		return v
	}

	velX = adjustAxis(velX, func(delta float64) bool {
		return hasGroundSupportAt(pos.X+delta, pos.Y, pos.Z, blockStore)
	})
	velZ = adjustAxis(velZ, func(delta float64) bool {
		return hasGroundSupportAt(pos.X+velX, pos.Y, pos.Z+delta, blockStore)
	})
	return velX, velZ
}

func hasGroundSupportAt(x, y, z float64, blockStore BlockStore) bool {
	probe := PlayerAABB(x, y, z)
	probe.MinY -= SneakEdgeProbeDistance
	probe.MaxY -= SneakEdgeProbeDistance
	return CollidesWithBlock(probe, blockStore)
}

func zeroResidualVelocity(v *Vec3) {
	if v == nil {
		return
	}
	if math.Abs(v.X) < MinimumResidualHorizontalSpeed {
		v.X = 0
	}
	if math.Abs(v.Z) < MinimumResidualHorizontalSpeed {
		v.Z = 0
	}
	if math.Abs(v.Y) < MinimumResidualVerticalSpeed {
		v.Y = 0
	}
}

func isStandingOnSolidBlock(pos Vec3, blockStore BlockStore) bool {
	if blockStore == nil {
		return false
	}
	player := PlayerAABB(pos.X, pos.Y, pos.Z)
	probe := player
	probe.MinY -= GroundProbeDistance
	probe.MaxY -= GroundProbeDistance
	return CollidesWithBlock(probe, blockStore)
}
