package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

// A line graph isolates road rules; it is not an official scenario map.
func earthquakeGraph(t *testing.T, owners []int) *State {
	t.Helper()
	s := catanGame(t, 3)
	g := s.Catan
	g.SetupStep, g.TurnSerial = 6, 1
	s.Turn, s.Phase = 0, "catan_turn"
	g.Vertices = make([]CatanVertex, len(owners)+1)
	g.Edges = make([]CatanEdge, len(owners))
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6}}
	g.Ports = nil
	for i := range g.Vertices {
		g.Vertices[i] = CatanVertex{ID: i, Owner: -1}
		g.Tiles[0].Vertices = append(g.Tiles[0].Vertices, i)
	}
	for i, owner := range owners {
		g.Edges[i] = CatanEdge{ID: i, A: i, B: i + 1, Owner: owner, Tiles: []int{0}}
	}
	return s
}

func earthquakeDamage(t *testing.T, s *State, player int, edges ...int) {
	t.Helper()
	for _, edge := range edges {
		if err := s.catanDamageRoad(player, edge); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCatanEarthquakeDamageGuards(t *testing.T) {
	s := earthquakeGraph(t, []int{0, 0, 1, -1})
	s.Catan.Edges[1].Ship = true
	for _, pair := range [][2]int{{-1, 0}, {3, 0}, {0, -1}, {0, 4}, {0, 1}, {0, 2}, {0, 3}} {
		before, _ := json.Marshal(s)
		if err := s.catanDamageRoad(pair[0], pair[1]); err == nil {
			t.Fatalf("accepted invalid damage: %v", pair)
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("invalid damage mutated state")
		}
	}
	earthquakeDamage(t, s, 0, 0)
	before, _ := json.Marshal(s)
	if err := s.catanDamageRoad(0, 0); err == nil {
		t.Fatal("damaged the same road twice")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("repeated damage mutated state")
	}
	s.Catan.Players[1].Eliminated = true
	if s.Catan.roadDamageable(1, 2) {
		t.Fatal("eliminated player can damage road")
	}
	// No client action exists that can arbitrarily damage somebody's road.
	helperReject(t, s, 0, Action{Type: "catan_damage_road", Edge: 0})
}

func TestCatanEarthquakeRepairAllBeforeNewRoadAndLongestRoute(t *testing.T) {
	s := earthquakeGraph(t, []int{0, 0, 0, 0, 0, 0, -1, 1, -1})
	s.catanScores()
	earthquakeDamage(t, s, 0, 0, 5)
	s.catanScores()
	g := s.Catan
	if g.roadLength(0) != 6 || g.LongestOwner != 0 || g.Players[0].Score != 2 || !g.canRoad(1, 8) || g.canRoad(0, 6) {
		t.Fatal("damage changed scoring or failed to block own new roads")
	}
	helperGrant(s, 0, []int{3, 3, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_road", Edge: 6})
	helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: 0})
	if g.canRoad(0, 6) || !g.Edges[5].Damaged || g.Players[0].Resources[0] != 2 || g.Players[0].Resources[1] != 2 {
		t.Fatal("partial repair lifted block or charged wrong cost")
	}
	helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: 5})
	if !g.canRoad(0, 6) || g.roadLength(0) != 6 {
		t.Fatal("final repair failed to restore builds")
	}
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: 6})
	if sum(g.Players[0].Resources) != 0 || g.roadLength(0) != 7 {
		t.Fatal("new road after repair")
	}
	catanCheck(t, s)
}

