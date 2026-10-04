package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func tribeMapGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	build := (*Catan).makeSeafarersTribeFour
	if n > 4 {
		build = (*Catan).makeSeafarersTribeSix
	}
	if err := build(s.Catan); err != nil {
		t.Fatal(err)
	}
	return s
}

func assertTribeInventory(t *testing.T, g *Catan) {
	t.Helper()
	tribe := g.tribe()
	tokens, generic := 8, 1
	if len(g.Players) > 4 {
		tokens, generic = 10, 3
	}
	if sum(tribe.Points)+len(tribe.Tokens) != tokens {
		t.Fatal("tribe point tokens not conserved")
	}
	ports := map[int]int{}
	for _, list := range [][]CatanPort{g.Ports, tribe.Ports} {
		for _, p := range list {
			ports[p.Resource]++
		}
	}
	for _, hand := range tribe.HeldPorts {
		for _, r := range hand {
			ports[r]++
		}
	}
	if !reflect.DeepEqual(ports, map[int]int{-1: generic, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}) {
		t.Fatal("tribe ports not conserved", ports)
	}
	for _, tile := range g.Tiles {
		if tile.Number == 0 {
			for _, v := range tile.Vertices {
				if g.Vertices[v].Level > 0 && !g.landVertex(v) {
					t.Fatal("building on unnumbered tribe island")
				}
			}
		}
	}
	for _, seat := range g.Seafarers.Seats {
		if seat.IslandPoints != 0 {
			t.Fatal("tribe granted exploration island points")
		}
	}
}

func TestCatanTribeOfficialMapComponentsAndRewards(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := tribeMapGame(t, n)
			g := s.Catan
			terrain, numbers := make([]int, 8), make([]int, 13)
			mainSize := 0
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				numbers[tile.Number]++
				if g.Seafarers.Islands[tile.ID] == g.Seafarers.StartIslands[0] {
					mainSize++
					if tile.Number == 0 {
						t.Fatal("mainland missing number")
					}
				} else if tile.Number != 0 {
					t.Fatal("outer tribe island has number")
				}
			}
			wantTerrain := []int{5, 5, 5, 5, 5, 3, 19, 2}
			wantNumbers := []int{31, 0, 1, 2, 2, 2, 2, 0, 2, 2, 2, 2, 1}
			wantMain, robber, pirate, cards, deck := 18, 47, 2, 4, 21
			if n > 4 {
				wantTerrain = []int{7, 7, 7, 7, 6, 4, 22, 3}
				wantNumbers = []int{34, 0, 1, 4, 4, 4, 3, 0, 3, 3, 3, 3, 1}
				wantMain, robber, pirate, cards, deck = 29, 0, 57, 6, 28
			}
			if !reflect.DeepEqual(terrain, wantTerrain) || !reflect.DeepEqual(numbers, wantNumbers) || mainSize != wantMain {
				t.Fatal("printed components / home island", terrain, numbers, mainSize)
			}
			if g.Robber != robber || g.Seafarers.Pirate != pirate || g.Tiles[robber].Resource != CatanDesert || g.Tiles[pirate].Resource != CatanSea || g.Seafarers.VictoryPoints != 13 || g.Seafarers.IslandBonus != 0 {
				t.Fatal("starting markers / winning target")
			}
			if len(g.Ports) != 0 || len(g.tribe().Development) != cards || len(g.DevDeck) != deck {
				t.Fatal("initial ports or development distribution")
			}
			for _, viewer := range []int{-1, 0, n - 1} {
				view := s.View(viewer)["catan"].(map[string]any)
				if view["ports"] == nil {
					t.Fatal("initial public ports must be an empty array")
				}
			}
			dev := make([]int, 5)
			for _, card := range g.DevDeck {
				dev[card]++
			}
			for _, reward := range g.tribe().Development {
				dev[reward.Card]++
			}
			wantDev := []int{14, 2, 2, 2, 5}
			if n > 4 {
				wantDev = []int{20, 3, 3, 3, 5}
			}
			if !reflect.DeepEqual(dev, wantDev) {
				t.Fatal("development supply", dev)
			}
			rewards := append([]int{}, g.tribe().Tokens...)
			for _, c := range g.tribe().Development {
				rewards = append(rewards, c.Edge)
			}
			portEnds := map[int]bool{}
			for _, p := range g.tribe().Ports {
				rewards = append(rewards, p.Edge)
				e := g.Edges[p.Edge]
				if portEnds[e.A] || portEnds[e.B] {
					t.Fatal("ports share an intersection")
				}
				portEnds[e.A], portEnds[e.B] = true, true
			}
			seen := map[int]bool{}
			for _, id := range rewards {
				if seen[id] || !g.edgeTerrain(id, true) || !g.edgeTerrain(id, false) {
					t.Fatal("reward edge overlaps or is not coastal", id)
				}
				seen[id] = true
				for _, tile := range g.edgeTiles(id) {
					if g.Tiles[tile].Number > 0 {
						t.Fatal("reward on home island")
					}
				}
			}
			for _, v := range g.Vertices {
				if len(g.touching(v.ID)) > 3 {
					t.Fatal("nonphysical intersection")
				}
				main := false
				for _, tile := range g.Tiles {
					if tile.Number > 0 && slices.Contains(tile.Vertices, v.ID) {
						main = true
					}
				}
				if g.landVertex(v.ID) != main || g.seaSetupAllowed(v.ID) != main {
					t.Fatal("settlement allowed outside numbered mainland")
				}
			}
			assertTribeInventory(t, g)
			if n > 4 {
				before := clone(*s)
				if g.randomizeSeafarersMap() == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("unsupported five/six variable layout accepted or mutated")
				}
			}
		})
	}
}

