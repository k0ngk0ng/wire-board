package game

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func explorerTradeProgressFixture(t *testing.T, n int, secondary bool) *State {
	t.Helper()
	s := explorerCityProductionFixture(t, n)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	if secondary {
		explorerCityFinishAction(t, s)
	}
	for p := range s.Catan.Players {
		for color, count := range s.Catan.Players[p].Resources {
			s.Catan.Bank[color] += count
			s.Catan.Players[p].Resources[color] = 0
		}
	}
	explorerCityRestore(t, s)
	return s
}

func explorerTradeProgressRespond(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	if err := s.catanExplorerCityRespond(p, a); err != nil {
		t.Fatal(err)
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerCityTradeProgressMonopolies(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, false}, {6, true}} {
		for color := range 8 {
			t.Run(fmt.Sprintf("%d/%t/%d", config.n, config.secondary, color), func(t *testing.T) {
				s := explorerTradeProgressFixture(t, config.n, config.secondary)
				actor, card, limit := s.Turn, 14, 2
				if color >= 5 {
					card, limit = 15, 1
				}
				ckProgressGive(t, s, actor, card)
				want := 0
				for p := range s.Catan.Players {
					if p == actor {
						continue
					}
					amount := []int{0, 1, 4, 2, 0, 1}[p]
					explorerDevelopmentGrant(t, s, p, color, amount)
					want += min(amount, limit)
				}
				before := clone(*s.Catan)
				wrong := 5
				if color >= 5 {
					wrong = 0
				}
				for _, invalid := range []int{-1, 8, wrong} {
					explorerCityActionReject(t, s, actor, Action{Type: "catan_progress", Card: card, Color: invalid, Prompt: int(s.Catan.TurnSerial)})
				}
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: card, Color: color})
				if s.Catan.Players[actor].Resources[color] != want || !slices.Equal(s.Catan.Bank, before.Bank) || !reflect.DeepEqual(s.Catan.Explorer.Economy, before.Explorer.Economy) || !reflect.DeepEqual(s.Catan.Explorer.Cargo, before.Explorer.Cargo) {
					t.Fatal("monopoly changed bank, gold, or action phase")
				}
				for p := range s.Catan.Players {
					if p != actor && s.Catan.Players[p].Resources[color] != max(0, before.Players[p].Resources[color]-limit) {
						t.Fatal("wrong per-player monopoly limit")
					}
				}
			})
		}
	}
}

