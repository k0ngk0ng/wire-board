package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishClothGame(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := NewCatanFishingSeafarers(n, CatanOptions{}, CatanSeafarersSetup{Scenario: "cloth", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestCatanFishingClothMapGroupsAndVariants(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			for range 12 {
				s := fishClothGame(t, n, layout)
				g := s.Catan
				if err := g.validateFishing(); err != nil {
					t.Fatal(err)
				}
				if len(g.Fishing.Map.Lakes) != 0 || g.SetupLimit() != n*3 || g.victoryTargetFor(0) != 14 || g.Seafarers.IslandBonus != 0 || g.Seafarers.Pirate != -1 || clothTotal(g) != 50 {
					t.Fatal("scenario setup changed")
				}
				coasts, groups, err := g.fishingClothCoasts()
				if err != nil {
					t.Fatal(err)
				}
				counts := map[int]int{}
				for _, ground := range g.Fishing.Map.Grounds {
					found := false
					for _, coast := range coasts {
						if ground.Edges == coast.Edges {
							found = true
							counts[coast.Island]++
						}
					}
					if !found {
						t.Fatal("ground outside main islands")
					}
				}
				if counts[groups[0]] != 3 || counts[groups[1]] != 3 {
					t.Fatal("not three per island")
				}
				if (s.Phase == "catan_cloth_start") != (layout == "variable") {
					t.Fatal("lost initial robber choice")
				}
				copy := clone(*s)
				if !reflect.DeepEqual(copy, *s) || copy.Catan.validateFishing() != nil {
					t.Fatal("restore")
				}
				places := []CatanFishingGroundPlacement{}
				for i, ground := range g.Fishing.Map.Grounds {
					places = append(places, CatanFishingGroundPlacement{g.Fishing.Map.Grounds[(i+1)%6].Number, [2]int{ground.Edges[1], ground.Edges[0]}})
				}
				before := clone(*g)
				f, err := g.makeFishingCloth(places)
				if err != nil || f.validate(g) != nil || !reflect.DeepEqual(before, *g) {
					t.Fatal("custom grounds or map purity", err)
				}
			}
		}
	}
	for _, change := range []func(*Catan){
		func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{Tile: 0, Numbers: []int{2, 3, 11, 12}}} },
		func(g *Catan) { g.Fishing.Map.ExtraNumbers = []catanFishingExtraNumber{{Tile: 0, Number: 2}} },
		func(g *Catan) { g.Fishing.Map.Grounds[0] = g.Fishing.Map.Grounds[1] },
		func(g *Catan) { g.Fishing.Map.Grounds[0].Number = 7 },
		func(g *Catan) { g.cloth().HomeTiles[0] = 14 },
		func(g *Catan) { g.Seafarers.StartIslands[1] = g.Seafarers.StartIslands[0] },
		func(g *Catan) { g.Ports[0].Edge = g.Fishing.Map.Grounds[0].Edges[0] },
		func(g *Catan) { g.Robber = 14 },
	} {
		s := fishClothGame(t, 3, "fixed")
		change(s.Catan)
		if s.Catan.validateFishing() == nil {
			t.Fatal("corruption accepted")
		}
	}
	for _, n := range []int{5, 6} {
		if _, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "cloth", Layout: "variable"}, nil); err == nil {
			t.Fatal("unverified 5/6 recipe accepted")
		}
	}
}
func TestCatanFishingClothThirdStartingSettlement(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			s := fishClothGame(t, n, layout)
			if s.Phase == "catan_cloth_start" {
				helperApply(t, s, s.Turn, Action{Type: "catan_cloth_start", Tile: s.Catan.clothStartTiles()[1]})
			}
			for step := 0; step < 3*n; step++ {
				g := s.Catan
				p := s.Turn
				fishTop(&g.Fishing.Tokens, step)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				expected := 0
				if step >= 2*n {
					for _, ground := range g.Fishing.Map.Grounds {
						if slices.Contains(ground.Vertices[:], a.Vertex) {
							expected = 1
						}
					}
				}
				helperApply(t, s, p, a)
				if len(s.Catan.Fishing.Tokens.Hands[p]) != expected || s.Catan.Fishing.Started[p] != (step >= 2*n) {
					t.Fatal("fish awarded before third settlement", step)
				}
				if step < 2*n && sum(s.Catan.Players[p].Resources) != 0 {
					t.Fatal("early starting resources")
				}
				s.AutoCatanPending()
				if s.Catan.SetupStep != step+1 {
					t.Fatal("setup route did not advance")
				}
			}
			if s.Phase != "catan_roll" {
				t.Fatal("setup never reached roll")
			}
			for p := range s.Catan.Players {
				_, villages, cities := s.Catan.pieces(p)
				if villages != 3 || cities != 0 || !s.Catan.Fishing.Started[p] {
					t.Fatal("starting pieces/entitlements")
				}
			}
		}
	}
}
func TestCatanFishingClothProductionPendingAndRestore(t *testing.T) {
	for _, layout := range []string{"fixed", "variable"} {
		for _, full := range []bool{false, true} {
			s := fishClothGame(t, 3, layout)
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_roll"
			g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
			g.Robber = -1
			ground := g.Fishing.Map.Grounds[0]
			roll := ground.Number
			v := ground.Vertices
			g.Vertices[v[0]].Owner, g.Vertices[v[0]].Level = 0, 1
			g.Vertices[v[2]].Owner, g.Vertices[v[2]].Level = 1, 2
			village := slices.IndexFunc(g.cloth().Villages, func(v CatanClothVillage) bool { return v.Number == roll })
			if village < 0 {
				t.Fatal("missing cloth production number")
			}
			g.cloth().Villages[village].Traders = []int{0, 1}
			if full {
				fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
				fishOwn(&g.Fishing.Tokens, 1, 11, 12, 13, 14, 15, 16, 17)
			}
			fishTop(&g.Fishing.Tokens, 21, 22, 23)
			if err := s.catanRollProduction(roll); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(g.cloth().Held, []int{1, 1, 0}) || g.cloth().Villages[village].Stock != 3 || clothTotal(g) != 50 {
				t.Fatal("cloth payout")
			}
			resources := []int{}
			for _, p := range g.Players {
				resources = append(resources, p.Resources...)
			}
			if full {
				if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != 0 {
					t.Fatal("missing first fish responder")
				}
				for _, p := range []int{0, 1} {
					copy := clone(*s)
					s = &copy
					helperReject(t, s, 2, Action{Type: "catan_fish_keep"})
					helperApply(t, s, p, Action{Type: "catan_fish_keep"})
				}
			}
			after := []int{}
			for _, p := range s.Catan.Players {
				after = append(after, p.Resources...)
			}
			if s.Phase != "catan_turn" || s.Catan.Fishing.Pending != nil || !slices.Equal(s.Catan.cloth().Held, []int{1, 1, 0}) || !slices.Equal(resources, after) || clothTotal(s.Catan) != 50 {
				t.Fatal("repaid production or lost turn")
			}
			fleetSupply(t, s.Catan)
		}
	}
}
func TestCatanFishingClothPirateRequiresTradeAndBlocksGround(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		s := fishClothGame(t, 3, "fixed")
		g := s.Catan
		s.Turn, s.Phase = 0, phase
		g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
		at := slices.IndexFunc(g.Fishing.Map.Grounds, func(v catanFishingGround) bool { return v.SeaTile != nil })
		if at < 0 {
			t.Fatal("missing sea ground")
		}
		ground := g.Fishing.Map.Grounds[at]
		g.Seafarers.Pirate = *ground.SeaTile
		g.Vertices[ground.Vertices[1]].Owner, g.Vertices[ground.Vertices[1]].Level = 0, 2
		fishOwn(&g.Fishing.Tokens, 0, 11)
		if g.fishCanRemovePirate(0) || slices.Contains(s.catanFishLegal(0)["actions"].([]string), "catan_fish_pirate") {
			t.Fatal("pirate allowed without village trade")
		}
		helperReject(t, s, 0, Action{Type: "catan_fish_pirate", Tokens: []int{11}})
		due, err := g.Fishing.Map.production(g, ground.Number)
		if err != nil || due[0] != 0 {
			t.Fatal("pirate did not block ground", err, due)
		}
		g.cloth().Villages[0].Traders = []int{0}
		g.cloth().Villages[0].Stock--
		g.cloth().Held[0]++
		// Existing trade relationships use the same permission as an ordinary pirate move.
		if !g.fishCanRemovePirate(0) || !slices.Contains(s.catanFishLegal(0)["actions"].([]string), "catan_fish_pirate") {
			t.Fatal("trade did not unlock fish pirate action")
		}
		before := clone(g.Players[0].Resources)
		robber := g.Robber
		helperApply(t, s, 0, Action{Type: "catan_fish_pirate", Tokens: []int{11}})
		g = s.Catan
		due, err = g.Fishing.Map.production(g, ground.Number)
		if err != nil || due[0] != 2 || g.Seafarers.Pirate != -1 || g.Robber != robber || s.Phase != phase || !slices.Equal(before, g.Players[0].Resources) || g.cloth().Held[0] != 1 {
			t.Fatal("pirate removal stole cloth/resources or failed unblock", err, due)
		}
	}
}
func TestCatanFishingClothBotsFinishAndConserve(t *testing.T) {
	for _, n := range []int{3, 4} {
		for _, layout := range []string{"fixed", "variable"} {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := fishClothGame(t, n, layout)
				paid := 0
				for step := 0; step < 10000 && !s.Finished; step++ {
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
					g := s.Catan
					if err = g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					fleetSupply(t, g)
					if clothTotal(g) != 50 {
						t.Fatal("cloth inventory")
					}
					cards := len(g.DevDeck) + len(g.DevDiscard)
					for p, seat := range g.Players {
						cards += sum(seat.Dev)
						roads, villages, cities := g.pieces(p)
						if roads > 15 || villages > 5 || cities > 4 || g.shipCount(p) > 15 {
							t.Fatal("piece inventory")
						}
					}
					if cards != 25 {
						t.Fatal("development inventory")
					}
					if step%31 == 0 {
						copy := clone(*s)
						if !reflect.DeepEqual(copy, *s) {
							t.Fatal("restore")
						}
						s = &copy
					}
				}
				if !s.Finished || paid == 0 || len(s.Winners) == 0 {
					t.Fatal("incomplete game", s.Round, paid)
				}
				t.Logf("round=%d fish-actions=%d cloth=%v", s.Round, paid, s.Catan.cloth().Held)
			})
		}
	}
}

