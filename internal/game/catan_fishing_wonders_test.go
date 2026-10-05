package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishWondersGame(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "wonders", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanFishingWondersMapAndSetup(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed", "variable"}
		if n > 4 {
			layouts = []string{"fixed"}
		}
		for _, layout := range layouts {
			for range 12 {
				s := fishWondersGame(t, n, layout)
				g := s.Catan
				if err := g.validateFishing(); err != nil {
					t.Fatal(err)
				}
				wantWonders := 5
				if n > 4 {
					wantWonders = 7
				}
				if s.Phase != "catan_wonders_start" || len(g.Fishing.Map.Lakes) != 0 || g.Seafarers.Pirate != -1 || g.victoryTargetFor(0) != 10 || g.Seafarers.IslandBonus != 1 || len(g.wonders().Cards) != wantWonders {
					t.Fatal("scenario rules changed")
				}
				coasts, err := g.fishingWondersCoasts()
				if err != nil || !reflect.DeepEqual(coasts, g.FishingCoasts()) {
					t.Fatal("coast listing", err)
				}
				islands := map[int]bool{}
				for _, c := range coasts {
					islands[c.Island] = true
				}
				wantCoastalIslands := 3
				if n > 4 {
					wantCoastalIslands = 2
				} // Single-hex islets have no concave V.
				if len(islands) != wantCoastalIslands {
					t.Fatal("coastal island eligibility", len(islands))
				}
				before := clone(*g)
				places := []CatanFishingGroundPlacement{}
				for i, ground := range g.Fishing.Map.Grounds {
					places = append(places, CatanFishingGroundPlacement{g.Fishing.Map.Grounds[(i+1)%len(g.Fishing.Map.Grounds)].Number, [2]int{ground.Edges[1], ground.Edges[0]}})
				}
				f, err := g.makeFishingWonders(places)
				if err != nil || f.validate(g) != nil || !reflect.DeepEqual(before, *g) {
					t.Fatal("custom map mutation", err)
				}
				for _, v := range g.wonders().SetupBlocked {
					if g.canSettlement(0, v, true) {
						t.Fatal("fishing removed a wonder setup restriction")
					}
				}
				for _, tile := range g.Tiles {
					if g.Seafarers.Islands[tile.ID] >= 0 && g.Seafarers.Islands[tile.ID] != g.Seafarers.StartIslands[0] {
						for _, v := range tile.Vertices {
							if g.canSettlement(0, v, true) {
								t.Fatal("small-island initial building allowed")
							}
						}
					}
				}
			}
			// Complete real bot setup while verifying that only the second
			// settlement starts fish production. Keep the boot for separate tests.
			s := fishWondersGame(t, n, layout)
			helperApply(t, s, s.Turn, Action{Type: "catan_wonders_start", Tile: s.Catan.wonderStartTiles()[0]})
			for step := 0; step < 2*n; step++ {
				g, p := s.Catan, s.Turn
				fishTop(&g.Fishing.Tokens, step)
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(err)
				}
				expected := 0
				if step >= n {
					for _, ground := range g.Fishing.Map.Grounds {
						if slices.Contains(ground.Vertices[:], a.Vertex) {
							expected = 1
						}
					}
				}
				helperApply(t, s, p, a)
				if len(s.Catan.Fishing.Tokens.Hands[p]) != expected || s.Catan.Fishing.Started[p] != (step >= n) {
					t.Fatal("wrong initial entitlement", step)
				}
				s.AutoCatanPending()
				copy := clone(*s)
				s = &copy
			}
			if s.Phase != "catan_roll" || s.Catan.SetupStep != 2*n {
				t.Fatal("setup continuation")
			}
		}
	}
	for _, mutate := range []func(*Catan){
		func(g *Catan) { g.Seafarers.Pirate = 2 },
		func(g *Catan) { g.Seafarers.StartIslands[0] = -1 },
		func(g *Catan) { g.Fishing.Map.Grounds[0].Number = 7 },
		func(g *Catan) { g.Fishing.Map.Grounds[1] = g.Fishing.Map.Grounds[0] },
		func(g *Catan) { g.Fishing.Map.Lakes = []catanFishingLake{{Tile: 0, Numbers: []int{2, 3, 11, 12}}} },
		func(g *Catan) { g.Fishing.Map.ExtraNumbers = []catanFishingExtraNumber{{Tile: 0, Number: 2}} },
		func(g *Catan) { g.Ports[0].Edge = g.Fishing.Map.Grounds[0].Edges[0] },
		func(g *Catan) {
			g.Robber = slices.IndexFunc(g.Tiles, func(v CatanTile) bool { return v.Resource == CatanSea })
		},
	} {
		s := fishWondersGame(t, 3, "fixed")
		mutate(s.Catan)
		if s.Catan.validateFishing() == nil {
			t.Fatal("corrupt map accepted")
		}
	}
}

