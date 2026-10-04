package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Engine-only fixture. Official maps are tested separately once transcribed.
func clothTestGame(t *testing.T) *State {
	t.Helper()
	s := seaChain(t, []bool{true, true, true})
	g := s.Catan
	g.SetupStep = 9
	g.Seafarers.Scenario = "cloth"
	g.Seafarers.VictoryPoints = 14
	g.Seafarers.Cloth = &CatanClothState{Stock: 10, Held: make([]int, 3), HomeTiles: []int{0}, EmptyLimit: 5, Villages: []CatanClothVillage{{Vertex: 2, Number: 6, Stock: 5}}}
	g.Tiles[0].Vertices = []int{0}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	g.ResumePhase = "catan_turn"
	return s
}
func clothTotal(g *Catan) int {
	c := g.cloth()
	n := c.Stock + sum(c.Held)
	for _, v := range c.Villages {
		n += v.Stock
	}
	return n
}
func TestCatanClothThreePassSetupAndHelpers(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := NewCatan(n, CatanOptions{FiveSix: n > 4, Helpers: true, AllHelpers: true})
			if err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			homes := []int{}
			for _, tile := range g.Tiles {
				homes = append(homes, tile.ID)
			}
			g.Seafarers = &CatanSeafarers{Pirate: -1, VictoryPoints: 14, Cloth: &CatanClothState{Held: make([]int, n), HomeTiles: homes, EmptyLimit: 5}}
			g.StartPlayer = n - 1
			s.Turn = g.StartPlayer
			for step := 0; step < 3*n; step++ {
				g = s.Catan
				want := (g.StartPlayer + step) % n
				if step >= n && step < 2*n {
					want = (g.StartPlayer + 2*n - 1 - step) % n
				}
				if s.Turn != want || !g.setup() {
					t.Fatalf("step %d turn %d want %d", step, s.Turn, want)
				}
				if step == 2*n {
					for i := range g.Players {
						g.Players[i].Helper.Moon = true
					}
				}
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				if a.Type != "catan_settlement" {
					t.Fatal(a)
				}
				expected := make([]int, 5)
				if step >= 2*n {
					for _, tile := range g.Tiles {
						if tile.Resource < 5 && slices.Contains(tile.Vertices, a.Vertex) {
							expected[tile.Resource]++
						}
					}
				}
				helperApply(t, s, want, a)
				if !reflect.DeepEqual(s.Catan.Players[want].Resources, expected) {
					t.Fatal("starting resources from wrong settlement", step, s.Catan.Players[want].Resources, expected)
				}
				// Only after the second route is the initial helper awarded.
				if step < n && s.Catan.Players[want].Helper != nil {
					t.Fatal("early helper")
				}
				s.AutoCatanPending()
				if s.Catan.SetupStep != step+1 {
					t.Fatal("setup timeout did not finish exactly one group", step, s.Phase)
				}
				if step >= n && step < 2*n {
					h := s.Catan.Players[want].Helper
					if h == nil || h.ID != (want-s.Catan.StartPlayer+n)%n+1 || h.Moon {
						t.Fatal("wrong second-pass helper", h)
					}
				}
				if step >= 2*n && !s.Catan.Players[want].Helper.Moon {
					t.Fatal("third pass overwrote helper")
				}
				restored := clone(*s)
				s = &restored
			}
			if s.Catan.setup() || s.Phase != "catan_roll" || s.Turn != n-1 || s.Catan.TurnSerial != 1 {
				t.Fatal("setup did not finish")
			}
			for i, p := range s.Catan.Players {
				_, settlements, _ := s.Catan.pieces(i)
				if settlements != 3 || p.Score != 3 {
					t.Fatal("missing starting settlement/score", i, settlements, p.Score)
				}
			}
			if s.View(-1)["catan"].(map[string]any)["setupLimit"] != 3*n {
				t.Fatal("missing public setup length")
			}
		})
	}
}
func TestCatanClothTradeRoutesIndependentAndPersistent(t *testing.T) {
	s := clothTestGame(t)
	g := s.Catan
	c := g.cloth()
	total := clothTotal(g)
	g.Edges[0].Ship = false
	s.catanClothTrade(0)
	if c.Held[0] != 0 || g.pirateAllowed(0) {
		t.Fatal("road joined directly to ship")
	}
	g.Vertices[1].Owner, g.Vertices[1].Level = 0, 1
	s.catanClothTrade(0)
	s.catanClothTrade(0)
	if c.Held[0] != 1 || c.Villages[0].Stock != 4 || !g.pirateAllowed(0) {
		t.Fatal("trade missing/duplicated")
	}
	g.Vertices[1].Owner = 1
	// New players need their own ship, not an opponent's adjoining route.
	s.catanClothTrade(1)
	if c.Held[1] != 0 {
		t.Fatal("opponent ship used")
	}
	g.Edges[1].Owner = 1
	s.catanClothTrade(1)
	if c.Held[1] != 1 || !reflect.DeepEqual(c.Villages[0].Traders, []int{0, 1}) {
		t.Fatal("relations not independent")
	}
	if err := s.catanProduceCloth(6); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Held, []int{2, 2, 0}) || clothTotal(g) != total {
		t.Fatal("existing relation lost or cloth not conserved")
	}
}
func TestCatanClothOpponentBuildingStopsNewRelation(t *testing.T) {
	s := clothTestGame(t)
	g := s.Catan
	g.Vertices[1].Owner, g.Vertices[1].Level = 1, 1
	s.catanClothTrade(0)
	if g.cloth().Held[0] != 0 {
		t.Fatal("passed opponent building")
	}
}
func TestCatanClothEmptyVillageRelationAndClosedRoute(t *testing.T) {
	s := clothTestGame(t)
	g := s.Catan
	c := g.cloth()
	c.Villages[0].Stock = 0
	s.catanClothTrade(0)
	if c.Held[0] != 0 || !g.pirateAllowed(0) {
		t.Fatal("empty village relation")
	}
	for _, edge := range []int{0, 1} {
		if g.movableShip(0, edge) {
			t.Fatal("closed route ship movable", edge)
		}
	}
	if !g.movableShip(0, 2) {
		t.Fatal("branch cannot move")
	}
	// Remove branch so the village is itself the end of the route.
	g.Edges[2].Owner = -1
	if g.movableShip(0, 1) {
		t.Fatal("village is not a closed endpoint")
	}
	if err := s.catanProduceCloth(6); err != nil {
		t.Fatal(err)
	}
	if c.Stock != 10 || sum(c.Held) != 0 {
		t.Fatal("empty village took common stock")
	}
}
func TestCatanClothProductionShortageEliminationAndDuplicates(t *testing.T) {
	s := clothTestGame(t)
	g := s.Catan
	c := g.cloth()
	c.Villages = []CatanClothVillage{{Vertex: 2, Number: 6, Stock: 1, Traders: []int{0, 1, 1, 2}}, {Vertex: 3, Number: 6, Stock: 2, Traders: []int{0, 1}}, {Vertex: 0, Number: 8, Stock: 5, Traders: []int{0}}}
	g.Players[2].Eliminated = true
	total := clothTotal(g)
	if err := s.catanRoll(6); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Held, []int{2, 2, 0}) || c.Stock != 9 || c.Villages[0].Stock != 0 || c.Villages[1].Stock != 0 || c.Villages[2].Stock != 5 {
		t.Fatal("bad production", c)
	}
	if g.Players[0].Score != 2 || g.Players[1].Score != 1 || clothTotal(g) != total || s.Finished {
		t.Fatal("score/conservation/premature depletion end")
	}
	if err := s.catanRoll(6); err != nil {
		t.Fatal(err)
	}
	if c.Stock != 9 || !reflect.DeepEqual(c.Held, []int{2, 2, 0}) {
		t.Fatal("empty villages produced again")
	}
}
func TestCatanClothProductionCountsAsNoResourceForHilda(t *testing.T) {
	s := clothTestGame(t)
	g := s.Catan
	g.Options.Helpers = true
	g.TurnSerial = 2
	g.Players[1].Helper = &CatanHelperSeat{ID: 3}
	g.cloth().Villages[0].Traders = []int{1}
	if err := s.catanRoll(6); err != nil {
		t.Fatal(err)
	}
	if g.cloth().Held[1] != 1 || g.HelperPending == nil || g.HelperPending.Player != 1 {
		t.Fatal("cloth incorrectly suppressed Hilda")
	}
}
func TestCatanClothLongestRouteDisabledAndArmyRetained(t *testing.T) {
	s := seaChain(t, []bool{true, true, true, true, true, true})
	g := s.Catan
	g.Seafarers.Cloth = &CatanClothState{Held: []int{3, 0, 0}}
	g.Players[0].Knights = 3
	s.catanScores()
	if g.LongestOwner != -1 || g.ArmyOwner != 0 || g.Players[0].RoadLength != 6 || g.Players[0].Score != 3 {
		t.Fatal("wrong awards", g.Players[0], g.LongestOwner, g.ArmyOwner)
	}
}
func TestCatanClothProductionVictoryOnlyCurrentPlayer(t *testing.T) {
	for _, turn := range []int{0, 1} {
		s := clothTestGame(t)
		g := s.Catan
		c := g.cloth()
		c.Held[0] = 25
		c.Villages[0].Traders = []int{0}
		s.Turn = turn
		g.Options.Helpers = true
		g.TurnSerial = 3
		g.Players[2].Helper = &CatanHelperSeat{ID: 3}
		if err := s.catanRoll(6); err != nil {
			t.Fatal(err)
		}
		if g.Players[0].Score != 14 || s.Finished != (turn == 0) {
			t.Fatal("wrong own-turn victory", turn)
		}
		if turn == 0 && (g.HelperPending != nil || !reflect.DeepEqual(s.Winners, []int{0})) {
			t.Fatal("optional helper delayed victory")
		}
	}
}
func TestCatanClothRouteActionsCollectAndWin(t *testing.T) {
	for _, kind := range []string{"paid", "free", "move"} {
		t.Run(kind, func(t *testing.T) {
			s := clothTestGame(t)
			g := s.Catan
			g.Edges[1].Owner = -1
			g.Edges[1].Ship = false
			g.Edges[2].Owner = -1
			g.Edges[2].Ship = false
			g.cloth().Held[0] = 25
			a := Action{Type: "catan_ship", Edge: 1}
			switch kind {
			case "paid":
				helperGrant(s, 0, []int{1, 0, 1, 0, 0})
			case "free":
				s.Phase = "catan_roads"
				g.FreeRoads = 2
			case "move":
				g.Edges[2].A = 1
				g.Edges[2].Owner = 0
				g.Edges[2].Ship = true
				a = Action{Type: "catan_move_ship", Edge: 2, Target: 1}
			}
			helperApply(t, s, 0, a)
			if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) || s.Catan.cloth().Held[0] != 26 {
				t.Fatal("route did not collect/win", s.Phase)
			}
		})
	}
}
func TestCatanClothSettlingRoadShipJunctionEstablishesTrade(t *testing.T) {
	s := clothTestGame(t)
	g := s.Catan
	// Move the home building back, leaving two road edges to the new coastal site.
	g.Vertices[0].Owner = -1
	g.Vertices[0].Level = 0
	g.Vertices = append(g.Vertices, CatanVertex{ID: 4, Owner: 0, Level: 1}, CatanVertex{ID: 5, Owner: -1})
	g.Edges = append(g.Edges, CatanEdge{ID: 3, A: 4, B: 5, Owner: 0}, CatanEdge{ID: 4, A: 5, B: 0, Owner: 0})
	s.catanClothTrade(0)
	if g.cloth().Held[0] != 0 {
		t.Fatal("roads directly established relation")
	}
	helperGrant(s, 0, catanPrices["catan_settlement"])
	helperApply(t, s, 0, Action{Type: "catan_settlement", Vertex: 0})
	if s.Catan.cloth().Held[0] != 1 {
		t.Fatal("coastal settlement did not join shipping route")
	}
}
func TestCatanClothDepletionEndTieBreakAndPriority(t *testing.T) {
	for _, scenario := range []string{"score", "cloth", "tie", "immediate"} {
		t.Run(scenario, func(t *testing.T) {
			s := clothTestGame(t)
			g := s.Catan
			c := g.cloth()
			c.Villages = make([]CatanClothVillage, 5)
			c.Held = []int{4, 4, 0} // Player 0 has a building, player 1 a hidden VP.
			g.Players[1].Dev[4] = 1
			want := []int{0}
			switch scenario {
			case "score":
				c.Held[1] = 8
				want = []int{1}
			case "cloth":
				c.Held[1] = 5
				want = []int{1}
			case "tie":
				want = []int{0, 1}
			case "immediate":
				c.Held = []int{26, 30, 0}
			}
			s.catanScores()
			if s.Finished {
				t.Fatal("depletion ended before turn end")
			}
			helperApply(t, s, 0, Action{Type: "catan_end"})
			if !s.Finished || !reflect.DeepEqual(s.Winners, want) {
				t.Fatal("wrong ending", scenario, s.Winners, want)
			}
		})
	}
}
func TestCatanClothSmallIslandsForbiddenForBuildingsRobberAndDigur(t *testing.T) {
	s := clothTestGame(t)
	g := s.Catan
	g.Tiles = append(g.Tiles, CatanTile{ID: 2, Resource: CatanDesert, Vertices: []int{2}})
	if g.landVertex(2) || g.robberAllowed(2) || !g.landVertex(0) {
		t.Fatal("small island allowed")
	}
	s.Phase = "catan_robber"
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 2})
	helperReject(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	v := s.View(0)["catan"].(map[string]any)
	if len(v["legal"].(map[string][]int)["pirate"]) != 0 || len(g.pirateBotChoices(0)) != 0 {
		t.Fatal("unearned pirate offered")
	}
	g.Options.Helpers = true
	g.TurnSerial = 2
	g.Players[0].Helper = &CatanHelperSeat{ID: 10}
	s.Phase = "catan_turn"
	helperReject(t, s, 0, Action{Type: "catan_helper"})
	helperReject(t, s, 0, Action{Type: "catan_helper", Choice: "desert", Tile: 2})
	if _, ok := g.digurBotAction(0); ok {
		t.Fatal("Digur bypasses cloth islands")
	}
	s.catanClothTrade(0)
	s.Phase = "catan_robber"
	if len(s.View(0)["catan"].(map[string]any)["legal"].(map[string][]int)["pirate"]) == 0 {
		t.Fatal("earned pirate absent")
	}
}
func clothTheftFixture(t *testing.T) *State {
	s := clothTestGame(t)
	g := s.Catan
	g.cloth().Villages[0].Traders = []int{0}
	g.cloth().Held = []int{1, 2, 0}
	g.Edges[0].Owner = 1
	g.Edges[1].Owner = -1
	g.Edges[2].Owner = -1
	helperGrant(s, 1, []int{0, 1, 0, 0, 0})
	s.Phase = "catan_robber"
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	if s.Phase != "catan_cloth_steal" || s.CatanPendingActor() != 0 || !reflect.DeepEqual(s.Catan.Victims, []int{1}) {
		t.Fatal("single victim must still choose kind")
	}
	return s
}
func TestCatanClothTheftChoiceRestoreAndTimeout(t *testing.T) {
	for _, kind := range []string{"cloth", "resource", "timeout"} {
		t.Run(kind, func(t *testing.T) {
			s := clothTheftFixture(t)
			total := clothTotal(s.Catan)
			helperReject(t, s, 1, Action{Type: "catan_cloth_steal", Target: 1, Choice: "cloth"})
			helperReject(t, s, 0, Action{Type: "catan_cloth_steal", Target: 2, Choice: "cloth"})
			helperReject(t, s, 0, Action{Type: "catan_cloth_steal", Target: 1, Choice: "both"})
			helperReject(t, s, 0, Action{Type: "catan_steal", Target: 1})
			helperReject(t, s, 0, Action{Type: "catan_end"})
			if err := s.EliminateCatan(0); err == nil {
				t.Fatal("eliminated mandatory responder")
			}
			before, _ := json.Marshal(s)
			restored := clone(*s)
			s = &restored
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("save lost theft")
			}
			if kind == "timeout" {
				s.AutoCatanPending()
			} else {
				helperApply(t, s, 0, Action{Type: "catan_cloth_steal", Target: 1, Choice: kind})
			}
			if s.Phase != "catan_turn" || s.CatanPendingActor() != -1 || len(s.Catan.Victims) != 0 || clothTotal(s.Catan) != total {
				t.Fatal("theft did not resume/conserve")
			}
			if kind == "resource" {
				if s.Catan.Players[0].Resources[1] != 1 || s.Catan.cloth().Held[0] != 1 {
					t.Fatal("resource theft")
				}
			} else if !reflect.DeepEqual(s.Catan.cloth().Held, []int{2, 1, 0}) || s.Catan.Players[0].Score != 2 {
				t.Fatal("cloth theft score")
			}
		})
	}
}
func TestCatanClothTheftOnlyClothOrOnlyResource(t *testing.T) {
	for _, cloth := range []bool{false, true} {
		s := clothTheftFixture(t)
		g := s.Catan
		if cloth {
			catanMove(g.Players[1].Resources, g.Bank, append([]int{}, g.Players[1].Resources...))
		} else {
			g.cloth().Held[1] = 0
		}
		bad := "cloth"
		if cloth {
			bad = "resource"
		}
		helperReject(t, s, 0, Action{Type: "catan_cloth_steal", Target: 1, Choice: bad})
		// Repeat pirate movement to verify target eligibility with just one loot kind.
		s.Phase = "catan_robber"
		g.Seafarers.Pirate = -1
		helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
		s.AutoCatanPending()
		if s.Phase != "catan_turn" {
			t.Fatal("mandatory theft stuck")
		}
	}
}
func TestCatanClothBotUsesPublicInformation(t *testing.T) {
	s := clothTheftFixture(t)
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	changed := clone(*s)
	changed.Catan.Players[1].Resources = []int{0, 0, 0, 1, 0}
	changed.Catan.Players[1].Dev = []int{0, 0, 0, 0, 4}
	b, err := changed.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("theft bot reads hidden identities")
	}
	if s.Catan.seaRouteValue(0, 1, true) != changed.Catan.seaRouteValue(0, 1, true) {
		t.Fatal("route valuation reads private hand")
	}
}

