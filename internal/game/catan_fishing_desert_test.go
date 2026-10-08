package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishDesertGame(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanFishingSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCatanFishingDesertPrintedRecipes(t *testing.T) {
	for _, n := range []int{3, 4} {
		for range 24 {
			s := fishDesertGame(t, n)
			g := s.Catan
			r, err := g.fishingDesertRecipe()
			if err != nil {
				t.Fatal(err)
			}
			if err = g.validateFishing(); err != nil {
				t.Fatal(err)
			}
			if g.victoryTargetFor(0) != 14 || g.Seafarers.IslandBonus != 2 || g.Tiles[g.Robber].Resource != CatanDesert || g.Seafarers.Pirate != -1 {
				t.Fatal("scenario setup changed")
			}
			if g.Tiles[r.lake].Resource != catanLake || g.Tiles[r.lake].Number != 0 || len(g.Fishing.Map.Grounds) != 6 {
				t.Fatal("missing lake/grounds")
			}
			if !g.tileProduces(g.Tiles[r.recipient], r.moved) || !g.tileProduces(g.Tiles[r.recipient], r.original) {
				t.Fatal("missing relocated disc")
			}
			if !slices.Equal(g.Seafarers.Islands, g.findLandRegions(true)) {
				t.Fatal("lake changed desert regions")
			}
			copy := clone(*s)
			if !reflect.DeepEqual(*s, copy) || copy.Catan.validateFishing() != nil {
				t.Fatal("restore")
			}
			// The transformed lake cannot be applied twice or mutate an existing save.
			before := clone(*s)
			if _, err = g.makeFishingDesert(nil); err == nil || !reflect.DeepEqual(before, *s) {
				t.Fatal("double transformation")
			}
			raw, err := NewCatanSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert", Layout: "fixed"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			old := clone(raw.Catan.Tiles)
			placements := []CatanFishingGroundPlacement{}
			for i, ground := range g.Fishing.Map.Grounds {
				placements = append(placements, CatanFishingGroundPlacement{g.Fishing.Map.Grounds[(i+1)%6].Number, [2]int{ground.Edges[1], ground.Edges[0]}})
			}
			f, err := raw.Catan.makeFishingDesert(placements)
			if err != nil {
				t.Fatal("host placement", err)
			}
			if err = f.validate(raw.Catan); err != nil {
				t.Fatal(err)
			}
			for i, tile := range raw.Catan.Tiles {
				if i == r.lake {
					continue
				}
				if !reflect.DeepEqual(tile, old[i]) {
					t.Fatal("changed unrelated terrain/number")
				}
			}
			fishTop(&g.Fishing.Tokens, catanFishBoot)
			due := make([]int, n)
			due[0] = 1
			if err = g.Fishing.Tokens.beginDraw(0, due); err != nil {
				t.Fatal(err)
			}
			if g.victoryTargetFor(0) != 15 || g.victoryTargetFor(1) != 14 {
				t.Fatal("boot goal")
			}
		}
	}
}
func TestCatanFishingDesertRejectsCorruptAndUnsupported(t *testing.T) {
	for _, change := range []func(*Catan){
		func(g *Catan) { g.Fishing.Map.ExtraNumbers = nil },
		func(g *Catan) { g.Fishing.Map.ExtraNumbers[0].Number = 7 },
		func(g *Catan) { g.Fishing.Map.ExtraNumbers[0].Tile = -1 },
		func(g *Catan) { g.Fishing.Map.Lakes[0].Tile = -1 },
		func(g *Catan) { g.Fishing.Map.Lakes[0].Numbers = []int{4, 10} },
		func(g *Catan) { g.Tiles[g.Fishing.Map.Lakes[0].Tile].Number = 2 },
		func(g *Catan) { g.Tiles[g.Fishing.Map.ExtraNumbers[0].Tile].Number = 3 },
		func(g *Catan) { g.Ports[0].Edge = g.Fishing.Map.Grounds[0].Edges[0] },
		func(g *Catan) { g.Fishing.Map.Grounds[0] = g.Fishing.Map.Grounds[1] },
		func(g *Catan) { g.Seafarers.Islands[0] = g.Seafarers.StartIslands[0] },
		func(g *Catan) { g.Seafarers.Variable = true },
	} {
		s := fishDesertGame(t, 3)
		change(s.Catan)
		if err := s.Catan.validateFishing(); err == nil {
			t.Fatal("corruption accepted")
		}
	}
	for _, n := range []int{3, 4, 5, 6} {
		layout := "variable"
		if _, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "desert", Layout: layout}, nil); err == nil {
			t.Fatal("unverified recipe accepted", n, layout)
		}
	}
	for _, s := range []*State{fishingGame(t, 3), fishSeaGame(t, 3, "fixed"), fishFogGame(t, 3, "fixed")} {
		s.Catan.Fishing.Map.ExtraNumbers = []catanFishingExtraNumber{{Tile: 0, Number: 2}}
		if err := s.Catan.validateFishing(); err == nil {
			t.Fatal("extra disc accepted by unrelated scenario")
		}
	}
	s, err := NewCatanSeafarers(3, CatanOptions{}, CatanSeafarersSetup{Scenario: "desert"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := clone(*s)
	if _, err = s.Catan.makeFishingDesert([]CatanFishingGroundPlacement{}); err == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("rejected placement changed board")
	}
}
func TestCatanFishingDesertBothNumbersLakeAndRobber(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, blocked := range []string{"none", "ordinary", "lake"} {
			for _, which := range []string{"original", "moved", "unrelated"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, blocked, which), func(t *testing.T) {
					s := fishDesertGame(t, n)
					g := s.Catan
					r, _ := g.fishingDesertRecipe()
					s.Phase, s.Turn = "catan_roll", 0
					g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
					g.Robber = -1
					for i := range g.Tiles {
						if i != r.recipient && g.Tiles[i].Resource < 5 {
							g.Tiles[i].Number = 5
						}
					}
					v := g.Tiles[r.recipient].Vertices
					g.Vertices[v[0]].Owner, g.Vertices[v[0]].Level = 0, 1
					g.Vertices[v[3]].Owner, g.Vertices[v[3]].Level = 1, 2
					lv := g.Tiles[r.lake].Vertices[0]
					g.Vertices[lv].Owner, g.Vertices[lv].Level = 2, 2
					fishOwn(&g.Fishing.Tokens, 2, 0, 1, 2, 3, 4, 5, 6)
					fishTop(&g.Fishing.Tokens, 21, 22)
					if blocked == "ordinary" {
						g.Robber = r.recipient
					}
					if blocked == "lake" {
						g.Robber = r.lake
					}
					roll := r.original
					if which == "moved" {
						roll = r.moved
					}
					if which == "unrelated" {
						roll = 4
					}
					if err := s.catanRollProduction(roll); err != nil {
						t.Fatal(err)
					}
					for p, want := range []int{1, 2} {
						if blocked == "ordinary" || which == "unrelated" {
							want = 0
						}
						if sum(g.Players[p].Resources) != want || g.Players[p].Resources[r.recipientResource] != want {
							t.Fatal("wrong ordinary payout", p, g.Players[p].Resources, want)
						}
					}
					if sum(g.Players[2].Resources) != 0 {
						t.Fatal("lake produced ordinary resource")
					}
					fishDue := blocked != "lake" && which != "unrelated"
					if fishDue {
						if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 2 {
							t.Fatal("missing lake responder")
						}
						copy := clone(*s)
						s = &copy
						if err := s.Apply(2, Action{Type: "catan_fish_keep"}); err != nil {
							t.Fatal(err)
						}
					}
					if s.Phase != "catan_turn" || s.Turn != 0 || s.Catan.Fishing.Pending != nil {
						t.Fatal("lost production turn")
					}
					fleetSupply(t, s.Catan)
				})
			}
		}
	}
}
func TestCatanFishingDesertStartingFishAndBotOdds(t *testing.T) {
	for _, n := range []int{3, 4} {
		s := fishDesertGame(t, n)
		g := s.Catan
		r, _ := g.fishingDesertRecipe()
		s.Turn = 0
		g.SetupStep = n
		v := g.Tiles[r.lake].Vertices[0]
		fishTop(&g.Fishing.Tokens, 0)
		if err := s.Apply(0, Action{Type: "catan_settlement", Vertex: v}); err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		if !slices.Equal(g.Fishing.Tokens.Hands[0], []int{0}) || s.Phase != "catan_setup_road" {
			t.Fatal("lake starting entitlement")
		}
		want := make([]int, 5)
		for _, tile := range g.Tiles {
			if tile.Resource < 5 && slices.Contains(tile.Vertices, v) {
				want[tile.Resource]++
			}
		}
		if !slices.Equal(want, g.Players[0].Resources) {
			t.Fatal("lake granted removed resource")
		}
		recipient := g.Tiles[r.recipient]
		odds := (6 - absCatan(7-r.original)) + (6 - absCatan(7-r.moved))
		if g.tileNumberWeight(recipient) != odds {
			t.Fatal("bot omits second disc")
		}
		boosted := g.vertexValue(0, recipient.Vertices[0])
		before := clone(g.Fishing.Map.ExtraNumbers)
		g.Fishing.Map.ExtraNumbers = nil
		if boosted <= g.vertexValue(0, recipient.Vertices[0]) {
			t.Fatal("settlement bot ignores relocated number")
		}
		g.Fishing.Map.ExtraNumbers = before
		for _, viewer := range []int{-1, 0, 1} {
			public := s.View(viewer)["catan"].(map[string]any)["fishing"].(map[string]any)
			encoded, _ := json.Marshal(public["map"])
			expected, _ := json.Marshal(g.Fishing.Map)
			if string(encoded) != string(expected) {
				t.Fatal("public disc metadata missing")
			}
		}
	}
}
func TestCatanFishingDesertBotsFinishAndConserve(t *testing.T) {
	for _, n := range []int{3, 4} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishDesertGame(t, n)
			paid := 0
			for step := 0; step < 8000 && !s.Finished; step++ {
				p := s.Turn
				if actor := s.CatanPendingActor(); actor >= 0 {
					p = actor
				} else if s.Phase == "catan_discard" {
					for actor, due := range s.Catan.DiscardDue {
						if due > 0 {
							p = actor
							break
						}
					}
				}
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if _, ok := catanFishCosts[a.Type]; ok {
					paid++
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				if err = s.Catan.validateFishing(); err != nil {
					t.Fatal(err)
				}
				fleetSupply(t, s.Catan)
				for p := range s.Catan.Players {
					roads, settlements, cities := s.Catan.pieces(p)
					if roads > 15 || settlements > 5 || cities > 4 || s.Catan.shipCount(p) > 15 {
						t.Fatal("piece inventory")
					}
				}
				if step%31 == 0 {
					copy := clone(*s)
					if !reflect.DeepEqual(copy, *s) {
						t.Fatal("restore")
					}
					s = &copy
				}
			}
			if !s.Finished || paid == 0 || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
				t.Fatal("incomplete game", s.Round, paid)
			}
			t.Logf("round=%d fish-actions=%d", s.Round, paid)
		})
	}
}
