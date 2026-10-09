package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func riversSeaRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.Catan.validateRivers(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	*s = next
}

func TestCatanRiversSeaPrintedShores(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := newCatanRiversShores(n)
			if err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			if s.Phase != "catan_rivers_start" || g.Robber != -1 || g.Seafarers.VictoryPoints != 14 {
				t.Fatal("setup/target")
			}
			want := [][]int{{15, 21, 27, 32}, {11, 17, 23}}
			double := 11
			if n == 4 {
				want = [][]int{{21, 20, 19, 18}, {33, 32, 31}}
				double = 28
			}
			if g.Rivers.Map.DoubleNumberTile != double || g.Tiles[double].Number != 12 {
				t.Fatal("double token")
			}
			if len(g.Rivers.Map.Bridges) != 7 || len(g.Rivers.Map.Swamps) != 2 {
				t.Fatal("river inventory")
			}
			for i, channel := range g.Rivers.Map.Channels {
				if !slices.Equal(channel.Tiles, want[i]) {
					t.Fatal("river path", channel)
				}
				if g.Tiles[channel.Tiles[0]].Resource != 4 || g.Tiles[channel.Tiles[len(channel.Tiles)-1]].Resource != catanSwamp {
					t.Fatal("source/mouth")
				}
				sea := false
				for _, id := range g.Edges[channel.Outlet].Tiles {
					sea = sea || g.Tiles[id].Resource == CatanSea
				}
				// A river can also empty directly into the printed frame.
				if !sea && len(g.Edges[channel.Outlet].Tiles) != 1 {
					t.Fatal("outlet does not reach sea", channel)
				}
			}
			if n == 3 && (g.Tiles[15].Number != 5 || g.Tiles[16].Resource != 2 || g.Tiles[16].Number != 4) {
				t.Fatal("western main island")
			}
			for _, port := range g.Ports {
				if port.Edge < 0 {
					t.Fatal("missing port edge")
				}
			}
			for _, id := range g.Rivers.Map.Bridges {
				if g.edgeTerrain(id, true) || g.edgeTerrain(id, false) {
					t.Fatal("bridge accepts ship/road")
				}
			}
			riversSeaRestore(t, s)
			for name, mutate := range map[string]func(*Catan){
				"marker":         func(g *Catan) { g.Rivers.Sea = "unknown" },
				"missing sea":    func(g *Catan) { g.Seafarers = nil },
				"sea version":    func(g *Catan) { g.Seafarers.Rules = "unknown" },
				"foreign fog":    func(g *Catan) { g.Seafarers.Fog = &CatanFogState{} },
				"foreign layout": func(g *Catan) { g.Seafarers.Variable = true },
				"pirate on land": func(g *Catan) { g.Seafarers.Pirate = double },
				"missing seats":  func(g *Catan) { g.Seafarers.Seats = nil },
				"terrain":        func(g *Catan) { g.Tiles[double].Resource = 1 },
				"number":         func(g *Catan) { g.Tiles[double].Number = 4 },
				"port":           func(g *Catan) { g.Ports[0].Resource = 0 },
				"river":          func(g *Catan) { g.Rivers.Map.Channels[0].Tiles[0] = 0 },
				"bridge":         func(g *Catan) { g.Rivers.Map.Bridges[0] = 0 },
				"start island":   func(g *Catan) { g.Seafarers.StartIslands = nil },
				"victory":        func(g *Catan) { g.Seafarers.VictoryPoints = 10 },
			} {
				broken := clone(*s)
				mutate(broken.Catan)
				if broken.Catan.validateRivers() == nil {
					t.Fatal("accepted corrupt", name)
				}
			}
		})
	}
	for _, n := range []int{1, 2, 7} {
		if _, err := newCatanRiversShores(n); err == nil {
			t.Fatal("unsupported map admitted", n)
		}
	}
}

