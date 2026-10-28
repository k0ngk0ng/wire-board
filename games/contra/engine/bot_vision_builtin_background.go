package contra

import "errors"

func botBuiltinBackgroundTemplates(catalog BotVisualCatalog, bundle *LevelBundle) ([]BotVisionTemplate, error) {
	recipes := BotBackgroundVisionRecipes()
	recipes = append(recipes, BotSnowBackgroundVisionRecipes()...)
	recipes = append(recipes, BotMachineBackgroundVisionRecipes()...)
	recipes = append(recipes, BotMachineNozzleVisionRecipes()...)
	recipes = append(recipes, BotAlienBackgroundVisionRecipes()...)
	return buildBotBackgroundVisionTemplates(catalog, bundle, recipes)
}

func buildBotBackgroundVisionTemplates(catalog BotVisualCatalog, bundle *LevelBundle, recipes []BotVisionBackgroundRecipe) ([]BotVisionTemplate, error) {
	rgb, e := catalog.RGB()
	if e != nil {
		return nil, e
	}
	if len(rgb) != 192 {
		return nil, errors.New("invalid public artwork RGB")
	}
	pages := map[uint8][]uint8{}
	load := func(id uint8) ([]uint8, error) {
		if p, ok := pages[id]; ok {
			return p, nil
		}
		p, e := catalog.CHRPage(id)
		if e != nil {
			return nil, e
		}
		if len(p) != 4096 {
			return nil, errors.New("invalid public artwork page")
		}
		pages[id] = append([]byte(nil), p...)
		return pages[id], nil
	}
	var templates []BotVisionTemplate
	for _, recipe := range recipes {
		res, e := bundle.Stage(recipe.Level)
		if e != nil {
			return nil, e
		}
		info, e := res.Stage(int(recipe.Level))
		if e != nil {
			return nil, e
		}
		header, preset := info.Initial, info.CHRPreset
		if recipe.Alternate {
			header = info.Alternate.Palette
			if info.Alternate.HasCHR {
				preset = info.Alternate.CHRPreset
			}
		}
		if recipe.Variant == BaseBossMap {
			header, preset = info.Alternate.Palette, info.Boss.CHRPreset
		}
		mapping, e := res.CHRPreset(preset)
		if e != nil {
			return nil, e
		}
		appearances := recipe.AppearanceVariants
		if len(appearances) == 0 {
			appearances = []BotBackgroundAppearance{{CHR: mapping, PaletteIndices: header.PaletteIndices}}
			for phase := 0; phase < 4; phase++ {
				indices := header.PaletteIndices
				indices[3] = header.CycleIndices[phase]
				if recipe.Variant == BaseBossMap {
					indices[2] = paletteBossSecond[int(recipe.Level&2)*2+phase]
				} else if recipe.Level == 0 || recipe.Level != 7 && !recipe.Alternate {
					indices[2] = paletteOrdinarySecond[phase]
				}
				appearances = append(appearances, BotBackgroundAppearance{CHR: mapping, PaletteIndices: indices})
			}
		}
		for _, code := range recipe.Codes {
			var tiles []uint8
			columns := 4
			attribute := uint8(0)
			switch recipe.Table {
			case "base":
				id := res.stages[int(recipe.Level)].TileSetID
				if recipe.Variant == BaseBossMap {
					id = info.Boss.TilesetID
				}
				source := res.tiles[id]
				if int(code) >= len(source.Supertiles) {
					return nil, errors.New("missing background base artwork")
				}
				art := source.Supertiles[code]
				tiles = append(tiles, art.Tiles[:]...)
				attribute = art.Attribute
			case "update":
				source, e := res.Graphics(int(recipe.Level), recipe.Variant)
				if e != nil {
					return nil, e
				}
				if int(code) >= len(source.Updates) {
					return nil, errors.New("missing background update artwork")
				}
				art := source.Updates[code]
				tiles = append(tiles, art.Tiles[:]...)
				attribute = art.Attribute
			case "tile-animation":
				source, e := OriginalTileAnimations(recipe.Level)
				if e != nil {
					return nil, e
				}
				if int(code) >= len(source) {
					return nil, errors.New("missing tile-animation artwork")
				}
				columns = 2
				for _, row := range source[code].Rows {
					tiles = append(tiles, row[:]...)
				}
			default:
				return nil, errors.New("unknown public background artwork table")
			}
			if recipe.CropX < 0 || recipe.CropY < 0 || recipe.Width < 1 || recipe.Height < 1 || recipe.CropX+recipe.Width > columns*8 || recipe.CropY+recipe.Height > len(tiles)/columns*8 {
				return nil, errors.New("invalid background artwork crop")
			}
			for _, appearance := range appearances {
				palette, err := res.Palette(appearance.PaletteIndices)
				if err != nil {
					return nil, err
				}
				var bg [4][]uint8
				for i := range bg {
					bg[i], err = load(appearance.CHR[i+4])
					if err != nil {
						return nil, err
					}
				}
				activeAttribute := attribute
				if appearance.AttributeKnown {
					activeAttribute = appearance.Attribute
					if recipe.CropX+recipe.Width > 32 || recipe.CropY+recipe.Height > 32 {
						return nil, errors.New("retained attribute exceeds its 32px region")
					}
				}
				backdrops := []uint8{palette[0]}
				if len(recipe.AppearanceVariants) == 0 {
					backdrops = []uint8{palette[0], 15, 32, 22, 17}
				}
				if appearance.BackdropKnown {
					backdrops = []uint8{appearance.Backdrop}
				}
				for _, backdrop := range backdrops {
					template := BotVisionTemplate{Width: recipe.Width, Height: recipe.Height, AnchorX: recipe.AnchorX - recipe.CropX, AnchorY: recipe.AnchorY - recipe.CropY, RGBA: make([]byte, recipe.Width*recipe.Height*4), Label: recipe.Label}
					for y := 0; y < recipe.Height; y++ {
						for x := 0; x < recipe.Width; x++ {
							px, py := x+recipe.CropX, y+recipe.CropY
							tile := tiles[(py/8)*columns+px/8]
							local := int(tile & 63)
							value := bg[tile>>6][((local>>3)*8+py%8)*64+(local&7)*8+px%8]
							group := recipe.TilePalette
							if recipe.Table != "tile-animation" || appearance.AttributeKnown {
								group = (activeAttribute >> uint((py/16*2+px/16)*2)) & 3
							}
							color := backdrop
							if value != 0 {
								color = palette[int(group)*4+int(value)]
							}
							at := (y*recipe.Width + x) * 4
							copy(template.RGBA[at:at+3], rgb[int(color&63)*3:int(color&63)*3+3])
							template.RGBA[at+3] = 255
						}
					}
					templates = append(templates, template)
				}
			}
		}
	}
	return templates, nil
}
