package game

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func desertGame(t *testing.T, n int) *State {
	t.Helper()
	s, e := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if e != nil {
		t.Fatal(e)
	}
	build := (*Catan).makeSeafarersDesertSix
	if n == 3 {
		build = (*Catan).makeSeafarersDesertThree
	} else if n == 4 {
		build = (*Catan).makeSeafarersDesertFour
	}
	if e = build(s.Catan); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestCatanDesertOfficialComponentsAndRegions(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := desertGame(t, n)
			g := s.Catan
			terrain, numbers := make([]int, 8), make([]int, 13)
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				numbers[tile.Number]++
				if (tile.Resource == CatanSea || tile.Resource == CatanDesert) && g.Seafarers.Islands[tile.ID] != -1 {
					t.Fatal("sea/desert counted as a reward region")
				}
			}
			wantTerrain := []int{7, 7, 7, 7, 7, 5, 20, 3}
			wantNumbers := []int{25, 0, 3, 4, 4, 4, 4, 0, 4, 4, 4, 4, 3}
			wantRegions := []int{1, 1, 2, 4, 4, 5, 21}
			mainSize, robber, pirate := 21, 12, 61
			generic := 5
			if n == 3 {
				wantTerrain = []int{5, 3, 4, 4, 4, 3, 10, 2}
				wantNumbers = []int{13, 0, 1, 2, 3, 3, 3, 0, 3, 3, 2, 1, 1}
				wantRegions = []int{1, 2, 2, 3, 14}
				mainSize, robber, pirate, generic = 14, 5, -1, 3
			}
			if n == 4 {
				wantTerrain = []int{5, 5, 5, 5, 5, 3, 12, 2}
				wantNumbers = []int{15, 0, 1, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}
				wantRegions = []int{2, 2, 3, 3, 17}
				mainSize, robber, pirate, generic = 17, 6, -1, 4
			}
			if !reflect.DeepEqual(terrain, wantTerrain) || !reflect.DeepEqual(numbers, wantNumbers) {
				t.Fatal("printed components", terrain, numbers)
			}
			counts := map[int]int{}
			for _, id := range g.Seafarers.Islands {
				if id >= 0 {
					counts[id]++
				}
			}
			sizes := []int{}
			for _, v := range counts {
				sizes = append(sizes, v)
			}
			slices.Sort(sizes)
			if !reflect.DeepEqual(sizes, wantRegions) || len(g.Seafarers.StartIslands) != 1 || counts[g.Seafarers.StartIslands[0]] != mainSize {
				t.Fatal("exploration regions", sizes, g.Seafarers.StartIslands)
			}
			if g.Robber != robber || g.Tiles[g.Robber].Resource != CatanDesert || g.Seafarers.Pirate != pirate || g.Seafarers.VictoryPoints != 14 {
				t.Fatal("starting markers / score target")
			}
			physical := g.findIslands()
			if physical[0] != physical[g.Robber] || g.Seafarers.Islands[0] == g.Seafarers.StartIslands[0] {
				t.Fatal("land across desert was not split from mainland")
			}
			ports := map[int]int{}
			used := map[int]bool{}
			for _, p := range g.Ports {
				ports[p.Resource]++
				if !g.edgeTerrain(p.Edge, true) || !g.edgeTerrain(p.Edge, false) {
					t.Fatal("noncoastal port")
				}
				e := g.Edges[p.Edge]
				for _, v := range []int{e.A, e.B} {
					if used[v] {
						t.Fatal("ports share intersection")
					}
					used[v] = true
				}
			}
			wantPorts := map[int]int{-1: generic, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}
			if n > 4 {
				wantPorts[2] = 2
			}
			if !reflect.DeepEqual(ports, wantPorts) {
				t.Fatal("port supply", ports)
			}
			for _, v := range g.Vertices {
				if len(g.touching(v.ID)) > 3 {
					t.Fatal("nonphysical intersection")
				}
				main := false
				for _, tile := range g.Tiles {
					if slices.Contains(tile.Vertices, v.ID) && g.Seafarers.Islands[tile.ID] == g.Seafarers.StartIslands[0] {
						main = true
					}
				}
				if g.seaSetupAllowed(v.ID) != main {
					t.Fatal("starting outside mainland", v.ID)
				}
			}
			before := clone(*s)
			if n > 4 {
				if err := g.randomizeSeafarersMap(); err == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("unsupported six-player variable setup accepted/mutated")
				}
			}
		})
	}
}
func TestCatanDesertCrossingRewardsAndSetup(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := desertGame(t, n)
			g := s.Catan
			helperReject(t, s, s.Turn, Action{Type: "catan_settlement", Vertex: g.Tiles[0].Vertices[0]})
			for s.Catan.setup() {
				p := s.Turn
				if actor := s.CatanPendingActor(); actor >= 0 {
					p = actor
				}
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				helperApply(t, s, p, a)
			}
			g = s.Catan
			main := g.Seafarers.StartIslands[0]
			for _, seat := range g.Seafarers.Seats {
				if !reflect.DeepEqual(seat.HomeIslands, []int{main}) || seat.IslandPoints != 0 {
					t.Fatal("setup home or bonus", seat)
				}
			}
			// A land-only path across the desert does not turn the northern land strip
			// into part of the home region. Two players may each earn the same reward.
			region := g.Seafarers.Islands[0]
			vertices := []int{}
			for _, v := range g.Vertices {
				if g.islandAt(v.ID) == region {
					vertices = append(vertices, v.ID)
				}
			}
			if len(vertices) < 2 {
				t.Fatal("missing northern vertices")
			}
			s.catanSettleIsland(0, vertices[0], false)
			restored := clone(*s)
			s = &restored
			g = s.Catan
			s.catanSettleIsland(0, vertices[1], false)
			s.catanSettleIsland(1, vertices[1], false)
			if g.Seafarers.Seats[0].IslandPoints != 2 || g.Seafarers.Seats[1].IslandPoints != 2 {
				t.Fatal("reward missing/repeated", g.Seafarers.Seats)
			}
			if !strings.Contains(strings.Join(s.Log, " "), "新的区域") {
				t.Fatal("misleading exploration log")
			}
			for _, v := range g.Vertices {
				if g.islandAt(v.ID) < 0 {
					s.catanSettleIsland(0, v.ID, false)
				}
			}
			if g.Seafarers.Seats[0].IslandPoints != 2 {
				t.Fatal("desert/sea-only vertex granted reward")
			}
			s.Turn = 0
			s.Phase = "catan_turn"
			g.Players[0].Score = 13
			s.catanVictory()
			if s.Finished {
				t.Fatal("early victory")
			}
			g.Players[0].Score = 14
			s.catanVictory()
			if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) {
				t.Fatal("fourteen-point victory")
			}
		})
	}
}
func TestCatanDesertVariableSetupPreservesBeltAndSupplies(t *testing.T) {
	for _, n := range []int{3, 4} {
		for attempt := 0; attempt < 60; attempt++ {
			s := desertGame(t, n)
			g := s.Catan
			before := clone(*s)
			inventory := func(c *Catan) [2][2][]int {
				var out [2][2][]int
				for k := range out {
					out[k][0] = make([]int, 8)
					out[k][1] = make([]int, 13)
				}
				for _, tile := range c.Tiles {
					if tile.Resource == CatanSea || tile.Resource == CatanDesert {
						continue
					}
					k := 0
					if c.Seafarers.Islands[tile.ID] != c.Seafarers.StartIslands[0] {
						k = 1
					}
					out[k][0][tile.Resource]++
					out[k][1][tile.Number]++
				}
				return out
			}
			if err := g.randomizeSeafarersMap(); err != nil {
				t.Fatal(n, attempt, err)
			}
			if !reflect.DeepEqual(inventory(g), inventory(before.Catan)) {
				t.Fatal("main/unexplored component pools mixed")
			}
			if !reflect.DeepEqual(g.Seafarers.Islands, before.Catan.Seafarers.Islands) || g.Robber != before.Catan.Robber || g.Seafarers.Pirate != -1 || !g.Seafarers.Variable {
				t.Fatal("regions or starting markers changed")
			}
			for i, tile := range g.Tiles {
				old := before.Catan.Tiles[i]
				if (old.Resource == CatanSea || old.Resource == CatanDesert) && !reflect.DeepEqual(tile, old) {
					t.Fatal("sea/desert belt moved")
				}
				if tile.Resource == CatanGold && (tile.Number == 6 || tile.Number == 8) {
					t.Fatal("gold has red number")
				}
			}
			for _, e := range g.Edges {
				if len(e.Tiles) == 2 {
					a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
					if (a == 6 || a == 8) && (b == 6 || b == 8) {
						t.Fatal("adjacent red numbers")
					}
				}
			}
			for i, p := range g.Ports {
				if p.Edge != before.Catan.Ports[i].Edge {
					t.Fatal("port location moved")
				}
			}
			restored := clone(*s)
			if !reflect.DeepEqual(restored, *s) {
				t.Fatal("variable map restoration")
			}
		}
	}
}
