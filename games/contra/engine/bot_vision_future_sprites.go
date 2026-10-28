package contra

import "errors"

// BotVisualCartKind is one publicly recognizable vehicle body. Moving and
// initially stationary native carts have identical art; this is not kind20/21.
const BotVisualCartKind uint8 = 253

// BotFutureSpriteVisionRecipes supplies only source-confirmed late-stage art.
// Level indexes choose immutable assets at knowledge construction time. There
// is no current-stage, slot, owner, HP, timer or World argument or label filter.
// See docs/research/nes-contra/bot-future-sprites.md for the source/ambiguity audit.
func BotFutureSpriteVisionRecipes(bundle *LevelBundle) ([]BotVisualSpriteRecipe, error) {
	if bundle == nil {
		return nil, errors.New("missing fixed late-stage art resources")
	}
	var out []BotVisualSpriteRecipe
	add := func(mapping [8]uint8, palette [32]uint8, code, attr, kind uint8, class string) {
		out = append(out, BotVisualSpriteRecipe{Mapping: mapping, Palette: palette, Code: code, Attributes: attr, Label: BotVisionLabel{Class: class, Player: -1, Kind: kind, Sprite: code, Appearance: attr & 0xc0}})
	}
	for _, level := range []uint8{4, 5, 6} {
		r, e := bundle.Stage(level)
		if e != nil {
			return nil, e
		}
		info, e := r.Stage(int(level))
		if e != nil {
			return nil, e
		}
		mapping, e := r.CHRPreset(info.CHRPreset)
		if e != nil {
			return nil, e
		}
		palette, e := r.Palette(info.Initial.PaletteIndices)
		if e != nil {
			return nil, e
		}
		// F02F/F041/F052 and the actual stage5/6/7 scripts: delay attributes
		// never become sprite palette/flip attributes. Live gunner OAM attr is 0.
		for _, code := range []uint8{189, 190} {
			add(mapping, palette, code, 0, 14, "enemy")
		}
		// F085's hostile code31/attr0 is exactly the same visible pattern as
		// player M/S code31/attr0. Do not infer owner from its native dispatch.
		add(mapping, palette, 31, 0, 0, "ambiguous")
		if level == 6 {
			// F13C sets code32/attr6. The compact descriptor's attr2 OR6 differs
			// from friendly attr2 only in a non-rendered OAM bit: no allegiance cue.
			add(mapping, palette, 32, 6, 0, "ambiguous")
			for _, code := range []uint8{42, 43} {
				add(mapping, palette, code, 0, BotVisualCartKind, "enemy")
			}
		}
	}
	r, e := bundle.Stage(5)
	if e != nil {
		return nil, e
	}
	info, e := r.Stage(5)
	if e != nil {
		return nil, e
	}
	if !info.Alternate.HasCHR {
		return nil, errors.New("missing Energy Zone final art")
	}
	mapping, e := r.CHRPreset(info.Alternate.CHRPreset)
	if e != nil {
		return nil, e
	}
	base, e := r.Palette(info.Alternate.Palette.PaletteIndices)
	if e != nil {
		return nil, e
	}
	// The giant is spawned on screen11 after the original alternate CHR
	// transition. AE6D/AEAF give its disks only BB/BC, with initialized attr0.
	for _, code := range []uint8{187, 188} {
		add(mapping, base, code, 0, 20, "enemy")
	}
	// ACB8/giantPalette visibly changes only sprite palette group7. This fixed
	// knowledge recognizes those colors without returning the hidden HP value.
	for _, group := range []uint8{info.Alternate.Palette.PaletteIndices[7], 0x51, 0x52} {
		indices := info.Alternate.Palette.PaletteIndices
		indices[7] = group
		palette, e := r.Palette(indices)
		if e != nil {
			return nil, e
		}
		for _, code := range []uint8{184, 185, 186, 195} {
			for _, attr := range []uint8{0, 64} {
				add(mapping, palette, code, attr, 19, "enemy")
			}
		}
	}

	// Snow's live grenade uses background priority20 while rising and0 after
	// descending. Neither value introduces a geometric flip. Its sprite pages
	// and palette0 are unchanged by the later background resource switch.
	snow, e := bundle.Stage(4)
	if e != nil {
		return nil, e
	}
	snowInfo, e := snow.Stage(4)
	if e != nil {
		return nil, e
	}
	snowMap, e := snow.CHRPreset(snowInfo.CHRPreset)
	if e != nil {
		return nil, e
	}
	snowPalette, e := snow.Palette(snowInfo.Initial.PaletteIndices)
	if e != nil {
		return nil, e
	}
	for _, code := range []uint8{116, 117, 118, 119} {
		for _, attr := range []uint8{0, 32} {
			add(snowMap, snowPalette, code, attr, 17, "enemy")
		}
	}
	// Type19 has StateWidth81 and HP F0: neither player nor projectile contact.
	// It is a scenery seam; code196 is not the carrier's falling bomb.
	add(snowMap, snowPalette, 196, 0, 0, "scenery")
	if !snowInfo.Alternate.HasCHR {
		return nil, errors.New("missing carrier art")
	}
	snowMap, e = snow.CHRPreset(snowInfo.Alternate.CHRPreset)
	if e != nil {
		return nil, e
	}
	snowPalette, e = snow.Palette(snowInfo.Alternate.Palette.PaletteIndices)
	if e != nil {
		return nil, e
	}
	for _, code := range []uint8{124, 125} {
		add(snowMap, snowPalette, code, 0, 21, "enemy")
	}
	// Only UFO frame126 actually uses slot5. Avoid a spurious Cartesian product
	// for frames124/125, while retaining every reachable animated page for126.
	for _, page := range []uint8{81, 108, 109, 110} {
		variant := snowMap
		variant[5] = page
		add(variant, snowPalette, 126, 0, 21, "enemy")
	}
	add(snowMap, snowPalette, 34, 0, 0, "ambiguous") // Same source drawing as player F.
	// All these Lair sprites use fixed slots2/3; background animations and the
	// alternate pages6/7 do not change their visible sprite patterns or colors.
	alien, e := bundle.Stage(7)
	if e != nil {
		return nil, e
	}
	alienInfo, e := alien.Stage(7)
	if e != nil {
		return nil, e
	}
	alienMap, e := alien.CHRPreset(alienInfo.CHRPreset)
	if e != nil {
		return nil, e
	}
	alienPalette, e := alien.Palette(alienInfo.Initial.PaletteIndices)
	if e != nil {
		return nil, e
	}
	for _, code := range []uint8{172, 173, 174, 175} {
		for _, attr := range []uint8{0, 192} {
			add(alienMap, alienPalette, code, attr, 17, "enemy")
		}
	}
	for _, code := range []uint8{176, 177, 178} {
		add(alienMap, alienPalette, code, 0, 19, "enemy")
	}
	for _, code := range []uint8{179, 180, 181, 182} {
		for _, attr := range []uint8{0, 128} {
			add(alienMap, alienPalette, code, attr, 20, "enemy")
		}
	}

	return out, nil
}
