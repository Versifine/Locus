package world

const (
	DimensionOverworld = "minecraft:overworld"
	DimensionNether    = "minecraft:the_nether"
	DimensionEnd       = "minecraft:the_end"
)

type DimensionBounds struct {
	MinY   int
	Height int
}

func VanillaDimensionBounds(name string) (DimensionBounds, bool) {
	switch name {
	case DimensionOverworld:
		return DimensionBounds{MinY: -64, Height: 384}, true
	case DimensionNether, DimensionEnd:
		return DimensionBounds{MinY: 0, Height: 256}, true
	default:
		return DimensionBounds{}, false
	}
}
