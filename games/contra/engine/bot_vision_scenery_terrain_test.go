package contra

// Failure inventory before scenery recovery: a complete static seam hides a
// unique bridge remainder; damaged/partial seams, unknown artwork, transparent
// holes, ambiguous materials and excessive coverage must remain unknown.
// Scenery must never supply the player-only moving-support recovery mask.
import (
	"image/png"
	"os"
	"testing"
)

func snowSceneryPNG(t *testing.T, path string) (BotVisualFrame, *BotVision) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	im, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	frame := botVisionTestFrame()
	frame.hud = BotPublicHUD{Seats: 2, Phase: "battle", Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}
	for y := 0; y < 240; y++ {
		for x := 0; x < 256; x++ {
			r, g, b, a := im.At(x, y).RGBA()
			at := (y*256 + x) * 4
			frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2], frame.pixels[at+3] = uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
		}
	}
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewBuiltinBotVision(botBuiltinTestCatalog(t), bundle)
	if err != nil {
		t.Fatal(err)
	}
	return frame, v
}

// Independent exhaustive exact-pixel audit, using only the immutable catalog
// and final PNG. No native slot, OAM, World or runtime sprite ID is supplied.
func exactSnowScenery(t *testing.T, frame BotVisualFrame, vision *BotVision) []botVisionMatch {
	t.Helper()
	colors := make([]uint32, 256*240)
	for at := range colors {
		colors[at] = botPixelRGB(frame.pixels, at*4)
	}
	var matches []botVisionMatch
	for i := range vision.sprites {
		p := &vision.sprites[i]
		if p.label.Class != "scenery" || p.label.Sprite != 196 || p.playerMask != 0 {
			continue
		}
		for y := 0; y+p.height <= 240; y++ {
			for x := 0; x+p.width <= 256; x++ {
				score, ok := botMatchVisualPattern(colors, p, x, y, false)
				if ok {
					matches = append(matches, botVisionMatch{pattern: p, left: x, top: y, x: x + p.anchorX, y: y + p.anchorY, score: score, exact: score == p.detailCount*20+len(p.points)})
				}
			}
		}
	}
	return matches
}

func TestBotSceneryTerrainRealPNG(t *testing.T) {
	for _, name := range []string{"119968", "129000"} {
		for _, damaged := range []bool{false, true} {
			mode := "exact"
			if damaged {
				mode = "damaged"
			}
			t.Run(name+"/"+mode, func(t *testing.T) {
				frame, vision := snowSceneryPNG(t, "testdata/bot-snow-scenery-"+name+".png")
				matches := exactSnowScenery(t, frame, vision)
				wantCount := 1
				if name == "119968" {
					wantCount = 2
				}
				if len(matches) != wantCount {
					t.Fatal("unexpected exact static seams", matches)
				}
				m := matches[len(matches)-1]
				if !m.exact || m.x != 183 || m.y != 176 {
					t.Fatal("fixture lacks the exact supporting seam", m)
				}
				t.Logf("exact scenery196 anchor=(%d,%d) rectangle=(%d,%d,%d,%d) opaque=%d", m.x, m.y, m.left, m.top, m.pattern.width, m.pattern.height, len(m.pattern.points))
				if damaged {
					point := m.pattern.points[0]
					at := ((m.top+point.y)*256 + m.left + point.x) * 4
					frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = 3, 17, 239
					if len(exactSnowScenery(t, frame, vision)) != wantCount-1 {
						t.Fatal("damaged seam remained exact")
					}
				}
				o, err := ObserveBotFrame(frame, vision)
				if err != nil {
					t.Fatal(err)
				}
				for _, point := range [][2]int{{168, 180}, {176, 180}} {
					x, y := (point[0]+int(o.ViewX))/16, (point[1]+int(o.ViewY))/16
					if damaged {
						if o.TerrainKnown[y][x] {
							t.Fatal("damaged seam revealed terrain", point)
						}
					} else if !o.TerrainKnown[y][x] || o.Terrain[y][x] != TerrainPlatform {
						t.Errorf("exact scenery did not recover unique visible bridge at %v: grid=%v view=(%d,%d) known=%v kind=%v", point, o.GridKnown, o.ViewX, o.ViewY, o.TerrainKnown[y][x], o.Terrain[y][x])
					}
				}
				for _, point := range [][2]int{{208, 180}, {224, 180}} {
					x, y := (point[0]+int(o.ViewX))/16, (point[1]+int(o.ViewY))/16
					if !o.TerrainKnown[y][x] || o.Terrain[y][x] != TerrainPlatform {
						t.Fatal("uncovered bridge was changed", point)
					}
				}
			})
		}
	}
}

