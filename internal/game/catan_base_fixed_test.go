package game

import (
	"fmt"
	"reflect"
	"testing"
)

func TestCatanFixedFiveSixPrintedSetup(t *testing.T) {
	// Resource hands printed beside the six outlined villages on page 2.
	hands := [][]int{{0, 1, 1, 0, 1}, {0, 0, 1, 1, 1}, {1, 0, 0, 1, 1}, {0, 0, 1, 1, 1}, {1, 1, 1, 0, 0}, {1, 0, 0, 1, 1}}
	for _, n := range []int{5, 6} {
		for shift := range 6 {
			for _, helpers := range []bool{false, true} {
				s, err := NewCatan(n, CatanOptions{FiveSix: true, Helpers: helpers, AllHelpers: helpers})
				if err != nil {
					t.Fatal(err)
				}
				colors := make([]int, 6)
				for i := range colors {
					colors[i] = (i + shift) % 6
				}
				if err := s.applyCatanFixedFiveSix(colors); err != nil {
					t.Fatal(err)
				}
				g := s.Catan
				if g.setup() || s.Phase != "catan_roll" || s.Turn != g.StartPlayer || g.TurnSerial != 1 || g.Paired.Primary != g.StartPlayer || g.Paired.Secondary != (g.StartPlayer+3)%n {
					t.Fatal("fixed setup did not reach first production turn")
				}
				for seat, p := range g.Players {
					if !reflect.DeepEqual(p.Resources, hands[colors[seat]]) || p.Score != 2 {
						t.Fatal("printed starting hand/score", seat, colors[seat], p.Resources, p.Score)
					}
					r, v, c := g.pieces(seat)
					if r != 2 || v != 2 || c != 0 {
						t.Fatal("starting pieces", r, v, c)
					}
					if helpers && (p.Helper == nil || p.Helper.ID != (seat-g.StartPlayer+n)%n+1 || !g.helperReady(seat, p.Helper.ID)) {
						t.Fatal("initial helper not ready in first turn")
					}
				}
				neutral := 0
				for _, v := range g.Vertices {
					if v.Level > 0 && v.Owner < 0 {
						neutral++
					}
				}
				if neutral != (6-n)*2 || (n == 5 && g.BaseSetup.NeutralColor != colors[5]) || (n == 6 && g.BaseSetup.NeutralColor != -1) {
					t.Fatal("unused color's two neutral villages")
				}
				terrain, numbers, ports := make([]int, 6), make([]int, 13), make([]int, 6)
				for _, tile := range g.Tiles {
					terrain[tile.Resource]++
					numbers[tile.Number]++
				}
				if !reflect.DeepEqual(terrain, []int{6, 5, 6, 6, 5, 2}) || !reflect.DeepEqual(numbers, []int{2, 0, 2, 3, 3, 3, 3, 0, 3, 3, 3, 3, 2}) {
					t.Fatal("printed inventory", terrain, numbers)
				}
				seen := map[int]bool{}
				for _, port := range g.Ports {
					ports[port.Resource+1]++
					e := g.Edges[port.Edge]
					if len(e.Tiles) != 1 || seen[e.A] || seen[e.B] {
						t.Fatal("port off coast or overlaps", port)
					}
					seen[e.A], seen[e.B] = true, true
				}
				if !reflect.DeepEqual(ports, []int{5, 1, 1, 2, 1, 1}) {
					t.Fatal("port inventory", ports)
				}
				for _, e := range g.Edges {
					if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
						t.Fatal("adjacent starting villages")
					}
					if e.Owner >= 0 && g.Vertices[e.A].Owner != e.Owner && g.Vertices[e.B].Owner != e.Owner {
						t.Fatal("road detached from starting village")
					}
				}
				fleetSupply(t, g)
				copy := clone(*s)
				if !reflect.DeepEqual(*s, copy) {
					t.Fatal("fixed layout persistence")
				}
				colors[0] = -99
				if g.BaseSetup.Colors[0] < 0 {
					t.Fatal("color slice alias")
				}
			}
		}
	}
}

