package game

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"
)

func fishingMap(t *testing.T, n int) (*State, *catanFishingMap) {
	t.Helper()
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Catan.makeFishingMap()
	if err != nil {
		t.Fatal(err)
	}
	return s, f
}

func TestCatanFishingMapOfficialInventoriesAndGeometry(t *testing.T) {
	for n := 3; n <= 6; n++ {
		lakeSites := map[int]bool{}
		for range 24 {
			s, f := fishingMap(t, n)
			g := s.Catan
			if g.Robber != -1 || s.Phase != "catan_setup_settlement" || g.SetupStep != 0 {
				t.Fatal("fishing starts with the robber offboard and normal setup")
			}
			terrain, numbers, ports := make([]int, 10), make([]int, 13), make([]int, 6)
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				numbers[tile.Number]++
				if len(tile.Vertices) != 6 {
					t.Fatal("nonhexagonal tile")
				}
				for _, id := range tile.Vertices {
					v := g.Vertices[id]
					if math.Abs(math.Hypot(tile.X-v.X, tile.Y-v.Y)-g.HexSize) > .001 {
						t.Fatal("hex geometry changed")
					}
				}
			}
			for _, p := range g.Ports {
				ports[p.Resource+1]++
			}
			wantTerrain := []int{4, 3, 4, 4, 3, 0, 0, 0, 0, 1}
			wantNumbers := []int{1, 0, 1, 2, 2, 2, 2, 0, 2, 2, 2, 2, 1}
			wantPorts := []int{4, 1, 1, 1, 1, 1}
			wantVertices, wantEdges, wantBoundary := 54, 72, 30
			if n > 4 {
				wantTerrain = []int{6, 5, 6, 6, 5, 0, 0, 0, 0, 2}
				wantNumbers = []int{2, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}
				wantPorts = []int{5, 1, 1, 2, 1, 1}
				wantVertices, wantEdges, wantBoundary = 80, 109, 38
			}
			if !slices.Equal(terrain, wantTerrain) || !slices.Equal(numbers, wantNumbers) || !slices.Equal(ports, wantPorts) || len(g.Vertices) != wantVertices || len(g.Edges) != wantEdges {
				t.Fatal("printed map inventories", n, terrain, numbers, ports)
			}
			boundary := 0
			for _, e := range g.Edges {
				if len(e.Tiles) == 1 {
					boundary++
				} else if len(e.Tiles) != 2 {
					t.Fatal("invalid edge adjacency")
				} else {
					a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
					if (a == 6 || a == 8) && (b == 6 || b == 8) {
						t.Fatal("adjacent red numbers")
					}
				}
			}
			if boundary != wantBoundary {
				t.Fatal("wrong island outline")
			}
			for _, lake := range f.Lakes {
				lakeSites[lake.Tile] = true
				for _, e := range g.Edges {
					if slices.Contains(e.Tiles, lake.Tile) && len(e.Tiles) == 1 {
						t.Fatal("lake is on the outer ring")
					}
				}
			}
			for _, ground := range f.Grounds {
				// The central V vertex touches exactly two hexes, not the
				// convex three-corner shape used by some naive placements.
				touching := 0
				for _, tile := range g.Tiles {
					if slices.Contains(tile.Vertices, ground.Vertices[1]) {
						touching++
					}
				}
				if touching != 2 {
					t.Fatal("ground does not occupy a concave coastal V")
				}
			}
			encoded, _ := json.Marshal(f)
			var restored catanFishingMap
			if err := json.Unmarshal(encoded, &restored); err != nil {
				t.Fatal(err)
			}
			copy := clone(*s)
			if err := restored.validate(copy.Catan); err != nil || !reflect.DeepEqual(*f, restored) {
				t.Fatal("map save/restore", err)
			}
		}
		if len(lakeSites) < 2 {
			t.Fatal("lake location never varies")
		}
	}
}

