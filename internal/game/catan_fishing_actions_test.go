package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func fishActionFixture(t *testing.T, n int, phase string) (*State, int) {
	t.Helper()
	s := fishingGame(t, n)
	g := s.Catan
	actor := 0
	if n > 4 {
		actor = 3
		g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 0, 3, true
		if phase == "catan_roll" {
			g.Paired.Primary, g.Paired.Secondary, g.Paired.Second = 3, 0, false
		}
	}
	g.SetupStep, g.TurnSerial = g.SetupLimit(), 1
	s.Turn, s.Phase = actor, phase
	g.Vertices[0].Owner, g.Vertices[0].Level = actor, 1
	g.Robber = g.Fishing.Map.Lakes[0].Tile
	fishOwn(&g.Fishing.Tokens, actor, 0, 1, 11, 12, 21, 22, 23)
	s.catanScores()
	return s, actor
}

func TestCatanFishingPaidRoadAndHiddenPointImmediateVictory(t *testing.T) {
	for _, kind := range []string{"road", "dev"} {
		s, p := fishActionFixture(t, 3, "catan_roll")
		g := s.Catan
		// Four cities supply eight points; this fixture isolates the fifth
		// connected road's award from the process of paying for the cities.
		for _, v := range []int{0, 10, 25, 40} {
			g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
		}
		a := Action{Type: "catan_fish_road", Tokens: []int{11, 21}}
		if kind == "road" {
			vertex := 0
			visited := map[int]bool{0: true}
			for step := 0; step < 5; step++ {
				found := false
				for _, id := range g.touching(vertex) {
					e := g.Edges[id]
					next := e.A
					if next == vertex {
						next = e.B
					}
					if visited[next] {
						continue
					}
					visited[next] = true
					vertex = next
					found = true
					if step < 4 {
						g.Edges[id].Owner = p
					} else {
						a.Edge = id
					}
					break
				}
				if !found {
					t.Fatal("could not create route fixture")
				}
			}
		} else {
			g.Vertices[45].Owner, g.Vertices[45].Level = p, 1
			at := slices.Index(g.DevDeck, 4)
			g.DevDeck[at], g.DevDeck[len(g.DevDeck)-1] = g.DevDeck[len(g.DevDeck)-1], g.DevDeck[at]
			a = Action{Type: "catan_fish_dev", Tokens: []int{0, 21, 22}}
		}
		s.catanScores()
		if err := s.Apply(p, a); err != nil {
			t.Fatal(err)
		}
		if !s.Finished || s.Catan.Players[p].Score != 10 || !slices.Equal(s.Winners, []int{p}) {
			t.Fatal("fish build did not settle award/victory", kind, s.Catan.Players[p].Score)
		}
	}
}

func fishLegalRoad(t *testing.T, g *Catan, player int) int {
	t.Helper()
	for _, e := range g.Edges {
		if g.canRoad(player, e.ID) {
			return e.ID
		}
	}
	t.Fatal("fixture has no road")
	return -1
}

func TestCatanFishingPaidEffectsAndBothTurnPhases(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, phase := range []string{"catan_roll", "catan_turn"} {
			for _, kind := range []string{"robber", "steal", "resource", "road", "dev"} {
				t.Run(fmt.Sprintf("%d/%s/%s", n, phase, kind), func(t *testing.T) {
					s, p := fishActionFixture(t, n, phase)
					g := s.Catan
					a := Action{Type: "catan_fish_" + kind, Target: 1, Color: 4, Tokens: []int{21}}
					switch kind {
					case "steal":
						g.Bank[3]--
						g.Players[1].Resources[3]++
					case "resource":
						a.Tokens = []int{0, 21}
					case "road":
						a.Tokens = []int{11, 21}
						a.Edge = fishLegalRoad(t, g, p)
					case "dev":
						a.Tokens = []int{0, 21, 22}
						at := slices.Index(g.DevDeck, 0)
						g.DevDeck[at], g.DevDeck[len(g.DevDeck)-1] = g.DevDeck[len(g.DevDeck)-1], g.DevDeck[at]
					}
					resources := clone(g.Players[p].Resources)
					bank := clone(g.Bank)
					deck := len(g.DevDeck)
					if err := s.Apply(p, a); err != nil {
						t.Fatal(err)
					}
					g = s.Catan
					if s.Phase != phase || s.Turn != p || g.Fishing.LastRollID != -1 || !slices.Equal(g.Fishing.Tokens.Discard, a.Tokens) {
						t.Fatal("fish action changed phase/production or wrong payment")
					}
					switch kind {
					case "robber":
						if g.Robber != -1 || !slices.Equal(resources, g.Players[p].Resources) {
							t.Fatal("removal stole a card or failed")
						}
					case "steal":
						if g.Players[1].Resources[3] != 0 || g.Players[p].Resources[3] != 1 || !slices.Equal(bank, g.Bank) {
							t.Fatal("random resource theft")
						}
						for _, line := range s.Log {
							if strings.Contains(line, "随机偷取") && strings.Contains(line, "粮食") {
								t.Fatal("theft color in public log")
							}
						}
					case "resource":
						if g.Bank[4] != bank[4]-1 || g.Players[p].Resources[4] != 1 {
							t.Fatal("bank resource")
						}
					case "road":
						if g.Edges[a.Edge].Owner != p || !slices.Equal(resources, g.Players[p].Resources) || !slices.Equal(bank, g.Bank) || g.FreeRoads != 0 {
							t.Fatal("fish road charged resources or consumed development action")
						}
					case "dev":
						if len(g.DevDeck) != deck-1 || g.Players[p].Dev[0] != 1 || g.Players[p].NewDev[0] != 1 || !slices.Equal(resources, g.Players[p].Resources) {
							t.Fatal("fish development/new card restriction")
						}
						fishGameReject(t, s, p, Action{Type: "catan_dev", Card: 0})
					}
					if err := g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					copy := clone(*s)
					if !reflect.DeepEqual(*s, copy) {
						t.Fatal("paid action restore")
					}
				})
			}
		}
	}
}

