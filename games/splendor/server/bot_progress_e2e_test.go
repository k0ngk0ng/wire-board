package splendor

import (
	"math/rand"
	"testing"
)

func finishBotMatch(t *testing.T, s *State) {
	t.Helper()
	for step := 0; step < 1400 && !s.Finished; step++ {
		actor := s.Turn
		action, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(actor, action); err != nil {
			t.Fatalf("illegal action %+v: %v", action, err)
		}
		gemInvariant(t, s)
	}
	if !s.Finished || len(s.Winners) == 0 {
		t.Fatalf("bots did not finish, round %d", s.Round)
	}
}

func TestBotsEscapeCapacityGoldCycle(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	g := s.Splendor
	all := Cards()
	used := map[int]bool{}
	cards := func(ids ...int) []Card {
		result := []Card{}
		for _, id := range ids {
			for _, card := range all {
				if card.ID == id {
					result = append(result, card)
					used[id] = true
					break
				}
			}
		}
		return result
	}
	g.Market = [][]Card{cards(12, 36, 31, 35), cards(48, 43, 49, 65), cards(75, 81, 83, 89)}
	g.Players[0].Cards = cards(26, 24, 45)
	g.Players[0].Reserved = cards(88, 87)
	g.Players[0].Tokens = []int{2, 1, 0, 4, 3, 0}
	g.Players[0].Bonus = []int{1, 0, 0, 1, 1}
	g.Players[0].Score = 1
	g.Players[1].Cards = cards(3)
	g.Players[1].Reserved = cards(55)
	g.Players[1].Tokens = []int{2, 3, 4, 0, 1, 1}
	g.Players[1].Bonus = []int{0, 0, 1, 0, 0}
	g.Decks = make([][]Card, 3)
	for _, card := range all {
		if !used[card.ID] {
			g.Decks[card.Tier-1] = append(g.Decks[card.Tier-1], card)
		}
	}
	g.Nobles = []Noble{}
	for _, noble := range Nobles() {
		if noble.ID == 3 || noble.ID == 5 || noble.ID == 6 {
			g.Nobles = append(g.Nobles, noble)
		}
	}
	g.Bank = []int{0, 0, 0, 0, 0, 4}
	s.Turn, s.Phase = 1, "discard"
	gemInvariant(t, s)
	finishBotMatch(t, s)
}

func TestBotsCompleteDeterministicBaseDecks(t *testing.T) {
	for seed := int64(0); seed < 96; seed++ {
		s := mustGame(t, "splendor", 2)
		g := s.Splendor
		random := rand.New(rand.NewSource(seed))
		g.Decks = make([][]Card, 3)
		for _, card := range Cards() {
			g.Decks[card.Tier-1] = append(g.Decks[card.Tier-1], card)
		}
		for tier := range g.Decks {
			deck := g.Decks[tier]
			random.Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
			g.Market[tier] = append([]Card{}, deck[:4]...)
			g.Decks[tier] = deck[4:]
		}
		g.Nobles = Nobles()
		random.Shuffle(len(g.Nobles), func(i, j int) { g.Nobles[i], g.Nobles[j] = g.Nobles[j], g.Nobles[i] })
		g.Nobles = g.Nobles[:3]
		finishBotMatch(t, s)
	}
}