// Put the first ground on a real small-island gold coast. The others may
// be anywhere legal; the official combination does not prescribe a split.
func fishWondersGoldGame(t *testing.T, n int, layout string) (*State, int) {
	t.Helper()
	s := fishWondersGame(t, n, layout)
	g := s.Catan
	coasts, err := g.fishingWondersCoasts()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range coasts {
		for _, tile := range g.Tiles {
			if tile.Resource != CatanGold || !slices.Contains(tile.Vertices, c.Vertices[1]) {
				continue
			}
			numbers := catanFishingGroundNumbers(n)
			total := len(numbers)
			numbers = slices.DeleteFunc(numbers, func(n int) bool { return n == tile.Number })
			places := []CatanFishingGroundPlacement{{tile.Number, c.Edges}}
			used := map[int]bool{c.Edges[0]: true, c.Edges[1]: true}
			for _, other := range coasts {
				if len(places) == total {
					break
				}
				if used[other.Edges[0]] || used[other.Edges[1]] {
					continue
				}
				places = append(places, CatanFishingGroundPlacement{numbers[len(places)-1], other.Edges})
				used[other.Edges[0]], used[other.Edges[1]] = true, true
			}
			if len(places) != total {
				continue
			}
			m, err := g.makeFishingWonders(places)
			if err != nil {
				t.Fatal(err)
			}
			g.Fishing.Map = *m
			return s, tile.ID
		}
	}
	t.Fatal("no eligible small-island gold coast")
	return nil, -1
}

func TestCatanFishingWondersGoldAndFishResponses(t *testing.T) {
	for _, n := range []int{3, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) { testFishingWondersGold(t, n) })
	}
}
func testFishingWondersGold(t *testing.T, n int) {
	layouts := []string{"fixed", "variable"}
	if n > 4 {
		layouts = []string{"fixed"}
	}
	for _, layout := range layouts {
		for _, full := range []bool{false, true} {
			s, gold := fishWondersGoldGame(t, n, layout)
			g := s.Catan
			s.Turn, s.Phase = 0, "catan_roll"
			if n > 4 {
				g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
			}
			g.SetupStep, g.TurnSerial, g.RollID, g.Robber = g.SetupLimit(), 1, 1, -1
			ground := g.Fishing.Map.Grounds[0]
			g.Vertices[ground.Vertices[1]].Owner, g.Vertices[ground.Vertices[1]].Level = 0, 2
			other := -1
			for _, v := range ground.Vertices {
				if !slices.Contains(g.Tiles[gold].Vertices, v) {
					other = v
				}
			}
			if other < 0 {
				t.Fatal("missing nongold endpoint")
			}
			g.Vertices[other].Owner, g.Vertices[other].Level = 1, 1
			if full {
				fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
				fishOwn(&g.Fishing.Tokens, 1, 11, 12, 13, 14, 15, 16, 17)
			}
			fishTop(&g.Fishing.Tokens, 21, 22, 23)
			if err := s.catanRollProduction(ground.Number); err != nil {
				t.Fatal(err)
			}
			resources := clone(s.Catan.Players[0].Resources)
			if full {
				for _, p := range []int{0, 1} {
					if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != p {
						t.Fatal("fish/gold sequence", s.Phase, s.CatanPendingActor())
					}
					copy := clone(*s)
					s = &copy
					helperReject(t, s, 2, Action{Type: "catan_fish_keep"})
					helperApply(t, s, p, Action{Type: "catan_fish_keep"})
				}
			}
			if s.Phase != "catan_gold" || s.CatanPendingActor() != 0 || s.Catan.GoldPending.Claims[0].Count != 2 {
				t.Fatal("lost gold production", s.Phase)
			}
			copy := clone(*s)
			s = &copy
			helperApply(t, s, 0, Action{Type: "catan_gold", Take: []int{0, 0, 0, 2, 0}})
			resources[3] += 2
			if s.Phase != "catan_turn" || s.Catan.Fishing.Pending != nil || s.Catan.GoldPending != nil || !slices.Equal(resources, s.Catan.Players[0].Resources) {
				t.Fatal("incorrect production resume")
			}
			fleetSupply(t, s.Catan)
		}
	}
}

