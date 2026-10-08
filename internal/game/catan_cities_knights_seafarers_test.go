package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func ckSea(t *testing.T, n int, scenario string) *State {
	t.Helper()
	var world *CatanNewWorldMap
	var err error
	if scenario == "new_world" {
		world, err = GenerateCatanNewWorldMap(n)
		if err != nil {
			t.Fatal(err)
		}
	}
	s, err := NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: scenario}, world)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func ckActor(s *State) int {
	if p := s.CatanPendingActor(); p >= 0 {
		return p
	}
	if s.Phase == "catan_discard" {
		for p, n := range s.Catan.DiscardDue {
			if n > 0 {
				return p
			}
		}
	}
	return s.Turn
}
func TestCatanCitiesKnightsSeafarersSetupAndVictory(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, scenario := range []string{"shores", "islands", "fog", "desert", "new_world", "tribe"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s := ckSea(t, n, scenario)
				g := s.Catan
				k := g.CitiesKnights
				want := 0
				for _, info := range CatanSeafarersScenarios(n) {
					if info.ID == scenario {
						want = info.VictoryPoints + 2
					}
				}
				if g.Seafarers.VictoryPoints != want || g.Robber != -1 || g.Seafarers.Pirate != -1 || len(g.DevDeck) > 0 || len(g.Bank) != 8 {
					t.Fatal("combination setup")
				}
				for step := 0; g.setup() && step < 100; step++ {
					p := ckActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(e)
					}
					helperApply(t, s, p, a)
					g = s.Catan
				}
				if g.setup() || s.Phase != "catan_roll" || !g.citySeaSupported() {
					t.Fatal("setup incomplete", s.Phase)
				}
				for p := range g.Players {
					_, v, c := g.pieces(p)
					if v != 1 || c != 1 || sum(g.Players[p].Resources[5:]) != 0 {
						t.Fatal("one village/city, ordinary resources only")
					}
				}
				ckProgressStock(t, g)
				ckKnightStock(t, g)
				if !reflect.DeepEqual(*s, clone(*s)) {
					t.Fatal("persist setup")
				}
				// First invasion restores each actual map's initial robber/pirate positions.
				s.catanFinishBarbarians()
				if g.Robber != k.RobberStart || g.Seafarers.Pirate != k.PirateStart {
					t.Fatal("wrong scenario bandit origin")
				}
				g.Players[s.Turn].Score = want - 1
				s.catanVictory()
				if s.Finished {
					t.Fatal("base or thirteen-point threshold used")
				}
				g.Players[s.Turn].Score = want
				s.catanVictory()
				if !s.Finished {
					t.Fatal("scenario +2 did not win")
				}
			})
		}
	}
	for _, scenario := range []string{"bad"} {
		if _, err := NewCatanCitiesKnightsSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: scenario}, nil); err == nil {
			t.Fatal("unverified special combination enabled", scenario)
		}
	}
	if helper, err := NewCatanCitiesKnightsSeafarers(3, CatanOptions{Helpers: true}, CatanSeafarersSetup{Scenario: "shores"}, nil); err != nil || !helper.Catan.cityHelpers() {
		t.Fatal("missing versioned Helpers adaptation", err)
	}
}
func TestCatanCitiesKnightsSeafarersGoldAndAqueduct(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	k := g.CitiesKnights
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: -1}
	g.Tiles = []CatanTile{{ID: 0, Resource: CatanGold, Number: 6, Vertices: []int{0, 1}}, {ID: 1, Resource: 0, Number: 6, Vertices: []int{2}}, {ID: 2, Resource: CatanDesert}}
	g.Vertices = []CatanVertex{{ID: 0, Owner: 0, Level: 2}, {ID: 1, Owner: 1, Level: 1}, {ID: 2, Owner: 2, Level: 2}}
	g.Edges = nil
	g.Ports = nil
	k.Players[0].Improvements[0] = 3
	if err := s.catanRollProduction(6); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_gold" || g.Players[2].Resources[0] != 1 || g.Players[2].Resources[5] != 1 {
		t.Fatal("ordinary production before gold")
	}
	if !reflect.DeepEqual(g.GoldPending.Claims, []CatanGoldClaim{{Player: 0, Count: 2}, {Player: 1, Count: 1}}) {
		t.Fatal(g.GoldPending)
	}
	helperReject(t, s, 0, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, 0, 2, 0, 0}})
	a, err := s.BotAction(0)
	if err != nil || len(a.Take) != 5 || sum(a.Take) != 2 {
		t.Fatal("bot gold selected commodities", a, err)
	}
	helperApply(t, s, 0, a)
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 1, Action{Type: "catan_gold", Take: []int{0, 1, 0, 0, 0}})
	if s.Phase != "catan_turn" || s.Catan.CitiesKnights.Pending != nil {
		t.Fatal("gold earner also received aqueduct")
	}
	ckProgressStock(t, s.Catan)
	// Bank contains commodities but no ordinary resources: no impossible choice.
	s = ckEmptyTurn(t)
	g = s.Catan
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: -1}
	helperGrant(s, 2, []int{19, 19, 19, 19, 19, 0, 0, 0})
	s.catanStartGold([]int{2, 1, 0}, []int{0, 0, 0}, "catan_turn")
	if g.GoldPending != nil || s.Phase != "catan_turn" {
		t.Fatal("commodity-only bank blocked gold queue")
	}
	// The final resource goes to the first claimant; the next is skipped.
	g.Players[2].Resources[0]--
	g.Bank[0]++
	s.catanStartGold([]int{2, 1, 0}, []int{0, 0, 0}, "catan_turn")
	a, err = s.BotAction(0)
	if err != nil || sum(a.Take) != 1 || len(a.Take) != 5 {
		t.Fatal(a, err)
	}
	helperApply(t, s, 0, a)
	if s.Catan.GoldPending != nil {
		t.Fatal("depleted queue stalled")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanCitiesKnightsSeafarersKnightRoutesAndChase(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 1})
	g := s.Catan
	k := g.CitiesKnights
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: 1}
	g.Tiles = append(g.Tiles, CatanTile{ID: 1, Resource: CatanSea, Vertices: []int{1, 2, 3, 4}}, CatanTile{ID: 2, Resource: CatanSea, Vertices: []int{0, 1}})
	for i := 1; i < len(g.Edges); i++ {
		g.Edges[i].Ship = true
	}
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 2, Active: true, ActivatedAt: 1}, {Owner: 1, Vertex: 3, Strength: 1, Active: true, ActivatedAt: 2}}
	helperApply(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 3})
	if s.Phase != "catan_knight_retreat" {
		t.Fatal("sea knight displacement")
	}
	helperApply(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: 4})
	if !s.Catan.knightAt(4).Active {
		t.Fatal("retreat activation lost")
	}
	// Both bandits are dormant until the first attack, even for a direct request.
	s.Turn = 1
	s.Catan.CitiesKnights.ActionSerial = 6
	helperReject(t, s, 1, Action{Type: "catan_knight_chase", Vertex: 4, Choice: "pirate"})
	s.Catan.CitiesKnights.Invasions = 1
	helperApply(t, s, 1, Action{Type: "catan_knight_chase", Vertex: 4, Choice: "pirate"})
	if s.Catan.CitiesKnights.Chase != "pirate" || s.Catan.knightAt(4).Active {
		t.Fatal("chase target not persisted")
	}
	saved := clone(*s)
	s = &saved
	helperReject(t, s, 1, Action{Type: "catan_robber", Tile: 0})
	a, err := s.BotAction(1)
	if err != nil || a.Type != "catan_pirate" {
		t.Fatal("chase bot moved wrong bandit", a, err)
	}
	helperApply(t, s, 1, a)
	if s.Catan.CitiesKnights.Chase != "" {
		t.Fatal("chase lock persisted after moving pirate")
	}
	// The inverse lock prevents a land knight from moving a distant pirate.
	s = ckKnightGraph(t, []int{0, 0})
	g = s.Catan
	k = g.CitiesKnights
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: 1}
	g.Tiles = append(g.Tiles, CatanTile{ID: 1, Resource: CatanSea, Vertices: []int{2}}, CatanTile{ID: 2, Resource: 1, Vertices: []int{2}})
	k.Invasions = 1
	g.Robber = 0
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1, Active: true, ActivatedAt: 1}}
	helperApply(t, s, 0, Action{Type: "catan_knight_chase", Vertex: 0})
	helperReject(t, s, 0, Action{Type: "catan_pirate", Tile: -1})
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: 2})
}
func TestCatanCitiesKnightsSeafarersDiplomacyAndKnightAnchors(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, -1})
	g := s.Catan
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: 0}
	g.Tiles[0].Resource = CatanSea
	g.Tiles[0].Vertices = []int{0, 1, 2, 3}
	g.Vertices[0].Owner = 0
	g.Vertices[0].Level = 1
	for i := range g.Edges {
		g.Edges[i].Ship = i < 2
	}
	if g.movableShip(0, 1) || !slices.Contains(g.diplomacyRoads(), 1) {
		t.Fatal("diplomacy pirate exception")
	}
	ckProgressGive(t, s, 0, 16)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: 1})
	if !s.Catan.CitiesKnights.Pending.Ship {
		t.Fatal("ship kind lost")
	}
	saved := clone(*s)
	s = &saved
	helperReject(t, s, 0, Action{Type: "catan_road", Edge: 1})
	helperApply(t, s, 0, Action{Type: "catan_diplomacy", Edge: 1})
	if !s.Catan.Edges[1].Ship || s.Catan.Seafarers.Pirate != 0 || sum(s.Catan.Players[0].Resources) != 0 || !slices.Contains(s.Catan.Seafarers.BuiltShips, 1) {
		t.Fatal("diplomacy changed ship, charged cost, or displaced pirate")
	}
	ckProgressStock(t, s.Catan)
	s = ckKnightGraph(t, []int{0, 0})
	g = s.Catan
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: -1}
	g.Edges[1].Ship = true
	g.Vertices[0].Owner = 0
	g.Vertices[0].Level = 1
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 2, Strength: 1}}
	if g.preservesKnightConnections(0, 1) || g.movableShip(0, 1) || slices.Contains(g.diplomacyRoads(), 1) {
		t.Fatal("mixed route bridge orphaned a knight")
	}
	// An alternative same-color route to a building keeps the knight anchored.
	g.Edges = append(g.Edges, CatanEdge{ID: 2, A: 0, B: 2, Owner: 0})
	if !g.preservesKnightConnections(0, 1) {
		t.Fatal("alternative anchor ignored")
	}
}