func TestCatanExplorerCityTradeProgressGuildDuesPrivacy(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, true}} {
		for _, count := range []int{0, 1, 3} {
			t.Run(fmt.Sprintf("%d/%d", config.n, count), func(t *testing.T) {
				s := explorerTradeProgressFixture(t, config.n, config.secondary)
				actor, target := s.Turn, (s.Turn+1)%config.n
				s.Catan.CitiesKnights.Players[target].DefenderPoints = 1 // Controlled higher public score.
				s.catanScores()
				explorerDevelopmentGrant(t, s, target, 0, min(1, count))
				explorerDevelopmentGrant(t, s, target, 5, max(0, count-1))
				ckProgressGive(t, s, actor, 11)
				explorerCityActionReject(t, s, actor, Action{Type: "catan_progress", Card: 11, Target: actor, Prompt: int(s.Catan.TurnSerial)})
				explorerCityActionReject(t, s, actor, Action{Type: "catan_progress", Card: 11, Target: (target + 1) % config.n, Prompt: int(s.Catan.TurnSerial)})
				hand, before := slices.Clone(s.Catan.Players[target].Resources), clone(*s.Catan)
				logStart := len(s.Log)
				explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 11, Target: target})
				if count == 0 {
					if s.Phase != "catan_turn" {
						t.Fatal("empty hand stalled card")
					}
					return
				}
				if s.Phase != "catan_guild_dues" || s.CatanPendingActor() != actor {
					t.Fatal("missing private card selection")
				}
				for viewer := -1; viewer < config.n; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)
					pending := v["citiesKnights"].(map[string]any)["pending"].(map[string]any)
					revealed, ok := pending["resources"]
					if ok != (viewer == actor) || ok && !reflect.DeepEqual(revealed, hand) {
						t.Fatal("hand disclosed to wrong viewer")
					}
					for p, raw := range v["players"].([]any) {
						_, visible := raw.(map[string]any)["resources"]
						if visible != (p == viewer) {
							t.Fatal("main hand privacy changed")
						}
					}
				}
				take := make([]int, 8)
				take[0] = 1
				if count > 1 {
					take[5] = 1
				}
				explorerCityReject(t, s, target, Action{Type: "catan_guild_dues", Take: take, Prompt: int(s.Catan.TurnSerial)})
				explorerCityReject(t, s, actor, Action{Type: "catan_guild_dues", Take: take, Prompt: 0})
				explorerCityReject(t, s, actor, Action{Type: "catan_guild_dues", Take: []int{0, 2, 0, 0, 0, 0, 0, 0}, Prompt: int(s.Catan.TurnSerial)})
				explorerCityActionReject(t, s, actor, Action{Type: "catan_end", Prompt: int(s.Catan.TurnSerial)})
				explorerTradeProgressRespond(t, s, actor, Action{Type: "catan_guild_dues", Take: take})
				if !slices.Equal(s.Catan.Players[actor].Resources, take) || !slices.Equal(s.Catan.Bank, before.Bank) || !reflect.DeepEqual(s.Catan.Explorer.Economy, before.Explorer.Economy) || s.Turn != actor || s.Phase != "catan_turn" {
					t.Fatal("guild response failed to resume same action")
				}
				for _, line := range s.Log[logStart:] {
					if strings.Contains(line, "木材") || strings.Contains(line, "纸张") {
						t.Fatal("private selection leaked in log")
					}
				}
				explorerCityReject(t, s, actor, Action{Type: "catan_guild_dues", Take: take, Prompt: int(s.Catan.TurnSerial)})
			})
		}
	}
}

