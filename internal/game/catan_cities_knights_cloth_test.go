package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Public creation remains closed; this exercises the internal constructor.
func ckClothFixture(t *testing.T, n int, layout string) *State {
	t.Helper()
	s, err := NewCatanCitiesKnightsSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "cloth", Layout: layout}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatanCitiesKnightsClothSetupFoundation(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed"}
		if n < 5 {
			layouts = append(layouts, "variable")
		}
		for _, layout := range layouts {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := ckClothFixture(t, n, layout)
				g := s.Catan
				origin := g.CitiesKnights.RobberStart
				pirate := g.CitiesKnights.PirateStart
				if layout == "variable" {
					starts := g.clothStartTiles()
					origin = starts[len(starts)-1]
					helperReject(t, s, (s.Turn+1)%n, Action{Type: "catan_cloth_start", Tile: origin})
					helperApply(t, s, s.Turn, Action{Type: "catan_cloth_start", Tile: origin})
				}
				for step := 0; step < 3*n; step++ {
					g = s.Catan
					if g.Robber != -1 || g.Seafarers.Pirate != -1 || g.CitiesKnights.RobberStart != origin {
						t.Fatal("bandits woke early or origin lost")
					}
					actor := (g.StartPlayer + step) % n
					want := "catan_settlement"
					if step >= n && step < 2*n {
						actor = (g.StartPlayer + 2*n - 1 - step) % n
						want = "catan_city"
					}
					if s.Turn != actor || g.SetupStep != step {
						t.Fatal("incorrect setup order", step, s.Turn)
					}
					a, err := s.BotAction(actor)
					if err != nil || a.Type != want {
						t.Fatal("wrong setup building", step, a, err)
					}
					expected := make([]int, 8)
					if step >= 2*n {
						for _, tile := range g.Tiles {
							if tile.Resource < 5 && slices.Contains(tile.Vertices, a.Vertex) {
								expected[tile.Resource]++
							}
						}
					}
					helperApply(t, s, actor, a)
					if !reflect.DeepEqual(s.Catan.Players[actor].Resources, expected) {
						t.Fatal("resources must come only from third building", step, s.Catan.Players[actor].Resources, expected)
					}
					s.AutoCatanPending()
					if s.Catan.SetupStep != step+1 {
						t.Fatal("route timeout did not finish exactly one setup group")
					}
					saved := clone(*s)
					if !reflect.DeepEqual(*s, saved) {
						t.Fatal("setup persistence")
					}
					s = &saved
				}
				g = s.Catan
				if g.setup() || s.Phase != "catan_roll" || s.Turn != g.StartPlayer {
					t.Fatal("setup did not finish")
				}
				for p := range g.Players {
					_, settlements, cities := g.pieces(p)
					if settlements != 2 || cities != 1 || g.Players[p].Score != 4 {
						t.Fatal("expected settlement, city, settlement", p, settlements, cities)
					}
				}
				s.catanFinishBarbarians()
				if g.Robber != origin || g.Seafarers.Pirate != pirate {
					t.Fatal("first invasion did not use saved bandit origins")
				}
				if g.pirateAllowed(s.Turn) {
					t.Fatal("pirate requires a cloth trade relation even after invasion")
				}
				ckProgressStock(t, g)
				ckKnightStock(t, g)
			})
		}
	}
}

func TestCatanCitiesKnightsClothEndingFoundation(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, auto := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/auto=%t", n, auto), func(t *testing.T) {
				s := ckClothFixture(t, n, "fixed")
				g := s.Catan
				g.SetupStep = g.SetupLimit()
				s.Phase = "catan_turn"
				p := s.Turn
				c := g.cloth()
				// Preserve all physical cloth while constructing an end condition.
				for i := 0; i < c.EmptyLimit; i++ {
					c.Held[p] += c.Villages[i].Stock
					c.Villages[i].Stock = 0
				}
				total := clothTotal(g)
				ckProgressGive(t, s, p, 0, 1, 2, 3, 4)
				s.catanScores()
				helperApply(t, s, p, Action{Type: "catan_end"})
				if s.Finished || s.Phase != "catan_progress_end" || s.Turn != p {
					t.Fatal("must finish progress discard before cloth ending")
				}
				saved := clone(*s)
				s = &saved
				helperReject(t, s, (p+1)%n, Action{Type: "catan_progress_discard", Cards: []int{0}})
				if auto {
					s.AutoCatanPending()
				} else {
					helperApply(t, s, p, Action{Type: "catan_progress_discard", Cards: []int{0}})
				}
				if !s.Finished || s.Phase != "finished" || !reflect.DeepEqual(s.Winners, []int{p}) || s.Turn != p {
					t.Fatal("discard skipped cloth ending or advanced paired turn", s.Phase, s.Winners)
				}
				if clothTotal(s.Catan) != total || len(s.Catan.CitiesKnights.Players[p].Progress) != 4 {
					t.Fatal("cloth/progress lost")
				}
				ckProgressStock(t, s.Catan)
			})
		}
	}
	for _, score := range []int{14, 15, 16} {
		s := ckClothFixture(t, 3, "fixed")
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		s.Phase = "catan_turn"
		g.Players[s.Turn].Score = score
		s.catanVictory()
		if s.Finished != (score >= 16) {
			t.Fatal("combined points threshold", score)
		}
	}
}

