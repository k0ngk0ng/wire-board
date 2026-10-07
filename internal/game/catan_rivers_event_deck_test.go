package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func riversReferenceGame(t *testing.T, n int, setup bool) *State {
	t.Helper()
	s, err := newCatanRiversReferenceEvents(n)
	if err != nil {
		t.Fatal(err)
	}
	for setup && s.Catan.setup() {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, s.Turn, a)
	}
	return s
}

func TestCatanRiversEventDeckAllFaces(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for kind := range catanCardEventNames {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				s := riversReferenceGame(t, n, true)
				beforeGold := slices.Clone(s.Catan.Rivers.Gold)
				referenceEventTop(t, s, kind)
				helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
				if s.Catan.RevealedEvent.Kind != kind {
					t.Fatal("wrong actual event")
				}
				s = resolveTwoReferenceProduction(t, s)
				if s.Catan.RollID != 1 || !s.Catan.RevealedEvent.ProductionStarted {
					t.Fatal("duplicate or missing production")
				}
				if !slices.Equal(beforeGold, s.Catan.Rivers.Gold) {
					t.Fatal("ordinary event minted river coins")
				}
				if kind == "robber_flees" {
					if n == 2 && !slices.Contains(s.Catan.Rivers.Map.Swamps, s.Catan.Robber) || n != 2 && s.Catan.Robber != -1 {
						t.Fatal("wrong desert replacement rule", s.Catan.Robber)
					}
				}
				riverConserved(t, s)
				s = referenceEventRestore(t, s)
				if n == 2 {
					twoCoreRestore(t, s)
				}
			})
		}
	}
}

func TestCatanRiversEventDeckFleeChoices(t *testing.T) {
	s := riversReferenceGame(t, 2, true)
	referenceEventTop(t, s, "robber_flees")
	hands := clone(s.Catan.Players)
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	if s.Phase != "catan_card_event" || s.CatanPendingActor() != s.Turn {
		t.Fatal("two swamps need a choice")
	}
	choices := s.View(s.Turn)["catan"].(map[string]any)["legal"].(map[string][]int)["fleeDeserts"]
	if !slices.Equal(choices, s.Catan.Rivers.Map.Swamps) {
		t.Fatal("wrong swamp choices")
	}
	helperReject(t, s, 1-s.Turn, Action{Type: "catan_robber_flees", Tile: choices[1]})
	helperReject(t, s, s.Turn, Action{Type: "catan_robber_flees", Tile: -1})
	s = referenceEventRestore(t, s)
	// Independently account for the card's production, which follows movement.
	expected := make([][]int, 2)
	for p := range expected {
		expected[p] = slices.Clone(hands[p].Resources)
	}
	for _, tile := range s.Catan.Tiles {
		if tile.Number == 4 && tile.Resource >= 0 && tile.Resource < 5 {
			for _, v := range tile.Vertices {
				b := s.Catan.Vertices[v]
				if b.Owner >= 0 && b.Level > 0 {
					expected[b.Owner][tile.Resource] += b.Level
				}
			}
		}
	}
	helperApply(t, s, s.Turn, Action{Type: "catan_robber_flees", Tile: choices[1]})
	if s.Phase != "catan_roll" || s.Catan.Robber != choices[1] || s.Catan.RollID != 1 {
		t.Fatal("swamp did not resume second production")
	}
	for p, want := range expected {
		if !slices.Equal(want, s.Catan.Players[p].Resources) {
			t.Fatal("flee stole cards or produced incorrectly")
		}
	}
}

