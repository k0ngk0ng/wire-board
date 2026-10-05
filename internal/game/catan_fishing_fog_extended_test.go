package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanFishingFogExtendedInventoryAndGates(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := fishFogGame(t, n, "fixed")
		g := s.Catan
		terrain, numbers := fogInventory(g)
		fish, cards, groundNumbers := make([]int, 4), make([]int, 5), []int{}
		for _, id := range g.Fishing.Tokens.DrawPile {
			fish[catanFishValue(id)]++
		}
		for _, card := range g.DevDeck {
			cards[card]++
		}
		for _, ground := range g.Fishing.Map.Grounds {
			groundNumbers = append(groundNumbers, ground.Number)
		}
		slices.Sort(groundNumbers)
		// Seafarers extension p.6, T&B extension p.5, combination p.2.
		if !slices.Equal(terrain, []int{7, 7, 7, 7, 7, 1, 17, 3}) ||
			!slices.Equal(numbers, []int{0, 0, 3, 4, 4, 4, 4, 0, 4, 4, 4, 4, 3}) ||
			!slices.Equal(fish, []int{1, 15, 15, 13}) ||
			!slices.Equal(cards, []int{20, 3, 3, 3, 5}) ||
			!slices.Equal(groundNumbers, []int{4, 5, 5, 6, 8, 9, 9, 10}) {
			t.Fatal("printed inventory", terrain, numbers, fish, cards, groundNumbers)
		}
		if len(g.Tiles) != 56 || len(g.Seafarers.Fog.StartTiles) != 24 || len(g.Seafarers.Fog.Terrain) != 18 || len(g.Seafarers.Fog.Numbers) != 14 || len(g.Ports) != 11 || g.Paired == nil || g.Tiles[g.Robber].Resource != 3 || g.Tiles[g.Robber].Number != 12 {
			t.Fatal("extended map recipe")
		}
		fleetSupply(t, g)
		for _, change := range []func(*Catan){
			func(g *Catan) { g.Options.FiveSix = false },
			func(g *Catan) { g.Paired = nil },
			func(g *Catan) { g.Seafarers.Variable = true },
			func(g *Catan) { g.Fishing.Map.Grounds = g.Fishing.Map.Grounds[:6] },
			func(g *Catan) { g.Fishing.Map.Grounds[1].Number = 4 },
			func(g *Catan) { g.Options.Helpers = true },
			func(g *Catan) { g.CitiesKnights = &CatanCitiesKnights{} },
		} {
			copy := clone(*s)
			change(copy.Catan)
			if err := copy.Catan.validateFishing(); err == nil {
				t.Fatal("corrupt extended recipe accepted")
			}
		}
		if _, err := NewCatanFishingSeafarers(n, CatanOptions{FiveSix: true}, CatanSeafarersSetup{Scenario: "fog", Layout: "variable"}, nil); err == nil {
			t.Fatal("unverified variable layout accepted")
		}
		// The layout must not depend on either hidden stack, even for a bot.
		copy := clone(*s)
		slices.Reverse(copy.Catan.Seafarers.Fog.Terrain)
		slices.Reverse(copy.Catan.Seafarers.Fog.Numbers)
		for viewer := -1; viewer < n; viewer++ {
			if !reflect.DeepEqual(s.View(viewer), copy.View(viewer)) {
				t.Fatal("hidden exploration order leaked")
			}
		}
		if !reflect.DeepEqual(g.FishingCoasts(), copy.Catan.FishingCoasts()) {
			t.Fatal("coast derived from hidden order")
		}
	}
}

