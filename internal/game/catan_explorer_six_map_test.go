package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Absolute row-major positions independently read from the 2025 5–6 PDF,
// pp4–7. Counts include the two printed frame hexes and the council island.
func TestCatanExplorerSixMapsOfficialGeometryAndExploration(t *testing.T) {
	for _, scene := range []struct {
		name                                                string
		total, fog, sea, target, pasture, frameSea, council int
		parrot, goose                                       []int
	}{
		{"pirate-lairs", 79, 28, 29, 12, 34, 44, -1,
			[]int{4, 6, 11, 12, 13, 14, 19, 20, 21, 22, 23, 29, 31, 33},
			[]int{50, 52, 54, 59, 60, 61, 62, 63, 68, 69, 70, 71, 76, 78}},
		{"fish-for-catan", 88, 32, 34, 15, 38, 49, 42,
			[]int{4, 6, 7, 12, 13, 14, 15, 16, 21, 22, 23, 26, 32, 34, 35, 37},
			[]int{55, 57, 58, 60, 65, 66, 67, 70, 75, 76, 77, 78, 79, 84, 86, 87}},
		{"spices-for-catan", 88, 32, 34, 15, 38, 49, 42,
			[]int{4, 6, 7, 12, 13, 14, 15, 16, 21, 22, 23, 26, 32, 34, 35, 37},
			[]int{55, 57, 58, 60, 65, 66, 67, 70, 75, 76, 77, 78, 79, 84, 86, 87}},
		{"explorers-and-pirates", 97, 40, 35, 17, 42, 54, 46,
			[]int{4, 6, 8, 13, 14, 15, 16, 17, 18, 23, 24, 25, 26, 27, 28, 29, 35, 37, 39, 41},
			[]int{60, 62, 64, 66, 71, 72, 73, 74, 75, 76, 77, 82, 83, 84, 85, 86, 87, 92, 94, 96}},
	} {
		for _, n := range []int{5, 6} {
			t.Run(fmt.Sprintf("%s/%d", scene.name, n), func(t *testing.T) {
				for trial := 0; trial < 4; trial++ {
					g, b, err := newCatanExplorerBoard(n, scene.name, "variable")
					if err != nil {
						t.Fatal(err)
					}
					if len(g.Tiles) != scene.total || len(b.Hidden) != scene.fog || len(b.Starting) != 22 || len(b.HarborStarts) != 18 || b.Target != scene.target || b.FramePasture != scene.pasture || b.FrameSea != scene.frameSea || len(b.Opening) != 0 {
						t.Fatal("official enlarged footprint", b.publicView())
					}
					if !slices.Equal(b.Regions[0], scene.parrot) || !slices.Equal(b.Regions[1], scene.goose) {
						t.Fatal("official hidden-region positions", b.Regions)
					}
					if scene.council < 0 {
						if b.Council != nil {
							t.Fatal("unexpected council")
						}
					} else if b.Council == nil || b.Council.Tile != scene.council || !slices.Equal(b.Council.Anchors, []int{g.Tiles[scene.council].Vertices[4], g.Tiles[scene.council].Vertices[1]}) {
						t.Fatal("council and its north/south anchors")
					}
					counts := make([]int, 9)
					for _, tile := range g.Tiles {
						counts[tile.Resource]++
					}
					if !slices.Equal(counts, []int{5, 3, 6, 3, 5, 0, scene.sea, 0, scene.fog}) {
						t.Fatal("starting land/sea inventory", counts)
					}
					if g.Tiles[b.FramePasture].Resource != 2 || g.Tiles[b.FramePasture].Number != 6 || len(g.Vertices)-len(g.Edges)+len(g.Tiles) != 1 {
						t.Fatal("printed pasture or connected topology")
					}
					for i, want := range []int{2, 9, 11, 8, 4, 10, 10, 3, 8, 6, 5, 4, 5, 3, 10, 6, 8, 4, 11, 6, 12, 9} {
						if g.Tiles[b.Starting[i]].Number != want {
							t.Fatal("fixed starting number position", i)
						}
					}
					for region := 0; region < 2; region++ {
						counts = make([]int, 8)
						for _, h := range b.Hidden {
							if h.Region == region {
								counts[h.Resource]++
							}
						}
						want := []int{2, 1, 1, 2, 3, 0, 1, 4}
						if region == 1 {
							want = []int{1, 2, 2, 2, 2, 0, 1, 4}
						}
						switch scene.name {
						case "fish-for-catan":
							want[6] = 3
						case "spices-for-catan":
							want[5], want[6], want[7] = 3, 4, 0
						case "explorers-and-pirates":
							want[5], want[6] = 3, 4
						}
						if !slices.Equal(counts, want) {
							t.Fatal("physical region inventory", region, counts)
						}
						wantNumbers := []int{2, 3, 4, 5, 5, 6, 9, 9, 10}
						if region == 1 {
							wantNumbers = []int{3, 4, 4, 5, 8, 9, 10, 10, 11}
						}
						if !catanExplorerSameInventory(b.Numbers[region], wantNumbers) {
							t.Fatal("nine printed discovery numbers", region, b.Numbers[region])
						}
					}
					g, b = explorerMapRestore(t, g, b)
					before := b.publicView()
					permuted := clone(*b)
					for r := range permuted.Numbers {
						slices.Reverse(permuted.Numbers[r])
						indices := []int{}
						for i, h := range permuted.Hidden {
							if h.Region == r {
								indices = append(indices, i)
							}
						}
						// Move whole secret components (including farm/die pairs)
						// between positions, preserving each region's physical set.
						for i, j := 0, len(indices)-1; i < j; i, j = i+1, j-1 {
							a, z := &permuted.Hidden[indices[i]], &permuted.Hidden[indices[j]]
							first, last := a.Tile, z.Tile
							*a, *z = *z, *a
							a.Tile, z.Tile = first, last
						}
					}
					if err = permuted.validate(g); err != nil || !reflect.DeepEqual(before, permuted.publicView()) {
						t.Fatal("unexplored information leaked", err)
					}
					// Every ordinary discovery takes exactly one number from its
					// own stack; gold, farms, shoals and sea take none.
					for i, h := range slices.Clone(b.Hidden) {
						left := len(b.Numbers[h.Region])
						public := b.publicView()
						revealed, e := b.reveal(g, h.Tile)
						if e != nil {
							t.Fatal(i, e)
						}
						spent := 0
						if h.Resource < CatanDesert {
							spent = 1
						}
						if len(b.Numbers[h.Region]) != left-spent || (revealed.Number > 0) != (spent == 1) || b.publicView().Unexplored[h.Region] != public.Unexplored[h.Region]-1 {
							t.Fatal("incorrect discovery", h)
						}
						if _, e = b.reveal(g, h.Tile); e == nil {
							t.Fatal("repeated discovery accepted")
						}
						if i%7 == 0 {
							g, b = explorerMapRestore(t, g, b)
						}
					}
					if len(b.Numbers[0]) != 0 || len(b.Numbers[1]) != 0 {
						t.Fatal("nine ordinary land numbers not exhausted")
					}
					view := b.publicView()
					if catanExplorerFishScenario(scene.name) && len(view.Shoals) != 6 {
						t.Fatal("all six physical shoals required")
					}
					if catanExplorerSpiceScenario(scene.name) && len(view.Farms) != 6 {
						t.Fatal("all six physical farms required")
					}
					explorerMapRestore(t, g, b)
				}
			})
		}
	}
}

