package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func seaTestGame(t *testing.T) *State {
	t.Helper()
	s := catanGame(t, 3)
	g := s.Catan
	g.SetupStep = 6
	g.TurnSerial = 1
	g.Seafarers = &CatanSeafarers{Pirate: -1}
	s.Turn = 0
	s.Phase = "catan_turn"
	// A small coastal fixture, not a substitute for any official scenario map.
	if err := g.makeScenarioMap([]CatanHexSpec{{0, 0, 0, 6}, {1, 0, CatanSea, 0}, {1, -1, CatanSea, 0}, {0, -1, 1, 5}, {-1, 0, 2, 9}, {-1, 1, CatanSea, 0}, {0, 1, CatanSea, 0}, {2, 0, 3, 8}}); err != nil {
		t.Fatal(err)
	}
	g.Robber = 0
	return s
}
func seaEdge(t *testing.T, g *Catan, first, second int) int {
	t.Helper()
	for _, e := range g.Edges {
		if slices.Contains(e.Tiles, first) && slices.Contains(e.Tiles, second) {
			return e.ID
		}
	}
	t.Fatal("no shared edge", first, second)
	return -1
}
func seaChain(t *testing.T, ships []bool) *State {
	s := seaTestGame(t)
	g := s.Catan
	g.Vertices = nil
	g.Edges = nil
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6}, {ID: 1, Resource: CatanSea}}
	for i := 0; i <= len(ships); i++ {
		g.Vertices = append(g.Vertices, CatanVertex{ID: i, Owner: -1})
	}
	for i, ship := range ships {
		g.Edges = append(g.Edges, CatanEdge{ID: i, A: i, B: i + 1, Owner: 0, Ship: ship, Tiles: []int{0, 1}})
	}
	return s
}
func TestCatanSeafarersHexTopologyAndLegacyAdjacency(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	for _, tile := range g.Tiles {
		if len(tile.Vertices) != 6 {
			t.Fatal("nonhex tile")
		}
		for k, a := range tile.Vertices {
			b := tile.Vertices[(k+1)%6]
			count := 0
			for _, e := range g.Edges {
				if (e.A == a && e.B == b) || (e.A == b && e.B == a) {
					count++
					if !slices.Contains(e.Tiles, tile.ID) {
						t.Fatal("missing adjacency")
					}
				}
			}
			if count != 1 {
				t.Fatal("edge missing or duplicated")
			}
		}
	}
	for _, e := range g.Edges {
		if len(e.Tiles) > 2 {
			t.Fatal("edge shared by more than two hexes")
		}
		g.Edges[e.ID].Tiles = nil
		if !reflect.DeepEqual(g.edgeTiles(e.ID), e.Tiles) {
			t.Fatal("legacy save adjacency differs")
		}
		g.Edges[e.ID].Tiles = e.Tiles
	}
	before, _ := json.Marshal(g)
	if err := g.makeScenarioMap([]CatanHexSpec{{0, 0, 0, 6}, {0, 0, 1, 8}}); err == nil {
		t.Fatal("duplicate hex accepted")
	}
	after, _ := json.Marshal(g)
	if string(before) != string(after) {
		t.Fatal("bad map partly replaced existing map")
	}
}
func TestCatanSeafarersLandSeaAndCoastRules(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	inland := seaEdge(t, g, 0, 3)
	sea := seaEdge(t, g, 1, 2)
	coast := seaEdge(t, g, 0, 1)
	for _, tc := range []struct {
		id         int
		road, ship bool
	}{{inland, true, false}, {sea, false, true}, {coast, true, true}} {
		for i := range g.Vertices {
			g.Vertices[i].Owner = -1
			g.Vertices[i].Level = 0
		}
		v := g.Edges[tc.id].A
		g.Vertices[v].Owner = 0
		g.Vertices[v].Level = 1
		if g.canRoad(0, tc.id) != tc.road || g.canShip(0, tc.id) != tc.ship {
			t.Fatal("wrong terrain", tc)
		}
		g.Edges[tc.id].Owner = 1
		if g.canRoad(0, tc.id) || g.canShip(0, tc.id) {
			t.Fatal("occupied coastal edge accepted")
		}
		g.Edges[tc.id].Owner = -1
	}
	for _, v := range g.Vertices {
		if !g.landVertex(v.ID) && g.canSettlement(0, v.ID, true) {
			t.Fatal("village allowed in ocean")
		}
	}
}
func TestCatanSeafarersRoadShipJunctionAndLongestRoute(t *testing.T) {
	s := seaChain(t, []bool{false, false, true, true, true})
	g := s.Catan
	if g.roadLength(0) != 3 {
		t.Fatal("road and ship connected without building")
	}
	g.Vertices[2].Owner = 0
	g.Vertices[2].Level = 1
	if g.roadLength(0) != 5 {
		t.Fatal("own building did not join modes")
	}
	g.Vertices[2].Owner = 1
	if g.roadLength(0) != 3 {
		t.Fatal("opponent building failed to cut route")
	}
	g.Vertices[2].Owner = -1
	g.Vertices[2].Level = 0
	g.Edges[2].Owner = -1
	if g.canShip(0, 2) != true {
		t.Fatal("existing ship did not connect")
	}
	g.Edges[3].Owner = -1
	if g.canShip(0, 2) {
		t.Fatal("ship connected to road without building")
	}
	if !g.canRoad(0, 2) {
		t.Fatal("road failed to connect to road")
	}
	g.Vertices[2].Owner = 0
	g.Vertices[2].Level = 1
	if !g.canShip(0, 2) {
		t.Fatal("coastal own building cannot start ship")
	}
	g.Vertices[2].Owner = 1
	if g.canShip(0, 2) || g.canRoad(0, 2) {
		t.Fatal("built through opponent")
	}
}
func TestCatanSeafarersShipPaymentSupplyAndMovePersistence(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	edge := seaEdge(t, g, 0, 1)
	g.Vertices[g.Edges[edge].A].Owner = 0
	g.Vertices[g.Edges[edge].A].Level = 1
	helperGrant(s, 0, []int{2, 0, 2, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	g = s.Catan
	if g.shipCount(0) != 1 || !reflect.DeepEqual(g.Players[0].Resources, []int{1, 0, 1, 0, 0}) {
		t.Fatal("wrong ship cost")
	}
	roads, _, _ := g.pieces(0)
	if roads != 0 {
		t.Fatal("ship consumed road supply")
	}
	if g.movableShip(0, edge) {
		t.Fatal("new ship movable")
	}
	helperReject(t, s, 0, Action{Type: "catan_move_ship", Edge: edge, Target: edge})
	helperApply(t, s, 0, Action{Type: "catan_end"})
	s.Turn = 0
	s.Phase = "catan_turn"
	destinations := s.Catan.shipDestinations(0, edge)
	if len(destinations) == 0 {
		t.Fatal("old open ship cannot move")
	}
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_move_ship", Edge: edge, Target: destinations[0]})
	if s.Catan.Edges[edge].Owner != -1 || s.Catan.Edges[edge].Ship || !s.Catan.Edges[destinations[0]].Ship || !s.Catan.Seafarers.MovedShip {
		t.Fatal("ship not moved")
	}
	restored = clone(*s)
	s = &restored
	helperReject(t, s, 0, Action{Type: "catan_move_ship", Edge: destinations[0], Target: edge})
	if !reflect.DeepEqual(s.Catan.Players[0].Resources, []int{1, 0, 1, 0, 0}) {
		t.Fatal("moving charged resources")
	}
	catanCheck(t, s)
}
func TestCatanSeafarersClosedShipsAndPirateRestrictions(t *testing.T) {
	s := seaChain(t, []bool{true, true, true, true})
	g := s.Catan
	g.Vertices[0].Owner = 0
	g.Vertices[0].Level = 1
	g.Vertices[4].Owner = 0
	g.Vertices[4].Level = 1
	// An opponent's inserted building must not release a previously closed line.
	g.Vertices[2].Owner = 1
	g.Vertices[2].Level = 1
	for _, e := range g.Edges {
		if g.movableShip(0, e.ID) {
			t.Fatal("closed route released by opponent building")
		}
	}
	g.Vertices[4].Owner = -1
	g.Vertices[4].Level = 0
	if !g.movableShip(0, 3) || g.movableShip(0, 1) {
		t.Fatal("open/middle ship classification")
	}
	g.Seafarers.Pirate = 1
	if g.movableShip(0, 3) {
		t.Fatal("pirate failed to lock departure")
	}
	s = seaTestGame(t)
	g = s.Catan
	edge := seaEdge(t, g, 0, 1)
	g.Vertices[g.Edges[edge].A].Owner = 0
	g.Vertices[g.Edges[edge].A].Level = 1
	g.Seafarers.Pirate = 1
	if g.canShip(0, edge) || !g.canRoad(0, edge) {
		t.Fatal("pirate blocks wrong route types")
	}
	helperGrant(s, 0, []int{1, 0, 1, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_ship", Edge: edge})
}
func TestCatanSeafarersRoadBuildingCanMixAndUseShipAfterRoadLimit(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	coast := seaEdge(t, g, 0, 1)
	v := g.Edges[coast].A
	g.Vertices[v].Owner = 0
	g.Vertices[v].Level = 1
	catanCard(g, 0, 1)
	helperApply(t, s, 0, Action{Type: "catan_dev", Card: 1})
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: coast})
	road := -1
	for _, e := range s.Catan.Edges {
		if s.Catan.canRoad(0, e.ID) {
			road = e.ID
			break
		}
	}
	if road < 0 {
		t.Fatal("no fixture road")
	}
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: road})
	if s.Phase != "catan_turn" || s.Catan.FreeRoads != 0 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("mixed free construction failed")
	}
	s = seaChain(t, make([]bool, 15))
	g = s.Catan
	g.Vertices[15].Owner = 0
	g.Vertices[15].Level = 1
	g.Vertices = append(g.Vertices, CatanVertex{ID: 16, Owner: -1})
	g.Edges = append(g.Edges, CatanEdge{ID: 15, A: 15, B: 16, Owner: -1, Tiles: []int{0, 1}})
	if g.hasRoad(0) || !g.hasRoute(0) {
		t.Fatal("road supply prevented ship construction")
	}
	catanCard(g, 0, 1)
	helperApply(t, s, 0, Action{Type: "catan_dev", Card: 1})
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: 15})
	if s.Phase != "catan_turn" || s.Catan.shipCount(0) != 1 {
		t.Fatal("free route did not finish when all positions exhausted")
	}
}
func TestCatanSeafarersPirateVictimsAndFrame(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	s.Phase = "catan_robber"
	g.ResumePhase = "catan_turn"
	coast := seaEdge(t, g, 0, 1)
	sea := seaEdge(t, g, 1, 2)
	g.Edges[coast].Owner = 1
	g.Edges[coast].Ship = true
	g.Edges[sea].Owner = 2
	g.Edges[sea].Ship = true
	helperGrant(s, 1, []int{1, 0, 0, 0, 0})
	helperGrant(s, 2, []int{0, 1, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_pirate", Tile: 0})
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 1})
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	if s.Phase != "catan_steal" || len(s.Catan.Victims) != 2 || s.Catan.Robber != 0 {
		t.Fatal("wrong pirate victim set or robber moved")
	}
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_steal", Target: 2})
	if s.Catan.Players[0].Resources[1] != 1 || s.Phase != "catan_turn" {
		t.Fatal("pirate theft failed")
	}
	s.Phase = "catan_robber"
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: -1})
	if s.Catan.Seafarers.Pirate != -1 || len(s.Catan.Victims) != 0 || s.Phase != "catan_turn" {
		t.Fatal("frame move stole or blocked")
	}
	catanCheck(t, s)
}
func TestCatanSeafarersPirateIgnoresRoadsAndBuildings(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	s.Phase = "catan_robber"
	g.ResumePhase = "catan_turn"
	edge := seaEdge(t, g, 0, 1)
	g.Edges[edge].Owner = 1
	g.Vertices[g.Edges[edge].A].Owner = 1
	g.Vertices[g.Edges[edge].A].Level = 1
	helperGrant(s, 1, []int{1, 0, 0, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	if s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("pirate robbed coastal building/road owner")
	}
}
func TestCatanSeafarersViewAndBotActions(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	edge := seaEdge(t, g, 0, 1)
	g.Vertices[g.Edges[edge].A].Owner = 0
	g.Vertices[g.Edges[edge].A].Level = 1
	helperGrant(s, 0, []int{1, 0, 1, 0, 0})
	view := s.View(0)["catan"].(map[string]any)
	legal := view["legal"].(map[string][]int)
	if !slices.Contains(legal["ships"], edge) {
		t.Fatal("ship hint missing")
	}
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_ship" {
		t.Fatal("bot did not use affordable ships", a, err)
	}
	helperApply(t, s, 0, a)
	if len(s.View(-1)["catan"].(map[string]any)["legal"].(map[string][]int)["ships"]) != 0 {
		t.Fatal("spectator given actions")
	}
	s.Phase = "catan_robber"
	s.Catan.ResumePhase = "catan_turn"
	for _, tile := range s.Catan.Tiles {
		if tile.Resource != CatanSea {
			continue
		}
		for _, e := range s.Catan.Edges {
			if slices.Contains(e.Tiles, tile.ID) {
				s.Catan.Edges[e.ID].Owner = 1
				s.Catan.Edges[e.ID].Ship = true
			}
		}
		break
	}
	helperGrant(s, 1, []int{1, 0, 0, 0, 0})
	a, err = s.BotAction(0)
	if err != nil || a.Type != "catan_pirate" {
		t.Fatal("bot failed pirate alternative", a, err)
	}
	helperApply(t, s, 0, a)
}
func TestCatanSeafarersPairedPhasesResetShipLocks(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	g.Paired = &CatanPairedTurn{Primary: 0, Secondary: 2}
	g.Seafarers.MovedShip = true
	g.Seafarers.BuiltShips = []int{1}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	if s.Turn != 2 || s.Phase != "catan_turn" || s.Catan.Seafarers.MovedShip || len(s.Catan.Seafarers.BuiltShips) > 0 {
		t.Fatal("second player inherited first player's ship locks")
	}
	s.Catan.Seafarers.MovedShip = true
	helperApply(t, s, 2, Action{Type: "catan_end"})
	if s.Turn != 1 || s.Phase != "catan_roll" || s.Catan.Seafarers.MovedShip {
		t.Fatal("next primary inherited ship locks")
	}
}
func TestCatanSeafarersStartingShipAndHelperCannotMoveIt(t *testing.T) {
	s := seaTestGame(t)
	g := s.Catan
	g.SetupStep = 0
	s.Phase = "catan_setup_settlement"
	edge := seaEdge(t, g, 0, 1)
	vertex := g.Edges[edge].A
	helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: vertex})
	helperApply(t, s, 0, Action{Type: "catan_ship", Edge: edge})
	if s.Turn != 1 || s.Catan.SetupStep != 1 || !s.Catan.Edges[edge].Ship {
		t.Fatal("starting ship failed")
	}
	if s.Catan.helperEndRoad(0, edge) {
		t.Fatal("road helper treats ship as road")
	}
}

