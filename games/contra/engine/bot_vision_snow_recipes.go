package contra

// BotSnowBackgroundVisionRecipes declares globally available Snow artwork.
// Japanese A5C5/A6BA/A77E select the visible cannon/carrier art and colors;
// D096's alternate header selects Carrier CHR page107 instead of page82.
// Independent planar-ROM pixels, palette context, visible anchors and negative
// examples are retained in docs/research/nes-contra/bot-snow-background-recipes.md.
// Families identify artwork only: identical intact pixels do not reveal HP,
// vulnerability or whether an unseen destruction sequence has already begun.
func BotSnowBackgroundVisionRecipes() []BotVisionBackgroundRecipe {
	tankCHR := [8]uint8{1, 4, 14, 15, 80, 81, 82, 83}
	carrierCHR := [8]uint8{1, 4, 14, 15, 80, 81, 107, 83}
	tankPalettes := [][8]uint8{
		{61, 62, 63, 65, 0, 1, 34, 7},
		{61, 62, 95, 65, 0, 1, 34, 7},
		{61, 62, 96, 65, 0, 1, 34, 7},
		{61, 62, 97, 65, 0, 1, 34, 7},
	}
	carrierPalettes := [][8]uint8{{61, 62, 64, 66, 0, 1, 34, 6}}
	var out []BotVisionBackgroundRecipe
	add := func(codes []uint8, ax, ay int, carrier bool) {
		chr, palettes, kind, family := tankCHR, tankPalettes, uint8(18), "snow-tank"
		if carrier {
			chr, palettes, kind, family = carrierCHR, carrierPalettes, 20, "snow-carrier"
		}
		r := BotVisionBackgroundRecipe{Level: 4, Variant: RegularMap, Alternate: carrier, Table: "update",
			Codes: append([]uint8(nil), codes...), Width: 32, Height: 32, AnchorX: ax, AnchorY: ay,
			Label: BotVisionLabel{Class: "enemy", Family: family, Player: -1, Kind: kind}}
		for _, palette := range palettes {
			// Original global backdrop flashes preserve this complete color/CHR
			// pairing. Supertile quadrant attributes remain from the source art.
			for _, backdrop := range [...]uint8{15, 32, 22, 17} {
				r.AppearanceVariants = append(r.AppearanceVariants, BotBackgroundAppearance{
					CHR: chr, PaletteIndices: palette, BackdropKnown: true, Backdrop: backdrop,
				})
			}
		}
		out = append(out, r)
	}
	// Distinct muzzle ends, not the cannon's hidden logical origin. Wheels and
	// generic hull fragments remain unknown when no complete cannon is visible.
	add([]uint8{0x0f}, 3, 12, false)
	add([]uint8{0x12}, 6, 22, false)
	add([]uint8{0x13}, 10, 27, false)
	// Each code is one full left-upper picture, not a four-part composite. The
	// anchor is its visible inner silver housing. Erased/exit/thruster art and
	// fully black disappearance are deliberately absent; Stage/Scene stay unknown.
	add([]uint8{0x09, 0x0a, 0x0b, 0x0d}, 28, 16, true)
	return out
}
