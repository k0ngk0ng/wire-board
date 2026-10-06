package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerDepartureReturnsMobilePiecesAndSkipsRolls(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn", "catan_explorer_move"} {
		t.Run(phase, func(t *testing.T) {
			s, err := newCatanExplorerState(3)
			if err != nil {
				t.Fatal(err)
			}
			actor := s.Turn
			if phase != "catan_roll" {
				if err = s.catanExplorerRoll([2]int{1, 1}); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "catan_explorer_move" {
				if err = s.Apply(actor, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
					t.Fatal(err)
				}
			}
			before := clone(*s)
			if err = s.EliminateCatan(actor); err != nil {
				t.Fatal(err)
			}
			g, x := s.Catan, s.Catan.Explorer
			if s.Finished || s.Turn != (actor+1)%3 || g.TurnSerial != 2 || s.Phase != "catan_roll" || !g.Players[actor].Eliminated || sum(g.Players[actor].Resources) != 0 || x.Economy.Gold[actor] != 0 {
				t.Fatal("departure turn/holdings")
			}
			for r, n := range before.Catan.Players[actor].Resources {
				if g.Bank[r] != before.Catan.Bank[r]+n {
					t.Fatal("resource not returned")
				}
			}
			if x.Economy.GoldBank != before.Catan.Explorer.Economy.GoldBank+before.Catan.Explorer.Economy.Gold[actor] {
				t.Fatal("gold not returned")
			}
			for unit := actor * 11; unit < (actor+1)*11; unit++ {
				if x.Cargo.Units[unit] != (catanExplorerCargoLocation{"supply", -1}) {
					t.Fatal("departed cargo remains deployed")
				}
			}
			for ship := actor * 3; ship < (actor+1)*3; ship++ {
				if x.Fleet.Positions[ship] != -1 {
					t.Fatal("departed ship blocks sea")
				}
			}
			if !reflect.DeepEqual(g.Vertices, before.Catan.Vertices) || !reflect.DeepEqual(g.Edges, before.Catan.Edges) {
				t.Fatal("fixed buildings/roads must remain")
			}
			wantSkip := 0
			if phase == "catan_roll" {
				wantSkip = 1
			}
			if x.SkippedRolls != wantSkip || g.RollID != before.Catan.RollID {
				t.Fatal("departure must not fabricate production")
			}
			s = explorerStateRestore(t, s)
			for steps := 0; steps < 100 && s.Catan.TurnSerial < 6 && !s.Finished; steps++ {
				if s.Turn == actor {
					t.Fatal("rotation returned to absent player")
				}
				p := s.Turn
				if s.Phase == "catan_discard" {
					for i, due := range s.Catan.DiscardDue {
						if due > 0 {
							p = i
							break
						}
					}
				}
				a, e := s.BotAction(p)
				if e == nil {
					e = s.Apply(p, a)
				}
				if e != nil {
					t.Fatal(e)
				}
			}
			if s.Catan.TurnSerial < 6 && !s.Finished {
				t.Fatal("remaining game stuck")
			}
			if sum(s.Catan.Players[actor].Resources) != 0 || s.Catan.Explorer.Economy.Gold[actor] != 0 {
				t.Fatal("departed buildings still produce or get compensation")
			}
			explorerStateRestore(t, s)
		})
	}
}

func TestCatanExplorerDepartureRejectsResponsesAndAwardsSurvivor(t *testing.T) {
	s, err := newCatanExplorerState(3)
	if err != nil {
		t.Fatal(err)
	}
	a := s.Turn
	explorerFlowResources(s, map[int][]int{a: {8, 0, 0, 0, 0}})
	if err = s.catanExplorerRoll([2]int{3, 4}); err != nil {
		t.Fatal(err)
	}
	for _, target := range []int{a, (a + 1) % 3, -1, 3} {
		before, _ := json.Marshal(s)
		if err = s.EliminateCatan(target); err == nil {
			t.Fatal("cannot remove during mandatory discard or wrong seat")
		}
		after, _ := json.Marshal(s)
		if string(before) != string(after) {
			t.Fatal("invalid departure mutated state")
		}
	}
	s.AutoCatanPending()
	if err = s.EliminateCatan(a); err != nil {
		t.Fatal(err)
	}
	if err = s.EliminateCatan(s.Turn); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Phase != "finished" || !slices.Equal(s.Winners, []int{s.Turn}) || s.Catan.Players[s.Turn].Score >= 8 || s.Catan.Explorer.SkippedRolls != 1 {
		t.Fatal("last player should win below point target, without fabricated roll")
	}
	explorerStateRestore(t, s)
	if err = s.EliminateCatan(s.Turn); err == nil {
		t.Fatal("cannot remove final winner")
	}
}
