package game

import (
	"encoding/json"
	"slices"
	"testing"
)

// Real map and inventory, with an isolated route approaching one fog tile.
func twoSeaFogFixture(t *testing.T, helpers bool, owner int) (*State, int) {
	t.Helper()
	s := twoSeaGame(t, "fog", "fixed", helpers, false)
	g := s.Catan
	// A previously explored lumber hex creates a coastal road approach.
	for _, tile := range g.Tiles {
		if tile.Resource == CatanFog {
			f := g.Seafarers.Fog
			i := slices.Index(f.Terrain, 0)
			f.Terrain = slices.Delete(f.Terrain, i, i+1)
			g.Tiles[tile.ID].Resource, g.Tiles[tile.ID].Number = 0, f.Numbers[len(f.Numbers)-1]
			f.Numbers = f.Numbers[:len(f.Numbers)-1]
			f.StartTiles = append(f.StartTiles, tile.ID)
			g.Seafarers.Islands = g.findIslands()
			break
		}
	}
	for _, e := range g.Edges {
		if len(g.fogAtRoute(e.ID)) != 1 || !g.edgeTerrain(e.ID, true) || !g.edgeTerrain(e.ID, false) {
			continue
		}
		for _, v := range []int{e.A, e.B} {
			if !g.canSettlement(owner, v, true) {
				continue
			}
			g.Vertices[v].Owner, g.Vertices[v].Level = owner, 1
			g.SetupStep, g.TurnSerial = 4, 1
			s.Turn, s.Phase = 0, "catan_turn"
			if helpers {
				for p := range g.Players {
					g.Players[p].Helper = &CatanHelperSeat{ID: p + 1}
				}
			}
			g.Two.Rolls = []int{6, 8}
			fog := g.Seafarers.Fog
			i := slices.Index(fog.Terrain, CatanGold)
			if i < 0 {
				t.Fatal("missing gold")
			}
			fog.Terrain[i], fog.Terrain[len(fog.Terrain)-1] = fog.Terrain[len(fog.Terrain)-1], fog.Terrain[i]
			s.catanScores()
			return s, e.ID
		}
	}
	t.Fatal("no fog approach")
	return nil, -1
}
func TestCatanTwoSeafarersRouteGoldContinuation(t *testing.T) {
	for _, mode := range []string{"paid_ship", "free_ship", "helper_road"} {
		t.Run(mode, func(t *testing.T) {
			s, edge := twoSeaFogFixture(t, mode == "helper_road", 0)
			a := Action{Type: "catan_ship", Edge: edge}
			resume := "catan_turn"
			switch mode {
			case "paid_ship":
				helperGrant(s, 0, catanPrices[a.Type])
			case "free_ship":
				s.Phase = "catan_roads"
				s.Catan.FreeRoads = 2
				s.Catan.ResumePhase = "catan_turn"
				resume = "catan_roads"
			case "helper_road":
				eventAssignHelper(t, s, 0, 2)
				a.Type = "catan_road"
				a.Skill = "helper"
				helperGrant(s, 0, catanPrices[a.Type])
			}
			seq := s.Catan.Two.Sequence
			helperApply(t, s, 0, a)
			if s.Phase != "catan_gold" || s.Catan.Two.AfterRoute == "" || s.Catan.Two.Pending != nil {
				t.Fatal("discovery did not defer neutral", s.Phase)
			}
			twoSeaRestore(t, s)
			helperReject(t, s, 1, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
			helperReject(t, s, 0, Action{Type: "catan_two_build", Target: 0})
			helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
			twoSeaRestore(t, s)
			if mode == "helper_road" {
				if s.Phase != "catan_helper" || s.Catan.Two.Pending != nil || s.Catan.Two.AfterRoute != "road" {
					t.Fatal("lost helper continuation")
				}
				helperApply(t, s, 0, Action{Type: "catan_helper_choice", Choice: "flip"})
				twoSeaRestore(t, s)
			}
			q := s.Catan.Two
			if s.Phase != "catan_two_build" || q.Pending == nil || q.Pending.Resume != resume || q.Sequence != seq+1 || q.AfterRoute != "" {
				t.Fatal("lost or duplicated neutral response", s.Phase, q)
			}
			if mode == "free_ship" && s.Catan.FreeRoads != 1 {
				t.Fatal("free route consumed twice")
			}
			a = twoSeaStep(t, s)
			twoSeaRestore(t, s)
			if s.Phase != resume || s.Catan.Two.Pending != nil || s.Catan.Two.Sequence != seq+1 {
				t.Fatal("wrong continuation")
			}
			if mode != "helper_road" && !s.Catan.Edges[a.Edge].Ship {
				t.Fatal("neutral ship became road")
			}
		})
	}
}
func TestCatanTwoSeafarersNeutralDiscoveryNoReward(t *testing.T) {
	for _, kind := range []string{"road", "ship"} {
		t.Run(kind, func(t *testing.T) {
			s, edge := twoSeaFogFixture(t, false, -2)
			before, _ := json.Marshal(s.Catan.Players)
			bank := slices.Clone(s.Catan.Bank)
			pending := s.Catan.fogAtRoute(edge)
			if err := s.catanTwoStartBuild(kind); err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, 0, Action{Type: "catan_two_build", Target: 0, Edge: edge, Vertex: -1})
			after, _ := json.Marshal(s.Catan.Players)
			if s.Catan.Tiles[pending[0]].Resource != CatanGold || s.Catan.GoldPending != nil || s.Phase != "catan_turn" || string(before) != string(after) || !slices.Equal(bank, s.Catan.Bank) {
				t.Fatal("neutral received resource/claim")
			}
			twoSeaRestore(t, s)
		})
	}
}
func TestCatanTwoSeafarersShipFallbackAndRetreat(t *testing.T) {
	s := twoSeaGame(t, "islands", "fixed", false, false)
	g := s.Catan
	ships := g.twoNeutralChoices("ship")
	if len(ships) == 0 {
		t.Fatal("missing coastal ships")
	}
	for _, choice := range ships {
		if !g.canShip(choice.Owner, choice.Edge) {
			t.Fatal("road offered while ship possible")
		}
	}
	// Both neutral fleets exhausted: only then fall back to a legal road.
	for _, owner := range catanTwoNeutralOwners {
		for g.shipCount(owner) < 15 {
			found := false
			for _, e := range g.Edges {
				if e.Owner == -1 {
					g.Edges[e.ID].Owner, g.Edges[e.ID].Ship = owner, true
					found = true
					break
				}
			}
			if !found {
				t.Fatal("fixture full")
			}
		}
	}
	if len(g.twoNeutralShipChoices()) != 0 {
		t.Fatal("fleet limit")
	}
	if len(g.twoNeutralChoices("ship")) == 0 {
		t.Fatal("fixture lacks fallback road")
	}
	for _, choice := range g.twoNeutralChoices("ship") {
		if err := g.placeTwoNeutral("ship", choice); err != nil {
			t.Fatal(err)
		}
		if g.Edges[choice.Edge].Ship {
			t.Fatal("fallback consumed ship")
		}
		break
	}
	s = twoSeaGame(t, "islands", "fixed", false, false)
	finishFishHelperSetup(t, s)
	g = s.Catan
	s.Phase = "catan_turn"
	g.Two.Rolls = []int{6, 8}
	g.Two.Spent = false
	for _, tile := range g.Tiles {
		if tile.Resource < 5 {
			g.Robber = tile.ID
			break
		}
	}
	if g.twoDesert() != -1 || !slices.Equal(g.twoRetreatTiles(), []int{-1}) {
		t.Fatal("islands retreat")
	}
	p := s.Turn
	tokens, cost, pirate := g.Two.Tokens[p], g.twoTokenCost(p), g.Seafarers.Pirate
	helperApply(t, s, p, Action{Type: "catan_two_robber", Tile: -1})
	g = s.Catan
	if g.Robber != -1 || g.Seafarers.Pirate != pirate || g.Two.Tokens[p] != tokens-cost || !g.Two.Spent {
		t.Fatal("wrong retreat cost/piece")
	}
	twoSeaRestore(t, s)
	helperReject(t, s, p, Action{Type: "catan_two_robber", Tile: -1})
}

