package game

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// These are controlled boundary states: discovered farms, claimed sacks and
// resources were arranged explicitly. They are not natural-game evidence.
func TestCatanExplorerSpiceNetworkActionFixtures(t *testing.T) {
	raw, err := os.ReadFile("../server/testdata/catan_explorer_spice_actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var states map[string]*State
	if err = json.Unmarshal(raw, &states); err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatal("unexpected boundary fixtures")
	}
	for _, name := range []string{"gold", "transfer"} {
		t.Run(name, func(t *testing.T) {
			s := states[name]
			if s == nil {
				t.Fatal("missing fixture")
			}
			if err := s.validateCatanExplorer(); err != nil {
				t.Fatal(err)
			}
			p, x := s.Turn, s.Catan.Explorer
			if name == "gold" {
				if s.Phase != "catan_turn" || x.Cargo.farmCount(x.Board, p, "gold") != 2 || s.Catan.Players[p].Resources[0] != 3 {
					t.Fatal("gold boundary changed")
				}
				for i := 0; i < 2; i++ {
					explorerSpiceApply(t, s, Action{Type: "catan_explorer_spice_gold", Card: 0})
				}
				explorerFishReject(t, s, p, Action{Type: "catan_explorer_spice_gold", Card: 0, Prompt: int(s.Catan.TurnSerial)})
				return
			}
			sacks := []int{}
			for id, sack := range x.Cargo.Spice {
				if sack.Owner == p && sack.At.Kind == "harbor" {
					sacks = append(sacks, id)
				}
			}
			if len(sacks) != 2 {
				t.Fatal("expected two stored sacks")
			}
			for _, a := range s.catanExplorerChoices(p) {
				if a.Type == "catan_explorer_transfer" && slices.Equal(a.SpiceLoad, sacks) && slices.Equal(a.Take, []int{p * 11}) {
					explorerSpiceApply(t, s, a)
					return
				}
			}
			t.Fatal("no legal simultaneous settler/spice swap")
		})
	}
}

func TestCatanExplorerSpiceNetworkSetupFixture(t *testing.T) {
	raw, err := os.ReadFile("../server/testdata/catan_explorer_spice_setup.json")
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if err = s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	x := s.Catan.Explorer
	if s.Phase != "catan_explorer_setup" || x.Setup == nil || x.Setup.Step != 0 || len(s.Catan.Players) != 3 || s.Catan.RollID != 0 || x.Board.Scenario != "spices-for-catan" || x.Lairs != nil {
		t.Fatal("not an untouched official three-player setup")
	}
	for _, p := range s.Catan.Players {
		if sum(p.Resources) != 0 || p.Score != 0 {
			t.Fatal("fixture injected resources or scores")
		}
	}
	for step := 0; s.CatanPendingActor() >= 0; step++ {
		if step >= 12 {
			t.Fatal("automatic setup stalled")
		}
		s.AutoCatanPending()
		s = *explorerStateRestore(t, &s)
	}
	if s.Phase != "catan_roll" || s.Catan.TurnSerial != 1 {
		t.Fatal("fixture failed normal setup")
	}
}