func TestCatanExplorerCityTradeProgressCommercialHarbor(t *testing.T) {
	for _, config := range []struct {
		n         int
		secondary bool
	}{{3, false}, {6, false}, {6, true}} {
		t.Run(fmt.Sprintf("%d/%t", config.n, config.secondary), func(t *testing.T) {
			s := explorerTradeProgressFixture(t, config.n, config.secondary)
			actor, target, empty := s.Turn, (s.Turn+1)%config.n, (s.Turn+2)%config.n
			explorerDevelopmentGrant(t, s, actor, 0, 5)
			explorerDevelopmentGrant(t, s, actor, 1, 1)
			explorerDevelopmentGrant(t, s, target, 5, 2)
			explorerDevelopmentGrant(t, s, target, 6, 1)
			ckProgressGive(t, s, actor, 10, 10)
			offer := Action{Type: "catan_commercial_offer", Card: 0, Target: target, Color: 0, Prompt: int(s.Catan.TurnSerial)}
			explorerCityActionReject(t, s, actor, offer)
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 10})
			for _, bad := range []Action{{Type: "catan_commercial_offer", Card: 1, Target: target}, {Type: "catan_commercial_offer", Target: actor}, {Type: "catan_commercial_offer", Target: -1}, {Type: "catan_commercial_offer", Target: config.n}, {Type: "catan_commercial_offer", Target: target, Color: 5}, {Type: "catan_commercial_offer", Target: target, Color: 0, Choice: "skip"}} {
				bad.Prompt = int(s.Catan.TurnSerial)
				explorerCityActionReject(t, s, actor, bad)
			}
			before := clone(*s.Catan)
			logStart := len(s.Log)
			explorerKnightAct(t, s, offer)
			if s.Phase != "catan_commercial_harbor" || s.CatanPendingActor() != target || !reflect.DeepEqual(s.Catan.Players, before.Players) {
				t.Fatal("offer moved cards before response")
			}
			for viewer := -1; viewer < config.n; viewer++ {
				v := s.View(viewer)["catan"].(map[string]any)["citiesKnights"].(map[string]any)["pending"].(map[string]any)
				_, visible := v["color"]
				if visible != (viewer == actor || viewer == target) {
					t.Fatal("commercial resource revealed outside the exchange")
				}
			}
			for _, p := range []int{actor, empty} {
				explorerCityReject(t, s, p, Action{Type: "catan_commercial_harbor", Color: 6, Prompt: int(s.Catan.TurnSerial)})
			}
			for _, color := range []int{-1, 0, 7, 8} {
				explorerCityReject(t, s, target, Action{Type: "catan_commercial_harbor", Color: color, Prompt: int(s.Catan.TurnSerial)})
			}
			explorerCityActionReject(t, s, actor, Action{Type: "catan_progress", Card: 10, Prompt: int(s.Catan.TurnSerial)})
			explorerTradeProgressRespond(t, s, target, Action{Type: "catan_commercial_harbor", Color: 6})
			for _, line := range s.Log[logStart:] {
				if strings.Contains(line, "木材") || strings.Contains(line, "布料") {
					t.Fatal("commercial card types leaked in public log")
				}
			}
			if s.Catan.Players[actor].Resources[0] != 4 || s.Catan.Players[actor].Resources[6] != 1 || s.Catan.Players[target].Resources[0] != 1 || !slices.Equal(s.Catan.Bank, before.Bank) || !reflect.DeepEqual(s.Catan.Explorer.Economy, before.Explorer.Economy) {
				t.Fatal("mandatory exchange wrong")
			}
			explorerCityActionReject(t, s, actor, offer)
			// Other ordinary construction may occur between offers.
			sites := s.catanExplorerFreeRoadSites(actor)
			if len(sites) == 0 {
				t.Fatal("fixture road missing")
			}
			explorerKnightAct(t, s, Action{Type: "catan_road", Edge: sites[0]})
			before = clone(*s.Catan)
			explorerKnightAct(t, s, Action{Type: "catan_commercial_offer", Target: empty, Color: 0})
			if s.Phase != "catan_turn" || !reflect.DeepEqual(s.Catan.Players, before.Players) {
				t.Fatal("no-commodity offer did not return resource")
			}
			explorerCityActionReject(t, s, actor, Action{Type: "catan_commercial_offer", Target: empty, Color: 0, Prompt: int(s.Catan.TurnSerial)})
			explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 10})
			explorerKnightAct(t, s, Action{Type: "catan_commercial_offer", Card: 1, Target: target, Color: 0})
			explorerTradeProgressRespond(t, s, target, Action{Type: "catan_commercial_harbor", Color: 5})
			if len(s.Catan.CitiesKnights.TradePowers.Harbors) != 2 {
				t.Fatal("second card not independent")
			}
			explorerCityFinishAction(t, s)
			if s.Catan.CitiesKnights.TradePowers != nil {
				t.Fatal("commercial harbor survived handoff")
			}
		})
	}
}

