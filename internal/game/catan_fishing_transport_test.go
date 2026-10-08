package game

import (
	"encoding/json"
	"fmt"
	"testing"
)

func fishingTransportRestore(t *testing.T, s *State) {
	t.Helper()
	raw, _ := json.Marshal(s)
	var next State
	if e := json.Unmarshal(raw, &next); e != nil {
		t.Fatal(e)
	}
	if e := next.validateCatanTransport(); e != nil {
		t.Fatal(e)
	}
	if e := next.validateCatanEventSession(); e != nil {
		t.Fatal(e)
	}
	*s = next
}
func TestCatanFishingTransportConstruction(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			s, e := NewCatanFishingTransport(n, knights)
			if e != nil {
				t.Fatal(n, knights, e)
			}
			fishingTransportRestore(t, s)
		}
	}
}
func TestCatanFishingTransportNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, e := NewCatanFishingTransport(n, knights)
					if e != nil {
						t.Fatal(e)
					}
					if events {
						if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
							t.Fatal(e)
						}
					}
					actions := map[string]int{}
					for step := 0; step < 16000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, e := s.BotAction(p)
						if e != nil {
							t.Fatal(step, s.Phase, e)
						}
						if e = s.Apply(p, a); e != nil {
							t.Fatal(step, s.Phase, a, e)
						}
						actions[a.Type]++
						if step%83 == 0 {
							fishingTransportRestore(t, s)
							for viewer := -1; viewer < n; viewer++ {
								s.View(viewer)
							}
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round, s.Phase)
					}
					t.Log("rounds", s.Round, "fish boosts", actions["catan_transport_fish"])
				})
			}
		}
	}
}