func TestCatanFishingPaymentAndIllegalEffectAtomicity(t *testing.T) {
	for _, setup := range []func(*State, int) Action{
		func(s *State, p int) Action { return Action{Type: "catan_fish_resource", Color: 0, Tokens: []int{21}} },
		func(s *State, p int) Action {
			return Action{Type: "catan_fish_resource", Color: 0, Tokens: []int{21, 21}}
		},
		func(s *State, p int) Action { return Action{Type: "catan_fish_steal", Target: 1, Tokens: []int{21}} },
		func(s *State, p int) Action {
			s.Catan.Bank[0] = 0
			return Action{Type: "catan_fish_resource", Color: 0, Tokens: []int{0, 21}}
		},
		func(s *State, p int) Action {
			return Action{Type: "catan_fish_resource", Color: 5, Tokens: []int{0, 21}}
		},
		func(s *State, p int) Action {
			s.Catan.DevDeck = nil
			return Action{Type: "catan_fish_dev", Tokens: []int{0, 21, 22}}
		},
		func(s *State, p int) Action { return Action{Type: "catan_fish_road", Edge: -1, Tokens: []int{11, 21}} },
		func(s *State, p int) Action {
			s.Catan.Robber = -1
			return Action{Type: "catan_fish_robber", Tokens: []int{21}}
		},
		func(s *State, p int) Action { return Action{Type: "catan_fish_robber", Tokens: []int{29}} },
		func(s *State, p int) Action {
			return Action{Type: "catan_fish_robber", Tokens: []int{21}, Take: []int{1, 0, 0, 0, 0}}
		},
		func(s *State, p int) Action {
			return Action{Type: "catan_fish_robber", Tokens: []int{21}, Skill: "helper"}
		},
		func(s *State, p int) Action {
			s.Phase = "catan_roads"
			s.Catan.FreeRoads = 2
			return Action{Type: "catan_fish_road", Edge: fishLegalRoad(t, s.Catan, p), Tokens: []int{11, 21}}
		},
		func(s *State, p int) Action {
			e := fishLegalRoad(t, s.Catan, p)
			placed := 0
			for i := range s.Catan.Edges {
				if i != e && placed < 15 {
					s.Catan.Edges[i].Owner = p
					placed++
				}
			}
			return Action{Type: "catan_fish_road", Edge: e, Tokens: []int{11, 21}}
		},
	} {
		s, p := fishActionFixture(t, 3, "catan_turn")
		a := setup(s, p)
		fishGameReject(t, s, p, a)
	}
	s, p := fishActionFixture(t, 3, "catan_turn")
	fishGameReject(t, s, 1, Action{Type: "catan_fish_robber", Tokens: []int{21}})
	if err := s.Apply(p, Action{Type: "catan_fish_robber", Tokens: []int{21}}); err != nil {
		t.Fatal(err)
	}
	// Three pays two; its lost excess cannot complete another three-for-four payment.
	fishGameReject(t, s, p, Action{Type: "catan_fish_resource", Color: 0, Tokens: []int{22}})
	if err := s.Apply(p, Action{Type: "catan_fish_resource", Color: 0, Tokens: []int{0, 22}}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Catan.Fishing.Tokens.Discard, []int{21, 0, 22}) {
		t.Fatal("overpayment produced change or credit")
	}
}

