package monopoly_test

// Failure scenarios fixed before movement code: wrong GO salary or third-double
// movement; double turn lost across purchase; mortgaged rent collected; monopoly
// bonus lost when a sibling is mortgaged; utility rent reuses movement dice;
// prison failure moves early; prison doubles grant an extra turn; third prison
// failure rerolls or loses the movement when debt is resumed after restart.

import (
	"fmt"
	"testing"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
)

// These fixed secret RNG fixtures yield 1+2, 3+3 and 6+6. They are server-only
// test data, not action parameters or production configuration.
func dice(s *monopoly.State, key byte) { s.Random = monopoly.Random{}; s.Random.Key[31] = key }
func player(r map[string]any, id int) map[string]any {
	return data(r)["players"].([]any)[id].(map[string]any)
}

func TestMonopolyRentAndLandingHTTP(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		position, land, cash0, cash1 int
		setup                        func(*monopoly.State)
		phase                        string
	}{
		{"go_salary", 38, 1, 1700, 1500, nil, "purchase"},
		{"monopoly_rent", 3, 6, 1488, 1512, func(s *monopoly.State) {
			for _, i := range []int{6, 8, 9} {
				s.Properties[i].Owner = 1
			}
			s.Properties[8].Mortgaged = true
		}, "end"},
		{"mortgaged_no_rent", 3, 6, 1500, 1500, func(s *monopoly.State) { s.Properties[6].Owner = 1; s.Properties[6].Mortgaged = true }, "end"},
		{"station_count", 2, 5, 1450, 1550, func(s *monopoly.State) {
			s.Properties[5].Owner = 1
			s.Properties[15].Owner = 1
			s.Properties[15].Mortgaged = true
		}, "end"},
		{"utility_new_dice", 9, 12, 1420, 1580, func(s *monopoly.State) { s.Properties[12].Owner = 1; s.Properties[28].Owner = 1 }, "end"},
		{"tax", 1, 4, 1300, 1500, nil, "end"},
		{"free_parking", 17, 20, 1500, 1500, nil, "end"},
		{"go_to_jail", 27, 10, 1500, 1500, nil, "end"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) {
				s.Phase = "roll"
				s.Players[0].Position = tc.position
				dice(s, 5)
				if tc.setup != nil {
					tc.setup(s)
				}
			})
			tb.people[0].act(map[string]any{"type": "roll"}, 200)
			r := tb.people[0].room()
			if money(r, 0) != tc.cash0 || money(r, 1) != tc.cash1 || player(r, 0)["position"] != float64(tc.land) || data(r)["phase"] != tc.phase {
				t.Fatal("landing mismatch", r)
			}
			if tc.name == "utility_new_dice" {
				d := data(r)["utilityDice"].([]any)
				if d[0] != float64(6) || d[1] != float64(2) {
					t.Fatal("utility reused move dice", d)
				}
			}
			if tc.name == "go_to_jail" && player(r, 0)["jailed"] != true {
				t.Fatal("not jailed")
			}
			tb.record("landing settled")
		})
	}
}

func TestMonopolyDoublesHTTP(t *testing.T) {
	for _, third := range []bool{false, true} {
		t.Run(fmt.Sprint(third), func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) {
				s.Phase = "roll"
				if third {
					s.Players[0].Position = 28
					s.Doubles = 2
					dice(s, 18)
				} else {
					dice(s, 0)
				}
			})
			a := tb.people[0]
			a.act(map[string]any{"type": "roll"}, 200)
			if third {
				r := a.room()
				if player(r, 0)["position"] != float64(10) || player(r, 0)["jailed"] != true || money(r, 0) != 1500 || data(r)["phase"] != "end" {
					t.Fatal("third doubles moved or paid GO", r)
				}
			} else {
				if data(a.room())["phase"] != "purchase" {
					t.Fatal("double roll landing missing")
				}
				tb.restart()
				a.act(map[string]any{"type": "buy"}, 200)
				if data(a.room())["phase"] != "roll" || data(a.room())["turn"] != float64(0) {
					t.Fatal("lost extra roll")
				}
			}
			tb.record("double resolved")
		})
	}
}

func TestMonopolyJailHTTP(t *testing.T) {
	for _, tc := range []struct {
		name           string
		key            byte
		turns          int
		phase          string
		position, cash int
	}{
		{"failed_attempt", 5, 0, "end", 10, 1500},
		{"double_release", 0, 0, "purchase", 16, 1500},
		{"third_failed", 5, 2, "purchase", 13, 1450},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) {
				s.Phase = "roll"
				s.Players[0].Position = 10
				s.Players[0].Jailed = true
				s.Players[0].JailTurns = tc.turns
				dice(s, tc.key)
			})
			a := tb.people[0]
			a.act(map[string]any{"type": "roll"}, 200)
			r := a.room()
			if data(r)["phase"] != tc.phase || player(r, 0)["position"] != float64(tc.position) || money(r, 0) != tc.cash {
				t.Fatal("prison state mismatch", r)
			}
			if tc.phase == "purchase" {
				a.act(map[string]any{"type": "buy"}, 200)
				if data(a.room())["phase"] != "end" {
					t.Fatal("jail doubles granted extra roll")
				}
			}
			tb.record("prison resolved")
		})
	}
}

func TestMonopolyJailDebtResumeHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "roll"
		s.Players[0].Position = 10
		s.Players[0].Jailed = true
		s.Players[0].JailTurns = 2
		s.Players[0].Cash = 20
		s.Properties[5].Owner = 0
		dice(s, 5)
	})
	a := tb.people[0]
	a.act(map[string]any{"type": "roll"}, 200)
	if data(a.room())["phase"] != "debt" || player(a.room(), 0)["position"] != float64(10) {
		t.Fatal("moved before paying jail fee")
	}
	tb.restart()
	a.act(map[string]any{"type": "mortgage", "property": 5}, 200)
	r := a.room()
	if money(r, 0) != 70 || player(r, 0)["position"] != float64(13) || data(r)["phase"] != "purchase" || player(r, 0)["jailed"] == true {
		t.Fatal("deferred prison movement lost", r)
	}
	tb.record("prison debt resumed original dice")
}

func TestMonopolyJailPayAndCardHTTP(t *testing.T) {
	for _, card := range []bool{false, true} {
		t.Run(fmt.Sprint(card), func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) {
				s.Phase = "roll"
				s.Players[0].Position = 10
				s.Players[0].Jailed = true
				dice(s, 0)
				if card {
					s.Players[0].JailCards = []string{"chance"}
					for i, c := range s.Decks[0] {
						if c == 0 {
							s.Decks[0] = append(s.Decks[0][:i], s.Decks[0][i+1:]...)
							break
						}
					}
				}
			})
			a := tb.people[0]
			kind := "jail_pay"
			cash := 1450
			if card {
				kind = "jail_card"
				cash = 1500
			}
			a.act(map[string]any{"type": kind, "choice": "chance"}, 200)
			if money(a.room(), 0) != cash || player(a.room(), 0)["jailed"] == true {
				t.Fatal("jail exit did not settle")
			}
			a.act(map[string]any{"type": "roll"}, 200)
			a.act(map[string]any{"type": "buy"}, 200)
			if data(a.room())["phase"] != "roll" {
				t.Fatal("paid/card release wrongly suppresses doubles")
			}
			tb.record("voluntary jail release")
		})
	}
}
