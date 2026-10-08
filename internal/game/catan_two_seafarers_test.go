package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

var twoSeaScenarios = []string{"shores", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world"}

func twoSeaGame(t *testing.T, scenario, layout string, helpers, events bool) *State {
	t.Helper()
	var world *CatanNewWorldMap
	if scenario == "new_world" {
		var err error
		world, err = GenerateCatanNewWorldMap(4)
		if err != nil {
			t.Fatal(err)
		}
		layout = "prepared"
	}
	s, err := NewCatanTwoSeafarers(2, CatanOptions{Helpers: helpers, AllHelpers: helpers}, CatanSeafarersSetup{Scenario: scenario, Layout: layout}, world)
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
func twoSeaStep(t *testing.T, s *State) Action {
	t.Helper()
	actor := twoFullActor(s)
	a, err := s.BotAction(actor)
	if err != nil {
		t.Fatal(s.Phase, err)
	}
	if err = s.Apply(actor, a); err != nil {
		t.Fatalf("phase=%s a=%+v two=%+v: %v", s.Phase, a, s.Catan.Two, err)
	}
	return a
}
func twoSeaRestore(t *testing.T, s *State) {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	if err = restored.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
	if err = restored.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	*s = restored
}
func TestCatanTwoSeafarersRecipes(t *testing.T) {
	for _, scenario := range twoSeaScenarios {
		layouts := []string{"fixed", "variable"}
		if scenario == "new_world" {
			layouts = []string{"prepared"}
		}
		for _, layout := range layouts {
			for _, helpers := range []bool{false, true} {
				for _, events := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/helpers%t/events%t", scenario, layout, helpers, events), func(t *testing.T) {
						s := twoSeaGame(t, scenario, layout, helpers, events)
						if err := s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
							t.Fatal(err)
						}
						if err := s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
							t.Fatal(err)
						}
						for step := 0; step < 450 && s.Catan.TurnSerial < 4; step++ {
							twoSeaStep(t, s)
							twoSeaRestore(t, s)
						}
						if s.Catan.setup() || s.Catan.TurnSerial < 4 {
							t.Fatal("opening stalled")
						}
						if s.Catan.RollID > int(s.Catan.TurnSerial)*2 || s.Catan.Paired != nil || len(s.Catan.Players) != 2 {
							t.Fatal("wrong production controller")
						}
						for _, owner := range catanTwoNeutralOwners {
							_, villages, cities := s.Catan.pieces(owner)
							if villages < 1 || cities != 0 {
								t.Fatal("neutral stock")
							}
						}
					})
				}
			}
		}
	}
}
func TestCatanTwoSeafarersNaturalGames(t *testing.T) {
	for i, scenario := range twoSeaScenarios {
		t.Run(scenario, func(t *testing.T) {
			s := twoSeaGame(t, scenario, "fixed", i%2 == 0, i%2 == 1)
			seen := map[string]int{}
			for step := 0; step < 7000 && !s.Finished; step++ {
				a := twoSeaStep(t, s)
				seen[a.Type]++
				if step%47 == 0 {
					twoSeaRestore(t, s)
				}
			}
			if !s.Finished {
				t.Fatal("natural game stalled", s.Round, s.Phase, seen)
			}
			if seen["catan_two_build"] == 0 {
				t.Fatal("no neutral/sea building", seen)
			}
			twoSeaRestore(t, s)
			t.Log(s.Round, "rounds", seen)
		})
	}
}
