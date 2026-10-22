package splendor

// Failure inventory: always buying a free zero-point card; ignoring expensive
// high-point goals; never reserving for a missing gold; not blocking a public
// winning purchase; peeking at private reservations/deck order; filling every
// reserve slot pointlessly; choosing an illegal action under expansions.
import (
	"reflect"
	"testing"
)

func strategyTable(t *testing.T) *State {
	s := mustGame(t, "splendor", 2)
	s.Splendor.Market = [][]Card{{}, {}, {}}
	s.Splendor.Nobles = nil
	return s
}
func strategyCard(id, points int, cost []int) Card {
	return Card{ID: id, Tier: 3, Points: points, Color: 1, Cost: cost}
}
func TestBotPlansHighPointInsteadOfFreeFiller(t *testing.T) {
	s := strategyTable(t)
	g := s.Splendor
	g.Market[0] = []Card{strategyCard(901, 0, []int{0, 0, 0, 0, 0})}
	g.Market[2] = []Card{strategyCard(902, 4, []int{7, 0, 0, 0, 0})}
	g.Players[0].Bonus[0] = 2
	g.Players[0].Tokens[0] = 3
	a, e := s.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	if a.Type != "take" || a.Tokens[0] != 2 {
		t.Fatalf("did not plan seven-cost card: %+v", a)
	}
	if e = s.Apply(0, a); e != nil {
		t.Fatal(e)
	}
	s.Turn = 0
	a, e = s.BotAction(0)
	if e != nil || a.Type != "buy" || a.Card != 902 {
		t.Fatalf("did not follow through: %+v %v", a, e)
	}
}
func TestBotReservesForUsefulGold(t *testing.T) {
	s := strategyTable(t)
	g := s.Splendor
	c := strategyCard(903, 4, []int{5, 0, 0, 0, 0})
	g.Market[2] = []Card{c}
	g.Bank[0] = 0
	g.Players[0].Tokens[0] = 4
	a, e := s.BotAction(0)
	if e != nil || a.Type != "reserve" || a.Card != c.ID {
		t.Fatalf("did not obtain useful gold: %+v %v", a, e)
	}
	if e = s.Apply(0, a); e != nil {
		t.Fatal(e)
	}
	s.Turn = 0
	a, e = s.BotAction(0)
	if e != nil || a.Type != "buy" || a.Card != c.ID {
		t.Fatalf("did not buy reservation: %+v %v", a, e)
	}
}
func TestBotBlocksPublicWinButTakesOwnWin(t *testing.T) {
	s := strategyTable(t)
	g := s.Splendor
	g.Market[0] = []Card{strategyCard(904, 0, []int{0, 0, 0, 0, 0})}
	g.Market[2] = []Card{strategyCard(905, 4, []int{0, 0, 0, 0, 7})}
	g.Players[1].Score = 11
	g.Players[1].Bonus[4] = 7
	a, e := s.BotAction(0)
	if e != nil || a.Type != "reserve" || a.Card != 905 {
		t.Fatalf("did not block public winning card: %+v %v", a, e)
	}
	g.Players[0].Score = 11
	g.Players[0].Bonus[4] = 7
	a, e = s.BotAction(0)
	if e != nil || a.Type != "buy" || a.Card != 905 {
		t.Fatalf("missed own win: %+v %v", a, e)
	}
}
func TestBotStrategyIgnoresSecrets(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	a, e := s.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	s.Splendor.Players[1].Reserved = []Card{strategyCard(9999, 99, []int{0, 0, 0, 0, 0})}
	for _, deck := range s.Splendor.Decks {
		for i, j := 0, len(deck)-1; i < j; i, j = i+1, j-1 {
			deck[i], deck[j] = deck[j], deck[i]
		}
	}
	b, e := s.BotAction(0)
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("secrets changed decision: %+v %+v %v", a, b, e)
	}
}
