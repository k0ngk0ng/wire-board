package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCatanCaravansTransportNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, err := NewCatanCaravansTransport(n, knights)
					if err != nil {
						t.Fatal(err)
					}
					if events {
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					votes := 0
					for step := 0; step < 16000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(step, s.Phase, e)
						}
						before := s.Phase
						if e = s.Apply(p, a); e != nil {
							t.Fatal(step, before, a, e)
						}
						if s.Phase == "catan_caravan_bid" && before != s.Phase {
							votes++
						}
						if step%53 == 0 {
							raw, e := json.Marshal(s)
							if e != nil {
								t.Fatal(e)
							}
							var restored State
							if e = json.Unmarshal(raw, &restored); e != nil {
								t.Fatal(e)
							}
							if e = restored.validateCatanTransport(); e != nil {
								t.Fatal(e)
							}
							s = &restored
							for viewer := -1; viewer < n; viewer++ {
								s.View(viewer)
							}
						}
					}
					if !s.Finished || votes == 0 {
						t.Fatal("incomplete", s.Round, votes)
					}
					t.Log("round", s.Round, "votes", votes)
				})
			}
		}
	}
}
func TestCatanCaravansTransportMapGuards(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for trial := 0; trial < 8; trial++ {
				s, err := NewCatanCaravansTransport(n, knights)
				if err != nil {
					t.Fatal(n, knights, err)
				}
				g := s.Catan
				wantHoles, wantStarts := 1, 3
				if n > 4 {
					wantHoles, wantStarts = 2, 6
				}
				if len(g.Caravans.Map.WateringHoles) != wantHoles || len(g.Caravans.Map.Starts) != wantStarts || g.victoryTarget() != 15 || g.LongestOwner != -1 || g.Robber != -1 {
					t.Fatal("wrong setup", n)
				}
				for _, site := range g.Transport.Map.Sites {
					if g.Tiles[site.Tile].Number == 0 || g.productionResource(g.Tiles[site.Tile]) >= 5 {
						t.Fatal("commodity not producing")
					}
				}
				broken := clone(*s)
				broken.Catan.Transport.Map.Caravans = ""
				if broken.validateCatanTransport() == nil {
					t.Fatal("missing marker accepted")
				}
				broken = clone(*s)
				broken.Catan.Caravans.Map.Starts[0].Edge++
				if broken.validateCatanTransport() == nil {
					t.Fatal("wrong start accepted")
				}
				broken = clone(*s)
				broken.Catan.Tiles[g.Caravans.Map.WateringHoles[0]].Number = 8
				if broken.validateCatanTransport() == nil {
					t.Fatal("numbered hole accepted")
				}
			}
		}
	}
}

func TestCatanCaravansTransportTurnOrderAndEscrow(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, knights := range []bool{false, true} {
			s, err := NewCatanCaravansTransport(n, knights)
			if err != nil {
				t.Fatal(err)
			}
			reached := false
			for step := 0; step < 8000 && !s.Finished; step++ {
				p := twoFullActor(s)
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				previous, active, serial := s.Phase, s.Turn, s.Catan.TurnSerial
				if e = s.Apply(p, a); e != nil {
					t.Fatal(step, previous, e)
				}
				if s.Phase != "catan_caravan_bid" || previous == s.Phase {
					continue
				}
				reached = true
				g := s.Catan
				if previous != "catan_transport_move" || s.Turn != active || g.TurnSerial != serial || g.Transport.Travel != nil || g.Transport.ArrivalResolved {
					t.Fatal("turn advanced before vote or travel still pending")
				}
				before := len(g.Caravans.Wagons)
				wantSerial, wantActor := serial+1, (active+1)%n
				if g.Paired != nil {
					if !g.Paired.Second {
						wantSerial = serial
						wantActor = g.Paired.Secondary
					} else {
						wantActor = (g.Paired.Primary + 1) % n
					}
				}
				bidder := g.Caravans.Pending.actor()
				color := g.caravanBidColors()[0]
				if g.Players[bidder].Resources[color] == 0 {
					g.Bank[color]--
					g.Players[bidder].Resources[color]++
				}
				bid := make([]int, 5)
				bid[color] = 1
				if e = s.Apply(bidder, Action{Type: "catan_caravan_bid", Offer: g.Caravans.Sequence, Tokens: bid}); e != nil {
					t.Fatal("escrow", e)
				}
				raw, _ := json.Marshal(s)
				var restored State
				if e = json.Unmarshal(raw, &restored); e != nil {
					t.Fatal(e)
				}
				s = &restored
				if e = s.validateCatanTransport(); e != nil {
					t.Fatal(e)
				}
				for response := 0; s.Catan.Caravans.Pending != nil && response < 30; response++ {
					actor := twoFullActor(s)
					a, e = s.BotAction(actor)
					if e != nil {
						t.Fatal(e)
					}
					if e = s.Apply(actor, a); e != nil {
						t.Fatal(e)
					}
				}
				if s.Catan.Caravans.Pending != nil {
					t.Fatal("vote unfinished")
				}
				want := 1
				if n == 2 {
					want = 2
				}
				if len(s.Catan.Caravans.Wagons) != before+want {
					t.Fatal("wrong wagon count")
				}
				if !s.Finished && (s.Catan.TurnSerial != wantSerial || s.Turn != wantActor || s.Catan.Transport.GameTurn != s.Catan.TurnSerial || s.Catan.Transport.Active != s.Turn) {
					t.Fatal("missing or double turn advance")
				}
				break
			}
			if !reached {
				t.Fatal("never reached vote", n, knights)
			}
		}
	}
}

func TestCatanCaravansTransportProductionAndInvention(t *testing.T) {
	for _, n := range []int{2, 6} {
		s, err := NewCatanCaravansTransport(n, true)
		if err != nil {
			t.Fatal(err)
		}
		for s.Catan.setup() {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e == nil {
				e = s.Apply(p, a)
			}
			if e != nil {
				t.Fatal(e)
			}
		}
		g := s.Catan
		left, right := -1, -1
		for _, a := range g.inventionNumbers() {
			for _, b := range g.inventionNumbers() {
				if a.Number != b.Number {
					left, right = a.Tile, b.Tile
				}
			}
		}
		if left < 0 {
			t.Fatal("no invention choices")
		}
		if err = s.catanInvention(Action{Tile: left, Target: right}); err != nil {
			t.Fatal(err)
		}
		if err = s.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
		bad := clone(*s)
		bad.Catan.Caravans.Map.NumberSwaps = nil
		if bad.validateCatanTransport() == nil {
			t.Fatal("missing provenance accepted")
		}
		// Production lookup must also apply at setup and in city commodity splitting.
		for _, site := range g.Transport.Map.Sites {
			tile := g.Tiles[site.Tile]
			want := map[string]int{"quarry": 1, "glassworks": 0, "castle": 2}[site.Kind]
			if g.productionResource(tile) != want {
				t.Fatal("wrong resource", site.Kind)
			}
		}
		for _, choice := range g.Caravans.choices(g) {
			if !g.caravanEdgeAllowed(choice.Edge) {
				t.Fatal("invalid border")
			}
			old := g.Transport.Barbarians
			g.Transport.Barbarians[0] = choice.Edge
			found := false
			for _, c := range g.Caravans.choices(g) {
				if c == choice {
					found = true
				}
			}
			g.Transport.Barbarians = old
			if !found {
				t.Fatal("barbarian blocks caravan")
			}
		}
	}
}
