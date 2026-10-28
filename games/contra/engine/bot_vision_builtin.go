package contra

import "errors"

// NewBuiltinBotVision assembles fixed visual knowledge outside gameplay
// callbacks. It reads immutable artwork/interpretation catalogs, never maps,
// live collisions, slots, clocks, random values or the current level selection.
// The initial assembly supplies real terrain patterns; independently verified
// sprite/background-label recipes extend this same public construction API.
func NewBuiltinBotVision(catalog BotVisualCatalog, bundle *LevelBundle) (*BotVision, error) {
	if catalog == nil || bundle == nil {
		return nil, errors.New("missing fixed visual knowledge sources")
	}
	var knowledge BotVisionKnowledge
	pages := map[uint8][]uint8{}
	page := func(id uint8) ([]uint8, error) {
		if data, ok := pages[id]; ok {
			return data, nil
		}
		data, e := catalog.CHRPage(id)
		if e != nil {
			return nil, e
		}
		if len(data) != 4096 {
			return nil, errors.New("invalid knowledge CHR dimensions")
		}
		copyData := append([]uint8(nil), data...)
		for _, v := range copyData {
			if v > 3 {
				return nil, errors.New("invalid knowledge CHR index")
			}
		}
		pages[id] = copyData
		return copyData, nil
	}
	for level := 0; level < 8; level++ {
		resources, e := bundle.Stage(uint8(level))
		if e != nil {
			return nil, e
		}
		info, e := resources.Stage(level)
		if e != nil {
			return nil, e
		}
		source := resources.stages[level]
		type visualSet struct {
			tiles     resourceTiles
			preset    int
			limits    [3]uint8
			animate   bool
			alternate bool
		}
		sets := []visualSet{{resources.tiles[source.TileSetID], info.CHRPreset, info.Initial.CollisionLimits, level != 5 && level != 6, false}}
		if info.Alternate.HasCHR {
			sets = append(sets, visualSet{resources.tiles[source.TileSetID], info.Alternate.CHRPreset, info.Alternate.Palette.CollisionLimits, level != 2 && level != 5 && level != 6, true})
		}
		if info.Boss.Available {
			sets = append(sets, visualSet{resources.tiles[info.Boss.TilesetID], info.Boss.CHRPreset, info.Alternate.Palette.CollisionLimits, false, true})
		}
		for _, set := range sets {
			mapping, e := resources.CHRPreset(set.preset)
			if e != nil {
				return nil, e
			}
			mappings := [][8]uint8{mapping}
			if set.animate {
				row := chrAnimationRows[level]
				for _, id := range row.pages {
					variant := mapping
					variant[row.slot] = id
					mappings = append(mappings, variant)
				}
				if level == 7 {
					if !set.alternate {
						for phase := range 4 {
							mappings[phase+1][7] = [4]uint8{54, 53, 52, 95}[phase]
						}
					}
					for _, id := range chrAnimationRows[8].pages {
						variant := mapping
						variant[6] = id
						mappings = append(mappings, variant)
					}
				}
			}
			for _, mapping := range mappings {
				var bg [4][]uint8
				for i := range bg {
					bg[i], e = page(mapping[4+i])
					if e != nil {
						return nil, e
					}
				}
				for tileIndex, tile := range set.tiles.Supertiles {
					for cy := 0; cy < 2; cy++ {
						for cx := 0; cx < 2; cx++ {
							pattern := make([]uint8, 256)
							for y := 0; y < 16; y++ {
								for x := 0; x < 16; x++ {
									code := tile.Tiles[(cy*2+y/8)*4+cx*2+x/8]
									local := int(code & 63)
									pattern[y*16+x] = bg[code>>6][((local>>3)*8+y%8)*64+(local&7)*8+x%8]
								}
							}
							kind := []TerrainKind{TerrainEmpty, TerrainPlatform, TerrainWater, TerrainSolid}[collisionBits(tile.Tiles[cy*8+cx*2], set.limits)]
							// Hangar U04 is the spike-wall remnant overlay,
							// not a newly classified map tile. The original
							// destruction request preserves its platform top;
							// only the other destruction rows clear collision.
							if level == 6 && set.tiles.ID == source.TileSetID && tileIndex == set.tiles.UpdateBase+4 && cy == 0 {
								kind = TerrainPlatform
							}
							knowledge.Terrain = append(knowledge.Terrain, BotVisionTerrain{Pattern: pattern, Kind: kind})
							// Armored-door state7 also draws U04 and makes all
							// four cells Platform. Its lower pixels are the
							// same as the Empty spike remnant: preserve both
							// meanings and let global matching report unknown.
							if level == 6 && set.tiles.ID == source.TileSetID && tileIndex == set.tiles.UpdateBase+4 && cy == 1 {
								knowledge.Terrain = append(knowledge.Terrain, BotVisionTerrain{Pattern: pattern, Kind: TerrainPlatform})
							}
						}
					}
				}
			}
		}
	}
	recipes, e := botBuiltinSpriteRecipes(bundle)
	if e != nil {
		return nil, e
	}
	knowledge.Sprites, e = BuildBotVisualSpriteTemplates(catalog, recipes)
	if e != nil {
		return nil, e
	}
	background, e := botBuiltinBackgroundTemplates(catalog, bundle)
	if e != nil {
		return nil, e
	}
	knowledge.Sprites = append(knowledge.Sprites, background...)
	markers, e := botBuiltinMarkerTemplates(catalog, bundle)
	if e != nil {
		return nil, e
	}
	knowledge.Sprites = append(knowledge.Sprites, markers...)
	return NewBotVision(knowledge)
}