func TestCatanFishingClothFishShipEstablishesTrade(t *testing.T) {
	for _, phase := range []string{"catan_roll", "catan_turn"} {
		for _, layout := range []string{"fixed", "variable"} {
			t.Run(phase+"/"+layout, func(t *testing.T) {
				s := fishClothGame(t, 3, layout)
				g := s.Catan
				s.Turn, s.Phase = 0, phase
				g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
				g.Seafarers.Pirate = -1
				root := g.Fishing.Map.Grounds[0].Vertices[0]
				goal := g.cloth().Villages[0].Vertex
				g.Vertices[root].Owner, g.Vertices[root].Level = 0, 1
				previous := map[int]int{root: -1}
				via := map[int]int{}
				queue := []int{root}
				found := false
				for len(queue) > 0 {
					v := queue[0]
					queue = queue[1:]
					if v == goal {
						found = true
						break
					}
					for _, id := range g.touching(v) {
						if !g.edgeTerrain(id, true) {
							continue
						}
						e := g.Edges[id]
						next := e.A
						if next == v {
							next = e.B
						}
						if _, ok := previous[next]; ok {
							continue
						}
						previous[next], via[next] = v, id
						queue = append(queue, next)
					}
				}
				if !found || previous[goal] < 0 {
					t.Fatal("no village water path")
				}
				last := via[goal]
				for v := previous[goal]; v != root; v = previous[v] {
					edge := via[v]
					g.Edges[edge].Owner, g.Edges[edge].Ship = 0, true
				}
				if !g.canShip(0, last) || g.pirateAllowed(0) {
					t.Fatal("last leg or early pirate")
				}
				fishOwn(&g.Fishing.Tokens, 0, 11, 21)
				helperApply(t, s, 0, Action{Type: "catan_fish_ship", Edge: last, Tokens: []int{11, 21}})
				g = s.Catan
				if !slices.Contains(g.cloth().Villages[0].Traders, 0) || g.cloth().Held[0] != 1 || g.cloth().Villages[0].Stock != 4 || !g.pirateAllowed(0) || s.Phase != phase {
					t.Fatal("fish ship did not establish trade or resume phase")
				}
				copy := clone(*s)
				s = &copy
				g = s.Catan
				for v := goal; v != root; v = previous[v] {
					if len(g.shipDestinations(0, via[v])) != 0 {
						t.Fatal("closed trade route can move")
					}
				}
				if !slices.Equal(g.Fishing.Tokens.Discard, []int{11, 21}) || clothTotal(g) != 50 {
					t.Fatal("payment/cloth inventory")
				}
			})
		}
	}
}

