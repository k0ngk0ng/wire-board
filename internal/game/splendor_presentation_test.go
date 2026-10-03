package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

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
