package splendor

import (
	"reflect"
	"testing"
)

func mustGame(t *testing.T, kind string, n int) *State {
	t.Helper()
	s, e := New(kind, n)
	if e != nil {
		t.Fatal(e)
	}
	// Rule fixtures use a fixed seat order; random starts are tested separately.
	if s.Splendor != nil {
		s.Turn, s.Splendor.StartPlayer = 0, 0
	}
	return s
}
func apply(t *testing.T, s *State, a Action) {
	t.Helper()
	if e := s.Apply(s.Turn, a); e != nil {
		t.Fatalf("%+v: %v", a, e)
	}
}
func reject(t *testing.T, s *State, a Action) {
	t.Helper()
	c := clone(*s)
	if e := c.Apply(c.Turn, a); e == nil {
		t.Fatalf("accepted invalid action %+v", a)
	}
}
func TestSplendorSetup(t *testing.T) {
	for n := 2; n <= 4; n++ {
		s := mustGame(t, "splendor", n)
		g := s.Splendor
		tokens := 7
		if n < 4 {
			tokens = n + 2
		}
		if g.Bank[0] != tokens || g.Bank[5] != 5 || len(g.Nobles) != n+1 {
			t.Fatal("incorrect setup")
		}
		if e := s.Apply(1, Action{Type: "take", Tokens: []int{1, 1, 1, 0, 0, 0}}); e == nil {
			t.Fatal("accepted out of turn")
		}
	}
}
func TestSplendorTakeRules(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	reject(t, s, Action{Type: "take", Tokens: []int{1, 1, 0, 0, 0, 0}})
	reject(t, s, Action{Type: "take", Tokens: []int{0, 0, 0, 0, 0, 1}})
	reject(t, s, Action{Type: "take", Tokens: []int{-1, 1, 1, 1, 0, 0}})
	apply(t, s, Action{Type: "take", Tokens: []int{2, 0, 0, 0, 0, 0}})
	reject(t, s, Action{Type: "take", Tokens: []int{2, 0, 0, 0, 0, 0}})
	apply(t, s, Action{Type: "take", Tokens: []int{1, 1, 1, 0, 0, 0}})
	if s.Splendor.Bank[0] != 1 {
		t.Fatal("bank accounting")
	}
}
func TestSplendorShortSupply(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	s.Splendor.Bank = []int{1, 0, 0, 2, 0, 0}
	apply(t, s, Action{Type: "take", Tokens: []int{1, 0, 0, 1, 0, 0}})
}
func TestSplendorReservePrivacyAndLimit(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	id := s.Splendor.Market[0][0].ID
	apply(t, s, Action{Type: "reserve", Card: id})
	if s.Splendor.Players[0].Tokens[5] != 1 || s.Splendor.Players[0].Reserved[0].ID != id {
		t.Fatal("reserve")
	}
	v := s.View(1)["splendor"].(map[string]any)
	if _, ok := v["decks"]; ok {
		t.Fatal("deck leaked")
	}
	op := v["players"].([]any)[0].(map[string]any)
	if _, ok := op["reserved"]; ok {
		t.Fatal("reservation leaked")
	}
	if op["reservedCount"] != 1 {
		t.Fatal("missing public count")
	}
	s.Turn = 0
	s.Splendor.Players[0].Reserved = append(s.Splendor.Players[0].Reserved, Card{}, Card{})
	reject(t, s, Action{Type: "reserve", Tier: 1})
}
func TestSplendorBuyDiscountGoldAndReplacement(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	p := &s.Splendor.Players[0]
	c := Card{ID: 999, Tier: 1, Color: 0, Points: 2, Cost: []int{2, 2, 0, 0, 0}}
	s.Splendor.Market[0][0] = c
	p.Bonus[0] = 1
	p.Tokens = []int{1, 1, 0, 0, 0, 1}
	n := len(s.Splendor.Decks[0])
	apply(t, s, Action{Type: "buy", Card: 999})
	if p.Score != 2 || p.Bonus[0] != 2 || sum(p.Tokens) != 0 || len(s.Splendor.Decks[0]) != n-1 {
		t.Fatalf("purchase result: %+v", p)
	}
}
func TestSplendorDiscardAndNobleChoice(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	p := &s.Splendor.Players[0]
	p.Tokens = []int{2, 2, 2, 2, 2, 0}
	apply(t, s, Action{Type: "reserve", Tier: 1})
	if s.Phase != "discard" || s.Turn != 0 {
		t.Fatal("must discard before advance")
	}
	reject(t, s, Action{Type: "discard", Tokens: []int{0, 0, 0, 0, 0, 0}})
	p.Bonus = []int{5, 5, 5, 5, 5}
	apply(t, s, Action{Type: "discard", Tokens: []int{0, 0, 0, 0, 0, 1}})
	if s.Phase != "noble" {
		t.Fatal("must choose one noble")
	}
	noble := s.Splendor.Nobles[0].ID
	apply(t, s, Action{Type: "noble", Noble: noble})
	if len(p.Nobles) != 1 || p.Score != 3 || s.Turn != 1 {
		t.Fatal("noble result")
	}
}
func TestSplendorEndEqualTurnsAndTieBreak(t *testing.T) {
	s := mustGame(t, "splendor", 3)
	s.Splendor.Players[0].Score = 15
	s.Splendor.Players[0].Cards = []Card{{}, {}}
	s.Splendor.Players[1].Score = 15
	s.Splendor.Players[1].Cards = []Card{{}}
	s.gemNext()
	if s.Finished {
		t.Fatal("ended before final round")
	}
	s.gemNext()
	if s.Finished {
		t.Fatal("ended before last player")
	}
	s.gemNext()
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{1}) {
		t.Fatal(s.Winners)
	}
}
func gemInvariant(t *testing.T, s *State) {
	t.Helper()
	g := s.Splendor
	total := append([]int{}, g.Bank...)
	cards := 0
	for i := 0; i < 3; i++ {
		cards += len(g.Decks[i]) + len(g.Market[i])
	}
	for _, p := range g.Players {
		cards += len(p.Cards) + len(p.Reserved)
		for i, v := range p.Tokens {
			if v < 0 {
				t.Fatal("negative tokens")
			}
			total[i] += v
		}
		if len(p.Reserved) > 3 {
			t.Fatal("reservation overflow")
		}
	}
	if cards != 90 {
		t.Fatal("card conservation", cards)
	}
	want := 7
	if len(g.Players) < 4 {
		want = len(g.Players) + 2
	}
	for i, n := range total {
		w := want
		if i == 5 {
			w = 5
		}
		if n != w {
			t.Fatal("token conservation", total)
		}
	}
}
func TestFullSplendorGame(t *testing.T) {
	for _, n := range []int{2, 4} {
		s := mustGame(t, "splendor", n)
		for step := 0; step < 1200 && !s.Finished; step++ {
			gemInvariant(t, s)
			g := s.Splendor
			p := g.Players[s.Turn]
			if s.Phase == "noble" {
				for _, noble := range g.Nobles {
					if eligible(&p, noble) {
						apply(t, s, Action{Type: "noble", Noble: noble.ID})
						break
					}
				}
				continue
			}
			cards := append([]Card{}, p.Reserved...)
			for _, row := range g.Market {
				cards = append(cards, row...)
			}
			var target Card
			bestDeficit := 999
			for _, c := range cards {
				deficit := 0
				for i, cost := range c.Cost {
					deficit += max(0, cost-p.Bonus[i]-p.Tokens[i])
				}
				if deficit < bestDeficit {
					bestDeficit = deficit
					target = c
				}
			}
			if s.Phase == "discard" {
				drop := make([]int, 6)
				excess := sum(p.Tokens) - 10
				for excess > 0 {
					chosen := -1
					score := -999
					for i, held := range p.Tokens {
						if held-drop[i] == 0 {
							continue
						}
						surplus := held - drop[i]
						if i < 5 && len(target.Cost) > 0 {
							surplus -= max(0, target.Cost[i]-p.Bonus[i])
						} else {
							surplus -= 20
						}
						if surplus > score {
							score = surplus
							chosen = i
						}
					}
					drop[chosen]++
					excess--
				}
				apply(t, s, Action{Type: "discard", Tokens: drop})
				continue
			}
			best := Action{}
			score := -9999
			for _, c := range cards {
				candidate := Action{Type: "buy", Card: c.ID}
				copy := clone(*s)
				if copy.Apply(copy.Turn, candidate) == nil && 1000+c.Points*100 > score {
					best = candidate
					score = 1000 + c.Points*100
				}
			}
			if score < 0 {
				for mask := 1; mask < 32; mask++ {
					tokens := make([]int, 6)
					for i := 0; i < 5; i++ {
						if mask&(1<<i) != 0 {
							tokens[i] = 1
						}
					}
					options := [][]int{tokens}
					if sum(tokens) == 1 {
						double := append([]int{}, tokens...)
						for i, v := range double {
							double[i] = 2 * v
						}
						options = append(options, double)
					}
					for _, take := range options {
						candidate := Action{Type: "take", Tokens: take}
						copy := clone(*s)
						if copy.Apply(copy.Turn, candidate) != nil {
							continue
						}
						value := 0
						for i, n := range take {
							if i < 5 && len(target.Cost) > 0 {
								value += min(n, max(0, target.Cost[i]-p.Bonus[i]-p.Tokens[i])) * 20
							}
						}
						if value > score {
							score = value
							best = candidate
						}
					}
				}
			}
			if score < 0 {
				for _, c := range cards {
					candidate := Action{Type: "reserve", Card: c.ID}
					copy := clone(*s)
					if copy.Apply(copy.Turn, candidate) == nil {
						best = candidate
						score = 1
						break
					}
				}
			}
			if score < 0 {
				best = Action{Type: "pass"}
			}
			apply(t, s, best)
		}
		gemInvariant(t, s)
		if !s.Finished {
			t.Fatalf("%d player Splendor failed to finish", n)
		}
	}
}
func TestSplendorChosenGoldPayment(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	s.Splendor.Market[0][0] = Card{ID: 999, Tier: 1, Color: 2, Cost: []int{2, 1, 0, 0, 0}}
	p := &s.Splendor.Players[0]
	p.Bonus[0] = 1
	p.Tokens = []int{1, 1, 0, 0, 0, 1}
	reject(t, s, Action{Type: "buy", Card: 999, Tokens: []int{1, 1, 0, 0, 0, 1}}) // Overpayment.
	reject(t, s, Action{Type: "buy", Card: 999, Tokens: []int{0, 1, 0, 0, 0, 0}}) // Missing gold.
	reject(t, s, Action{Type: "buy", Card: 999, Tokens: []int{-1, 1, 0, 0, 0, 2}})
	reject(t, s, Action{Type: "buy", Card: 999, Tokens: []int{1, 1}})
	apply(t, s, Action{Type: "buy", Card: 999, Tokens: []int{0, 1, 0, 0, 0, 1}})
	if p.Tokens[0] != 1 || p.Tokens[1] != 0 || p.Tokens[5] != 0 || p.Bonus[2] != 1 {
		t.Fatalf("chosen gold payment not respected: %+v", p)
	}
}
