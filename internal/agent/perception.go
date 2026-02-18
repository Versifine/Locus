package agent

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Versifine/locus/internal/world"
)

func FormatBlocks(blocks []BlockInfo) string {
	if len(blocks) == 0 {
		return "none"
	}

	grouped := make(map[string][][3]int)
	for _, block := range blocks {
		grouped[block.Type] = append(grouped[block.Type], block.Pos)
	}

	types := make([]string, 0, len(grouped))
	for blockType := range grouped {
		types = append(types, blockType)
	}
	sort.Strings(types)

	lines := make([]string, 0, len(types))
	for _, blockType := range types {
		positions := grouped[blockType]
		sort.Slice(positions, func(i, j int) bool {
			if positions[i][1] != positions[j][1] {
				return positions[i][1] < positions[j][1]
			}
			if positions[i][0] != positions[j][0] {
				return positions[i][0] < positions[j][0]
			}
			return positions[i][2] < positions[j][2]
		})

		if summary, ok := summarizeAsBox(positions); ok {
			lines = append(lines, fmt.Sprintf("%s: %s", blockType, summary))
			continue
		}

		coords := make([]string, 0, len(positions))
		for _, pos := range positions {
			coords = append(coords, fmt.Sprintf("[%d,%d,%d]", pos[0], pos[1], pos[2]))
		}
		lines = append(lines, fmt.Sprintf("%s: %s", blockType, strings.Join(coords, " ")))
	}

	return strings.Join(lines, "\n")
}

func FormatEntities(entities []world.Entity, players []world.Player) string {
	if len(entities) == 0 {
		return "none"
	}

	nameByUUID := make(map[string]string, len(players))
	for _, player := range players {
		nameByUUID[player.UUID] = player.Name
	}

	lines := make([]string, 0, len(entities))
	for _, entity := range entities {
		label := world.EntityTypeName(entity.Type)
		if label == "" {
			label = fmt.Sprintf("Unknown(%d)", entity.Type)
		}
		if playerName, ok := nameByUUID[entity.UUID]; ok {
			label = playerName + "(玩家)"
		}
		if entity.Type == 71 && entity.ItemName != "" {
			label = fmt.Sprintf("Item(%s)", entity.ItemName)
		}
		lines = append(lines, fmt.Sprintf("%s(id=%d): [%d,%d,%d]", label, entity.EntityID, int(math.Round(entity.X)), int(math.Round(entity.Y)), int(math.Round(entity.Z))))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func summarizeAsBox(positions [][3]int) (string, bool) {
	if len(positions) < 4 {
		return "", false
	}
	minX, maxX := positions[0][0], positions[0][0]
	minY, maxY := positions[0][1], positions[0][1]
	minZ, maxZ := positions[0][2], positions[0][2]
	set := make(map[[3]int]struct{}, len(positions))
	for _, pos := range positions {
		if pos[0] < minX {
			minX = pos[0]
		}
		if pos[0] > maxX {
			maxX = pos[0]
		}
		if pos[1] < minY {
			minY = pos[1]
		}
		if pos[1] > maxY {
			maxY = pos[1]
		}
		if pos[2] < minZ {
			minZ = pos[2]
		}
		if pos[2] > maxZ {
			maxZ = pos[2]
		}
		set[pos] = struct{}{}
	}
	volume := (maxX - minX + 1) * (maxY - minY + 1) * (maxZ - minZ + 1)
	if volume != len(set) {
		return "", false
	}
	return fmt.Sprintf("[%s,%s,%s]", rangeString(minX, maxX), rangeString(minY, maxY), rangeString(minZ, maxZ)), true
}

func rangeString(min, max int) string {
	if min == max {
		return fmt.Sprintf("%d", min)
	}
	return fmt.Sprintf("%d~%d", min, max)
}
