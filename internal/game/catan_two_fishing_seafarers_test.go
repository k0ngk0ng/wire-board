package game

import (
	"fmt"
	"testing"
)

func twoFishSeaGame(t *testing.T, scenario, layout string, helpers, events bool) *State {
	t.Helper()
	var world *CatanNewWorldMap
	if scenario == "new_world" {
		var err error
		world, err = GenerateCatanFishingNewWorldMap(4)
		if err != nil {
			t.Fatal(err)
		}
		layout = "prepared"
	}
	s, err := NewCatanTwoFishingSeafarers(2, CatanOptions{Helpers: helpers, AllHelpers: helpers}, CatanSeafarersSetup{Scenario: scenario, Layout: layout}, world)
	if err != nil {
		t.Fatal(err)
	}
	if events {
		if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
			t.Fatal(err)
		}
	}
	return s
}
func TestCatanTwoFishingSeafarersRecipes(t *testing.T) {
	for _, scenario := range twoSeaScenarios {
		layouts := []string{"fixed", "variable"}
		if scenario == "desert" || scenario == "tribe" {
			layouts = []string{"fixed"}
		}
		if scenario == "new_world" {
			layouts = []string{"prepared"}
		}
		for _, layout := range layouts {
			for _, helpers := range []bool{false, true} {
				for _, events := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/helpers%t/events%t", scenario, layout, helpers, events), func(t *testing.T) {
						s := twoFishSeaGame(t, scenario, layout, helpers, events)
						if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
							t.Fatal(err)
						}
						if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
							t.Fatal(err)
						}
						for p := range 2 {
							hand := s.Catan.Fishing.Tokens.Hands[p]
							value := 0
							for _, id := range hand {
								value += catanFishValue(id)
							}
							if len(hand) != 5 || value != 9 || s.Catan.Two.Tokens[p] != 0 {
								t.Fatal("wrong starting economy")
							}
						}
						for step := 0; step < 450 && s.Catan.TurnSerial < 4; step++ {
							twoSeaStep(t, s)
							twoSeaRestore(t, s)
							if err := s.Catan.validateFishing(); err != nil {
								t.Fatal(s.Phase, err)
							}
						}
						if s.Catan.setup() || s.Catan.TurnSerial < 4 || s.Catan.Two.Bank != 0 || s.Catan.Two.TokensIssued != 0 {
							t.Fatal("opening/economy stalled")
						}
					})
				}
			}
		}
	}
}
func TestCatanTwoFishingSeafarersNaturalGames(t *testing.T) {
	for i, scenario := range twoSeaScenarios {
		t.Run(scenario, func(t *testing.T) {
			s := twoFishSeaGame(t, scenario, "fixed", i%2 == 0, i%2 == 1)
			seen := map[string]int{}
			for step := 0; step < 7500 && !s.Finished; step++ {
				a := twoSeaStep(t, s)
				seen[a.Type]++
				if step%41 == 0 {
					twoSeaRestore(t, s)
					if err := s.Catan.validateFishing(); err != nil {
						t.Fatal(err)
					}
				}
			}
			if !s.Finished {
				t.Fatal("natural game stalled", s.Round, s.Phase, seen)
			}
			if seen["catan_two_build"] == 0 {
				t.Fatal("missing neutral building")
			}
			t.Log(s.Round, seen)
		})
	}
}

// Exercise the same fish-funded route before production, between the two
// productions, and after both. The response must consume neither a roll nor
// another fish payment, including after saving at each response boundary.
func TestCatanTwoFishingSeafarersShipContinuation(t *testing.T) {
	for rolls := 0; rolls <= 2; rolls++ {
		t.Run(fmt.Sprint(rolls), func(t *testing.T) {
			s, edge := prepareTwoSeaFogFixture(t, twoFishSeaGame(t, "fog", "fixed", false, false), false, 0)
			s.Catan.Two.Rolls = []int{4, 5}[:rolls]
			phase := "catan_roll"
			if rolls == 2 {
				phase = "catan_turn"
			}
			s.Phase = phase
			q := s.Catan.Two
			seq := q.Sequence
			payment := s.Catan.fishPayment(0, s.Catan.fishActionCost(0, "catan_fish_ship"))
			helperApply(t, s, 0, Action{Type: "catan_fish_ship", Edge: edge, Tokens: payment})
			if s.Phase != "catan_gold" || s.Catan.Two.AfterRoute != "ship" || s.Catan.Two.Pending != nil {
				t.Fatal("gold did not precede neutral ship", s.Phase)
			}
			twoSeaRestore(t, s)
			helperReject(t, s, 1, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
			helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{1, 0, 0, 0, 0}})
			twoSeaRestore(t, s)
			if s.Phase != "catan_two_build" || s.Catan.Two.Pending.Resume != phase || s.Catan.Two.Sequence != seq+1 {
				t.Fatal("missing neutral continuation", s.Phase, s.Catan.Two)
			}
			a := twoSeaStep(t, s)
			twoSeaRestore(t, s)
			if s.Phase != phase || s.Catan.Two.Pending != nil || s.Catan.Two.AfterRoute != "" || s.Catan.Two.Sequence != seq+1 || len(s.Catan.Two.Rolls) != rolls {
				t.Fatal("duplicated/lost response or production", s.Phase, s.Catan.Two)
			}
			if !s.Catan.Edges[a.Edge].Ship || len(s.Catan.Fishing.Tokens.Discard) != len(payment) || s.Catan.Two.Bank != 0 {
				t.Fatal("wrong neutral ship/payment")
			}
			if err := s.Catan.validateFishing(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatanTwoFishingSeafarersRuleIsolation(t *testing.T) {
	s := twoFishSeaGame(t, "islands", "fixed", false, false)
	for _, change := range []func(*State){
		func(s *State) { s.Catan.Fishing.TwoSea = "" },
		func(s *State) { s.Catan.Fishing.TwoSea = "unknown" },
		func(s *State) { s.Catan.Two.Seafarers = "" },
		func(s *State) { s.Catan.Fishing.Two = "" },
		func(s *State) { s.Catan.Two = nil },
	} {
		next := clone(*s)
		change(&next)
		if err := next.Catan.validateFishing(); err == nil {
			t.Fatal("accepted incompatible saved recipe")
		}
	}
	multi, err := NewCatanFishingSeafarers(4, CatanOptions{}, CatanSeafarersSetup{Scenario: "islands", Layout: "fixed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	multi.Catan.Fishing.TwoSea = CatanTwoFishingSeafarersRules
	if err := multi.Catan.validateFishing(); err == nil {
		t.Fatal("two-player marker accepted on multiplayer map")
	}
}
