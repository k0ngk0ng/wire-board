package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSplendorNobleAcquisitionAnimation(t *testing.T) {
	for _, choice := range []bool{false, true} {
		t.Run(map[bool]string{false: "automatic", true: "chosen"}[choice], func(t *testing.T) {
			s := mustGame(t, "splendor", 2)
			noble := Noble{ID: 1, Cost: []int{1, 0, 0, 0, 0}}
			s.Splendor.Nobles = []Noble{noble}
			if choice {
				s.Splendor.Nobles = append(s.Splendor.Nobles, Noble{ID: 2, Cost: []int{1, 0, 0, 0, 0}})
			}
			s.Splendor.Market[0][0] = Card{ID: 999, Tier: 1, Color: 0, Cost: []int{0, 0, 0, 0, 0}}
			apply(t, s, Action{Type: "buy", Card: 999})
			if choice {
				if s.Phase != "noble" || s.Splendor.CardEventID != 1 {
					t.Fatal("animated noble before the player selected one")
				}
				if err := s.Apply(s.Turn, Action{Type: "noble", Noble: 999}); err == nil || s.Splendor.CardEventID != 1 {
					t.Fatal("invalid noble choice animated")
				}
				apply(t, s, Action{Type: "noble", Noble: noble.ID})
			}
			g := s.Splendor
			if len(g.CardEvents) != 2 || g.CardEvents[0].Action != "buy" {
				t.Fatal("purchase and noble animations lost order", g.CardEvents)
			}
			e := g.CardEvents[1]
			if e.ID != 2 || e.Player != 0 || e.Action != "noble" || e.Source != "nobles" || e.Card != nil || e.Noble == nil || !reflect.DeepEqual(*e.Noble, noble) || g.Players[0].Score != 3 || len(g.Players[0].Nobles) != 1 || s.Turn != 1 {
				t.Fatal("noble animation or owner incorrect", e)
			}
			for _, viewer := range []int{0, 1, -1} {
				view := s.View(viewer)["splendor"].(map[string]any)
				raw, _ := json.Marshal(view["cardEvents"])
				var events []SplendorCardEvent
				if err := json.Unmarshal(raw, &events); err != nil || !reflect.DeepEqual(events, g.CardEvents) {
					t.Fatal("noble animation differs for player, opponent or spectator")
				}
			}
			if !reflect.DeepEqual(clone(*s).Splendor.CardEvents, g.CardEvents) {
				t.Fatal("noble event lost after restore")
			}
		})
	}
}

func TestSplendorTokenAnimationEventsFollowActualTransfers(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	take := []int{2, 0, 0, 0, 0, 0}
	apply(t, s, Action{Type: "take", Tokens: take})
	take[0] = 99 // The journal must not retain the caller's mutable slice.
	apply(t, s, Action{Type: "take", Tokens: []int{0, 1, 1, 1, 0, 0}})
	apply(t, s, Action{Type: "reserve", Tier: 1})
	want := []SplendorTokenEvent{
		{ID: 1, Player: 0, Action: "take", Tokens: []int{2, 0, 0, 0, 0, 0}},
		{ID: 2, Player: 1, Action: "take", Tokens: []int{0, 1, 1, 1, 0, 0}},
		{ID: 3, Player: 0, Action: "gold", Tokens: []int{0, 0, 0, 0, 0, 1}},
	}
	if !reflect.DeepEqual(s.Splendor.TokenEvents, want) {
		t.Fatal("wrong transfer events", s.Splendor.TokenEvents)
	}
	// An unaffordable or invalid action must never animate a transfer.
	if err := s.Apply(s.Turn, Action{Type: "take", Tokens: []int{0, 0, 0, 0, 0, 1}}); err == nil {
		t.Fatal("gold taken directly")
	}
	if !reflect.DeepEqual(s.Splendor.TokenEvents, want) {
		t.Fatal("rejected action produced animation")
	}
	// A reservation without available gold still moves the card, not a token.
	s.Splendor.Bank[5] = 0
	apply(t, s, Action{Type: "reserve", Tier: 2})
	if s.Splendor.TokenEventID != 3 {
		t.Fatal("animated nonexistent gold")
	}
	for _, viewer := range []int{0, 1, -1} {
		v := s.View(viewer)["splendor"].(map[string]any)
		raw, _ := json.Marshal(v["tokenEvents"])
		var events []SplendorTokenEvent
		if err := json.Unmarshal(raw, &events); err != nil || !reflect.DeepEqual(events, want) {
			t.Fatal("player or spectator sees different transfers", viewer)
		}
	}
	if !reflect.DeepEqual(clone(*s).Splendor.TokenEvents, want) {
		t.Fatal("transfer journal lost on restart")
	}
}

