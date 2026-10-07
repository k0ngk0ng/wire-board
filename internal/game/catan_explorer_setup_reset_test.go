package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"testing"
)

// Replay the recorded, formally legal coast-blocking selections. Synthetic
// lair numbers remain explicit acceptance data, not a public game recipe.
func explorerCityBlockedOpening(t *testing.T, n int, scenario string) (*State, *State) {
	t.Helper()
	var numbers []int
	if catanExplorerMissionScenario(scenario) {
		numbers = []int{3, 4, 5, 9, 10, 11}
		if n > 4 {
			numbers = append(numbers, 3, 4)
		}
	}
	s, err := newCatanExplorerCityState(n, scenario, numbers)
	if err != nil {
		t.Fatal(err)
	}
	start := 0
	if n == 4 {
		start = 2
	}
	g, x := s.Catan, s.Catan.Explorer
	g.StartPlayer, x.Setup.Start, s.Turn = start, start, start
	if g.Paired != nil {
		g.Paired.Primary, g.Paired.Secondary = start, (start+3)%n
	}
	initial := clone(*s)
	rng := rand.New(rand.NewSource(int64(41*n + start)))
	for x.Setup.Step < 2*n-1 {
		choices, _ := s.catanExplorerSpecialChoices(s.Turn)
		if len(choices) == 0 {
			t.Fatal("unexpected earlier deadlock")
		}
		if err := s.Apply(s.Turn, choices[rng.Intn(len(choices))]); err != nil {
			t.Fatal(err)
		}
		x = s.Catan.Explorer
	}
	if !s.CatanExplorerSetupBlocked() {
		t.Fatal("expected recorded coast deadlock")
	}
	explorerCityStateRestore(t, s)
	return s, &initial
}

func TestCatanExplorerSetupReset(t *testing.T) {
	for _, n := range []int{4, 6} {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s, initial := explorerCityBlockedOpening(t, n, scenario)
				before := clone(*s)
				for p := -1; p < n; p++ {
					view := s.View(p)["catan"].(map[string]any)["explorer"].(map[string]any)
					if view["setupBlocked"] != true {
						t.Fatal("deadlock not visible to player/spectator")
					}
				}
				if err := s.Apply(s.Turn, Action{Type: "catan_explorer_reset"}); err == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("player action bypassed room authority")
				}
				if err := s.ResetCatanExplorerSetup(); err != nil {
					t.Fatal(err)
				}
				x := s.Catan.Explorer
				initial.Log = s.Log
				initial.Catan.Explorer.ActionID = before.Catan.Explorer.ActionID + 1
				initial.Catan.Explorer.Setup.PromptBase = 4*n + 1
				if !reflect.DeepEqual(*s, *initial) {
					t.Fatal("reset changed more than pieces, prompt, log and action ID")
				}
				explorerCityStateRestore(t, s)
				before = clone(*s)
				if err := s.ResetCatanExplorerSetup(); err == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("normal opening could be reset")
				}
				stale, _ := s.BotAction(s.Turn)
				stale.Prompt = 1
				if err := s.Apply(s.Turn, stale); err == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("old generation prompt accepted")
				}
				for x.Setup != nil {
					a, err := s.BotAction(s.Turn)
					if err != nil || a.Prompt != x.Setup.prompt() {
						t.Fatal("bad reset bot prompt", err)
					}
					if err := s.Apply(s.Turn, a); err != nil {
						t.Fatal(err)
					}
					explorerCityStateRestore(t, s)
					x = s.Catan.Explorer
				}
				before = clone(*s)
				if s.Phase != "catan_roll" || s.ResetCatanExplorerSetup() == nil || !reflect.DeepEqual(*s, before) {
					t.Fatal("completed opening could be reset")
				}
			})
		}
	}
}

func TestCatanExplorerSetupResetRepeatedAndInvalid(t *testing.T) {
	s, _ := explorerCityBlockedOpening(t, 4, "explorers-and-pirates")
	for _, offset := range []int{-1, int(^uint(0) >> 1)} {
		bad := clone(*s)
		bad.Catan.Explorer.Setup.PromptBase = offset
		before := clone(bad)
		if bad.ResetCatanExplorerSetup() == nil || !reflect.DeepEqual(bad, before) {
			t.Fatal("corrupt prompt generation was repaired or mutated")
		}
	}
	var old Action
	for generation := 1; generation <= 3; generation++ {
		if err := s.ResetCatanExplorerSetup(); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Explorer.Setup.prompt() != 17*generation+1 {
			t.Fatal("generation did not advance")
		}
		explorerCityStateRestore(t, s)
		before := clone(*s)
		if generation > 1 {
			if err := s.Apply(s.Turn, old); err == nil || !reflect.DeepEqual(*s, before) {
				t.Fatal("previous reset request remained valid")
			}
		}
		old, _ = s.BotAction(s.Turn)
		rng := rand.New(rand.NewSource(41*4 + 2))
		for s.Catan.Explorer.Setup.Step < 7 {
			choices, _ := s.catanExplorerSpecialChoices(s.Turn)
			if err := s.Apply(s.Turn, choices[rng.Intn(len(choices))]); err != nil {
				t.Fatal(err)
			}
		}
		if !s.CatanExplorerSetupBlocked() {
			t.Fatal("manual deadlock was prohibited")
		}
	}
}

func TestCatanExplorerSetupBlockedHTTPFixtures(t *testing.T) {
	for _, n := range []int{4, 6} {
		name := fmt.Sprintf("../server/testdata/catan_explorer_city_blocked_%d.json", n)
		if os.Getenv("WIRE_BOARD_UPDATE_EXPLORER_BLOCKED_FIXTURES") == "1" {
			s, _ := explorerCityBlockedOpening(t, n, "explorers-and-pirates")
			raw, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(name, append(raw, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var s State
		if err = json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		explorerCityStateRestore(t, &s)
		if !s.CatanExplorerSetupBlocked() || len(s.Catan.Players) != n || s.Catan.Explorer.Setup.PromptBase != 0 {
			t.Fatal("wrong blocked opening fixture")
		}
	}
}
