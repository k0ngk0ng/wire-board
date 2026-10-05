package game

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

type fishWorldGoldLayout struct {
	Layout       CatanNewWorldMap `json:"layout"`
	GoldTiles    []int            `json:"goldTiles"`
	PortEdges    []int            `json:"portEdges"`
	FishVertices []int            `json:"fishVertices"`
	FishNumbers  []int            `json:"fishNumbers"`
}

// The fixture is a valid custom map and complete placement plan, not a saved
// fabricated production result. Every port/ground goes through actual Apply.
func fishWorldGoldGame(t *testing.T, n int) (*State, fishWorldGoldLayout) {
	t.Helper()
	data, err := os.ReadFile("testdata/catan-new-world-gold.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture fishWorldGoldLayout
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if err = ValidateCatanNewWorldMap(n, &fixture.Layout); err != nil {
		t.Fatal(err)
	}
	s, err := NewCatanFishingNewWorld(n, CatanOptions{}, &fixture.Layout)
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.Fishing.WorldSetup.Numbers = slices.Clone(fixture.FishNumbers)
	for _, edge := range fixture.PortEdges {
		helperApply(t, s, s.Turn, Action{Type: "catan_world_port", Edge: edge})
	}
	for _, vertex := range fixture.FishVertices {
		helperApply(t, s, s.Turn, Action{Type: "catan_world_fish", Vertex: vertex})
	}
	if !reflect.DeepEqual(s.Catan.NewWorldMap(), &fixture.Layout) || s.Phase != "catan_setup_settlement" {
		t.Fatal("approved gold map changed")
	}
	return s, fixture
}

func fishWorldGoldProduction(t *testing.T, n int, full, scarce bool, block string) (*State, fishWorldGoldLayout) {
	t.Helper()
	s, fixture := fishWorldGoldGame(t, n)
	g := s.Catan
	s.Turn, s.Phase = 0, "catan_roll"
	g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
	for p := range g.Players {
		g.Fishing.Started[p] = true
	}
	ground := g.Fishing.Map.Grounds[0]
	// Both endpoints can legally hold buildings, unlike adjacent middle/end
	// vertices. Each endpoint touches exactly one of the two 9-point gold hexes.
	g.Vertices[ground.Vertices[0]].Owner, g.Vertices[ground.Vertices[0]].Level = 1, 2
	if !g.canSettlement(2, ground.Vertices[2], true) {
		t.Fatal("fixture buildings violate distance rule")
	}
	g.Vertices[ground.Vertices[2]].Owner, g.Vertices[ground.Vertices[2]].Level = 2, 1
	g.Players[1].Score, g.Players[2].Score = 2, 1
	// Tile14 is an ordinary 9-point field, away from both gold buildings.
	if g.Tiles[14].Resource != 3 || g.Tiles[14].Number != 9 {
		t.Fatal("ordinary grain fixture changed")
	}
	ordinary := -1
	for _, v := range g.Tiles[14].Vertices {
		if g.canSettlement(0, v, true) {
			ordinary = v
			break
		}
	}
	if ordinary < 0 {
		t.Fatal("missing separate ordinary producer")
	}
	g.Vertices[ordinary].Owner, g.Vertices[ordinary].Level = 0, 1
	g.Players[0].Score = 1
	if full {
		fishOwn(&g.Fishing.Tokens, 1, 0, 1, 2, 3, 4, 5, 6)
		fishOwn(&g.Fishing.Tokens, 2, 11, 12, 13, 14, 15, 16, 17)
		fishTop(&g.Fishing.Tokens, catanFishBoot, 21, 22)
	} else {
		fishTop(&g.Fishing.Tokens, 21, 22, 23)
	}
	if scarce {
		for c := range g.Bank {
			left := 0
			if c == 4 {
				left = 1
			}
			g.Players[0].Resources[c] += g.Bank[c] - left
			g.Bank[c] = left
		}
	}
	if block == "pirate" {
		g.Seafarers.Pirate = *ground.SeaTile
	}
	if block == "robber" {
		g.Robber = fixture.GoldTiles[0]
	}
	fleetSupply(t, g)
	return s, fixture
}

func TestCatanFishingNewWorldGoldProductionOrderAndShortage(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, full := range []bool{false, true} {
			for _, scarce := range []bool{false, true} {
				for _, block := range []string{"none", "pirate", "robber"} {
					t.Run(fmt.Sprintf("%d/full=%v/scarce=%v/%s", n, full, scarce, block), func(t *testing.T) {
						s, fixture := fishWorldGoldProduction(t, n, full, scarce, block)
						before := make([][]int, n)
						for p, seat := range s.Catan.Players {
							before[p] = slices.Clone(seat.Resources)
						}
						if err := s.catanRollProduction(9); err != nil {
							t.Fatal(err)
						}
						g := s.Catan
						if !scarce {
							before[0][3]++
						}
						for p := range before {
							if !slices.Equal(before[p], g.Players[p].Resources) {
								t.Fatal("ordinary production not paid exactly once before responses", p)
							}
						}
						fishResponders := []int{}
						if full && block != "pirate" {
							fishResponders = []int{1, 2}
						}
						for _, actor := range fishResponders {
							if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != actor || s.Catan.GoldPending != nil {
								t.Fatal("fish must precede gold", actor, s.Phase)
							}
							restored := clone(*s)
							s = &restored
							helperReject(t, s, 0, Action{Type: "catan_fish_keep"})
							helperReject(t, s, actor, Action{Type: "catan_gold", Take: []int{0, 0, 0, 1, 0}})
							action := Action{Type: "catan_fish_keep"}
							if actor == 1 {
								action = Action{Type: "catan_fish_replace", Card: 0}
							}
							helperApply(t, s, actor, action)
						}
						expected := []int{1, 2}
						if block == "robber" {
							expected = []int{2}
						}
						if scarce {
							expected = expected[:1]
						}
						for _, actor := range expected {
							if s.Phase != "catan_gold" || s.CatanPendingActor() != actor || s.Catan.Fishing.Pending != nil {
								t.Fatal("gold response order", actor, s.Phase)
							}
							count := 3 - actor
							if scarce {
								count = 1
							}
							claim := s.Catan.GoldPending.Claims[0]
							if claim.Count != 3-actor {
								t.Fatal("original gold entitlement lost")
							}
							restored := clone(*s)
							s = &restored
							helperReject(t, s, 0, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, count}})
							helperReject(t, s, actor, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, count + 1}})
							helperReject(t, s, actor, Action{Type: "catan_fish_keep"})
							helperApply(t, s, actor, Action{Type: "catan_gold", Take: []int{0, 0, 0, 0, count}})
							before[actor][4] += count
						}
						g = s.Catan
						if s.Phase != "catan_turn" || s.Turn != 0 || s.CatanPendingActor() != -1 || g.GoldPending != nil || g.Fishing.Pending != nil || g.Fishing.LastRollID != 1 {
							t.Fatal("production failed to resume once")
						}
						for p := range before {
							if !slices.Equal(before[p], g.Players[p].Resources) {
								t.Fatal("gold lost or ordinary resources duplicated", p)
							}
						}
						if full && block != "pirate" {
							if g.Fishing.Tokens.BootOwner != 1 || len(g.Fishing.Tokens.Hands[1]) != 6 || len(g.Fishing.Tokens.Hands[2]) != 7 || g.victoryTargetFor(1) != 13 {
								t.Fatal("replacement boot/keep semantics")
							}
						} else if !full {
							a, b := 2, 1
							if block == "pirate" {
								a, b = 0, 0
							}
							if len(g.Fishing.Tokens.Hands[1]) != a || len(g.Fishing.Tokens.Hands[2]) != b {
								t.Fatal("wrong fish production")
							}
						}
						if !reflect.DeepEqual(g.NewWorldMap(), &fixture.Layout) || g.validateFishing() != nil {
							t.Fatal("approved map/grounds changed")
						}
						saved := clone(*s)
						if s.catanRollProduction(9) == nil || !reflect.DeepEqual(*s, saved) {
							t.Fatal("same roll paid twice")
						}
						fleetSupply(t, g)
					})
				}
			}
		}
	}
}

