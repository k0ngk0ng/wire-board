package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func clothMapGame(t *testing.T, n int, helpers bool) *State {
	t.Helper()
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers})
	if err != nil {
		t.Fatal(err)
	}
	build := (*Catan).makeSeafarersClothFour
	if n > 4 {
		build = (*Catan).makeSeafarersClothSix
	}
	if err = build(s.Catan); err != nil {
		t.Fatal(err)
	}
	return s
}
func assertClothInventory(t *testing.T, g *Catan) {
	t.Helper()
	want := 50
	if len(g.Players) > 4 {
		want = 70
	}
	if clothTotal(g) != want || g.cloth().Stock < 0 {
		t.Fatal("cloth conservation", clothTotal(g), want)
	}
	for _, v := range g.cloth().Villages {
		if v.Stock < 0 || v.Stock > 5 {
			t.Fatal("village stock", v)
		}
		seen := map[int]bool{}
		for _, p := range v.Traders {
			if p < 0 || p >= len(g.Players) || seen[p] {
				t.Fatal("invalid/duplicate trader", v)
			}
			seen[p] = true
		}
	}
	for _, held := range g.cloth().Held {
		if held < 0 {
			t.Fatal("negative cloth")
		}
	}
	for _, v := range g.Vertices {
		if v.Level > 0 && !g.landVertex(v.ID) {
			t.Fatal("building on cloth island")
		}
	}
	for _, p := range g.Seafarers.Seats {
		if p.IslandPoints != 0 {
			t.Fatal("unexpected exploration bonus")
		}
	}
	if g.LongestOwner != -1 {
		t.Fatal("cloth awarded longest route")
	}
}
func TestCatanClothOfficialMapComponentsAndVillages(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := clothMapGame(t, n, false)
			g := s.Catan
			c := g.cloth()
			terrain, numbers := make([]int, 8), make([]int, 13)
			islands := map[int]int{}
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				if tile.Number > 0 {
					numbers[tile.Number]++
				}
				if island := g.Seafarers.Islands[tile.ID]; island >= 0 {
					islands[island]++
				}
				if (tile.Number > 0) != slices.Contains(c.HomeTiles, tile.ID) {
					t.Fatal("home tiles include small island")
				}
			}
			for _, v := range c.Villages {
				numbers[v.Number]++
			}
			wantTerrain := []int{4, 3, 4, 5, 4, 2, 18, 2}
			wantNumbers := []int{0, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}
			wantIslands := []int{1, 1, 1, 1, 10, 10}
			wantHomes, wantVillages, wantPorts, robber, pirate := 20, 8, 9, 11, -1
			if n > 4 {
				wantTerrain = []int{6, 4, 5, 6, 5, 4, 24, 2}
				wantNumbers = []int{0, 0, 3, 4, 4, 4, 4, 0, 4, 4, 4, 4, 3}
				wantIslands = []int{1, 1, 1, 1, 1, 1, 13, 13}
				wantHomes, wantVillages, wantPorts, robber, pirate = 26, 12, 11, 15, -1
			}
			if !reflect.DeepEqual(terrain, wantTerrain) || !reflect.DeepEqual(numbers, wantNumbers) {
				t.Fatal("printed inventory", terrain, numbers)
			}
			sizes := []int{}
			for _, size := range islands {
				sizes = append(sizes, size)
			}
			slices.Sort(sizes)
			if !reflect.DeepEqual(sizes, wantIslands) {
				t.Fatal("island outlines", sizes)
			}
			if len(c.HomeTiles) != wantHomes || len(c.Villages) != wantVillages || c.Stock != 10 || c.EmptyLimit != 5 || len(g.Ports) != wantPorts {
				t.Fatal("wrong setup supply", c)
			}
			if g.Robber != robber || g.Seafarers.Pirate != pirate || g.Seafarers.VictoryPoints != 14 || g.Seafarers.IslandBonus != 0 {
				t.Fatal("starting marker / victory settings")
			}
			if len(g.Seafarers.StartIslands) != 2 || g.SetupLimit() != 3*n {
				t.Fatal("setup islands / passes")
			}
			seen := map[int]bool{}
			for _, v := range c.Villages {
				if seen[v.Vertex] || g.landVertex(v.Vertex) || g.seaSetupAllowed(v.Vertex) || v.Stock != 5 || len(v.Traders) != 0 {
					t.Fatal("invalid village location", v)
				}
				seen[v.Vertex] = true
				if len(g.touching(v.Vertex)) != 3 {
					t.Fatal("village not a three-way intersection")
				}
				for _, id := range g.touching(v.Vertex) {
					if !g.edgeTerrain(id, true) {
						t.Fatal("village route not navigable")
					}
				}
				// Printed villages use the north and south corners of isolated hexes.
				matches := 0
				for _, tile := range g.Tiles {
					if tile.Resource != CatanSea && slices.Contains(tile.Vertices, v.Vertex) {
						matches++
						if v.Vertex != tile.Vertices[4] && v.Vertex != tile.Vertices[1] {
							t.Fatal("village on wrong corner")
						}
						if tile.Number != 0 {
							t.Fatal("cloth village became hex production")
						}
					}
				}
				if matches != 1 {
					t.Fatal("village not on an isolated island")
				}
			}
			ports := map[int]int{}
			endpoints := map[int]bool{}
			for _, p := range g.Ports {
				ports[p.Resource]++
				e := g.Edges[p.Edge]
				if endpoints[e.A] || endpoints[e.B] || !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) {
					t.Fatal("overlapping / inland port")
				}
				endpoints[e.A], endpoints[e.B] = true, true
			}
			wantPortTypes := map[int]int{-1: 4, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}
			if n > 4 {
				wantPortTypes[-1] = 5
				wantPortTypes[2] = 2
			}
			if !reflect.DeepEqual(ports, wantPortTypes) {
				t.Fatal("port inventory", ports)
			}
			for _, v := range g.Vertices {
				if g.landVertex(v.ID) != g.seaSetupAllowed(v.ID) {
					t.Fatal("setup allowed outside large islands")
				}
			}
			assertClothInventory(t, g)
			if n > 4 {
				before := clone(*s)
				if g.randomizeSeafarersMap() == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("unsupported variable layout accepted/mutated")
				}
			}
		})
	}
}
func TestCatanClothVariableMapKeepsVillagesAndSea(t *testing.T) {
	for _, n := range []int{3, 4} {
		for attempt := 0; attempt < 60; attempt++ {
			s := clothMapGame(t, n, false)
			before := clone(*s)
			g := s.Catan
			if err := g.randomizeSeafarersMap(); err != nil {
				t.Fatal(err)
			}
			terrain, oldTerrain, numbers, oldNumbers := []int{}, []int{}, []int{}, []int{}
			for i, tile := range g.Tiles {
				old := before.Catan.Tiles[i]
				if !g.clothLand(i) {
					if !reflect.DeepEqual(tile, old) {
						t.Fatal("village island or sea moved")
					}
					continue
				}
				terrain = append(terrain, tile.Resource)
				oldTerrain = append(oldTerrain, old.Resource)
				numbers = append(numbers, tile.Number)
				oldNumbers = append(oldNumbers, old.Number)
				if tile.X != old.X || tile.Y != old.Y || !reflect.DeepEqual(tile.Vertices, old.Vertices) {
					t.Fatal("home outline moved")
				}
			}
			for _, v := range [][]int{terrain, oldTerrain, numbers, oldNumbers} {
				slices.Sort(v)
			}
			if !reflect.DeepEqual(terrain, oldTerrain) || !reflect.DeepEqual(numbers, oldNumbers) {
				t.Fatal("home components changed")
			}
			if !reflect.DeepEqual(g.cloth(), before.Catan.cloth()) || !reflect.DeepEqual(g.Seafarers.StartIslands, before.Catan.Seafarers.StartIslands) || g.Seafarers.Pirate != before.Catan.Seafarers.Pirate || !g.Seafarers.Variable {
				t.Fatal("fixed scenario state changed")
			}
			if g.Tiles[g.Robber].Number != 12 || !g.clothLand(g.Robber) {
				t.Fatal("robber must start on large-island 12")
			}
			for i, p := range g.Ports {
				if p.Edge != before.Catan.Ports[i].Edge {
					t.Fatal("port moved")
				}
			}
			restored := clone(*s)
			if !reflect.DeepEqual(*s, restored) {
				t.Fatal("variable map persistence")
			}
			assertClothInventory(t, g)
		}
	}
}

