package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishTribeGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanFishingSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "tribe", Layout: "fixed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCatanFishingTribePrintedMapAndRejection(t *testing.T) {
	for _, n := range []int{3, 4} {
		for range 12 {
			s := fishTribeGame(t, n)
			g := s.Catan
			if err := g.validateFishing(); err != nil {
				t.Fatal(err)
			}
			if g.Tiles[24].Resource != catanLake || g.Tiles[24].Number != 0 || g.Robber != 47 || g.Seafarers.Pirate != 2 || g.victoryTargetFor(0) != 13 {
				t.Fatal("printed setup")
			}
			for _, c := range g.FishingCoasts() {
				if c.Island != g.Seafarers.StartIslands[0] {
					t.Fatal("outer island ground offered")
				}
			}
			for _, e := range g.Edges {
				if slices.Contains(g.edgeTiles(e.ID), 24) {
					if len(g.edgeTiles(e.ID)) != 2 {
						t.Fatal("lake not enclosed")
					}
					for _, tile := range g.edgeTiles(e.ID) {
						if g.Tiles[tile].Resource == CatanSea {
							t.Fatal("lake touches sea")
						}
					}
				}
			}
			restored := clone(*s)
			if !reflect.DeepEqual(restored, *s) || restored.Catan.validateFishing() != nil {
				t.Fatal("restore")
			}
			before := clone(*s)
			if _, err := g.makeFishingTribe(nil); err == nil || !reflect.DeepEqual(before, *s) {
				t.Fatal("double transform")
			}
			raw, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "tribe"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			old := clone(raw.Catan.Tiles)
			placements := []CatanFishingGroundPlacement{}
			for i, p := range g.Fishing.Map.Grounds {
				placements = append(placements, CatanFishingGroundPlacement{g.Fishing.Map.Grounds[(i+1)%6].Number, [2]int{p.Edges[1], p.Edges[0]}})
			}
			if _, err = raw.Catan.makeFishingTribe(placements); err != nil {
				t.Fatal(err)
			}
			for i, tile := range raw.Catan.Tiles {
				if i != 24 && !reflect.DeepEqual(tile, old[i]) {
					t.Fatal("changed unrelated hex")
				}
			}
			assertTribeInventory(t, g)
		}
	}
	for _, change := range []func(*Catan){
		func(g *Catan) { g.Fishing.Map.Lakes[0].Tile = -1 },
		func(g *Catan) { g.Fishing.Map.Lakes[0].Numbers = []int{4, 10} },
		func(g *Catan) { g.Fishing.Map.ExtraNumbers = []catanFishingExtraNumber{{Tile: 24, Number: 12}} },
		func(g *Catan) { g.Tiles[24].Number = 12 },
		func(g *Catan) { g.Fishing.Map.Grounds[0] = g.Fishing.Map.Grounds[1] },
		func(g *Catan) {
			g.Ports = append(g.Ports, CatanPort{Edge: g.Fishing.Map.Grounds[0].Edges[0], Resource: -1})
		},
		func(g *Catan) { g.Seafarers.StartIslands[0] = 0 },
		func(g *Catan) { g.Robber = 0 },
	} {
		s := fishTribeGame(t, 3)
		change(s.Catan)
		if s.Catan.validateFishing() == nil {
			t.Fatal("corrupt map accepted")
		}
	}
	for _, n := range []int{3, 4, 5, 6} {
		layout := "variable"
		if _, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "tribe", Layout: layout}, nil); err == nil {
			t.Fatal("unverified recipe accepted")
		}
	}
	raw, _ := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "tribe"}, nil)
	before := clone(*raw)
	if _, err := raw.Catan.makeFishingTribe([]CatanFishingGroundPlacement{}); err == nil || !reflect.DeepEqual(before, *raw) {
		t.Fatal("invalid placement was not atomic")
	}
}
func TestCatanFishingTribeLakeProductionAndRobber(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, roll := range []int{2, 3, 11, 12} {
			for _, blocked := range []bool{false, true} {
				s := fishTribeGame(t, n)
				g := s.Catan
				s.Turn, s.Phase = 0, "catan_turn"
				g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
				v := g.Tiles[24].Vertices[0]
				g.Vertices[v].Owner, g.Vertices[v].Level = 1, 2
				if !g.landVertex(v) || !g.robberAllowed(24) {
					t.Fatal("lake lost mainland permissions")
				}
				for _, tile := range g.Tiles {
					if g.Seafarers.Islands[tile.ID] != g.Seafarers.StartIslands[0] && g.robberAllowed(tile.ID) {
						t.Fatal("robber can leave mainland")
					}
				}
				g.Robber = -1
				if blocked {
					g.Robber = 24
				}
				fishTop(&g.Fishing.Tokens, 0, 1)
				due, err := g.Fishing.Map.production(g, roll)
				if err != nil {
					t.Fatal(err)
				}
				want := 2
				if blocked {
					want = 0
				}
				if due[1] != want {
					t.Fatal("lake production", due, roll, blocked)
				}
				resources := clone(g.Players[1].Resources)
				if err = s.catanStartFishing(due, make([]int, n), "catan_turn"); err != nil {
					t.Fatal(err)
				}
				if len(g.Fishing.Tokens.Hands[1]) != want || !slices.Equal(resources, g.Players[1].Resources) {
					t.Fatal("lake fish payout")
				}
			}
		}
	}
	s := fishTribeGame(t, 3)
	g := s.Catan
	s.Turn, s.Phase = 0, "catan_turn"
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	fishOwn(&g.Fishing.Tokens, 0, 11)
	helperApply(t, s, 0, Action{Type: "catan_fish_robber", Tokens: []int{11}})
	if s.Catan.Robber != -1 {
		t.Fatal("fish cannot remove outer initial robber")
	}
	s.Phase = "catan_robber"
	s.Catan.ResumePhase = "catan_turn"
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 47})
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: 24})
	if s.Catan.Robber != 24 || s.Phase != "catan_turn" {
		t.Fatal("robber did not enter lake")
	}
}
func TestCatanFishingTribePortsRespectGrounds(t *testing.T) {
	s := fishTribeGame(t, 3)
	g := s.Catan
	s.Turn, s.Phase = 0, "catan_turn"
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	ground := g.Fishing.Map.Grounds[0]
	for _, v := range ground.Vertices {
		g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
	}
	// Move a real reward into the held supply, as after collecting it at sea.
	port := g.tribe().Ports[0]
	g.tribe().Ports = g.tribe().Ports[1:]
	g.tribe().HeldPorts[0] = []int{port.Resource}
	edges := g.tribePortEdges(0)
	for _, e := range ground.Edges {
		if slices.Contains(edges, e) {
			t.Fatal("port offered on fishing ground")
		}
	}
	if len(edges) == 0 {
		t.Fatal("fixture needs a nearby free coast")
	}
	if !s.catanAskTribePort(0, "catan_turn", nil, false) {
		t.Fatal("no port response")
	}
	before := clone(g.Fishing.Map)
	helperReject(t, s, 0, Action{Type: "catan_port", Edge: ground.Edges[0]})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_port", Edge: edges[0]})
	if !reflect.DeepEqual(before, s.Catan.Fishing.Map) || s.Catan.validateFishing() != nil || s.Phase != "catan_turn" {
		t.Fatal("port moved grounds or broke production")
	}
	for _, roll := range []int{4, 5, 6, 8, 9, 10} {
		if _, err := s.Catan.Fishing.Map.production(s.Catan, roll); err != nil {
			t.Fatal(err)
		}
	}
	assertTribeInventory(t, s.Catan)
}
func TestCatanFishingTribeBotsFinishAndConserve(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishTribeGame(t, n)
			paid := 0
			for step := 0; step < 10000 && !s.Finished; step++ {
				p := s.Turn
				if actor := s.CatanPendingActor(); actor >= 0 {
					p = actor
				} else if s.Phase == "catan_discard" {
					for actor, due := range s.Catan.DiscardDue {
						if due > 0 {
							p = actor
							break
						}
					}
				}
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if _, ok := catanFishCosts[a.Type]; ok {
					paid++
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				g := s.Catan
				if err = g.validateFishing(); err != nil {
					t.Fatal(err)
				}
				cardTribeInventory(t, g)
				assertTribeInventory(t, g)
				for p := range g.Players {
					roads, villages, cities := g.pieces(p)
					if roads > 15 || villages > 5 || cities > 4 || g.shipCount(p) > 15 {
						t.Fatal("piece inventory")
					}
				}
				if step%31 == 0 {
					copy := clone(*s)
					if !reflect.DeepEqual(copy, *s) {
						t.Fatal("restore")
					}
					s = &copy
				}
			}
			if !s.Finished || paid == 0 || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
				t.Fatal("incomplete game", s.Round, paid)
			}
			t.Logf("round=%d fish-actions=%d", s.Round, paid)
		})
	}
}

