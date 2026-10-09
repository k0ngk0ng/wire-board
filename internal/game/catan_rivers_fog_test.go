package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func riverFogFixture(t *testing.T, n int) *State {
	t.Helper()
	var s *State
	var err error
	if n > 4 {
		s, err = newCatanRiversFogExtended(n)
	} else {
		s, err = newCatanRiversPrintedSea(n, "fog")
	}
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCatanRiversFogPrintedMaps(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riverFogFixture(t, n)
			g := s.Catan
			if g.Seafarers.VictoryPoints != 12 || g.Seafarers.IslandBonus != 0 || g.Robber != -1 || s.Phase != "catan_rivers_start" {
				t.Fatal("setup")
			}
			if len(g.Rivers.Map.Bridges) != 7 || len(g.Rivers.Map.Swamps) != 2 || len(g.Seafarers.Fog.Terrain) != 12 {
				t.Fatal("components")
			}
			paths := [][]int{{18, 25, 32, 38}, {4, 10, 17}}
			if n == 4 {
				paths = [][]int{{3, 9, 16, 23}, {25, 32, 38}}
			}
			for i, c := range g.Rivers.Map.Channels {
				if !slices.Equal(c.Tiles, paths[i]) {
					t.Fatal("river path")
				}
				e := g.Edges[c.Outlet]
				sea := len(e.Tiles) == 1
				for _, id := range e.Tiles {
					sea = sea || g.Tiles[id].Resource == CatanSea
				}
				if !sea {
					t.Fatal("mouth not on coast", c)
				}
			}
			for _, id := range g.Rivers.Map.Bridges {
				if g.edgeTerrain(id, true) || g.edgeTerrain(id, false) {
					t.Fatal("bridge accepts route")
				}
			}
			riversSeaRestore(t, s)
			for name, mutate := range map[string]func(*Catan){
				"initial number": func(g *Catan) { g.Tiles[3].Number = 2 },
				"extra token": func(g *Catan) {
					g.Rivers.Map.ExtraNumbers = append(g.Rivers.Map.ExtraNumbers, catanFishingExtraNumber{Tile: 0, Number: 6})
				},
				"terrain pile":             func(g *Catan) { g.Seafarers.Fog.Terrain[0] = catanSwamp },
				"number pile":              func(g *Catan) { g.Seafarers.Fog.Numbers[0] = 7 },
				"missing pile":             func(g *Catan) { g.Seafarers.Fog.Terrain = g.Seafarers.Fog.Terrain[1:] },
				"robber at fog":            func(g *Catan) { g.Robber = 0 },
				"start islands":            func(g *Catan) { g.Seafarers.Fog.StartTiles = nil },
				"revealed wrong inventory": func(g *Catan) { g.Tiles[0].Resource = CatanGold; g.Tiles[0].Number = 6 },
			} {
				b := clone(*s)
				mutate(b.Catan)
				if b.Catan.validateRivers() == nil {
					t.Fatal("accepted", name)
				}
			}
			for viewer := -1; viewer < n; viewer++ {
				fog := s.View(viewer)["catan"].(map[string]any)["seafarers"].(map[string]any)["fog"].(map[string]any)
				if _, ok := fog["terrain"]; ok {
					t.Fatal("terrain leaked")
				}
				if _, ok := fog["numbers"]; ok {
					t.Fatal("numbers leaked")
				}
			}
		})
	}
}
func TestCatanRiversFogNaturalEngine(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s := riverFogFixture(t, n)
				if events {
					if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 16000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%71 == 0 {
						riversSeaRestore(t, s)
						riverConserved(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				riversSeaRestore(t, s)
				t.Log("round", s.Round, "remaining fog", len(s.Catan.Seafarers.Fog.Terrain))
			})
		}
	}
}
func TestCatanRiversFogDoubleProduction(t *testing.T) {
	for _, row := range [][4]int{{3, 25, 2, 11}, {3, 10, 3, 12}, {4, 10, 2, 12}} {
		for _, roll := range row[2:] {
			for _, blocked := range []bool{false, true} {
				s := riverFogFixture(t, row[0])
				g := s.Catan
				g.SetupStep = g.SetupLimit()
				s.Phase = "catan_turn"
				g.Robber = g.Rivers.Map.Swamps[0]
				tile := g.Tiles[row[1]]
				g.Vertices[tile.Vertices[0]].Owner = 0
				g.Vertices[tile.Vertices[0]].Level = 2
				if blocked {
					g.Robber = tile.ID
				}
				if err := s.catanRollProduction(roll); err != nil {
					t.Fatal(err)
				}
				want := 2
				if blocked {
					want = 0
				}
				if g.Players[0].Resources[tile.Resource] != want || g.Rivers.Gold[0] != 0 {
					t.Fatal("production", row, roll, blocked)
				}
				if g.tileNumberWeight(tile) != (6-absCatan(7-row[2]))+(6-absCatan(7-row[3])) {
					t.Fatal("bot number weight")
				}
				riversSeaRestore(t, s)
			}
		}
	}
}

