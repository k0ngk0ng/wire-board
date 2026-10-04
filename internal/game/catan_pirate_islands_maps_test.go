package game

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
)

func pirateIslandsMapGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	build := (*Catan).makeSeafarersPirateIslandsFour
	if n > 4 {
		build = (*Catan).makeSeafarersPirateIslandsSix
	}
	if err = build(s.Catan); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanPirateIslandsPrintedComponentsAndTopology(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := pirateIslandsMapGame(t, n)
			g := s.Catan
			p := g.Seafarers.PirateIslands
			terrain, numbers := make([]int, 8), make([]int, 13)
			islands := map[int]int{}
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				if tile.Number > 0 {
					numbers[tile.Number]++
				}
				if id := g.Seafarers.Islands[tile.ID]; id >= 0 {
					islands[id]++
				}
			}
			// Independent inventories printed below the maps, base p17 / extension p10.
			wantTerrain := []int{5, 5, 5, 5, 5, 3, 19, 2}
			wantNumbers := []int{0, 0, 1, 2, 3, 3, 3, 0, 3, 3, 3, 2, 1}
			wantIslands := []int{1, 2, 2, 4, 4, 17}
			homes, ports := 17, 8
			if n > 4 {
				wantTerrain = []int{6, 4, 6, 5, 7, 5, 26, 4}
				wantNumbers = []int{0, 0, 1, 4, 4, 4, 4, 0, 4, 3, 3, 4, 1}
				wantIslands = []int{1, 1, 1, 1, 1, 1, 1, 3, 3, 24}
				homes, ports = 24, 9
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
			if len(p.HomeTiles) != homes || len(g.Ports) != ports || g.Seafarers.IslandBonus != 0 || g.Seafarers.VictoryPoints != 10 || g.Robber != -1 {
				t.Fatal("scenario setup")
			}
			if len(p.Fortresses) != n || len(p.Colors) != n || len(p.FleetPath) != 14 || g.Seafarers.Pirate != p.FleetPath[0] {
				t.Fatal("starting markers")
			}
			if n == 3 && slices.Contains(p.Colors, 2) || n == 5 && slices.Contains(p.Colors, 4) {
				t.Fatal("unused printed color retained")
			}
			seenPath := map[int]bool{}
			for i, id := range p.FleetPath {
				if seenPath[id] || g.Tiles[id].Resource != CatanSea {
					t.Fatal("invalid fleet path", id)
				}
				seenPath[id] = true
				a, b := g.Tiles[id], g.Tiles[p.FleetPath[(i+1)%len(p.FleetPath)]]
				if math.Abs(math.Hypot(a.X-b.X, a.Y-b.Y)-math.Sqrt(3)*g.HexSize) > 0.001 {
					t.Fatal("nonadjacent fleet step", a.ID, b.ID)
				}
			}
			if n <= 4 && p.SafeTile != -1 || n > 4 && (!slices.Contains(p.FleetPath, p.SafeTile) || g.Tiles[p.SafeTile].Resource != CatanSea) {
				t.Fatal("safe ! sea position")
			}
			seen := map[int]bool{}
			for i, f := range p.Fortresses {
				v := g.Vertices[f.StartVertex]
				ship := g.Edges[f.StartShip]
				if v.Owner != i || v.Level != 1 || ship.Owner != i || !ship.Ship || ship.A != v.ID && ship.B != v.ID {
					t.Fatal("preset settlement and ship", i, f)
				}
				if !g.seaSetupAllowed(v.ID) || g.seaSetupAllowed(f.Beachhead) || g.seaSetupAllowed(f.Vertex) {
					t.Fatal("main/pirate island distinction", i)
				}
				if f.Strength != 3 || g.Vertices[f.Vertex].Level != 0 || g.Vertices[f.Vertex].Owner != -1 {
					t.Fatal("fortress became productive building")
				}
				if seen[f.Beachhead] || seen[f.Vertex] || f.Beachhead == f.Vertex {
					t.Fatal("overlapping markers")
				}
				seen[f.Beachhead] = true
				seen[f.Vertex] = true
				if !g.landVertex(f.Beachhead) || !g.landVertex(f.Vertex) {
					t.Fatal("marker in open water")
				}
				for _, id := range g.touching(v.ID) {
					e := g.Edges[id]
					other := e.A
					if other == v.ID {
						other = e.B
					}
					if g.Vertices[other].Level > 0 {
						t.Fatal("preset distance violation")
					}
				}
				// Both legs can be reached on sea/coastal edges using at most the 15 ships.
				first := pirateMapDistance(g, v.ID, f.Beachhead)
				second := pirateMapDistance(g, f.Beachhead, f.Vertex)
				if first < 1 || second < 1 || first+second > 15 {
					t.Fatal("unreachable fortress", first, second)
				}
				other := ship.A
				if other == v.ID {
					other = ship.B
				}
				if pirateMapDistance(g, other, f.Beachhead) != first-1 {
					t.Fatal("preset ship does not lead towards beachhead", i)
				}
				roads, settlements, cities := g.pieces(i)
				if roads != 0 || settlements != 1 || cities != 0 || g.shipCount(i) != 1 {
					t.Fatal("preset piece stock")
				}
			}
			ends := map[int]bool{}
			types := map[int]int{}
			for _, port := range g.Ports {
				e := g.Edges[port.Edge]
				types[port.Resource]++
				if ends[e.A] || ends[e.B] || !g.edgeTerrain(e.ID, true) || !g.edgeTerrain(e.ID, false) {
					t.Fatal("port not separated / coastal")
				}
				ends[e.A], ends[e.B] = true, true
				if !g.seaSetupAllowed(e.A) || !g.seaSetupAllowed(e.B) {
					t.Fatal("port outside main island")
				}
			}
			if !reflect.DeepEqual(types, map[int]int{-1: ports - 5, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}) {
				t.Fatal("port types", types)
			}
			saved := clone(*s)
			if !reflect.DeepEqual(*s, saved) {
				t.Fatal("map lost on JSON restore")
			}
			raw, err := json.Marshal(s)
			if err != nil || !json.Valid(raw) {
				t.Fatal("save invalid", err)
			}
			if g.randomizeSeafarersMap() == nil || !reflect.DeepEqual(*s, saved) {
				t.Fatal("unsupported variable setup accepted")
			}
		})
	}
}

func TestCatanPirateIslandsRejectWrongPlayerCount(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
		if err != nil {
			t.Fatal(err)
		}
		before := clone(*s)
		build := (*Catan).makeSeafarersPirateIslandsFour
		if n <= 4 {
			build = (*Catan).makeSeafarersPirateIslandsSix
		}
		if build(s.Catan) == nil || !reflect.DeepEqual(*s, before) {
			t.Fatal("wrong player count accepted or mutated map", n)
		}
	}
}

func pirateMapDistance(g *Catan, from, to int) int {
	distances := map[int]int{from: 0}
	queue := []int{from}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		if v == to {
			return distances[v]
		}
		for _, id := range g.touching(v) {
			if !g.edgeTerrain(id, true) {
				continue
			}
			e := g.Edges[id]
			next := e.A
			if next == v {
				next = e.B
			}
			if _, ok := distances[next]; !ok {
				distances[next] = distances[v] + 1
				queue = append(queue, next)
			}
		}
	}
	return -1
}
