package contra

// Before anchor caching: preserve least-frequency ties, skip absent corner
// colors, keep original pixel order for equal distances, and count each physical
// anchor only once. These choices must not add false partial matches.
import "testing"

func TestBotVisualAnchorsPreserveVisibleSelection(t *testing.T) {
	const red, green, blue uint32 = 0xaa0000, 0xbb00, 0xcc
	art := BotVisionTemplate{Width: 4, Height: 4, Label: BotVisionLabel{Class: "player", Player: 0}, RGBA: make([]byte, 64)}
	for _, p := range []struct {
		x, y int
		c    uint32
	}{{0, 0, red}, {3, 0, green}, {1, 1, 0}, {0, 3, blue}, {3, 3, red}} {
		i := (p.y*4 + p.x) * 4
		art.RGBA[i], art.RGBA[i+1], art.RGBA[i+2], art.RGBA[i+3] = uint8(p.c>>16), uint8(p.c>>8), uint8(p.c), 255
	}
	vision, err := NewBotVision(BotVisionKnowledge{Sprites: []BotVisionTemplate{art}})
	if err != nil {
		t.Fatal(err)
	}
	pattern := &vision.sprites[0]
	cases := []struct {
		name    string
		counts  map[uint32]int
		partial bool
		want    [][2]int
	}{
		{"equal-frequency", map[uint32]int{red: 2, green: 2, blue: 2, 0: 5}, true, [][2]int{{0, 0}, {3, 0}, {0, 3}, {3, 3}}},
		{"missing-blue", map[uint32]int{red: 5, green: 1, 0: 9}, true, [][2]int{{0, 3}, {0, 0}, {3, 0}, {3, 3}}},
		{"exact-mode", map[uint32]int{red: 5, green: 1, 0: 9}, false, [][2]int{{0, 3}}},
		{"no-bright-colors", map[uint32]int{0: 10}, true, [][2]int{{0, 0}}},
		{"corner-tie", map[uint32]int{green: 2, blue: 2, 0: 4}, true, [][2]int{{0, 0}, {3, 0}, {0, 3}}},
		{"single-present-color", map[uint32]int{red: 2, 0: 4}, true, [][2]int{{3, 0}, {0, 0}, {3, 3}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			locations := map[uint32][]int{}
			for color, count := range c.counts {
				locations[color] = make([]int, count)
			}
			anchors, n := botVisualAnchors(pattern, locations, c.partial)
			if n != len(c.want) {
				t.Fatalf("anchors=%v n=%d want%v", anchors, n, c.want)
			}
			for i, point := range c.want {
				if anchors[i].x != point[0] || anchors[i].y != point[1] {
					t.Fatalf("anchor%d=(%d,%d), want%v", i, anchors[i].x, anchors[i].y, point)
				}
			}
		})
	}
}
