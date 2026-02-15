package physics

import "math"

type PhysicsState struct {
	Position Vec3
	Velocity Vec3
	OnGround bool
}

type InputState struct {
	Forward  bool
	Backward bool
	Left     bool
	Right    bool
	Jump     bool
	Sneak    bool
	Sprint   bool
	Attack   bool
	Use      bool
	Yaw      float32
	Pitch    float32
}

func PhysicsTick(state *PhysicsState, input InputState, blockStore BlockStore) {
	if state == nil {
		return
	}

	hx, hz := desiredHorizontalVelocity(input)
	state.Velocity.X = hx
	state.Velocity.Z = hz

	state.Velocity.Y -= GravityAcceleration
	if state.OnGround && input.Jump {
		state.Velocity.Y = JumpInitialVelocity
	}

	state.Position, state.Velocity = ResolveMovement(state.Position, state.Velocity, blockStore)

	state.Velocity.Y *= VerticalDrag
	horizontalDrag := HorizontalDragBase
	if state.OnGround {
		horizontalDrag *= DefaultGroundSlippery
	}
	state.Velocity.X *= horizontalDrag
	state.Velocity.Z *= horizontalDrag

	state.OnGround = isStandingOnSolidBlock(state.Position, blockStore)
}

func desiredHorizontalVelocity(input InputState) (float64, float64) {
	var forward float64
	if input.Forward {
		forward += 1
	}
	if input.Backward {
		forward -= 1
	}

	var strafe float64
	if input.Right {
		strafe += 1
	}
	if input.Left {
		strafe -= 1
	}

	length := math.Sqrt(forward*forward + strafe*strafe)
	if length > 1 {
		forward /= length
		strafe /= length
	}

	speed := WalkBaseSpeed
	if input.Sprint {
		speed *= SprintSpeedMultiplier
	}
	if input.Sneak {
		speed *= SneakSpeedMultiplier
	}
	if speed < 0 {
		speed = 0
	}

	yawRad := float64(input.Yaw) * math.Pi / 180.0
	worldX := forward*(-math.Sin(yawRad)) + strafe*math.Cos(yawRad)
	worldZ := forward*math.Cos(yawRad) + strafe*math.Sin(yawRad)

	return worldX * speed, worldZ * speed
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