func TestCatanEarthquakeRepairGuardsAndPersistence(t *testing.T) {
	s := earthquakeGraph(t, []int{0, 0, 1, -1})
	earthquakeDamage(t, s, 0, 0)
	s.Catan.Edges[2].Damaged = true
	for _, edge := range []int{-1, 1, 2, 3, 4} {
		helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: edge})
	}
	helperReject(t, s, 1, Action{Type: "catan_repair_road", Edge: 0})
	helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: 0}) // No resources.
	helperGrant(s, 0, []int{1, 0, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: 0}) // Missing brick.
	helperGrant(s, 0, []int{0, 1, 0, 0, 0})
	for _, phase := range []string{"catan_roll", "catan_discard", "catan_robber", "catan_roads"} {
		s.Phase = phase
		helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: 0})
	}
	s.Phase = "catan_turn"
	helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: 0, Skill: "helper"})
	helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: 0, Choice: "free"})
	data, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(data, &restored); err != nil || !reflect.DeepEqual(*s, restored) {
		t.Fatal("damaged road did not persist", err)
	}
	s = &restored
	for _, viewer := range []int{-1, 0, 1, 2} {
		v := s.View(viewer)["catan"].(map[string]any)
		legal := v["legal"].(map[string][]int)
		if (len(legal["repairRoads"]) > 0) != (viewer == 0) {
			t.Fatal("repair actions exposed to wrong seat")
		}
		if v["edges"].([]any)[0].(map[string]any)["damaged"] != true {
			t.Fatal("public damage missing")
		}
		for player, raw := range v["players"].([]any) {
			_, resources := raw.(map[string]any)["resources"]
			if resources != (player == viewer) {
				t.Fatal("repair view leaked hand")
			}
		}
	}
	helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: 0})
	helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: 0})
	if !s.Catan.Edges[2].Damaged || s.Catan.Edges[0].Damaged || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("repair affected wrong road or hand")
	}
	catanCheck(t, s)
}

func TestCatanEarthquakeSettlementIntactRoadOrShipException(t *testing.T) {
	for _, ship := range []bool{false, true} {
		s := earthquakeGraph(t, []int{0, 0, 0})
		g := s.Catan
		earthquakeDamage(t, s, 0, 0, 1)
		if ship {
			g.Seafarers = &CatanSeafarers{Pirate: -1}
			g.Edges[2].Ship = true
		}
		if g.canSettlement(0, 1, false) || !g.canSettlement(0, 2, false) {
			t.Fatal("FAQ22 intact connection exception", ship)
		}
		g.Edges[2].Owner = 1
		if g.canSettlement(0, 2, false) {
			t.Fatal("opponent connection permitted settlement")
		}
		g.Edges[2].Owner = 0
		helperGrant(s, 0, catanPrices["catan_settlement"])
		helperReject(t, s, 0, Action{Type: "catan_settlement", Vertex: 1})
		helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: 2})
		if s.Catan.Vertices[2].Level != 1 || !s.Catan.Edges[1].Damaged {
			t.Fatal("settlement exception altered damage")
		}
		catanCheck(t, s)
	}
}

func TestCatanEarthquakeRoadBuildingRepairsAndPieceLimit(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		for _, damaged := range [][]int{{0}, {0, 1}, {0, 1, 2}} {
			s := earthquakeGraph(t, make([]int, 15))
			earthquakeDamage(t, s, 0, damaged...)
			s.Phase = phase
			catanCard(s.Catan, 0, 1)
			helperApply(t, s, 0, Action{Type: "catan_dev", Card: 1})
			helperReject(t, s, 0, Action{Type: "catan_skip_roads"})
			for index, edge := range damaged[:min(2, len(damaged))] {
				restored := clone(*s)
				s = &restored
				helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: edge})
				if index == 0 && len(damaged) > 1 && (s.Phase != "catan_roads" || s.Catan.FreeRoads != 1) {
					t.Fatal("first repair lost remaining free action")
				}
			}
			roads, _, _ := s.Catan.pieces(0)
			if s.Phase != phase || s.Catan.FreeRoads != 0 || roads != 15 || sum(s.Catan.Players[0].Resources) != 0 || s.Catan.hasDamagedRoad(0) != (len(damaged) > 2) {
				t.Fatal("repair allowance, piece inventory or resume phase")
			}
			catanCheck(t, s)
		}
	}
}

