package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishFogGame(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := NewCatanFishingSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "fog", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanFishingFogMapsAndExploration(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				for range 24 {
					s := fishFogGame(t, n, layout)
					g := s.Catan
					if len(g.Fishing.Map.Lakes) != 0 || len(g.Fishing.Map.Grounds) != 6 || g.victoryTargetFor(0) != 12 {
						t.Fatal("wrong recipe/target")
					}
					coasts := g.FishingCoasts()
					before := clone(g.Fishing.Map)
					placements := []CatanFishingGroundPlacement{}
					for i, ground := range before.Grounds {
						// Fog has no Four Islands' mandatory number groups.
						placements = append(placements, CatanFishingGroundPlacement{before.Grounds[(i+1)%6].Number, [2]int{ground.Edges[1], ground.Edges[0]}})
					}
					if _, err := g.makeFishingFog(placements); err != nil {
						t.Fatal("host-selected grounds rejected", err)
					}
					for _, e := range g.Edges {
						if _, err := s.catanDiscover(0, e.ID); err != nil {
							t.Fatal(err)
						}
						if err := g.validateFishing(); err != nil {
							t.Fatal("discovery invalidated grounds", err)
						}
					}
					if !reflect.DeepEqual(coasts, g.FishingCoasts()) || !reflect.DeepEqual(before, g.Fishing.Map) || len(g.Seafarers.Fog.Terrain) != 0 {
						t.Fatal("discovery changed initial coast/grounds or remained incomplete")
					}
					copy := clone(*s)
					if err := copy.Catan.validateFishing(); err != nil || !reflect.DeepEqual(*s, copy) {
						t.Fatal("restore", err)
					}
				}
			})
		}
	}
}

func TestCatanFishingFogRejectsInvalidRecipe(t *testing.T) {
	for _, change := range []func(*Catan){
		func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{Tile: 0}} },
		func(g *Catan) { g.Fishing.Map.Grounds = g.Fishing.Map.Grounds[:5] },
		func(g *Catan) { g.Fishing.Map.Grounds[0].Number = 7 },
		func(g *Catan) { g.Fishing.Map.Grounds[0] = g.Fishing.Map.Grounds[1] },
		func(g *Catan) { g.Fishing.Map.Grounds[0].Vertices[1] = -1 },
		func(g *Catan) { g.Ports[0].Edge = g.Fishing.Map.Grounds[0].Edges[0] },
		func(g *Catan) { g.Fishing.Map.Grounds[0].SeaTile = new(int) },
		func(g *Catan) { g.Seafarers.Fog.StartTiles = append(g.Seafarers.Fog.StartTiles, 0) },
		func(g *Catan) { g.Seafarers.Fog.Terrain[0] = catanLake },
		func(g *Catan) { g.Tiles[0].Resource = catanLake },
	} {
		s := fishFogGame(t, 3, "fixed")
		change(s.Catan)
		if err := s.Catan.validateFishing(); err == nil {
			t.Fatal("invalid map accepted")
		}
	}
	s := fishFogGame(t, 3, "fixed")
	before := clone(*s)
	if _, err := s.Catan.makeFishingFog([]CatanFishingGroundPlacement{{4, [2]int{0, 1}}}); err == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("invalid placement not atomic")
	}
	for _, n := range []int{5, 6} {
		if _, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "fog"}, nil); err == nil {
			t.Fatal("unverified five/six recipe accepted")
		}
	}
}