func TestCatanRiversEventDeckBridgesAndRepairRewards(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			// Explicit small controlled position on the official river map.
			s := riversFixture(t, n)
			g := s.Catan
			bridge := g.Rivers.Map.Bridges[0]
			e := g.Edges[bridge]
			road := -1
			for _, id := range g.touching(e.A) {
				if id != bridge && !slices.Contains(g.Rivers.Map.Bridges, id) {
					road = id
					break
				}
			}
			if road < 0 {
				t.Fatal("fixture road")
			}
			g.Edges[road].Owner = 0
			g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
			existing := g.Rivers.Map.Bridges[len(g.Rivers.Map.Bridges)-1]
			g.Edges[existing].Owner, g.Edges[existing].Bridge = 0, true
			s.catanScores()
			g.EventDeck = &catanEventSession{Catalogue: catanEventReferenceCatalogue, Deck: newCatanEventDeck()}
			s.Phase = "catan_roll"
			referenceEventTop(t, s, "earthquake")
			helperApply(t, s, 0, Action{Type: "catan_roll"})
			choices := s.View(0)["catan"].(map[string]any)["legal"].(map[string][]int)["earthquakeRoads"]
			if !slices.Equal(choices, []int{road}) {
				t.Fatal("bridge offered as a damageable road", choices)
			}
			helperReject(t, s, 0, Action{Type: "catan_earthquake", Edge: existing})
			helperApply(t, s, 0, Action{Type: "catan_earthquake", Edge: road})
			g = s.Catan
			if !g.Edges[road].Damaged || g.Edges[existing].Damaged {
				t.Fatal("wrong damaged component")
			}
			// FAQ21: damaged road alone cannot connect the new bridge; own building can.
			g.Vertices[e.A].Owner, g.Vertices[e.A].Level = -1, 0
			if g.canBridge(0, bridge) {
				t.Fatal("damaged road alone allowed bridge")
			}
			g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
			if !g.canBridge(0, bridge) {
				t.Fatal("own settlement did not allow bridge")
			}
			for c, want := range []int{3, 4, 0, 0, 0} {
				catanGive(g, 0, c, want-g.Players[0].Resources[c])
			}
			before := slices.Clone(g.Rivers.Gold)
			helperApply(t, s, 0, Action{Type: "catan_bridge", Edge: bridge})
			if s.Catan.Rivers.Gold[0] != before[0]+3 {
				t.Fatal("new bridge did not reward three coins")
			}
			gold := slices.Clone(s.Catan.Rivers.Gold)
			helperApply(t, s, 0, Action{Type: "catan_repair_road", Edge: road})
			if !slices.Equal(gold, s.Catan.Rivers.Gold) || !slices.Equal(s.Catan.Players[0].Resources, []int{1, 1, 0, 0, 0}) || s.Catan.Edges[road].Damaged {
				t.Fatal("repair rewarded coins or charged wrong price")
			}
			riverConserved(t, s)
			s = referenceEventRestore(t, s)
		})
	}
}

func TestCatanRiversEventDeckNaturalMatches(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riversReferenceGame(t, n, false)
			for step := 0; !s.Finished && step < 6000; step++ {
				actor := ckActor(s)
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(step, err)
				}
				if a.Type == "catan_roll" {
					other := clone(*s)
					pile := other.Catan.EventDeck.Deck.DrawPile
					pile[0], pile[1] = pile[1], pile[0]
					b, err := other.BotAction(actor)
					if err != nil || !reflect.DeepEqual(a, b) {
						t.Fatal("bot used hidden deck order")
					}
					if !reflect.DeepEqual(s.View(-1), other.View(-1)) {
						t.Fatal("hidden order exposed")
					}
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatalf("step=%d phase=%s action=%+v: %v", step, s.Phase, a, err)
				}
				riverConserved(t, s)
				if err = s.validateCatanEventSession(); err != nil {
					t.Fatal(err)
				}
				if step%29 == 0 || s.Catan.CardEvent != nil {
					s = referenceEventRestore(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 10 {
				t.Fatal("natural match failed", s.Round)
			}
			t.Logf("%d players: %d draws, %d cycles, %d rounds", n, s.Catan.RollID, s.Catan.EventDeck.Deck.Cycle, s.Round)
		})
	}
}

