package game

import (
	"fmt"
	"testing"
)

// This verifies completion/resume mechanics, not the quality of the current
// identity-oriented target heuristics. Faction-aware strategy is audited apart.
func TestSanguoshaHegemonyBotFlow(t *testing.T) {
	for _, n := range []int{4, 8} {
		t.Run(fmt.Sprintf("%d seats", n), func(t *testing.T) {
			s := &State{Kind: "sanguosha", Round: 1}
			s.initSGHegemony(n)
			for step := 0; step < 8000 && !s.Finished; step++ {
				i := s.SanguoshaActor()
				a, err := s.sgBot(i)
				if err != nil {
					t.Fatalf("step %d actor %d prompt %+v: %v", step, i, s.Sanguosha.Pending, err)
				}
				if err := s.Apply(i, a); err != nil {
					t.Fatalf("step %d action %+v: %v", step, a, err)
				}
				if step%50 == 0 {
					sgHegConservation(t, s)
				}
				if step%200 == 0 {
					sgRestoreMountain(t, s)
				}
			}
			if !s.Finished || len(s.Winners) == 0 {
				t.Fatalf("bot game did not settle: round %d, prompt %+v", s.Round, s.Sanguosha.Pending)
			}
			sgHegConservation(t, s)
		})
	}
}