// Production fixture uses the real scenario and discovery stack, with chosen
// buildings/numbers so fish, ordinary resources and gold trigger together.
func fishFogProduction(t *testing.T, full bool) *State {
	t.Helper()
	s := fishFogGame(t, 3, "fixed")
	g := s.Catan
	for _, e := range g.Edges {
		if _, err := s.catanDiscover(0, e.ID); err != nil {
			t.Fatal(err)
		}
	}
	for p := range g.Players {
		catanMove(g.Players[p].Resources, g.Bank, slices.Clone(g.Players[p].Resources))
	}
	g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
	s.Turn, s.Phase = 0, "catan_roll"
	g.Seafarers.Pirate, g.Robber = -1, -1
	ground := g.Fishing.Map.Grounds[0]
	for i := range g.Tiles {
		if g.Tiles[i].Resource < 5 || g.Tiles[i].Resource == CatanGold {
			g.Tiles[i].Number = 2
		}
	}
	g.Vertices[ground.Vertices[0]].Owner, g.Vertices[ground.Vertices[0]].Level = 1, 1
	g.Vertices[ground.Vertices[2]].Owner, g.Vertices[ground.Vertices[2]].Level = 2, 1
	gold := slices.IndexFunc(g.Tiles, func(tile CatanTile) bool { return tile.Resource == CatanGold })
	g.Tiles[gold].Number = ground.Number
	p := 1
	for _, v := range g.Tiles[gold].Vertices {
		if g.Vertices[v].Owner < 0 && p <= 2 {
			g.Vertices[v].Owner, g.Vertices[v].Level = p, 3-p
			p++
		}
	}
	ordinary := g.Seafarers.Fog.StartTiles[0]
	g.Tiles[ordinary].Number = ground.Number
	for _, v := range g.Tiles[ordinary].Vertices {
		if g.Vertices[v].Owner < 0 {
			g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
			break
		}
	}
	if full {
		fishOwn(&g.Fishing.Tokens, 1, 0, 1, 2, 3, 4, 5, 6)
		fishOwn(&g.Fishing.Tokens, 2, 11, 12, 13, 14, 15, 16, 17)
	}
	fishTop(&g.Fishing.Tokens, 21, 22)
	return s
}