func TestCatanFishingClothBootBeforeProductionVictory(t *testing.T) {
	for _, boot := range []bool{false, true} {
		s := fishClothGame(t, 3, "fixed")
		g := s.Catan
		s.Turn, s.Phase = 0, "catan_roll"
		g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
		g.Robber = -1
		ground := g.Fishing.Map.Grounds[0]
		root := ground.Vertices[1]
		g.Vertices[root].Owner, g.Vertices[root].Level = 0, 2
		cities := 1
		for _, v := range g.Vertices {
			if cities == 4 {
				break
			}
			if g.canSettlement(0, v.ID, true) {
				g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = 0, 2
				cities++
			}
		}
		if cities != 4 {
			t.Fatal("need four legal cities")
		}
		for range 5 {
			at := slices.Index(g.DevDeck, 4)
			g.DevDeck = slices.Delete(g.DevDeck, at, at+1)
			g.Players[0].Dev[4]++
		}
		village := slices.IndexFunc(g.cloth().Villages, func(v CatanClothVillage) bool { return v.Number == ground.Number })
		g.cloth().Villages[village].Traders = []int{0}
		g.cloth().Villages[village].Stock--
		g.cloth().Held[0] = 1
		s.catanScores()
		if g.Players[0].Score != 13 {
			t.Fatal("expected near-win fixture")
		}
		fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
		token := 21
		if boot {
			token = catanFishBoot
		}
		fishTop(&g.Fishing.Tokens, token)
		if err := s.catanRollProduction(ground.Number); err != nil {
			t.Fatal(err)
		}
		if s.Finished || s.Phase != "catan_fish_replace" {
			t.Fatal("victory checked before pending fish/boot")
		}
		copy := clone(*s)
		s = &copy
		helperApply(t, s, 0, Action{Type: "catan_fish_replace", Card: 0})
		if s.Finished == boot || s.Catan.Players[0].Score != 14 || clothTotal(s.Catan) != 50 {
			t.Fatal("wrong fish/cloth victory ordering", boot, s.Phase, s.Catan.Players[0].Score)
		}
		if boot && (s.Catan.victoryTargetFor(0) != 15 || s.Phase != "catan_turn") {
			t.Fatal("boot target lost")
		}
	}
}
