package game

import (
	"reflect"
	"testing"
)

func TestSplendorEliminationReturnsResourcesAndContinues(t *testing.T) {
	s := mustGame(t, "splendor", 4)
	apply(t, s, Action{Type: "reserve", Tier: 1})
	for s.Turn != 0 {
		s.gemNext()
	}
	card := s.Splendor.Players[0].Reserved[0]
	s.Phase = "discard"
	if err := s.EliminateSplendor(0); err != nil {
		t.Fatal(err)
	}
	g := s.Splendor
	if !g.Players[0].Eliminated || sum(g.Players[0].Tokens) != 0 || len(g.Players[0].Reserved) != 0 || g.Bank[5] != 5 {
		t.Fatal("eliminated player's resources were not returned")
	}
	found := false
	for _, c := range g.Decks[0] {
		found = found || c.ID == card.ID
	}
	if !found || s.Turn != 1 || s.Phase != "turn" || s.Finished {
		t.Fatal("reservation or next turn incorrect")
	}
	if err := s.Apply(0, Action{Type: "take", Tokens: []int{1, 1, 1, 0, 0, 0}}); err == nil {
		t.Fatal("eliminated player acted")
	}
	for i := 0; i < 9; i++ {
		if s.Turn == 0 {
			t.Fatal("turn returned to eliminated seat")
		}
		s.gemNext()
	}
}

func TestSplendorEliminationInNoblePhaseDoesNotAwardNoble(t *testing.T) {
	s := mustGame(t, "splendor", 3)
	s.Phase = "noble"
	s.Splendor.Players[0].Bonus = []int{9, 9, 9, 9, 9}
	count := len(s.Splendor.Nobles)
	if err := s.EliminateSplendor(0); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "turn" || len(s.Splendor.Nobles) != count || len(s.Splendor.Players[0].Nobles) != 0 {
		t.Fatal("pending noble was awarded to eliminated player")
	}
}

func TestSplendorFinalRoundSkipsEliminatedAndExcludesTheirScore(t *testing.T) {
	s := mustGame(t, "splendor", 4)
	s.Splendor.Players[0].Score = 30
	if err := s.EliminateSplendor(0); err != nil {
		t.Fatal(err)
	}
	if s.Splendor.LastRound {
		t.Fatal("unfinished turn of eliminated player triggered final round")
	}
	s.Splendor.Players[1].Score = 15
	s.gemNext()
	if s.Finished {
		t.Fatal("final round ended early")
	}
	if err := s.EliminateSplendor(2); err != nil {
		t.Fatal(err)
	}
	if s.Turn != 3 || s.Finished {
		t.Fatal("remaining player lost their final turn")
	}
	s.gemNext()
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{1}) {
		t.Fatal("eliminated player ranked or round failed to end", s.Winners)
	}
}

func TestSplendorEliminationLeavesOneWinner(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	if err := s.EliminateSplendor(1); err == nil {
		t.Fatal("non-current player eliminated")
	}
	if err := s.EliminateSplendor(0); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Phase != "finished" || !reflect.DeepEqual(s.Winners, []int{1}) {
		t.Fatal("sole remaining player did not win")
	}
	if err := s.EliminateSplendor(1); err == nil {
		t.Fatal("eliminated player after game finished")
	}
	r := mustGame(t, "rail", 2)
	if err := r.EliminateSplendor(0); err == nil {
		t.Fatal("Splendor elimination changed rail game")
	}
}