func TestCatanEarthquakeFreeRepairThenBuildRoadOrShip(t *testing.T) {
	for _, ship := range []bool{false, true} {
		s := earthquakeGraph(t, []int{0, -1})
		earthquakeDamage(t, s, 0, 0)
		if ship {
			s.Catan.Seafarers = &CatanSeafarers{Pirate: -1}
			s.Catan.Tiles = append(s.Catan.Tiles, CatanTile{ID: 1, Resource: CatanSea})
			s.Catan.Edges[1].Tiles = []int{0, 1}
			s.Catan.Vertices[1].Owner, s.Catan.Vertices[1].Level = 0, 1
			if !s.Catan.canShip(0, 1) {
				t.Fatal("damaged road prevented ship construction")
			}
		}
		catanCard(s.Catan, 0, 1)
		helperApply(t, s, 0, Action{Type: "catan_dev", Card: 1})
		helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: 0})
		if s.Phase != "catan_roads" || s.Catan.FreeRoads != 1 {
			t.Fatal("repair did not leave second build")
		}
		kind := "catan_road"
		if ship {
			kind = "catan_ship"
		}
		helperApply(t, s, 0, Action{Type: kind, Edge: 1})
		if s.Phase != "catan_turn" || s.Catan.Edges[1].Owner != 0 || s.Catan.Edges[1].Ship != ship || sum(s.Catan.Players[0].Resources) != 0 {
			t.Fatal("mixed repair/build continuation")
		}
		catanCheck(t, s)
	}
}

func TestCatanEarthquakeBuildShipAndUpgradeCityBeforeRepair(t *testing.T) {
	s := earthquakeGraph(t, []int{0, -1})
	g := s.Catan
	g.Seafarers = &CatanSeafarers{Pirate: -1}
	g.Tiles = append(g.Tiles, CatanTile{ID: 1, Resource: CatanSea})
	g.Edges[1].Tiles = []int{0, 1}
	g.Vertices[1].Owner, g.Vertices[1].Level = 0, 1
	earthquakeDamage(t, s, 0, 0)
	helperGrant(s, 0, catanPrices["catan_ship"])
	helperGrant(s, 0, catanPrices["catan_city"])
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: 1})
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: 1})
	if !s.Catan.Edges[0].Damaged || !s.Catan.Edges[1].Ship || s.Catan.Vertices[1].Level != 2 {
		t.Fatal("damage blocked unrelated construction")
	}
	catanCheck(t, s)
}

func TestCatanEarthquakePairedPlayerCanRepair(t *testing.T) {
	s, err := NewCatan(6, CatanOptions{FiveSix: true})
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	g.Paired = &CatanPairedTurn{Primary: 0, Secondary: 3, Second: true}
	s.Turn, s.Phase = 3, "catan_turn"
	g.Edges[0].Owner = 3
	earthquakeDamage(t, s, 3, 0)
	helperGrant(s, 3, catanPrices["catan_repair_road"])
	helperReject(t, s, 0, Action{Type: "catan_repair_road", Edge: 0})
	helperApply(t, s, 3, Action{Type: "catan_repair_road", Edge: 0})
	if s.Catan.Edges[0].Damaged || !s.Catan.Paired.Second || s.Turn != 3 || s.Phase != "catan_turn" {
		t.Fatal("repair changed paired turn")
	}
	for color, bank := range s.Catan.Bank {
		if bank != 24 || s.Catan.Players[3].Resources[color] != 0 {
			t.Fatal("paired repair inventory")
		}
	}
}

