//go:build contra_match_perf_prototype

package contra

// Prototype-only failure inventory, declared before implementation:
// - partial masks lose exact score, dark matches, transparency or duplicate RGB;
// - 12 details / 40% / two bright colors / 7x11 spans drift at equality edges;
// - a 64-bit extraction wraps into another row or clips right/bottom candidates;
// - frame-plane construction or immutable-mask preparation mutates source pixels;
// - optimized candidate generation drops any existing accepted candidate;
// - a menu screen is benchmarked as battle, or a matcher reads hidden World data.
import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"testing"
)

func TestBotMatchPerfPrototypeBoundaries(t *testing.T) {
	all := func(int, int, int) bool { return true }
	twelve := map[int]bool{0: true, 1: true, 4: true, 5: true, 8: true, 9: true, 12: true, 13: true, 16: true, 17: true, 24: true, 25: true}
	keep := func(_, _, n int) bool { return twelve[n] }
	two := []uint32{0x410000, 0x004100}
	cases := []struct {
		name    string
		bright  int
		colors  []uint32
		keep    func(int, int, int) bool
		partial bool
		score   int
		ok      bool
	}{
		{"exact-two", 192, two, all, false, 4032, true}, {"exact-partial", 192, two, all, true, 4032, true},
		{"top-four", 192, two, func(_, y, _ int) bool { return y >= 4 }, true, 3024, true},
		{"exact-occluded", 192, two, func(_, y, _ int) bool { return y >= 4 }, false, 0, false},
		{"height", 192, two, func(_, y, _ int) bool { return y >= 5 }, true, 0, false},
		{"left-four", 192, two, func(x, _, _ int) bool { return x >= 4 }, true, 2688, true},
		{"width", 192, two, func(x, _, _ int) bool { return x >= 5 }, true, 0, false},
		{"twelve-of28", 28, two, keep, true, 416, true}, {"twelve-of31", 31, two, keep, true, 0, false},
		{"eleven", 28, two, func(x, y, n int) bool { return keep(x, y, n) && n != 25 }, true, 0, false},
		{"one-color-partial", 28, []uint32{0x410000}, keep, true, 0, false},
		{"one-color-exact", 192, []uint32{0x410000}, all, true, 4032, true},
		{"small-span-exact", 12, two, all, true, 432, true}, {"under-minimum-exact", 11, two, all, true, 412, true},
		{"late-details", 28, two, func(_, _, n int) bool { return n < 2 || n >= 16 && n <= 23 || n >= 26 }, true, 416, true},
		{"dark-exact", 192, []uint32{0x400000}, all, true, 192, true},
		{"dark-partial", 192, []uint32{0x400000}, func(_, _, n int) bool { return n != 0 }, true, 0, false},
	}
	checked := 0
	for _, c := range cases {
		for _, origin := range [][2]int{{0, 0}, {110, 80}, {244, 224}} {
			t.Run(fmt.Sprintf("%s-%d-%d", c.name, origin[0], origin[1]), func(t *testing.T) {
				pattern, pixels := matchFixture(t, c.bright, c.colors, c.keep, origin[0], origin[1])
				frame := newBotPerfPlanes(pixels)
				compiled := compileBotPerfPattern(pattern)
				score, ok := botPerfMatch(pixels, frame, compiled, pattern, origin[0], origin[1], c.partial)
				if score != c.score || ok != c.ok {
					t.Fatalf("got (%d,%t), want (%d,%t)", score, ok, c.score, c.ok)
				}
				checked++
			})
		}
	}
	if checked != 51 {
		t.Fatal("boundary coverage changed", checked)
	}
}

func TestBotMatchPerfPrototypeRandomizedEquivalence(t *testing.T) {
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
		compiled := compileBotPerfPattern(pattern)
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
		frame := newBotPerfPlanes(pixels)
		origins := [][2]int{{ox, oy}, {0, 0}, {256 - width, 240 - height}}
		for x := 0; x < 64 && x+width <= 256; x++ {
			origins = append(origins, [2]int{x, oy})
		}
		for _, origin := range origins {
			for _, partial := range []bool{false, true} {
				want, wok := botMatchVisualPattern(pixels, pattern, origin[0], origin[1], partial)
				got, gok := botPerfMatch(pixels, frame, compiled, pattern, origin[0], origin[1], partial)
				if got != want || gok != wok {
					t.Fatalf("case%d %dx%d origin%v partial%t got(%d,%t) want(%d,%t)", n, width, height, origin, partial, got, gok, want, wok)
				}
				production, productionOK := botMatchBitPattern(pixels, botRGBPlanes(frame), pattern.bitPattern, pattern, origin[0], origin[1], partial)
				if production != want || productionOK != wok {
					t.Fatal("production matcher differs", n, origin, partial, production, productionOK, want, wok)
				}
				comparisons++
			}
		}
	}
	t.Logf("equivalent candidate results: %d", comparisons)
}

