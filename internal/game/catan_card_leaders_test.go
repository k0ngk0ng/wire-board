package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanCardCalmSeasCountsBuildingsNotHarborPoints(t *testing.T) {
	s := earthquakeGraph(t, []int{0, 0, 0, 0, 0, 0, 1, 1, 1, 1})
	s.Turn, s.Phase, s.Catan.Robber = 1, "catan_roll", -1
	g := s.Catan
	for i := range g.Vertices {
		g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
	}
	for _, id := range []int{0, 2, 4} {
		g.Vertices[id].Owner, g.Vertices[id].Level = 0, 1
	}
	for _, id := range []int{6, 8} {
		g.Vertices[id].Owner, g.Vertices[id].Level = 1, 2
	}
	g.Ports = []CatanPort{{Edge: 0}, {Edge: 2}, {Edge: 4}, {Edge: 6}, {Edge: 8}, {Edge: 0}, {Edge: -1}}
	s.enableCatanHarbors()
	if g.Harbors.Owner != 1 || !slices.Equal(g.harborPoints(), []int{3, 4, 0}) {
		t.Fatal("fixture needs different building and harbor-point leaders")
	}
	beginCardEvent(t, s, "calm_seas", 2, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{0}) {
		t.Fatal("city weighted as two buildings or duplicate port counted twice")
	}
	helperReject(t, s, 1, Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}})
	s.AutoCatanPending()
	if sum(s.Catan.Players[0].Resources) != 1 || sum(s.Catan.Players[1].Resources) != 0 || s.Catan.Harbors.Owner != 1 || s.Phase != "catan_turn" {
		t.Fatal("reward changed harbor award or rewarded wrong player")
	}
	catanCheck(t, s)
}

func TestCatanCardTournamentFaceupKnightsAndHiddenIndependence(t *testing.T) {
	s := cardEarthquakeFixture(t)
	g := s.Catan
	g.Players[0].Knights, g.Players[1].Knights, g.Players[2].Knights = 1, 2, 2
	for range 5 {
		catanCard(g, 0, 0) // Unplayed knights do not count.
	}
	g.ArmyOwner = 0 // The award itself does not decide tournament rewards.
	beginCardEvent(t, s, "tournament", 6, 0, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{1, 2}) {
		t.Fatal("tournament used army holder or hidden knights")
	}
	a, err := s.BotAction(1)
	if err != nil {
		t.Fatal(err)
	}
	other := clone(*s)
	other.Catan.Players[0].Dev[0] = 0
	other.Catan.Players[0].Resources = []int{4, 3, 2, 1, 0}
	slices.Reverse(other.Catan.DevDeck)
	b, err := other.BotAction(1)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("reward bot inspected hidden information")
	}
	helperApply(t, s, 1, Action{Type: "catan_event_resource", Take: []int{0, 1, 0, 0, 0}})
	if s.Catan.Players[0].Resources[0] != 0 || s.Catan.Players[2].Resources[0] != 0 {
		t.Fatal("production before all eligible rewards")
	}
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 2, Action{Type: "catan_event_resource", Take: []int{0, 0, 1, 0, 0}})
	for p, want := range [][]int{{1, 0, 0, 0, 0}, {1, 1, 0, 0, 0}, {1, 0, 1, 0, 0}} {
		if !slices.Equal(s.Catan.Players[p].Resources, want) {
			t.Fatal("wrong reward or production after restore", p, s.Catan.Players[p].Resources)
		}
	}
	catanCheck(t, s)
}

func TestCatanCardLeaderZeroTiesAndEliminatedSeats(t *testing.T) {
	for _, kind := range []string{"calm_seas", "tournament"} {
		for _, eliminated := range []bool{false, true} {
			s := cardEarthquakeFixture(t)
			if eliminated {
				s.Catan.Players[2].Eliminated = true
				s.Catan.Players[2].Knights = 5
				// An eliminated city's port does not beat the live zero tie.
				s.Catan.Ports = []CatanPort{{Edge: 5}}
			}
			beginCardEvent(t, s, kind, 2, 0, 0)
			want := []int{1, 2, 0}
			if eliminated {
				want = []int{1, 0}
			}
			if !slices.Equal(s.Catan.CardEvent.Players, want) {
				t.Fatal("zero tie or eliminated seat mishandled", kind, s.Catan.CardEvent.Players)
			}
			for range len(want) {
				s.AutoCatanPending()
			}
			for p := range 3 {
				count := 1
				if eliminated && p == 2 {
					count = 0
				}
				if sum(s.Catan.Players[p].Resources) != count {
					t.Fatal("zero-tie reward missing", kind, p)
				}
			}
			if s.Phase != "catan_turn" || s.Catan.CardEvent != nil {
				t.Fatal("zero-tie event stalled")
			}
		}
	}
}

