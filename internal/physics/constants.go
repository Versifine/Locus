package physics

const (
	GravityAcceleration = 0.08
	VerticalDrag        = 0.98

	HorizontalDragBase             = 0.91
	DefaultGroundSlippery          = 0.6
	WalkBaseSpeed                  = 0.1
	SprintSpeedMultiplier          = 1.0 + 0.3
	SneakSpeedMultiplier           = 1.0 - 0.7
	JumpInitialVelocity            = 0.42
	AirAcceleration                = 0.02
	GroundAccelerationFactor       = 0.21600002
	GroundProbeDistance            = 0.001
	SneakEdgeProbeDistance         = 0.05
	SneakEdgeAdjustStep            = 0.05
	MinimumResidualHorizontalSpeed = 1e-4
	MinimumResidualVerticalSpeed   = 1e-4
	CollisionAxisTolerance         = 1e-9

	PlayerWidth     = 0.6
	PlayerDepth     = 0.6
	PlayerHeight    = 1.8
	PlayerHalfWidth = PlayerWidth / 2.0
	PlayerHalfDepth = PlayerDepth / 2.0
)