func TestCatanCitiesKnightsSeafarersBotsComplete(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, scenario := range []string{"shores", "islands", "fog", "desert", "new_world"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s := ckSea(t, n, scenario)
				steps := 0
				ships := 0
				for ; steps < 12000 && !s.Finished; steps++ {
					p := ckActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(steps, s.Phase, p, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(steps, s.Phase, p, a, err)
					}
					if a.Type == "catan_ship" {
						ships++
					}
					g := s.Catan
					ckProgressStock(t, g)
					ckKnightStock(t, g)
					for p := range g.Players {
						if g.shipCount(p) > 15 || g.cityPiecesLeft(p) < 0 || g.settlementPiecesLeft(p) < 0 {
							t.Fatal("piece inventory")
						}
					}
					if g.ArmyOwner != -1 || len(g.DevDeck) != 0 {
						t.Fatal("base development or largest army enabled")
					}
					if steps%53 == 0 {
						saved := clone(*s)
						if !reflect.DeepEqual(*s, saved) {
							t.Fatal("persistence")
						}
						s = &saved
					}
				}
				if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.Seafarers.VictoryPoints || s.Catan.CitiesKnights.Invasions == 0 {
					t.Fatal("unfinished or invalid combined victory", steps, s.Round, s.Phase)
				}
				t.Logf("steps=%d round=%d ships=%d invasions=%d victory=%d", steps, s.Round, ships, s.Catan.CitiesKnights.Invasions, s.Catan.Seafarers.VictoryPoints)
			})
		}
	}
}

