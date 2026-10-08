package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

var explorerTwoCityScenarios = []string{"land-ho", "pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"}

func explorerTwoCityGame(t *testing.T, scenario string, fish int, helpers, events bool) *State {
	t.Helper()
	var s *State
	var err error
	if fish > 0 {
		s, err = NewCatanExplorerFishing(2, scenario, true, fish == 2)
	} else {
		s, err = NewCatanExplorerCitiesKnights(2, scenario)
	}
	if err != nil {
		t.Fatal(err)
	}
	if helpers {
		if err = s.EnableCatanExplorerHelpers(true); err != nil {
			t.Fatal(err)
		}
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func explorerTwoCityStep(t *testing.T, s *State) Action {
	t.Helper()
	if s.CatanExplorerSetupBlocked() {
		if err := s.ResetCatanExplorerSetup(); err != nil {
			t.Fatal(err)
		}
	}
	p := explorerIntroActor(s)
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(s.Phase, err)
	}
	if err = s.Apply(p, a); err != nil {
		t.Fatal(s.Phase, a, err)
	}
	return a
}
func explorerTwoCitySetup(t *testing.T, s *State) {
	t.Helper()
	for step := 0; step < 160 && s.Catan.Explorer.Setup != nil; step++ {
		explorerTwoCityStep(t, s)
	}
	if s.Catan.Explorer.Setup != nil {
		t.Fatal("opening stalled")
	}
}
func TestCatanExplorerTwoKnightsRecipes(t *testing.T) {
	for _, scenario := range explorerTwoCityScenarios {
		for fish := range 3 {
			for _, helpers := range []bool{false, true} {
				for _, events := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/fish%d/helpers%t/events%t", scenario, fish, helpers, events), func(t *testing.T) {
						s := explorerTwoCityGame(t, scenario, fish, helpers, events)
						explorerTwoCitySetup(t, s)
						g := s.Catan
						if g.Two != nil || g.Paired != nil || g.Explorer.Board.TwoKnights != CatanExplorerTwoKnightsRules {
							t.Fatal("wrong two-player controller")
						}
						for _, owner := range []int{-2, -3} {
							village, harbor := 0, 0
							for _, v := range g.Vertices {
								if v.Owner == owner {
									if g.cityAt(v.ID) {
										t.Fatal("neutral city")
									}
									if v.Harbor {
										harbor++
									} else {
										village++
									}
								}
							}
							if village != 1 || harbor != 1 {
								t.Fatal("neutral opening", owner, village, harbor)
							}
						}
						for p, player := range g.Players {
							if player.Score != 4 || len(player.Resources) != 8 {
								t.Fatal("actual player city opening", p)
							}
						}
						for step := 0; step < 350 && s.Catan.TurnSerial < 4; step++ {
							explorerTwoCityStep(t, s)
							explorerFishingRestore(t, s)
						}
						if s.Catan.TurnSerial < 4 {
							t.Fatal("no turn progression")
						}
						if s.Catan.RollID > int(s.Catan.TurnSerial) {
							t.Fatal("double production leaked")
						}
					})
				}
			}
		}
	}
}
func TestCatanExplorerTwoKnightsNeutralAndBarbarian(t *testing.T) {
	s := explorerTwoCityGame(t, "pirate-lairs", 0, false, false)
	explorerTwoCitySetup(t, s)
	g := s.Catan
	// Two active weak knights defend the two real cities exactly. Static
	// neutral harbor settlements must not inflate the barbarian strength.
	for p := range 2 {
		v := -1
		for _, edge := range g.Edges {
			if edge.Owner == p {
				for _, end := range []int{edge.A, edge.B} {
					if g.Vertices[end].Owner == -1 && g.explorerKnightSite(end) {
						v = end
						break
					}
				}
			}
			if v >= 0 {
				break
			}
		}
		if v < 0 {
			t.Fatal("missing knight fixture")
		}
		g.CitiesKnights.Knights = append(g.CitiesKnights.Knights, CatanKnight{Owner: p, Vertex: v, Strength: 1, Active: true})
	}
	before := clone(g.Vertices)
	g.CitiesKnights.BarbarianPosition = 6
	if err := s.catanExplorerCityRoll(2, 2, 3); err != nil {
		t.Fatal(err)
	}
	if s.Catan.CitiesKnights.Pending == nil || s.Catan.CitiesKnights.Pending.Kind != "defender_reward" {
		t.Fatal("neutral harbor counted as city", s.Phase)
	}
	for step := 0; step < 12 && s.Catan.CitiesKnights.Pending != nil; step++ {
		explorerTwoCityStep(t, s)
	}
	for _, v := range s.Catan.Vertices {
		if v.Owner < -1 && v != before[v.ID] {
			t.Fatal("neutral modified by battle")
		}
	}
	if s.Catan.RollID != 1 || s.Catan.CitiesKnights.Invasions != 1 {
		t.Fatal("city event repeated")
	}
	// Neutral objects remain blockers and are never valid hand/knight targets.
	for _, v := range s.Catan.Vertices {
		if v.Owner < -1 {
			if s.Catan.canSettlement(s.Turn, v.ID, false) || slices.Contains(s.Catan.cityMetropolisSites(s.Turn), v.ID) {
				t.Fatal("neutral build target")
			}
		}
	}
}
func TestCatanExplorerTwoKnightsMarkerAndIsolation(t *testing.T) {
	s := explorerTwoCityGame(t, "land-ho", 0, false, false)
	for _, marker := range []string{"", "unknown"} {
		bad := clone(*s)
		bad.Catan.Explorer.Board.TwoKnights = marker
		before, _ := json.Marshal(bad)
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		if bad.Apply(bad.Turn, a) == nil {
			t.Fatal("unmarked two-city accepted")
		}
		after, _ := json.Marshal(bad)
		if string(before) != string(after) {
			t.Fatal("corrupt action not atomic")
		}
	}
	ordinary, err := NewCatanExplorerLandHo(2)
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.Catan.Explorer.Board.TwoKnights != "" || ordinary.Catan.Explorer.Board.Layout != "fixed" || ordinary.Catan.CitiesKnights != nil || ordinary.Catan.Explorer.Board.Target != 8 {
		t.Fatal("base opening changed")
	}
	three, err := NewCatanExplorerCitiesKnights(3, "land-ho")
	if err != nil {
		t.Fatal(err)
	}
	if three.Catan.Explorer.Board.TwoKnights != "" {
		t.Fatal("three-player marker leaked")
	}
	if _, err = NormalizeCatanCitiesKnightsSetup(2, CatanCitiesKnightsSetup{}); err == nil {
		t.Fatal("ordinary constructor bypass")
	}
}

