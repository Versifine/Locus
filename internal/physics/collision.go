package physics

import "math"

type BlockStore interface {
	IsSolid(x, y, z int) bool
}

type Vec3 struct {
	X float64
	Y float64
	Z float64
}

type AABB struct {
	MinX float64
	MinY float64
	MinZ float64
	MaxX float64
	MaxY float64
	MaxZ float64
}

func PlayerAABB(x, y, z float64) AABB {
	return AABB{
		MinX: x - PlayerHalfWidth,
		MinY: y,
		MinZ: z - PlayerHalfDepth,
		MaxX: x + PlayerHalfWidth,
		MaxY: y + PlayerHeight,
		MaxZ: z + PlayerHalfDepth,
	}
}

func CollidesWithBlock(aabb AABB, blockStore BlockStore) bool {
	if blockStore == nil {
		return false
	}

	minX := floorForMin(aabb.MinX)
	maxX := floorForMax(aabb.MaxX)
	minY := floorForMin(aabb.MinY)
	maxY := floorForMax(aabb.MaxY)
	minZ := floorForMin(aabb.MinZ)
	maxZ := floorForMax(aabb.MaxZ)

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			for z := minZ; z <= maxZ; z++ {
				if !blockStore.IsSolid(x, y, z) {
					continue
				}
				block := AABB{
					MinX: float64(x),
					MinY: float64(y),
					MinZ: float64(z),
					MaxX: float64(x + 1),
					MaxY: float64(y + 1),
					MaxZ: float64(z + 1),
				}
				if intersects(aabb, block) {
					return true
				}
			}
		}
	}

	return false
}

func ResolveMovement(pos, velocity Vec3, blockStore BlockStore) (Vec3, Vec3) {
	newPos := pos
	newVel := velocity

	newPos.Y, newVel.Y = resolveAxisY(newPos, newVel.Y, blockStore)
	newPos.X, newVel.X = resolveAxisX(newPos, newVel.X, blockStore)
	newPos.Z, newVel.Z = resolveAxisZ(newPos, newVel.Z, blockStore)

	return newPos, newVel
}

func resolveAxisY(pos Vec3, delta float64, blockStore BlockStore) (float64, float64) {
	if blockStore == nil || nearlyZero(delta) {
		return pos.Y + delta, delta
	}

	aabb := PlayerAABB(pos.X, pos.Y, pos.Z)
	allowed := delta

	if delta > 0 {
		minX := floorForMin(aabb.MinX)
		maxX := floorForMax(aabb.MaxX)
		minZ := floorForMin(aabb.MinZ)
		maxZ := floorForMax(aabb.MaxZ)

		startY := int(math.Floor(aabb.MaxY))
		endY := int(math.Floor(aabb.MaxY + delta))
		for y := startY; y <= endY; y++ {
			for x := minX; x <= maxX; x++ {
				for z := minZ; z <= maxZ; z++ {
					if !blockStore.IsSolid(x, y, z) {
						continue
					}
					candidate := float64(y) - aabb.MaxY
					if candidate < allowed {
						allowed = candidate
					}
				}
			}
		}
	} else {
		minX := floorForMin(aabb.MinX)
		maxX := floorForMax(aabb.MaxX)
		minZ := floorForMin(aabb.MinZ)
		maxZ := floorForMax(aabb.MaxZ)

		startY := int(math.Floor(aabb.MinY + delta))
		endY := int(math.Floor(aabb.MinY - CollisionAxisTolerance))
		for y := endY; y >= startY; y-- {
			for x := minX; x <= maxX; x++ {
				for z := minZ; z <= maxZ; z++ {
					if !blockStore.IsSolid(x, y, z) {
						continue
					}
					candidate := float64(y+1) - aabb.MinY
					if candidate > allowed {
						allowed = candidate
					}
				}
			}
		}
	}

	newY := pos.Y + allowed
	if !nearlyEqual(allowed, delta) {
		return newY, 0
	}
	return newY, delta
}

