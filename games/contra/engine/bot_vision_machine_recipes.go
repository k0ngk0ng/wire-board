package contra

// BotMachineBackgroundVisionRecipes is fixed public-art knowledge for the
// Energy Zone and Hangar. It takes no match, level, timer, HP or entity input.
// Sources: Japanese AB73/AFD7 animation choices, C63A/9E58/A5EE art tables,
// B343 launcher art and the complete intact map emblem 5E. Independent ROM
// pixels, original palette observations and exclusions are retained in
// docs/research/nes-contra/bot-machine-background-recipes.md.
func BotMachineBackgroundVisionRecipes() []BotVisionBackgroundRecipe {
	flameCHR := [8]uint8{1, 4, 16, 17, 84, 85, 86, 87}
	hangarCHR := [8]uint8{1, 4, 18, 19, 88, 89, 90, 91}
	flamePalettes := [][8]uint8{
		{51, 52, 4, 53, 0, 1, 34, 7},
		{51, 52, 92, 54, 0, 1, 34, 7},
		{51, 52, 4, 55, 0, 1, 34, 7},
		{51, 52, 93, 56, 0, 1, 34, 7},
	}
	clawPalettes := [][8]uint8{
		{69, 70, 4, 71, 0, 1, 34, 7},
		{69, 70, 92, 87, 0, 1, 34, 7},
		{69, 70, 93, 88, 0, 1, 34, 7},
	}
	// This is a palette-only appearance change: the Hangar keeps CHR preset6.
	bossPalettes := [][8]uint8{
		{69, 70, 89, 95, 0, 1, 34, 7},
		{69, 70, 89, 90, 0, 1, 34, 7},
		{69, 70, 89, 91, 0, 1, 34, 7},
	}
	var out []BotVisionBackgroundRecipe
	add := func(level uint8, alternate bool, table string, codes []uint8, width, height, ax, ay int, group, kind uint8, chr [8]uint8, palettes [][8]uint8) {
		family := "machine-flame"
		if level == 6 {
			switch kind {
			case 16:
				family = "machine-claw"
			case 23:
				family = "machine-mortar-launcher"
			case 22:
				family = "machine-armored-door"
			}
		}
		r := BotVisionBackgroundRecipe{Level: level, Variant: RegularMap, Alternate: alternate, Table: table,
			Codes: append([]uint8(nil), codes...), Width: width, Height: height, AnchorX: ax, AnchorY: ay, TilePalette: group,
			Label: BotVisionLabel{Class: "enemy", Family: family, Player: -1, Kind: kind}}
		for _, palette := range palettes {
			// CF6D/D04E's global flash can replace the universal backdrop;
			// it does not invent a different CHR or mix independent color rows.
			for _, backdrop := range [...]uint8{15, 32, 22, 17} {
				r.AppearanceVariants = append(r.AppearanceVariants, BotBackgroundAppearance{CHR: chr, PaletteIndices: palette,
					AttributeKnown: table == "tile-animation", Attribute: group * 0x55, BackdropKnown: true, Backdrop: backdrop})
			}
		}
		out = append(out, r)
	}
	// Fire's retreating end occupies only half of its complete 16px pattern.
	// Its anchor follows that visible end, never the emitter's hidden origin.
	add(5, false, "tile-animation", []uint8{1, 2}, 16, 16, 8, 8, 3, 18, flameCHR, flamePalettes)
	add(5, false, "tile-animation", []uint8{3}, 16, 16, 4, 8, 3, 18, flameCHR, flamePalettes)
	add(5, false, "tile-animation", []uint8{4, 5}, 16, 16, 8, 8, 3, 17, flameCHR, flamePalettes)
	add(5, false, "tile-animation", []uint8{6}, 16, 16, 12, 8, 3, 17, flameCHR, flamePalettes)
	add(5, false, "tile-animation", []uint8{7, 8}, 16, 16, 8, 8, 3, 16, flameCHR, flamePalettes)
	add(5, false, "tile-animation", []uint8{9}, 16, 16, 8, 4, 3, 16, flameCHR, flamePalettes)
	// All six tiles use retained group0. A06 adds an 8px shaft above the same
	// visible claw body; both matching views therefore share its body anchor.
	add(6, false, "tile-animation", []uint8{0, 2, 4}, 16, 24, 8, 8, 0, 16, hangarCHR, clawPalettes)
	add(6, false, "tile-animation", []uint8{6}, 16, 24, 8, 16, 0, 16, hangarCHR, clawPalettes)
	// A08 is a shared closed seam, A0B is rubble. Neither proves a launcher.
	add(6, true, "tile-animation", []uint8{9, 10}, 16, 16, 8, 8, 2, 23, hangarCHR, bossPalettes)
	// The full intact emblem is unique. U05 and exit fragments are not targets.
	add(6, true, "base", []uint8{0x5e}, 32, 32, 16, 16, 3, 22, hangarCHR, bossPalettes)
	// Carts are OAM sprites 2A/2B, shared by moving/stationary kinds; this
	// background catalog deliberately makes no assertion about those sprites.
	return out
}