func TestCatanExplorerCityTradeProgressCorruptPending(t *testing.T) {
	s := explorerTradeProgressFixture(t, 3, false)
	explorerDevelopmentGrant(t, s, 0, 0, 2)
	explorerDevelopmentGrant(t, s, 1, 5, 1)
	ckProgressGive(t, s, 0, 10)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 10})
	explorerKnightAct(t, s, Action{Type: "catan_commercial_offer", Target: 1, Color: 0})
	for _, kind := range []string{"owner", "fleets", "missing-powers", "empty-powers", "excess-cards", "duplicate-player", "self-player", "outside-player", "respondent", "target", "same-player", "extra-player", "color", "source-empty", "commodity-empty", "unused-offer", "phase"} {
		t.Run(kind, func(t *testing.T) {
			q := clone(*s)
			g, k := q.Catan, q.Catan.CitiesKnights
			switch kind {
			case "owner":
				k.TradePowers.Player = 1
			case "fleets":
				k.TradePowers.Fleets = []int{-1}
			case "missing-powers":
				k.TradePowers = nil
			case "empty-powers":
				k.TradePowers.Harbors = nil
			case "excess-cards":
				k.TradePowers.Harbors = [][]int{{}, {}, {}}
			case "duplicate-player":
				k.TradePowers.Harbors[0] = []int{2, 2}
			case "self-player":
				k.TradePowers.Harbors[0] = []int{0}
			case "outside-player":
				k.TradePowers.Harbors[0] = []int{3}
			case "respondent":
				k.Pending.Players = []int{-1}
			case "target":
				k.Pending.Target = 3
			case "same-player":
				k.Pending.Target = 1
			case "extra-player":
				k.Pending.Players = []int{1, 2}
			case "color":
				k.Pending.Color = 8
			case "source-empty":
				g.Bank[0] += g.Players[0].Resources[0]
				g.Players[0].Resources[0] = 0
			case "commodity-empty":
				g.Bank[5] += g.Players[1].Resources[5]
				g.Players[1].Resources[5] = 0
			case "unused-offer":
				k.TradePowers.Harbors[0] = []int{1, 2}
			case "phase":
				g.Explorer.Cargo.Turn.Phase = "movement"
			}
			before := clone(q)
			if err := q.validateExplorerCityProduction(); err == nil {
				t.Fatal("corrupt progress save accepted")
			}
			if err := q.catanExplorerCityRespond(1, Action{Type: "catan_commercial_harbor", Color: 5, Prompt: 1}); err == nil || !reflect.DeepEqual(q, before) {
				t.Fatal("corrupt response transferred cards or changed state")
			}
		})
	}
	// Guild Dues must not index a forged target or reveal an equal-score hand.
	s = explorerTradeProgressFixture(t, 3, false)
	s.Catan.CitiesKnights.Players[1].DefenderPoints = 1
	s.catanScores()
	explorerDevelopmentGrant(t, s, 1, 0, 1)
	ckProgressGive(t, s, 0, 11)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 11, Target: 1})
	for _, kind := range []string{"outside", "actor", "equal-score", "empty-hand"} {
		q := clone(*s)
		switch kind {
		case "outside":
			q.Catan.CitiesKnights.Pending.Target = -1
		case "actor":
			q.Catan.CitiesKnights.Pending.Players = []int{2}
		case "equal-score":
			q.Catan.CitiesKnights.Players[1].DefenderPoints = 0
			q.catanScores()
		case "empty-hand":
			q.Catan.Players[1].Resources[0]--
			q.Catan.Bank[0]++
		}
		explorerCityReject(t, &q, 0, Action{Type: "catan_guild_dues", Take: []int{1, 0, 0, 0, 0, 0, 0, 0}, Prompt: 1})
	}
}

func TestCatanExplorerCityTradeProgressEliminatedSkipAndOfferCancel(t *testing.T) {
	s := explorerTradeProgressFixture(t, 3, false)
	explorerDevelopmentGrant(t, s, 0, 0, 1)
	explorerDevelopmentGrant(t, s, 1, 1, 1)
	ckProgressGive(t, s, 0, 10, 11, 14, 15)
	explorerKnightAct(t, s, Action{Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0, 0, 0, 0}})
	for _, card := range []int{10, 11, 14, 15} {
		explorerCityActionReject(t, s, 1, Action{Type: "catan_progress", Card: card, Choice: "skip", Prompt: 1})
		explorerKnightAct(t, s, Action{Type: "catan_progress", Card: card, Choice: "skip"})
		if s.Catan.Trade != nil || s.Catan.CitiesKnights.Pending != nil || s.Catan.CitiesKnights.TradePowers != nil {
			t.Fatal("skipped card applied effect or kept ordinary offer")
		}
	}
	for _, card := range []int{14, 15} {
		s = explorerTradeProgressFixture(t, 3, false)
		color := 0
		if card == 15 {
			color = 5
		}
		explorerDevelopmentGrant(t, s, 1, color, 2)
		s.Catan.Players[1].Eliminated = true
		ckProgressGive(t, s, 0, card)
		explorerKnightAct(t, s, Action{Type: "catan_progress", Card: card, Color: color})
		if s.Catan.Players[1].Resources[color] != 2 || s.Catan.Players[0].Resources[color] != 0 {
			t.Fatal("monopoly took departed hand")
		}
	}
}
