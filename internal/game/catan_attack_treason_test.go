package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func attackTreasonFixture(t *testing.T, n int, kind string) *State {
	t.Helper()
	s := newAttackState(t, n, false)
	a := s.Catan.Attack
	clear(a.Barbarians)
	stock := 0
	switch kind {
	case "empty_stock":
	case "single_supply":
		stock = 1
	case "single_board":
		a.Barbarians[a.Map.Coast[0]] = 2
	case "single_board_with_supply":
		a.Barbarians[a.Map.Coast[0]] = 1
		stock = 1
	case "two_board":
		a.Barbarians[a.Map.Coast[0]], a.Barbarians[a.Map.Coast[1]] = 1, 1
	case "no_destination", "one_destination", "two_destinations":
		for _, id := range a.Map.Coast {
			a.Barbarians[id] = 3
		}
		if kind != "no_destination" {
			a.Barbarians[a.Map.Coast[0]] = 2
		}
		if kind == "two_destinations" {
			a.Barbarians[a.Map.Coast[1]] = 2
		}
		stock = a.Map.Barbarians - sum(a.Barbarians)
	default:
		t.Fatal("unknown treason fixture")
	}
	for i, count := 0, a.Map.Barbarians-stock-sum(a.Barbarians); i < count; i++ {
		a.Prisoners[i%n]++
	}
	attackCoins(s, (s.Turn+1)%n, a.GoldBank)
	s.catanScores()
	assertAttackRestored(t, s)
	return s
}

func TestCatanAttackTreasonMaximumFeasibleEffect(t *testing.T) {
	cases := map[string]int{"empty_stock": 0, "single_supply": 1, "single_board": 1, "single_board_with_supply": 2, "two_board": 2, "no_destination": 0, "one_destination": 1, "two_destinations": 2}
	for _, n := range []int{3, 6} {
		for kind, count := range cases {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				s := attackTreasonFixture(t, n, kind)
				a, p := s.Catan.Attack, s.Turn
				beforeCounts, prisoners := slices.Clone(a.Barbarians), slices.Clone(a.Prisoners)
				beforeStock := a.supply()
				plans := a.treasonPlans()
				if len(plans[0].Sources) != count {
					t.Fatal("wrong maximum", plans)
				}
				buyAttackCard(t, s, "treason")
				if count == 0 {
					a = s.Catan.Attack
					if s.Phase != "catan_turn" || a.Pending != nil || len(a.Deck) != 25 || len(a.Discard) != 1 || a.Discard[0] != "treason" || a.Gold[p] != 2 || a.GoldIssued != 2 || !slices.Equal(a.Barbarians, beforeCounts) {
						t.Fatal("no-move card must pay exactly once, discard, and not redraw")
					}
					attackReject(t, s, p, Action{Type: "catan_attack_card", Choice: "treason", Prompt: 1})
					assertAttackRestored(t, s)
					return
				}
				choice, err := s.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				if len(choice.Give) != count || len(choice.Take) != count {
					t.Fatal("bot skipped required movement", choice)
				}
				bad := choice
				bad.Give, bad.Take = choice.Give[:count-1], choice.Take[:count-1]
				attackReject(t, s, p, bad)
				if count == 1 {
					bad = choice
					bad.Give = append(slices.Clone(choice.Give), choice.Give[0])
					bad.Take = append(slices.Clone(choice.Take), choice.Take[0])
					attackReject(t, s, p, bad)
				}
				for viewer := -1; viewer < n; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)["attack"].(map[string]any)
					if v["treasonRule"] != "as-much-as-possible" || v["deck"] != nil {
						t.Fatal("rule/deck visibility")
					}
					if viewer == p {
						if v["treasonCount"] != count {
							t.Fatal("wrong UI count")
						}
					} else if v["treasonPlans"] != nil || v["treasonCount"] != nil {
						t.Fatal("opponent controls leaked")
					}
				}
				// Hidden future deck order cannot affect plans or the bot.
				other := clone(*s)
				slices.Reverse(other.Catan.Attack.Deck)
				if !reflect.DeepEqual(s.View(p), other.View(p)) {
					t.Fatal("future deck leaked")
				}
				second, err := other.BotAction(p)
				if err != nil || !reflect.DeepEqual(choice, second) {
					t.Fatal("bot inspected hidden deck", err)
				}
				if err := s.Apply(p, choice); err != nil {
					t.Fatal(err)
				}
				used := 0
				for _, id := range choice.Give {
					if id < 0 {
						used++
					} else {
						beforeCounts[id]--
					}
				}
				for _, id := range choice.Take {
					beforeCounts[id]++
				}
				a = s.Catan.Attack
				if !slices.Equal(a.Barbarians, beforeCounts) || !slices.Equal(a.Prisoners, prisoners) || a.supply() != beforeStock-used || a.Gold[p] != 2 || a.GoldIssued != 2 || len(a.Discard) != 1 || len(a.Deck) != 25 || s.Phase != "catan_turn" {
					t.Fatal("partial treason stock, reward, discard or phase")
				}
				assertAttackRestored(t, s)
				attackReject(t, s, p, choice)
			})
		}
	}
}

func TestCatanAttackTreasonSourcesAlwaysCompleteMaximum(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"one_destination", "two_destinations"} {
			s := attackTreasonFixture(t, n, kind)
			buyAttackCard(t, s, "treason")
			a, p := s.Catan.Attack, s.Turn
			for _, plan := range a.treasonPlans() {
				for _, id := range plan.Sources {
					if id == a.Map.Coast[0] || kind == "two_destinations" && id == a.Map.Coast[1] {
						t.Fatal("advertised source consumes a required destination")
					}
				}
				next := clone(*s)
				choice := Action{Type: "catan_attack_card", Choice: "treason", Prompt: a.Pending.ID, Give: plan.Sources, Take: plan.Destinations[:len(plan.Sources)]}
				if err := next.Apply(p, choice); err != nil {
					t.Fatal("advertised plan cannot finish", err)
				}
				assertAttackRestored(t, &next)
			}
			choice, err := s.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			choice.Give[0] = a.Map.Coast[0]
			attackReject(t, s, p, choice)
		}
	}
}

func TestCatanAttackTreasonShortageDoesNotBlockOtherCards(t *testing.T) {
	for _, kind := range []string{"empty_stock", "no_destination", "single_supply"} {
		for _, card := range []string{"knighthood", "swift_knight"} {
			s := attackTreasonFixture(t, 3, kind)
			buyAttackCard(t, s, card)
			choice, err := s.BotAction(s.Turn)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Apply(s.Turn, choice); err != nil {
				t.Fatal(err)
			}
			if len(s.Catan.Attack.Knights) != 1 {
				t.Fatal("shortage blocked knight recruitment")
			}
			assertAttackRestored(t, s)
		}
	}
}
