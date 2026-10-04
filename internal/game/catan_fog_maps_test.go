package game

import (
	"reflect"
	"slices"
	"testing"
)

func fogInventory(g *Catan) ([]int, []int) {
	terrain, numbers := make([]int, 8), make([]int, 13)
	for _, tile := range g.Tiles {
		if tile.Resource != CatanFog {
			terrain[tile.Resource]++
			if tile.Number > 0 {
				numbers[tile.Number]++
			}
		}
	}
	for _, r := range g.Seafarers.Fog.Terrain {
		terrain[r]++
	}
	for _, n := range g.Seafarers.Fog.Numbers {
		numbers[n]++
	}
	return terrain, numbers
}

func TestCatanFogOfficialMapComponents(t *testing.T) {
	for _, tc := range []struct {
		n                                        int
		build                                    func(*Catan) error
		terrain, numbers, hidden, discs, islands []int
		generic, robber                          int
	}{
		{3, (*Catan).makeSeafarersFogThree, []int{4, 2, 4, 2, 2, 0, 16, 0, 12}, []int{28, 0, 0, 1, 1, 2, 2, 0, 2, 2, 1, 2, 1}, []int{1, 2, 1, 2, 2, 0, 2, 2}, []int{0, 0, 0, 2, 1, 1, 1, 0, 1, 1, 1, 1, 1}, []int{7, 7}, 3, 2},
		{4, (*Catan).makeSeafarersFogFour, []int{4, 3, 4, 3, 3, 0, 13, 0, 12}, []int{25, 0, 1, 2, 2, 2, 2, 0, 2, 2, 2, 1, 1}, []int{1, 2, 1, 2, 2, 0, 2, 2}, []int{0, 0, 0, 1, 1, 1, 1, 0, 1, 1, 1, 2, 1}, []int{7, 10}, 4, 1},
		{5, (*Catan).makeSeafarersFogSix, []int{5, 5, 5, 5, 4, 0, 14, 0, 18}, []int{32, 0, 1, 2, 3, 2, 3, 0, 3, 3, 3, 2, 2}, []int{2, 2, 2, 2, 3, 1, 3, 3}, []int{0, 0, 2, 2, 1, 2, 1, 0, 1, 1, 1, 2, 1}, []int{12, 12}, 5, 3},
		{6, (*Catan).makeSeafarersFogSix, []int{5, 5, 5, 5, 4, 0, 14, 0, 18}, []int{32, 0, 1, 2, 3, 2, 3, 0, 3, 3, 3, 2, 2}, []int{2, 2, 2, 2, 3, 1, 3, 3}, []int{0, 0, 2, 2, 1, 2, 1, 0, 1, 1, 1, 2, 1}, []int{12, 12}, 5, 3},
	} {
		s, err := NewCatan(tc.n, CatanOptions{FiveSix: tc.n > 4})
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if err = tc.build(g); err != nil {
			t.Fatal(tc.n, err)
		}
		terrain, numbers, hidden, discs := make([]int, 9), make([]int, 13), make([]int, 8), make([]int, 13)
		for _, tile := range g.Tiles {
			terrain[tile.Resource]++
			numbers[tile.Number]++
		}
		for _, resource := range g.Seafarers.Fog.Terrain {
			hidden[resource]++
		}
		for _, number := range g.Seafarers.Fog.Numbers {
			discs[number]++
		}
		if !reflect.DeepEqual(terrain, tc.terrain) || !reflect.DeepEqual(numbers, tc.numbers) || !reflect.DeepEqual(hidden, tc.hidden) || !reflect.DeepEqual(discs, tc.discs) {
			t.Fatal("printed component inventory", tc.n, terrain, numbers, hidden, discs)
		}
		if len(g.Seafarers.Fog.Terrain) != terrain[CatanFog] || g.Seafarers.VictoryPoints != 12 || g.Seafarers.IslandBonus != 0 || g.Seafarers.Pirate != -1 || g.Tiles[g.Robber].Resource != tc.robber || g.Tiles[g.Robber].Number != 12 {
			t.Fatal("markers or victory rules", tc.n)
		}
		regions := map[int]int{}
		for _, id := range g.Seafarers.Islands {
			if id >= 0 {
				regions[id]++
			}
		}
		sizes := []int{}
		for _, size := range regions {
			sizes = append(sizes, size)
		}
		slices.Sort(sizes)
		if !reflect.DeepEqual(sizes, tc.islands) {
			t.Fatal("visible island shapes", tc.n, sizes)
		}
		seen := map[int]bool{}
		for _, id := range g.Seafarers.Fog.StartTiles {
			if seen[id] || g.Tiles[id].Resource >= 5 {
				t.Fatal("invalid start area")
			}
			seen[id] = true
		}
		if len(seen) != sum(tc.islands) {
			t.Fatal("incomplete start area")
		}
		ports := map[int]int{}
		used := map[int]bool{}
		for _, p := range g.Ports {
			ports[p.Resource]++
			if !g.edgeTerrain(p.Edge, false) || !g.edgeTerrain(p.Edge, true) {
				t.Fatal("inland port")
			}
			e := g.Edges[p.Edge]
			for _, id := range []int{e.A, e.B} {
				if used[id] {
					t.Fatal("ports share vertex")
				}
				used[id] = true
			}
		}
		wantPorts := map[int]int{-1: tc.generic, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}
		if tc.n > 4 {
			wantPorts[2] = 2
		}
		if !reflect.DeepEqual(ports, wantPorts) {
			t.Fatal("port inventory", tc.n, ports)
		}
		for _, v := range g.Vertices {
			if len(g.touching(v.ID)) > 3 {
				t.Fatal("nonphysical intersection")
			}
		}
	}
}

func TestCatanFogVariableMapsPreserveStacksAndOutlines(t *testing.T) {
	for _, n := range []int{3, 4} {
		for sample := 0; sample < 30; sample++ {
			s := catanGame(t, n)
			g := s.Catan
			build := (*Catan).makeSeafarersFogThree
			if n == 4 {
				build = (*Catan).makeSeafarersFogFour
			}
			if err := build(g); err != nil {
				t.Fatal(err)
			}
			before := clone(*s).Catan
			a, b := fogInventory(g)
			if err := g.randomizeSeafarersMap(); err != nil {
				t.Fatal(err)
			}
			c, d := fogInventory(g)
			if !reflect.DeepEqual(a, c) || !reflect.DeepEqual(b, d) || !reflect.DeepEqual(g.Seafarers.Fog, before.Seafarers.Fog) {
				t.Fatal("hidden pile or inventory changed")
			}
			if !reflect.DeepEqual(g.Edges, before.Edges) || !reflect.DeepEqual(g.Vertices, before.Vertices) {
				t.Fatal("map geometry changed")
			}
			for id, tile := range g.Tiles {
				old := before.Tiles[id]
				if old.Resource >= CatanSea && !reflect.DeepEqual(old, tile) {
					t.Fatal("water/fog outline changed")
				}
			}
			for i, p := range g.Ports {
				if p.Edge != before.Ports[i].Edge {
					t.Fatal("port moved")
				}
			}
			if !g.Seafarers.Variable || g.Tiles[g.Robber].Number != 12 {
				t.Fatal("variable setup marker")
			}
			after := clone(*s).Catan
			if !reflect.DeepEqual(g, after) {
				t.Fatal("save did not preserve random board")
			}
		}
	}
}