func TestCatanSeafarersShoresFourFixedComponents(t *testing.T) {
	s := catanGame(t, 4)
	g := s.Catan
	if err := g.makeSeafarersShoresFour(); err != nil {
		t.Fatal(err)
	}
	terrain := make([]int, 8)
	numbers := make([]int, 13)
	for _, tile := range g.Tiles {
		terrain[tile.Resource]++
		numbers[tile.Number]++
	}
	if len(g.Tiles) != 42 || !reflect.DeepEqual(terrain, []int{5, 5, 5, 5, 5, 1, 14, 2}) {
		t.Fatal("incorrect fixed-map terrain components", terrain)
	}
	if !reflect.DeepEqual(numbers, []int{15, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 1}) {
		t.Fatal("incorrect number discs", numbers)
	}
	if len(g.Ports) != 9 || g.Tiles[g.Robber].Resource != CatanDesert || g.Seafarers.Pirate != -1 {
		t.Fatal("fixed marker placement")
	}
	resources := map[int]int{}
	used := map[int]bool{}
	for _, port := range g.Ports {
		resources[port.Resource]++
		if !g.edgeTerrain(port.Edge, true) || !g.edgeTerrain(port.Edge, false) {
			t.Fatal("port is not on coast")
		}
		for _, v := range []int{g.Edges[port.Edge].A, g.Edges[port.Edge].B} {
			if used[v] {
				t.Fatal("ports share vertex")
			}
			used[v] = true
		}
	}
	if !reflect.DeepEqual(resources, map[int]int{-1: 4, 0: 1, 1: 1, 2: 1, 3: 1, 4: 1}) {
		t.Fatal("incorrect port inventory")
	}
	visited := map[int]bool{}
	sizes := []int{}
	for _, tile := range g.Tiles {
		if visited[tile.ID] || tile.Resource == CatanSea {
			continue
		}
		queue := []int{tile.ID}
		visited[tile.ID] = true
		size := 0
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			size++
			for _, e := range g.Edges {
				if !slices.Contains(e.Tiles, id) {
					continue
				}
				for _, next := range e.Tiles {
					if !visited[next] && g.Tiles[next].Resource != CatanSea {
						visited[next] = true
						queue = append(queue, next)
					}
				}
			}
		}
		sizes = append(sizes, size)
	}
	slices.Sort(sizes)
	if !reflect.DeepEqual(sizes, []int{2, 2, 5, 19}) {
		t.Fatal("incorrect island outlines", sizes)
	}
	for _, v := range g.Vertices {
		if len(g.touching(v.ID)) > 3 {
			t.Fatal("nonphysical hex intersection")
		}
	}
}