func TestCatanRiversSeaNaturalEngine(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanRiversShores(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
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
					if step%83 == 0 {
						riversSeaRestore(t, s)
						for viewer := -1; viewer < n; viewer++ {
							s.View(viewer)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				riversSeaRestore(t, s)
				t.Log("round", s.Round, "winners", s.Winners)
			})
		}
	}
}

func riversSeaRouteFixture(t *testing.T, river bool) (*State, int) {
	t.Helper()
	s, err := newCatanRiversShores(3)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	g.Robber = g.Rivers.Map.Swamps[0]
	s.Phase = "catan_turn"
	s.Turn = 0
	for _, e := range g.Edges {
		if g.riverEdge(e.ID) != river || !g.edgeTerrain(e.ID, true) || g.pirateBlocks(e.ID) {
			continue
		}
		for _, v := range []int{e.A, e.B} {
			if !g.landVertex(v) {
				continue
			}
			g.Vertices[v].Owner = 0
			g.Vertices[v].Level = 1
			if g.canShip(0, e.ID) {
				return s, e.ID
			}
			g.Vertices[v].Owner = -1
			g.Vertices[v].Level = 0
		}
	}
	t.Fatal("no coastal fixture")
	return nil, -1
}

func TestCatanRiversSeaShipBuildAndMove(t *testing.T) {
	for _, free := range []bool{false, true} {
		t.Run(fmt.Sprint("free", free), func(t *testing.T) {
			s, id := riversSeaRouteFixture(t, true)
			g := s.Catan
			a := Action{Type: "catan_ship", Edge: id}
			if free {
				s.Phase = "catan_roads"
				g.FreeRoads = 1
				g.ResumePhase = "catan_turn"
			} else {
				catanGive(g, 0, 0, 1)
				catanGive(g, 0, 2, 1)
			}
			if err := s.Apply(0, a); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Rivers.Gold[0] != 1 || s.Catan.Rivers.Bank != 99 {
				t.Fatal("ship reward")
			}
			riversSeaRestore(t, s)
		})
	}
	// Prepare ships and a second coastal settlement, ensuring both the source
	// and destination use ordinary ship legality instead of bypassing movement.
	for _, fromRiver := range []bool{false, true} {
		for _, toRiver := range []bool{false, true} {
			t.Run(fmt.Sprintf("move%t-%t", fromRiver, toRiver), func(t *testing.T) {
				s, from := riversSeaRouteFixture(t, fromRiver)
				g := s.Catan
				g.Edges[from].Owner = 0
				g.Edges[from].Ship = true
				riverGold(g, 0, 1)
				target := -1
				for _, e := range g.Edges {
					if e.ID == from || g.riverEdge(e.ID) != toRiver || !g.edgeTerrain(e.ID, true) || g.pirateBlocks(e.ID) {
						continue
					}
					for _, v := range []int{e.A, e.B} {
						if !g.landVertex(v) || g.Vertices[v].Owner != -1 {
							continue
						}
						g.Vertices[v].Owner = 0
						g.Vertices[v].Level = 1
						if slices.Contains(g.shipDestinations(0, from), e.ID) {
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
					t.Fatal("missing destination")
				}
				if fromRiver {
					riverGold(g, 0, 0)
					before := clone(*s)
					if err := s.Apply(0, Action{Type: "catan_move_ship", Edge: from, Target: target}); err == nil || !reflect.DeepEqual(*s, before) {
						t.Fatal("free/partial river departure")
					}
					g = s.Catan
					riverGold(g, 0, 1)
				}
				if err := s.Apply(0, Action{Type: "catan_move_ship", Edge: from, Target: target}); err != nil {
					t.Fatal(err)
				}
				want := 1
				if fromRiver {
					want--
				}
				if toRiver {
					want++
				}
				if s.Catan.Rivers.Gold[0] != want || s.Catan.Rivers.Bank != 100-want {
					t.Fatal("move ledger", s.Catan.Rivers)
				}
				if s.Catan.Edges[from].Owner != -1 || !s.Catan.Edges[target].Ship {
					t.Fatal("ship move")
				}
				riversSeaRestore(t, s)
			})
		}
	}
}

func TestCatanRiversSeaSetupShipAndProduction(t *testing.T) {
	s, id := riversSeaRouteFixture(t, true)
	g := s.Catan
	g.SetupStep = 0
	g.StartPlayer = 0
	s.Phase = "catan_setup_road"
	e := g.Edges[id]
	g.SetupVertex = e.A
	if g.Vertices[e.A].Owner != 0 {
		g.SetupVertex = e.B
	}
	if err := s.Apply(0, Action{Type: "catan_ship", Edge: id}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.Gold[0] != 1 || s.Catan.SetupStep != 1 {
		t.Fatal("setup ship reward/advance")
	}
	riversSeaRestore(t, s)
	for _, number := range []int{2, 12} {
		next, err := newCatanRiversShores(4)
		if err != nil {
			t.Fatal(err)
		}
		g = next.Catan
		g.SetupStep = g.SetupLimit()
		g.Robber = g.Rivers.Map.Swamps[0]
		next.Phase = "catan_turn"
		tile := g.Tiles[g.Rivers.Map.DoubleNumberTile]
		g.Vertices[tile.Vertices[0]].Owner = 0
		g.Vertices[tile.Vertices[0]].Level = 2
		if err = next.catanRollProduction(number); err != nil {
			t.Fatal(err)
		}
		if g.Players[0].Resources[tile.Resource] != 2 || g.Rivers.Gold[0] != 0 {
			t.Fatal("double number produces resources", number)
		}
		riversSeaRestore(t, next)
	}
	s, err := newCatanRiversShores(3)
	if err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	g.SetupStep = g.SetupLimit()
	g.Robber = g.Rivers.Map.Swamps[0]
	s.Phase = "catan_turn"
	tile := g.Tiles[1]
	if tile.Resource != CatanGold {
		t.Fatal("gold island fixture")
	}
	g.Vertices[tile.Vertices[0]].Owner = 0
	g.Vertices[tile.Vertices[0]].Level = 2
	if err = s.catanRollProduction(tile.Number); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_gold" || g.GoldPending.Claims[0].Count != 2 || g.Rivers.Gold[0] != 0 {
		t.Fatal("gold field incorrectly minted coins")
	}
	riversSeaRestore(t, s)
	if err = s.Apply(0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 1}}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.Gold[0] != 0 || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[0].Resources[4] != 1 {
		t.Fatal("gold choice")
	}
	riverConserved(t, s)
}

func TestCatanRiversSeaShipIssuedGoldAndAtomicFailure(t *testing.T) {
	s, id := riversSeaRouteFixture(t, true)
	g := s.Catan
	riverGold(g, 1, 100)
	catanGive(g, 0, 0, 1)
	catanGive(g, 0, 2, 1)
	if err := s.Apply(0, Action{Type: "catan_ship", Edge: id}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.GoldIssued != 1 || s.Catan.Rivers.Gold[0] != 1 || s.Catan.Rivers.Bank != 0 {
		t.Fatal("issued ship reward")
	}
	riversSeaRestore(t, s)
	for _, bridge := range s.Catan.Rivers.Map.Bridges {
		riverReject(t, s, 0, Action{Type: "catan_ship", Edge: bridge})
	}
	// A failed reward at the ledger ceiling must roll back the ship, its
	// resource payment and the ordinary turn state together.
	s, id = riversSeaRouteFixture(t, true)
	g = s.Catan
	g.Rivers.Bank = 0
	g.Rivers.GoldIssued = catanGoldLedgerLimit
	g.Rivers.Gold[1] = 100 + catanGoldLedgerLimit
	catanGive(g, 0, 0, 1)
	catanGive(g, 0, 2, 1)
	riverReject(t, s, 0, Action{Type: "catan_ship", Edge: id})
	riverConserved(t, s)
}
