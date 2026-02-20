package agent

import "testing"

type transparencyTestBlocks struct {
	names map[int32]string
}

func (b transparencyTestBlocks) GetBlockState(x, y, z int) (int32, bool) {
	return 0, true
}

func (b transparencyTestBlocks) GetBlockNameByStateID(stateID int32) (string, bool) {
	name, ok := b.names[stateID]
	return name, ok
}

func (b transparencyTestBlocks) IsSolid(x, y, z int) bool {
	return false
}

func TestNormalizeBlockNameStripsNamespaceAndSpaces(t *testing.T) {
	normalized := normalizeBlockName("  minecraft:Light Blue Stained Glass Pane  ")
	if normalized != "light_blue_stained_glass_pane" {
		t.Fatalf("normalized=%q want light_blue_stained_glass_pane", normalized)
	}
}

func TestIsTransparentRecognizesConfiguredBlocks(t *testing.T) {
	blocks := transparencyTestBlocks{names: map[int32]string{
		1: "minecraft:glass",
		2: "minecraft:Light Blue Stained Glass Pane",
		3: "minecraft:water",
		4: "minecraft:stone",
	}}

	if !isTransparent(blocks, 1) {
		t.Fatal("expected glass to be transparent")
	}
	if !isTransparent(blocks, 2) {
		t.Fatal("expected stained glass pane to be transparent")
	}
	if !isTransparent(blocks, 3) {
		t.Fatal("expected water to be transparent")
	}
	if isTransparent(blocks, 4) {
		t.Fatal("stone should not be transparent")
	}
}