func TestCatanSeafarersShoresSetupAndIslandBonuses(t *testing.T) {
	s := catanGame(t, 4)
	if err := s.Catan.makeSeafarersShoresFour(); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	small := -1
	for _, v := range g.Vertices {
		if g.islandAt(v.ID) >= 0 && !g.seaSetupAllowed(v.ID) {
			small = v.ID
			break
		}
	}
	if small < 0 {
		t.Fatal("no small island")
	}
	helperReject(t, s, s.Turn, Action{Type: "catan_settlement", Vertex: small})
	for s.Catan.setup() {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, s.Turn, a)
	}
	g = s.Catan
	for i, p := range g.Seafarers.Seats {
		if !reflect.DeepEqual(p.HomeIslands, g.Seafarers.StartIslands) || p.IslandPoints != 0 {
			t.Fatal("wrong home island or awarded setup", i, p)
		}
	}
	s.Phase = "catan_turn"
	s.Turn = 0
	edge := -1
	for _, id := range g.touching(small) {
		if g.edgeTerrain(id, true) {
			edge = id
			break
		}
	}
	if edge < 0 {
		t.Fatal("no coastal approach")
	}
	g.Edges[edge].Owner = 0
	g.Edges[edge].Ship = true
	helperGrant(s, 0, []int{1, 1, 1, 1, 0})
	helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: small})
	if s.Catan.Seafarers.Seats[0].IslandPoints != 2 {
		t.Fatal("missing arrival bonus")
	}
	points := s.Catan.Players[0].Score
	s.catanSettleIsland(0, small, false)
	s.catanScores()
	if s.Catan.Players[0].Score != points {
		t.Fatal("duplicate island bonus")
	}
	// A second player has an independent discovery: another player's arrival
	// does not consume this island's victory points.
	s.catanSettleIsland(1, small, false)
	if s.Catan.Seafarers.Seats[1].IslandPoints != 2 {
		t.Fatal("bonus incorrectly exclusive")
	}
	restored := clone(*s)
	s = &restored
	s.Catan.Players[0].Score = 13
	s.catanVictory()
	if s.Finished {
		t.Fatal("base ten-point goal used for shores")
	}
	s.Catan.Players[0].Score = 14
	s.catanVictory()
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) {
		t.Fatal("fourteen-point goal failed")
	}
}

