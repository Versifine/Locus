package skill

import (
	"math"

	"github.com/Versifine/locus/internal/world"
)

type Vec3 struct {
	X float64
	Y float64
	Z float64
}

func CalcLookAt(self world.Position, target Vec3) (float32, float32) {
	dx := target.X - self.X
	dy := target.Y - self.Y
	dz := target.Z - self.Z

	yaw := float32(math.Atan2(-dx, dz) * 180.0 / math.Pi)
	horizontal := math.Sqrt(dx*dx + dz*dz)
	pitch := float32(-math.Atan2(dy, horizontal) * 180.0 / math.Pi)

	return normalizeYaw(yaw), clampPitch(pitch)
}

func CalcYawTo(self world.Position, target Vec3) float32 {
	dx := target.X - self.X
	dz := target.Z - self.Z
	yaw := float32(math.Atan2(-dx, dz) * 180.0 / math.Pi)
	return normalizeYaw(yaw)
}

func CalcWalkToward(self world.Position, target Vec3) (bool, float32) {
	yaw := CalcYawTo(self, target)
	return !IsNear(self, target, 0.2), yaw
}

func IsNear(self world.Position, target Vec3, dist float64) bool {
	if dist < 0 {
		return false
	}
	dx := target.X - self.X
	dy := target.Y - self.Y
	dz := target.Z - self.Z
	return dx*dx+dy*dy+dz*dz <= dist*dist
}

func FindEntity(snap world.Snapshot, entityID int32) *world.Entity {
	for i := range snap.Entities {
		if snap.Entities[i].EntityID == entityID {
			return &snap.Entities[i]
		}
	}
	return nil
}

func AngleDiff(current, target float32) float32 {
	d := normalizeYaw(target - current)
	if d > 180 {
		d -= 360
	}
	if d <= -180 {
		d += 360
	}
	return d
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