func TestCatanRiversEventDeckDoubleNumberProduction(t *testing.T) {
	// Explicit city on the official 2/12 hex: neither bank shortage nor a port
	// reward can conceal a missing production at either printed number.
	s := riversFixture(t, 3)
	g := s.Catan
	tile := g.Tiles[g.Rivers.Map.DoubleNumberTile]
	g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = 0, 2
	s.catanScores()
	g.EventDeck = &catanEventSession{Catalogue: catanEventReferenceCatalogue, Deck: newCatanEventDeck()}
	s.Phase = "catan_roll"
	twoReferenceTop(t, s, 0, 35) // Plentiful Year / 2, Calm Seas / 12.
	rewardColor := (tile.Resource + 1) % 5
	for draw := 1; draw <= 2; draw++ {
		helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
		for steps := 0; s.Catan.CardEvent != nil && steps < 3; steps++ {
			take := make([]int, 5)
			take[rewardColor] = 1
			helperApply(t, s, s.CatanPendingActor(), Action{Type: "catan_event_resource", Take: take})
		}
		if s.Phase != "catan_turn" || s.Catan.Players[0].Resources[tile.Resource] != 2*draw {
			t.Fatal("2/12 did not each produce city quantity", draw)
		}
		if s.Catan.Rivers.Gold[0] != 0 {
			t.Fatal("production awarded construction gold")
		}
		riverConserved(t, s)
		s = referenceEventRestore(t, s)
		helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	}
}

func TestCatanTwoRiversEventDeckNeutralBridgeRewards(t *testing.T) {
	s := riversReferenceGame(t, 2, true)
	twoReferenceTop(t, s, 11, 22) // Damage a road, then finish both productions.
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	s = resolveTwoReferenceProduction(t, s)
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	s = resolveTwoReferenceProduction(t, s)
	g, p := s.Catan, s.Turn
	// A controlled midgame settlement gives a legal bridge anchor while the
	// player's earthquake damage remains elsewhere. Respect settlement spacing.
	bridge := -1
	for _, id := range g.Rivers.Map.Bridges {
		if g.Edges[id].Owner != -1 {
			continue
		}
		for _, v := range []int{g.Edges[id].A, g.Edges[id].B} {
			if g.canSettlement(p, v, true) {
				g.Vertices[v].Owner, g.Vertices[v].Level = p, 1
				bridge = id
				break
			}
		}
		if bridge >= 0 {
			break
		}
	}
	if bridge < 0 {
		t.Fatal("no controlled bridge anchor")
	}
	s.catanScores()
	if !g.hasDamagedRoad(p) || !g.canBridge(p, bridge) {
		t.Fatal("bridge should remain possible through own building")
	}
	for c, want := range []int{1, 2} {
		catanGive(g, p, c, want-g.Players[p].Resources[c])
	}
	gold, tokens := slices.Clone(g.Rivers.Gold), slices.Clone(g.Two.Tokens)
	before0, before1 := g.bridgeCount(-2), g.bridgeCount(-3)
	helperApply(t, s, p, Action{Type: "catan_bridge", Edge: bridge})
	if s.Catan.Two.Pending == nil || s.Catan.Two.Pending.Kind != "bridge" {
		t.Fatal("real bridge did not request neutral bridge")
	}
	s = referenceEventRestore(t, s)
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, p, a)
	gold[p] += 3
	if !slices.Equal(gold, s.Catan.Rivers.Gold) || !slices.Equal(tokens, s.Catan.Two.Tokens) {
		t.Fatal("neutral construction received reward")
	}
	if s.Catan.bridgeCount(-2)+s.Catan.bridgeCount(-3) != before0+before1+1 {
		t.Fatal("missing neutral bridge")
	}
	if s.Phase != "catan_turn" || s.Catan.RollID != 2 || !s.Catan.hasDamagedRoad(p) {
		t.Fatal("bridge repaired road or consumed event")
	}
	riverConserved(t, s)
	twoCoreRestore(t, s)
}
