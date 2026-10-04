package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func fogTestGame(t *testing.T, resource int) (*State, int) {
	t.Helper()
	s := catanGame(t, 3)
	g := s.Catan
	if err := g.makeScenarioMap([]CatanHexSpec{{0, 0, 0, 6}, {1, 0, CatanSea, 0}, {1, -1, CatanFog, 0}}); err != nil {
		t.Fatal(err)
	}
	g.Seafarers = &CatanSeafarers{Scenario: "fog", VictoryPoints: 12, Pirate: -1, Fog: &CatanFogState{Terrain: []int{resource}, StartTiles: []int{0}}}
	if resource != CatanSea && resource != CatanDesert {
		g.Seafarers.Fog.Numbers = []int{8}
	}
	g.Seafarers.Islands = g.findIslands()
	g.SetupStep = 6
	g.TurnSerial = 1
	s.Turn = 0
	s.Phase = "catan_turn"
	edge := seaEdge(t, g, 0, 1)
	e := g.Edges[edge]
	root := e.A
	if g.Vertices[e.B].Y > g.Vertices[root].Y {
		root = e.B
	}
	g.Vertices[root].Owner = 0
	g.Vertices[root].Level = 1
	return s, edge
}

func TestCatanFogRoutesRevealAndConserve(t *testing.T) {
	for _, kind := range []string{"catan_road", "catan_ship"} {
		for _, resource := range []int{0, 1, 2, 3, 4, CatanSea, CatanDesert, CatanGold} {
			s, edge := fogTestGame(t, resource)
			helperGrant(s, 0, catanPrices[kind])
			helperApply(t, s, 0, Action{Type: kind, Edge: edge})
			g := s.Catan
			if g.Tiles[2].Resource != resource || len(g.Seafarers.Fog.Terrain) != 0 || len(g.Seafarers.Fog.Numbers) != 0 {
				t.Fatal("discovery not consumed", kind, resource)
			}
			if resource == CatanGold {
				if s.Phase != "catan_gold" || g.GoldPending.AfterRoute == nil || g.GoldPending.AfterRoute.Edge != edge {
					t.Fatal("gold missing continuation")
				}
				helperReject(t, s, 1, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, 1}})
				helperReject(t, s, 0, Action{Type: "catan_end"})
				restored := clone(*s)
				s = &restored
				helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, 1}})
				if s.Catan.Players[0].Resources[4] != 1 {
					t.Fatal("gold reward")
				}
			} else if resource < 5 && g.Players[0].Resources[resource] != 1 {
				t.Fatal("missing discovery resource")
			}
			if s.Phase != "catan_turn" || s.Catan.GoldPending != nil {
				t.Fatal("route not completed")
			}
			if (resource == CatanSea || resource == CatanDesert) && s.Catan.Tiles[2].Number != 0 {
				t.Fatal("sea/desert got token")
			}
			catanCheck(t, s)
		}
	}
}

