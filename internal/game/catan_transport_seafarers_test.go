package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCatanTransportSeaNatural(t *testing.T) {
	for _, scenario := range []string{"shores", "desert"} {
		for n := 2; n <= 6; n++ {
			t.Run(fmt.Sprintf("%s/%d", scenario, n), func(t *testing.T) {
				s, err := NewCatanTransportSeafarers(n, scenario)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
				for step := 0; step < 14000 && !s.Finished; step++ {
					actor := ckActor(s)
					action, e := s.BotAction(actor)
					if e != nil {
						t.Fatal(step, s.Phase, e)
					}
					if e = s.Apply(actor, action); e != nil {
						t.Fatal(step, s.Phase, action, e)
					}
					if step == 83 || s.Phase == "catan_gold" {
						cp := clone(*s)
						s = &cp
						if e = s.validateCatanTransport(); e != nil {
							t.Fatal(e)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round, s.Phase)
				}
				if s.Catan.victoryTarget() != 17 {
					t.Fatal("goal changed")
				}
				t.Logf("completed round %d", s.Round)
			})
		}
	}
}

func TestCatanTransportSeaShipsAndCorruptMap(t *testing.T) {
	s, err := NewCatanTransportSeafarers(3, "desert")
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	for _, site := range g.Transport.Map.Sites {
		for _, id := range site.Paths {
			if g.edgeTerrain(id, true) || !g.edgeTerrain(id, false) {
				t.Fatal("ship into commodity center")
			}
		}
	}
	id := -1
	for _, e := range g.Edges {
		if g.edgeTerrain(e.ID, true) {
			id = e.ID
			break
		}
	}
	if id < 0 {
		t.Fatal("no sea edge")
	}
	e := g.Edges[id]
	q, _ := newCatanTransportTravel(g, g.Transport.Map, 0, e.A, 0)
	for _, owner := range []int{-1, 0, 1} {
		g.Edges[id].Owner = owner
		g.Edges[id].Ship = owner >= 0
		quote, e := q.quote(g, g.Transport.Map, g.Transport.Barbarians, g.Transport.Gold, id)
		if e != nil {
			t.Fatal(e)
		}
		mp, toll := 2, 0
		if owner >= 0 {
			mp = 1
		}
		if owner == 1 {
			toll = 1
		}
		if quote.MP != mp || quote.Toll != toll {
			t.Fatal(owner, quote)
		}
	}
	for _, mutate := range []func(*Catan){func(g *Catan) { g.Transport.Map.Sea = "bad" }, func(g *Catan) { g.Seafarers.VictoryPoints = 14 }, func(g *Catan) { g.Transport.Map.Sites[0].Center = 0 }, func(g *Catan) { g.Tiles[0].Number = 7 }} {
		cp := clone(*s)
		mutate(cp.Catan)
		if cp.validateCatanTransport() == nil {
			t.Fatal("accepted corrupt map")
		}
	}
	base, _ := NewCatanTransport(3)
	if base.Catan.Seafarers != nil || base.Catan.victoryTarget() != 13 {
		t.Fatal("base transport changed")
	}
	before := clone(*base)
	if base.Apply(base.Turn, Action{Type: "catan_ship", Edge: id}) == nil || !reflect.DeepEqual(before, *base) {
		t.Fatal("base admitted ship")
	}
}

func TestCatanTransportSeaGoldResponseUsesClaimingPlayer(t *testing.T) {
	s, err := NewCatanTransportSeafarers(3, "desert")
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		actor := ckActor(s)
		a, e := s.BotAction(actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(actor, a); e != nil {
			t.Fatal(e)
		}
	}
	// Isolate an out-of-turn claim after legal starting construction.
	actor := (s.Turn + 1) % 3
	due := make([]int, 3)
	due[actor] = 1
	s.catanStartGold(due, nil, "catan_turn")
	before := sum(s.Catan.Players[actor].Resources)
	s.AutoCatanPending()
	if s.Catan.GoldPending != nil || s.Phase != "catan_turn" || sum(s.Catan.Players[actor].Resources) != before+1 {
		t.Fatal("gold claim stuck on turn owner")
	}
}

func TestCatanTransportSeaExtendedConfiguration(t *testing.T) {
	for _, scenario := range []string{"shores", "desert"} {
		for _, n := range []int{5, 6} {
			s, err := NewCatanTransportSeafarers(n, scenario)
			if err != nil {
				t.Fatal(err)
			}
			if s.Catan.Seafarers.Layout != CatanTransportSeaExtendedLayout || s.Catan.Seafarers.Variable {
				t.Fatal("site map mislabelled as official variable layout")
			}
			if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
				t.Fatal(err)
			}
			cp := clone(*s)
			if err = cp.validateCatanTransport(); err != nil {
				t.Fatal(err)
			}
			cp.Catan.Seafarers.Layout = "fixed"
			if cp.validateCatanTransport() == nil {
				t.Fatal("accepted wrong layout version")
			}
		}
	}
}

func TestCatanTransportSeaStandardDiceKeepTwoAndTwelve(t *testing.T) {
	for _, n := range []int{2, 3} {
		for _, die := range []int{1, 6} {
			s, err := NewCatanTransportSeafarers(n, "shores")
			if err != nil {
				t.Fatal(err)
			}
			for s.Catan.setup() {
				p := ckActor(s)
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				if e = s.Apply(p, a); e != nil {
					t.Fatal(e)
				}
			}
			calls := 0
			before := clone(*s)
			if err = s.catanTransportRoll(func() [2]int { calls++; return [2]int{die, die} }); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || s.Catan.Dice[0]+s.Catan.Dice[1] != 2*die {
				t.Fatal("transport sea rerolled two/twelve")
			}
			if err = s.catanTwoAfterAction(&before, Action{Type: "catan_roll"}); err != nil {
				t.Fatal(err)
			}
			if err = s.validateCatanTransport(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
