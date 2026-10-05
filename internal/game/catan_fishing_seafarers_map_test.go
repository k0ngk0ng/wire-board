package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishingFourIslands(t *testing.T, n int, layout string) (*State, *catanFishingMap) {
	t.Helper()
	s, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Catan.makeFishingFourIslands(nil)
	if err != nil {
		t.Fatal(n, layout, err, s.Catan.FishingCoasts())
	}
	return s, f
}

func TestCatanFishingFourIslandsMaps(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				for range 24 {
					s, f := fishingFourIslands(t, n, layout)
					g := s.Catan
					if len(f.Lakes) != 0 || len(f.Grounds) != 6 {
						t.Fatal("Four Islands uses six grounds, no lake")
					}
					islands := g.findIslands()
					counts, byNumber := map[int]int{}, map[int]int{}
					for _, island := range islands {
						if island >= 0 {
							counts[island]++
						}
					}
					used := map[int]bool{}
					for _, ground := range f.Grounds {
						lands := []int{}
						for _, e := range ground.Edges {
							if used[e] {
								t.Fatal("overlapping grounds")
							}
							used[e] = true
							for _, p := range g.Ports {
								if e == p.Edge {
									t.Fatal("ground overlaps port")
								}
							}
							land, sea := -1, -1
							for _, tile := range g.edgeTiles(e) {
								if g.Tiles[tile].Resource == CatanSea {
									sea = tile
								} else {
									land = tile
								}
							}
							if land < 0 {
								t.Fatal("ground not adjacent to land")
							}
							lands = append(lands, land)
							if (ground.SeaTile == nil) != (sea < 0) || ground.SeaTile != nil && *ground.SeaTile != sea {
								t.Fatal("wrong pirate blocking hex")
							}
						}
						if lands[0] == lands[1] || islands[lands[0]] != islands[lands[1]] {
							t.Fatal("not a concave coast of one island")
						}
						byNumber[ground.Number] = islands[lands[0]]
					}
					if byNumber[4] != byNumber[8] || byNumber[6] != byNumber[10] || byNumber[4] == byNumber[6] || counts[byNumber[4]] != 4 || counts[byNumber[6]] != 4 || byNumber[5] == byNumber[9] || counts[byNumber[5]] <= 4 || counts[byNumber[9]] <= 4 {
						t.Fatal("wrong printed number/island groups", byNumber, counts)
					}
					before, _ := json.Marshal(s)
					placements := []CatanFishingGroundPlacement{}
					// The two small pairs and the two large numbers may swap islands.
					for _, ground := range f.Grounds {
						number := map[int]int{4: 6, 8: 10, 6: 4, 10: 8, 5: 9, 9: 5}[ground.Number]
						placements = append(placements, CatanFishingGroundPlacement{number, [2]int{ground.Edges[1], ground.Edges[0]}})
					}
					chosen, err := g.makeFishingFourIslands(placements)
					if err != nil {
						t.Fatal("valid host placement rejected", err)
					}
					encoded, _ := json.Marshal(chosen)
					var restored catanFishingMap
					if err := json.Unmarshal(encoded, &restored); err != nil {
						t.Fatal(err)
					}
					if err := restored.validate(g); err != nil || !reflect.DeepEqual(*chosen, restored) {
						t.Fatal("restore", err)
					}
					after, _ := json.Marshal(s)
					if string(before) != string(after) {
						t.Fatal("map configuration mutated scenario")
					}
				}
			})
		}
	}
}