func TestCatanFixedNeutralProductionRobberAndBlockers(t *testing.T) {
	for neutralColor := range 6 {
		s, err := NewCatan(5, CatanOptions{FiveSix: true})
		if err != nil {
			t.Fatal(err)
		}
		colors := []int{}
		for c := range 6 {
			if c != neutralColor {
				colors = append(colors, c)
			}
		}
		colors = append(colors, neutralColor)
		if err := s.applyCatanFixedFiveSix(colors); err != nil {
			t.Fatal(err)
		}
		for total := 2; total <= 12; total++ {
			if total == 7 {
				continue
			}
			with, without := clone(*s), clone(*s)
			for i, v := range without.Catan.Vertices {
				if v.Owner < 0 {
					without.Catan.Vertices[i].Level = 0
				}
			}
			if err := with.catanRollProduction(total); err != nil {
				t.Fatal(err)
			}
			if err := without.catanRollProduction(total); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(with.Catan.Bank, without.Catan.Bank) || !reflect.DeepEqual(with.Catan.Players, without.Catan.Players) {
				t.Fatal("neutral village received production")
			}
		}
		for _, tile := range s.Catan.Tiles {
			if tile.ID == s.Catan.Robber {
				continue
			}
			copy := clone(*s)
			copy.Phase, copy.Catan.ResumePhase = "catan_robber", "catan_turn"
			if err := copy.catanMoveRobber(copy.Turn, tile.ID); err != nil {
				t.Fatal(err)
			}
			for _, victim := range copy.Catan.Victims {
				if victim < 0 || victim >= 5 {
					t.Fatal("neutral robber victim")
				}
			}
		}
		for _, v := range s.Catan.Vertices {
			if v.Level == 0 || v.Owner >= 0 {
				continue
			}
			g := clone(*s.Catan)
			for _, edge := range g.Edges {
				g.Edges[edge.ID].Owner = -1
			}
			edges := g.touching(v.ID)
			g.Edges[edges[0]].Owner = 0
			if g.canRoad(0, edges[1]) {
				t.Fatal("road crossed neutral village")
			}
			g.Edges[edges[1]].Owner = 0
			if g.roadLength(0) != 1 {
				t.Fatal("longest road crossed neutral village")
			}
			for _, e := range edges {
				other := g.Edges[e].A
				if other == v.ID {
					other = g.Edges[e].B
				}
				if g.canSettlement(0, other, true) {
					t.Fatal("distance ignored neutral village")
				}
			}
		}
	}
}

func TestCatanFixedFiveSixRejectsInvalidSetup(t *testing.T) {
	for _, n := range []int{3, 4, 7} {
		if _, err := NewCatanFixedFiveSix(n, CatanOptions{FiveSix: true}); err == nil {
			t.Fatal("invalid player count")
		}
	}
	if _, err := NewCatanFixedFiveSix(5, CatanOptions{}); err == nil {
		t.Fatal("missing extension")
	}
	s, _ := NewCatan(5, CatanOptions{FiveSix: true})
	before := clone(*s)
	if err := s.applyCatanFixedFiveSix([]int{0, 1, 2, 3, 4, 4}); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("invalid colors mutated game")
	}
	if err := s.applyCatanFixedFiveSix([]int{0, 1, 2, 3, 4, 5}); err != nil {
		t.Fatal(err)
	}
	before = clone(*s)
	if err := s.applyCatanFixedFiveSix([]int{0, 1, 2, 3, 4, 5}); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("running game could be reset")
	}
	s, _ = NewCatan(5, CatanOptions{FiveSix: true})
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, s.Turn, a)
	before = clone(*s)
	if err := s.applyCatanFixedFiveSix([]int{0, 1, 2, 3, 4, 5}); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("partial first placement could be reset")
	}
}

func TestCatanFixedFiveSixViewPrivacy(t *testing.T) {
	s, err := NewCatanFixedFiveSix(5, CatanOptions{FiveSix: true, Helpers: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, viewer := range []int{-1, 0, 4} {
		v := s.View(viewer)["catan"].(map[string]any)
		setup := v["baseSetup"].(map[string]any)
		if setup["layout"] != "fixed" || int(setup["neutralColor"].(float64)) != s.Catan.BaseSetup.NeutralColor || len(setup["colors"].([]any)) != 5 {
			t.Fatal("fixed public identity missing")
		}
		if _, ok := v["devDeck"]; ok {
			t.Fatal("private development deck exposed")
		}
		for i, raw := range v["players"].([]any) {
			p := raw.(map[string]any)
			_, hand := p["resources"]
			_, dev := p["dev"]
			if hand != (i == viewer) || dev != (i == viewer) {
				t.Fatal("private hand exposure", viewer, i)
			}
		}
	}
}

func TestCatanFixedFiveSixBotsComplete(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, helpers := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/helpers=%v", n, helpers), func(t *testing.T) {
				s, err := NewCatanFixedFiveSix(n, CatanOptions{FiveSix: true, Helpers: helpers, AllHelpers: helpers})
				if err != nil {
					t.Fatal(err)
				}
				for step := 0; step < 10000 && !s.Finished; step++ {
					actor := s.Turn
					if pending := s.CatanPendingActor(); pending >= 0 {
						actor = pending
					} else if s.Phase == "catan_discard" {
						for i, due := range s.Catan.DiscardDue {
							if due > 0 {
								actor = i
								break
							}
						}
					}
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					helperApply(t, s, actor, a)
					fleetSupply(t, s.Catan)
					dev := len(s.Catan.DevDeck) + len(s.Catan.DevDiscard) + len(s.Catan.HelperExile)
					if q := s.Catan.HelperPending; q != nil {
						dev += len(q.Cards)
					}
					for i, p := range s.Catan.Players {
						dev += sum(p.Dev)
						r, v, c := s.Catan.pieces(i)
						if r > 15 || v > 5 || c > 4 {
							t.Fatal("piece inventory")
						}
					}
					if dev != 34 {
						t.Fatal("development inventory", dev)
					}
					if step%31 == 0 {
						restored := clone(*s)
						if !reflect.DeepEqual(*s, restored) {
							t.Fatal("save roundtrip")
						}
						s = &restored
					}
				}
				if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 10 {
					t.Fatal("no valid victory", s.Round, s.Phase)
				}
			})
		}
	}
}
