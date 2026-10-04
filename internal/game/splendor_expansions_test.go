package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func gemExpandedTest(t *testing.T, o SplendorOptions) *State {
	t.Helper()
	s, err := NewSplendor(3, o)
	if err != nil {
		t.Fatal(err)
	}
	s.Turn, s.Splendor.StartPlayer = 0, 0
	return s
}

func gemExpansionApply(t *testing.T, s *State, a Action) {
	t.Helper()
	if err := s.Apply(s.Turn, a); err != nil {
		t.Fatalf("phase %s action %+v: %v", s.Phase, a, err)
	}
}

func gemExpansionReject(t *testing.T, s *State, a Action) {
	t.Helper()
	before, _ := json.Marshal(s)
	if err := s.Apply(s.Turn, a); err == nil {
		t.Fatalf("accepted %+v in %s", a, s.Phase)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("invalid action mutated game")
	}
}

func TestSplendorPostsCountCardsAndChooseOnlyOne(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{TradingPosts: true})
	s.Splendor.Nobles = nil
	p := &s.Splendor.Players[0]
	// Large discounts alone never earn a post: requirements count cards.
	p.Bonus = []int{9, 9, 9, 9, 9}
	if len(s.gemPostOptions()) != 0 {
		t.Fatal("posts used bonuses instead of cards")
	}
	p.Cards = []Card{{Color: 1}, {Color: 1}, {Color: 3}, {Color: 3}, {Color: 3}}
	gemExpansionApply(t, s, Action{Type: "take", Tokens: []int{1, 1, 1, 0, 0, 0}})
	if s.Phase != "gem_post" || s.Turn != 0 {
		t.Fatal("choice not offered")
	}
	gemExpansionReject(t, s, Action{Type: "gem_post", Card: GemPostDoubleGold})
	gemExpansionApply(t, s, Action{Type: "gem_post", Card: GemPostExtraToken})
	if s.Turn != 1 || !reflect.DeepEqual(s.Splendor.Players[0].TradingPosts, []int{GemPostExtraToken}) {
		t.Fatal("more than one post acquired")
	}
}

func TestSplendorPostPrestigePersistsAndTriggersFinalRound(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{TradingPosts: true})
	p := &s.Splendor.Players[0]
	p.TradingPosts = []int{GemPostPurchaseToken, GemPostExtraToken}
	p.Score = 12
	for i := 0; i < 5; i++ {
		p.Cards = append(p.Cards, Card{Color: 0})
	}
	gemExpansionApply(t, s, Action{Type: "take", Tokens: []int{1, 1, 1, 0, 0, 0}})
	if !s.Splendor.LastRound || s.Splendor.Players[0].Score != 15 {
		t.Fatal("post score did not trigger final round")
	}
	// Once obtained, powers survive discarding their qualifying cards.
	s.Splendor.Players[0].Cards = nil
	s.Turn = 0
	s.gemGainPost(GemPostBlindReserve)
	if s.Splendor.Players[0].Score != 16 || !s.Splendor.Players[0].hasPost(GemPostPrestige) {
		t.Fatal("permanent post lost")
	}
}

func TestSplendorDoubleGoldAssignedPerColor(t *testing.T) {
	p := GemPlayer{Tokens: []int{0, 0, 0, 0, 0, 2}, Bonus: make([]int, 5), TradingPosts: []int{GemPostDoubleGold}}
	for _, tc := range []struct {
		cost []int
		gold int
	}{{[]int{3, 0, 0, 0, 0}, 2}, {[]int{1, 1, 0, 0, 0}, 2}, {[]int{2, 0, 0, 0, 0}, 1}} {
		pay, err := gemPayment(p, Card{Cost: tc.cost}, nil)
		if err != nil || pay[5] != tc.gold {
			t.Fatalf("%v: %v %v", tc.cost, pay, err)
		}
	}
	if _, err := gemPayment(p, Card{Cost: []int{1, 1, 0, 0, 0}}, []int{0, 0, 0, 0, 0, 1}); err == nil {
		t.Fatal("one gold incorrectly split across colors")
	}
	if _, err := gemPayment(p, Card{Cost: []int{2, 0, 0, 0, 0}}, []int{0, 0, 0, 0, 0, 2}); err == nil {
		t.Fatal("unneeded extra gold accepted")
	}
	p.Tokens = []int{1, 0, 0, 0, 0, 1}
	pay, err := gemPayment(p, Card{Cost: []int{2, 0, 0, 0, 0}}, []int{0, 0, 0, 0, 0, 1})
	if err != nil || pay[0] != 0 {
		t.Fatal("voluntary gold substitution rejected", err)
	}
}

