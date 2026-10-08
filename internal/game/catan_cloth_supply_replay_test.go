package game

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
)

type clothSupplyPlan struct {
	Players, Number int
	Villages        []int
	Seats           []struct{ Homes, Ships, Trades []int }
}

// Replay legal actions from all three setup rounds to simultaneous production
// with insufficient common cloth. Only possible dice results are sampled; no
// resources, ships or relations are injected. Verify the site's exact-deficit
// supplement, persistence, and the original five-empty-village ending.
func TestCatanClothExtendedSupplyShortageIsReachable(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			raw, err := os.ReadFile(fmt.Sprintf("testdata/catan-cloth-supply-%d.json", n))
			if err != nil {
				t.Fatal(err)
			}
			var plan clothSupplyPlan
			if err = json.Unmarshal(raw, &plan); err != nil {
				t.Fatal(err)
			}
			s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "cloth"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Players != n || len(plan.Seats) != n || len(plan.Villages) != 6 {
				t.Fatal("invalid route fixture")
			}
			// Fix only the initially random starting seat, before any setup action.
			s.Turn, s.Catan.StartPlayer = 0, 0
			s.Catan.Paired.Primary, s.Catan.Paired.Secondary, s.Catan.Paired.Second = 0, 3, false
			moves, rolls := 0, 0
			check := func() {
				t.Helper()
				assertClothInventory(t, s.Catan)
				fleetSupply(t, s.Catan)
				for p, seat := range s.Catan.Players {
					if s.Catan.shipCount(p) > 15 || s.Catan.settlementPiecesLeft(p) < 0 || s.Catan.cityPiecesLeft(p) < 0 {
						t.Fatal("piece supply exceeded", p)
					}
					for _, count := range seat.Resources {
						if count < 0 {
							t.Fatal("negative resource hand", p)
						}
					}
				}
				for _, village := range s.Catan.cloth().Villages {
					if len(village.Traders) > 3 {
						t.Fatal("impossible village degree")
					}
				}
				if s.Finished {
					t.Fatal("premature victory", s.Winners)
				}
			}
			apply := func(p int, a Action) {
				t.Helper()
				if err := s.Apply(p, a); err != nil {
					t.Fatalf("move%d turn%d phase%s action%+v: %v", moves, s.Turn, s.Phase, a, err)
				}
				moves++
				check()
			}
			placed := make([]int, n)
			for s.Catan.setup() {
				p := s.Turn
				g := s.Catan
				if s.Phase == "catan_setup_settlement" {
					v := plan.Seats[p].Homes[placed[p]]
					placed[p]++
					apply(p, Action{Type: "catan_settlement", Vertex: v})
				} else {
					edge := -1
					ship := false
					for _, id := range plan.Seats[p].Ships {
						if g.setupRoute(p, id, true) {
							edge = id
							ship = true
							break
						}
					}
					if edge < 0 {
						for _, e := range g.Edges {
							if g.setupRoute(p, e.ID, false) {
								edge = e.ID
								break
							}
						}
					}
					if edge < 0 {
						t.Fatal("initial route")
					}
					kind := "catan_road"
					if ship {
						kind = "catan_ship"
					}
					apply(p, Action{Type: kind, Edge: edge})
				}
			}
			if sum(s.Catan.cloth().Held) != 0 {
				t.Fatal("unexpected setup trade")
			}
			roll := func(total int) {
				t.Helper()
				if s.Phase != "catan_roll" {
					t.Fatal("not ready to roll", s.Phase)
				}
				for attempt := 0; attempt < 1000; attempt++ {
					trial := clone(*s)
					err := trial.Apply(trial.Turn, Action{Type: "catan_roll"})
					if err == nil && sum(trial.Catan.Dice) == total {
						*s = trial
						moves++
						rolls++
						check()
						return
					}
				}
				t.Fatal("unable to sample dice", total)
			}
			// Every fund-raising roll is an actually possible 3 or 11, neither of which
			// produces cloth on this map. A player can exchange real surplus resources.
			ready := func(p int) {
				t.Helper()
				for attempts := 0; attempts < 1000; attempts++ {
					if s.Phase == "catan_roll" {
						total := 3
						if rolls%2 == 1 {
							total = 11
						}
						roll(total)
					}
					if s.Phase != "catan_turn" {
						t.Fatal("unexpected pending phase", s.Phase)
					}
					if s.Turn == p && !s.Catan.Paired.Second {
						return
					}
					// Reduce hoards through ordinary bank trades so other settlements can
					// produce again; this never gifts cards or changes the finite bank.
					g := s.Catan
					actor := s.Turn
					for color := range g.Players[actor].Resources {
						g = s.Catan
						count := g.Players[actor].Resources[color]
						rate := g.rates(actor)[color]
						if count >= rate {
							for take, stock := range g.Bank {
								if take != color && stock > 0 {
									give, want := make([]int, 5), make([]int, 5)
									give[color] = rate
									want[take] = 1
									apply(actor, Action{Type: "catan_bank", Give: give, Take: want})
									break
								}
							}
						}
					}
					apply(s.Turn, Action{Type: "catan_end"})
				}
				t.Fatal("could not reach primary")
			}
			fund := func(p int) {
				t.Helper()
				for attempt := 0; attempt < 80; attempt++ {
					ready(p)
					g := s.Catan
					if g.Players[p].Resources[0] > 0 && g.Players[p].Resources[2] > 0 {
						return
					}
					changed := false
					for _, need := range []int{0, 2} {
						g = s.Catan
						if g.Players[p].Resources[need] > 0 {
							continue
						}
						for color, count := range g.Players[p].Resources {
							reserve := 0
							if color == 0 || color == 2 {
								reserve = 1
							}
							if color == need || count <= reserve {
								continue
							}
							give, take := make([]int, 5), make([]int, 5)
							take[need] = 1
							donor := -1
							for q, seat := range g.Players {
								if q != p && seat.Resources[need] > 0 {
									donor = q
									break
								}
							}
							if donor >= 0 {
								give[color] = 1
								apply(p, Action{Type: "catan_trade_offer", Give: give, Take: take})
								offer := s.Catan.Trade.ID
								apply(donor, Action{Type: "catan_trade_accept", Offer: offer})
								apply(p, Action{Type: "catan_trade_complete", Offer: offer, Target: donor})
								changed = true
								break
							}
							rate := g.rates(p)[color]
							if count-reserve >= rate && g.Bank[need] > 0 {
								give[color] = rate
								apply(p, Action{Type: "catan_bank", Give: give, Take: take})
								changed = true
								break
							}
						}
					}
					if !changed {
						apply(p, Action{Type: "catan_end"})
					}
				}
				t.Fatal("funding stalled", p, s.Catan.Players)
			}
			// Find each terminal's path back to one of this player's three legal homes.
			paths := make([]map[int][]int, n)
			for p, seat := range plan.Seats {
				paths[p] = map[int][]int{}
				prev := map[int]int{}
				queue := slices.Clone(seat.Homes)
				for _, v := range queue {
					prev[v] = -1
				}
				for len(queue) > 0 {
					v := queue[0]
					queue = queue[1:]
					for _, id := range seat.Ships {
						e := s.Catan.Edges[id]
						next := -1
						if e.A == v {
							next = e.B
						} else if e.B == v {
							next = e.A
						}
						if next < 0 {
							continue
						}
						if _, ok := prev[next]; ok {
							continue
						}
						prev[next] = id
						queue = append(queue, next)
					}
				}
				for _, i := range seat.Trades {
					v := s.Catan.cloth().Villages[i].Vertex
					path := []int{}
					for {
						id, ok := prev[v]
						if !ok {
							t.Fatal("unreachable solver relation", p, i)
						}
						if id < 0 {
							break
						}
						path = append(path, id)
						e := s.Catan.Edges[id]
						if e.A == v {
							v = e.B
						} else {
							v = e.A
						}
					}
					slices.Reverse(path)
					paths[p][i] = path
				}
			}
			join := func(p, i int) {
				t.Helper()
				for _, edge := range paths[p][i] {
					if s.Catan.Edges[edge].Owner == p {
						continue
					}
					fund(p)
					apply(p, Action{Type: "catan_ship", Edge: edge})
				}
				if !slices.Contains(s.Catan.cloth().Villages[i].Traders, p) {
					t.Fatal("path failed to join", p, i)
				}
			}
			first := map[int]int{}
			for _, i := range plan.Villages {
				for p := range plan.Seats {
					if _, ok := paths[p][i]; ok {
						first[i] = p
						join(p, i)
						break
					}
				}
			}
			produce := func(number int) {
				t.Helper()
				if s.Phase == "catan_turn" {
					apply(s.Turn, Action{Type: "catan_end"})
				}
				if s.Phase == "catan_turn" {
					apply(s.Turn, Action{Type: "catan_end"})
				}
				roll(number)
			}
			done := map[int]bool{}
			for _, i := range plan.Villages {
				number := s.Catan.cloth().Villages[i].Number
				if !done[number] {
					produce(number)
					done[number] = true
				}
			}
			for _, i := range plan.Villages {
				v := s.Catan.cloth().Villages[i]
				if len(v.Traders) != 1 || v.Stock != 3 {
					t.Fatal("first join/production", i, v)
				}
			}
			for _, i := range plan.Villages {
				for p := range plan.Seats {
					if _, ok := paths[p][i]; ok && p != first[i] {
						join(p, i)
					}
				}
			}
			for _, i := range plan.Villages {
				v := s.Catan.cloth().Villages[i]
				if len(v.Traders) != 3 || v.Stock != 1 {
					t.Fatal("three-trader terminal", i, v)
				}
			}
			done = map[int]bool{}
			for _, i := range plan.Villages {
				number := s.Catan.cloth().Villages[i].Number
				if number != plan.Number && !done[number] {
					produce(number)
					done[number] = true
				}
			}
			c := s.Catan.cloth()
			empty := 0
			for _, v := range c.Villages {
				if v.Stock == 0 {
					empty++
				}
			}
			if empty != 4 || c.Stock != 2 || clothTotal(s.Catan) != 70 {
				t.Fatal("boundary not reached", empty, c.Stock, clothTotal(s.Catan))
			}
			if s.Phase == "catan_turn" {
				apply(s.Turn, Action{Type: "catan_end"})
			}
			if s.Phase == "catan_turn" {
				apply(s.Turn, Action{Type: "catan_end"})
			}
			if s.Phase != "catan_roll" {
				t.Fatal("boundary not at roll phase", s.Phase)
			}
			needed, matching := 0, 0
			for _, village := range c.Villages {
				if village.Number == plan.Number {
					if village.Stock != 1 || len(village.Traders) != 3 {
						t.Fatal("unexpected final village", village)
					}
					needed += len(village.Traders) - village.Stock
					matching++
				}
			}
			if matching != 2 || needed != 4 || needed <= c.Stock {
				t.Fatal("no simultaneous shortage")
			}
			saved, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			var restored State
			if err := json.Unmarshal(saved, &restored); err != nil {
				t.Fatal(err)
			}
			*s = restored
			check()
			paid := false
			for attempt := 0; attempt < 1000; attempt++ {
				trial := clone(*s)
				if err := trial.Apply(trial.Turn, Action{Type: "catan_roll"}); err != nil {
					t.Fatal(err)
				}
				if trial.Catan.Dice[0]+trial.Catan.Dice[1] != plan.Number {
					continue
				}
				after := trial.Catan.cloth()
				if after.Issued != 2 || after.Stock != 0 || clothTotal(trial.Catan) != 72 {
					t.Fatal("incorrect exact deficit", after)
				}
				for p, held := range c.Held {
					want := held
					for _, village := range c.Villages {
						if village.Number == plan.Number && slices.Contains(village.Traders, p) {
							want++
						}
					}
					if after.Held[p] != want {
						t.Fatal("trader was not paid exactly once per producing village", p, after.Held[p], want)
					}
				}
				*s = clone(trial)
				if s.Catan.cloth().Issued != 2 || s.Catan.validateClothSupply() != nil {
					t.Fatal("issued cloth lost on restore")
				}
				if s.Finished || s.Phase != "catan_turn" {
					t.Fatal("depletion must wait for the end of this action turn", s.Phase)
				}
				if err := s.Apply(s.Turn, Action{Type: "catan_end"}); err != nil || !s.Finished || len(s.Winners) == 0 {
					t.Fatal("six empty villages did not end the game", err)
				}
				paid = true
				break
			}
			if !paid {
				t.Fatal("did not sample simultaneous production")
			}
			t.Logf("legal moves=%d sampled production turns=%d stocks=%d held=%v ships=%v", moves, rolls, c.Stock, c.Held, func() []int {
				r := []int{}
				for p := range plan.Seats {
					r = append(r, s.Catan.shipCount(p))
				}
				return r
			}())
		})
	}
}