func TestCatanClothTheftBeforeRollCanWin(t *testing.T) {
	s := clothTheftFixture(t)
	s.Catan.ResumePhase = "catan_roll"
	s.Catan.cloth().Held[0] = 25
	helperApply(t, s, 0, Action{Type: "catan_cloth_steal", Target: 1, Choice: "cloth"})
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) {
		t.Fatal("pre-roll cloth theft did not win")
	}
	s = clothTheftFixture(t)
	s.Catan.ResumePhase = "catan_roll"
	helperApply(t, s, 0, Action{Type: "catan_cloth_steal", Target: 1, Choice: "resource"})
	if s.Phase != "catan_roll" {
		t.Fatal("pre-roll theft skipped dice")
	}
}
func TestCatanClothEliminationEndsDepletedGameAndStopsProduction(t *testing.T) {
	s := clothTestGame(t)
	c := s.Catan.cloth()
	c.Held = []int{30, 2, 4}
	c.Villages = make([]CatanClothVillage, 5)
	s.catanScores()
	if err := s.EliminateCatan(0); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{2}) || c.Held[0] != 30 {
		t.Fatal("eliminated player won/depletion ignored/cloth destroyed")
	}
}
func TestCatanClothDepletionEndsFirstPairedAction(t *testing.T) {
	s := pairedGame(t, 5, false)
	g := s.Catan
	g.SetupStep = 15
	g.Seafarers = &CatanSeafarers{Pirate: -1, VictoryPoints: 14, Cloth: &CatanClothState{Held: []int{0, 0, 0, 0, 3}, EmptyLimit: 5, Villages: make([]CatanClothVillage, 5)}}
	s.Phase = "catan_turn"
	s.catanScores()
	first := s.Turn
	helperApply(t, s, first, Action{Type: "catan_end"})
	if !s.Finished || s.Turn != first || !reflect.DeepEqual(s.Winners, []int{4}) {
		t.Fatal("depletion incorrectly waited for paired player", s.Winners, s.Turn)
	}
}
func TestCatanClothTradeAndTheftViewsKeepHandsPrivate(t *testing.T) {
	s := clothTheftFixture(t)
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)
		cloth := v["seafarers"].(map[string]any)["cloth"].(map[string]any)
		if !reflect.DeepEqual(cloth["held"], []any{float64(1), float64(2), float64(0)}) {
			t.Fatal("missing public count")
		}
		for i, raw := range v["players"].([]any) {
			p := raw.(map[string]any)
			_, resources := p["resources"]
			_, dev := p["dev"]
			if resources != (i == viewer) || dev != (i == viewer) {
				t.Fatal("hidden hand exposed")
			}
		}
	}
}
func TestCatanClothRejectsCorruptProductionWithoutMutatingAction(t *testing.T) {
	s := clothTestGame(t)
	// Every non-seven outcome hits an invalid trader, making the randomized
	// action deterministic for the corruption check without controlling dice.
	s.Catan.cloth().Villages = nil
	for n := 2; n <= 12; n++ {
		if n != 7 {
			s.Catan.cloth().Villages = append(s.Catan.cloth().Villages, CatanClothVillage{Vertex: 2, Number: n, Stock: 1, Traders: []int{3}})
		}
	}
	for attempt := 0; attempt < 30; attempt++ {
		s.Phase = "catan_roll"
		before, _ := json.Marshal(s)
		err := s.Apply(0, Action{Type: "catan_roll"})
		if err == nil {
			if sum(s.Catan.Dice) != 7 {
				t.Fatal("corrupt trader accepted")
			}
			continue
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("rejected production mutated action")
		}
		return
	}
	t.Fatal("no non-seven roll generated")
}