func TestSplendorTokenPaymentAndReturnAnimations(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	p := &s.Splendor.Players[0]
	p.Tokens = []int{1, 1, 0, 0, 0, 2}
	card := Card{ID: 999, Tier: 1, Color: 0, Cost: []int{1, 1, 0, 0, 0}}
	s.Splendor.Market[0][0] = card
	apply(t, s, Action{Type: "buy", Card: 999, Tokens: []int{0, 0, 0, 0, 0, 2}})
	e := s.Splendor.TokenEvents[0]
	if e.Player != 0 || e.Action != "pay" || !reflect.DeepEqual(e.Tokens, []int{0, 0, 0, 0, 0, 2}) {
		t.Fatal("animation ignored selected gold payment", e)
	}
	s.Splendor.Players[1].Tokens = []int{3, 3, 3, 2, 0, 1}
	s.Phase = "discard"
	bank := append([]int{}, s.Splendor.Bank...)
	apply(t, s, Action{Type: "discard", Tokens: []int{1, 0, 0, 0, 0, 1}})
	e = s.Splendor.TokenEvents[1]
	if e.Player != 1 || e.Action != "return" || !reflect.DeepEqual(e.Tokens, []int{1, 0, 0, 0, 0, 1}) || s.Splendor.Bank[0] != bank[0]+1 || s.Splendor.Bank[5] != bank[5]+1 {
		t.Fatal("return animation differs from bank transfer", e)
	}
	// Fully discounted purchases do not create empty payment animations.
	s.Splendor.Market[0][0] = card
	p.Bonus = []int{10, 10, 10, 10, 10}
	apply(t, s, Action{Type: "buy", Card: 999})
	if s.Splendor.TokenEventID != 2 {
		t.Fatal("free purchase animated payment")
	}
}

func TestSplendorCardEventsSourcesAndPrivacy(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	market := s.Splendor.Market[1][2]
	apply(t, s, Action{Type: "reserve", Card: market.ID})
	e := s.Splendor.CardEvents[0]
	if e.ID != 1 || e.Player != 0 || e.Source != "market" || e.Slot != 2 || e.Tier != 2 || e.Action != "reserve" || !reflect.DeepEqual(*e.Card, market) {
		t.Fatal("wrong public reservation event", e)
	}
	apply(t, s, Action{Type: "reserve", Tier: 3})
	for _, viewer := range []int{-1, 0, 1} {
		v := s.View(viewer)["splendor"].(map[string]any)
		blind := v["cardEvents"].([]any)[1].(map[string]any)
		if _, exposed := blind["card"]; exposed || blind["source"] != "deck" || blind["tier"] != float64(3) || blind["player"] != float64(1) {
			t.Fatal("blind event leaked or wrong source", blind)
		}
	}
	s.Splendor.Players[0].Bonus = []int{10, 10, 10, 10, 10}
	apply(t, s, Action{Type: "buy", Card: market.ID})
	e = s.Splendor.CardEvents[2]
	if e.Source != "reserved" || e.Action != "buy" || e.Player != 0 || e.Card.ID != market.ID {
		t.Fatal("reserved purchase did not reveal acquired card", e)
	}
	before := clone(*s)
	if err := s.Apply(s.Turn, Action{Type: "buy", Card: -1}); err == nil || !reflect.DeepEqual(before.Splendor.CardEvents, s.Splendor.CardEvents) {
		t.Fatal("rejected action changed event stream")
	}
	if !reflect.DeepEqual(clone(*s).Splendor.CardEvents, s.Splendor.CardEvents) {
		t.Fatal("animation history lost on restore")
	}
}