func TestCatanTribeVariableMapKeepsOuterIslandsAndEastRestrictions(t *testing.T) {
	for _, n := range []int{3, 4} {
		for attempt := 0; attempt < 60; attempt++ {
			s := tribeMapGame(t, n)
			before := clone(*s)
			g := s.Catan
			if err := g.randomizeSeafarersMap(); err != nil {
				t.Fatal(err)
			}
			terrain, oldTerrain, numbers, oldNumbers := []int{}, []int{}, []int{}, []int{}
			for id, tile := range g.Tiles {
				old := before.Catan.Tiles[id]
				if old.Number == 0 {
					if !reflect.DeepEqual(tile, old) {
						t.Fatal("outer islands / sea moved")
					}
					continue
				}
				terrain, oldTerrain = append(terrain, tile.Resource), append(oldTerrain, old.Resource)
				numbers, oldNumbers = append(numbers, tile.Number), append(oldNumbers, old.Number)
				if slices.Contains([]int{18, 26, 33}, id) && slices.Contains([]int{5, 6, 8, 9}, tile.Number) {
					t.Fatal("blue-bordered hex received forbidden number")
				}
				if !reflect.DeepEqual(tile.Vertices, old.Vertices) || tile.X != old.X || tile.Y != old.Y {
					t.Fatal("mainland shape changed")
				}
			}
			for _, list := range [][]int{terrain, oldTerrain, numbers, oldNumbers} {
				slices.Sort(list)
			}
			if !reflect.DeepEqual(terrain, oldTerrain) || !reflect.DeepEqual(numbers, oldNumbers) {
				t.Fatal("mainland components changed")
			}
			if !g.Seafarers.Variable || g.Robber != before.Catan.Robber || g.Seafarers.Pirate != before.Catan.Seafarers.Pirate || !reflect.DeepEqual(g.tribe().Tokens, before.Catan.tribe().Tokens) || !reflect.DeepEqual(g.tribe().Development, before.Catan.tribe().Development) {
				t.Fatal("markers or edge prizes changed")
			}
			for i, p := range g.tribe().Ports {
				if p.Edge != before.Catan.tribe().Ports[i].Edge {
					t.Fatal("port prize moved")
				}
			}
			assertTribeInventory(t, g)
			restored := clone(*s)
			if !reflect.DeepEqual(*s, restored) {
				t.Fatal("random map did not survive persistence")
			}
		}
	}
}
