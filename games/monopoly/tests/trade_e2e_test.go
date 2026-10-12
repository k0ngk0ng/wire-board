package monopoly_test

// Failure scenarios written before implementation: unrelated players see private
// proposals; acceptance from wrong recipient; absent/duplicate/out-of-range
// properties or jail cards; improved groups traded; stale approval after editing;
// overspending one asset in two offers; mortgage fees create half-transactions;
// incoming cash cannot fund simultaneous exchange; trade clears unrelated debt;
// expiry triggers autoplay or resets the turn timer; trades lost after restart.

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
)

func bundle(cash int, properties ...int) map[string]any {
	return map[string]any{"cash": cash, "properties": properties}
}
func offers(c *client) []any { return data(c.room())["offers"].([]any) }
func lastOffer(c *client) float64 {
	v := offers(c)
	return v[len(v)-1].(map[string]any)["id"].(float64)
}
func propose(c *client, target int, give, want map[string]any, status int) {
	c.t.Helper()
	c.act(map[string]any{"type": "trade_offer", "offer": map[string]any{"target": target, "give": give, "want": want}}, status)
}
func (tb *table) waitOffersExpired(c *client) {
	tb.t.Helper()
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
		if len(offers(c)) == 0 {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	tb.t.Fatal("offers did not expire")
}

func TestMonopolyTradeMortgageChangedHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { s.Properties[5].Owner = 0 })
	a, b := tb.people[0], tb.people[1]
	propose(a, 1, bundle(0, 5), bundle(100), 200)
	id := lastOffer(a)
	a.act(map[string]any{"type": "mortgage", "property": 5}, 200)
	before := a.room()
	b.act(map[string]any{"type": "trade_accept", "tradeId": id}, 400)
	if !reflect.DeepEqual(before, a.room()) {
		t.Fatal("changed mortgage sold under stale approval")
	}
	tb.record("mortgage change invalidates original terms")
}

func TestMonopolyTradeAtomicHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Properties[5].Owner = 0
		s.Properties[5].Mortgaged = true
		s.Properties[6].Owner = 1
	})
	a, b, c := tb.people[0], tb.people[1], tb.people[2]
	deadline := a.room()["turnDeadline"]
	propose(a, 1, bundle(0, 5), bundle(100, 6), 200)
	id := lastOffer(a)
	if len(offers(c)) != 0 || len(offers(tb.spectator)) != 0 {
		t.Fatal("private offer leaked")
	}
	c.act(map[string]any{"type": "trade_accept", "tradeId": id}, 400)
	tb.restart()
	b.act(map[string]any{"type": "trade_accept", "tradeId": id}, 200)
	r := a.room()
	if money(r, 0) != 1600 || money(r, 1) != 1390 || property(r, 5)["owner"] != float64(1) || property(r, 6)["owner"] != float64(0) || property(r, 5)["mortgaged"] != true {
		t.Fatal("atomic trade/fee wrong", r)
	}
	if r["turnDeadline"] != deadline {
		t.Fatal("optional trade extended deadline")
	}
	b.act(map[string]any{"type": "trade_accept", "tradeId": id}, 400)
	tb.record("private proposal, public settled assets")
}

func TestMonopolyTradeMortgageAffordabilityHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Properties[37].Owner = 0
		s.Properties[37].Mortgaged = true
		s.Players[1].Cash = 10
	})
	a, b := tb.people[0], tb.people[1]
	before := a.room()
	propose(a, 1, bundle(0, 37), bundle(0), 400)
	if !reflect.DeepEqual(before, a.room()) {
		t.Fatal("unfunded fee mutated assets")
	}
	give := bundle(200, 37)
	give["redeem"] = []int{37}
	propose(a, 1, give, bundle(0), 200)
	id := lastOffer(a)
	b.act(map[string]any{"type": "trade_accept", "tradeId": id}, 200)
	r := a.room()
	if money(r, 0) != 1300 || money(r, 1) != 17 || property(r, 37)["mortgaged"] != false || property(r, 37)["owner"] != float64(1) {
		t.Fatal("incoming funds and immediate redemption", r)
	}
	tb.record("incoming cash funded mortgage redemption")
}

func TestMonopolyTradeRevisionAndDoubleSpendHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Properties[5].Owner = 0
		s.Properties[6].Owner = 1
		s.Properties[9].Owner = 2
	})
	a, b, c := tb.people[0], tb.people[1], tb.people[2]
	propose(a, 1, bundle(0, 5), bundle(100), 200)
	old := lastOffer(a)
	propose(a, 1, bundle(0, 5), bundle(200), 200)
	current := lastOffer(a)
	if old == current {
		t.Fatal("edited offer reused approval token")
	}
	b.act(map[string]any{"type": "trade_accept", "tradeId": old}, 400)
	propose(c, 0, bundle(300), bundle(0, 5), 200)
	competing := lastOffer(c)
	b.act(map[string]any{"type": "trade_accept", "tradeId": current}, 200)
	before := a.room()
	a.act(map[string]any{"type": "trade_accept", "tradeId": competing}, 400)
	if !reflect.DeepEqual(before, a.room()) {
		t.Fatal("duplicate property spent")
	}
	if money(a.room(), 0) != 1700 || property(a.room(), 5)["owner"] != float64(1) {
		t.Fatal("first trade not retained")
	}
	tb.record("old confirmations and spent assets rejected")
}

func TestMonopolyTradeInvalidAssetsHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		group(s, 0, []int{1, 3}, []int{1, 1})
		s.Properties[5].Owner = 0
		s.Properties[6].Owner = 1
	})
	a := tb.people[0]
	for _, give := range []map[string]any{bundle(0, 1), bundle(0, 5, 5), bundle(0, 6), bundle(0, -1), bundle(0, 40), bundle(-1, 5), {"cash": 0, "jailCards": []string{"chance"}}} {
		propose(a, 1, give, bundle(100), 400)
	}
	propose(a, 0, bundle(0, 5), bundle(100), 400)
	propose(a, 1, bundle(50), bundle(0), 400)
	tb.record("invalid trade assets rejected")
}

func TestMonopolyTradeFundsDebtHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "debt"
		s.Players[0].Cash = 10
		s.Properties[5].Owner = 0
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 2, Amount: 200, Reason: "租金"}
	})
	a, b := tb.people[0], tb.people[1]
	propose(a, 1, bundle(0, 5), bundle(250), 200)
	id := lastOffer(a)
	b.act(map[string]any{"type": "trade_accept", "tradeId": id}, 200)
	r := a.room()
	if money(r, 0) != 60 || money(r, 1) != 1250 || money(r, 2) != 1700 || data(r)["phase"] != "end" {
		t.Fatal("debt not settled from proceeds", r)
	}
	tb.record("transaction funded pending debt")
}

func TestMonopolyTradeExpiryHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { s.Options.TradeSeconds = 2; s.Properties[5].Owner = 0 })
	a, b := tb.people[0], tb.people[1]
	deadline := a.room()["turnDeadline"]
	propose(a, 1, bundle(0, 5), bundle(100), 200)
	id := lastOffer(a)
	tb.restart()
	tb.waitOffersExpired(a)
	r := a.room()
	if r["turnDeadline"] != deadline || data(r)["phase"] != "end" || money(r, 0) != 1500 {
		t.Fatal("expiry changed turn or assets", r)
	}
	for _, seat := range r["seats"].([]any) {
		if seat.(map[string]any)["autoPlay"] == true {
			t.Fatal("optional trade expiry enabled autoplay")
		}
	}
	b.act(map[string]any{"type": "trade_accept", "tradeId": id}, 400)
	tb.record("offer expired without takeover")
}

func TestMonopolyTradeJailCardHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Players[0].JailCards = []string{"chance"}
		for i, id := range s.Decks[0] {
			if id == 0 {
				s.Decks[0] = append(s.Decks[0][:i], s.Decks[0][i+1:]...)
				break
			}
		}
	})
	a, b := tb.people[0], tb.people[1]
	propose(a, 1, map[string]any{"jailCards": []string{"chance"}}, bundle(50), 200)
	b.act(map[string]any{"type": "trade_accept", "tradeId": lastOffer(a)}, 200)
	r := a.room()
	if money(r, 0) != 1550 || len(player(r, 0)["jailCards"].([]any)) != 0 || len(player(r, 1)["jailCards"].([]any)) != 1 {
		t.Fatal("jail card trade duplicated/lost card", r)
	}
	tb.record("jail card exchanged")
}

func TestMonopolyTradeConcurrentApprovalsHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) { s.Properties[5].Owner = 0; s.Properties[9].Owner = 2 })
	a, b, c := tb.people[0], tb.people[1], tb.people[2]
	propose(a, 1, bundle(0, 5), bundle(100), 200)
	first := lastOffer(a)
	propose(c, 1, bundle(0, 9), bundle(100), 200)
	second := lastOffer(c)
	old := b.room()
	a.act(map[string]any{"type": "end"}, 200)
	deadline := b.room()["turnDeadline"]
	for _, id := range []float64{first, second} {
		b.serial++
		b.request("POST", "/api/rooms/"+old["id"].(string), map[string]any{
			"type": "action", "nonce": fmt.Sprintf("e2e-%012d", b.serial), "version": old["version"], "action": map[string]any{"type": "trade_accept", "tradeId": id, "window": data(old)["window"]},
		}, 200)
	}
	r := b.room()
	if money(r, 1) != 1300 || property(r, 5)["owner"] != float64(1) || property(r, 9)["owner"] != float64(1) || r["turnDeadline"] != deadline || data(r)["phase"] != "roll" {
		t.Fatal("independent immutable proposals lost on concurrent/phase updates", r)
	}
	tb.record("immutable approvals survive unrelated versions and phase changes")
}

func TestMonopolyTradeAndAuctionTimersHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "purchase"
		s.Players[0].Position = 6
		s.Properties[5].Owner = 2
		s.Options.TradeSeconds = 1
	})
	a, b, c := tb.people[0], tb.people[1], tb.people[2]
	propose(c, 1, bundle(0, 5), bundle(100), 200)
	a.act(map[string]any{"type": "auction"}, 200)
	tb.waitOffersExpired(b)
	tb.waitPhase("end")
	r := a.room()
	if property(r, 6)["owner"] != float64(-1) || money(r, 1) != 1500 || len(offers(c)) != 0 {
		t.Fatal("one system deadline displaced the other", r)
	}
	tb.record("proposal and auction deadlines both serviced")
}