func TestCatanTwoSeafarersGoldProductionAndMove(t *testing.T) {
	s := twoSeaGame(t, "shores", "fixed", false, false)
	finishFishHelperSetup(t, s)
	g := s.Catan
	p := s.Turn
	s.Phase = "catan_roll"
	g.Two.Rolls = nil
	tile := -1
	for _, v := range g.Vertices {
		if v.Owner == p {
			for _, hex := range g.Tiles {
				if slices.Contains(hex.Vertices, v.ID) && hex.Resource < 5 {
					tile = hex.ID
					break
				}
			}
			if tile >= 0 {
				break
			}
		}
	}
	if tile < 0 {
		t.Fatal("no production site")
	}
	g.Tiles[tile].Resource, g.Tiles[tile].Number = CatanGold, 6
	if err := s.catanTwoRoll(3, 3); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_gold" {
		t.Fatal("no first gold production")
	}
	for s.Catan.GoldPending != nil {
		twoSeaRestore(t, s)
		twoSeaStep(t, s)
	}
	if s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 1 {
		t.Fatal("first gold skipped second production")
	}
	// A previously built open ship can move without creating a neutral ship.
	s.Phase = "catan_turn"
	s.Catan.Two.Rolls = []int{6, 8}
	g = s.Catan
	for _, e := range g.Edges {
		if g.canShip(p, e.ID) {
			g.Edges[e.ID].Owner, g.Edges[e.ID].Ship = p, true
			break
		}
	}
	found := false
	for _, e := range g.Edges {
		if e.Owner != p || !e.Ship {
			continue
		}
		to := g.shipDestinations(p, e.ID)
		if len(to) == 0 {
			continue
		}
		seq := g.Two.Sequence
		helperApply(t, s, p, Action{Type: "catan_move_ship", Edge: e.ID, Target: to[0]})
		if s.Catan.Two.Sequence != seq || s.Catan.Two.Pending != nil || s.Catan.Two.AfterRoute != "" {
			t.Fatal("moving ship built neutral")
		}
		twoSeaRestore(t, s)
		found = true
		break
	}
	if !found {
		t.Fatal("missing movable ship")
	}
}
func TestCatanTwoSeafarersCorruptSaveAndIsolation(t *testing.T) {
	valid := twoSeaGame(t, "shores", "fixed", false, false)
	for _, damage := range []func(*State){
		func(s *State) { s.Catan.Two.Seafarers = "" },
		func(s *State) { s.Catan.Two.Seafarers = "unknown" },
		func(s *State) { s.Catan.Two.SeaStarts[0] = -1 },
		func(s *State) { s.Catan.Seafarers.VictoryPoints = 1 },
		func(s *State) { s.Catan.Seafarers.IslandBonus = 99 },
		func(s *State) { s.Catan.Edges[0].A = len(s.Catan.Vertices) },
		func(s *State) { s.Catan.Two.AfterRoute = "ship" },
		func(s *State) { s.Catan.Two.SeaStarts[1] = s.Catan.Two.SeaStarts[0] },
	} {
		s := clone(*valid)
		damage(&s)
		if s.validateCatanTwo() == nil {
			t.Fatal("corrupt sea accepted")
		}
	}
	for _, n := range []int{3, 4} {
		if _, err := NewCatanTwoSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "shores"}, nil); err == nil {
			t.Fatal("two constructor accepted multiplayer")
		}
	}
	base, err := NewCatanTwo(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if base.Catan.Seafarers != nil || base.Catan.Two.Seafarers != "" || len(base.Catan.Tiles) != 19 {
		t.Fatal("base changed")
	}
	twoSeaRestore(t, base)
}

