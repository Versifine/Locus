package agent

import (
	"math"
	"sort"
)

const maxRaycastTransparentPassThrough = 8

type Vec3 struct {
	X float64
	Y float64
	Z float64
}

type BlockAccess interface {
	GetBlockState(x, y, z int) (int32, bool)
	GetBlockNameByStateID(stateID int32) (string, bool)
	IsSolid(x, y, z int) bool
}

type Camera struct {
	FOV     float64
	MaxDist float64
	Width   int
	Height  int
}

type BlockInfo struct {
	Type string
	Pos  [3]int
}

func DefaultCamera() Camera {
	return Camera{
		FOV:     70,
		MaxDist: 32,
		Width:   40,
		Height:  20,
	}
}

func (c Camera) VisibleSurfaceBlocks(eyePos Vec3, yaw, pitch float64, blocks BlockAccess) []BlockInfo {
	if blocks == nil {
		return nil
	}
	if c.FOV <= 0 {
		c.FOV = 70
	}
	if c.MaxDist <= 0 {
		c.MaxDist = 32
	}
	if c.Width <= 0 {
		c.Width = 40
	}
	if c.Height <= 0 {
		c.Height = 20
	}

	verticalFOV := c.FOV * float64(c.Height) / float64(c.Width)
	seen := make(map[[3]int]BlockInfo)

	for y := 0; y < c.Height; y++ {
		for x := 0; x < c.Width; x++ {
			yawOffset := ((float64(x)+0.5)/float64(c.Width) - 0.5) * c.FOV
			pitchOffset := -((float64(y)+0.5)/float64(c.Height) - 0.5) * verticalFOV

			dir := directionFromYawPitch(yaw+yawOffset, pitch+pitchOffset)
			hit, passedThrough, ok := ddaFirstHit(eyePos, dir, c.MaxDist, blocks)
			for _, through := range passedThrough {
				seen[through.Pos] = through
			}
			if ok {
				seen[hit.Pos] = hit
			}
		}
	}

	out := make([]BlockInfo, 0, len(seen))
	for _, info := range seen {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Pos[1] != out[j].Pos[1] {
			return out[i].Pos[1] < out[j].Pos[1]
		}
		if out[i].Pos[0] != out[j].Pos[0] {
			return out[i].Pos[0] < out[j].Pos[0]
		}
		return out[i].Pos[2] < out[j].Pos[2]
	})
	return out
}

func directionFromYawPitch(yaw, pitch float64) Vec3 {
	yawRad := yaw * math.Pi / 180.0
	pitchRad := pitch * math.Pi / 180.0
	return Vec3{
		X: -math.Sin(yawRad) * math.Cos(pitchRad),
		Y: -math.Sin(pitchRad),
		Z: math.Cos(yawRad) * math.Cos(pitchRad),
	}
}

func ddaFirstHit(origin Vec3, dir Vec3, maxDist float64, blocks BlockAccess) (BlockInfo, []BlockInfo, bool) {
	if nearlyZero(dir.X) && nearlyZero(dir.Y) && nearlyZero(dir.Z) {
		return BlockInfo{}, nil, false
	}

	x := int(math.Floor(origin.X))
	y := int(math.Floor(origin.Y))
	z := int(math.Floor(origin.Z))

	stepX, tMaxX, tDeltaX := ddaAxis(origin.X, dir.X, x)
	stepY, tMaxY, tDeltaY := ddaAxis(origin.Y, dir.Y, y)
	stepZ, tMaxZ, tDeltaZ := ddaAxis(origin.Z, dir.Z, z)

	passedThrough := make([]BlockInfo, 0, 4)
	distance := 0.0
	for distance <= maxDist {
		stateID, ok := blocks.GetBlockState(x, y, z)
		if ok {
			if !isAirState(blocks, stateID) {
				name, _ := blocks.GetBlockNameByStateID(stateID)
				if normalizeBlockName(name) == "" {
					name = "state_" + itoa(int(stateID))
				}
				info := BlockInfo{Type: name, Pos: [3]int{x, y, z}}
				if isTransparent(blocks, stateID) {
					if len(passedThrough) < maxRaycastTransparentPassThrough {
						passedThrough = append(passedThrough, info)
					}
				} else {
					return info, passedThrough, true
				}
			}
		}

		switch {
		case tMaxX <= tMaxY && tMaxX <= tMaxZ:
			x += stepX
			distance = tMaxX
			tMaxX += tDeltaX
		case tMaxY <= tMaxX && tMaxY <= tMaxZ:
			y += stepY
			distance = tMaxY
			tMaxY += tDeltaY
		default:
			z += stepZ
			distance = tMaxZ
			tMaxZ += tDeltaZ
		}
	}

	return BlockInfo{}, passedThrough, false
}

func ddaAxis(origin, dir float64, cell int) (step int, tMax float64, tDelta float64) {
	if nearlyZero(dir) {
		return 0, math.Inf(1), math.Inf(1)
	}
	if dir > 0 {
		step = 1
		tMax = (float64(cell+1) - origin) / dir
		tDelta = 1.0 / dir
		return
	}
	step = -1
	inv := -dir
	tMax = (origin - float64(cell)) / inv
	tDelta = 1.0 / inv
	return
}

func isAirState(blocks BlockAccess, stateID int32) bool {
	if stateID == 0 {
		return true
	}
	name, ok := blocks.GetBlockNameByStateID(stateID)
	if !ok {
		return false
	}
	name = normalizeBlockName(name)
	switch name {
	case "air", "cave_air", "void_air":
		return true
	default:
		return false
	}
}

func nearlyZero(v float64) bool {
	return math.Abs(v) < 1e-9
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