func TestCatanFishingFogExtendedStartingEntitlement(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := fishFogGame(t, n, "fixed")
		awarded := 0
		for s.Catan.setup() {
			g, actor := s.Catan, s.Turn
			a, err := s.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			second := g.SetupStep >= n
			if second && s.Phase == "catan_setup_settlement" {
				found := false
				for _, ground := range g.Fishing.Map.Grounds {
					for _, v := range ground.Vertices {
						if g.canSettlement(actor, v, true) {
							a = Action{Type: "catan_settlement", Vertex: v}
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			want := 0
			if second && a.Type == "catan_settlement" {
				for _, ground := range g.Fishing.Map.Grounds {
					if slices.Contains(ground.Vertices[:], a.Vertex) {
						want = 1 // Even if two grounds touch this settlement.
					}
				}
			}
			pile := len(g.Fishing.Tokens.DrawPile)
			helperApply(t, s, actor, a)
			if pile-len(s.Catan.Fishing.Tokens.DrawPile) != want {
				t.Fatal("starting fish missing, repeated or awarded in first round")
			}
			if a.Type == "catan_settlement" && (s.Phase != "catan_setup_road" || s.Turn != actor || s.Catan.Fishing.Started[actor] != second) {
				t.Fatal("starting entitlement or route continuation")
			}
			awarded += want
			copy := clone(*s)
			if !reflect.DeepEqual(*s, copy) {
				t.Fatal("setup restore")
			}
			s = &copy
		}
		if awarded == 0 || s.Phase != "catan_roll" || s.Catan.Paired.Second {
			t.Fatal("setup coverage/first primary turn")
		}
		for _, started := range s.Catan.Fishing.Started {
			if !started {
				t.Fatal("second-settlement entitlement left unconsumed")
			}
		}
		fleetSupply(t, s.Catan)
	}
}

// The buildings and hands are declared midgame fixtures on the real map, not
// a claimed full legal game. All production and response transitions are real.
func TestCatanFishingFogExtendedDuplicateProduction(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, number := range []int{5, 9} {
			for _, full := range []bool{false, true} {
				for _, blocked := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d/roll%d/full%v/pirate%v", n, number, full, blocked), func(t *testing.T) {
						s := fishFogGame(t, n, "fixed")
						g := s.Catan
						s.Turn, s.Phase = n-1, "catan_roll"
						g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
						g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = n-1, (n+2)%n, false
						g.Robber = -1
						for p := range g.Fishing.Started {
							g.Fishing.Started[p] = true
						}
						grounds := []catanFishingGround{}
						for _, ground := range g.Fishing.Map.Grounds {
							if ground.Number == number {
								grounds = append(grounds, ground)
							}
						}
						if len(grounds) != 2 {
							t.Fatal("missing doubled face")
						}
						for p, ground := range grounds {
							placed := false
							for _, vertex := range ground.Vertices {
								if g.canSettlement(p, vertex, true) {
									g.Vertices[vertex].Owner, g.Vertices[vertex].Level = p, 2-p
									placed = true
									break
								}
							}
							if !placed {
								t.Fatal("no distance-legal production fixture")
							}
						}
						blockedPlayer := -1
						if blocked {
							for p, ground := range grounds {
								if ground.SeaTile != nil {
									g.Seafarers.Pirate = *ground.SeaTile
									blockedPlayer = p
									break
								}
							}
							if blockedPlayer < 0 {
								t.Fatal("fixture has no pirate-blockable ground")
							}
						}
						want := make([]int, n)
						want[0], want[1] = 2, 1
						if blockedPlayer >= 0 {
							want[blockedPlayer] = 0
						}
						due, err := g.Fishing.Map.production(g, number)
						if err != nil || !slices.Equal(due, want) {
							t.Fatal("doubled production/pirate", due, want, err)
						}
						if full {
							fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
							fishOwn(&g.Fishing.Tokens, 1, 11, 12, 13, 14, 15, 16, 17)
						}
						fishTop(&g.Fishing.Tokens, 21, 22, 23)
						pile, responses := len(g.Fishing.Tokens.DrawPile), 0
						if err = s.catanRollProduction(number); err != nil {
							t.Fatal(err)
						}
						for p, amount := range want {
							if !full || amount == 0 {
								continue
							}
							if s.Phase != "catan_fish_replace" || s.CatanPendingActor() != p {
								t.Fatal("response order/cap")
							}
							copy := clone(*s)
							s = &copy
							helperReject(t, s, (p+1)%n, Action{Type: "catan_fish_keep"})
							helperApply(t, s, p, Action{Type: "catan_fish_replace", Card: s.Catan.Fishing.Tokens.Hands[p][0]})
							responses++
						}
						wantDraws := sum(want)
						if full {
							wantDraws = responses
						}
						if pile-len(s.Catan.Fishing.Tokens.DrawPile) != wantDraws || s.Phase != "catan_turn" || s.Turn != n-1 {
							t.Fatal("wrong payout/continuation")
						}
						before := clone(*s)
						if err = s.catanRollProduction(number); err == nil || !reflect.DeepEqual(before, *s) {
							t.Fatal("production repeated")
						}
						tokens := clone(s.Catan.Fishing.Tokens)
						helperApply(t, s, s.Turn, Action{Type: "catan_end"})
						if s.Turn != (n+2)%n || s.Phase != "catan_turn" || !s.Catan.Paired.Second || !reflect.DeepEqual(tokens, s.Catan.Fishing.Tokens) || s.Catan.RollID != 1 {
							t.Fatal("secondary action produced fish")
						}
						helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
						fleetSupply(t, s.Catan)
						if err := s.Catan.validateFishing(); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