// This harness alone reads the standard session to render a real public frame
// and assert the legal Start reached battle. No World reaches a matcher.
func botPerfBattle(b testing.TB) ([]uint32, *BotVision, map[uint32][]int) {
	b.Helper()
	f, err := os.Open("testdata/bot-visual-catalog.json.gz")
	if err != nil {
		b.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		b.Fatal(err)
	}
	defer z.Close()
	var data struct {
		Schema    int
		RomSHA256 string
		Catalog   BotVisualCatalogManifest
		RGB       []byte
		Pages     map[uint8][]byte
	}
	if err = json.NewDecoder(z).Decode(&data); err != nil {
		b.Fatal(err)
	}
	if data.Schema != 1 || data.RomSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" {
		b.Fatal("wrong public-art source")
	}
	catalog, err := NewVerifiedBotVisualCatalog(data.Catalog, data.RGB, func(id uint8) ([]uint8, error) { return data.Pages[id], nil })
	if err != nil {
		b.Fatal(err)
	}
	bundle, err := BuiltinLevelBundle()
	if err != nil {
		b.Fatal(err)
	}
	vision, err := NewBuiltinBotVision(catalog, bundle)
	if err != nil {
		b.Fatal(err)
	}
	resources, err := bundle.Stage(0)
	if err != nil {
		b.Fatal(err)
	}
	session, err := NewSession(42, 2, "pixel-match-performance")
	if err != nil {
		b.Fatal(err)
	}
	for frame := uint64(1); frame <= 600; frame++ {
		var pads [2]Buttons
		if frame == 2 {
			pads[0] = ButtonStart
		}
		if _, err = session.StepPads(frame, pads, resources); err != nil {
			b.Fatal(err)
		}
	}
	if session.World.Active.Combat.World.LevelRoutine != 4 {
		b.Fatal("fixture never reached battle")
	}
	hud := BotPublicHUD{Seats: 2, Players: [2]BotPublicPlayer{{Occupied: true}, {Occupied: true}}}
	frame, err := NewBotVisualFrame(session.World.Active.Presentation.Snapshot(), hud, catalog, 0, 600)
	if err != nil {
		b.Fatal(err)
	}
	rgba := frame.Pixels()
	pixels := make([]uint32, 256*240)
	locations := map[uint32][]int{}
	for i := range pixels {
		pixels[i] = botPixelRGB(rgba, i*4)
		locations[pixels[i]] = append(locations[pixels[i]], i)
	}
	b.Logf("legal Start battle frame600 routine4: templates=%d colors=%d", len(vision.sprites), len(locations))
	return pixels, vision, locations
}

type botPerfCandidate struct {
	pattern, x, y int
	partial       bool
}