func eachFishingWonderAction(t *testing.T, run func(*testing.T, *State)) {
	t.Helper()
	for _, n := range []int{3, 5, 6} {
		for _, secondary := range []bool{false, true} {
			if secondary && n < 5 {
				continue
			}
			for _, phase := range []string{"catan_roll", "catan_turn"} {
				if secondary && phase == "catan_roll" {
					continue
				}
				t.Run(fmt.Sprintf("%d/%s/secondary%v", n, phase, secondary), func(t *testing.T) {
					s := fishWondersGame(t, n, "fixed")
					s.Turn, s.Phase = 0, phase
					g := s.Catan
					g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
					if n > 4 {
						g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
						if secondary {
							g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = n-3, 0, true
						}
					}
					run(t, s)
				})
			}
		}
	}
}

func TestCatanFishingWondersShipAndNoPirate(t *testing.T) {
	eachFishingWonderAction(t, func(t *testing.T, s *State) {
		g := s.Catan
		phase := s.Phase
		v := g.Fishing.Map.Grounds[0].Vertices[1]
		g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
		fishOwn(&g.Fishing.Tokens, 0, 0, 11, 12, 21)
		if g.fishCanRemovePirate(0) || slices.Contains(s.catanFishLegal(0)["actions"].([]string), "catan_fish_pirate") {
			t.Fatal("pirate action exists in no-pirate scenario")
		}
		helperReject(t, s, 0, Action{Type: "catan_fish_pirate", Tokens: []int{12}})
		edge := -1
		for _, e := range g.Edges {
			if g.canShip(0, e.ID) {
				edge = e.ID
				break
			}
		}
		if edge < 0 {
			t.Fatal("missing coastal ship")
		}
		pile, bank := slices.Clone(g.Fishing.Tokens.DrawPile), slices.Clone(g.Bank)
		helperApply(t, s, 0, Action{Type: "catan_fish_ship", Edge: edge, Tokens: []int{11, 21}})
		g = s.Catan
		if s.Phase != phase || g.Edges[edge].Owner != 0 || !g.Edges[edge].Ship || !slices.Equal(pile, g.Fishing.Tokens.DrawPile) || !slices.Equal(bank, g.Bank) || len(g.shipDestinations(0, edge)) != 0 {
			t.Fatal("ship paid resources, drew fish or could move immediately")
		}
	})
}