func TestCatanFishingNewWorldGoldSecondSettlement(t *testing.T) {
	for _, n := range []int{3, 4} {
		s, fixture := fishWorldGoldGame(t, n)
		target := s.Catan.Fishing.Map.Grounds[0].Vertices[0]
		for s.Catan.SetupStep < n {
			if s.Phase == "catan_setup_settlement" {
				g := s.Catan
				pick := -1
				for _, v := range g.Vertices {
					if !g.canSettlement(s.Turn, v.ID, true) || v.ID == target {
						continue
					}
					adjacent := false
					for _, e := range g.Edges {
						if e.A == v.ID && e.B == target || e.B == v.ID && e.A == target {
							adjacent = true
						}
					}
					if !adjacent {
						pick = v.ID
						break
					}
				}
				if pick < 0 {
					t.Fatal("could not preserve golden starting site")
				}
				helperApply(t, s, s.Turn, Action{Type: "catan_settlement", Vertex: pick})
			} else {
				s.AutoCatanPending()
			}
		}
		actor := s.Turn
		g := s.Catan
		fishTop(&g.Fishing.Tokens, 21)
		pile := len(g.Fishing.Tokens.DrawPile)
		helperApply(t, s, actor, Action{Type: "catan_settlement", Vertex: target})
		g = s.Catan
		if s.Phase != "catan_gold" || g.GoldPending.Claims[0].Count != 1 || g.GoldPending.Resume != "catan_setup_road" || len(g.Fishing.Tokens.DrawPile) != pile-1 || !g.Fishing.Started[actor] {
			t.Fatal("gold second settlement fish/resource entitlement")
		}
		before := slices.Clone(g.Players[actor].Resources)
		restored := clone(*s)
		s = &restored
		helperApply(t, s, actor, Action{Type: "catan_gold", Take: []int{0, 0, 0, 1, 0}})
		before[3]++
		if s.Phase != "catan_setup_road" || s.Turn != actor || s.Catan.SetupStep != n || !slices.Equal(before, s.Catan.Players[actor].Resources) {
			t.Fatal("gold setup continuation")
		}
		s.AutoCatanPending()
		if s.Catan.SetupStep != n+1 || s.Catan.Fishing.Tokens.Hands[actor][0] != 21 || len(s.Catan.Fishing.Tokens.Hands[actor]) != 1 || !reflect.DeepEqual(s.Catan.NewWorldMap(), &fixture.Layout) {
			t.Fatal("road step duplicated starting fish or changed map")
		}
		fleetSupply(t, s.Catan)
	}
}
