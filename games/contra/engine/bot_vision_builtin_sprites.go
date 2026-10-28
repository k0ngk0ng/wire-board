package contra

// Recipes describe fixed, publicly recognizable artwork families. The numbers
// are visual class compatibility values consumed by the controller policy, not
// a projection of a live enemy slot. All declared stage art is compiled up
// front into the same knowledge library; the running level cannot choose it.
func botBuiltinSpriteRecipes(bundle *LevelBundle) ([]BotVisualSpriteRecipe, error) {
	var out []BotVisualSpriteRecipe
	seen := map[BotVisualSpriteRecipe]bool{}
	add := func(mapping [8]uint8, palette [32]uint8, code, attr uint8, label BotVisionLabel) {
		recipe := BotVisualSpriteRecipe{Mapping: mapping, Palette: palette, Code: code, Attributes: attr, Label: label}
		if !seen[recipe] {
			seen[recipe] = true
			out = append(out, recipe)
		}
	}
	for level := 0; level < 8; level++ {
		r, e := bundle.Stage(uint8(level))
		if e != nil {
			return nil, e
		}
		info, e := r.Stage(level)
		if e != nil {
			return nil, e
		}
		type appearance struct {
			preset  int
			palette PaletteState
			scene   uint8
		}
		sets := []appearance{{info.CHRPreset, info.Initial, info.SceneType}}
		if info.Alternate.HasCHR {
			sets = append(sets, appearance{info.Alternate.CHRPreset, info.Alternate.Palette, info.SceneType})
		}
		if info.Boss.Available {
			sets = append(sets, appearance{info.Boss.CHRPreset, info.Alternate.Palette, 0x80})
		}
		for _, set := range sets {
			mapping, e := r.CHRPreset(set.preset)
			if e != nil {
				return nil, e
			}
			palette, e := r.Palette(set.palette.PaletteIndices)
			if e != nil {
				return nil, e
			}
			var characters []uint8
			if set.scene != 1 {
				for code := uint8(2); code <= 29; code++ {
					if code != 7 {
						characters = append(characters, code)
					}
				}
			}
			if set.scene == 0 {
				// animateWater emits this entry splash. Adjacent table codes
				// 112..114 are not gameplay player animations.
				characters = append(characters, 115)
			}
			if set.scene != 0 {
				characters = append(characters, 8, 9)
				for code := uint8(80); code <= 88; code++ {
					characters = append(characters, code)
				}
				characters = append(characters, 145)
			}
			for _, code := range characters {
				for actor := 0; actor < 2; actor++ {
					for _, effects := range []uint8{0, 8, 64, 72, 128, 136, 192, 200} {
						attr := effects
						if actor == 1 {
							attr |= 5
						}
						add(mapping, palette, code, attr, BotVisionLabel{Class: "player", Player: actor, Sprite: code, Appearance: effects & 0xc0, Phase: "battle"})
					}
				}
			}
			entity := func(kind uint8, codes []uint8, attrs []uint8) {
				for _, code := range codes {
					for _, attr := range attrs {
						add(mapping, palette, code, attr, BotVisionLabel{Class: "enemy", Player: -1, Kind: kind, Sprite: code, Appearance: attr & 0xc0})
					}
				}
			}
			rangeCodes := func(first, last uint8) []uint8 {
				var codes []uint8
				for n := int(first); n <= int(last); n++ {
					codes = append(codes, uint8(n))
				}
				return codes
			}
			flips := []uint8{0, 64, 128, 192}
			walking := []uint8{0, 8, 64, 72, 6, 14, 70, 78}
			entity(0, []uint8{47, 48, 49, 50, 51, 52, 78}, []uint8{5})
			entity(3, []uint8{77}, []uint8{0, 3, 7})
			for code := uint8(30); code <= 34; code++ {
				add(mapping, palette, code, 0, BotVisionLabel{Class: "friendly-projectile", Player: -1, Sprite: code})
			}
			if set.scene == 1 {
				for index, code := range indoorLaserSprites {
					attr := uint8(0)
					if index < 4 {
						attr = 64
					}
					add(mapping, palette, code, attr, BotVisionLabel{Class: "friendly-projectile", Player: -1, Sprite: code, Appearance: attr})
				}
			} else {
				for _, pose := range laserPoses {
					add(mapping, palette, pose[0], pose[1], BotVisionLabel{Class: "friendly-projectile", Player: -1, Sprite: pose[0], Appearance: pose[1]})
				}
			}
			// StepBullet's ordinary live glyphs. A source-confirmed palette
			// override is not a Cartesian product of every weapon and flip.
			entity(1, []uint8{30}, []uint8{1})
			entity(1, []uint8{33}, []uint8{2})
			if level == 4 {
				entity(1, []uint8{7}, []uint8{2})
			}
			if set.scene == 0 {
				entity(5, append([]uint8{38, 39, 40}, rangeCodes(56, 63)...), walking)
				entity(6, []uint8{41, 44, 45, 65, 66, 67, 68, 69, 70}, []uint8{0, 8, 64, 72})
				entity(12, []uint8{75, 76}, []uint8{0, 8, 64, 72})
			}
			if set.scene == 1 {
				entity(21, rangeCodes(147, 152), walking)
			}
			if set.scene == 0x80 {
				if level == 1 {
					entity(16, rangeCodes(93, 98), []uint8{0})
					entity(27, []uint8{100}, flips)
				}
				if level == 3 {
					entity(28, rangeCodes(104, 108), flips)
					entity(29, rangeCodes(109, 111), flips)
				}
			}
			switch level {
			case 2:
				entity(16, []uint8{72}, []uint8{0})
				entity(17, []uint8{73}, []uint8{0, 64})
				entity(19, []uint8{74}, flips)
				entity(21, []uint8{122, 123}, flips)
			}
		}
	}
	future, err := BotFutureSpriteVisionRecipes(bundle)
	if err != nil {
		return nil, err
	}
	out = append(out, future...)
	return out, nil
}