func TestSplendorPostBlindReservationPrivacyAndRestore(t *testing.T) {
	for _, size := range []int{1, 2, 3} {
		t.Run(string(rune('0'+size)), func(t *testing.T) {
			s := gemExpandedTest(t, SplendorOptions{TradingPosts: true})
			s.Splendor.Players[0].TradingPosts = []int{GemPostBlindReserve}
			s.Splendor.Decks[0] = []Card{{ID: 501, Tier: 1, Color: 0}, {ID: 502, Tier: 1, Color: 1}, {ID: 503, Tier: 1, Color: 2}}[:size]
			gemExpansionApply(t, s, Action{Type: "reserve", Tier: 1})
			if s.Phase != "gem_reserve" || len(s.Splendor.ReserveChoice) != min(size, 2) {
				t.Fatal("missing private choice")
			}
			for _, viewer := range []int{-1, 1, 2} {
				view, _ := json.Marshal(s.View(viewer))
				if strings.Contains(string(view), "reserveChoice") || strings.Contains(string(view), "501") || strings.Contains(string(view), "502") {
					t.Fatal("blind choice leaked", string(view))
				}
			}
			if _, ok := s.View(0)["splendor"].(map[string]any)["reserveChoice"]; !ok {
				t.Fatal("owner cannot see choice")
			}
			gemExpansionReject(t, s, Action{Type: "gem_reserve", Card: 9999})
			// JSON restoration must preserve an unresolved private choice.
			restored := clone(*s)
			chosen := 501
			if size > 1 {
				chosen = 502
			}
			gemExpansionApply(t, &restored, Action{Type: "gem_reserve", Card: chosen})
			g := restored.Splendor
			if restored.Turn != 1 || len(g.Players[0].Reserved) != 1 || g.Players[0].Reserved[0].ID != chosen || g.Players[0].Tokens[5] != 1 {
				t.Fatal("reservation not completed")
			}
			if size > 1 && g.Decks[0][len(g.Decks[0])-1].ID != 501 {
				t.Fatal("unchosen card not returned to bottom")
			}
			if len(g.CardEvents) != 1 || g.CardEvents[0].Card != nil {
				t.Fatal("private card leaked in animation")
			}
		})
	}
}

func TestSplendorPostExtraTokenBeforeHandLimit(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{TradingPosts: true})
	s.Splendor.Players[0].TradingPosts = []int{GemPostExtraToken}
	s.Splendor.Players[0].Tokens = []int{0, 2, 2, 2, 2, 1}
	gemExpansionApply(t, s, Action{Type: "take", Tokens: []int{2, 0, 0, 0, 0, 0}})
	if s.Phase != "gem_token" {
		t.Fatal("discard occurred before extra token")
	}
	gemExpansionReject(t, s, Action{Type: "gem_token", Color: 0})
	gemExpansionReject(t, s, Action{Type: "gem_token", Color: 5})
	gemExpansionApply(t, s, Action{Type: "gem_token", Color: 1})
	if s.Phase != "discard" || sum(s.Splendor.Players[0].Tokens) != 12 {
		t.Fatal("extra token not counted toward limit")
	}
}

func TestSplendorPurchaseTokenBeforeRefillCanTakePaidToken(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{TradingPosts: true})
	s.Splendor.Players[0].TradingPosts = []int{GemPostPurchaseToken}
	s.Splendor.Players[0].Tokens = []int{1, 0, 0, 0, 0, 0}
	s.Splendor.Bank = []int{0, 0, 0, 0, 0, 5}
	s.Splendor.Market[0][0] = Card{ID: 501, Tier: 1, Color: 0, Cost: []int{1, 0, 0, 0, 0}}
	next := s.Splendor.Decks[0][0].ID
	gemExpansionApply(t, s, Action{Type: "buy", Card: 501})
	if s.Phase != "gem_token" || s.Splendor.Market[0][0].ID != 0 || s.Splendor.Decks[0][0].ID != next {
		t.Fatal("replacement revealed too soon")
	}
	gemExpansionReject(t, s, Action{Type: "buy", Card: 0})
	gemExpansionApply(t, s, Action{Type: "gem_token", Color: 0})
	if s.Splendor.Players[0].Tokens[0] != 1 || s.Splendor.Market[0][0].ID != next || s.Turn != 1 {
		t.Fatal("paid token or refill incorrect")
	}
}

