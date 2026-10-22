package shengji

// Failure cases: bury splits a useful pair or high trumps; bury ignores a
// short side suit; lead throws weak pairs instead of taking initiative with
// a strong pair; final follower fails to feed a partner's winning trick.
import (
	"slices"
	"testing"
)

func TestBotBuryPreservesPairsAndVoidsShortSuit(t *testing.T) {
	h := cards("C3 C3 C4 C6 C8 C9 D4 D6 H2 H2")
	got := botBury(h, Rules{Decks: 2, Level: 2, Trump: 2}, 2)
	want := cards("D4 D6")
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("bury %v, want %v", got, want)
	}
}
func TestBotLeadUsesStrongPair(t *testing.T) {
	s := State{Options: Options{Decks: 2}, Level: 2, Trump: 2, Hands: [][]int{cards("C3 C3 CA CA D4 D7 H9")}}
	m := s.botPlay(0, s.moves(0))
	if !slices.Equal(m.Cards, cards("CA CA")) {
		t.Fatalf("weak initiative: %+v", m)
	}
}
func TestBotFeedsLastPartnerWithoutOvertaking(t *testing.T) {
	s := State{Options: Options{Decks: 2}, Level: 2, Trump: 2, Hands: [][]int{cards("C5 C9")}}
	shape, e := leadShape(cards("C3"), s.rules())
	if e != nil {
		t.Fatal(e)
	}
	s.Lead = &shape
	s.Trick = []Play{{Seat: 1, Cards: cards("C3")}, {Seat: 2, Cards: cards("CA")}, {Seat: 3, Cards: cards("CQ")}}
	m := s.botPlay(0, s.moves(0))
	if !slices.Equal(m.Cards, cards("C5")) {
		t.Fatalf("did not feed partner: %+v", m)
	}
}
