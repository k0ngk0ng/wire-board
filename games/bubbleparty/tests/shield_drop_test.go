package bubbleparty_test

// Failure inventory: docs/research/bubbleparty/shield-correction-20261021.md.
// Exercise crate explosion and drops through public module initialization,
// saved-state restoration and timer; no direct call to the drop function.
import (
	"encoding/json"
	"testing"

	bubbleparty "github.com/k0ngk0ng/wire-board/games/bubbleparty/server"
)

func TestClassicCrateDropsExcludeCustomPassiveShield(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "classic"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			seen := map[string]int{}
			for seed := uint64(1); seed <= 64; seed++ {
				m := refactorArena(t, func(s *bubbleparty.State) {
					s.Seed = seed
					if legacy {
						s.RulesVersion = 0
						s.SuddenAt = s.StartAt + 180000
					}
					for _, xy := range [][2]int{{5, 4}, {5, 6}, {4, 5}, {6, 5}} {
						s.Cells[xy[1]*bubbleparty.Cols+xy[0]] = bubbleparty.CellCrate
					}
					for i := range s.Players {
						s.Players[i].X = 2 + i
						s.Players[i].Y = 2
					}
					s.BombID = 1
					s.Bombs = []bubbleparty.Bomb{{ID: 1, Seat: 0, Team: 0, X: 5, Y: 5, Range: 1, At: s.ServerAt - 2000, ExplodeAt: s.ServerAt + 20}}
					s.Cells[5*bubbleparty.Cols+5] = bubbleparty.CellBomb
				})
				m = refactorTick(t, m, refactorRead(t, m, 0).ServerAt+100)
				var state bubbleparty.State
				if err := json.Unmarshal(m.Data, &state); err != nil {
					t.Fatal(err)
				}
				for _, item := range state.Items {
					seen[item.Type]++
					if !legacy && item.Type == bubbleparty.ItemShield {
						t.Fatalf("classic crate dropped custom passive shield at seed %d", seed)
					}
				}
			}
			for _, kind := range []string{bubbleparty.ItemSpeed, bubbleparty.ItemBomb, bubbleparty.ItemRange} {
				if seen[kind] == 0 {
					t.Fatalf("missing basic drop %s: %v", kind, seen)
				}
			}
			if legacy && seen[bubbleparty.ItemShield] == 0 {
				t.Fatal("legacy shield pool changed")
			}
		})
	}
}
