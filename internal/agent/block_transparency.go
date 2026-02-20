package agent

import "strings"

var transparentBlocks map[string]struct{}

func init() {
	transparentBlocks = make(map[string]struct{}, 256)

	addTransparentBlocks(
		"glass",
		"glass_pane",
		"tinted_glass",
		"water",
	)

	for _, color := range dyeColors() {
		addTransparentBlocks(
			color+"_stained_glass",
			color+"_stained_glass_pane",
			color+"_carpet",
			color+"_banner",
			color+"_wall_banner",
			color+"_candle",
		)
	}

	addTransparentBlocks(
		"candle",
		"snow",
		"snow_layer",
	)

	addTransparentBlocks(
		"oak_leaves",
		"spruce_leaves",
		"birch_leaves",
		"jungle_leaves",
		"acacia_leaves",
		"dark_oak_leaves",
		"mangrove_leaves",
		"azalea_leaves",
		"flowering_azalea_leaves",
		"cherry_leaves",
		"pale_oak_leaves",
	)

	addTransparentBlocks(
		"grass",
		"short_grass",
		"tall_grass",
		"fern",
		"large_fern",
		"vine",
		"kelp",
		"kelp_plant",
		"seagrass",
		"tall_seagrass",
		"big_dripleaf",
		"small_dripleaf",
		"hanging_roots",
		"moss_carpet",
		"spore_blossom",
		"cave_vines",
		"cave_vines_plant",
		"weeping_vines",
		"weeping_vines_plant",
		"twisting_vines",
		"twisting_vines_plant",
	)

	addTransparentBlocks(
		"dandelion",
		"poppy",
		"blue_orchid",
		"allium",
		"azure_bluet",
		"red_tulip",
		"orange_tulip",
		"white_tulip",
		"pink_tulip",
		"oxeye_daisy",
		"cornflower",
		"lily_of_the_valley",
		"wither_rose",
		"sunflower",
		"lilac",
		"rose_bush",
		"peony",
		"torchflower",
		"pitcher_plant",
	)

	addTransparentBlocks(
		"torch",
		"wall_torch",
		"soul_torch",
		"soul_wall_torch",
		"redstone_torch",
		"redstone_wall_torch",
		"ladder",
		"chain",
		"iron_bars",
		"lever",
		"redstone_wire",
		"rail",
		"powered_rail",
		"detector_rail",
		"activator_rail",
		"flower_pot",
		"lantern",
		"soul_lantern",
		"campfire",
		"soul_campfire",
		"cobweb",
		"end_rod",
		"lightning_rod",
		"stone_button",
		"polished_blackstone_button",
		"stone_pressure_plate",
		"polished_blackstone_pressure_plate",
		"light_weighted_pressure_plate",
		"heavy_weighted_pressure_plate",
	)

	for _, wood := range woodVariants() {
		addTransparentBlocks(
			wood+"_fence",
			wood+"_fence_gate",
			wood+"_trapdoor",
			wood+"_door",
			wood+"_button",
			wood+"_pressure_plate",
			wood+"_sign",
			wood+"_wall_sign",
			wood+"_hanging_sign",
			wood+"_wall_hanging_sign",
		)
	}
}

func isTransparent(blocks BlockAccess, stateID int32) bool {
	if blocks == nil {
		return false
	}
	name, ok := blocks.GetBlockNameByStateID(stateID)
	if !ok {
		return false
	}
	_, ok = transparentBlocks[normalizeBlockName(name)]
	return ok
}

func normalizeBlockName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.TrimPrefix(normalized, "minecraft:")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func addTransparentBlocks(names ...string) {
	for _, name := range names {
		normalized := normalizeBlockName(name)
		if normalized == "" {
			continue
		}
		transparentBlocks[normalized] = struct{}{}
	}
}

func dyeColors() []string {
	return []string{
		"white",
		"orange",
		"magenta",
		"light_blue",
		"yellow",
		"lime",
		"pink",
		"gray",
		"light_gray",
		"cyan",
		"purple",
		"blue",
		"brown",
		"green",
		"red",
		"black",
	}
}

func woodVariants() []string {
	return []string{
		"oak",
		"spruce",
		"birch",
		"jungle",
		"acacia",
		"dark_oak",
		"mangrove",
		"cherry",
		"bamboo",
		"crimson",
		"warped",
		"pale_oak",
	}
}
