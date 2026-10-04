package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanSeafarersAdditionalFixedComponents(t *testing.T) {
	for _, tc := range []struct {
		name                                           string
		players                                        int
		build                                          func(*Catan) error
		terrain, numbers, islands                      []int
		generic, robberResource, robberNumber, victory int
	}{
		{"shores3", 3, (*Catan).makeSeafarersShoresThree, []int{3, 4, 5, 4, 4, 0, 13, 2}, []int{13, 0, 1, 2, 3, 3, 2, 0, 3, 2, 3, 2, 1}, []int{2, 2, 4, 14}, 3, 1, 12, 14},
		{"islands3", 3, (*Catan).makeSeafarersIslandsThree, []int{4, 4, 4, 4, 4, 0, 15, 0}, []int{15, 0, 1, 2, 2, 3, 2, 0, 2, 3, 2, 2, 1}, []int{4, 4, 6, 6}, 4, 2, 12, 13},
		{"islands4", 4, (*Catan).makeSeafarersIslandsFour, []int{5, 4, 5, 5, 4, 0, 12, 0}, []int{12, 0, 1, 2, 3, 3, 2, 0, 2, 3, 3, 3, 1}, []int{4, 4, 7, 8}, 4, 3, 12, 13},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := catanGame(t, tc.players)
			g := s.Catan
			if err := tc.build(g); err != nil {
				t.Fatal(err)
			}
			terrain, numbers := make([]int, 8), make([]int, 13)
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				numbers[tile.Number]++
			}
			if len(g.Tiles) != 35 || !reflect.DeepEqual(terrain, tc.terrain) || !reflect.DeepEqual(numbers, tc.numbers) {
				t.Fatal("components", terrain, numbers)
			}
			sizes := map[int]int{}
			for _, id := range g.Seafarers.Islands {
				if id >= 0 {
					sizes[id]++
				}
			}
			islands := []int{}
			for _, size := range sizes {
				islands = append(islands, size)
			}
			slices.Sort(islands)
			if !reflect.DeepEqual(islands, tc.islands) {
				t.Fatal("island outlines", islands)
			}
			ports := map[int]int{}
			used := map[int]bool{}
			for _, port := range g.Ports {
				ports[port.Resource]++
				if !g.edgeTerrain(port.Edge, true) || !g.edgeTerrain(port.Edge, false) {
					t.Fatal("noncoastal port")
				}
				e := g.Edges[port.Edge]
				for _, v := range []int{e.A, e.B} {
					if used[v] {
						t.Fatal("ports share vertex")
					}
					used[v] = true
				}
			}
			if !reflect.DeepEqual(ports, map[int]int{-1: tc.generic, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}) {
				t.Fatal("ports", ports)
			}
			robber := g.Tiles[g.Robber]
			if robber.Resource != tc.robberResource || robber.Number != tc.robberNumber || g.Seafarers.Pirate != -1 || g.Seafarers.VictoryPoints != tc.victory {
				t.Fatal("starting markers or victory")
			}
			for _, v := range g.Vertices {
				if len(g.touching(v.ID)) > 3 {
					t.Fatal("nonphysical vertex")
				}
			}
			if tc.name == "shores3" {
				if len(g.Seafarers.StartIslands) != 1 || sizes[g.Seafarers.StartIslands[0]] != 14 || slices.Contains(g.Seafarers.StartIslands, g.Seafarers.Islands[g.Robber]) {
					t.Fatal("home island incorrectly determined by robber")
				}
				helperReject(t, s, s.Turn, Action{Type: "catan_settlement", Vertex: robber.Vertices[0]})
			} else if len(g.Seafarers.StartIslands) != 0 {
				t.Fatal("Four Islands has global home restriction")
			}
		})
	}
}