func TestCatanFishingWondersBootVictoryPaths(t *testing.T) {
	for _, boot := range []bool{false, true} {
		for _, tc := range []struct {
			score, own, other int
			win               bool
		}{
			{10, 0, 0, false}, {10, 1, 1, false}, {10, 2, 1, !boot},
			{11, 2, 1, true}, {11, 2, 2, false}, {2, 4, 3, true},
		} {
			eachFishingWonderAction(t, func(t *testing.T, s *State) {
				g := s.Catan
				g.wonders().Cards[0].Owner, g.wonders().Cards[0].Level = 0, tc.own
				g.wonders().Cards[1].Owner, g.wonders().Cards[1].Level = 1, tc.other
				g.Players[0].Score = tc.score
				if boot {
					fishTop(&g.Fishing.Tokens, catanFishBoot)
					if err := g.Fishing.Tokens.beginDraw(0, append([]int{1}, make([]int, len(g.Players)-1)...)); err != nil {
						t.Fatal(err)
					}
				}
				s.catanVictory()
				if s.Finished != tc.win {
					t.Fatal("wrong wonder/boot victory", boot, tc)
				}
			})
		}
	}
}

func TestCatanFishingWondersPassingBootCanWin(t *testing.T) {
	eachFishingWonderAction(t, func(t *testing.T, s *State) {
		g := s.Catan
		g.wonders().Cards[0].Owner, g.wonders().Cards[0].Level = 0, 2
		g.wonders().Cards[1].Owner, g.wonders().Cards[1].Level = 1, 1
		g.Players[0].Score, g.Players[1].Score = 10, 9
		fishTop(&g.Fishing.Tokens, catanFishBoot)
		if err := g.Fishing.Tokens.beginDraw(0, append([]int{1}, make([]int, len(g.Players)-1)...)); err != nil {
			t.Fatal(err)
		}
		s.catanVictory()
		if s.Finished {
			t.Fatal("ten points with boot won early")
		}
		helperReject(t, s, 0, Action{Type: "catan_fish_boot", Target: 1})
		s.Catan.Players[1].Score = 10
		copy := clone(*s)
		s = &copy
		helperApply(t, s, 0, Action{Type: "catan_fish_boot", Target: 1})
		if !s.Finished || !slices.Equal(s.Winners, []int{0}) || s.Catan.Fishing.Tokens.BootOwner != 1 || s.Catan.wonders().Cards[0].Level != 2 {
			t.Fatal("passing boot did not lower wonder score threshold")
		}
	})
}

func TestCatanFishingWondersBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed", "variable"}
		if n > 4 {
			layouts = []string{"fixed"}
		}
		for _, layout := range layouts {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := fishWondersGame(t, n, layout)
				paid := 0
				for step := 0; step < 10000 && !s.Finished; step++ {
					actor := s.Turn
					if p := s.CatanPendingActor(); p >= 0 {
						actor = p
					} else if s.Phase == "catan_discard" {
						for p, count := range s.Catan.DiscardDue {
							if count > 0 {
								actor = p
								break
							}
						}
					}
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if _, ok := catanFishCosts[a.Type]; ok {
						paid++
					}
					if err = s.Apply(actor, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					g := s.Catan
					if err := g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					fleetSupply(t, g)
					cards := len(g.DevDeck) + len(g.DevDiscard)
					for p, seat := range g.Players {
						cards += sum(seat.Dev)
						roads, villages, cities := g.pieces(p)
						if roads > 15 || villages > 5 || cities > 4 || g.shipCount(p) > 15 {
							t.Fatal("piece supply")
						}
					}
					wantCards := 25
					if n > 4 {
						wantCards = 34
					}
					if cards != wantCards {
						t.Fatal("development supply")
					}
					if step%37 == 0 {
						copy := clone(*s)
						s = &copy
					}
				}
				if !s.Finished || len(s.Winners) != 1 || !s.Catan.wonderVictory(s.Winners[0]) {
					t.Fatal("did not finish under wonder rules")
				}
				t.Logf("round=%d fish-actions=%d winning-level=%d score=%d", s.Round, paid, s.Catan.wonders().Cards[s.Catan.wonderOwned(s.Winners[0])].Level, s.Catan.Players[s.Winners[0]].Score)
			})
		}
	}
}