func TestSplendorStrongholdOccupationAndProtectedStacks(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{Strongholds: true})
	id := s.Splendor.Market[0][0].ID
	s.Splendor.Strongholds[id] = GemStronghold{Player: 1, Count: 2}
	s.Splendor.Players[0].Bonus = []int{9, 9, 9, 9, 9}
	gemExpansionReject(t, s, Action{Type: "buy", Card: id})
	gemExpansionReject(t, s, Action{Type: "reserve", Card: id})
	s.Phase = "gem_stronghold"
	s.Splendor.Effects = []GemEffect{{Kind: "stronghold"}}
	gemExpansionReject(t, s, Action{Type: "gem_stronghold", Choice: "remove", Card: id})
	gemExpansionReject(t, s, Action{Type: "gem_stronghold", Choice: "place", Card: id})
	s.Splendor.Strongholds[id] = GemStronghold{Player: 1, Count: 1}
	gemExpansionApply(t, s, Action{Type: "gem_stronghold", Choice: "remove", Card: id})
	if _, ok := s.Splendor.Strongholds[id]; ok {
		t.Fatal("single enemy stronghold not removed")
	}
}

func TestSplendorStrongholdConquestTriggersAndDelayedRefill(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{TradingPosts: true, Strongholds: true})
	s.Splendor.Nobles = nil
	s.Splendor.Players[0].TradingPosts = []int{GemPostPurchaseToken}
	s.Splendor.Players[0].Bonus = []int{9, 9, 9, 9, 9}
	a, b, c := s.Splendor.Market[0][0].ID, s.Splendor.Market[0][1].ID, s.Splendor.Market[0][2].ID
	s.Splendor.Strongholds[b] = GemStronghold{Player: 0, Count: 2}
	deckSize := len(s.Splendor.Decks[0])
	gemExpansionApply(t, s, Action{Type: "buy", Card: a})
	gemExpansionApply(t, s, Action{Type: "gem_token", Color: 0})
	if s.Phase != "gem_stronghold" {
		t.Fatal("missing purchase stronghold effect")
	}
	gemExpansionApply(t, s, Action{Type: "gem_stronghold", Choice: "place", Card: b})
	if s.Phase != "gem_conquest" || len(s.Splendor.Decks[0]) != deckSize {
		t.Fatal("conquest not offered before replacement")
	}
	gemExpansionReject(t, s, Action{Type: "gem_conquest", Card: c})
	gemExpansionApply(t, s, Action{Type: "gem_conquest", Card: b})
	if s.Phase != "gem_token" {
		t.Fatal("conquest did not trigger trading post")
	}
	gemExpansionApply(t, s, Action{Type: "gem_token", Color: 1})
	gemExpansionApply(t, s, Action{Type: "gem_stronghold", Choice: "place", Card: c})
	if s.Turn != 1 || len(s.Splendor.Decks[0]) != deckSize-2 || len(s.Splendor.Players[0].Cards) != 2 {
		t.Fatal("conquest resolution incorrect")
	}
	if s.Splendor.Strongholds[c].Count != 1 {
		t.Fatal("conquest did not return and place strongholds")
	}
}

func TestSplendorExpansionEliminationCleansPendingChoices(t *testing.T) {
	s := gemExpandedTest(t, SplendorOptions{TradingPosts: true, Strongholds: true})
	s.Splendor.Players[0].TradingPosts = []int{GemPostBlindReserve}
	id := s.Splendor.Market[0][0].ID
	s.Splendor.Strongholds[id] = GemStronghold{Player: 0, Count: 3}
	n := len(s.Splendor.Decks[0])
	gemExpansionApply(t, s, Action{Type: "reserve", Tier: 1})
	if err := s.EliminateSplendor(0); err != nil {
		t.Fatal(err)
	}
	if len(s.Splendor.Decks[0]) != n || len(s.Splendor.Strongholds) != 0 || len(s.Splendor.ReserveChoice) != 0 || s.Turn != 1 {
		t.Fatal("elimination lost cards or left stale effects")
	}
}

func TestSplendorExpansionBotsFinishAndConserveTokens(t *testing.T) {
	for _, o := range []SplendorOptions{{TradingPosts: true}, {Strongholds: true}, {TradingPosts: true, Strongholds: true}} {
		s := gemExpandedTest(t, o)
		for step := 0; step < 2400 && !s.Finished; step++ {
			a, err := s.BotAction(s.Turn)
			if err != nil {
				t.Fatalf("options %+v step %d phase %s: %v", o, step, s.Phase, err)
			}
			gemExpansionApply(t, s, a)
			for color := 0; color < 6; color++ {
				total := s.Splendor.Bank[color]
				for _, p := range s.Splendor.Players {
					total += p.Tokens[color]
				}
				if total != 5 {
					t.Fatalf("token %d conservation: %d", color, total)
				}
			}
		}
		if !s.Finished {
			t.Fatalf("bots failed to finish %+v", o)
		}
	}
}
