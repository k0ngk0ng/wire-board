package contra

// BotVisionBackgroundRecipe identifies immutable public artwork. Level,
// Variant and Alternate select source assets only while constructing the global
// knowledge catalog; they must never select a live match's observation branch.
// Codes are alternative visible patterns in the named source table. Coordinates
// are pixels in that pattern, and anchors describe visible features, not slots.
type BotVisionBackgroundRecipe struct {
	Level                       uint8
	Variant                     MapVariant
	Alternate                   bool
	Table                       string // "base", "update", or "tile-animation"
	Codes                       []uint8
	CropX, CropY, Width, Height int
	AnchorX, AnchorY            int
	// Tile animations use the retained background attribute. Original 0x8x
	// requests do not install animation.Attribute; Base wall art retains group 3.
	TilePalette uint8
	Label       BotVisionLabel
	// Nonempty variants replace the default header/cycle expansion. Each row
	// binds one complete, evidenced static appearance; fields are not crossed.
	AppearanceVariants []BotBackgroundAppearance
}

type BotBackgroundAppearance struct {
	CHR            [8]uint8
	PaletteIndices [8]uint8
	AttributeKnown bool
	Attribute      uint8 // retained TL/TR/BL/BR palette groups for a 32px region
	BackdropKnown  bool
	Backdrop       uint8
}

// BotBackgroundVisionRecipes returns detached, global artwork declarations.
// Source: pinned Japanese ROM 6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe,
// C593/8731 two-column definitions, EB1B supertile requests and static map art.
// ROM byte identities, original palette/anchor observations, REA references and
// review sheets are in docs/research/nes-contra/bot-background-recipes.md.
func BotBackgroundVisionRecipes() []BotVisionBackgroundRecipe {
	var out []BotVisionBackgroundRecipe
	add := func(level uint8, variant MapVariant, alternate bool, table string, codes []uint8, size, x, y int, palette, kind uint8) {
		label := BotVisionLabel{Class: "enemy", Player: -1, Kind: kind}
		if (level == 1 || level == 3) && variant == RegularMap {
			label.Family = "base-emplacement"
		}
		if level == 0 && alternate && kind == 17 {
			label.Family = "jungle-hatch"
		}
		out = append(out, BotVisionBackgroundRecipe{Level: level, Variant: variant, Alternate: alternate, Table: table, Codes: append([]uint8(nil), codes...), Width: size, Height: size, AnchorX: x, AnchorY: y, TilePalette: palette, Label: label})
	}
	for _, level := range []uint8{1, 3} {
		// 0/1/2 are the distinct armored core faces, 6/7 the open ordinary
		// core, and 10 its distinct alternate face. None reveals remaining HP.
		add(level, RegularMap, false, "tile-animation", []uint8{0, 1, 2, 6, 7, 10}, 16, 8, 8, 3, 20)
		add(level, RegularMap, false, "tile-animation", []uint8{8, 9}, 16, 8, 8, 3, 19)
		// Closed/partly opening core and turret are literally the same artwork.
		// 254 is a visual emplacement class, never an asserted hidden enemy Kind.
		add(level, RegularMap, false, "tile-animation", []uint8{4, 5}, 16, 8, 8, 3, 254)
		// Boss-room cannon and round plating, excluding erased/rubble update 5.
		add(level, BaseBossMap, false, "update", []uint8{0, 1, 2}, 32, 16, 16, 0, 8)
		add(level, BaseBossMap, false, "update", []uint8{3, 4}, 32, 16, 16, 0, 10)
	}
	// These shared outdoor guns occur in Jungle and Waterfall. Jungle's last
	// screen retains a rotating gun after switching to its alternate art bank.
	rotating := []uint8{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	for _, level := range []uint8{0, 2} {
		add(level, RegularMap, false, "update", rotating, 32, 16, 16, 0, 4)
		// 0x16/0x17 are bare scenery before/after the red gun is visible, so
		// they are not templates for a visible enemy.
		add(level, RegularMap, false, "update", []uint8{0x11, 0x12, 0x13, 0x14, 0x15, 0x18}, 32, 16, 16, 0, 7)
	}
	add(0, RegularMap, true, "update", rotating, 32, 16, 16, 0, 4)
	// Jungle cannons: anchors are the visible barrel band, not original entity
	// coordinates. Right band occupies tiles (2..3,row2); left (1..3,row2).
	add(0, RegularMap, true, "update", []uint8{0x26, 0x27}, 32, 24, 20, 0, 16)
	add(0, RegularMap, true, "update", []uint8{0x29, 0x2a}, 32, 20, 20, 0, 16)
	// The intact hatch's lower-right panel is static base art 0x35. Its lower
	// seam at the left edge is the visible target anchor. Updates 0x1e..0x25
	// depict the opened tunnel, and must not become surviving-door templates.
	add(0, RegularMap, true, "base", []uint8{0x35}, 32, 0, 16, 0, 17)
	return out
}
