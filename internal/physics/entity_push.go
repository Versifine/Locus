package physics

import "math"

const (
	entityPushMaxPerEntity = 0.08
	entityPushMaxPerTick   = 0.12
	entityPushStrength     = 0.7
)

func ApplyEntityPush(pos Vec3, blockStore BlockStore, entities []EntityCollider) Vec3 {
	if len(entities) == 0 {
		return pos
	}

	var pushX, pushZ float64
	player := PlayerAABB(pos.X, pos.Y, pos.Z)

	for _, entity := range entities {
		w := entity.Width
		h := entity.Height
		if w <= 0 {
			w = PlayerWidth
		}
		if h <= 0 {
			h = PlayerHeight
		}

		entityAABB := AABB{
			MinX: entity.X - w*0.5,
			MinY: entity.Y,
			MinZ: entity.Z - w*0.5,
			MaxX: entity.X + w*0.5,
			MaxY: entity.Y + h,
			MaxZ: entity.Z + w*0.5,
		}

		if player.MaxY <= entityAABB.MinY || player.MinY >= entityAABB.MaxY {
			continue
		}

		dx := pos.X - entity.X
		dz := pos.Z - entity.Z
		dist2 := dx*dx + dz*dz

		minDist := PlayerHalfWidth + w*0.5
		if dist2 >= minDist*minDist {
			continue
		}

		dist := math.Sqrt(dist2)
		if dist < CollisionAxisTolerance {
			dx = 1
			dz = 0
			dist = 1
		}

		overlap := minDist - dist
		if overlap <= 0 {
			continue
		}

		mag := overlap * entityPushStrength
		if mag > entityPushMaxPerEntity {
			mag = entityPushMaxPerEntity
		}
		pushX += (dx / dist) * mag
		pushZ += (dz / dist) * mag
	}

	length := math.Sqrt(pushX*pushX + pushZ*pushZ)
	if length <= CollisionAxisTolerance {
		return pos
	}
	if length > entityPushMaxPerTick {
		scale := entityPushMaxPerTick / length
		pushX *= scale
		pushZ *= scale
	}

	newPos, _ := ResolveMovement(pos, Vec3{X: pushX, Z: pushZ}, blockStore)
	return newPos
}