func TestCatanFishingFourIslandsPirateProduction(t *testing.T) {
	seaCount, frameCount := 0, 0
	for _, n := range []int{3, 4} {
		s, f := fishingFourIslands(t, n, "fixed")
		g := s.Catan
		// Ports are randomized even on the fixed map. Remove them in this
		// production-only fixture so it deterministically exercises frame
		// coasts, independently of the host's/default placement strategy.
		g.Ports = nil
		var err error
		f, err = g.makeFishingFourIslands(nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, ground := range f.Grounds {
			if ground.SeaTile == nil {
				frameCount++
			} else {
				seaCount++
			}
			for _, vertex := range ground.Vertices {
				for level := 1; level <= 2; level++ {
					g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, level
					for pirate := -1; pirate < len(g.Tiles); pirate++ {
						if pirate >= 0 && g.Tiles[pirate].Resource != CatanSea {
							continue
						}
						g.Seafarers.Pirate = pirate
						want := level
						if ground.SeaTile != nil && *ground.SeaTile == pirate {
							want = 0
						}
						due, err := f.production(g, ground.Number)
						if err != nil || sum(due) != want || due[0] != want {
							t.Fatal("pirate production", ground, vertex, level, pirate, due, err)
						}
					}
					g.Vertices[vertex].Owner, g.Vertices[vertex].Level = -1, 0
				}
			}
			// Ships alongside a ground never produce fish on their own.
			for _, edge := range ground.Edges {
				g.Edges[edge].Owner, g.Edges[edge].Ship = 1, true
			}
			g.Seafarers.Pirate = -1
			due, err := f.production(g, ground.Number)
			if err != nil || sum(due) != 0 {
				t.Fatal("ships produced fish", due, err)
			}
		}
	}
	if seaCount == 0 || frameCount == 0 {
		t.Fatal("test did not exercise both sea hex and frame grounds", seaCount, frameCount)
	}
}

func TestCatanFishingFourIslandsRejectsCorruption(t *testing.T) {
	for _, corrupt := range []func(*Catan, *catanFishingMap){
		func(g *Catan, f *catanFishingMap) { f.Lakes = append(f.Lakes, catanFishingLake{}) },
		func(g *Catan, f *catanFishingMap) { f.Grounds = f.Grounds[:5] },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Number = 7 },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Number = f.Grounds[1].Number },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Vertices[1] = -1 },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Edges[0] = -1 },
		func(g *Catan, f *catanFishingMap) { id := -1; f.Grounds[0].SeaTile = &id },
		func(g *Catan, f *catanFishingMap) { g.Ports[0].Edge = f.Grounds[0].Edges[0] },
		func(g *Catan, f *catanFishingMap) {
			f.Grounds[0].Number, f.Grounds[4].Number = f.Grounds[4].Number, f.Grounds[0].Number
		},
		func(g *Catan, f *catanFishingMap) {
			g.Robber = slices.IndexFunc(g.Tiles, func(tile CatanTile) bool { return tile.Resource == CatanSea })
		},
	} {
		s, f := fishingFourIslands(t, 3, "fixed")
		corrupt(s.Catan, f)
		if err := f.validate(s.Catan); err == nil {
			t.Fatal("corrupt map accepted")
		}
	}
	for _, n := range []int{3, 5, 6} {
		scenario := "islands"
		if n == 3 {
			scenario = "fog"
		}
		s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scenario}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Catan.makeFishingFourIslands(nil); err == nil {
			t.Fatal("applied Four Islands recipe to another map")
		}
	}
}

func TestCatanFishingFourIslandsPlacementRejectionIsAtomic(t *testing.T) {
	s, f := fishingFourIslands(t, 4, "fixed")
	placements := make([]CatanFishingGroundPlacement, len(f.Grounds))
	for i, ground := range f.Grounds {
		placements[i] = CatanFishingGroundPlacement{ground.Number, ground.Edges}
	}
	for _, change := range []func([]CatanFishingGroundPlacement) []CatanFishingGroundPlacement{
		func(p []CatanFishingGroundPlacement) []CatanFishingGroundPlacement { return p[:0] },
		func(p []CatanFishingGroundPlacement) []CatanFishingGroundPlacement { return p[:5] },
		func(p []CatanFishingGroundPlacement) []CatanFishingGroundPlacement { p[0].Edges = p[1].Edges; return p },
		func(p []CatanFishingGroundPlacement) []CatanFishingGroundPlacement {
			p[0].Edges[0] = s.Catan.Ports[0].Edge
			return p
		},
		func(p []CatanFishingGroundPlacement) []CatanFishingGroundPlacement {
			p[0].Edges[1] = p[0].Edges[0]
			return p
		},
		func(p []CatanFishingGroundPlacement) []CatanFishingGroundPlacement { p[0].Number = 2; return p },
	} {
		p := change(slices.Clone(placements))
		before, _ := json.Marshal(s)
		request, _ := json.Marshal(p)
		if _, err := s.Catan.makeFishingFourIslands(p); err == nil {
			t.Fatal("invalid placement accepted")
		}
		after, _ := json.Marshal(s)
		requestAfter, _ := json.Marshal(p)
		if string(before) != string(after) || string(request) != string(requestAfter) {
			t.Fatal("rejected placement mutated state/input")
		}
	}
	// This stage must not silently enable unfinished runtime combination rules.
	tokens, err := newCatanFishingTokens(4)
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.Fishing = &CatanFishing{Map: *f, Tokens: *tokens, Started: make([]bool, 4), LastRollID: -1}
	if err := s.Catan.validateFishing(); err == nil {
		t.Fatal("unfinished Fishing + Seafarers became playable")
	}
}