func TestSplendorCardEventHistoryIsBounded(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	s.Splendor.Nobles = nil
	for i := 0; i < 20; i++ {
		card := Card{ID: 1000 + i, Tier: 1, Color: 0, Cost: []int{0, 0, 0, 0, 0}}
		s.Splendor.Market[0][0] = card
		apply(t, s, Action{Type: "buy", Card: card.ID})
	}
	g := s.Splendor
	if g.CardEventID != 20 || len(g.CardEvents) != 12 || g.CardEvents[0].ID != 9 || g.CardEvents[11].ID != 20 {
		t.Fatal("history is unbounded or loses sequence IDs")
	}
}

func TestSplendorRandomOpeningSeat(t *testing.T) {
	for n := 2; n <= 4; n++ {
		seen := make(map[int]bool)
		for i := 0; i < 128; i++ {
			s, err := New("splendor", n)
			if err != nil || s.Turn < 0 || s.Turn >= n || s.Turn != s.Splendor.StartPlayer || s.Round != 1 {
				t.Fatal("invalid opening turn", s, err)
			}
			seen[s.Turn] = true
		}
		if len(seen) != n {
			t.Fatal("opening seat fixed or excludes a seat", n, seen)
		}
	}
}

func TestSplendorEveryStarterGetsEqualFinalTurns(t *testing.T) {
	for n := 2; n <= 4; n++ {
		for start := 0; start < n; start++ {
			for winner := 0; winner < n; winner++ {
				s := mustGame(t, "splendor", n)
				s.Turn, s.Splendor.StartPlayer = start, start
				turns := make([]int, n)
				for i := 0; i < n; i++ {
					actor := s.Turn
					turns[actor]++
					if actor == winner {
						s.Splendor.Players[actor].Score = 15
					}
					s.gemNext()
					if s.Finished != (i == n-1) {
						t.Fatal("final round ended at wrong seat", n, start, winner, actor)
					}
				}
				if s.Round != 2 || !reflect.DeepEqual(s.Winners, []int{winner}) {
					t.Fatal("incorrect final round", s)
				}
				for _, count := range turns {
					if count != 1 {
						t.Fatal("unequal final turns", turns)
					}
				}
			}
		}
	}
}

func TestSplendorStarterSurvivesRestoreAndElimination(t *testing.T) {
	s := mustGame(t, "splendor", 4)
	s.Turn, s.Splendor.StartPlayer = 2, 2
	s = func() *State { restored := clone(*s); return &restored }()
	if s.Splendor.StartPlayer != 2 {
		t.Fatal("starter lost on restore")
	}
	if err := s.EliminateSplendor(2); err != nil {
		t.Fatal(err)
	}
	s.Splendor.Players[3].Score = 15
	s.gemNext()
	if s.Finished || s.Round != 1 || s.Turn != 0 {
		t.Fatal("wrapped at host instead of starter")
	}
	s.gemNext()
	if s.Finished || s.Turn != 1 {
		t.Fatal("last player lost their turn")
	}
	s.gemNext()
	if !s.Finished || s.Splendor.StartPlayer != 2 {
		t.Fatal("eliminated starter broke round boundary")
	}
	var legacy Splendor
	if err := json.Unmarshal([]byte(`{"bank":[4,4,4,4,4,5]}`), &legacy); err != nil || legacy.StartPlayer != 0 {
		t.Fatal("legacy games changed starter")
	}
}
