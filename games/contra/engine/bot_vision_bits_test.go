package contra

// The pre-implementation prototype equivalence corpus is also a normal CI gate
// for the production matcher; it must not depend on an opt-in prototype tag.
import (
	"math/rand"
	"testing"
)

func TestBotBitMatcherRandomizedEquivalence(t *testing.T) {
	random := rand.New(rand.NewSource(0x4e4553))
	palette := []uint32{0, 0x400000, 0x410000, 0x004100, 0x000041, 0xffffff, 0x332211}
	comparisons := 0
	for n := 0; n < 140; n++ {
		width, height := 3+random.Intn(126), 3+random.Intn(126)
		if n < 8 {
			width = []int{7, 8, 12, 63, 64, 65, 127, 128}[n]
			height = 16
		}
		raw := make([]byte, width*height*4)
		for i := 0; i < width*height; i++ {
			color := palette[random.Intn(len(palette))]
			raw[i*4] = byte(color >> 16)
			raw[i*4+1] = byte(color >> 8)
			raw[i*4+2] = byte(color)
			if i < 3 || random.Intn(4) != 0 {
				raw[i*4+3] = 255
			}
		}
		vision, err := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{{Width: width, Height: height, RGBA: raw, Label: BotVisionLabel{Class: "player", Player: 0}}}})
		if err != nil {
			t.Fatal(err)
		}
		pattern := &vision.sprites[0]
		ox, oy := random.Intn(257-width), random.Intn(241-height)
		pixels := make([]uint32, 256*240)
		for i := range pixels {
			pixels[i] = palette[random.Intn(len(palette))]
		}
		for _, point := range pattern.points {
			if n%4 == 0 || random.Intn(100) < n%101 {
				pixels[(oy+point.y)*256+ox+point.x] = point.color
			}
		}
		planes := newBotRGBPlanes(pixels)
		origins := [][2]int{{ox, oy}, {0, 0}, {256 - width, 240 - height}}
		for x := 0; x < 64 && x+width <= 256; x++ {
			origins = append(origins, [2]int{x, oy})
		}
		for _, origin := range origins {
			for _, partial := range []bool{false, true} {
				want, wok := botMatchVisualPattern(pixels, pattern, origin[0], origin[1], partial)
				got, gok := botMatchBitPattern(pixels, planes, pattern.bitPattern, pattern, origin[0], origin[1], partial)
				if got != want || gok != wok {
					t.Fatalf("case%d %dx%d origin%v partial%t got(%d,%t) want(%d,%t)", n, width, height, origin, partial, got, gok, want, wok)
				}
				comparisons++
			}
		}
	}
	t.Logf("equivalent candidate results: %d", comparisons)
}
