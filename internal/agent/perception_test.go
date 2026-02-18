package agent

import (
	"strings"
	"testing"

	"github.com/Versifine/locus/internal/world"
)

func TestFormatBlocksBoxCompression(t *testing.T) {
	blocks := []BlockInfo{
		{Type: "stone", Pos: [3]int{1, 64, 1}},
		{Type: "stone", Pos: [3]int{1, 64, 2}},
		{Type: "stone", Pos: [3]int{2, 64, 1}},
		{Type: "stone", Pos: [3]int{2, 64, 2}},
	}

	formatted := FormatBlocks(blocks)
	if !strings.Contains(formatted, "1~2") {
		t.Fatalf("expected compressed range, got %q", formatted)
	}
}

func TestFormatBlocksSparse(t *testing.T) {
	blocks := []BlockInfo{
		{Type: "stone", Pos: [3]int{1, 64, 1}},
		{Type: "stone", Pos: [3]int{3, 64, 3}},
	}

	formatted := FormatBlocks(blocks)
	if !strings.Contains(formatted, "[1,64,1]") || !strings.Contains(formatted, "[3,64,3]") {
		t.Fatalf("expected explicit coordinates, got %q", formatted)
	}
}

func TestFormatEntitiesUsesPlayerName(t *testing.T) {
	entities := []world.Entity{{EntityID: 1, UUID: "u1", Type: 155, X: 10, Y: 64, Z: 20}}
	players := []world.Player{{Name: "Steve", UUID: "u1"}}
	formatted := FormatEntities(entities, players)
	if !strings.Contains(formatted, "Steve(玩家)") {
		t.Fatalf("expected player label in %q", formatted)
	}
}