func TestCatanFishingTribeFishShipsCollectAndResume(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		for _, reward := range []string{"point", "development", "port"} {
			t.Run(phase+"/"+reward, func(t *testing.T) {
				s := fishTribeGame(t, 3)
				g := s.Catan
				s.Turn, s.Phase = 0, phase
				g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
				g.Seafarers.Pirate = -1
				edge := g.tribe().Tokens[0]
				if reward == "development" {
					edge = g.tribe().Development[0].Edge
				}
				if reward == "port" {
					edge = g.tribe().Ports[0].Edge
				}
				root := g.Fishing.Map.Grounds[0].Vertices[0]
				g.Vertices[root].Owner, g.Vertices[root].Level = 0, 1
				// Find an actual water route from a mainland settlement to the final reward edge.
				previous := map[int]int{root: -1}
				via := map[int]int{}
				queue := []int{root}
				end := -1
				for len(queue) > 0 {
					v := queue[0]
					queue = queue[1:]
					if v == g.Edges[edge].A || v == g.Edges[edge].B {
						end = v
						break
					}
					for _, e := range g.Edges {
						if e.ID == edge || !g.edgeTerrain(e.ID, true) {
							continue
						}
						next := -1
						if e.A == v {
							next = e.B
						} else if e.B == v {
							next = e.A
						}
						if next < 0 {
							continue
						}
						if _, ok := previous[next]; ok {
							continue
						}
						previous[next], via[next] = v, e.ID
						queue = append(queue, next)
					}
				}
				if end < 0 {
					t.Fatal("no water path")
				}
				for v := end; v != root; v = previous[v] {
					e := via[v]
					g.Edges[e].Owner, g.Edges[e].Ship = 0, true
				}
				if !g.canShip(0, edge) {
					t.Fatal("final reward edge not legal")
				}
				if reward == "port" && len(g.tribePortEdges(0)) == 0 {
					for _, e := range g.Edges {
						if g.fishingGroundEdge(e.ID) || !g.edgeTerrain(e.ID, true) || !g.edgeTerrain(e.ID, false) || !g.landVertex(e.A) {
							continue
						}
						g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
						if len(g.tribePortEdges(0)) > 0 {
							break
						}
					}
				}
				fishOwn(&g.Fishing.Tokens, 0, 11, 21)
				bank, dev, points, ports := clone(g.Bank), sum(g.Players[0].Dev), g.tribe().Points[0], len(g.tribe().Ports)
				helperReject(t, s, 1, Action{Type: "catan_fish_ship", Edge: edge, Tokens: []int{11, 21}})
				helperApply(t, s, 0, Action{Type: "catan_fish_ship", Edge: edge, Tokens: []int{11, 21}})
				g = s.Catan
				if !slices.Equal(g.Bank, bank) || !slices.Equal(g.Fishing.Tokens.Discard, []int{11, 21}) || !slices.Contains(g.Seafarers.BuiltShips, edge) {
					t.Fatal("payment / new ship marker")
				}
				if reward == "point" && g.tribe().Points[0] != points+1 {
					t.Fatal("point not collected")
				}
				if reward == "development" && (sum(g.Players[0].Dev) != dev+1 || sum(g.Players[0].NewDev) != 1) {
					t.Fatal("development missing or playable early")
				}
				if reward == "port" {
					if len(g.tribe().Ports) != ports-1 || s.Phase != "catan_port" || g.tribe().Pending.Resume != phase || g.tribe().Pending.AfterRoute == nil {
						t.Fatal("port continuation")
					}
					copy := clone(*s)
					s = &copy
					a, err := s.BotAction(0)
					if err != nil {
						t.Fatal(err)
					}
					if s.Catan.fishingGroundEdge(a.Edge) {
						t.Fatal("bot overwrites ground")
					}
					helperApply(t, s, 0, a)
				}
				if s.Phase != phase || s.Catan.validateFishing() != nil {
					t.Fatal("fish ship lost original phase", s.Phase)
				}
				if len(s.Catan.shipDestinations(0, edge)) != 0 {
					t.Fatal("new fish ship movable")
				}
				cardTribeInventory(t, s.Catan)
				assertTribeInventory(t, s.Catan)
			})
		}
	}
}

