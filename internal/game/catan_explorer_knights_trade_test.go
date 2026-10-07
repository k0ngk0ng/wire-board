package game

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func explorerCityTradeFixture(t *testing.T, n int) *State {
	t.Helper()
	s := explorerCityProductionFixture(t, n)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	// Conserved controlled hands. The offer exercises wood, paper and gold
	// against cloth/coin, which ordinary five-resource bundles cannot express.
	for p, hand := range [][]int{{2, 0, 0, 0, 0, 2, 0, 0}, {0, 0, 0, 0, 0, 0, 2, 1}} {
		for r, want := range hand {
			s.Catan.Bank[r] += s.Catan.Players[p].Resources[r] - want
			s.Catan.Players[p].Resources[r] = want
		}
	}
	explorerCityRestore(t, s)
	return s
}
func explorerCityTradeOffer(s *State) Action {
	return Action{Type: "catan_trade_offer", Prompt: int(s.Catan.TurnSerial), Give: []int{1, 0, 0, 0, 0, 1, 0, 0}, Take: []int{0, 0, 0, 0, 0, 0, 1, 1}, GoldGive: 1}
}
func explorerCityTradeApply(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	if err := s.catanExplorerCityAction(p, a); err != nil {
		t.Fatal(a.Type, err)
	}
	explorerCityRestore(t, s)
}
func TestCatanExplorerCityTradeConsentConservationAndPrivacy(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityTradeFixture(t, n)
			before := clone(*s)
			a := explorerCityTradeOffer(s)
			explorerCityTradeApply(t, s, 0, a)
			id := s.Catan.Trade.ID
			complete := Action{Type: "catan_trade_complete", Prompt: a.Prompt, Offer: id, Target: 1}
			explorerCityActionReject(t, s, 0, complete)
			explorerCityActionReject(t, s, 0, Action{Type: "catan_trade_accept", Prompt: a.Prompt, Offer: id})
			explorerCityActionReject(t, s, 1, Action{Type: "catan_trade_accept", Prompt: a.Prompt, Offer: id + 1})
			explorerCityTradeApply(t, s, 1, Action{Type: "catan_trade_accept", Prompt: a.Prompt, Offer: id})
			if !reflect.DeepEqual(s.Catan.Players, before.Catan.Players) || !reflect.DeepEqual(s.Catan.Explorer.Economy, before.Catan.Explorer.Economy) {
				t.Fatal("acceptance moved cards or gold before confirmation")
			}
			// The publicly offered quantities reveal no other cards or progress.
			ckProgressGive(t, s, 1, 5)
			for _, viewer := range []int{-1, 0, 2} {
				v := s.View(viewer)["catan"].(map[string]any)
				other := v["players"].([]any)[1].(map[string]any)
				if _, exists := other["resources"]; exists {
					t.Fatal("opponent hand leaked with public offer")
				}
				k := v["citiesKnights"].(map[string]any)
				if _, exists := k["players"].([]any)[1].(map[string]any)["progress"]; exists {
					t.Fatal("progress hand leaked")
				}
				if v["trade"] == nil {
					t.Fatal("public offer missing")
				}
			}
			stale := clone(*s)
			stale.Catan.Players[1].Resources[7]--
			stale.Catan.Bank[7]++
			explorerCityActionReject(t, &stale, 0, complete)
			stale = clone(*s)
			stale.Catan.Explorer.Economy.GoldBank += stale.Catan.Explorer.Economy.Gold[0]
			stale.Catan.Explorer.Economy.Gold[0] = 0
			explorerCityActionReject(t, &stale, 0, complete)
			explorerCityTradeApply(t, s, 0, complete)
			g := s.Catan
			for r := range g.Bank {
				if g.Players[0].Resources[r] != before.Catan.Players[0].Resources[r]-a.Give[r]+a.Take[r] || g.Players[1].Resources[r] != before.Catan.Players[1].Resources[r]+a.Give[r]-a.Take[r] {
					t.Fatal("incorrect card transfer", r)
				}
			}
			if !slices.Equal(g.Bank, before.Catan.Bank) || g.Explorer.Economy.GoldBank != before.Catan.Explorer.Economy.GoldBank || g.Explorer.Economy.Gold[0] != before.Catan.Explorer.Economy.Gold[0]-1 || g.Explorer.Economy.Gold[1] != before.Catan.Explorer.Economy.Gold[1]+1 || g.Trade != nil {
				t.Fatal("bank or gold changed incorrectly")
			}
			for _, label := range []string{catanCardName(5), catanCardName(6), catanCardName(7), "金币"} {
				if !strings.Contains(s.Log[len(s.Log)-1], label) {
					t.Fatal("missing mixed trade log", label)
				}
			}
			explorerCityActionReject(t, s, 0, complete)
		})
	}
}

