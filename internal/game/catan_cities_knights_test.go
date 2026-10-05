package game

import (
	"reflect"
	"strings"
	"testing"
)

func ckGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanCitiesKnights(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func ckSupply(t *testing.T, g *Catan) {
	t.Helper()
	for c, stock := range g.Bank {
		want := 19
		if len(g.Players) > 4 {
			want = 24
		}
		if c >= 5 {
			want = 12
			if len(g.Players) > 4 {
				want = 18
			}
		}
		if stock < 0 {
			t.Fatal("negative bank", c)
		}
		for _, p := range g.Players {
			if p.Resources[c] < 0 {
				t.Fatal("negative hand", c)
			}
			stock += p.Resources[c]
		}
		if stock != want {
			t.Fatal("card conservation", c, stock, want)
		}
	}
}
func ckEmptyTurn(t *testing.T) *State {
	s := ckGame(t, 3)
	s.Catan.SetupStep = 6
	s.Catan.TurnSerial = 1
	s.Turn = 0
	s.Phase = "catan_turn"
	return s
}
func TestCatanCitiesKnightsInitialCitiesAndInventory(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		s := ckGame(t, n)
		for step := 0; s.Catan.setup() && step < 4*n; step++ {
			g := s.Catan
			if g.SetupStep >= n && s.Phase == "catan_setup_city" {
				v := s.View(s.Turn)["catan"].(map[string]any)["legal"].(map[string][]int)
				if len(v["cities"]) == 0 {
					t.Fatal("missing initial city choices")
				}
			}
			a, e := s.BotAction(s.Turn)
			if e != nil {
				t.Fatal(e)
			}
			helperApply(t, s, s.Turn, a)
			ckSupply(t, s.Catan)
		}
		g := s.Catan
		if g.setup() || s.Phase != "catan_roll" || g.Robber != -1 || len(g.DevDeck) != 0 {
			t.Fatal("initial setup incomplete")
		}
		for i, p := range g.Players {
			r, v, c := g.pieces(i)
			if r != 2 || v != 1 || c != 1 || p.Score != 3 || sum(p.Resources[5:]) != 0 {
				t.Fatal("one settlement/one city or initial resource only", i, r, v, c, p)
			}
		}
		restored := clone(*s)
		if !reflect.DeepEqual(*s, restored) {
			t.Fatal("setup persistence")
		}
		helperApply(t, s, s.Turn, Action{Type: "catan_roll", Tokens: []int{99, 99}, Color: 99})
		if s.Catan.CitiesKnights.EventDie < 0 || s.Catan.CitiesKnights.EventDie > 5 || len(s.Catan.Dice) != 2 || s.Catan.Dice[0] < 1 || s.Catan.Dice[0] > 6 || s.Catan.Dice[1] < 1 || s.Catan.Dice[1] > 6 {
			t.Fatal("normal roll bypassed event die or trusted client dice")
		}
	}
	if _, e := NewCatanCitiesKnights(3, CatanOptions{Helpers: true}); e == nil {
		t.Fatal("unverified Helpers combination accepted")
	}
	if _, e := NewCatanCitiesKnights(5, CatanOptions{}); e == nil {
		t.Fatal("missing extension")
	}
}
func TestCatanCitiesKnightsProductionAndIndependentShortages(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.Vertices[1].Owner, g.Vertices[1].Level = 1, 1
	g.Tiles = nil
	for c := range 5 {
		g.Tiles = append(g.Tiles, CatanTile{ID: c, Resource: c, Number: 6, Vertices: []int{0, 1}})
	}
	if e := s.catanRollProduction(6); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(g.Players[0].Resources, []int{1, 2, 1, 2, 1, 1, 1, 1}) || !reflect.DeepEqual(g.Players[1].Resources, []int{1, 1, 1, 1, 1, 0, 0, 0}) {
		t.Fatal("city production", g.Players)
	}
	ckSupply(t, g)
	for _, empty := range []int{0, CatanPaper} {
		s = ckEmptyTurn(t)
		g = s.Catan
		g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
		g.Vertices[1].Owner, g.Vertices[1].Level = 1, 2
		g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0, 1}}}
		g.Players[2].Resources[empty] = g.Bank[empty] - 1
		g.Bank[empty] = 1
		if e := s.catanRollProduction(6); e != nil {
			t.Fatal(e)
		}
		for p := range 2 {
			if g.Players[p].Resources[empty] != 0 || g.Players[p].Resources[5-empty] != 1 {
				t.Fatal("resource and commodity shortage coupled", empty, g.Players[p].Resources)
			}
		}
		ckSupply(t, g)
	}
	s = ckEmptyTurn(t)
	g = s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.Vertices[1].Owner, g.Vertices[1].Level = 0, 2
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0, 1}}}
	g.Players[2].Resources[5] = 11
	g.Bank[5] = 1
	if e := s.catanRollProduction(6); e != nil {
		t.Fatal(e)
	}
	if g.Players[0].Resources[0] != 2 || g.Players[0].Resources[5] != 1 {
		t.Fatal("sole claimant shortage")
	}
	ckSupply(t, g)
	g.Robber = 0
	before := clone(g.Players)
	if e := s.catanRollProduction(6); e != nil || !reflect.DeepEqual(before, g.Players) {
		t.Fatal("robber did not block both card types")
	}
}
func TestCatanCitiesKnightsCommodityBankAndPlayerTrades(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Ports = nil
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 8, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_bank", Give: []int{0, 0, 0, 0, 0, 4, 0, 0}, Take: []int{0, 0, 0, 0, 0, 0, 1, 0}})
	g = s.Catan
	if g.Players[0].Resources[6] != 1 || !strings.Contains(strings.Join(s.Log, " "), "纸张×4") {
		t.Fatal("commodity trade or log")
	}
	helperReject(t, s, 0, Action{Type: "catan_bank", Give: []int{0, 0, 0, 0, 0}, Take: []int{1, 0, 0, 0, 0}})
	// Commodity trades benefit from general ports, not the wood specialty port.
	edge := g.Edges[0]
	g.Vertices[edge.A].Owner, g.Vertices[edge.A].Level = 0, 1
	g.Ports = []CatanPort{{Edge: 0, Resource: 0}}
	if rates := g.rates(0); rates[0] != 2 || rates[5] != 4 {
		t.Fatal("special port applies to paper", rates)
	}
	g.Ports = []CatanPort{{Edge: 0, Resource: -1}}
	if g.rates(0)[5] != 3 {
		t.Fatal("general port omitted commodity")
	}
	g.CitiesKnights.Players[0].Improvements[1] = 3
	if !reflect.DeepEqual(g.rates(0), []int{3, 3, 3, 3, 3, 2, 2, 2}) {
		t.Fatal("guild rates", g.rates(0))
	}
	helperApply(t, s, 0, Action{Type: "catan_bank", Give: []int{0, 0, 0, 0, 0, 2, 0, 0}, Take: []int{0, 1, 0, 0, 0, 0, 0, 0}})
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 0, 0, 1})
	helperApply(t, s, 0, Action{Type: "catan_trade_offer", Give: []int{0, 1, 0, 0, 0, 0, 1, 0}, Take: []int{0, 0, 0, 0, 0, 0, 0, 1}})
	offer := s.Catan.Trade.ID
	helperApply(t, s, 1, Action{Type: "catan_trade_accept", Offer: offer})
	helperApply(t, s, 0, Action{Type: "catan_trade_complete", Offer: offer, Target: 1})
	if s.Catan.Players[0].Resources[7] != 1 || s.Catan.Players[1].Resources[6] != 1 {
		t.Fatal("mixed player exchange")
	}
	ckSupply(t, s.Catan)
	base, e := NewCatan(3, CatanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	base.Catan.SetupStep = 6
	base.Phase = "catan_turn"
	helperReject(t, base, base.Turn, Action{Type: "catan_bank", Give: make([]int, 8), Take: []int{0, 0, 0, 0, 0, 1, 0, 0}})
}
func TestCatanCitiesKnightsWallsDiscardAndDormantRobber(t *testing.T) {
	for _, invaded := range []bool{false, true} {
		s := ckEmptyTurn(t)
		g := s.Catan
		g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
		helperGrant(s, 0, []int{0, 2, 0, 0, 0, 0, 0, 0})
		helperApply(t, s, 0, Action{Type: "catan_wall", Vertex: 0})
		helperReject(t, s, 0, Action{Type: "catan_wall", Vertex: 0})
		helperReject(t, s, 1, Action{Type: "catan_wall", Vertex: 0})
		helperGrant(s, 0, []int{0, 0, 0, 0, 0, 9, 0, 0})
		helperGrant(s, 1, []int{0, 0, 0, 0, 0, 0, 8, 0})
		g = s.Catan
		if invaded {
			g.CitiesKnights.Invasions = 1
			g.Robber = 0
		}
		if e := s.catanRollProduction(7); e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(g.DiscardDue, []int{0, 4, 0}) {
			t.Fatal("wall threshold or commodity count", g.DiscardDue)
		}
		helperReject(t, s, 1, Action{Type: "catan_discard", Tokens: []int{0, 0, 0, 0, 0}})
		s.AutoCatanPending()
		g = s.Catan
		want := "catan_turn"
		if invaded {
			want = "catan_robber"
		}
		if s.Phase != want || sum(g.Players[1].Resources) != 4 {
			t.Fatal("discard/robber continuation", s.Phase)
		}
		ckSupply(t, g)
		if !invaded {
			helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 1})
		}
	}
	s := ckEmptyTurn(t)
	g := s.Catan
	for i := range 4 {
		g.Vertices[i].Owner, g.Vertices[i].Level = 0, 2
	}
	helperGrant(s, 0, []int{0, 8, 0, 0, 0, 0, 0, 0})
	for i := range 3 {
		helperApply(t, s, 0, Action{Type: "catan_wall", Vertex: i})
	}
	helperReject(t, s, 0, Action{Type: "catan_wall", Vertex: 3})
	if s.Catan.catanDiscardLimit(0) != 13 {
		t.Fatal("three wall threshold")
	}
}
func TestCatanCitiesKnightsAqueductQueuePrivateHandsAndRestore(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	for i := range g.CitiesKnights.Players {
		g.CitiesKnights.Players[i].Improvements[0] = 3
	}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0}}}
	g.Players[2].Resources[0] = g.Bank[0]
	g.Bank[0] = 0
	if e := s.catanRollProduction(6); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_aqueduct" || !reflect.DeepEqual(g.CitiesKnights.Pending.Players, []int{1, 2}) {
		t.Fatal("commodity recipient incorrectly compensated", g.CitiesKnights.Pending)
	}
	helperReject(t, s, 0, Action{Type: "catan_aqueduct", Color: 1})
	helperReject(t, s, 1, Action{Type: "catan_aqueduct", Color: 5})
	helperReject(t, s, 1, Action{Type: "catan_aqueduct", Color: 0})
	saved := clone(*s)
	if !reflect.DeepEqual(*s, saved) {
		t.Fatal("aqueduct persistence")
	}
	s = &saved
	for _, viewer := range []int{-1, 0, 1} {
		v := s.View(viewer)["catan"].(map[string]any)
		for i, raw := range v["players"].([]any) {
			_, hand := raw.(map[string]any)["resources"]
			if hand != (viewer == i) {
				t.Fatal("commodity hand leaked")
			}
		}
	}
	helperApply(t, s, 1, Action{Type: "catan_aqueduct", Color: 4})
	if s.CatanPendingActor() != 2 {
		t.Fatal("aqueduct next player")
	}
	s.AutoCatanPending()
	if s.CatanPendingActor() != -1 || s.Phase != "catan_turn" {
		t.Fatal("aqueduct timeout did not finish")
	}
	ckSupply(t, s.Catan)
	// Empty ordinary bank cannot pay with commodities or stall the turn.
	s = ckEmptyTurn(t)
	g = s.Catan
	for c := range 5 {
		g.Players[2].Resources[c] = g.Bank[c]
		g.Bank[c] = 0
	}
	g.CitiesKnights.Players[1].Improvements[0] = 3
	s.catanAfterProduction([]int{0, 0, 0})
	if s.Phase != "catan_turn" || g.CitiesKnights.Pending != nil {
		t.Fatal("empty bank blocked aqueduct")
	}
}
func TestCatanCitiesKnightsImprovementsMetropolisAndVictory(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.Vertices[1].Owner, g.Vertices[1].Level = 1, 2
	for level := 1; level <= 4; level++ {
		gift := make([]int, 8)
		gift[5] = level
		helperGrant(s, 0, gift)
		helperApply(t, s, 0, Action{Type: "catan_improvement", Color: 0})
	}
	if s.Phase != "catan_metropolis" || s.Catan.CitiesKnights.Players[0].Improvements[0] != 4 {
		t.Fatal("missing metropolis choice")
	}
	helperReject(t, s, 0, Action{Type: "catan_metropolis", Vertex: 1})
	helperReject(t, s, 1, Action{Type: "catan_metropolis", Vertex: 1})
	saved := clone(*s)
	s = &saved
	s.AutoCatanPending()
	if s.Catan.cityMetropolisOwner(0) != 0 || s.Catan.Players[0].Score != 4 {
		t.Fatal("metropolis score")
	}
	s.Turn = 1
	for level := 1; level <= 4; level++ {
		gift := make([]int, 8)
		gift[5] = level
		helperGrant(s, 1, gift)
		helperApply(t, s, 1, Action{Type: "catan_improvement", Color: 0})
	}
	if s.Phase != "catan_turn" || s.Catan.cityMetropolisOwner(0) != 0 {
		t.Fatal("tied fourth level stole metropolis")
	}
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 5, 0, 0})
	helperApply(t, s, 1, Action{Type: "catan_improvement", Color: 0})
	helperApply(t, s, 1, Action{Type: "catan_metropolis", Vertex: 1})
	if s.Catan.cityMetropolisOwner(0) != 1 || s.Catan.Players[0].Score != 2 || s.Catan.Players[1].Score != 4 {
		t.Fatal("fifth level metropolis transfer")
	}
	s.Turn = 0
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 5, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_improvement", Color: 0})
	if s.Phase != "catan_turn" || s.Catan.cityMetropolisOwner(0) != 1 {
		t.Fatal("permanent metropolis was stolen")
	}
	helperReject(t, s, 0, Action{Type: "catan_improvement", Color: 0})
	ckSupply(t, s.Catan)
	g = s.Catan
	g.CitiesKnights.Players[0].ProgressPoints = 10
	s.catanScores()
	s.catanVictory()
	if s.Finished {
		t.Fatal("base ten-point victory leaked")
	}
	g.CitiesKnights.Players[0].DefenderPoints = 1
	s.catanScores()
	s.catanVictory()
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) {
		t.Fatal("thirteen-point victory")
	}
}
func TestCatanCitiesKnightsImprovementsNeedAvailableCityAndExactGoods(t *testing.T) {
	s := ckEmptyTurn(t)
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 1, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_improvement", Color: 0})
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	helperReject(t, s, 0, Action{Type: "catan_improvement", Color: 1})
	helperApply(t, s, 0, Action{Type: "catan_improvement", Color: 0})
	g = s.Catan
	g.CitiesKnights.Players[0].Improvements[1] = 3
	g.CitiesKnights.Metropolises[0] = 0
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 0, 4, 0})
	helperReject(t, s, 0, Action{Type: "catan_improvement", Color: 1})
	g = s.Catan
	g.Vertices[0].Level = 1
	helperReject(t, s, 0, Action{Type: "catan_improvement", Color: 1})
	if g.CitiesKnights.Players[0].Improvements[0] != 1 {
		t.Fatal("lost city erased improvements")
	}
}

