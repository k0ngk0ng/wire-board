package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCatanAttackTransportState(t *testing.T) {
	for n := 2; n <= 6; n++ {
		s, err := newCatanAttackTransportState(n)
		if err != nil {
			t.Fatal(n, err)
		}
		for step := 0; s.Catan.setup() && step < 50; step++ {
			a, e := s.BotAction(s.Turn)
			if e != nil {
				t.Fatal(n, step, e)
			}
			if e = s.Apply(s.Turn, a); e != nil {
				t.Fatal(n, step, s.Phase, a, e)
			}
		}
		if s.Catan.setup() {
			t.Fatal("setup stuck")
		}
	}
}

func TestCatanAttackTransportNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanAttackTransportState(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				battles, moves := 0, 0
				for step := 0; step < 20000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(step, s.Phase, e)
					}
					before := s.Phase
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, before, a, e)
					}
					if a.Type == "catan_transport_step" {
						moves++
					}
					if s.Catan.Attack.End != nil {
						battles = max(battles, s.Catan.Attack.End.ID)
					}
					if step%79 == 0 {
						raw, _ := json.Marshal(s)
						var restored State
						if e = json.Unmarshal(raw, &restored); e != nil {
							t.Fatal(e)
						}
						if e = restored.validateCatanAttack(); e != nil {
							t.Fatal(e)
						}
						if e = restored.validateCatanTransport(); e != nil {
							t.Fatal(e)
						}
						s = &restored
						for v := -1; v < n; v++ {
							s.View(v)
						}
					}
				}
				if !s.Finished || moves == 0 || battles == 0 {
					t.Fatal("incomplete", s.Round, moves, battles)
				}
				t.Log("round", s.Round, "moves", moves, "battle phases", battles)
			})
		}
	}
}

func TestCatanAttackTransportKnightsNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := newCatanAttackTransportKnights(n)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				moves, plans := 0, 0
				for step := 0; step < 20000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(step, s.Phase, e)
					}
					before := s.Phase
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, before, a, e)
					}
					if a.Type == "catan_transport_step" {
						moves++
					}
					if before != s.Phase && s.Phase == catanAttackCityMovePhase {
						plans++
					}
					if step%83 == 0 {
						raw, _ := json.Marshal(s)
						var restored State
						if e = json.Unmarshal(raw, &restored); e != nil {
							t.Fatal(e)
						}
						if e = restored.validateCatanTransport(); e != nil {
							t.Fatal(e)
						}
						s = &restored
						for v := -1; v < n; v++ {
							s.View(v)
						}
					}
				}
				if !s.Finished || moves == 0 || plans == 0 {
					t.Fatal("incomplete", s.Round, moves, plans)
				}
				t.Log("round", s.Round, "wagon moves", moves, "knight plans", plans)
			})
		}
	}
}