func TestCatanFishingTribePrivateViewAndStartingFish(t *testing.T) {
	s := fishTribeGame(t, 3)
	g := s.Catan
	fishOwn(&g.Fishing.Tokens, 0, 0, 11)
	fishOwn(&g.Fishing.Tokens, 1, 21)
	for _, viewer := range []int{-1, 0, 1, 2} {
		raw, _ := json.Marshal(s.View(viewer))
		var view map[string]any
		if err := json.Unmarshal(raw, &view); err != nil {
			t.Fatal(err)
		}
		v := view["catan"].(map[string]any)
		fish := v["fishing"].(map[string]any)
		tokens := fish["tokens"].(map[string]any)
		if fish["pending"] != nil || tokens["drawPile"] != nil || v["devDeck"] != nil {
			t.Fatal("hidden supply/continuation leaked")
		}
		for p, player := range tokens["players"].([]any) {
			_, faces := player.(map[string]any)["tokens"]
			if faces != (viewer == p && p < 2) {
				t.Fatal("private fish leaked", viewer, p)
			}
		}
		tribe := v["seafarers"].(map[string]any)["tribe"].(map[string]any)
		for _, card := range tribe["development"].([]any) {
			if card.(map[string]any)["card"] != nil {
				t.Fatal("map development face leaked")
			}
		}
	}
	s = fishTribeGame(t, 3)
	g = s.Catan
	s.Turn, s.Phase = 0, "catan_setup_settlement"
	g.SetupStep = 5
	v := g.Tiles[24].Vertices[0]
	fishTop(&g.Fishing.Tokens, 0)
	helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: v})
	if len(s.Catan.Fishing.Tokens.Hands[0]) != 1 || s.Phase != "catan_setup_road" || !s.Catan.Fishing.Started[0] {
		t.Fatal("lake starting entitlement")
	}
	if s.Catan.Players[0].Resources[3] != 0 {
		t.Fatal("replaced field still produced grain")
	}
}
