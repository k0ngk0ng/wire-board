package contra

// Before phase-search pruning: preserve translated visible grid estimates,
// clipped borders and first-in-scan tie selection. No native scroll is supplied.
import "testing"

func TestBotVisibleGridSearchTranslationAndTies(t *testing.T) {
	for _, tie := range []bool{false, true} {
		pattern := make([]byte, 256)
		seed := uint32(12345)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				seed ^= seed << 13
				seed ^= seed >> 17
				seed ^= seed << 5
				pattern[y*16+x] = uint8(seed) & 3
				if tie {
					pattern[y*16+x] = uint8((x + y) % 4)
				}
			}
		}
		vision, err := NewBotVision(BotVisionKnowledge{Terrain: []BotVisionTerrain{{Pattern: pattern, Kind: TerrainSolid}}})
		if err != nil {
			t.Fatal(err)
		}
		for _, phase := range [][2]int{{0, 0}, {15, 15}, {7, 9}, {3, 12}} {
			frame := botVisionTestFrame()
			for y := 0; y < 240; y++ {
				for x := 0; x < 256; x++ {
					value := pattern[((y-phase[1]+16)%16)*16+(x-phase[0]+16)%16]
					at := (y*256 + x) * 4
					frame.pixels[at], frame.pixels[at+1], frame.pixels[at+2] = value*40+11, value*20+21, value*50+31
				}
			}
			var observed BotObservation
			vision.observeTerrain(frame.pixels, &observed)
			wantX, wantY := (16-phase[0])%16, (16-phase[1])%16
			if tie {
				wantX, wantY = 0, 0
			}
			if !observed.GridKnown || int(observed.ViewX) != wantX || int(observed.ViewY) != wantY {
				t.Fatal("visible phase changed", tie, phase, observed.GridKnown, observed.ViewX, observed.ViewY)
			}
		}
	}
}