func assertClothFinished(t *testing.T, s *State) {
	t.Helper()
	if !s.Finished || len(s.Winners) == 0 {
		t.Fatal("cloth bots did not finish", s.Round, s.Phase)
	}
	g := s.Catan
	c := g.cloth()
	if g.Players[s.Turn].Score >= 14 {
		if !reflect.DeepEqual(s.Winners, []int{s.Turn}) {
			t.Fatal("own fourteen-point victory must take priority")
		}
		return
	}
	empty := 0
	for _, v := range c.Villages {
		if v.Stock == 0 {
			empty++
		}
	}
	if empty < 5 {
		t.Fatal("cloth game ended without fourteen points or five depleted villages")
	}
	bestScore, bestCloth := -1, -1
	for i, p := range g.Players {
		if !p.Eliminated && (p.Score > bestScore || (p.Score == bestScore && c.Held[i] > bestCloth)) {
			bestScore, bestCloth = p.Score, c.Held[i]
		}
	}
	want := []int{}
	for i, p := range g.Players {
		if !p.Eliminated && p.Score == bestScore && c.Held[i] == bestCloth {
			want = append(want, i)
		}
	}
	if !reflect.DeepEqual(s.Winners, want) {
		t.Fatal("incorrect depletion winners", s.Winners, want)
	}
}