func TestCatanSeafarersFourIslandsPersonalHomes(t *testing.T) {
	for _, twoHomes := range []bool{false, true} {
		s := catanGame(t, 3)
		if err := s.Catan.makeSeafarersIslandsThree(); err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		vertices := map[int]int{}
		for _, v := range g.Vertices {
			if island := g.islandAt(v.ID); island >= 0 {
				vertices[island] = v.ID
			}
		}
		if len(vertices) != 4 {
			t.Fatal("fixture islands")
		}
		s.catanSettleIsland(0, vertices[0], true)
		s.catanSettleIsland(1, vertices[1], true)
		if twoHomes {
			s.catanSettleIsland(0, vertices[1], true)
		} else {
			s.catanSettleIsland(0, vertices[0], true)
		}
		wantHomes := 1
		if twoHomes {
			wantHomes = 2
		}
		if len(g.Seafarers.Seats[0].HomeIslands) != wantHomes || g.Seafarers.Seats[0].IslandPoints != 0 {
			t.Fatal("setup bonus or duplicate homes")
		}
		restored := clone(*s)
		s = &restored
		g = s.Catan
		for island, v := range vertices {
			if !g.seaSetupAllowed(v) {
				t.Fatal("Four Islands restricts setup")
			}
			s.catanSettleIsland(0, v, false)
			s.catanSettleIsland(0, v, false)
			if island == 0 {
				s.catanSettleIsland(1, v, false)
			}
		}
		if g.Seafarers.Seats[0].IslandPoints != (4-wantHomes)*2 || g.Seafarers.Seats[1].IslandPoints != 2 {
			t.Fatal("personal exploration bonus", g.Seafarers.Seats)
		}
		s.Turn = 0
		g.SetupStep = 6
		s.Phase = "catan_turn"
		g.Players[0].Score = 12
		s.catanVictory()
		if s.Finished {
			t.Fatal("ended before thirteen points")
		}
		g.Players[0].Score = 13
		s.catanVictory()
		if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) {
			t.Fatal("thirteen-point ending")
		}
	}
}

func TestCatanSeafarersVariableSetups(t *testing.T) {
	for _, tc := range []struct {
		name    string
		players int
		build   func(*Catan) error
	}{
		{"shores3", 3, (*Catan).makeSeafarersShoresThree}, {"shores4", 4, (*Catan).makeSeafarersShoresFour},
		{"islands3", 3, (*Catan).makeSeafarersIslandsThree}, {"islands4", 4, (*Catan).makeSeafarersIslandsFour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for sample := 0; sample < 50; sample++ {
				s := catanGame(t, tc.players)
				if err := tc.build(s.Catan); err != nil {
					t.Fatal(err)
				}
				before := clone(*s).Catan
				if err := s.Catan.randomizeSeafarersMap(); err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if !g.Seafarers.Variable || !reflect.DeepEqual(g.Seafarers.Islands, before.Seafarers.Islands) || !reflect.DeepEqual(g.Seafarers.StartIslands, before.Seafarers.StartIslands) || !reflect.DeepEqual(g.Edges, before.Edges) || !reflect.DeepEqual(g.Vertices, before.Vertices) {
					t.Fatal("variable setup changed board geometry or starting islands")
				}
				inventory := func(board *Catan, main bool) ([]int, []int) {
					terrain, numbers := []int{}, []int{}
					for _, tile := range board.Tiles {
						home := slices.Contains(board.Seafarers.StartIslands, board.Seafarers.Islands[tile.ID])
						if home == main {
							terrain = append(terrain, tile.Resource)
							numbers = append(numbers, tile.Number)
						}
					}
					slices.Sort(terrain)
					slices.Sort(numbers)
					return terrain, numbers
				}
				for _, main := range []bool{false, true} {
					a, b := inventory(g, main)
					c, d := inventory(before, main)
					if !reflect.DeepEqual(a, c) || !reflect.DeepEqual(b, d) {
						t.Fatal("components migrated between main and unexplored areas")
					}
				}
				ports, oldPorts := []int{}, []int{}
				for i, p := range g.Ports {
					if p.Edge != before.Ports[i].Edge {
						t.Fatal("port moved")
					}
					ports = append(ports, p.Resource)
					oldPorts = append(oldPorts, before.Ports[i].Resource)
				}
				slices.Sort(ports)
				slices.Sort(oldPorts)
				if !reflect.DeepEqual(ports, oldPorts) {
					t.Fatal("port inventory")
				}
				for i, tile := range g.Tiles {
					if (tile.Resource == CatanSea) != (before.Tiles[i].Resource == CatanSea) || ((tile.Resource == CatanSea || tile.Resource == CatanDesert) && tile.Number != 0) {
						t.Fatal("sea moved or nonproducing terrain numbered")
					}
					if g.Seafarers.Scenario == "islands" && (tile.Resource == 0 || tile.Resource == 2) && slices.Contains([]int{2, 3, 11, 12}, tile.Number) {
						t.Fatal("forest/pasture received low number")
					}
				}
				if g.Seafarers.Scenario == "shores" {
					for _, e := range g.Edges {
						if len(e.Tiles) == 2 {
							a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
							if (a == 6 || a == 8) && (b == 6 || b == 8) {
								t.Fatal("adjacent red numbers")
							}
						}
					}
				}
				if tc.name == "shores4" {
					if g.Tiles[g.Robber].Resource != CatanDesert {
						t.Fatal("robber missing desert")
					}
				} else if g.Tiles[g.Robber].Number != 12 {
					t.Fatal("robber missing twelve")
				}
				restored := clone(*s)
				if !reflect.DeepEqual(g, restored.Catan) {
					t.Fatal("variable board not preserved")
				}
			}
		})
	}
}
