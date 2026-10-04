package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func newWorldGame(t *testing.T, n int, helpers bool) *State {
	t.Helper()
	s, err := NewCatanNewWorld(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanNewWorldPrintedInventoriesAndSetup(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for repeat := 0; repeat < 40; repeat++ {
			s := newWorldGame(t, n, true)
			g := s.Catan
			terrain, numbers := make([]int, 9), make([]int, 13)
			for _, tile := range g.Tiles {
				terrain[tile.Resource]++
				numbers[tile.Number]++
			}
			wantTerrain := []int{5, 4, 5, 5, 4, 0, 19, 0, 0}
			wantNumbers := []int{19, 0, 1, 3, 3, 3, 2, 0, 2, 3, 3, 2, 1}
			wantPorts := []int{5, 1, 1, 1, 1, 1}
			if n > 4 {
				wantTerrain = []int{7, 7, 7, 7, 7, 3, 21, 4, 0}
				wantNumbers = []int{24, 0, 2, 3, 4, 5, 5, 0, 5, 5, 4, 4, 2}
				wantPorts = []int{5, 1, 1, 2, 1, 1}
			}
			if !slices.Equal(terrain, wantTerrain) || !slices.Equal(numbers, wantNumbers) {
				t.Fatal(n, "printed inventory", terrain, numbers)
			}
			if g.Robber != -1 || g.Seafarers.Pirate != -1 || g.Seafarers.VictoryPoints != 12 || g.Seafarers.IslandBonus != 1 || len(g.Seafarers.StartIslands) > 0 {
				t.Fatal("initial piece positions or rules")
			}
			for _, tile := range g.Tiles {
				if tile.Resource == CatanGold && (tile.Number == 6 || tile.Number == 8) {
					t.Fatal("red on gold")
				}
			}
			for _, e := range g.Edges {
				if len(e.Tiles) == 2 {
					a, b := g.Tiles[e.Tiles[0]].Number, g.Tiles[e.Tiles[1]].Number
					if (a == 6 || a == 8) && (b == 6 || b == 8) {
						t.Fatal("adjacent reds")
					}
				}
			}
			first := s.Turn
			for index := 0; index < len(g.newWorld().Ports); index++ {
				if s.Turn != (first+index)%n || s.Phase != "catan_world_ports" || s.CatanPendingActor() != s.Turn {
					t.Fatal("port order")
				}
				legal := g.worldPortEdges()
				if len(legal) == 0 {
					t.Fatal("port placement stranded", n, index)
				}
				for _, edge := range legal {
					e := g.Edges[edge]
					land, sea := false, len(e.Tiles) == 1
					for _, id := range e.Tiles {
						if g.Tiles[id].Resource == CatanSea {
							sea = true
						} else {
							land = true
						}
					}
					if !land || !sea {
						t.Fatal("not coast")
					}
				}
				// Timeouts must place exactly one port; do not also build settlements.
				s.AutoCatanPending()
				g = s.Catan
				if g.newWorld().Index != index+1 || g.SetupStep != 0 || len(g.Ports) != index+1 {
					t.Fatal("timeout advanced wrong step")
				}
				for _, p := range g.Players {
					if sum(p.Resources) > 0 || p.Helper != nil {
						t.Fatal("port step granted resources/helpers")
					}
				}
				for _, v := range g.Vertices {
					if v.Level > 0 {
						t.Fatal("port step built settlement")
					}
				}
			}
			if s.Turn != first || s.Phase != "catan_setup_settlement" {
				t.Fatal("not returned to first settler")
			}
			counts := make([]int, 6)
			ends := map[int]bool{}
			for _, p := range g.Ports {
				counts[p.Resource+1]++
				e := g.Edges[p.Edge]
				if ends[e.A] || ends[e.B] {
					t.Fatal("ports touch")
				}
				ends[e.A], ends[e.B] = true, true
			}
			if !slices.Equal(counts, wantPorts) {
				t.Fatal("printed ports", counts)
			}
		}
	}
}

func TestCatanNewWorldPortPrivacyAndInvalidActions(t *testing.T) {
	s := newWorldGame(t, 6, false)
	g := s.Catan
	edge := g.worldPortEdges()[0]
	actor := s.Turn
	for _, viewer := range []int{-1, (actor + 1) % 6, actor} {
		v := s.View(viewer)["catan"].(map[string]any)
		w := v["seafarers"].(map[string]any)["newWorld"].(map[string]any)
		if _, leaked := w["ports"]; leaked {
			t.Fatal("hidden deck exposed")
		}
		if w["current"] != g.newWorld().Ports[0] || w["remaining"] != 11 {
			t.Fatal("current port unavailable")
		}
		legal := v["legal"].(map[string][]int)
		if (len(legal["ports"]) > 0) != (viewer == actor) || len(legal["settlements"]) > 0 {
			t.Fatal("wrong legal hints")
		}
	}
	before, _ := json.Marshal(s.View(actor))
	slices.Reverse(g.newWorld().Ports[1:])
	after, _ := json.Marshal(s.View(actor))
	if string(before) != string(after) {
		t.Fatal("view reveals future ports")
	}
	helperReject(t, s, (actor+1)%6, Action{Type: "catan_world_port", Edge: edge})
	helperReject(t, s, actor, Action{Type: "catan_world_port", Edge: -1})
	helperReject(t, s, actor, Action{Type: "catan_settlement", Vertex: 0})
	helperReject(t, s, actor, Action{Type: "catan_roll"})
	helperApply(t, s, actor, Action{Type: "catan_world_port", Edge: edge})
	helperReject(t, s, s.Turn, Action{Type: "catan_world_port", Edge: edge})
	e := s.Catan.Edges[edge]
	for _, other := range s.Catan.Edges {
		if other.ID != edge && (other.A == e.A || other.B == e.A || other.A == e.B || other.B == e.B) {
			helperReject(t, s, s.Turn, Action{Type: "catan_world_port", Edge: other.ID})
			break
		}
	}
	saved := clone(*s)
	if !reflect.DeepEqual(*s, saved) {
		t.Fatal("port stage restore")
	}
	s = &saved
	for s.Phase == "catan_world_ports" {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, s.Turn, a)
	}
	helperReject(t, s, s.Turn, Action{Type: "catan_world_port", Edge: edge})
	v := s.View(-1)["catan"].(map[string]any)["seafarers"].(map[string]any)["newWorld"].(map[string]any)
	if _, present := v["current"]; present || v["remaining"] != 0 {
		t.Fatal("finished port stack still current")
	}
}

