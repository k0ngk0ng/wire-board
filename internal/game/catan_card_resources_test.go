package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanCardPlentifulChoicesBeforeProductionAndRestore(t *testing.T) {
	s := cardEarthquakeFixture(t)
	s.Catan.Tiles[0].Number = 2
	beginCardEvent(t, s, "plentiful_year", 2, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{1, 2, 0}) {
		t.Fatal("plentiful queue order")
	}
	for _, take := range [][]int{nil, {}, {0, 0, 0, 0, 0}, {2, 0, 0, 0, 0}, {-1, 2, 0, 0, 0}, {0, 0, 0, 0, 0, 1, 0, 0}} {
		helperReject(t, s, 1, Action{Type: "catan_event_resource", Take: take})
	}
	helperReject(t, s, 0, Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}})
	helperReject(t, s, 1, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
	helperReject(t, s, 1, Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}, Choice: "skip"})
	for index, actor := range []int{1, 2, 0} {
		for viewer := -1; viewer < 3; viewer++ {
			v := s.View(viewer)["catan"].(map[string]any)
			legal := v["legal"].(map[string][]int)
			if (len(legal["eventResources"]) == 5) != (viewer == actor) || len(legal["earthquakeRoads"])+len(legal["fleeDeserts"]) != 0 {
				t.Fatal("resource choices shown to wrong viewer")
			}
		}
		take := make([]int, 5)
		take[index] = 1
		helperApply(t, s, actor, Action{Type: "catan_event_resource", Take: take})
		saved := clone(*s)
		if !reflect.DeepEqual(*s, saved) {
			t.Fatal("resource response lost state on restore")
		}
		s = &saved
		if index < 2 && s.Catan.Players[0].Resources[0] != 0 {
			t.Fatal("production ran before all event rewards")
		}
	}
	for p, want := range [][]int{{1, 0, 1, 0, 0}, {2, 0, 0, 0, 0}, {1, 1, 0, 0, 0}} {
		if !slices.Equal(s.Catan.Players[p].Resources, want) {
			t.Fatal("event reward/production count", p, s.Catan.Players[p].Resources)
		}
	}
	if s.Catan.CardEvent != nil || s.Phase != "catan_turn" || s.Catan.RollID != 1 {
		t.Fatal("plentiful did not finish")
	}
	catanCheck(t, s)
}

func TestCatanCardPlentifulEmptyBankLastResourceAndElimination(t *testing.T) {
	for _, remaining := range []int{0, 1} {
		s := cardEarthquakeFixture(t)
		take := []int{19 - remaining, 19, 19, 19, 19}
		helperGrant(s, 2, take)
		beginCardEvent(t, s, "plentiful_year", 2, 0, 0)
		if remaining == 1 {
			if s.CatanPendingActor() != 1 {
				t.Fatal("first player should receive last resource")
			}
			v := s.View(1)["catan"].(map[string]any)["legal"].(map[string][]int)
			if !slices.Equal(v["eventResources"], []int{0}) {
				t.Fatal("empty resource offered")
			}
			helperReject(t, s, 1, Action{Type: "catan_event_resource", Take: []int{0, 1, 0, 0, 0}})
			s.AutoCatanPending()
		}
		if s.CatanPendingActor() != -1 || s.Catan.CardEvent != nil || s.Phase != "catan_turn" || sum(s.Catan.Bank) != 0 || sum(s.Catan.Players[1].Resources) != remaining {
			t.Fatal("exhausted bank stalled event or generated resources")
		}
		catanCheck(t, s)
	}
	s := cardEarthquakeFixture(t)
	s.Catan.Players[2].Eliminated = true
	beginCardEvent(t, s, "plentiful_year", 2, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{1, 0}) {
		t.Fatal("eliminated player included")
	}
	for range 2 {
		s.AutoCatanPending()
	}
	if sum(s.Catan.Players[2].Resources) != 0 || s.Catan.CardEvent != nil {
		t.Fatal("eliminated player awarded resource")
	}
}

