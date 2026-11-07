package contra

import "testing"

// Failures declared before implementation: feet obscure all four terrain
// halves; a rectangle mask hides transparent background; partial actor matches
// erase genuine terrain; equal remaining pixels imply different materials.
// Only a complete identified body's opaque pixels may be excluded, and the
// remaining current pixels must uniquely determine the material.
func TestBotEnergyFloorBehindVisibleFeet(t *testing.T) {
	for _, tc := range []struct {
		name string
		cell int
	}{{"67938", 7}, {"67944", 8}} {
		t.Run(tc.name, func(t *testing.T) {
			o := botVisionObservePNG(t, "testdata/bot-energy-floor-"+tc.name+".png")
			if !o.Players[0].Visible {
				t.Fatal("fixture has no complete visible actor")
			}
			if !o.TerrainKnown[13][tc.cell] || o.Terrain[13][tc.cell] != TerrainPlatform {
				t.Fatal("visible floor remainder lost behind feet", o.ViewX, o.ViewY, o.TerrainKnown[13][tc.cell], o.Terrain[13][tc.cell])
			}
		})
	}
}

func TestBotMaskedTerrainRequiresIdentifiedBodyAndUniqueMaterial(t *testing.T) {
	pattern := make([]uint8, 256)
	seed := uint32(0x789abc)
	for i := range pattern {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		pattern[i] = uint8(seed>>24) & 3
	}
	for _, mode := range []string{"complete", "no-player", "ambiguous", "transparent-hole"} {
		t.Run(mode, func(t *testing.T) {
			frame := botVisionTestFrame()
			for y := 7; y+16 <= 224; y += 16 {
				for x := 5; x+16 <= 256; x += 16 {
					for yy := 0; yy < 16; yy++ {
						for xx := 0; xx < 16; xx++ {
							v := pattern[yy*16+xx]
							at := ((y+yy)*256 + x + xx) * 4
							frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = v*40+11, v*20+21, v*50+31
						}
					}
				}
			}
			art := BotVisionTemplate{Width: 16, Height: 16, RGBA: make([]byte, 1024), Label: BotVisionLabel{Class: "player", Player: 0, Sprite: 13}}
			for y := 4; y < 12; y++ {
				for x := 4; x < 12; x++ {
					at := (y*16 + x) * 4
					art.RGBA[at], art.RGBA[at+1], art.RGBA[at+2], art.RGBA[at+3] = 250, uint8(x%2)*240, 250, 255
				}
			}
			if mode == "transparent-hole" {
				art.RGBA[(7*16+7)*4+3] = 0
			}
			botVisionPlace(&frame, art, 53, 55)
			knowledge := BotVisionKnowledge{Terrain: []BotVisionTerrain{{Pattern: pattern, Kind: TerrainPlatform}}}
			if mode != "no-player" {
				knowledge.Sprites = []BotVisionTemplate{art}
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
			if mode == "transparent-hole" {
				at := ((55+7)*256 + 53 + 7) * 4
				frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = 255, 255, 255
			}
			vision, err := NewBotVision(knowledge)
			if err != nil {
				t.Fatal(err)
			}
			o, err := ObserveBotFrame(frame, vision)
			if err != nil {
				t.Fatal(err)
			}
			x, y := (60+int(o.ViewX))/16, (60+int(o.ViewY))/16
			if mode == "complete" {
				if !o.TerrainKnown[y][x] || o.Terrain[y][x] != TerrainPlatform {
					t.Fatal("complete body did not permit unique current-pixel recovery")
				}
			} else if o.TerrainKnown[y][x] {
				t.Fatal("insufficient visual evidence disclosed terrain", mode)
			}
		})
	}
}