func TestCatanNewWorldPersonalIslandsAndVictory(t *testing.T) {
	s := newWorldGame(t, 4, false)
	g := s.Catan
	// An explicit three-island scoring fixture avoids assuming random maps
	// have a minimum number of islands (the rules impose no such constraint).
	if err := g.makeScenarioMap([]CatanHexSpec{{Q: 0, R: 0, Resource: 0, Number: 5}, {Q: 3, R: 0, Resource: 1, Number: 9}, {Q: 6, R: 0, Resource: 2, Number: 4}}); err != nil {
		t.Fatal(err)
	}
	g.Seafarers.Islands = g.findIslands()
	a, b, c := g.Tiles[0].Vertices[0], g.Tiles[1].Vertices[0], g.Tiles[2].Vertices[0]
	s.catanSettleIsland(0, a, true)
	s.catanSettleIsland(0, b, true)
	s.catanSettleIsland(1, a, true)
	s.catanSettleIsland(0, b, false)
	if len(g.Seafarers.Seats[0].HomeIslands) != 2 || g.Seafarers.Seats[0].IslandPoints != 0 {
		t.Fatal("home island bonus")
	}
	s.catanSettleIsland(0, c, false)
	s.catanSettleIsland(1, c, false)
	s.catanSettleIsland(0, c, false)
	if g.Seafarers.Seats[0].IslandPoints != 1 || g.Seafarers.Seats[1].IslandPoints != 1 {
		t.Fatal("personal exploration reward")
	}
	g.SetupStep = g.SetupLimit()
	s.Turn = 0
	s.Phase = "catan_turn"
	g.Players[0].Score = 11
	s.catanVictory()
	if s.Finished {
		t.Fatal("won below twelve")
	}
	g.Players[0].Score = 12
	s.Turn = 1
	s.catanVictory()
	if s.Finished {
		t.Fatal("off-turn victory")
	}
	s.Turn = 0
	s.catanVictory()
	if !s.Finished || !slices.Equal(s.Winners, []int{0}) {
		t.Fatal("twelve points did not win")
	}
}

func TestCatanNewWorldBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, helpers := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/helpers=%v", n, helpers), func(t *testing.T) {
				s := newWorldGame(t, n, helpers)
				initialDev := len(s.Catan.DevDeck)
				steps := 0
				for ; steps < 10000 && !s.Finished; steps++ {
					actor := s.Turn
					if p := s.CatanPendingActor(); p >= 0 {
						actor = p
					} else if s.Phase == "catan_discard" {
						for p, due := range s.Catan.DiscardDue {
							if due > 0 {
								actor = p
								break
							}
						}
					}
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(steps, s.Phase, err)
					}
					helperApply(t, s, actor, a)
					g := s.Catan
					fleetSupply(t, g)
					dev := len(g.DevDeck) + len(g.DevDiscard) + len(g.HelperExile)
					if g.HelperPending != nil {
						dev += len(g.HelperPending.Cards)
					}
					for p, seat := range g.Players {
						dev += sum(seat.Dev)
						r, v, c := g.pieces(p)
						if r > 15 || v > 5 || c > 4 || g.shipCount(p) > 15 {
							t.Fatal("piece supply")
						}
					}
					if dev != initialDev {
						t.Fatal("development inventory")
					}
					for _, e := range g.Edges {
						if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
							t.Fatal("adjacent buildings")
						}
					}
					if steps%29 == 0 {
						saved := clone(*s)
						if !reflect.DeepEqual(*s, saved) {
							t.Fatal("roundtrip")
						}
						s = &saved
					}
				}
				if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 12 {
					t.Fatal("game failed to finish", steps, s.Phase)
				}
				t.Logf("steps=%d rounds=%d winner=%d", steps, s.Round, s.Winners[0])
			})
		}
	}
}
