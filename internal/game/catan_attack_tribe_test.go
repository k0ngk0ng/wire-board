package game

import (
	"fmt"
	"testing"
)

func TestCatanAttackTribeNatural(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := NewCatanAttackTribe(n)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
			rewards, ports := 0, 0
			for step := 0; step < 18000 && !s.Finished; step++ {
				p := twoFullActor(s)
				if s.Catan.Attack.TribeRoute != nil {
					rewards++
				}
				if s.Phase == "catan_port" {
					ports++
				}
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(step, s.Phase, e)
				}
				if e = s.Apply(p, a); e != nil {
					t.Fatal(step, s.Phase, e)
				}
				if step == 83 || s.Catan.Attack.TribeRoute != nil {
					b := clone(*s)
					s = &b
					if e = s.validateCatanAttack(); e != nil {
						t.Fatal(e)
					}
				}
			}
			if !s.Finished {
				t.Fatal("unfinished")
			}
			t.Log("rounds", s.Round, "card responses", rewards, "ports", ports)
		})
	}
}
