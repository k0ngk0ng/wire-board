package game

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

// A fixed, approved random 63-hex map, not an official fixed-board recipe.
// Replaying all nineteen placements keeps response fixtures reproducible.
func fishWorldExtendedGame(t *testing.T, n int) *State {
	t.Helper()
	data, err := os.ReadFile("testdata/catan-new-world-five-six.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Layout       CatanNewWorldMap `json:"layout"`
		PortEdges    []int            `json:"portEdges"`
		FishVertices []int            `json:"fishVertices"`
		FishNumbers  []int            `json:"fishNumbers"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	s, err := NewCatanFishingNewWorld(n, CatanOptions{FiveSix: true}, &fixture.Layout)
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
		t.Fatal("approved layout changed")
	}
	return s
}

func TestCatanFishingNewWorldExtendedComponentsAndGates(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := fishWorldExtendedGame(t, n)
		g := s.Catan
		terrain, numbers, ports, cards, fish := make([]int, 8), make([]int, 13), make([]int, 6), make([]int, 5), make([]int, 4)
		for _, tile := range g.Tiles {
			terrain[tile.Resource]++
			numbers[tile.Number]++
		}
		for _, port := range g.Ports {
			ports[port.Resource+1]++
		}
		for _, card := range g.DevDeck {
			cards[card]++
		}
		for _, id := range g.Fishing.Tokens.DrawPile {
			fish[catanFishValue(id)]++
		}
		numbers[0] = 0
		if !slices.Equal(terrain, []int{7, 7, 7, 7, 7, 3, 21, 4}) || !slices.Equal(numbers, []int{0, 0, 2, 3, 4, 5, 5, 0, 5, 5, 4, 4, 2}) ||
			!slices.Equal(ports, []int{5, 1, 1, 2, 1, 1}) || !slices.Equal(cards, []int{20, 3, 3, 3, 5}) || !slices.Equal(fish, []int{1, 15, 15, 13}) {
			t.Fatal("extended printed inventory", terrain, numbers, ports, cards, fish)
		}
		if len(g.Tiles) != 63 || len(g.Fishing.Map.Grounds) != 8 || len(g.Fishing.Map.Lakes) != 0 || g.Robber != -1 || g.Seafarers.Pirate != -1 || g.Paired == nil {
			t.Fatal("extended setup")
		}
		fleetSupply(t, g)
		layout := g.NewWorldMap()
		for _, options := range []CatanOptions{{}, {FiveSix: true, AllHelpers: true}} {
			if _, err := NewCatanFishingNewWorld(n, options, layout); err == nil {
				t.Fatal("incompatible options accepted", options)
			}
		}
		for _, scenario := range []string{"islands", "desert", "tribe", "cloth", "pirate_islands"} {
			copy := clone(*s)
			copy.Catan.Seafarers.Scenario = scenario
			if copy.Catan.fishingSeaSupported() {
				t.Fatal("unverified extended recipe enabled", scenario)
			}
		}
		copy := clone(*s)
		copy.Catan.CitiesKnights = &CatanCitiesKnights{}
		if copy.Catan.fishingSeaSupported() {
			t.Fatal("unverified three-expansion combination enabled")
		}
		for _, change := range []func(*Catan){
			func(g *Catan) { g.Options.FiveSix = false },
			func(g *Catan) { g.Paired = nil },
			func(g *Catan) { g.Fishing.WorldSetup.Numbers = g.Fishing.WorldSetup.Numbers[:6] },
			func(g *Catan) { g.Fishing.WorldSetup.Numbers[0] = 4 },
			func(g *Catan) { g.newWorld().Ports = g.newWorld().Ports[:10] },
		} {
			copy := clone(*s)
			change(copy.Catan)
			if copy.Catan.validateFishing() == nil {
				t.Fatal("corrupt extended inventory accepted")
			}
		}
	}
}

func TestCatanFishingNewWorldDuplicateGroundProduction(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, number := range []int{5, 9} {
			for _, full := range []bool{false, true} {
				for _, blocked := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d/roll%d/full%v/pirate%v", n, number, full, blocked), func(t *testing.T) {
						s := fishWorldExtendedGame(t, n)
						g := s.Catan
						s.Turn, s.Phase = n-1, "catan_roll"
						g.SetupStep, g.TurnSerial, g.RollID = g.SetupLimit(), 1, 1
						g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = n-1, (n+2)%n, false
						for p := range g.Fishing.Started {
							g.Fishing.Started[p] = true
						}
						// Each doubled face has one inner-sea ground and one frame ground.
						// These nonadjacent buildings belong to different players, with a city
						// testing two tokens; seat zero must respond before seat one after n-1.
						for _, ground := range g.Fishing.Map.Grounds {
							if ground.Number == number {
								p, level := 0, 1
								if ground.SeaTile != nil {
									p, level = 1, 2
									if blocked {
										g.Seafarers.Pirate = *ground.SeaTile
									}
								}
								v := ground.Vertices[0]
								if !g.canSettlement(p, v, true) {
									t.Fatal("fixture building distance")
								}
								g.Vertices[v].Owner, g.Vertices[v].Level = p, level
							}
						}
						s.catanScores()
						if full {
							fishOwn(&g.Fishing.Tokens, 0, 0, 1, 2, 3, 4, 5, 6)
							fishOwn(&g.Fishing.Tokens, 1, 11, 12, 13, 14, 15, 16, 17)
						}
						fishTop(&g.Fishing.Tokens, 21, 22, 23)
						due, err := g.Fishing.Map.production(g, number)
						want := make([]int, n)
						want[0] = 1
						if !blocked {
							want[1] = 2
						}
						if err != nil || !slices.Equal(due, want) {
							t.Fatal("duplicate-ground aggregation/pirate", due, err)
						}
						pile := len(g.Fishing.Tokens.DrawPile)
						if err = s.catanRollProduction(number); err != nil {
							t.Fatal(err)
						}
						responses := 0
						for s.Phase == "catan_fish_replace" {
							p := s.CatanPendingActor()
							if p != responses {
								t.Fatal("clockwise fish response order", p, responses)
							}
							saved := clone(*s)
							if !reflect.DeepEqual(saved, *s) {
								t.Fatal("response restore")
							}
							s = &saved
							helperReject(t, s, (p+1)%n, Action{Type: "catan_fish_keep"})
							helperReject(t, s, p, Action{Type: "catan_roll"})
							helperApply(t, s, p, Action{Type: "catan_fish_replace", Card: s.Catan.Fishing.Tokens.Hands[p][0]})
							responses++
						}
						wantDraws := sum(want)
						if full {
							wantDraws = 1
							if !blocked {
								wantDraws = 2
							}
							if responses != wantDraws {
								t.Fatal("replacement cap per player")
							}
						}
						if pile-len(s.Catan.Fishing.Tokens.DrawPile) != wantDraws {
							t.Fatal("duplicated or missing fish draws")
						}
						// Any gold on this approved map finishes through the ordinary response path.
						for s.Phase == "catan_gold" {
							a, err := s.BotAction(s.CatanPendingActor())
							if err != nil {
								t.Fatal(err)
							}
							helperApply(t, s, s.CatanPendingActor(), a)
						}
						if s.Phase != "catan_turn" || s.Turn != n-1 || s.CatanPendingActor() != -1 {
							t.Fatal("production continuation")
						}
						before := clone(*s)
						if err = s.catanRollProduction(number); err == nil || !reflect.DeepEqual(before, *s) {
							t.Fatal("repeated production not rejected atomically")
						}
						tokens := clone(s.Catan.Fishing.Tokens)
						helperApply(t, s, s.Turn, Action{Type: "catan_end"})
						if s.Turn != (n+2)%n || s.Phase != "catan_turn" || !s.Catan.Paired.Second || !reflect.DeepEqual(tokens, s.Catan.Fishing.Tokens) || s.Catan.RollID != 1 {
							t.Fatal("secondary action produced fish")
						}
						helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
						fleetSupply(t, s.Catan)
					})
				}
			}
		}
	}
}

func TestCatanFishingNewWorldPairedShipDevAndBoot(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := fishWorldExtendedGame(t, n)
		g := s.Catan
		s.Turn, s.Phase = 0, "catan_turn"
		g.StartPlayer, g.SetupStep, g.TurnSerial = 0, g.SetupLimit(), 1
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, false
		for p := range g.Fishing.Started {
			g.Fishing.Started[p] = true
		}
		v := g.Fishing.Map.Grounds[0].Vertices[0]
		g.Vertices[v].Owner, g.Vertices[v].Level = 3, 1
		other := g.Fishing.Map.Grounds[2].Vertices[0]
		if !g.canSettlement(0, other, true) {
			t.Fatal("fixture distance")
		}
		g.Vertices[other].Owner, g.Vertices[other].Level = 0, 1
		s.catanScores()
		fishOwn(&g.Fishing.Tokens, 3, 0, 11, 12, 21, 22, 23, 24)
		at := slices.Index(g.Fishing.Tokens.DrawPile, catanFishBoot)
		g.Fishing.Tokens.DrawPile = slices.Delete(g.Fishing.Tokens.DrawPile, at, at+1)
		g.Fishing.Tokens.BootOwner = 3
		hand := clone(g.Fishing.Tokens)
		helperApply(t, s, 0, Action{Type: "catan_end"})
		if s.Turn != 3 || !s.Catan.Paired.Second || s.Phase != "catan_turn" || !reflect.DeepEqual(hand, s.Catan.Fishing.Tokens) {
			t.Fatal("second phase handoff")
		}
		helperReject(t, s, 3, Action{Type: "catan_roll"})
		helperGrant(s, 3, []int{1, 0, 0, 0, 0})
		helperReject(t, s, 3, Action{Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}})
		g = s.Catan
		edge := -1
		for _, e := range g.Edges {
			if g.canShip(3, e.ID) {
				edge = e.ID
				break
			}
		}
		if edge < 0 {
			t.Fatal("fixture ship")
		}
		resources := slices.Clone(g.Players[3].Resources)
		helperApply(t, s, 3, Action{Type: "catan_fish_ship", Edge: edge, Tokens: []int{11, 21}})
		if !s.Catan.Edges[edge].Ship || s.Catan.Edges[edge].Owner != 3 || !slices.Equal(resources, s.Catan.Players[3].Resources) {
			t.Fatal("paired fish ship")
		}
		helperApply(t, s, 3, Action{Type: "catan_fish_boot", Target: 0})
		if s.Catan.Fishing.Tokens.BootOwner != 0 || s.Catan.victoryTargetFor(0) != 13 || s.Catan.victoryTargetFor(3) != 12 {
			t.Fatal("paired boot")
		}
		g = s.Catan
		at = slices.Index(g.DevDeck, 2)
		g.DevDeck[at], g.DevDeck[len(g.DevDeck)-1] = g.DevDeck[len(g.DevDeck)-1], g.DevDeck[at]
		helperApply(t, s, 3, Action{Type: "catan_fish_dev", Tokens: []int{0, 22, 23}})
		helperReject(t, s, 3, Action{Type: "catan_dev", Card: 2, Take: []int{0, 1, 0, 0, 1}})
		if s.Catan.Players[3].NewDev[2] != 1 {
			t.Fatal("new development restriction")
		}
		saved := clone(*s)
		if !reflect.DeepEqual(saved, *s) {
			t.Fatal("paired state restore")
		}
		s = &saved
		helperApply(t, s, 3, Action{Type: "catan_end"})
		// Advance intervening primary actions without rolling. This fixture isolates
		// action-phase aging, while complete bot games cover natural production.
		for s.Turn != 3 {
			s.Phase = "catan_turn"
			helperApply(t, s, s.Turn, Action{Type: "catan_end"})
		}
		if s.Catan.Paired.Second || s.Phase != "catan_roll" || s.Catan.Players[3].NewDev[2] != 0 {
			t.Fatal("dev did not age from secondary to primary")
		}
		helperApply(t, s, 3, Action{Type: "catan_dev", Card: 2, Take: []int{0, 1, 0, 0, 1}})
		if s.Catan.Players[3].Dev[2] != 0 || !s.Catan.PlayedDev || s.Phase != "catan_roll" {
			t.Fatal("aged fish development cannot play")
		}
		fleetSupply(t, s.Catan)
	}
}

func TestCatanFishingNewWorldExtendedBootVictory(t *testing.T) {
	for _, n := range []int{5, 6} {
		for _, second := range []bool{false, true} {
			for _, boot := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/second%v/boot%v", n, second, boot), func(t *testing.T) {
					s := fishWorldExtendedGame(t, n)
					g := s.Catan
					actor := 0
					if second {
						actor = 3
					}
					other := (actor + 1) % n
					s.Turn, s.Phase = actor, "catan_turn"
					g.StartPlayer, g.SetupStep, g.TurnSerial = 0, g.SetupLimit(), 1
					g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, second
					// Explicit late-game position with legal distance and physical inventory:
					// each player has four cities and three settlements (eleven public VP).
					for _, p := range []int{actor, other} {
						count := 0
						for _, v := range g.Vertices {
							if g.canSettlement(p, v.ID, true) {
								level := 1
								if count < 4 {
									level = 2
								}
								g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = p, level
								count++
								if count == 7 {
									break
								}
							}
						}
						if count != 7 {
							t.Fatal("fixture lacks legal building sites")
						}
					}
					s.catanScores()
					if g.Players[actor].Score != 11 || g.Players[other].Score != 11 {
						t.Fatal("fixture scores")
					}
					fishOwn(&g.Fishing.Tokens, actor, 0, 21, 22)
					if boot {
						at := slices.Index(g.Fishing.Tokens.DrawPile, catanFishBoot)
						g.Fishing.Tokens.DrawPile = slices.Delete(g.Fishing.Tokens.DrawPile, at, at+1)
						g.Fishing.Tokens.BootOwner = actor
					}
					at := slices.Index(g.DevDeck, 4)
					g.DevDeck[at], g.DevDeck[len(g.DevDeck)-1] = g.DevDeck[len(g.DevDeck)-1], g.DevDeck[at]
					helperApply(t, s, actor, Action{Type: "catan_fish_dev", Tokens: []int{0, 21, 22}})
					if s.Catan.Players[actor].Score != 12 || s.Finished == boot {
						t.Fatal("boot-aware private point victory")
					}
					if boot {
						saved := clone(*s)
						s = &saved
						helperReject(t, s, other, Action{Type: "catan_fish_boot", Target: actor})
						helperApply(t, s, actor, Action{Type: "catan_fish_boot", Target: other})
					}
					if !s.Finished || !slices.Equal(s.Winners, []int{actor}) || s.Catan.Players[actor].Score != 12 {
						t.Fatal("immediate phase victory")
					}
					fleetSupply(t, s.Catan)
				})
			}
		}
	}
}