func TestCatanSeafarersMoveRechecksConnectivityAfterRemoval(t *testing.T) {
	s := seaChain(t, []bool{true, true})
	g := s.Catan
	g.Vertices[0].Owner = 0
	g.Vertices[0].Level = 1
	g.Vertices = append(g.Vertices, CatanVertex{ID: 3, Owner: -1}, CatanVertex{ID: 4, Owner: -1})
	g.Edges = append(g.Edges, CatanEdge{ID: 2, A: 2, B: 3, Owner: -1, Tiles: []int{1}}, CatanEdge{ID: 3, A: 0, B: 4, Owner: -1, Tiles: []int{1}})
	if !g.canShip(0, 2) {
		t.Fatal("fixture extension not connected before removal")
	}
	if !reflect.DeepEqual(g.shipDestinations(0, 1), []int{3}) {
		t.Fatal("moving relied on the removed ship for connection", g.shipDestinations(0, 1))
	}
	helperReject(t, s, 0, Action{Type: "catan_move_ship", Edge: 1, Target: 2})
	helperApply(t, s, 0, Action{Type: "catan_move_ship", Edge: 1, Target: 3})
}
func TestCatanSeafarersFifteenShipsDoNotConsumeRoadSupply(t *testing.T) {
	ships := make([]bool, 15)
	for i := range ships {
		ships[i] = true
	}
	s := seaChain(t, ships)
	g := s.Catan
	g.Vertices[15].Owner = 0
	g.Vertices[15].Level = 1
	g.Vertices = append(g.Vertices, CatanVertex{ID: 16, Owner: -1})
	g.Edges = append(g.Edges, CatanEdge{ID: 15, A: 15, B: 16, Owner: -1, Tiles: []int{0, 1}})
	helperGrant(s, 0, []int{1, 1, 1, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_ship", Edge: 15})
	helperApply(t, s, 0, Action{Type: "catan_road", Edge: 15})
	roads, _, _ := s.Catan.pieces(0)
	if roads != 1 || s.Catan.shipCount(0) != 15 || s.Catan.Players[0].Resources[2] != 1 {
		t.Fatal("supplies mixed")
	}
}
func TestCatanSeafarersKnightCanMovePirateBeforeRoll(t *testing.T) {
	s := seaTestGame(t)
	s.Phase = "catan_roll"
	catanCard(s.Catan, 0, 0)
	helperApply(t, s, 0, Action{Type: "catan_dev", Card: 0})
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	if s.Phase != "catan_roll" || !s.Catan.PlayedDev || s.Catan.Players[0].Knights != 1 {
		t.Fatal("knight pirate movement lost pre-roll resume")
	}
}