func TestCatanExplorerTwoKnightsPillageAndNeutralInventory(t *testing.T) {
	s := explorerTwoCityGame(t, "pirate-lairs", 0, false, false)
	explorerTwoCitySetup(t, s)
	initial := clone(*s.Catan)
	// A neutral city is impossible even when restoring a syntactically valid save.
	bad := clone(*s)
	for i, v := range bad.Catan.Vertices {
		if v.Owner < -1 && !v.Harbor {
			bad.Catan.Vertices[i].Level = 2
			break
		}
	}
	if bad.validateCatanExplorerCities() == nil {
		t.Fatal("neutral city accepted")
	}
	s.Catan.CitiesKnights.BarbarianPosition = 6
	if err := s.catanExplorerCityRoll(3, 3, 3); err != nil {
		t.Fatal(err)
	}
	replies := 0
	for s.Phase == "catan_pillage" && replies < 3 {
		actor := s.CatanPendingActor()
		if actor < 0 || actor >= 2 {
			t.Fatal("neutral responder", actor)
		}
		for _, site := range s.Catan.pillageSites(actor) {
			if s.Catan.Vertices[site].Owner != actor || s.Catan.Vertices[site].Harbor {
				t.Fatal("invalid pillage target")
			}
		}
		explorerTwoCityStep(t, s)
		explorerFishingRestore(t, s)
		replies++
	}
	if replies != 2 || s.Catan.RollID != 1 {
		t.Fatal("wrong attack/production", replies, s.Catan.RollID)
	}
	claims := explorerCityClaims(s.Catan, 6)
	for p, player := range s.Catan.Players {
		if player.Score != 3 {
			t.Fatal("harbor lost points", p)
		}
		for r, gain := range claims[p] {
			if player.Resources[r] != initial.Players[p].Resources[r]+gain {
				t.Fatal("neutral/pre-pillage production", p, r)
			}
		}
	}
	for _, v := range s.Catan.Vertices {
		if v.Owner < -1 && v != initial.Vertices[v.ID] {
			t.Fatal("neutral pillaged")
		}
	}
}
