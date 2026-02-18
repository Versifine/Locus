package skill

import (
	"container/heap"
	"math"
)

type BlockPos struct {
	X int
	Y int
	Z int
}

const (
	defaultMaxPathDist = 64
	maxDropHeight      = 3
)

type PathResult struct {
	Path     []BlockPos
	Complete bool
}

func FindPath(from, to BlockPos, blocks BlockAccess, maxDist int) []BlockPos {
	result := FindPathResult(from, to, blocks, maxDist)
	return result.Path
}

func FindPathResult(from, to BlockPos, blocks BlockAccess, maxDist int) PathResult {
	if blocks == nil {
		return PathResult{}
	}
	if maxDist <= 0 {
		maxDist = defaultMaxPathDist
	}

	start, ok := NormalizeWalkable(from, blocks)
	if !ok {
		return PathResult{}
	}
	goal, ok := NormalizeWalkable(to, blocks)
	if !ok {
		goal = nearestWalkable(to, start, blocks)
	}
	if !ok && goal == (BlockPos{}) {
		return PathResult{}
	}
	if start == goal {
		return PathResult{Path: []BlockPos{start}, Complete: true}
	}

	open := &nodeQueue{}
	heap.Init(open)
	heap.Push(open, node{Pos: start, G: 0, F: heuristic(start, goal)})

	cameFrom := make(map[BlockPos]BlockPos)
	gScore := map[BlockPos]int{start: 0}
	closed := make(map[BlockPos]struct{})

	best := start
	bestH := heuristic(start, goal)

	for open.Len() > 0 {
		current := heap.Pop(open).(node)
		if _, seen := closed[current.Pos]; seen {
			continue
		}
		closed[current.Pos] = struct{}{}

		if current.Pos == goal {
			return PathResult{Path: reconstructPath(cameFrom, start, goal), Complete: true}
		}

		h := heuristic(current.Pos, goal)
		if h < bestH || (h == bestH && gScore[current.Pos] < gScore[best]) {
			best = current.Pos
			bestH = h
		}

		for _, next := range neighbors(current.Pos, blocks) {
			if !withinRadius(start, next, maxDist) {
				continue
			}
			if _, seen := closed[next]; seen {
				continue
			}

			tentative := gScore[current.Pos] + moveCost(current.Pos, next)
			prev, known := gScore[next]
			if known && tentative >= prev {
				continue
			}

			cameFrom[next] = current.Pos
			gScore[next] = tentative
			heap.Push(open, node{
				Pos: next,
				G:   tentative,
				F:   tentative + heuristic(next, goal),
			})
		}
	}

	if best == start {
		return PathResult{}
	}
	return PathResult{Path: reconstructPath(cameFrom, start, best), Complete: false}
}

func NormalizeWalkable(pos BlockPos, blocks BlockAccess) (BlockPos, bool) {
	if IsWalkable(pos, blocks) {
		return pos, true
	}
	for dy := 1; dy <= 2; dy++ {
		up := BlockPos{X: pos.X, Y: pos.Y + dy, Z: pos.Z}
		if IsWalkable(up, blocks) {
			return up, true
		}
	}
	for dy := 1; dy <= maxDropHeight; dy++ {
		down := BlockPos{X: pos.X, Y: pos.Y - dy, Z: pos.Z}
		if IsWalkable(down, blocks) {
			return down, true
		}
	}
	return BlockPos{}, false
}

func IsWalkable(pos BlockPos, blocks BlockAccess) bool {
	if blocks == nil {
		return false
	}
	if blocks.IsSolid(pos.X, pos.Y, pos.Z) {
		return false
	}
	if blocks.IsSolid(pos.X, pos.Y+1, pos.Z) {
		return false
	}
	return blocks.IsSolid(pos.X, pos.Y-1, pos.Z)
}

func nearestWalkable(target, from BlockPos, blocks BlockAccess) BlockPos {
	best := BlockPos{}
	bestDist := math.MaxFloat64
	found := false

	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			for dz := -2; dz <= 2; dz++ {
				candidate := BlockPos{X: target.X + dx, Y: target.Y + dy, Z: target.Z + dz}
				if !IsWalkable(candidate, blocks) {
					continue
				}
				d := sqDist(candidate, from)
				if !found || d < bestDist {
					best = candidate
					bestDist = d
					found = true
				}
			}
		}
	}

	if !found {
		return BlockPos{}
	}
	return best
}

func neighbors(pos BlockPos, blocks BlockAccess) []BlockPos {
	dirs := [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	out := make([]BlockPos, 0, 12)

	for _, d := range dirs {
		nx := pos.X + d[0]
		nz := pos.Z + d[1]

		flat := BlockPos{X: nx, Y: pos.Y, Z: nz}
		if IsWalkable(flat, blocks) {
			out = append(out, flat)
			continue
		}

		up := BlockPos{X: nx, Y: pos.Y + 1, Z: nz}
		if IsWalkable(up, blocks) {
			out = append(out, up)
			continue
		}

		for drop := 1; drop <= maxDropHeight; drop++ {
			ny := pos.Y - drop
			down := BlockPos{X: nx, Y: ny, Z: nz}
			if !IsWalkable(down, blocks) {
				continue
			}
			if canDropTo(nx, pos.Y, nz, ny, blocks) {
				out = append(out, down)
				break
			}
		}
	}

	return out
}

func canDropTo(x, fromY, z, toY int, blocks BlockAccess) bool {
	for y := fromY; y >= toY+1; y-- {
		if blocks.IsSolid(x, y, z) {
			return false
		}
	}
	return true
}

func withinRadius(origin, pos BlockPos, maxDist int) bool {
	dx := abs(pos.X - origin.X)
	dy := abs(pos.Y - origin.Y)
	dz := abs(pos.Z - origin.Z)
	return max3(dx, dy, dz) <= maxDist
}

func reconstructPath(cameFrom map[BlockPos]BlockPos, start, goal BlockPos) []BlockPos {
	path := []BlockPos{goal}
	for cur := goal; cur != start; {
		prev, ok := cameFrom[cur]
		if !ok {
			return nil
		}
		path = append(path, prev)
		cur = prev
	}

	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func heuristic(a, b BlockPos) int {
	dx := abs(a.X - b.X)
	dy := abs(a.Y - b.Y)
	dz := abs(a.Z - b.Z)
	return dx + dy*2 + dz
}

func moveCost(a, b BlockPos) int {
	base := 10
	if b.Y > a.Y {
		return base + 8
	}
	if b.Y < a.Y {
		return base + 4
	}
	return base
}

func sqDist(a, b BlockPos) float64 {
	dx := float64(a.X - b.X)
	dy := float64(a.Y - b.Y)
	dz := float64(a.Z - b.Z)
	return dx*dx + dy*dy + dz*dz
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func max3(a, b, c int) int {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}

type node struct {
	Pos   BlockPos
	G     int
	F     int
	index int
}

type nodeQueue []node

func (q nodeQueue) Len() int { return len(q) }

func (q nodeQueue) Less(i, j int) bool {
	if q[i].F == q[j].F {
		return q[i].G < q[j].G
	}
	return q[i].F < q[j].F
}

func (q nodeQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *nodeQueue) Push(x any) {
	n := x.(node)
	n.index = len(*q)
	*q = append(*q, n)
}

func (q *nodeQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[:n-1]
	return item
}