func TestCatanRiversFogDiscoveryContinuation(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, phase := range []string{"catan_turn", "catan_roads", "catan_setup_road"} {
			for _, ship := range []bool{false, true} {
				if !ship && phase == "catan_setup_road" {
					continue
				} // Initial islands have no road edge touching fog.
				t.Run(fmt.Sprintf("%d/%s/ship%t", n, phase, ship), func(t *testing.T) {
					s := riverFogFixture(t, n)
					g := s.Catan
					g.SetupStep = g.SetupLimit()
					g.Robber = g.Rivers.Map.Swamps[0]
					s.Turn = 0
					s.Phase = phase
					if phase == "catan_roads" {
						g.FreeRoads = 2
						g.ResumePhase = "catan_turn"
					}
					if phase == "catan_setup_road" {
						g.SetupStep = 0
						g.StartPlayer = 0
					}
					if !ship {
						// A prior expedition revealed this land on the fog frontier.
						// Consume its components so the saved inventory remains valid.
						fog := g.Seafarers.Fog
						at := slices.Index(fog.Terrain, 0)
						id := 0
						for g.Tiles[id].Resource != CatanFog {
							id++
						}
						g.Tiles[id].Resource = fog.Terrain[at]
						fog.Terrain = slices.Delete(fog.Terrain, at, at+1)
						g.Tiles[id].Number = fog.Numbers[len(fog.Numbers)-1]
						fog.Numbers = fog.Numbers[:len(fog.Numbers)-1]
						g.Seafarers.Islands = g.findIslands()
					}
					edge := -1
					for _, e := range g.Edges {
						if len(g.fogAtRoute(e.ID)) == 0 || !g.edgeTerrain(e.ID, ship) || ship && g.pirateBlocks(e.ID) {
							continue
						}
						for _, v := range []int{e.A, e.B} {
							if !g.landVertex(v) {
								continue
							}
							g.Vertices[v].Owner = 0
							g.Vertices[v].Level = 1
							g.SetupVertex = v
							if g.canRoute(0, e.ID, ship) {
								edge = e.ID
								break
							}
							g.Vertices[v].Owner = -1
							g.Vertices[v].Level = 0
						}
						if edge >= 0 {
							break
						}
					}
					if edge < 0 {
						t.Fatal("discovery route missing")
					}
					pile := g.Seafarers.Fog.Terrain
					for i := len(pile) - 1; i >= 0; i-- {
						if pile[i] == CatanGold {
							pile[i], pile[len(pile)-1] = pile[len(pile)-1], pile[i]
							break
						}
					}
					a := Action{Type: "catan_road", Edge: edge}
					if ship {
						a.Type = "catan_ship"
					}
					if phase == "catan_turn" {
						helperGrant(s, 0, catanPrices[a.Type])
					}
					priorGold := g.Rivers.Gold[0]
					river := g.riverEdge(edge)
					if err := s.Apply(0, a); err != nil {
						t.Fatal(err)
					}
					g = s.Catan
					if s.Phase != "catan_gold" || g.GoldPending.AfterRoute == nil || g.GoldPending.AfterRoute.Edge != edge {
						t.Fatal("gold continuation")
					}
					if river {
						priorGold++
					}
					if g.Rivers.Gold[0] != priorGold {
						t.Fatal("river reward repeated/missing")
					}
					riversSeaRestore(t, s)
					for s.Phase == "catan_gold" {
						p := s.CatanPendingActor()
						a, err := s.BotAction(p)
						if err != nil {
							t.Fatal(err)
						}
						if err = s.Apply(p, a); err != nil {
							t.Fatal(err)
						}
					}
					g = s.Catan
					if g.Rivers.Gold[0] != priorGold || g.GoldPending != nil {
						t.Fatal("gold choice changed coins")
					}
					if phase == "catan_setup_road" && g.SetupStep != 1 {
						t.Fatal("setup not resumed")
					}
					if phase == "catan_roads" && (g.FreeRoads != 1 || s.Phase != "catan_roads") {
						t.Fatal("free road not resumed")
					}
					if phase == "catan_turn" && s.Phase != phase {
						t.Fatal("turn not resumed")
					}
					riversSeaRestore(t, s)
					riverConserved(t, s)
					before, _ := json.Marshal(s)
					s.View(0)
					after, _ := json.Marshal(s)
					if !reflect.DeepEqual(before, after) {
						t.Fatal("view mutated fog")
					}
				})
			}
		}
	}
}

