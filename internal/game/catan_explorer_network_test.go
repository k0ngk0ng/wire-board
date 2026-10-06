package game

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCatanExplorerLairsNetworkFixturesAndTimeouts(t *testing.T) {
	for _, name := range []string{"setup", "discard", "resolve", "chase"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("../server/testdata/catan_explorer_lairs_" + name + ".json")
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
			for i := 0; s.CatanPendingActor() >= 0 || s.Phase == "catan_discard"; i++ {
				if i > 60 {
					t.Fatal("timeout stalled")
				}
				before, _ := json.Marshal(s)
				s.AutoCatanPending()
				after, _ := json.Marshal(s)
				if string(before) == string(after) {
					t.Fatal("automatic response made no progress", s.Phase)
				}
				s = *explorerStateRestore(t, &s)
			}
			if s.CatanPendingActor() != -1 {
				t.Fatal("stale responder")
			}
		})
	}
}