func TestCatanCitiesKnightsAqueductBankDepletesBetweenClaimants(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	for c := range 5 {
		g.Players[2].Resources[c], g.Bank[c] = g.Bank[c], 0
	}
	g.Players[2].Resources[3]--
	g.Bank[3] = 1
	for p := range 2 {
		g.CitiesKnights.Players[p].Improvements[0] = 3
	}
	s.catanAfterProduction([]int{0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_end"})
	helperReject(t, s, 0, Action{Type: "catan_bank", Give: make([]int, 8), Take: make([]int, 8)})
	helperApply(t, s, 0, Action{Type: "catan_aqueduct", Color: 3})
	if s.Phase != "catan_turn" || s.CatanPendingActor() != -1 || sum(s.Catan.Players[1].Resources) != 0 {
		t.Fatal("empty ordinary bank left a stalled claimant or paid commodities")
	}
	ckSupply(t, s.Catan)
}

func TestCatanCitiesKnightsCommodityTheftBuildAndPairedTrade(t *testing.T) {
	s := ckEmptyTurn(t)
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 0, 0, 1})
	s.Catan.CitiesKnights.Invasions = 1
	s.Catan.Victims = []int{1}
	s.Catan.ResumePhase = "catan_turn"
	s.Phase = "catan_steal"
	helperApply(t, s, 0, Action{Type: "catan_steal", Target: 1})
	if s.Catan.Players[0].Resources[7] != 1 || s.Catan.Players[1].Resources[7] != 0 {
		t.Fatal("commodity theft")
	}
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	helperGrant(s, 0, []int{0, 0, 0, 2, 3, 0, 0, 0})
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: 0})
	if s.Catan.Vertices[0].Level != 2 || !reflect.DeepEqual(s.Catan.Players[0].Resources, []int{0, 0, 0, 0, 0, 0, 0, 1}) {
		t.Fatal("ordinary construction consumed commodity")
	}
	ckSupply(t, s.Catan)
	s = ckGame(t, 5)
	s.Catan.SetupStep = 10
	s.Turn, s.Phase = 0, "catan_turn"
	s.Catan.Paired.Primary, s.Catan.Paired.Secondary, s.Catan.Paired.Second = 2, 0, true
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 4, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_trade_offer", Give: []int{0, 0, 0, 0, 0, 1, 0, 0}, Take: []int{1, 0, 0, 0, 0, 0, 0, 0}})
	helperApply(t, s, 0, Action{Type: "catan_bank", Give: []int{0, 0, 0, 0, 0, 4, 0, 0}, Take: []int{1, 0, 0, 0, 0, 0, 0, 0}})
	ckSupply(t, s.Catan)
}

