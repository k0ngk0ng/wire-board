package game

import (
	"fmt"
	"testing"
)

func TestCatanAttackPiratesNatural(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, e := NewCatanAttackPirates(n)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
				t.Fatal(e)
			}
			for step := 0; step < 20000 && !s.Finished; step++ {
				p := twoFullActor(s)
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(step, s.Phase, e)
				}
				if e = s.Apply(p, a); e != nil {
					t.Fatal(step, s.Phase, e)
				}
				if s.Phase == "catan_attack_landing" {
					b := clone(*s)
					s = &b
					if e = s.validateCatanAttack(); e != nil {
						t.Fatal(e)
					}
				}
			}
			if !s.Finished {
				t.Fatal("unfinished", s.Round)
			}
			t.Log("round", s.Round)
		})
	}
}