func TestCatanTwoSeafarersMissionPrivacyAndCoastTokens(t *testing.T) {
	s := twoSeaGame(t, "tribe", "fixed", false, false)
	g := s.Catan
	tr := g.tribe()
	for _, c := range g.twoNeutralShipChoices() {
		if slices.Contains(tr.Tokens, c.Edge) || slices.ContainsFunc(tr.Development, func(d CatanTribeDevelopment) bool { return d.Edge == c.Edge }) || slices.ContainsFunc(tr.Ports, func(p CatanPort) bool { return p.Edge == c.Edge }) {
			t.Fatal("neutral blocks mission reward")
		}
	}
	// Human coastal building earns exactly once, neutrals receive no trade tokens.
	for _, v := range g.Two.SeaStarts {
		if g.twoSettlementTokens(0, v) != 1 || g.twoSettlementTokens(-2, v) != 0 {
			t.Fatal("coastal token reward")
		}
	}
	s = twoSeaGame(t, "fog", "fixed", false, false)
	altered := clone(*s)
	slices.Reverse(altered.Catan.Seafarers.Fog.Terrain)
	slices.Reverse(altered.Catan.Seafarers.Fog.Numbers)
	for _, viewer := range []int{-1, 0, 1} {
		a, _ := json.Marshal(s.View(viewer))
		b, _ := json.Marshal(altered.View(viewer))
		if string(a) != string(b) {
			t.Fatal("hidden fog leaked")
		}
	}
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	b, err := altered.BotAction(altered.Turn)
	if err != nil {
		t.Fatal(err)
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) != string(bj) {
		t.Fatal("bot read fog")
	}
}
