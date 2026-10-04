package game

import (
	"fmt"
	"testing"
)

// This verifies completion/resume mechanics. Specific decisions and hidden
// information independence have separate faction-policy tests.
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

// Rotate all sixty generals through real drafts and mixed bot/timeout turns.
// Multi-skill state machines must conserve physical cards and survive restores.
func TestSanguoshaHegemonyRosterMixedFlow(t *testing.T) {
	byKingdom := map[string][]string{}
	for _, general := range sgHegemonyGenerals {
		byKingdom[general.Kingdom] = append(byKingdom[general.Kingdom], general.ID)
	}
	seen := map[string]bool{}
	for _, n := range []int{4, 8} {
		for run := 0; run < 32/n; run++ {
			t.Run(fmt.Sprintf("%d seats rotation %d", n, run), func(t *testing.T) {
				s, err := NewSanguosha(n, SGOptions{Mode: "hegemony"})
				if err != nil {
					t.Fatal(err)
				}
				for i := range s.Sanguosha.Players {
					roster := byKingdom[[]string{"wei", "shu", "wu", "qun"}[i%4]]
					start := (run*n/2 + (i/4)*2) % len(roster)
					pair := []string{roster[start], roster[(start+1)%len(roster)]}
					s.Sanguosha.Players[i].Choices = pair
					for _, id := range pair {
						seen[id] = true
					}
				}
				steps := 0
				for ; steps < 12000 && !s.Finished; steps++ {
					i := s.SanguoshaActor()
					var a Action
					if steps%7 == 0 {
						a, err = s.SanguoshaTimeoutAction()
					} else {
						a, err = s.BotAction(i)
					}
					if err != nil {
						t.Fatalf("step %d, prompt %+v: %v", steps, s.Sanguosha.Pending, err)
					}
					if err = s.Apply(i, a); err != nil {
						t.Fatalf("step %d, action %+v: %v", steps, a, err)
					}
					if steps%50 == 0 {
						sgHegConservation(t, s)
					}
					if steps%200 == 0 {
						sgRestoreMountain(t, s)
					}
				}
				if !s.Finished || len(s.Winners) == 0 {
					t.Fatalf("did not settle: round %d, prompt %+v", s.Round, s.Sanguosha.Pending)
				}
				sgHegConservation(t, s)
				t.Logf("finished after %d actions", steps)
			})
		}
	}
	if len(seen) != 60 {
		t.Fatal("roster coverage", len(seen))
	}
}
