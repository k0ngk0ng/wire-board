package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestBotsFinishBothGames(t *testing.T) {
	for _, spec := range []struct {
		kind string
		n    int
	}{{"splendor", 2}, {"splendor", 4}, {"rail", 2}, {"rail", 5}} {
		t.Run(fmt.Sprintf("%s-%d", spec.kind, spec.n), func(t *testing.T) {
			for trial := 0; trial < 8; trial++ {
				s := mustGame(t, spec.kind, spec.n)
				for step := 0; step < 1400 && !s.Finished; step++ {
					player := s.Turn
					if s.Rail != nil && s.Rail.Setup {
						for i, pending := range s.Rail.SetupPending {
							if len(pending) > 0 {
								player = i
								break
							}
						}
					}
					before, _ := json.Marshal(s)
					a, err := s.BotAction(player)
					if err != nil {
						t.Fatalf("trial %d step %d phase %s: %v", trial, step, s.Phase, err)
					}
					after, _ := json.Marshal(s)
					if string(before) != string(after) {
						t.Fatal("thinking changed live state")
					}
					if err = s.Apply(player, a); err != nil {
						t.Fatalf("illegal bot action: %+v: %v", a, err)
					}
					if s.Rail != nil {
						railInvariant(t, s)
					} else {
						gemInvariant(t, s)
					}
				}
				if !s.Finished || len(s.Winners) == 0 {
					t.Fatalf("bot match did not finish: trial %d round %d", trial, s.Round)
				}
			}
		})
	}
}

func TestBotsDoNotUseOtherPlayersSecretsOrDeckOrder(t *testing.T) {
	for _, kind := range []string{"rail", "splendor"} {
		s := mustGame(t, kind, 2)
		if s.Rail != nil {
			s.AutoChooseRailSetup()
		}
		changed := clone(*s)
		if g := changed.Rail; g != nil {
			for i, j := 0, len(g.Deck)-1; i < j; i, j = i+1, j-1 {
				g.Deck[i], g.Deck[j] = g.Deck[j], g.Deck[i]
			}
			for i, j := 0, len(g.TicketDeck)-1; i < j; i, j = i+1, j-1 {
				g.TicketDeck[i], g.TicketDeck[j] = g.TicketDeck[j], g.TicketDeck[i]
			}
			g.Players[1].Hand[0], g.Players[1].Hand[1] = g.Players[1].Hand[1], g.Players[1].Hand[0]
			g.Players[1].Tickets = []Ticket{{A: 5, B: 12, Points: 20}}
		} else {
			g := changed.Splendor
			for _, deck := range g.Decks {
				for i, j := 0, len(deck)-1; i < j; i, j = i+1, j-1 {
					deck[i], deck[j] = deck[j], deck[i]
				}
			}
			g.Players[1].Reserved = []Card{{ID: 999, Tier: 3, Points: 5, Cost: []int{7, 0, 0, 0, 0}}}
		}
		a, e := s.BotAction(0)
		b, f := changed.BotAction(0)
		if e != nil || f != nil || !reflect.DeepEqual(a, b) {
			t.Fatal(kind, "hidden data changed bot decision", a, b, e, f)
		}
	}
}

func TestBotHandlesSubstepsAndInvalidSeats(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	if _, err := s.BotAction(1); err == nil {
		t.Fatal("bot acted out of turn")
	}
	s.Splendor.Players[0].Tokens = []int{2, 2, 2, 2, 2, 1}
	s.Phase = "discard"
	a, err := s.BotAction(0)
	if err != nil || a.Type != "discard" || sum(a.Tokens) != 1 {
		t.Fatal(a, err)
	}
	s.Phase = "noble"
	s.Splendor.Players[0].Bonus = []int{5, 5, 5, 5, 5}
	a, err = s.BotAction(0)
	if err != nil || a.Type != "noble" {
		t.Fatal(a, err)
	}
	s.Finished = true
	if _, err = s.BotAction(0); err == nil {
		t.Fatal("bot acted after finish")
	}
	rail := playingRail(t, 2)
	rail.Rail.Deck = nil
	rail.Rail.Discard = nil
	rail.Rail.Face = []int{-1, 2, -1, 8, -1}
	rail.Rail.Drawn = 1
	rail.Phase = "draw"
	a, err = rail.BotAction(0)
	if err != nil || a.Type != "draw" || a.Slot != 1 {
		t.Fatal("invalid second draw", a, err)
	}
}
