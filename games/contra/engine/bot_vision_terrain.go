package contra

type botTerrainPatchKey struct {
	Part    uint8
	Pattern [32]uint8
}

var botTerrainHalves = [4][4]int{{0, 0, 16, 8}, {0, 8, 16, 8}, {0, 0, 8, 16}, {8, 0, 8, 16}}

func botTerrainPatch(part int, pixel func(x, y int) uint32) (botTerrainPatchKey, bool) {
	key := botTerrainPatchKey{Part: uint8(part)}
	rectangle := botTerrainHalves[part]
	var colors [4]uint32
	var counts [4]int
	count, index := 0, 0
	for y := rectangle[1]; y < rectangle[1]+rectangle[3]; y++ {
		for x := rectangle[0]; x < rectangle[0]+rectangle[2]; x++ {
			color := pixel(x, y)
			label := 0
			for label < count && colors[label] != color {
				label++
			}
			if label == count {
				if count == 4 {
					return key, false
				}
				colors[count] = color
				count++
			}
			counts[label]++
			key.Pattern[index/4] |= uint8(label) << uint(6-2*(index%4))
			index++
		}
	}
	dominant := 0
	for _, n := range counts {
		dominant = max(dominant, n)
	}
	return key, count > 1 && 128-dominant >= 8
}
func (v *BotVision) addTerrainPatches(source BotVisionTerrain) {
	for part := range botTerrainHalves {
		key, ok := botTerrainPatch(part, func(x, y int) uint32 { return uint32(source.Pattern[y*16+x]) })
		if !ok {
			continue
		}
		if old, exists := v.terrainPatches[key]; exists && old.kind != source.Kind {
			old.ambiguous = true
			v.terrainPatches[key] = old
		} else if !exists {
			v.terrainPatches[key] = botTerrainMeaning{kind: source.Kind}
		}
	}
}
func (v *BotVision) visualTerrainCell(pixels []uint8, x, y int) (TerrainKind, bool) {
	if x < 0 || y < 0 || x+16 > 256 || y+16 > 240 {
		return TerrainEmpty, false
	}
	if key, ok := botImageCell(pixels, x, y); ok {
		if meaning, found := v.terrain[key]; found && !meaning.ambiguous {
			return meaning.kind, true
		}
	}
	result, known := TerrainEmpty, false
	for part := range botTerrainHalves {
		key, ok := botTerrainPatch(part, func(xx, yy int) uint32 { return botPixelRGB(pixels, ((y+yy)*256+x+xx)*4) })
		if !ok {
			continue
		}
		meaning, found := v.terrainPatches[key]
		if !found || meaning.ambiguous {
			continue
		}
		if known && meaning.kind != result {
			return TerrainEmpty, false
		}
		result, known = meaning.kind, true
	}
	return result, known
}

// Recover only unknown cells whose current pixels are covered by a complete
// identified player. Transparent parts of its sprite remain required evidence.
// Palette labels are compared bijectively, just as in full-cell recognition.
func (v *BotVision) recoverPlayerCoveredTerrain(pixels []uint8, mask *[256 * 240 / 64]uint64, out *BotObservation) {
	if mask == nil || !out.GridKnown || (!out.Players[0].Visible && !out.Players[1].Visible) {
		return
	}
	v.recoverCoveredTerrain(pixels, mask, out)
}

// The caller supplies opaque pixels of accepted exact artwork only. Player
// and fixed-scenery masks share the same conservative remaining-pixel proof;
// no concealed pixel is assigned a color or a presumed terrain material.
func (v *BotVision) recoverCoveredTerrain(pixels []uint8, mask *[256 * 240 / 64]uint64, out *BotObservation) {
	if mask == nil || !out.GridKnown {
		return
	}
	for y := range out.Terrain {
		for x := range out.Terrain[y] {
			if out.TerrainKnown[y][x] {
				continue
			}
			px, py := x*16-int(out.ViewX), y*16-int(out.ViewY)
			if px < 0 || py < 0 || px+16 > 256 || py+16 > 240 {
				continue
			}
			covered := 0
			for yy := 0; yy < 16; yy++ {
				for xx := 0; xx < 16; xx++ {
					at := (py+yy)*256 + px + xx
					if mask[at/64]&(uint64(1)<<uint(at%64)) != 0 {
						covered++
					}
				}
			}
			if covered == 0 || covered > 128 {
				continue
			}
			var samples [256]struct {
				index int
				color uint8
			}
			var colors [4]uint32
			var counts [4]int
			count, n := 0, 0
			valid := true
			for yy := 0; yy < 16 && valid; yy++ {
				for xx := 0; xx < 16; xx++ {
					at := (py+yy)*256 + px + xx
					if mask[at/64]&(uint64(1)<<uint(at%64)) != 0 {
						continue
					}
					color := botPixelRGB(pixels, at*4)
					label := 0
					for label < count && colors[label] != color {
						label++
					}
					if label == count {
						if count == 4 {
							valid = false
							break
						}
						colors[count] = color
						count++
					}
					counts[label]++
					samples[n].index, samples[n].color = yy*16+xx, uint8(label)
					n++
				}
			}
			dominant := 0
			for _, c := range counts {
				dominant = max(dominant, c)
			}
			if !valid || count < 2 || n-dominant < 8 {
				continue
			}
			known, kind := false, TerrainEmpty
			for key, meaning := range v.terrain {
				var forward, reverse [4]uint8
				matches := true
				for _, s := range samples[:n] {
					label := (key[s.index/4] >> uint(6-2*(s.index%4))) & 3
					if forward[label] != 0 && forward[label] != s.color+1 || reverse[s.color] != 0 && reverse[s.color] != label+1 {
						matches = false
						break
					}
					forward[label], reverse[s.color] = s.color+1, label+1
				}
				if !matches {
					continue
				}
				if meaning.ambiguous || known && kind != meaning.kind {
					known = false
					break
				}
				known, kind = true, meaning.kind
			}
			if known {
				out.Terrain[y][x], out.TerrainKnown[y][x] = kind, true
			}
		}
	}
}
