package physics

const (
	GravityAcceleration = 0.08
	VerticalDrag        = 0.98

	HorizontalDragBase     = 0.91
	DefaultGroundSlippery  = 0.6
	WalkBaseSpeed          = 0.1
	SprintSpeedMultiplier  = 1.0 + 0.3
	SneakSpeedMultiplier   = 1.0 - 0.3
	JumpInitialVelocity    = 0.42
	GroundProbeDistance    = 0.001
	CollisionAxisTolerance = 1e-9

	PlayerWidth     = 0.6
	PlayerDepth     = 0.6
	PlayerHeight    = 1.8
	PlayerHalfWidth = PlayerWidth / 2.0
	PlayerHalfDepth = PlayerDepth / 2.0
)