func TestCatanRiversFogHiddenOrderPrivacyAndAtomicCorruption(t *testing.T) {
	s := riverFogFixture(t, 3)
	next := clone(*s)
	slices.Reverse(next.Catan.Seafarers.Fog.Terrain)
	slices.Reverse(next.Catan.Seafarers.Fog.Numbers)
	for viewer := -1; viewer < 3; viewer++ {
		a, _ := json.Marshal(s.View(viewer))
		b, _ := json.Marshal(next.View(viewer))
		if string(a) != string(b) {
			t.Fatal("hidden order leaked")
		}
	}
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	b, err := next.BotAction(next.Turn)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("bot consulted hidden order", err)
	}
	next.Catan.Seafarers.Fog.Numbers[0] = 7
	riverReject(t, &next, next.Turn, b)
	// Extra river discs cannot silently enter existing ordinary saves.
	base, err := NewCatanRivers(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	base.Catan.Rivers.Map.ExtraNumbers = []catanFishingExtraNumber{{Tile: 0, Number: 6}}
	if base.Catan.validateRivers() == nil {
		t.Fatal("base river accepted sea discs")
	}
}

func TestCatanRiversFogFullDiscoveryAndDamagedSave(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		s := riverFogFixture(t, n)
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		g.Robber = g.Rivers.Map.Swamps[0]
		s.Phase = "catan_turn"
		expectedRevealed := len(g.Seafarers.Fog.Terrain)
		revealed := 0
		for _, e := range g.Edges {
			before := len(g.Seafarers.Fog.Terrain)
			if _, err := s.catanDiscover(0, e.ID); err != nil {
				t.Fatal(err)
			}
			revealed += before - len(g.Seafarers.Fog.Terrain)
			if err := g.validateRivers(); err != nil {
				t.Fatal("revealed", revealed, err)
			}
		}
		if revealed != expectedRevealed || len(g.Seafarers.Fog.Terrain) != 0 || len(g.Seafarers.Fog.Numbers) != 0 {
			t.Fatal("full discovery inventory")
		}
		riversSeaRestore(t, s)
		g = s.Catan
		var id int
		for _, tile := range g.Tiles {
			if !slices.Contains(g.Seafarers.Fog.StartTiles, tile.ID) && tile.Resource == CatanGold {
				id = tile.ID
				break
			}
		}
		g.Tiles[id].Number = 7
		if g.validateRivers() == nil {
			t.Fatal("invalid revealed number accepted")
		}
	}
}

func TestCatanRiversFogMoveShipDiscovery(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		s := riverFogFixture(t, n)
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		g.Robber = g.Rivers.Map.Swamps[0]
		s.Turn = 0
		s.Phase = "catan_turn"
		source, target := -1, -1
		// Use two coastal settlements, one anchoring the original open ship and
		// one anchoring its destination beside fog, all under ordinary legality.
		for _, e := range g.Edges {
			if !g.edgeTerrain(e.ID, true) || g.pirateBlocks(e.ID) || len(g.fogAtRoute(e.ID)) > 0 {
				continue
			}
			for _, v := range []int{e.A, e.B} {
				if g.landVertex(v) {
					g.Vertices[v].Owner = 0
					g.Vertices[v].Level = 1
					source = e.ID
					break
				}
			}
			if source >= 0 {
				break
			}
		}
		if source < 0 {
			t.Fatal("no source")
		}
		g.Edges[source].Owner = 0
		g.Edges[source].Ship = true
		riverGold(g, 0, 1)
		for _, e := range g.Edges {
			if e.ID == source || !g.edgeTerrain(e.ID, true) || g.pirateBlocks(e.ID) || len(g.fogAtRoute(e.ID)) == 0 {
				continue
			}
			for _, v := range []int{e.A, e.B} {
				if !g.landVertex(v) || g.Vertices[v].Owner != -1 {
					continue
				}
				g.Vertices[v].Owner = 0
				g.Vertices[v].Level = 1
				if slices.Contains(g.shipDestinations(0, source), e.ID) {
					target = e.ID
					break
				}
				g.Vertices[v].Owner = -1
				g.Vertices[v].Level = 0
			}
			if target >= 0 {
				break
			}
		}
		if target < 0 {
			t.Fatal("no fog destination")
		}
		gold := 1
		if g.riverEdge(source) {
			gold--
		}
		if g.riverEdge(target) {
			gold++
		}
		pile := g.Seafarers.Fog.Terrain
		at := slices.Index(pile, CatanGold)
		pile[at], pile[len(pile)-1] = pile[len(pile)-1], pile[at]
		if err := s.Apply(0, Action{Type: "catan_move_ship", Edge: source, Target: target}); err != nil {
			t.Fatal(err)
		}
		if s.Phase != "catan_gold" || !s.Catan.Seafarers.MovedShip || s.Catan.Rivers.Gold[0] != gold {
			t.Fatal("move discovery/ledger")
		}
		riversSeaRestore(t, s)
		for s.Phase == "catan_gold" {
			p := s.CatanPendingActor()
			a, err := s.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Apply(p, a); err != nil {
				t.Fatal(err)
			}
		}
		if s.Phase != "catan_turn" || !s.Catan.Seafarers.MovedShip || s.Catan.Rivers.Gold[0] != gold {
			t.Fatal("move resumed incorrectly")
		}
		riverReject(t, s, 0, Action{Type: "catan_move_ship", Edge: target, Target: source})
		riversSeaRestore(t, s)
	}
}
