package contra

// BotAlienBackgroundVisionRecipes describes potential visual targets, never
// whether a hidden actor is alive or currently vulnerable. Intact art retained
// during defeat must yield the same observation. Closed/common teeth, erased
// tissue and decorative arteries are not sufficient target evidence.
//
// Sources: pinned Japanese bank0/1 B50A/B522/B5AB, stage-eight original update
// patterns, maps and legal CHR/palette contexts. The independent raster matrix
// and visible-anchor review are in bot-alien-recipes.md. Level/Alternate select
// immutable source artwork while building global knowledge, never a live match.
func BotAlienBackgroundVisionRecipes() []BotVisionBackgroundRecipe {
	return []BotVisionBackgroundRecipe{
		{
			Level: 7, Variant: RegularMap, Table: "update", Codes: []uint8{0x12},
			// Remove the common forehead; anchor the visible opening between
			// the jaws. This crop is invariant across all 16 ordinary contexts.
			CropX: 0, CropY: 8, Width: 32, Height: 24, AnchorX: 8, AnchorY: 24,
			Label: BotVisionLabel{Class: "enemy", Family: "alien-guardian", Player: -1, Kind: 16},
		},
		{
			Level: 7, Variant: RegularMap, Alternate: true, Table: "update", Codes: []uint8{7, 11},
			// Both pulse shapes expose the lower-left core. Palette groups
			// 75/80 are the two legal colors; the crop is CHR-phase invariant.
			CropX: 8, CropY: 0, Width: 24, Height: 24, AnchorX: 24, AnchorY: 8,
			Label: BotVisionLabel{Class: "enemy", Family: "alien-heart", Player: -1, Kind: 22},
		},
	}
}
