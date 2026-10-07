package game

import (
	"fmt"
	"reflect"
	"testing"
)

func explorerPairedStart(t *testing.T, n int, scene string) *State {
	t.Helper()
	// Explicit artificial eight-token input; physical faces remain a release gate.
	s, err := newCatanExplorerMissionState(n, scene, "variable", []int{2, 3, 4, 5, 6, 8, 9, 10})
	if err != nil {
		t.Fatal(err)
	}
	steps := 0
	for s.Catan.Explorer.Setup != nil {
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(s.Turn, a); e != nil {
			t.Fatal(e)
		}
		s = explorerStateRestore(t, s)
		steps++
	}
	if steps != 4*n || s.Phase != "catan_roll" || s.Catan.Explorer.Economy.GoldBank != 172-2*n {
		t.Fatal("expanded formal setup")
	}
	for p, player := range s.Catan.Players {
		if player.Score != 3 || s.Catan.Explorer.Cargo.Units[p*11] != (catanExplorerCargoLocation{"ship", p * 3}) {
			t.Fatal("opening buildings/settler")
		}
	}
	return s
}
func explorerPairedAct(t *testing.T, s *State, kind string) {
	t.Helper()
	if err := s.Apply(s.Turn, Action{Type: kind, Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(kind, err)
	}
}
func explorerPairedEnd(t *testing.T, s *State) {
	t.Helper()
	explorerPairedAct(t, s, "catan_explorer_begin_move")
	explorerPairedAct(t, s, "catan_end")
}
func TestCatanExplorerPairedFormalSetupAndRotation(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, scene := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scene), func(t *testing.T) {
				s := explorerPairedStart(t, n, scene)
				start := s.Catan.StartPlayer
				for i := 0; i < 2*n; i++ {
					g := s.Catan
					primary := (start + i) % n
					secondary := (primary + 3) % n
					if s.Turn != primary || s.Round != 1+i/n || s.Phase != "catan_roll" || g.TurnSerial != uint64(2*i+1) || g.RollID != i || g.Paired.Primary != primary || g.Paired.Secondary != secondary || g.Paired.Second {
						t.Fatal("primary rotation", i)
					}
					if err := s.catanExplorerRoll([2]int{1, 1}); err != nil {
						t.Fatal(err)
					}
					// First player may offer a gold/resource trade; it must not survive handoff.
					if err := s.Apply(primary, Action{Type: "catan_trade_offer", Prompt: int(g.TurnSerial), Give: make([]int, 5), Take: []int{1, 0, 0, 0, 0}, GoldGive: 1}); err != nil {
						t.Fatal(err)
					}
					explorerPairedEnd(t, s)
					g = s.Catan
					if s.Turn != secondary || s.Phase != "catan_turn" || g.RollID != i+1 || g.TurnSerial != uint64(2*i+2) || !g.Paired.Second || g.Trade != nil || !g.Explorer.Economy.Turn.NoProduction || g.Explorer.Economy.Turn.Bought != 0 {
						t.Fatal("secondary handoff")
					}
					before := clone(*s)
					for _, a := range []Action{
						{Type: "catan_roll", Prompt: int(g.TurnSerial)},
						{Type: "catan_trade_offer", Prompt: int(g.TurnSerial), Give: make([]int, 5), Take: []int{1, 0, 0, 0, 0}, GoldGive: 1},
						{Type: "catan_explorer_begin_move", Prompt: int(g.TurnSerial) - 1},
					} {
						if s.Apply(secondary, a) == nil || !reflect.DeepEqual(*s, before) {
							t.Fatal("secondary illegal action accepted or mutated", a)
						}
					}
					s = explorerStateRestore(t, s)
					explorerPairedEnd(t, s)
					s = explorerStateRestore(t, s)
				}
			})
		}
	}
}
func TestCatanExplorerPairedBankAndMovementAllowances(t *testing.T) {
	s := explorerPairedStart(t, 6, "spices-for-catan")
	if err := s.catanExplorerRoll([2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	explorerPairedEnd(t, s)
	p, seq := s.Turn, int(s.Catan.TurnSerial)
	// Controlled resource/gold fixture, conserving the expanded banks, to
	// exercise both bank purchases and the per-ship wool allowance in part 2.
	x := s.Catan.Explorer
	x.Economy.GoldBank -= 6
	x.Economy.Gold[p] += 6
	for i := 0; i < 2; i++ {
		if err := s.Apply(p, Action{Type: "catan_explorer_bank", Prompt: seq, Color: -1, Target: 2}); err != nil {
			t.Fatal(err)
		}
	}
	explorerFishReject(t, s, p, Action{Type: "catan_explorer_bank", Prompt: seq, Color: -1, Target: 2})
	explorerPairedAct(t, s, "catan_explorer_begin_move")
	ship := p * 3
	if err := s.Apply(p, Action{Type: "catan_explorer_wool", Prompt: seq, Slot: ship}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Explorer.Fleet.Turn.Ships[ship].Remaining != 6 {
		t.Fatal("secondary wool allowance")
	}
	explorerFishReject(t, s, p, Action{Type: "catan_explorer_wool", Prompt: seq, Slot: ship})
	s = explorerStateRestore(t, s)
	explorerPairedAct(t, s, "catan_end")
	if s.Catan.RollID != 1 || s.Catan.Explorer.Economy.Turn.NoProduction {
		t.Fatal("phantom production")
	}
}
func TestCatanExplorerPairedDepartureAndCorruption(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, phase := range []string{"roll", "action", "movement", "secondary", "secondary-movement"} {
			t.Run(fmt.Sprintf("%d/%s", n, phase), func(t *testing.T) {
				s := explorerPairedStart(t, n, "explorers-and-pirates")
				if phase != "roll" {
					if err := s.catanExplorerRoll([2]int{1, 1}); err != nil {
						t.Fatal(err)
					}
				}
				if phase == "secondary" || phase == "secondary-movement" {
					explorerPairedEnd(t, s)
				}
				if phase == "movement" || phase == "secondary-movement" {
					explorerPairedAct(t, s, "catan_explorer_begin_move")
				}
				actor, rolls := s.Turn, s.Catan.RollID
				primary, secondary := s.Catan.Paired.Primary, s.Catan.Paired.Secondary
				if err := s.EliminateCatan(actor); err != nil {
					t.Fatal(err)
				}
				if s.Catan.RollID != rolls || !s.Catan.Players[actor].Eliminated {
					t.Fatal("departure rolled")
				}
				if actor == primary && s.Turn != secondary || actor == secondary && s.Turn != (primary+1)%n {
					t.Fatal("departure rotated wrong marker")
				}
				s = explorerStateRestore(t, s)
				// Repeated removals must retain the expanded economy down to the survivor.
				for !s.Finished {
					if err := s.EliminateCatan(s.Turn); err != nil {
						t.Fatal(err)
					}
					s = explorerStateRestore(t, s)
				}
				if len(s.Winners) != 1 {
					t.Fatal("missing departure winner")
				}
				if sum(s.Catan.Explorer.Economy.Gold)+s.Catan.Explorer.Economy.GoldBank != 172 {
					t.Fatal("departure shrank inventory")
				}
			})
		}
	}
	s := explorerPairedStart(t, 6, "pirate-lairs")
	for _, mutate := range []func(*State){
		func(q *State) { q.Catan.Paired = nil },
		func(q *State) { q.Catan.Paired.Second = true },
		func(q *State) { q.Catan.Paired.Secondary = (q.Catan.Paired.Secondary + 1) % 6 },
		func(q *State) { q.Catan.Explorer.Economy.Turn.NoProduction = true },
		func(q *State) { q.Catan.RollID++ },
	} {
		q := clone(*s)
		mutate(&q)
		if q.validateCatanExplorer() == nil {
			t.Fatal("corrupt paired state accepted")
		}
	}
}

func TestCatanExplorerPairedImmediateVictory(t *testing.T) {
	for _, second := range []bool{false, true} {
		t.Run(fmt.Sprint(second), func(t *testing.T) {
			s := explorerPairedStart(t, 6, "pirate-lairs")
			if err := s.catanExplorerRoll([2]int{1, 1}); err != nil {
				t.Fatal(err)
			}
			if second {
				explorerPairedEnd(t, s)
			}
			// Controlled physical building fixture at the official 12-point target.
			// Victory must stop this portion, before the next marker receives a turn.
			actor, seq, rolls := s.Turn, s.Catan.TurnSerial, s.Catan.RollID
			explorerVictoryBuildings(t, s, actor, 12)
			explorerPairedAct(t, s, "catan_explorer_begin_move")
			if !s.Finished || len(s.Winners) != 1 || s.Winners[0] != actor || s.Catan.TurnSerial != seq || s.Catan.RollID != rolls || s.Catan.Paired.Second != second {
				t.Fatal("paired victory advanced markers")
			}
			explorerStateRestore(t, s)
		})
	}
}

func TestCatanExplorerPairedNaturalMatches(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, scene := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scene), func(t *testing.T) {
				// Normal setup and full bot play without injected resources or scores.
				// Token faces are artificial pending independent physical verification.
				s, err := newCatanExplorerMissionState(n, scene, "variable", []int{2, 3, 4, 5, 6, 8, 9, 10})
				if err != nil {
					t.Fatal(err)
				}
				actions := map[string]int{}
				secondActions := 0
				for step := 0; step < 12000 && !s.Finished; step++ {
					actor := s.Turn
					if s.Phase == "catan_discard" {
						for p, due := range s.Catan.DiscardDue {
							if due > 0 {
								actor = p
								break
							}
						}
					}
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					second := s.Catan.Paired.Second
					if err = s.Apply(actor, a); err != nil {
						t.Fatalf("step %d phase %s actor %d action %+v: %v", step, s.Phase, actor, a, err)
					}
					actions[a.Type]++
					if second {
						secondActions++
					}
					if step%47 == 0 {
						s = explorerStateRestore(t, s)
					}
				}
				t.Log("round", s.Round, "serial", s.Catan.TurnSerial, "rolls", s.Catan.RollID, "actions", actions)
				if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.Explorer.Board.Target || secondActions == 0 {
					t.Fatal("natural paired mission failed to finish")
				}
				if catanExplorerFishScenario(scene) && (actions["catan_explorer_fish_load"] == 0 || actions["catan_explorer_fish_deliver"] == 0) {
					t.Fatal("fish logistics unused")
				}
				if catanExplorerSpiceScenario(scene) && (actions["catan_explorer_spice_land"] == 0 || actions["catan_explorer_spice_deliver"] == 0) {
					t.Fatal("spice logistics unused")
				}
				explorerStateRestore(t, s)
			})
		}
	}
}