func TestCatanFogBankEmptyAndAtomicInvalidDeck(t *testing.T) {
	s, edge := fogTestGame(t, 1)
	helperGrant(s, 0, catanPrices["catan_ship"])
	helperGrant(s, 1, []int{0, 19, 0, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if s.Catan.Players[0].Resources[1] != 0 {
		t.Fatal("discovery overdrew bank")
	}
	catanCheck(t, s)
	for _, damage := range []string{"terrain", "numbers"} {
		s, edge = fogTestGame(t, 1)
		helperGrant(s, 0, catanPrices["catan_ship"])
		if damage == "terrain" {
			s.Catan.Seafarers.Fog.Terrain = nil
		} else {
			s.Catan.Seafarers.Fog.Numbers = nil
		}
		helperReject(t, s, 0, Action{Type: "catan_ship", Edge: edge})
		if s.Catan.Tiles[2].Resource != CatanFog || s.Catan.Edges[edge].Owner != -1 {
			t.Fatal("partial reveal on invalid deck")
		}
	}
}

func TestCatanFogFreeRoadGoldResumesSecondRoute(t *testing.T) {
	s, edge := fogTestGame(t, CatanGold)
	s.Phase = "catan_roads"
	s.Catan.FreeRoads = 2
	s.Catan.ResumePhase = "catan_turn"
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if s.Phase != "catan_gold" || s.Catan.FreeRoads != 2 {
		t.Fatal("free placement completed before gold")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_roads" || s.Catan.FreeRoads != 1 {
		t.Fatal("lost second free route")
	}
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, 0, a)
	if s.Phase != "catan_turn" || s.Catan.FreeRoads != 0 {
		t.Fatal("free route chain did not finish")
	}
}

func TestCatanFogHelperRoadGoldThenExchange(t *testing.T) {
	s, edge := fogTestGame(t, CatanGold)
	g := s.Catan
	g.Options.Helpers = true
	g.Players[0].Helper = &CatanHelperSeat{ID: 2}
	g.HelperDisplay = []int{1, 3, 4}
	helperGrant(s, 0, catanPrices["catan_road"])
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: edge, Skill: "helper"})
	if s.Phase != "catan_gold" || s.Catan.HelperPending != nil {
		t.Fatal("helper exchange interrupted discovery")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_helper" || s.Catan.HelperPending.Kind != "exchange" || s.Catan.Players[0].Helper.UsedTurn != 1 {
		t.Fatal("lost deferred helper")
	}
	helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
	if s.Phase != "catan_turn" || !s.Catan.Players[0].Helper.Moon {
		t.Fatal("helper did not resume turn")
	}
}

func TestCatanFogSetupGoldDefersNextSeat(t *testing.T) {
	s, edge := fogTestGame(t, CatanGold)
	g := s.Catan
	g.SetupStep = 0
	g.StartPlayer = 0
	s.Phase = "catan_setup_road"
	e := g.Edges[edge]
	g.SetupVertex = e.A
	if g.Vertices[e.B].Owner == 0 {
		g.SetupVertex = e.B
	}
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if s.Turn != 0 || s.Catan.SetupStep != 0 || s.Phase != "catan_gold" {
		t.Fatal("setup advanced past gold claimant")
	}
	restored := clone(*s)
	s = &restored
	s.AutoCatanPending()
	if s.Turn != 1 || s.Catan.SetupStep != 1 || s.Phase != "catan_setup_settlement" {
		t.Fatal("next setup seat missing")
	}
	// Newly exposed land is not a new permitted starting island.
	for _, v := range s.Catan.Tiles[2].Vertices {
		if !slices.Contains(s.Catan.Tiles[0].Vertices, v) && s.Catan.seaSetupAllowed(v) {
			t.Fatal("discovery expanded starting area")
		}
	}
}

func TestCatanFogHiddenStacksAndBotPrivacy(t *testing.T) {
	s, edge := fogTestGame(t, 1)
	helperGrant(s, 0, catanPrices["catan_ship"])
	for _, viewer := range []int{-1, 0, 1, 2} {
		view := s.View(viewer)["catan"].(map[string]any)["seafarers"].(map[string]any)["fog"].(map[string]any)
		if _, ok := view["terrain"]; ok {
			t.Fatal("terrain leaked")
		}
		if _, ok := view["numbers"]; ok {
			t.Fatal("number stack leaked")
		}
		if view["remaining"] != 1 {
			t.Fatal("remaining count")
		}
	}
	before := clone(*s)
	changed := clone(*s)
	changed.Catan.Seafarers.Fog.Terrain = []int{CatanGold}
	changed.Catan.Seafarers.Fog.Numbers = []int{12}
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	b, err := changed.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("bot peeked at hidden stacks", a, b)
	}
	if s.Catan.seaRouteValue(0, edge, true) != changed.Catan.seaRouteValue(0, edge, true) {
		t.Fatal("route evaluation peeked")
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		v, _ := json.Marshal(s.View(viewer))
		w, _ := json.Marshal(changed.View(viewer))
		if string(v) != string(w) {
			t.Fatal("view depends on hidden stack")
		}
	}
	if !reflect.DeepEqual(before.Catan, s.Catan) {
		t.Fatal("bot or view mutated state")
	}
}