func resolveAxisX(pos Vec3, delta float64, blockStore BlockStore) (float64, float64) {
	if blockStore == nil || nearlyZero(delta) {
		return pos.X + delta, delta
	}

	aabb := PlayerAABB(pos.X, pos.Y, pos.Z)
	allowed := delta

	if delta > 0 {
		minY := floorForMin(aabb.MinY)
		maxY := floorForMax(aabb.MaxY)
		minZ := floorForMin(aabb.MinZ)
		maxZ := floorForMax(aabb.MaxZ)

		startX := int(math.Floor(aabb.MaxX))
		endX := int(math.Floor(aabb.MaxX + delta))
		for x := startX; x <= endX; x++ {
			for y := minY; y <= maxY; y++ {
				for z := minZ; z <= maxZ; z++ {
					if !blockStore.IsSolid(x, y, z) {
						continue
					}
					candidate := float64(x) - aabb.MaxX
					if candidate < allowed {
						allowed = candidate
					}
				}
			}
		}
	} else {
		minY := floorForMin(aabb.MinY)
		maxY := floorForMax(aabb.MaxY)
		minZ := floorForMin(aabb.MinZ)
		maxZ := floorForMax(aabb.MaxZ)

		startX := int(math.Floor(aabb.MinX + delta))
		endX := int(math.Floor(aabb.MinX - CollisionAxisTolerance))
		for x := endX; x >= startX; x-- {
			for y := minY; y <= maxY; y++ {
				for z := minZ; z <= maxZ; z++ {
					if !blockStore.IsSolid(x, y, z) {
						continue
					}
					candidate := float64(x+1) - aabb.MinX
					if candidate > allowed {
						allowed = candidate
					}
				}
			}
		}
	}

	newX := pos.X + allowed
	if !nearlyEqual(allowed, delta) {
		return newX, 0
	}
	return newX, delta
}

func resolveAxisZ(pos Vec3, delta float64, blockStore BlockStore) (float64, float64) {
	if blockStore == nil || nearlyZero(delta) {
		return pos.Z + delta, delta
	}

	aabb := PlayerAABB(pos.X, pos.Y, pos.Z)
	allowed := delta

	if delta > 0 {
		minX := floorForMin(aabb.MinX)
		maxX := floorForMax(aabb.MaxX)
		minY := floorForMin(aabb.MinY)
		maxY := floorForMax(aabb.MaxY)

		startZ := int(math.Floor(aabb.MaxZ))
		endZ := int(math.Floor(aabb.MaxZ + delta))
		for z := startZ; z <= endZ; z++ {
			for y := minY; y <= maxY; y++ {
				for x := minX; x <= maxX; x++ {
					if !blockStore.IsSolid(x, y, z) {
						continue
					}
					candidate := float64(z) - aabb.MaxZ
					if candidate < allowed {
						allowed = candidate
					}
				}
			}
		}
	} else {
		minX := floorForMin(aabb.MinX)
		maxX := floorForMax(aabb.MaxX)
		minY := floorForMin(aabb.MinY)
		maxY := floorForMax(aabb.MaxY)

		startZ := int(math.Floor(aabb.MinZ + delta))
		endZ := int(math.Floor(aabb.MinZ - CollisionAxisTolerance))
		for z := endZ; z >= startZ; z-- {
			for y := minY; y <= maxY; y++ {
				for x := minX; x <= maxX; x++ {
					if !blockStore.IsSolid(x, y, z) {
						continue
					}
					candidate := float64(z+1) - aabb.MinZ
					if candidate > allowed {
						allowed = candidate
					}
				}
			}
		}
	}

	newZ := pos.Z + allowed
	if !nearlyEqual(allowed, delta) {
		return newZ, 0
	}
	return newZ, delta
}

func floorForMin(v float64) int {
	return int(math.Floor(v + CollisionAxisTolerance))
}

func floorForMax(v float64) int {
	return int(math.Floor(v - CollisionAxisTolerance))
}

func intersects(a, b AABB) bool {
	return a.MinX < b.MaxX &&
		a.MaxX > b.MinX &&
		a.MinY < b.MaxY &&
		a.MaxY > b.MinY &&
		a.MinZ < b.MaxZ &&
		a.MaxZ > b.MinZ
}

func nearlyZero(v float64) bool {
	return math.Abs(v) <= CollisionAxisTolerance
}

func nearlyEqual(a, b float64) bool {
	return math.Abs(a-b) <= CollisionAxisTolerance
}