func TestBotSceneryTerrainConservativeEvidence(t *testing.T) {
	for _, mode := range []string{"exact-no-player", "water", "damaged", "unknown-art", "wrong-code", "enemy", "player-alias", "transparent-hole", "unmasked-conflict", "ambiguous", "overcovered", "single-color", "intro", "continue", "interlude", "finished"} {
		t.Run(mode, func(t *testing.T) {
			pattern := make([]uint8, 256)
			seed := uint32(0x789abc)
			for i := range pattern {
				seed ^= seed << 13
				seed ^= seed >> 17
				seed ^= seed << 5
				pattern[i] = uint8(seed>>24) & 3
			}
			frame := botVisionTestFrame()
			frame.hud.Phase = "battle"
			for y := 7; y+16 <= 224; y += 16 {
				for x := 5; x+16 <= 256; x += 16 {
					for yy := 0; yy < 16; yy++ {
						for xx := 0; xx < 16; xx++ {
							value := pattern[yy*16+xx]
							at := ((y+yy)*256 + x + xx) * 4
							frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = value*40+11, value*20+21, value*50+31
						}
					}
				}
			}
			art := BotVisionTemplate{Width: 16, Height: 16, RGBA: make([]byte, 1024), Label: BotVisionLabel{Class: "scenery", Player: -1, Sprite: 196}}
			lo, hi := 4, 12
			if mode == "overcovered" {
				lo, hi = 2, 14
			}
			for y := lo; y < hi; y++ {
				for x := lo; x < hi; x++ {
					at := (y*16 + x) * 4
					art.RGBA[at], art.RGBA[at+1], art.RGBA[at+2], art.RGBA[at+3] = 250, uint8((x+y)%2)*240, 250, 255
				}
			}
			if mode == "transparent-hole" {
				art.RGBA[(7*16+7)*4+3] = 0
			}
			if mode == "wrong-code" {
				art.Label.Sprite = 197
			}
			if mode == "enemy" {
				art.Label.Class = "enemy"
			}
			if mode == "single-color" {
				for y := 55; y < 71; y++ {
					for x := 53; x < 69; x++ {
						at := (y*256 + x) * 4
						frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = 41, 51, 61
					}
				}
			}
			botVisionPlace(&frame, art, 53, 55)
			kind := TerrainPlatform
			if mode == "water" {
				kind = TerrainWater
			}
			knowledge := BotVisionKnowledge{Sprites: []BotVisionTemplate{art}, Terrain: []BotVisionTerrain{{Pattern: pattern, Kind: kind}}}
			if mode == "unknown-art" {
				knowledge.Sprites = nil
			}
			if mode == "player-alias" {
				other := art
				other.Label.Class, other.Label.Player = "player", 0
				knowledge.Sprites = append(knowledge.Sprites, other)
			}
			if mode == "ambiguous" {
				other := append([]byte(nil), pattern...)
				for y := 4; y < 12; y++ {
					for x := 4; x < 12; x++ {
						other[y*16+x] ^= 1
					}
				}
				knowledge.Terrain = append(knowledge.Terrain, BotVisionTerrain{Pattern: other, Kind: TerrainEmpty})
			}
			corrupt := func(x, y int) {
				at := (y*256 + x) * 4
				frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = 3, 17, 239
			}
			switch mode {
			case "damaged":
				corrupt(58, 60)
			case "transparent-hole":
				corrupt(60, 62)
			case "unmasked-conflict":
				corrupt(54, 56)
			case "intro", "continue", "interlude", "finished":
				frame.hud.Phase = mode
			}
			vision, err := NewBotVision(knowledge)
			if err != nil {
				t.Fatal(err)
			}
			o, err := ObserveBotFrame(frame, vision)
			if err != nil {
				t.Fatal(err)
			}
			if !o.GridKnown {
				t.Fatal("fixture lost visible terrain grid")
			}
			x, y := (60+int(o.ViewX))/16, (60+int(o.ViewY))/16
			if mode == "exact-no-player" || mode == "water" {
				if o.Players[0].Visible || o.Players[1].Visible {
					t.Fatal("scenery became a player")
				}
				if !o.TerrainKnown[y][x] || o.Terrain[y][x] != kind {
					t.Fatal("exact scenery did not preserve the uniquely visible material", o.TerrainKnown[y][x], o.Terrain[y][x])
				}
			} else if o.TerrainKnown[y][x] {
				t.Fatal("insufficient scenery evidence disclosed terrain", mode, o.Terrain[y][x])
			}
		})
	}
}

func TestBotSceneryTerrainCannotMaskMovingSupport(t *testing.T) {
	f := botVisionTestFrame()
	stone := supportTestArt(24, 20, BotVisionLabel{Class: "enemy", Player: -1, Kind: 16, Sprite: 72}, true)
	seam := supportTestArt(12, 28, BotVisionLabel{Class: "scenery", Player: -1, Sprite: 196}, false)
	player := supportTestArt(10, 24, BotVisionLabel{Class: "player", Player: 0, Sprite: 13}, false)
	botVisionPlace(&f, stone, 110, 100)
	botVisionPlace(&f, seam, 116, 76)
	botVisionPlace(&f, player, 40, 40)
	v, err := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{stone, seam, player}})
	if err != nil {
		t.Fatal(err)
	}
	if len(exactSnowScenery(t, f, v)) != 1 {
		t.Fatal("fixture lacks an exact scenery match")
	}
	o, err := ObserveBotFrame(f, v)
	if err != nil {
		t.Fatal(err)
	}
	if !o.Players[0].Visible {
		t.Fatal("fixture lacks a separately accepted player")
	}
	for _, enemy := range o.Enemies {
		if enemy.Kind == 16 && enemy.Sprite == 72 {
			t.Fatal("scenery leaked into player-only support mask", enemy)
		}
	}
}
