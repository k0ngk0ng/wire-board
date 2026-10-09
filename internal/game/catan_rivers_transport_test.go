package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func riversTransportRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
	if err = next.validateCatanEventSession(); err != nil {
		t.Fatal(err)
	}
	*s = next
}
func TestCatanRiversTransportNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, err := NewCatanRiversTransport(n, knights)
					if err != nil {
						t.Fatal(err)
					}
					if events {
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					for step := 0; step < 12000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, err := s.BotAction(p)
						if err != nil {
							t.Fatal(step, s.Phase, err)
						}
						if err = s.Apply(p, a); err != nil {
							t.Fatal(step, s.Phase, a, err)
						}
						if step%37 == 0 {
							riversTransportRestore(t, s)
							for viewer := -1; viewer < n; viewer++ {
								s.View(viewer)
							}
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round)
					}
					riversTransportRestore(t, s)
					t.Log("round", s.Round, "winner", s.Winners)
				})
			}
		}
	}
}

func riversTransportPlaying(t *testing.T, n int, knights bool) *State {
	t.Helper()
	s, err := NewCatanRiversTransport(n, knights)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Catan.setup() && step < 100; step++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e == nil {
			e = s.Apply(p, a)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if s.Catan.setup() {
		t.Fatal("setup unfinished")
	}
	return s
}
func TestCatanRiversTransportSharedGoldAndInvention(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, knights := range []bool{false, true} {
			s := riversTransportPlaying(t, n, knights)
			g := s.Catan
			p := s.Turn
			before := g.Transport.Gold[p]
			if err := s.catanRiverReward(p, 2); err != nil {
				t.Fatal(err)
			}
			if g.riverGold()[p] != before+2 || g.tradeGold()[p] != before+2 || len(g.Rivers.Gold) != 0 {
				t.Fatal("duplicate ledger")
			}
			if g.riverBridgeReward() != 2 {
				t.Fatal("wrong bridge reward")
			}
			g.Transport.Gold[p] += g.Transport.GoldBank
			g.Transport.GoldBank = 0
			if err := s.catanRiverReward(p, 3); err != nil {
				t.Fatal(err)
			}
			if g.Transport.GoldIssued != 3 {
				t.Fatal("ledger not issued")
			}
			for i := range g.Players {
				if g.riverPoints(i) < 0 {
					t.Fatal("poverty penalty")
				}
			}
			riversTransportRestore(t, s)
			g = s.Catan
			if knights {
				choices := g.inventionNumbers()
				left, right := -1, -1
				for _, a := range choices {
					for _, b := range choices {
						if a.Number != b.Number {
							left, right = a.Tile, b.Tile
						}
					}
				}
				if left < 0 {
					t.Fatal("no invention choices")
				}
				if err := s.catanInvention(Action{Tile: left, Target: right}); err != nil {
					t.Fatal(err)
				}
				riversTransportRestore(t, s)
				broken := clone(*s)
				broken.Catan.Rivers.Map.NumberSwaps = nil
				if err := broken.validateCatanTransport(); err == nil {
					t.Fatal("missing river provenance accepted")
				}
			}
			broken := clone(*s)
			broken.Catan.Rivers.Gold = make([]int, n)
			if err := broken.validateCatanTransport(); err == nil {
				t.Fatal("duplicate ledger accepted")
			}
			broken = clone(*s)
			broken.Catan.Rivers.Transport = ""
			if err := broken.validateCatanTransport(); err == nil {
				t.Fatal("missing scenario marker accepted")
			}
		}
	}
}
func TestCatanRiversTransportCommodityProduction(t *testing.T) {
	for _, knights := range []bool{false, true} {
		s := riversTransportPlaying(t, 3, knights)
		g := s.Catan
		// Isolate each commodity hex in a production fixture to test exact resource
		// and commodity yields, including city production and the doubled 2/12 tile.
		for _, site := range g.Transport.Map.Sites {
			wantColor := map[string]int{"quarry": 1, "glassworks": 0, "castle": 2}[site.Kind]
			for _, level := range []int{1, 2} {
				q := clone(*s)
				b := q.Catan
				for i := range b.Vertices {
					b.Vertices[i].Owner = -1
					b.Vertices[i].Level = 0
				}
				for i := range b.Tiles {
					b.Tiles[i].Number = 0
				}
				for p := range b.Players {
					catanMove(b.Players[p].Resources, b.Bank, append([]int{}, b.Players[p].Resources...))
				}
				v := b.Tiles[site.Tile].Vertices[0]
				b.Vertices[v].Owner = 0
				b.Vertices[v].Level = level
				b.Rivers.Map.DoubleNumberTile = site.Tile
				b.Tiles[site.Tile].Number = 12
				want := make([]int, len(b.Bank))
				b.cityProduction(want, wantColor, level)
				for _, total := range []int{2, 12} {
					if err := q.catanRollProduction(total); err != nil {
						t.Fatal(err)
					}
					for color, count := range want {
						if b.Players[0].Resources[color] != count {
							t.Fatalf("%s knights%t level%d roll%d color%d got%d want%d", site.Kind, knights, level, total, color, b.Players[0].Resources[color], count)
						}
					}
					catanMove(b.Players[0].Resources, b.Bank, append([]int{}, b.Players[0].Resources...))
				}
				if b.Tiles[site.Tile].Resource != catanTransportTerrain {
					t.Fatal("production altered artwork terrain")
				}
			}
		}
	}
}
func TestCatanRiversTransportRollIsolation(t *testing.T) {
	for _, combined := range []bool{false, true} {
		s, err := NewCatanTransport(3)
		if combined {
			s, err = NewCatanRiversTransport(3, false)
		}
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
		calls := 0
		if err := s.catanTransportRoll(func() [2]int {
			calls++
			if calls == 1 {
				return [2]int{1, 1}
			}
			return [2]int{2, 3}
		}); err != nil {
			t.Fatal(err)
		}
		if combined && calls != 1 || !combined && calls != 2 {
			t.Fatal("base/combined dice isolation", combined, calls)
		}
	}
}

func TestCatanRiversTransportEliminationLedger(t *testing.T) {
	for _, knights := range []bool{false, true} {
		s := riversTransportPlaying(t, 6, knights)
		for step := 0; s.Phase != "catan_turn" && step < 100; step++ {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e == nil {
				e = s.Apply(p, a)
			}
			if e != nil {
				t.Fatal(e)
			}
		}
		if s.Phase != "catan_turn" {
			t.Fatal("turn unavailable")
		}
		g := s.Catan
		p := s.Turn
		bank, held := g.Transport.GoldBank, g.Transport.Gold[p]
		if err := s.EliminateCatan(p); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Transport.Gold[p] != 0 || s.Catan.Transport.GoldBank != bank+held || len(s.Catan.Rivers.Gold) != 0 {
			t.Fatal("elimination duplicated or lost coins")
		}
		riversTransportRestore(t, s)
	}
}
