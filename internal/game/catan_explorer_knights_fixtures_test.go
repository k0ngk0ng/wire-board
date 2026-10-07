package game

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Opt-in regeneration runs the private constructor and a completable legal
// opening. Synthetic lair tokens are test components, never retail defaults.
// Normal runs validate the committed HTTP fixtures without rewriting them.
func TestCatanExplorerCityHTTPFixtures(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, phase := range []string{"roll", "action"} {
			name := fmt.Sprintf("../server/testdata/catan_explorer_city_%s_%d.json", phase, n)
			if os.Getenv("WIRE_BOARD_UPDATE_EXPLORER_CITY_FIXTURES") == "1" {
				s := explorerCityStateStarted(t, n, "explorers-and-pirates")
				if phase == "action" {
					explorerCityStateReady(t, s)
					for p := range s.Catan.Players {
						amount := 1
						if p == s.Turn {
							amount = 4
						}
						for card := 0; card < 8; card++ {
							explorerDevelopmentGrant(t, s, p, card, amount)
						}
					}
					ckProgressGive(t, s, s.Turn, 7, 10, 18, 20)
					ckProgressGive(t, s, (s.Turn+1)%n, 2, 13)
				}
				explorerCityStateRestore(t, s)
				raw, err := json.Marshal(s)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(name, append(raw, '\n'), 0644); err != nil {
					t.Fatal(err)
				}
			}
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			var s State
			if err = json.Unmarshal(data, &s); err != nil {
				t.Fatal(err)
			}
			explorerCityStateRestore(t, &s)
			g := s.Catan
			if len(g.Players) != n || g.CitiesKnights == nil || g.Explorer.Setup != nil || g.Explorer.Board.Scenario != "explorers-and-pirates" {
				t.Fatal("wrong combined HTTP fixture", name)
			}
			if phase == "roll" && (s.Phase != "catan_roll" || g.RollID != 0 || g.TurnSerial != 1) || phase == "action" && s.Phase != "catan_turn" {
				t.Fatal("wrong fixture phase", name, s.Phase)
			}
		}
	}
}
