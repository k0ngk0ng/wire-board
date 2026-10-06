package game

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCatanExplorerFullNetworkSetupFixture(t *testing.T) {
	raw, err := os.ReadFile("../server/testdata/catan_explorer_full_setup.json")
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
	if s.Phase != "catan_explorer_setup" || x.Setup == nil || x.Setup.Step != 0 || len(s.Catan.Players) != 3 || s.Catan.RollID != 0 || x.Board.Scenario != "explorers-and-pirates" || x.Lairs == nil || x.Fish == nil || x.Spice == nil || x.Board.Target != 17 {
		t.Fatal("not a normal full-scenario setup")
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