func TestCatanEarthquakeCityKnightsDiplomacyAndProgressRepair(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1, 1})
	earthquakeDamage(t, s, 0, 0)
	earthquakeDamage(t, s, 1, 3)
	if slices.Contains(s.Catan.diplomacyRoads(), 0) || slices.Contains(s.Catan.diplomacyRoads(), 3) {
		t.Fatal("diplomat allowed a damaged road")
	}
	ckProgressGive(t, s, 0, 16, 7)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: 0})
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: 3})
	helperGrant(s, 0, catanPrices["catan_knight_recruit"])
	helperApply(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: 0})
	if s.Catan.knightAt(0) == nil {
		t.Fatal("damaged road prevented knight recruitment")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 7})
	helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: 0})
	if s.Phase != "catan_turn" || s.Catan.Edges[0].Damaged || !s.Catan.Edges[3].Damaged || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("progress repair outcome")
	}
	ckProgressStock(t, s.Catan)
	ckKnightStock(t, s.Catan)
}

func TestCatanEarthquakeRepairDoesNotExploreOrCollect(t *testing.T) {
	s, edge := fogTestGame(t, CatanGold)
	s.Catan.Edges[edge].Owner = 0
	earthquakeDamage(t, s, 0, edge)
	helperGrant(s, 0, catanPrices["catan_repair_road"])
	before := clone(*s.Catan.Seafarers.Fog)
	helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: edge})
	if !reflect.DeepEqual(before, *s.Catan.Seafarers.Fog) || s.Catan.Tiles[2].Resource != CatanFog || s.Catan.GoldPending != nil {
		t.Fatal("repair triggered route discovery")
	}
	s, edge = tribeGame(t)
	s.Catan.Edges[edge].Owner = 0
	s.Catan.tribe().Tokens = []int{edge}
	s.Catan.tribe().Ports = []CatanPort{{Edge: edge, Resource: 0}}
	tribeCard(t, s.Catan, edge, 0)
	earthquakeDamage(t, s, 0, edge)
	rewards := clone(*s.Catan.tribe())
	helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: edge})
	if !reflect.DeepEqual(rewards, *s.Catan.tribe()) {
		t.Fatal("repair collected new route rewards")
	}
}

func TestCatanEarthquakeBotsRepairTradeAndIgnoreHiddenHands(t *testing.T) {
	s := earthquakeGraph(t, []int{0, 0, -1})
	earthquakeDamage(t, s, 0, 0, 1)
	helperGrant(s, 0, []int{1, 1, 0, 0, 0})
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_repair_road" {
		t.Fatal("paid repair bot", a, err)
	}
	other := clone(*s)
	other.Catan.Players[1].Resources = []int{9, 8, 7, 6, 5}
	slices.Reverse(other.Catan.DevDeck)
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("repair bot read hidden data", err)
	}
	helperApply(t, s, 0, a)
	helperGrant(s, 0, []int{4, 1, 0, 0, 0})
	// Exactly enough to repair; use the missing-wood case to exercise bank trade.
	s.Catan.Bank[0] += 4
	s.Catan.Players[0].Resources[0] -= 4
	helperGrant(s, 0, []int{0, 0, 4, 0, 0})
	a, err = s.BotAction(0)
	if err != nil || a.Type != "catan_bank" || a.Take[0] != 1 {
		t.Fatal("bot did not trade toward blocked-road repair", a, err)
	}
	helperApply(t, s, 0, a)
	a, err = s.BotAction(0)
	if err != nil || a.Type != "catan_repair_road" {
		t.Fatal("bot failed to use repair resources", a, err)
	}
	helperApply(t, s, 0, a)
	catanCheck(t, s)
	s = earthquakeGraph(t, make([]int, 15))
	earthquakeDamage(t, s, 0, 0, 1)
	catanCard(s.Catan, 0, 1)
	helperApply(t, s, 0, Action{Type: "catan_dev", Card: 1})
	for range 2 {
		a, err = s.BotAction(0)
		if err != nil || a.Type != "catan_repair_road" {
			t.Fatal("free repair bot", a, err)
		}
		helperApply(t, s, 0, a)
	}
	if s.Phase != "catan_turn" || s.Catan.hasDamagedRoad(0) {
		t.Fatal("free repair bot stalled")
	}
}
