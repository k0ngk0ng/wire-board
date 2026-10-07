package game

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Exact private constructor snapshots and naturally reached shared lairs.
// Only the token-number input is artificial; no resources/scores were injected.
func TestCatanExplorerPairedNetworkFixtures(t *testing.T) {
	for _, name := range []string{"setup_5", "setup_6", "resolve_false", "resolve_true"} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile("../server/testdata/catan_explorer_paired_" + name + ".json")
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
			n := len(s.Catan.Players)
			x := s.Catan.Explorer
			if name == fmt.Sprintf("setup_%d", n) {
				if s.Phase != "catan_explorer_setup" || x.Setup == nil || x.Setup.Step != 0 || s.Catan.RollID != 0 || x.Board.Scenario != "explorers-and-pirates" {
					t.Fatal("not a normal opening")
				}
				for _, p := range s.Catan.Players {
					if sum(p.Resources) != 0 || p.Score != 0 {
						t.Fatal("injected opening")
					}
				}
				steps := 0
				for x.Setup != nil {
					if steps >= 4*n {
						t.Fatal("setup stalled")
					}
					s.AutoCatanPending()
					s = *explorerStateRestore(t, &s)
					x = s.Catan.Explorer
					steps++
				}
				if steps != 4*n || s.Phase != "catan_roll" {
					t.Fatal("opening failed")
				}
			} else {
				if n != 6 || s.Phase != "catan_explorer_resolve" || fmt.Sprintf("resolve_%t", s.Catan.Paired.Second) != name {
					t.Fatal("wrong response portion")
				}
				actor, serial, rolls := s.Turn, s.Catan.TurnSerial, s.Catan.RollID
				shared := false
				for _, site := range x.Lairs.Sites {
					if site.Ready == serial && site.Resolved == 0 {
						owners := map[int]bool{}
						for _, u := range x.Cargo.contents(catanExplorerCargoLocation{"lair", site.Tile}) {
							owners[u/11] = true
						}
						shared = shared || len(owners) > 1
					}
				}
				if !shared {
					t.Fatal("missing shared lair")
				}
				for step := 0; s.Catan.TurnSerial == serial; step++ {
					if step > 60 {
						t.Fatal("response stalled")
					}
					a, e := s.BotAction(actor)
					if e != nil {
						t.Fatal(e)
					}
					if e = s.Apply(actor, a); e != nil {
						t.Fatal(e)
					}
					s = *explorerStateRestore(t, &s)
				}
				if s.Catan.RollID != rolls || s.Catan.TurnSerial != serial+1 || s.Finished {
					t.Fatal("response invented production or ended early")
				}
				want := "catan_turn"
				if name == "resolve_true" {
					want = "catan_roll"
				}
				if s.Phase != want {
					t.Fatal("wrong next portion", s.Phase)
				}
			}
		})
	}
}
