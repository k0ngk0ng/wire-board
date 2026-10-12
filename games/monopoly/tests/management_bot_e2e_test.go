package monopoly_test

// Failure scenarios before bot changes: a non-bidding bot monopolizes auction
// scheduling; receiving an offer stalls waiting; unfair offers blindly accepted;
// buildings cannot be liquidated under debt; inventory-short hotel sale loops.
import (
	"fmt"
	"testing"
	"time"

	monopoly "github.com/k0ngk0ng/wire-board/games/monopoly/server"
)

func TestMonopolyBotTradeResponseHTTP(t *testing.T) {
	for _, price := range []int{100, 400} {
		t.Run(fmt.Sprint(price), func(t *testing.T) {
			tb := newTable(t, 3, func(s *monopoly.State) { s.Properties[5].Owner = 0 })
			a, b := tb.people[0], tb.people[1]
			propose(a, 1, bundle(0, 5), bundle(price), 200)
			b.command("autoplay", map[string]any{"enabled": true}, 200)
			for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
				if len(offers(a)) == 0 {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			if len(offers(a)) != 0 {
				t.Fatal("bot ignored trade")
			}
			wantOwner, wantCash := float64(0), 1500
			if price == 100 {
				wantOwner = 1
				wantCash = 1600
			}
			r := a.room()
			if property(r, 5)["owner"] != wantOwner || money(r, 0) != wantCash {
				t.Fatal("bot did not evaluate trade value", r)
			}
			tb.record("bot answered optional trade")
		})
	}
}

func TestMonopolyBotDebtBuildingSaleHTTP(t *testing.T) {
	tb := newTable(t, 3, func(s *monopoly.State) {
		s.Phase = "debt"
		s.Players[0].Cash = 10
		group(s, 0, []int{1, 3}, []int{1, 1})
		s.Debt = &monopoly.Debt{Debtor: 0, Creditor: 1, Amount: 30, Reason: "租金"}
	})
	tb.people[0].command("autoplay", map[string]any{"enabled": true}, 200)
	for until := time.Now().Add(5 * time.Second); time.Now().Before(until); {
		if money(tb.people[1].room(), 1) == 1530 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	r := tb.people[1].room()
	if money(r, 1) != 1530 || player(r, 0)["bankrupt"] == true {
		t.Fatal("bot could not fund debt from buildings", r)
	}
	tb.record("bot sold a building to settle debt")
}