func TestCatanCitiesKnightsEconomicBotsUseOwnHand(t *testing.T) {
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.CitiesKnights.Players[0].Improvements[0] = 2
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 3, 0, 0})
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_improvement" || a.Color != 0 {
		t.Fatal("bot failed to build Aqueduct", a, err)
	}
	other := clone(*s)
	other.Catan.Players[1].Resources = []int{8, 7, 6, 5, 4, 3, 2, 1}
	b, err := other.BotAction(0)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("economic bot used private opponent composition")
	}
	helperApply(t, s, 0, a)
	// A bot with no ordinary resources can exchange excess commodities towards
	// a useful wall. Avoid looping on exchange targets it cannot eventually buy.
	s = ckEmptyTurn(t)
	g = s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.CitiesKnights.Players[0].Improvements = [3]int{5, 5, 5}
	// Other players have occupied all routes, leaving this city a wall as its
	// only construction target. Otherwise expanding the road is also sensible.
	for i := range g.Edges {
		g.Edges[i].Owner = 1
	}
	helperGrant(s, 0, []int{0, 0, 0, 0, 0, 8, 0, 0})
	built := false
	for step := 0; step < 6; step++ {
		a, err = s.BotAction(0)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, 0, a)
		if a.Type == "catan_wall" {
			built = true
			break
		}
		if a.Type == "catan_end" {
			break
		}
	}
	if !built {
		t.Fatal("commodity-funded wall bot stalled")
	}
	ckSupply(t, s.Catan)
}