func TestCatanFogMoveShipAndHelperMoveRoadReveal(t *testing.T) {
	for _, ship := range []bool{false, true} {
		s, to := fogTestGame(t, CatanGold)
		g := s.Catan
		root := -1
		for _, v := range g.Vertices {
			if v.Owner == 0 {
				root = v.ID
			}
		}
		from := -1
		for _, id := range g.touching(root) {
			if id != to && g.edgeTerrain(id, ship) {
				from = id
				break
			}
		}
		if from < 0 {
			t.Fatal("fixture source route")
		}
		g.Edges[from].Owner = 0
		g.Edges[from].Ship = ship
		action := Action{Type: "catan_move_ship", Edge: from, Target: to}
		if !ship {
			g.Options.Helpers = true
			g.Players[0].Helper = &CatanHelperSeat{ID: 4}
			g.HelperDisplay = []int{1, 2, 3}
			action.Type = "catan_helper"
		}
		helperApply(t, s, 0, action)
		if s.Phase != "catan_gold" || s.Catan.Tiles[2].Resource != CatanGold {
			t.Fatal("moving did not discover")
		}
		s.AutoCatanPending()
		if ship {
			if s.Phase != "catan_turn" || !s.Catan.Seafarers.MovedShip {
				t.Fatal("ship movement lock")
			}
		} else if s.Phase != "catan_helper" {
			t.Fatal("lost moving helper exchange")
		}
		if s.Catan.Edges[from].Owner != -1 || s.Catan.Edges[to].Owner != 0 {
			t.Fatal("route not moved")
		}
	}
}

func TestCatanFogMultipleHexesAndNoRepeatReward(t *testing.T) {
	s := catanGame(t, 3)
	g := s.Catan
	if err := g.makeScenarioMap([]CatanHexSpec{{0, 0, CatanSea, 0}, {0, -1, CatanFog, 0}, {1, -1, CatanFog, 0}, {1, 0, 0, 6}}); err != nil {
		t.Fatal(err)
	}
	g.Seafarers = &CatanSeafarers{Scenario: "fog", Pirate: -1, Fog: &CatanFogState{Terrain: []int{CatanSea, 1}, Numbers: []int{9}, StartTiles: []int{3}}}
	g.SetupStep = 6
	s.Turn = 0
	s.Phase = "catan_turn"
	edge := seaEdge(t, g, 0, 2)
	e := g.Edges[edge]
	root := e.A
	if g.Vertices[e.B].Y > g.Vertices[root].Y {
		root = e.B
	}
	g.Vertices[root].Owner = 0
	g.Vertices[root].Level = 1
	helperGrant(s, 0, catanPrices["catan_ship"])
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	g = s.Catan
	if g.Tiles[1].Resource != 1 || g.Tiles[1].Number != 9 || g.Tiles[2].Resource != CatanSea || g.Tiles[2].Number != 0 || g.Players[0].Resources[1] != 1 {
		t.Fatal("multiple discovery handling")
	}
	before := clone(*s)
	if gold, err := s.catanDiscover(0, edge); err != nil || gold != 0 || !reflect.DeepEqual(before.Catan, s.Catan) {
		t.Fatal("repeated discovery")
	}
	catanCheck(t, s)
}

func TestCatanFogGoldEmptyBankFinishesRoute(t *testing.T) {
	s, edge := fogTestGame(t, CatanGold)
	helperGrant(s, 1, []int{19, 19, 19, 19, 19})
	s.Phase = "catan_roads"
	s.Catan.FreeRoads = 1
	s.Catan.ResumePhase = "catan_turn"
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if s.Phase != "catan_turn" || s.Catan.GoldPending != nil || s.Catan.FreeRoads != 0 {
		t.Fatal("empty bank stuck in pending route")
	}
	catanCheck(t, s)
}