func TestCatanCardLeaderBankExhaustionAndIllegalChoices(t *testing.T) {
	for _, kind := range []string{"calm_seas", "tournament"} {
		for _, remaining := range []int{0, 1} {
			s := cardEarthquakeFixture(t)
			helperGrant(s, 0, []int{19 - remaining, 19, 19, 19, 19})
			beginCardEvent(t, s, kind, 2, 0, 0)
			if remaining == 1 {
				for _, a := range []Action{
					{Type: "catan_event_resource", Take: []int{0, 1, 0, 0, 0}},
					{Type: "catan_event_resource", Take: []int{2, 0, 0, 0, 0}},
					{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}, Choice: "skip"},
					{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}},
				} {
					helperReject(t, s, 1, a)
				}
				for viewer := -1; viewer < 3; viewer++ {
					legal := s.View(viewer)["catan"].(map[string]any)["legal"].(map[string][]int)
					if (len(legal["eventResources"]) == 1) != (viewer == 1) {
						t.Fatal("reward choices visible to wrong viewer")
					}
				}
				s.AutoCatanPending()
			}
			if sum(s.Catan.Bank) != 0 || sum(s.Catan.Players[1].Resources) != remaining || sum(s.Catan.Players[2].Resources) != 0 || s.Catan.CardEvent != nil || s.Phase != "catan_turn" {
				t.Fatal("bank exhaustion duplicated reward or stalled event", kind)
			}
			catanCheck(t, s)
		}
	}
}

func TestCatanCardTournamentActiveStrengthBeforeBarbarians(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1, 1, 2, 2, 2})
	s.Phase = "catan_roll"
	g := s.Catan
	g.Tiles[0].Vertices = []int{0, 3, 6}
	for p, v := range []int{0, 3, 6} {
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
	}
	g.CitiesKnights.Knights = []CatanKnight{
		{Owner: 0, Vertex: 1, Strength: 2, Active: true},
		{Owner: 1, Vertex: 4, Strength: 1, Active: true},
		{Owner: 1, Vertex: 5, Strength: 1, Active: true},
		{Owner: 2, Vertex: 7, Strength: 3, Active: false},
	}
	g.CitiesKnights.BarbarianPosition = 6
	g.CitiesKnights.RobberStart = -1
	beginCardEvent(t, s, "tournament", 6, 6, 3)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{0, 1}) || s.Catan.CitiesKnights.BarbarianPosition != 6 {
		t.Fatal("inactive knights counted or dice resolved before tournament")
	}
	helperReject(t, s, 0, Action{Type: "catan_event_resource", Take: []int{0, 0, 0, 0, 0, 1, 0, 0}})
	helperApply(t, s, 0, Action{Type: "catan_event_resource", Take: []int{0, 1, 0, 0, 0}})
	saved := clone(*s)
	s = &saved
	helperApply(t, s, 1, Action{Type: "catan_event_resource", Take: []int{0, 0, 1, 0, 0}})
	if s.Catan.CardEvent != nil || s.Catan.CitiesKnights.Event == nil || !s.Catan.CitiesKnights.Event.Attack {
		t.Fatal("barbarians did not follow reward choices")
	}
	for step := 0; step < 8 && s.CatanPendingActor() >= 0; step++ {
		s.AutoCatanPending()
	}
	for p := range 3 {
		want := 2
		if p < 2 {
			want++
		}
		if sum(s.Catan.Players[p].Resources) != want || s.Catan.Players[p].Resources[0] != 1 || s.Catan.Players[p].Resources[5] != 1 {
			t.Fatal("reward or city production lost across barbarian responses", p, s.Catan.Players[p].Resources)
		}
	}
	for _, knight := range s.Catan.CitiesKnights.Knights {
		if knight.Active {
			t.Fatal("barbarians did not deactivate knights after tournament")
		}
	}
	if s.Phase != "catan_turn" || s.Catan.RollID != 1 {
		t.Fatal("tournament continuation failed")
	}
	ckSupply(t, s.Catan)
	ckProgressStock(t, s.Catan)
}

func TestCatanCardCalmSeasMetropolisAndAqueduct(t *testing.T) {
	s := ckEvent(t)
	g := s.Catan
	port := g.Ports[0]
	g.Ports = []CatanPort{port, port}
	vertex := g.Edges[port.Edge].A
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = 0, 2
	g.CitiesKnights.Metropolises[0] = vertex
	g.CitiesKnights.Players[0].Improvements[CatanScience] = 3
	beginCardEvent(t, s, "calm_seas", 2, 6, 0)
	if !slices.Equal(s.Catan.CardEvent.Players, []int{0}) {
		t.Fatal("metropolis port eligibility")
	}
	helperReject(t, s, 0, Action{Type: "catan_event_resource", Take: []int{0, 0, 0, 0, 0, 0, 1, 0}})
	s.AutoCatanPending()
	if s.Phase != "catan_aqueduct" || s.CatanPendingActor() != 0 || sum(s.Catan.Players[0].Resources) != 1 {
		t.Fatal("port reward incorrectly counted as production")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || sum(s.Catan.Players[0].Resources) != 2 {
		t.Fatal("aqueduct did not follow calm seas")
	}
	ckSupply(t, s.Catan)
}