func TestCatanFishingMapLakeAndAllThreeCoastalVerticesProduce(t *testing.T) {
	for _, n := range []int{3, 6} {
		s, f := fishingMap(t, n)
		g := s.Catan
		for _, ground := range f.Grounds {
			for _, id := range ground.Vertices {
				for level := 1; level <= 2; level++ {
					g.Vertices[id].Owner, g.Vertices[id].Level = 1, level
					// A robber on an adjacent ordinary tile cannot block coast fishing.
					g.Robber = g.Edges[ground.Edges[0]].Tiles[0]
					due, err := f.production(g, ground.Number)
					if err != nil || due[1] != level || sum(due) != level {
						t.Fatal("coastal endpoint/center production", ground, id, due, err)
					}
					g.Vertices[id].Owner, g.Vertices[id].Level = -1, 0
				}
			}
		}
		for _, lake := range f.Lakes {
			for _, id := range g.Tiles[lake.Tile].Vertices {
				for level := 1; level <= 2; level++ {
					g.Vertices[id].Owner, g.Vertices[id].Level = 0, level
					for total := 2; total <= 12; total++ {
						for _, robber := range []int{-1, lake.Tile} {
							g.Robber = robber
							want := 0
							// Adjacent lakes independently produce on their own numbers.
							for _, other := range f.Lakes {
								if other.Tile != robber && slices.Contains(other.Numbers, total) && slices.Contains(g.Tiles[other.Tile].Vertices, id) {
									want += level
								}
							}
							due, err := f.production(g, total)
							if err != nil || sum(due) != want || due[0] != want {
								t.Fatal("lake production/robber", total, due, want, err)
							}
						}
					}
					g.Vertices[id].Owner, g.Vertices[id].Level = -1, 0
				}
			}
		}
	}
}

func TestCatanFishingMapAggregatesLakeCoastAndSkipsEliminated(t *testing.T) {
	s, f := fishingMap(t, 6)
	g := s.Catan
	g.Robber = -1
	lake := f.Lakes[1] // The added 4/10 lake can produce along with a fishing ground.
	ground := f.Grounds[slices.IndexFunc(f.Grounds, func(x catanFishingGround) bool { return x.Number == 4 })]
	a, b := g.Tiles[lake.Tile].Vertices[0], ground.Vertices[0]
	g.Vertices[a].Owner, g.Vertices[a].Level = 2, 2
	g.Vertices[b].Owner, g.Vertices[b].Level = 2, 1
	g.Vertices[ground.Vertices[2]].Owner, g.Vertices[ground.Vertices[2]].Level = 3, 2
	g.Players[3].Eliminated = true
	before, _ := json.Marshal(s)
	due, err := f.production(g, 4)
	if err != nil || due[2] != 3 || sum(due) != 3 {
		t.Fatal("lake and coast must combine without paying eliminated seats", due, err)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("computing claims changed resources/state")
	}
	// Combined production must enter the token economy in one call, so a
	// player already at the cap cannot replace separately for lake and coast.
	tokens := fishingTokens(t, 6)
	fishOwn(tokens, 2, 0, 1, 2, 3, 4, 5, 6)
	fishTop(tokens, 21, 22, 23)
	if err := tokens.beginDraw(0, due); err != nil {
		t.Fatal(err)
	}
	id := 0
	if err := tokens.replace(2, &id); err != nil {
		t.Fatal(err)
	}
	if len(tokens.Pending) != 0 || len(tokens.Hands[2]) != 7 || !slices.Contains(tokens.Hands[2], 21) || slices.Contains(tokens.Hands[2], 22) {
		t.Fatal("multiple replacements for combined production")
	}
	if _, err := f.production(g, 1); err == nil {
		t.Fatal("invalid roll accepted")
	}
}

func TestCatanFishingMapRejectsChangedBoardAndCorruptLocations(t *testing.T) {
	for _, change := range []func(*Catan){
		func(g *Catan) { g.SetupStep = 1 },
		func(g *Catan) { g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1 },
		func(g *Catan) { g.Edges[0].Owner = 0 },
		func(g *Catan) { g.Seafarers = &CatanSeafarers{} },
		func(g *Catan) { g.CitiesKnights = &CatanCitiesKnights{} },
	} {
		s, _ := fishingMap(t, 3)
		change(s.Catan)
		before, _ := json.Marshal(s)
		if _, err := s.Catan.makeFishingMap(); err == nil {
			t.Fatal("replaced active or different scenario board")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("rejected map generation mutated game")
		}
	}
	for _, corrupt := range []func(*Catan, *catanFishingMap){
		func(g *Catan, f *catanFishingMap) { f.Lakes[0].Tile = 0 },
		func(g *Catan, f *catanFishingMap) { f.Lakes[0].Numbers = []int{4, 10} },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Vertices[0] = -1 },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Vertices[0] = f.Grounds[0].Vertices[1] },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Edges[0] = g.Ports[0].Edge },
		func(g *Catan, f *catanFishingMap) { f.Grounds[0].Number = 7 },
		func(g *Catan, f *catanFishingMap) { g.Ports[0].Edge = -1 },
	} {
		s, f := fishingMap(t, 3)
		corrupt(s.Catan, f)
		if _, err := f.production(s.Catan, 4); err == nil {
			t.Fatal("corrupt fishing map accepted")
		}
	}
}
