package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanFishingInventionDoubleNumbersContinueAndRestore(t *testing.T) {
	for _, n := range []int{4, 6} {
		for _, helpers := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/helpers%t", n, helpers), func(t *testing.T) {
				s, err := NewCatanFishingCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4, Helpers: helpers, AllHelpers: helpers}, CatanSeafarersSetup{Scenario: "desert"}, nil)
				if err != nil {
					t.Fatal(err)
				}
				finishFishHelperSetup(t, s)
				s.Phase = "catan_turn"
				p := s.Turn
				recipient := s.Catan.Fishing.Map.ExtraNumbers[len(s.Catan.Fishing.Map.ExtraNumbers)-1].Tile
				slots := []int{1, 1}
				if n == 6 {
					slots = []int{0, 1, 0}
				}
				for _, slot := range slots {
					g := s.Catan
					left := CatanNumberToken{recipient, slot}
					old := *g.numberToken(left)
					right := CatanNumberChoice{}
					found := false
					for _, candidate := range g.inventionNumbers() {
						if candidate.Tile != recipient && candidate.Slot == 0 && candidate.Number != old {
							right = candidate
							found = true
							break
						}
					}
					if !found {
						t.Fatal("no invention partner")
					}
					ckProgressGive(t, s, p, 3)
					helperReject(t, s, p, Action{Type: "catan_progress", Card: 3, Tile: recipient, Target: right.Tile, Tokens: []int{2, 0}})
					if n == 4 {
						helperReject(t, s, p, Action{Type: "catan_progress", Card: 3, Tile: recipient, Target: right.Tile})
					}
					helperApply(t, s, p, Action{Type: "catan_progress", Card: 3, Tile: recipient, Target: right.Tile, Tokens: []int{slot, 0}})
					if *s.Catan.numberToken(left) != right.Number || s.Catan.Tiles[right.Tile].Number != old {
						t.Fatal("wrong selected disc")
					}
					fishKnightHelperRestore(t, s)
					if !slices.Contains(s.Catan.inventionTiles(), recipient) {
						t.Fatal("double-number tile disappeared")
					}
					damaged := clone(*s)
					damaged.Catan.Fishing.Map.NumberSwaps = nil
					if len(s.Catan.Fishing.Map.NumberSwaps) == 1 && damaged.Catan.validateFishing() == nil {
						t.Fatal("unrecorded number mutation accepted")
					}
					damaged = clone(*s)
					damaged.Catan.Fishing.Map.NumberSwaps[0].Before[0] = 6
					if damaged.Catan.validateFishing() == nil {
						t.Fatal("invalid swap history accepted")
					}
				}
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal("bot stalled after invention", err)
				}
				helperApply(t, s, p, a)
				for step := 0; step < 20 && !s.Finished; step++ {
					fishHelperStep(t, s)
				}
				fishKnightHelperRestore(t, s)
			})
		}
	}
}