func TestCatanCitiesKnightsSeafarersPillageBeforeGoldAndProgressBoundaries(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	k := g.CitiesKnights
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: -1}
	g.Tiles = []CatanTile{{ID: 0, Resource: CatanGold, Number: 6, Vertices: []int{0}}, {ID: 1, Resource: 0, Number: 6, Vertices: []int{1}}, {ID: 2, Resource: CatanDesert}, {ID: 3, Resource: CatanSea}}
	g.Vertices = []CatanVertex{{ID: 0, Owner: 1, Level: 2}, {ID: 1, Owner: 0, Level: 2}}
	g.Edges = nil
	g.Ports = nil
	k.RobberStart = 2
	k.PirateStart = 3
	k.BarbarianPosition = 6
	s.Phase = "catan_roll"
	if err := s.catanCityRoll(1, 5, 3); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_pillage" || s.CatanPendingActor() != 0 {
		t.Fatal("attack did not precede production")
	}
	helperApply(t, s, 0, Action{Type: "catan_pillage", Vertex: 1})
	helperApply(t, s, 1, Action{Type: "catan_pillage", Vertex: 0})
	g = s.Catan
	if s.Phase != "catan_gold" || g.GoldPending.Claims[0].Count != 1 || g.Players[0].Resources[0] != 1 || sum(g.Players[0].Resources[5:]) != 0 {
		t.Fatal("pre-pillage city production used")
	}
	helperApply(t, s, 1, Action{Type: "catan_gold", Take: []int{0, 1, 0, 0, 0}})
	ckProgressGive(t, s, 0, 12, 21)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 12, Tile: 0})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: 3})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: 1})
	if s.Catan.Seafarers.Pirate != 3 {
		t.Fatal("taxation moved pirate")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanCitiesKnightsSeafarersFreeShipsAndGlobalDefense(t *testing.T) {
	s := ckKnightGraph(t, []int{-1, -1})
	g := s.Catan
	k := g.CitiesKnights
	g.Seafarers = &CatanSeafarers{Scenario: "shores", Pirate: -1}
	g.Tiles = []CatanTile{{ID: 0, Resource: CatanSea, Vertices: []int{0, 1, 2}}, {ID: 1, Resource: 0, Vertices: []int{0}}, {ID: 2, Resource: CatanGold, Vertices: []int{2}}, {ID: 3, Resource: CatanDesert}}
	g.Vertices[0].Owner = 0
	g.Vertices[0].Level = 1
	ckProgressGive(t, s, 0, 7)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 7})
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: 0})
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: 1})
	if s.Phase != "catan_turn" || s.Catan.shipCount(0) != 2 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("free roads progress did not build two ships")
	}
	g = s.Catan
	k = g.CitiesKnights
	g.Vertices[0].Level = 2
	g.Vertices[2].Owner = 1
	g.Vertices[2].Level = 2
	// A single active knight defends cities on both separated land hexes.
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 1, Strength: 2, Active: true, ActivatedAt: 1}}
	k.BarbarianPosition = 6
	k.RobberStart = 3
	k.PirateStart = -1
	s.Phase = "catan_roll"
	if err := s.catanCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	if k.Players[0].DefenderPoints != 1 || k.Invasions != 1 || k.Knights[0].Active || g.Vertices[2].Level != 2 {
		t.Fatal("global island defense failed")
	}
	ckProgressStock(t, g)
}