func TestCatanFishingFogGoldProductionContinuation(t *testing.T) {
	for _, full := range []bool{true, false} {
		for _, bank := range []int{95, 1, 0} {
			t.Run(fmt.Sprintf("responses=%t/bank=%d", full, bank), func(t *testing.T) {
				s := fishFogProduction(t, full)
				g := s.Catan
				if bank < 95 {
					for c, amount := range g.Bank {
						keep := 0
						if c == 4 {
							keep = bank
						}
						g.Bank[c] = keep
						g.Players[0].Resources[c] += amount - keep
					}
				}
				number := g.Fishing.Map.Grounds[0].Number
				if err := s.catanRollProduction(number); err != nil {
					t.Fatal(err)
				}
				ordinary := clone(g.Players)
				available := sum(g.Bank)
				if full {
					if s.Phase != "catan_fish_replace" || g.GoldPending != nil || !slices.Equal(g.Fishing.Pending.Gold, []int{0, 2, 1}) {
						t.Fatal("lost/premature gold", g.Fishing.Pending)
					}
					for _, p := range []int{1, 2} {
						if s.CatanPendingActor() != p {
							t.Fatal("fish order")
						}
						fishGameReject(t, s, p, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
						copy := clone(*s)
						s = &copy
						if err := s.Apply(p, Action{Type: "catan_fish_keep"}); err != nil {
							t.Fatal(err)
						}
					}
				}
				paid := []int{0, 0, 0}
				for s.Phase == "catan_gold" {
					copy := clone(*s)
					s = &copy
					p := s.CatanPendingActor()
					if p < 1 || p > 2 || paid[p] > 0 {
						t.Fatal("gold order/repetition")
					}
					if s.Catan.Fishing.Pending != nil {
						t.Fatal("overlapping response queues")
					}
					fishGameReject(t, s, p, Action{Type: "catan_fish_keep"})
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(err)
					}
					paid[p] = sum(a.Take)
					if err := s.Apply(p, a); err != nil {
						t.Fatal(err)
					}
				}
				g = s.Catan
				if s.Phase != "catan_turn" || s.Turn != 0 || g.GoldPending != nil || g.Fishing.Pending != nil || sum(paid) != min(3, available) {
					t.Fatal("continuation/payments", s.Phase, paid, available)
				}
				for p := range g.Players {
					if sum(g.Players[p].Resources) != sum(ordinary[p].Resources)+paid[p] {
						t.Fatal("duplicated/missing payout")
					}
				}
				before := clone(*s)
				if err := s.catanRollProduction(number); err == nil || !reflect.DeepEqual(before, *s) {
					t.Fatal("repeated production")
				}
				fleetSupply(t, g)
				if err := g.validateFishing(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestCatanFishingFogContinuationPrivacyAndValidation(t *testing.T) {
	s := fishFogProduction(t, true)
	if err := s.catanRollProduction(s.Catan.Fishing.Map.Grounds[0].Number); err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		v := s.View(viewer)["catan"].(map[string]any)
		f := v["fishing"].(map[string]any)
		if _, ok := f["pending"]; ok {
			t.Fatal("saved continuation leaked")
		}
		fog := v["seafarers"].(map[string]any)["fog"].(map[string]any)
		if _, ok := fog["terrain"]; ok {
			t.Fatal("hidden stack leaked")
		}
	}
	for _, change := range []func(*Catan){
		func(g *Catan) { g.Fishing.Pending.Gold = []int{1} },
		func(g *Catan) { g.Fishing.Pending.Gold = []int{0, -1, 1} },
		func(g *Catan) { g.Fishing.Pending.Received[0] = -1 },
		func(g *Catan) { g.GoldPending = &CatanGoldPending{Resume: "catan_turn"} },
	} {
		copy := clone(*s)
		change(copy.Catan)
		if err := copy.Catan.validateFishing(); err == nil {
			t.Fatal("corrupt continuation accepted")
		}
	}
	// Changing hidden stack order leaves every public view and coast unchanged.
	x := fishFogGame(t, 3, "fixed")
	y := clone(*x)
	slices.Reverse(y.Catan.Seafarers.Fog.Terrain)
	slices.Reverse(y.Catan.Seafarers.Fog.Numbers)
	for _, viewer := range []int{-1, 0, 1, 2} {
		a, _ := json.Marshal(x.View(viewer))
		b, _ := json.Marshal(y.View(viewer))
		if string(a) != string(b) {
			t.Fatal("view uses hidden order")
		}
	}
	if !reflect.DeepEqual(x.Catan.FishingCoasts(), y.Catan.FishingCoasts()) {
		t.Fatal("ground placement uses hidden order")
	}
}

func TestCatanFishingFogSetupGoldDoesNotOverwriteFish(t *testing.T) {
	for _, full := range []bool{false, true} {
		s := fishFogGame(t, 3, "fixed")
		g := s.Catan
		ground := g.Fishing.Map.Grounds[0]
		vertex := ground.Vertices[1]
		// Gold is normally hidden at Fog setup. This targeted continuation
		// fixture places it on a starting coast to exercise the shared setup
		// path used by other gold-bearing scenario combinations as well.
		gold := -1
		for _, tile := range g.Tiles {
			if tile.Resource < 5 && slices.Contains(tile.Vertices, vertex) {
				gold = tile.ID
				break
			}
		}
		if gold < 0 {
			t.Fatal("no coastal starting tile")
		}
		g.Tiles[gold].Resource = CatanGold
		g.Robber = -1
		g.SetupStep, s.Turn = 3, 0
		if full {
			fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
		}
		fishTop(&g.Fishing.Tokens, 21)
		if err := s.Apply(0, Action{Type: "catan_settlement", Vertex: vertex}); err != nil {
			t.Fatal(err)
		}
		if full {
			if s.Phase != "catan_fish_replace" || s.Catan.GoldPending != nil {
				t.Fatal("setup gold overwrote fish")
			}
			copy := clone(*s)
			s = &copy
			if err := s.Apply(0, Action{Type: "catan_fish_keep"}); err != nil {
				t.Fatal(err)
			}
		}
		if s.Phase != "catan_gold" || s.Catan.GoldPending.Claims[0].Count != 1 {
			t.Fatal("missing starting gold")
		}
		copy := clone(*s)
		s = &copy
		if err := s.Apply(0, Action{Type: "catan_gold", Take: []int{0, 0, 0, 1, 0}}); err != nil {
			t.Fatal(err)
		}
		if s.Phase != "catan_setup_road" || s.Turn != 0 || s.Catan.SetupStep != 3 || s.Catan.SetupVertex != vertex || !s.Catan.Fishing.Started[0] {
			t.Fatal("setup did not resume route")
		}
		a, err := s.BotAction(0)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(0, a); err != nil {
			t.Fatal(err)
		}
		if s.Catan.SetupStep != 4 {
			t.Fatal("setup step missing/duplicated")
		}
		fleetSupply(t, s.Catan)
	}
}

func fishFogDiscoveryRoute(t *testing.T, phase string, ship bool) (*State, int) {
	t.Helper()
	s := fishFogGame(t, 3, "fixed")
	g := s.Catan
	s.Turn, s.Phase = 0, phase
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	g.Seafarers.Pirate = -1
	fishOwn(&g.Fishing.Tokens, 0, 11, 21)
	if !ship {
		// The starting islands cannot reach fog by road. First discover one
		// ordinary land hex, then test a road extending toward adjacent fog.
		for _, edge := range g.Edges {
			if len(g.fogAtRoute(edge.ID)) != 1 {
				continue
			}
			fog := g.Seafarers.Fog
			at := slices.IndexFunc(fog.Terrain, func(r int) bool { return r < 5 })
			last := len(fog.Terrain) - 1
			fog.Terrain[at], fog.Terrain[last] = fog.Terrain[last], fog.Terrain[at]
			if _, err := s.catanDiscover(0, edge.ID); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	for _, edge := range g.Edges {
		if len(g.fogAtRoute(edge.ID)) == 0 {
			continue
		}
		for _, v := range []int{edge.A, edge.B} {
			g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
			legal := g.canRoad(0, edge.ID)
			if ship {
				legal = g.canShip(0, edge.ID)
			}
			if legal {
				fog := g.Seafarers.Fog
				for i, r := range fog.Terrain {
					if r == CatanGold {
						last := len(fog.Terrain) - 1
						fog.Terrain[i], fog.Terrain[last] = fog.Terrain[last], fog.Terrain[i]
						return s, edge.ID
					}
				}
			}
			g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
		}
	}
	t.Fatal("no route to fog")
	return nil, -1
}

func TestCatanFishingFogPaidRouteGoldAndResume(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		for _, ship := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/ship=%t", phase, ship), func(t *testing.T) {
				s, edge := fishFogDiscoveryRoute(t, phase, ship)
				kind := "catan_fish_road"
				if ship {
					kind = "catan_fish_ship"
				}
				fish := clone(s.Catan.Fishing.Tokens)
				terrainCount := len(s.Catan.Seafarers.Fog.Terrain)
				if err := s.Apply(0, Action{Type: kind, Edge: edge, Tokens: []int{11, 21}}); err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if s.Phase != "catan_gold" || g.GoldPending.AfterRoute == nil || g.GoldPending.Resume != phase || g.Edges[edge].Owner != 0 || g.Edges[edge].Ship != ship || len(g.Seafarers.Fog.Terrain) >= terrainCount {
					t.Fatal("discovery continuation")
				}
				if ship && !slices.Contains(g.Seafarers.BuiltShips, edge) {
					t.Fatal("missing new ship lock")
				}
				if len(g.Fishing.Tokens.Hands[0]) != 0 || !slices.Equal(g.Fishing.Tokens.Discard, []int{11, 21}) || !slices.Equal(fish.DrawPile, g.Fishing.Tokens.DrawPile) {
					t.Fatal("discovery paid/produced fish incorrectly")
				}
				fishGameReject(t, s, 0, Action{Type: kind, Edge: edge, Tokens: []int{11, 21}})
				copy := clone(*s)
				s = &copy
				a, err := s.BotAction(0)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Apply(0, a); err != nil {
					t.Fatal(err)
				}
				if s.Phase != phase || s.Turn != 0 || s.Catan.GoldPending != nil || s.Catan.FreeRoads != 0 {
					t.Fatal("gold lost original phase")
				}
				if ship && len(s.Catan.shipDestinations(0, edge)) > 0 {
					t.Fatal("fresh fish ship movable")
				}
				if err = s.Catan.validateFishing(); err != nil {
					t.Fatal(err)
				}
				fleetSupply(t, s.Catan)
			})
		}
	}
}

func TestCatanFishingFogBotsFinishAndConserve(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := fishFogGame(t, n, layout)
				paid, gold := 0, 0
				for step := 0; step < 8000 && !s.Finished; step++ {
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
						t.Fatal(step, s.Phase, err)
					}
					if _, ok := catanFishCosts[a.Type]; ok {
						paid++
					}
					if a.Type == "catan_gold" {
						gold++
					}
					if err = s.Apply(actor, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					g := s.Catan
					if err = g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					fleetSupply(t, g)
					for p := range g.Players {
						roads, settlements, cities := g.pieces(p)
						if roads > 15 || settlements > 5 || cities > 4 || g.shipCount(p) > 15 {
							t.Fatal("piece supply")
						}
					}
					if step%31 == 0 {
						copy := clone(*s)
						if !reflect.DeepEqual(*s, copy) {
							t.Fatal("restore")
						}
						s = &copy
					}
				}
				if !s.Finished || paid == 0 || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
					t.Fatal("incomplete game", s.Round, paid, gold)
				}
				t.Logf("round=%d fish-actions=%d gold-choices=%d unexplored=%d", s.Round, paid, gold, len(s.Catan.Seafarers.Fog.Terrain))
			})
		}
	}
}