func TestCatanCitiesKnightsClothAutomaticOriginStaysDormant(t *testing.T) {
	for _, n := range []int{3, 4} {
		s := ckClothFixture(t, n, "variable")
		origin := s.Catan.CitiesKnights.RobberStart
		saved := clone(*s)
		s = &saved
		s.AutoCatanPending()
		if s.Phase != "catan_setup_settlement" || s.Catan.SetupStep != 0 || s.Catan.Robber != -1 || s.Catan.CitiesKnights.RobberStart != origin {
			t.Fatal("automatic origin choice must preserve dormancy", n, s.Phase)
		}
	}
}

// Deliberately corrupted accounting state: four traders cannot all retain a
// closed route into a degree-three village. This verifies the error guard,
// NOT the reachability of a shortage in real play. See the small-map bound and
// the still-unresolved duplicated production numbers on the 5/6-player board.
func TestCatanClothCorruptCommonSupplyProtection(t *testing.T) {
	for _, n := range []int{4, 6} {
		s := ckClothFixture(t, n, "fixed")
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		c := g.cloth()
		total := clothTotal(g)
		c.Stock = 1
		for p := 0; p < 4; p++ {
			c.Held[p] = 7
		}
		for i := 0; i < 4; i++ {
			c.Villages[i].Stock = 0
			c.Villages[i].Traders = []int{0, 1, 2, 3}
		}
		c.Villages[3].Stock = 1
		if clothTotal(g) != total || s.catanClothEnd() {
			t.Fatal("corrupt fixture must conserve cloth and precede ending")
		}
		before := clone(*s)
		if err := s.catanProduceCloth(c.Villages[3].Number); err == nil {
			t.Fatal("unverified supply shortage was silently accepted")
		}
		if !reflect.DeepEqual(*s, before) {
			t.Fatal("failed first production should not consume cloth")
		}
	}
}

func TestCatanCitiesKnightsClothBotsComplete(t *testing.T) {
	clothAcrossGames := 0
	for _, n := range []int{3, 4, 5, 6} {
		layouts := []string{"fixed"}
		if n < 5 {
			layouts = append(layouts, "variable")
		}
		for _, layout := range layouts {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s := ckClothFixture(t, n, layout)
				total, steps := clothTotal(s.Catan), 0
				for ; steps < 10000 && !s.Finished; steps++ {
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(steps, s.Phase, err)
					}
					if err := s.Apply(actor, a); err != nil {
						t.Fatal(steps, s.Phase, a, err)
					}
					g := s.Catan
					ckProgressStock(t, g)
					ckKnightStock(t, g)
					if clothTotal(g) != total || g.cloth().Stock < 0 || g.LongestOwner != -1 || g.ArmyOwner != -1 || len(g.DevDeck) != 0 {
						t.Fatal("incorrect cloth/award/development components")
					}
					for p := range g.Players {
						if g.shipCount(p) > 15 || g.cityPiecesLeft(p) < 0 || g.settlementPiecesLeft(p) < 0 {
							t.Fatal("piece inventory")
						}
					}
					for _, knight := range g.CitiesKnights.Knights {
						if g.clothVillageAt(knight.Vertex) {
							t.Fatal("knight occupied village")
						}
					}
					if steps%53 == 0 {
						saved := clone(*s)
						if !reflect.DeepEqual(*s, saved) {
							t.Fatal("persistence")
						}
						s = &saved
					}
				}
				g := s.Catan
				if !s.Finished || len(s.Winners) == 0 || g.CitiesKnights.Invasions == 0 {
					t.Fatal("incomplete or unexercised combined cloth game", steps, s.Phase)
				}
				clothAcrossGames += sum(g.cloth().Held)
				if g.Players[s.Turn].Score >= 16 {
					if !reflect.DeepEqual(s.Winners, []int{s.Turn}) {
						t.Fatal("own-turn points victory has priority")
					}
				} else {
					empty, best, held := 0, -1, -1
					for _, v := range g.cloth().Villages {
						if v.Stock == 0 {
							empty++
						}
					}
					winners := []int{}
					for p, player := range g.Players {
						if player.Eliminated {
							continue
						}
						if player.Score > best || (player.Score == best && g.cloth().Held[p] > held) {
							best, held, winners = player.Score, g.cloth().Held[p], []int{p}
						} else if player.Score == best && g.cloth().Held[p] == held {
							winners = append(winners, p)
						}
					}
					if empty < g.cloth().EmptyLimit || !reflect.DeepEqual(s.Winners, winners) {
						t.Fatal("wrong depletion winners", empty, winners, s.Winners)
					}
				}
				t.Logf("steps=%d invasions=%d cloth=%d supply=%d winners=%v", steps, g.CitiesKnights.Invasions, sum(g.cloth().Held), g.cloth().Stock, s.Winners)
			})
		}
	}
	// A legal city/progress-heavy win need not collect cloth, particularly on
	// a variable map. Require scenario exercise across the full matrix instead.
	if clothAcrossGames == 0 {
		t.Fatal("full matrix never exercised cloth trade/production")
	}
}