func fishingTransportTurn(t *testing.T, n int, knights bool) *State {
	t.Helper()
	s, e := NewCatanFishingTransport(n, knights)
	if e != nil {
		t.Fatal(e)
	}
	for s.Catan.setup() || s.Phase != "catan_turn" {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	return s
}
func TestCatanFishingTransportFishWheatSwiftAndPrivacy(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, knights := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", n, knights), func(t *testing.T) {
				s := fishingTransportTurn(t, n, knights)
				p := s.Turn
				fishingAttackHand(t, s, p, 7)
				if !knights {
					transportGiveDev(t, s, p, 2)
					if e := s.Apply(p, Action{Type: "catan_dev", Card: 2}); e != nil {
						t.Fatal(e)
					}
				}
				if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
					t.Fatal(e)
				}
				tr := s.Catan.Transport
				seq := int(tr.Sequence)
				points := tr.Travel.Points
				action := Action{Type: "catan_transport_fish", Offer: seq, Tokens: s.Catan.fishPayment(p, s.Catan.fishActionCost(p, "catan_transport_fish"))}
				attackReject(t, s, (p+1)%n, action)
				bad := action
				bad.Offer--
				attackReject(t, s, p, bad)
				bad = action
				bad.Tokens = nil
				attackReject(t, s, p, bad)
				before := len(s.Catan.Fishing.Tokens.Hands[p])
				if e := s.Apply(p, action); e != nil {
					t.Fatal(e)
				}
				if s.Catan.Transport.Travel.Points != points+2 || !s.Catan.Transport.Travel.FishUsed || len(s.Catan.Fishing.Tokens.Hands[p]) != before-len(action.Tokens) {
					t.Fatal("fish boost payment")
				}
				attackReject(t, s, p, action)
				attackReject(t, s, p, Action{Type: "catan_transport_wheat", Offer: seq})
				fishingTransportRestore(t, s)
				for viewer := -1; viewer < n; viewer++ {
					choices := s.catanTransportChoices(viewer)
					if viewer != p && (choices["fishTokens"] != nil || choices["canFish"] != nil) {
						t.Fatal("fish identities leaked")
					}
				}
				if !knights {
					if e := s.Apply(p, Action{Type: "catan_transport_stop", Offer: seq}); e != nil {
						t.Fatal(e)
					}
					if s.Catan.Transport.Moves != 2 || s.Catan.Transport.Travel.Points != points || !s.Catan.Transport.Travel.FishUsed {
						t.Fatal("swift reset boost/use limit")
					}
					action.Offer = int(s.Catan.Transport.Sequence)
					attackReject(t, s, p, action)
					attackReject(t, s, p, Action{Type: "catan_transport_wheat", Offer: action.Offer})
					fishingTransportRestore(t, s)
				}
			})
		}
	}
	s := fishingTransportTurn(t, 3, false)
	p := s.Turn
	fishingAttackHand(t, s, p, 7)
	transportGive(s.Catan, p, []int{0, 0, 0, 1, 0})
	if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	seq := int(s.Catan.Transport.Sequence)
	if e := s.Apply(p, Action{Type: "catan_transport_wheat", Offer: seq}); e != nil {
		t.Fatal(e)
	}
	attackReject(t, s, p, Action{Type: "catan_transport_fish", Offer: seq, Tokens: s.Catan.fishPayment(p, 2)})
}
func TestCatanFishingTransportMapTargetAndInvention(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, knights := range []bool{false, true} {
			s := fishingTransportTurn(t, n, knights)
			g := s.Catan
			want := 12
			if knights {
				want = 15
			}
			if g.victoryTarget() != want {
				t.Fatal("wrong fish target")
			}
			for _, ground := range g.Fishing.Map.Grounds {
				used := map[int]bool{}
				for _, p := range g.Ports {
					used[p.Edge] = true
				}
				for _, site := range g.Transport.Map.Sites {
					for _, edge := range site.Blocked {
						used[edge] = true
					}
				}
				for _, edge := range ground.Edges {
					if used[edge] {
						t.Fatal("ground overlaps port/blocked road")
					}
				}
			}
			for _, change := range []func(*Catan){func(g *Catan) { g.Fishing.Transport = "" }, func(g *Catan) { g.Fishing.Map.Grounds[0].Number = 7 }, func(g *Catan) { g.Fishing.Map.Grounds[0].Edges[0] = -1 }, func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{Tile: 0, Numbers: []int{2, 3, 11, 12}}} }} {
				bad := clone(*s)
				change(bad.Catan)
				if bad.validateCatanTransport() == nil {
					t.Fatal("accepted malformed map")
				}
			}
			if knights {
				choices := g.inventionNumbers()
				if len(choices) < 2 {
					t.Fatal("no invention")
				}
				if e := s.catanInvention(Action{Tile: choices[0].Tile, Target: choices[1].Tile}); e != nil {
					t.Fatal(e)
				}
				fishingTransportRestore(t, s)
			}
			base, e := NewCatanTransport(n)
			if e != nil || base.Catan.Fishing != nil || base.Catan.victoryTarget() != 13 {
				t.Fatal("ordinary target changed", e)
			}
		}
	}
}

func TestCatanFishingTransportProductionDiscountAndBoot(t *testing.T) {
	for _, knights := range []bool{false, true} {
		s := fishingTransportTurn(t, 2, knights)
		g := s.Catan
		p := s.Turn
		ground := g.Fishing.Map.Grounds[0]
		for i := range g.Vertices {
			g.Vertices[i].Owner = -1
			g.Vertices[i].Level = 0
		}
		v := ground.Vertices[1]
		g.Vertices[v].Owner = p
		for _, level := range []int{1, 2} {
			g.Vertices[v].Level = level
			due, err := g.Fishing.Map.production(g, ground.Number)
			if err != nil || due[p] != level || due[1-p] != 0 {
				t.Fatal("coastal production", due, err)
			}
		}
		g.Vertices[v].Level = 1
		g.Vertices[ground.Vertices[0]].Owner = 1 - p
		g.Vertices[ground.Vertices[0]].Level = 2
		s.catanScores()
		if g.fishActionCost(p, "catan_transport_fish") != 1 || g.fishActionCost(1-p, "catan_transport_fish") != 2 {
			t.Fatal("trailing discount")
		}
		goal := 12
		if knights {
			goal = 15
		}
		g.Fishing.Tokens.BootOwner = p
		if g.victoryTargetFor(p) != goal+1 || g.victoryTargetFor(1-p) != goal {
			t.Fatal("boot target")
		}
	}
}
