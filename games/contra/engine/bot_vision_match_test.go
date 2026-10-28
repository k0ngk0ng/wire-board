package contra

// Before optimization: preserve exact/partial scores, the 12-detail minimum,
// 40-percent coverage, two distinct bright colors, 7/11-pixel spans, the >64
// brightness boundary and candidate origins at screen edges. Expectations come
// from these public matching conditions, not a copy of the matching loop.
import (
	"fmt"
	"testing"
)

func matchFixture(t testing.TB, bright int, colors []uint32, keep func(int, int, int) bool, ox, oy int) (*botVisionPattern, []uint32) {
	t.Helper()
	const width, height = 12, 16
	template := BotVisionTemplate{Width: width, Height: height, Label: BotVisionLabel{Class: "player", Player: 0}, RGBA: make([]uint8, width*height*4)}
	pixels := make([]uint32, 256*240)
	for i := 0; i < width*height; i++ {
		template.RGBA[i*4+3] = 255
	}
	for n := 0; n < bright; n++ {
		i := n
		if bright != width*height {
			i = n * 7 % (width * height)
		}
		x, y := i%width, i/width
		c := colors[n%len(colors)]
		template.RGBA[i*4], template.RGBA[i*4+1], template.RGBA[i*4+2] = uint8(c>>16), uint8(c>>8), uint8(c)
		if keep(x, y, n) {
			pixels[(oy+y)*256+ox+x] = c
		}
	}
	vision, err := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{template}})
	if err != nil {
		t.Fatal(err)
	}
	return &vision.sprites[0], pixels
}

func TestBotMatchVisualPatternBoundaries(t *testing.T) {
	all := func(int, int, int) bool { return true }
	twelve := map[int]bool{0: true, 1: true, 4: true, 5: true, 8: true, 9: true, 12: true, 13: true, 16: true, 17: true, 24: true, 25: true}
	keepTwelve := func(_, _, n int) bool { return twelve[n] }
	two := []uint32{0x410000, 0x004100}
	cases := []struct {
		name    string
		bright  int
		colors  []uint32
		keep    func(int, int, int) bool
		partial bool
		score   int
		matched bool
	}{
		{"exact-two-colors", 192, two, all, false, 4032, true},
		{"exact-partial-mode", 192, two, all, true, 4032, true},
		{"top-four-occluded", 192, two, func(_, y, _ int) bool { return y >= 4 }, true, 3024, true},
		{"occlusion-rejected-in-exact-mode", 192, two, func(_, y, _ int) bool { return y >= 4 }, false, 0, false},
		{"insufficient-height", 192, two, func(_, y, _ int) bool { return y >= 5 }, true, 0, false},
		{"left-four-occluded", 192, two, func(x, _, _ int) bool { return x >= 4 }, true, 2688, true},
		{"insufficient-width", 192, two, func(x, _, _ int) bool { return x >= 5 }, true, 0, false},
		{"twelve-of-twenty-eight", 28, two, keepTwelve, true, 416, true},
		{"twelve-of-thirty-one", 31, two, keepTwelve, true, 0, false},
		{"eleven-details", 28, two, func(x, y, n int) bool { return keepTwelve(x, y, n) && n != 25 }, true, 0, false},
		{"single-color-partial", 28, []uint32{0x410000}, keepTwelve, true, 0, false},
		{"single-color-exact", 192, []uint32{0x410000}, all, true, 4032, true},
		{"small-detail-span-exact", 12, two, all, true, 432, true},
		{"under-minimum-exact", 11, two, all, true, 412, true},
		{"late-details-remain-possible", 28, two, func(_, _, n int) bool { return n < 2 || n >= 16 && n <= 23 || n >= 26 }, true, 416, true},
		{"dark-exact", 192, []uint32{0x400000}, all, true, 192, true},
		{"dark-partial", 192, []uint32{0x400000}, func(_, _, n int) bool { return n != 0 }, true, 0, false},
	}
	for _, c := range cases {
		for _, origin := range [][2]int{{0, 0}, {110, 80}, {244, 224}} {
			t.Run(fmt.Sprintf("%s-%d-%d", c.name, origin[0], origin[1]), func(t *testing.T) {
				pattern, pixels := matchFixture(t, c.bright, c.colors, c.keep, origin[0], origin[1])
				score, matched := botMatchVisualPattern(pixels, pattern, origin[0], origin[1], c.partial)
				if score != c.score || matched != c.matched {
					t.Fatalf("got (%d,%v), want (%d,%v)", score, matched, c.score, c.matched)
				}
				planes := newBotRGBPlanes(pixels)
				bitScore, bitMatched := botMatchBitPattern(pixels, planes, pattern.bitPattern, pattern, origin[0], origin[1], c.partial)
				if bitScore != score || bitMatched != matched {
					t.Fatal("production bit matcher changed the boundary result", bitScore, bitMatched)
				}
			})
		}
	}
}

func BenchmarkBotPatternCandidateMatch(b *testing.B) {
	pattern, pixels := matchFixture(b, 192, []uint32{0x410000, 0x004100}, func(_, y, _ int) bool { return y >= 4 }, 110, 80)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		score, matched := botMatchVisualPattern(pixels, pattern, 110, 80, true)
		if !matched || score != 3024 {
			b.Fatal("candidate result changed")
		}
	}
}