func TestCatanExplorerSixMapRejectsCorruptionAndKeepsUnfinishedGameGate(t *testing.T) {
	g, b, err := newCatanExplorerBoard(6, "explorers-and-pirates", "variable")
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Catan, *catanExplorerBoard){
		func(g *Catan, b *catanExplorerBoard) { b.Players = 4 },
		func(g *Catan, b *catanExplorerBoard) { b.Numbers[0][0] = 12 },
		func(g *Catan, b *catanExplorerBoard) { b.Hidden = b.Hidden[1:] },
		func(g *Catan, b *catanExplorerBoard) { b.Regions[1][0] = b.Regions[0][0] },
		func(g *Catan, b *catanExplorerBoard) { g.Tiles[b.Starting[0]].Number = 6 },
		func(g *Catan, b *catanExplorerBoard) { g.Tiles[b.FramePasture].Resource = 0 },
		func(g *Catan, b *catanExplorerBoard) { b.Council.Tile = b.FrameSea },
		func(g *Catan, b *catanExplorerBoard) { g.Edges[0].B = g.Edges[0].A },
		func(g *Catan, b *catanExplorerBoard) { b.HarborStarts = b.HarborStarts[:17] },
	} {
		x, y := clone(*g), clone(*b)
		mutate(&x, &y)
		if y.validate(&x) == nil {
			t.Fatal("damaged enlarged board accepted")
		}
	}
	for _, n := range []int{5, 6} {
		if _, _, err := newCatanExplorerBoard(n, "land-ho", "fixed"); err == nil {
			t.Fatal("invented five/six Land Ho opening")
		}
		if _, _, err := newCatanExplorerBoard(n, "pirate-lairs", "fixed"); err == nil {
			t.Fatal("invented five/six fixed layout")
		}
		// Map support is not paired-turn or complete game support. Keep this
		// explicit until the remaining controllers are integrated and verified.
		if _, _, _, _, _, _, err := newCatanExplorerMissionSetup(n, "explorers-and-pirates", "variable", 0); err == nil {
			t.Fatal("unfinished six-player game was opened")
		}
	}
}