// Enumerate exactly the existing anchor/vote/grid/dedup candidate set. The
// prototype changes only the final scalar pixel-matching operation.
func botPerfCandidates(pixels []uint32, vision *BotVision, locations map[uint32][]int) []botPerfCandidate {
	counts := newBotPixelCounts(locations)
	tested := make([]uint32, 256*240)
	var out []botPerfCandidate
	for i := range vision.sprites {
		p := &vision.sprites[i]
		if p.label.Class == "ambiguous" && (p.playerMask == 0 || p.nonPlayerAlias) {
			continue
		}
		partial := p.playerMask != 0
		anchors, n := botVisualAnchors(p, locations, partial)
		count := n
		if partial && !p.partialPossible {
			if n < 2 {
				continue
			}
			count, partial = 1, false
		}
		for _, anchor := range anchors[:count] {
			for _, candidate := range locations[anchor.color] {
				x, y := candidate%256-anchor.x, candidate/256-anchor.y
				if x < 0 || y < 0 || x+p.width > 256 || y+p.height > 240 {
					continue
				}
				key := y*256 + x
				if tested[key] == uint32(i+1) {
					continue
				}
				tested[key] = uint32(i + 1)
				if partial {
					votes := 0
					for _, a := range anchors[:n] {
						if pixels[key+int(a.offset)] == a.color {
							votes++
						}
					}

					if votes < 2 || !counts.possible(p, x, y, true) {
						continue
					}
				}
				out = append(out, botPerfCandidate{i, x, y, partial})
			}
		}
	}
	return out
}
func BenchmarkBotMatchPerfPrototypeTemplate(b *testing.B) {
	for _, kind := range []string{"partial-accepted", "exact-accepted", "partial-rejected"} {
		b.Run(kind, func(b *testing.B) {
			keep := func(_, y, _ int) bool { return y >= 4 }
			if kind == "exact-accepted" {
				keep = func(int, int, int) bool { return true }
			}
			if kind == "partial-rejected" {
				keep = func(_, y, _ int) bool { return y >= 5 }
			}
			pattern, pixels := matchFixture(b, 192, []uint32{0x410000, 0x004100}, keep, 110, 80)
			compiled := compileBotPerfPattern(pattern)
			planes := newBotPerfPlanes(pixels)
			want, ok := botMatchVisualPattern(pixels, pattern, 110, 80, true)
			for _, mode := range []string{"original", "row-popcount"} {
				b.Run(mode, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						var score int
						var matched bool
						if mode == "original" {
							score, matched = botMatchVisualPattern(pixels, pattern, 110, 80, true)
						} else {
							score, matched = botPerfMatch(pixels, planes, compiled, pattern, 110, 80, true)
						}
						if score != want || matched != ok {
							b.Fatal("template result changed")
						}
					}
				})
			}
		})
	}
}
func BenchmarkBotMatchPerfPrototypeBattle(b *testing.B) {
	pixels, vision, locations := botPerfBattle(b)
	candidates := botPerfCandidates(pixels, vision, locations)
	compiled := make([]*botPerfPattern, len(vision.sprites))
	for i := range compiled {
		compiled[i] = compileBotPerfPattern(&vision.sprites[i])
	}
	planes := botPerfPlanesFromLocations(locations)
	accepted, partial := 0, 0
	for _, c := range candidates {
		p := &vision.sprites[c.pattern]
		want, wok := botMatchVisualPattern(pixels, p, c.x, c.y, c.partial)
		got, gok := botPerfMatch(pixels, planes, compiled[c.pattern], p, c.x, c.y, c.partial)
		if got != want || gok != wok {
			b.Fatal("battle candidate changed", c, want, wok, got, gok)
		}
		if wok {
			accepted++
		}
		if c.partial {
			partial++
		}
	}
	b.Logf("all candidates=%d partial=%d accepted=%d; every score/Exact identity equal", len(candidates), partial, accepted)
	for _, mode := range []string{"original", "row-popcount", "row-popcount-with-frame-index"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			sum := 0
			for i := 0; i < b.N; i++ {
				frame := planes
				if mode == "row-popcount-with-frame-index" {
					frame = botPerfPlanesFromLocations(locations)
				}
				for _, c := range candidates {
					p := &vision.sprites[c.pattern]
					var score int
					if mode == "original" {
						score, _ = botMatchVisualPattern(pixels, p, c.x, c.y, c.partial)
					} else {
						score, _ = botPerfMatch(pixels, frame, compiled[c.pattern], p, c.x, c.y, c.partial)
					}
					sum += score
				}
			}
			if sum == 0 {
				b.Fatal("battle workload has no accepted art")
			}
		})
	}
}

func TestProductionBitBattleCandidates(t *testing.T) {
	pixels, vision, locations := botPerfBattle(t)
	planes := botRGBPlanesFromLocations(locations)
	candidates := botPerfCandidates(pixels, vision, locations)
	accepted := 0
	for _, candidate := range candidates {
		pattern := &vision.sprites[candidate.pattern]
		want, wok := botMatchVisualPattern(pixels, pattern, candidate.x, candidate.y, candidate.partial)
		got, gok := botMatchBitPattern(pixels, planes, pattern.bitPattern, pattern, candidate.x, candidate.y, candidate.partial)
		if got != want || gok != wok {
			t.Fatal("production battle candidate changed", candidate, got, gok, want, wok)
		}
		if gok {
			accepted++
		}
	}
	t.Logf("production battle candidates equivalent: %d; accepted: %d", len(candidates), accepted)
}