func TestCatanExplorerCityTradeReplaceDeclineAndCancel(t *testing.T) {
	s := explorerCityTradeFixture(t, 3)
	a := explorerCityTradeOffer(s)
	explorerCityTradeApply(t, s, 0, a)
	old := s.Catan.Trade.ID
	explorerCityTradeApply(t, s, 1, Action{Type: "catan_trade_reject", Prompt: a.Prompt, Offer: old})
	if s.Catan.Trade.Responses[1] != -1 {
		t.Fatal("decline missing")
	}
	explorerCityActionReject(t, s, 0, Action{Type: "catan_trade_complete", Prompt: a.Prompt, Offer: old, Target: 1})
	// Reversing the public offer lets the other player pay gold for commodities.
	a.GoldGive, a.GoldTake = 0, 1
	explorerCityTradeApply(t, s, 0, a)
	id := s.Catan.Trade.ID
	if id <= old || s.Catan.Trade.Responses[1] != 0 {
		t.Fatal("replacement kept prior responses")
	}
	explorerCityActionReject(t, s, 1, Action{Type: "catan_trade_accept", Prompt: a.Prompt, Offer: old})
	explorerCityActionReject(t, s, 1, Action{Type: "catan_trade_cancel", Prompt: a.Prompt, Offer: id})
	explorerCityTradeApply(t, s, 1, Action{Type: "catan_trade_accept", Prompt: a.Prompt, Offer: id})
	gold := slices.Clone(s.Catan.Explorer.Economy.Gold)
	explorerCityTradeApply(t, s, 0, Action{Type: "catan_trade_complete", Prompt: a.Prompt, Offer: id, Target: 1})
	if s.Catan.Explorer.Economy.Gold[0] != gold[0]+1 || s.Catan.Explorer.Economy.Gold[1] != gold[1]-1 {
		t.Fatal("player gold-for-commodity exchange blocked or reversed")
	}
	// Fresh affordable offer can be cancelled only by its initiator.
	a.Give, a.Take = a.Take, a.Give
	explorerCityTradeApply(t, s, 0, a)
	explorerCityTradeApply(t, s, 0, Action{Type: "catan_trade_cancel", Prompt: a.Prompt, Offer: s.Catan.Trade.ID})
	if s.Catan.Trade != nil {
		t.Fatal("cancel retained offer")
	}
}

func TestCatanExplorerCityTradeInvalidRequestsAndSavedOffers(t *testing.T) {
	for _, reason := range []string{"short_bundle", "overlap", "empty", "both_gold", "negative_gold", "foreign", "stale", "eliminated", "missing_card", "missing_gold", "progress_in_bundle"} {
		t.Run(reason, func(t *testing.T) {
			s := explorerCityTradeFixture(t, 3)
			a := explorerCityTradeOffer(s)
			p := 0
			switch reason {
			case "short_bundle":
				a.Give = a.Give[:5]
			case "overlap":
				a.Take[5] = 1
			case "empty":
				a.Take = make([]int, 8)
			case "both_gold":
				a.GoldTake = 1
			case "negative_gold":
				a.GoldGive = -1
			case "foreign":
				p = 1
			case "stale":
				a.Prompt = 0
			case "eliminated":
				p = 1
				s.Catan.Players[1].Eliminated = true
				a.Type = "catan_trade_accept"
			case "missing_card":
				a.Give[3] = 1
			case "missing_gold":
				a.GoldGive = 10
			case "progress_in_bundle":
				a.Give = append(a.Give, 1)
			}
			explorerCityActionReject(t, s, p, a)
		})
	}
	base := explorerCityTradeFixture(t, 3)
	explorerCityTradeApply(t, base, 0, explorerCityTradeOffer(base))
	for _, reason := range []string{"responses", "self_accept", "bad_response", "id", "foreign_from", "short_bundle", "overlap", "both_gold", "negative_gold"} {
		t.Run("saved_"+reason, func(t *testing.T) {
			s := clone(*base)
			offer := s.Catan.Trade
			switch reason {
			case "responses":
				offer.Responses = offer.Responses[:1]
			case "self_accept":
				offer.Responses[0] = 1
			case "bad_response":
				offer.Responses[1] = 2
			case "id":
				offer.ID++
			case "foreign_from":
				offer.From = 1
			case "short_bundle":
				offer.Give = offer.Give[:5]
			case "overlap":
				offer.Take[5] = 1
			case "both_gold":
				offer.GoldTake = 1
			case "negative_gold":
				offer.GoldTake = -1
			}
			if err := s.validateExplorerCityProduction(); err == nil {
				t.Fatal("corrupt trade restored")
			}
			explorerCityActionReject(t, &s, 1, Action{Type: "catan_trade_accept", Prompt: 1, Offer: offer.ID})
		})
	}
}

func TestCatanExplorerCityTradePairedAndBuildCancel(t *testing.T) {
	s, v := explorerCityVillageFixture(t, 6)
	a := Action{Type: "catan_trade_offer", Prompt: 1, Give: []int{0, 0, 0, 1, 0, 0, 0, 0}, Take: []int{1, 0, 0, 0, 0, 0, 0, 0}}
	explorerCityTradeApply(t, s, 0, a)
	explorerCityTradeApply(t, s, 0, Action{Type: "catan_city", Prompt: 1, Vertex: v})
	if s.Catan.Trade != nil {
		t.Fatal("build retained old offer")
	}
	explorerCityFinishAction(t, s)
	a.Prompt = int(s.Catan.TurnSerial)
	if s.Turn != 3 {
		t.Fatal("missing secondary player")
	}
	explorerCityActionReject(t, s, 3, a)
	for _, kind := range []string{"catan_trade_accept", "catan_trade_reject", "catan_trade_complete", "catan_trade_cancel"} {
		a.Type = kind
		explorerCityActionReject(t, s, 3, a)
	}
	if s.Catan.Explorer.Economy.Turn.Bought != 0 {
		t.Fatal("player trade touched bank quota")
	}
}