func TestCatanFishingBootUsesPublicScoresAndCanWinImmediately(t *testing.T) {
	s, p := fishActionFixture(t, 3, "catan_roll")
	g := s.Catan
	fishTop(&g.Fishing.Tokens, catanFishBoot)
	// The boot is drawn by another seat and passed internally to this fixture's owner.
	if err := g.Fishing.Tokens.beginDraw(1, []int{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	g.Fishing.Tokens.BootOwner = p
	g.Players[p].Score, g.Players[p].Dev[4] = 4, 2 // Public score 2.
	g.Players[1].Score = 2
	g.Players[2].Score, g.Players[2].Dev[4] = 5, 4 // Public score 1, despite higher actual score.
	if !slices.Equal(g.fishBootTargets(p), []int{1}) {
		t.Fatal("hidden VP affected boot eligibility")
	}
	fishGameReject(t, s, p, Action{Type: "catan_fish_boot", Target: 2})
	fishGameReject(t, s, p, Action{Type: "catan_fish_boot", Target: p})
	fishGameReject(t, s, p, Action{Type: "catan_fish_boot", Target: 1, Tokens: []int{0}})
	g.Players[p].Score = 10
	g.Players[1].Score = 8
	if err := s.Apply(p, Action{Type: "catan_fish_boot", Target: 1}); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || !slices.Equal(s.Winners, []int{p}) || s.Catan.Players[p].Score != 10 || s.Catan.Fishing.Tokens.BootOwner != 1 {
		t.Fatal("boot passing did not lower victory target immediately")
	}
}

func TestCatanFishingActionViewsAndBotNoHiddenInformation(t *testing.T) {
	s, p := fishActionFixture(t, 3, "catan_turn")
	g := s.Catan
	g.Bank[0] -= 2
	g.Players[1].Resources[0] = 2
	for viewer := -1; viewer < 3; viewer++ {
		legal := s.catanFishLegal(viewer)
		if (len(legal["actions"].([]string)) > 0) != (viewer == p) {
			t.Fatal("private affordability in other's view")
		}
	}
	other := clone(*s)
	other.Catan.Players[1].Resources[0]--
	other.Catan.Players[1].Resources[1]++
	// Keep the public bank equal: this only tests the private view/decision boundary.
	slices.Reverse(other.Catan.DevDeck)
	slices.Reverse(other.Catan.Fishing.Tokens.DrawPile)
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := other.BotAction(p)
	if err != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("fish bot looked at hidden information", a, b, err)
	}
	left, _ := json.Marshal(s.View(p))
	right, _ := json.Marshal(other.View(p))
	if string(left) != string(right) {
		t.Fatal("hidden opponent color/deck order changed view")
	}
	pay := g.fishPayment(p, 4)
	if !slices.Equal(pay, []int{0, 1, 11}) {
		t.Fatal("payment should avoid overpaying and free small token slots", pay)
	}
}

func TestCatanFishingBotsFinishAndConserve(t *testing.T) {
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishingGame(t, n)
			paid := 0
			for step := 0; step < 5000 && !s.Finished; step++ {
				actor := s.Turn
				if pending := s.CatanPendingActor(); pending >= 0 {
					actor = pending
				} else if s.Phase == "catan_discard" {
					for p, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatalf("step %d %s: %v", step, s.Phase, err)
				}
				if _, ok := catanFishCosts[a.Type]; ok {
					paid++
				}
				if err := s.Apply(actor, a); err != nil {
					t.Fatalf("step %d %s: %v", step, a.Type, err)
				}
				g := s.Catan
				if err := g.validateFishing(); err != nil {
					t.Fatal(err)
				}
				fleetSupply(t, g)
				cards := make([]int, 5)
				for _, pile := range [][]int{g.DevDeck, g.DevDiscard} {
					for _, kind := range pile {
						cards[kind]++
					}
				}
				for p, seat := range g.Players {
					for kind, count := range seat.Dev {
						cards[kind] += count
						if count < 0 || seat.NewDev[kind] < 0 || seat.NewDev[kind] > count {
							t.Fatal("development inventory")
						}
					}
					for _, count := range seat.Resources {
						if count < 0 {
							t.Fatal("negative resources")
						}
					}
					roads, villages, cities := g.pieces(p)
					if roads > 15 || villages > 5 || cities > 4 {
						t.Fatal("piece inventory")
					}
				}
				wantCards := []int{14, 2, 2, 2, 5}
				if n > 4 {
					wantCards = []int{20, 3, 3, 3, 5}
				}
				if !slices.Equal(cards, wantCards) {
					t.Fatal("development conservation", cards)
				}
				if step%23 == 0 {
					restored := clone(*s)
					if !reflect.DeepEqual(*s, restored) {
						t.Fatal("full game restore")
					}
					s = &restored
				}
			}
			if !s.Finished || paid == 0 || s.Catan.Players[s.Winners[0]].Score < s.Catan.victoryTargetFor(s.Winners[0]) {
				t.Fatal("fishing game failed to finish at correct target", n, paid, s.Phase)
			}
			t.Logf("%d players: round %d, %d paid fish actions", n, s.Round, paid)
		})
	}
}
