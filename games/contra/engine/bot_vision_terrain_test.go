package contra

// Before partial terrain recognition: character feet can cover the top of the
// supporting cell. Unoccluded visual halves may identify it, but identical
// remaining pixels shared by different materials must stay unknown.
import "testing"

func TestBotVisionTerrainOccludedSupport(t *testing.T) {
	pattern := make([]uint8, 256)
	seed := uint32(0x789abc)
	for i := range pattern {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		pattern[i] = uint8(seed>>24) & 3
	}
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
	for y := 55; y < 59; y++ {
		for x := 53; x < 69; x++ {
			at := (y*256 + x) * 4
			frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = 250, uint8(x%2)*240, 250
		}
	}
	knowledge := BotVisionKnowledge{Terrain: []BotVisionTerrain{{Pattern: pattern, Kind: TerrainPlatform}}}
	vision, e := NewBotVision(knowledge)
	if e != nil {
		t.Fatal(e)
	}
	got, e := ObserveBotFrame(frame, vision)
	if e != nil {
		t.Fatal(e)
	}
	y, x := (60+int(got.ViewY))/16, (60+int(got.ViewX))/16
	if !got.TerrainKnown[y][x] || got.terrainAt(60, 60) != TerrainPlatform {
		t.Fatal("visible lower artwork did not identify support")
	}
	other := append([]byte(nil), pattern...)
	for i := 0; i < 64; i++ {
		other[i] ^= 1
	}
	knowledge.Terrain = append(knowledge.Terrain, BotVisionTerrain{Pattern: other, Kind: TerrainEmpty})
	ambiguous, e := NewBotVision(knowledge)
	if e != nil {
		t.Fatal(e)
	}
	unknown, e := ObserveBotFrame(frame, ambiguous)
	if e != nil {
		t.Fatal(e)
	}
	y, x = (60+int(unknown.ViewY))/16, (60+int(unknown.ViewX))/16
	if unknown.TerrainKnown[y][x] {
		t.Fatal("identical unoccluded pixels disclosed hidden material")
	}
}