func TestCatanCardPlentifulCityResourcesOnlyAndAqueduct(t *testing.T) {
	s := ckEvent(t)
	s.Catan.CitiesKnights.Players[0].Improvements[CatanScience] = 3
	beginCardEvent(t, s, "plentiful_year", 2, 6, CatanScience)
	helperReject(t, s, 0, Action{Type: "catan_event_resource", Take: []int{0, 0, 0, 0, 0, 1, 0, 0}})
	for range 3 {
		s.AutoCatanPending()
	}
	if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != 0 || sum(s.Catan.Players[0].Resources) != 1 {
		t.Fatal("event gift incorrectly counted as production for aqueduct")
	}
	saved := clone(*s)
	s = &saved
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 2 {
		t.Fatal("aqueduct did not follow event and zero production")
	}
	for _, p := range s.Catan.Players {
		if sum(p.Resources[5:]) != 0 {
			t.Fatal("event resource selected a commodity")
		}
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanCardPlentifulBotHiddenIndependence(t *testing.T) {
	s := cardEarthquakeFixture(t)
	beginCardEvent(t, s, "plentiful_year", 2, 0, 0)
	a, err := s.BotAction(1)
	if err != nil || a.Type != "catan_event_resource" || len(a.Take) != 5 || sum(a.Take) != 1 {
		t.Fatal("plentiful bot", a, err)
	}
	other := clone(*s)
	other.Catan.Players[0].Resources = []int{7, 8, 9, 10, 11}
	slices.Reverse(other.Catan.DevDeck)
	b, err := other.BotAction(1)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("resource bot used hidden state")
	}
}

func epidemicProductionFixture(t *testing.T, city bool) *State {
	t.Helper()
	s := catanGame(t, 3)
	if city {
		s = ckGame(t, 3)
	}
	g := s.Catan
	g.SetupStep, g.TurnSerial = 6, 1
	s.Turn, s.Phase = 0, "catan_roll"
	g.Robber = -1
	g.Vertices = []CatanVertex{{ID: 0, Owner: 0, Level: 2}, {ID: 1, Owner: 1, Level: 1}, {ID: 2, Owner: 2, Level: 2}}
	g.Edges, g.Ports, g.Tiles = nil, nil, nil
	for terrain := range 5 {
		g.Tiles = append(g.Tiles, CatanTile{ID: terrain, Resource: terrain, Number: 6, Vertices: []int{0, 1, 2}})
	}
	return s
}

func TestCatanCardEpidemicAllTerrainMetropolisAndNextProduction(t *testing.T) {
	for _, city := range []bool{false, true} {
		s := epidemicProductionFixture(t, city)
		red := 0
		if city {
			red = 6
			s.Catan.CitiesKnights.Metropolises[0] = 0
			s.Catan.CitiesKnights.Players[0].Improvements[0] = 4
		}
		beginCardEvent(t, s, "epidemic", 6, red, 0)
		if s.Phase != "catan_turn" || s.CatanPendingActor() != -1 {
			t.Fatal("epidemic unexpectedly blocked")
		}
		for _, p := range s.Catan.Players {
			if !slices.Equal(p.Resources[:5], []int{1, 1, 1, 1, 1}) || (city && sum(p.Resources[5:]) != 0) {
				t.Fatal("epidemic failed to reduce city production", city, p.Resources)
			}
		}
		if err := s.catanRollProduction(6); err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if g.Players[0].Resources[1] != 3 || g.Players[0].Resources[3] != 3 || g.Players[1].Resources[0] != 2 || g.Vertices[0].Level != 2 {
			t.Fatal("epidemic changed buildings or persisted to next production")
		}
		if city {
			if g.Players[0].Resources[0] != 2 || !slices.Equal(g.Players[0].Resources[5:], []int{1, 1, 1}) {
				t.Fatal("next city production still suppressed commodities")
			}
			ckSupply(t, g)
		} else {
			if g.Players[0].Resources[0] != 3 {
				t.Fatal("next city production still reduced")
			}
			catanCheck(t, s)
		}
	}
}

func TestCatanCardEpidemicShortageCalculatedAfterReduction(t *testing.T) {
	s := epidemicProductionFixture(t, false)
	// Three epidemic claims fit exactly; unreduced five-card demand would fail.
	helperGrant(s, 2, []int{16, 0, 0, 0, 0})
	beginCardEvent(t, s, "epidemic", 6, 0, 0)
	if s.Catan.Bank[0] != 0 || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[1].Resources[0] != 1 || s.Catan.Players[2].Resources[0] != 17 {
		t.Fatal("shortage used unreduced claims")
	}
	catanCheck(t, s)
}

func TestCatanCardEpidemicProgressDiscardRestore(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 1, 2})
	s.Phase = "catan_roll"
	g := s.Catan
	g.Tiles[0].Vertices = []int{0}
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 2
	g.CitiesKnights.Players[1].Improvements[0] = 1
	ckProgressGive(t, s, 1, 3, 4, 5, 6)
	ckProgressTop(t, s, 0)
	beginCardEvent(t, s, "epidemic", 6, 2, 0)
	if s.Phase != "catan_progress_discard" || !s.Catan.CitiesKnights.Event.Epidemic || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("epidemic flag lost before progress response")
	}
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 1, Action{Type: "catan_progress_discard", Cards: []int{3}})
	if s.Phase != "catan_turn" || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[0].Resources[5] != 0 || s.Catan.CitiesKnights.Event != nil {
		t.Fatal("restored epidemic did not reduce production")
	}
	s.Phase = "catan_roll"
	if err := s.catanCityRoll(3, 3, 3); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Players[0].Resources[0] != 2 || s.Catan.Players[0].Resources[5] != 1 {
		t.Fatal("next city roll retained epidemic")
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanCardEpidemicGoldChoiceOnlyOnce(t *testing.T) {
	s := cardEarthquakeFixture(t)
	g := s.Catan
	g.Seafarers = &CatanSeafarers{Pirate: -1}
	g.Tiles[0].Resource = CatanGold
	g.Vertices[0].Level = 2
	beginCardEvent(t, s, "epidemic", 6, 0, 0)
	for _, q := range s.Catan.GoldPending.Claims {
		if q.Count != 1 {
			t.Fatal("gold city did not receive reduced entitlement")
		}
	}
	saved := clone(*s)
	s = &saved
	for range 3 {
		s.AutoCatanPending()
	}
	if sum(s.Catan.Players[0].Resources) != 1 || s.Phase != "catan_turn" {
		t.Fatal("restored gold epidemic entitlement")
	}
	if err := s.catanRollProduction(6); err != nil {
		t.Fatal(err)
	}
	for _, q := range s.Catan.GoldPending.Claims {
		if q.Player == 0 && q.Count != 2 {
			t.Fatal("next gold city production still reduced")
		}
	}
	for range 3 {
		s.AutoCatanPending()
	}
	if sum(s.Catan.Players[0].Resources) != 3 {
		t.Fatal("next normal gold production")
	}
	catanCheck(t, s)
}
