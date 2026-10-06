package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func twoRiverFixture(t *testing.T) *State {
	t.Helper()
	s, err := NewCatanTwoRivers(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		helperApply(t, s, s.Turn, a)
	}
	twoCoreActionPhase(t, s)
	return s
}
func twoRiverCheck(t *testing.T, s *State) {
	t.Helper()
	riverConserved(t, s)
	if err := s.validateCatanTwo(); err != nil {
		t.Fatal(err)
	}
	twoCoreRestore(t, s)
}

func TestCatanTwoRiversSetupAndSwampRewards(t *testing.T) {
	rewards := map[int]bool{}
	for sample := 0; sample < 24; sample++ {
		s, err := NewCatanTwoRivers(2, CatanOptions{})
		if err != nil {
			t.Fatal(err)
		}
		g := s.Catan
		if s.Phase != "catan_rivers_start" || len(g.Players) != 2 || len(g.Tiles) != 19 || len(g.Rivers.Map.Bridges) != 7 || g.Rivers.Bank != 100 || sum(g.Rivers.Gold) != 0 || g.Two.Bank != 10 {
			t.Fatal("wrong initial combination")
		}
		for _, owner := range catanTwoNeutralOwners {
			r, v, c := g.pieces(owner)
			if r != 0 || v != 1 || c != 0 {
				t.Fatal("neutral setup inventory")
			}
		}
		helperReject(t, s, 1-s.Turn, Action{Type: "catan_rivers_start", Tile: g.Rivers.Map.Swamps[0]})
		s.AutoCatanPending()
		if !slices.Contains(s.Catan.Rivers.Map.Swamps, s.Catan.Robber) || s.Phase != "catan_setup_settlement" {
			t.Fatal("swamp timeout")
		}
		for s.Catan.setup() {
			g = s.Catan
			p := s.Turn
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			if a.Type == "catan_settlement" {
				for _, v := range g.Vertices {
					if g.canSettlement(p, v.ID, true) {
						n := g.twoSettlementTokens(p, v.ID)
						if !rewards[n] && n <= g.Two.Bank {
							a.Vertex = v.ID
							break
						}
					}
				}
			}
			gold, tokens := slices.Clone(g.Rivers.Gold), slices.Clone(g.Two.Tokens)
			goldGain, tokenGain := 0, 0
			if a.Type == "catan_settlement" {
				tokenGain = g.twoSettlementTokens(p, a.Vertex)
				if g.riverVertex(a.Vertex) {
					goldGain = 1
				}
				rewards[tokenGain] = true
			}
			if a.Type == "catan_road" && g.riverEdge(a.Edge) {
				goldGain = 1
			}
			helperApply(t, s, p, a)
			gold[p] += goldGain
			tokens[p] += tokenGain
			if !slices.Equal(s.Catan.Rivers.Gold, gold) || !slices.Equal(s.Catan.Two.Tokens, tokens) {
				t.Fatal("combined setup reward or recipient")
			}
			twoRiverCheck(t, s)
		}
		g = s.Catan
		// Swamp and coast give3trade tokens, independently of the1river coin.
		for _, tile := range g.Tiles {
			if tile.Resource == catanSwamp {
				for _, v := range tile.Vertices {
					if g.twoSettlementTokens(0, v) < 2 {
						t.Fatal("swamp reward missing")
					}
					if g.twoSettlementTokens(-2, v) != 0 {
						t.Fatal("neutral trade tokens")
					}
				}
			}
		}
		richest, poor := g.riverWealth()
		if richest < -1 || len(poor) == 0 {
			t.Fatal("neutral entered wealth comparison")
		}
		for _, p := range poor {
			if p < 0 || p > 1 {
				t.Fatal("neutral poorest")
			}
		}
	}
	for _, n := range []int{0, 1, 2, 3} {
		if !rewards[n] {
			t.Fatal("missing setup reward branch", n)
		}
	}
	for _, n := range []int{1, 3, 4, 5, 6} {
		if _, err := NewCatanTwoRivers(n, CatanOptions{}); err == nil {
			t.Fatal("wrong player count accepted")
		}
	}
	for _, o := range []CatanOptions{{Helpers: true}, {FiveSix: true}} {
		if _, err := NewCatanTwoRivers(2, o); err == nil {
			t.Fatal("unverified combination accepted")
		}
	}
}

func TestCatanTwoRiversPaidBridgeRequiresNeutralBridge(t *testing.T) {
	s := twoRiverFixture(t)
	p := s.Turn
	g := s.Catan
	// Build-only fixture: put a real village on a currently legal bank of an
	// unused bridge, preserving both neutral villages and distance rules.
	found := false
	for _, edge := range g.Rivers.Map.Bridges {
		if g.Edges[edge].Owner != -1 {
			continue
		}
		for _, v := range []int{g.Edges[edge].A, g.Edges[edge].B} {
			if !g.canSettlement(p, v, true) {
				continue
			}
			trial := clone(*s)
			trial.Catan.Vertices[v].Owner = p
			trial.Catan.Vertices[v].Level = 1
			neutral := trial.Catan.twoNeutralChoices("bridge")
			if len(neutral) > 0 && slices.Contains(g.Rivers.Map.Bridges, neutral[0].Edge) && neutral[0].Edge != edge {
				s = &trial
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("no combined bridge fixture")
	}
	g = s.Catan
	twoTokenHand(s, p, []int{2, 4, 0, 0, 0})
	s.catanScores()
	edge := -1
	for _, id := range g.Rivers.Map.Bridges {
		if g.canBridge(p, id) {
			edge = id
			break
		}
	}
	if edge < 0 {
		t.Fatal("no paid bridge")
	}
	bank, gold := slices.Clone(g.Bank), slices.Clone(g.Rivers.Gold)
	helperApply(t, s, p, Action{Type: "catan_bridge", Edge: edge})
	g = s.Catan
	if s.Phase != "catan_two_build" || g.Two.Pending.Kind != "bridge" || !g.Edges[edge].Bridge || g.Rivers.Gold[p] != gold[p]+3 || g.Bank[0] != bank[0]+1 || g.Bank[1] != bank[1]+2 {
		t.Fatal("paid bridge sequence")
	}
	choices := g.twoNeutralChoices("bridge")
	if len(choices) == 0 || !slices.Contains(g.Rivers.Map.Bridges, choices[0].Edge) {
		t.Fatal("expected neutral bridge")
	}
	for _, road := range g.twoNeutralChoices("road") {
		helperReject(t, s, p, Action{Type: "catan_two_build", Target: -road.Owner - 2, Vertex: -1, Edge: road.Edge})
		break
	}
	next := choices[0]
	gold = slices.Clone(g.Rivers.Gold)
	bank = slices.Clone(g.Bank)
	tokens := slices.Clone(g.Two.Tokens)
	beforeRoads, _, _ := g.pieces(next.Owner)
	twoRiverCheck(t, s)
	helperApply(t, s, p, Action{Type: "catan_two_build", Target: -next.Owner - 2, Vertex: -1, Edge: next.Edge})
	g = s.Catan
	afterRoads, _, _ := g.pieces(next.Owner)
	if !g.Edges[next.Edge].Bridge || g.Edges[next.Edge].Owner != next.Owner || beforeRoads != afterRoads || s.Phase != "catan_turn" || !slices.Equal(g.Rivers.Gold, gold) || !slices.Equal(g.Bank, bank) || !slices.Equal(g.Two.Tokens, tokens) {
		t.Fatal("neutral bridge charged resource/gold/road inventory")
	}
	if !strings.Contains(s.Log[len(s.Log)-1], "桥梁") {
		t.Fatal("bridge mislabeled in log", s.Log)
	}
	for _, owner := range []int{0, 1, -2, -3} {
		if g.canBridge(owner, next.Edge) || g.canRoad(owner, next.Edge) {
			t.Fatal("neutral bridge overwritten")
		}
	}
	twoRiverCheck(t, s)
}

func TestCatanTwoRiversBridgeFallbackInventoryAndRobber(t *testing.T) {
	s := twoRiverFixture(t)
	g := s.Catan
	p := s.Turn
	// Explicit inventory fixture: three bridges of each neutral color use
	// six of seven sites. Neither neutral may acquire a fourth bridge.
	for i, edge := range g.Rivers.Map.Bridges[:6] {
		g.Edges[edge].Owner = -2 - i/3
		g.Edges[edge].Bridge = true
	}
	choices := g.twoNeutralChoices("bridge")
	if len(choices) == 0 {
		t.Fatal("expected road fallback")
	}
	for _, c := range choices {
		if slices.Contains(g.Rivers.Map.Bridges, c.Edge) {
			t.Fatal("fourth bridge candidate")
		}
	}
	gold := slices.Clone(g.Rivers.Gold)
	g.Two.Pending = &CatanTwoPending{Kind: "bridge", Resume: "catan_turn"}
	g.Two.Sequence++
	s.Phase = "catan_two_build"
	c := choices[0]
	helperApply(t, s, p, Action{Type: "catan_two_build", Target: -c.Owner - 2, Vertex: -1, Edge: c.Edge})
	if s.Catan.Edges[c.Edge].Bridge || s.Catan.bridgeCount(-2) != 3 || s.Catan.bridgeCount(-3) != 3 || !slices.Equal(s.Catan.Rivers.Gold, gold) {
		t.Fatal("fallback bridge/inventory/gold")
	}
	twoRiverCheck(t, s)
	for _, target := range s.Catan.Rivers.Map.Swamps {
		trial := clone(*s)
		trial.Catan.Two.Spent = false
		for _, tile := range trial.Catan.Tiles {
			if tile.Resource < 5 {
				trial.Catan.Robber = tile.ID
				break
			}
		}
		hands := [][]int{slices.Clone(trial.Catan.Players[0].Resources), slices.Clone(trial.Catan.Players[1].Resources)}
		helperReject(t, &trial, p, Action{Type: "catan_two_robber", Tile: 0})
		helperApply(t, &trial, p, Action{Type: "catan_two_robber", Tile: target})
		if trial.Catan.Robber != target || trial.Phase != "catan_turn" {
			t.Fatal("swamp retreat failed")
		}
		for i := range 2 {
			if !slices.Equal(hands[i], trial.Catan.Players[i].Resources) {
				t.Fatal("swamp retreat stole")
			}
		}
		trial.Catan.Two.Spent = false
		helperReject(t, &trial, p, Action{Type: "catan_two_robber", Tile: target})
		twoRiverCheck(t, &trial)
	}
}

func TestCatanTwoRiversCompleteEngineGames(t *testing.T) {
	coverage := map[string]int{}
	for sample := 0; sample < 3; sample++ {
		t.Run(fmt.Sprint(sample), func(t *testing.T) {
			s, e := NewCatanTwoRivers(2, CatanOptions{})
			if e != nil {
				t.Fatal(e)
			}
			steps := 0
			for ; steps < 6000 && !s.Finished; steps++ {
				actor := s.Turn
				if s.Phase == "catan_discard" {
					for p, n := range s.Catan.DiscardDue {
						if n > 0 {
							actor = p
							break
						}
					}
				}
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(steps, s.Phase, err)
				}
				coverage[a.Type]++
				before := clone(*s)
				if err = s.Apply(actor, a); err != nil {
					t.Fatal(steps, s.Phase, a, err)
				}
				if a.Type == "catan_two_build" && !slices.Equal(before.Catan.Rivers.Gold, s.Catan.Rivers.Gold) {
					t.Fatal("neutral earned gold")
				}
				riverConserved(t, s)
				if err = s.validateCatanTwo(); err != nil {
					t.Fatal(err)
				}
				if steps%29 == 0 || s.CatanPendingActor() >= 0 {
					twoRiverCheck(t, s)
				}
				for viewer := -1; viewer < 2; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)
					players := v["players"].([]any)
					for p, x := range players {
						if (x.(map[string]any)["resources"] != nil) != (p == viewer || s.Finished) {
							t.Fatal("hand privacy")
						}
					}
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Winners[0] != s.Turn || s.Catan.Players[s.Turn].Score < 10 {
				t.Fatal("game failed to finish", steps)
			}
			// Independently recompute the winning score, including the
			// two-real-player wealth comparison (neutrals have no gold seat).
			g := s.Catan
			for p, player := range g.Players {
				score := player.Dev[4]
				for _, v := range g.Vertices {
					if v.Owner == p {
						score += v.Level
					}
				}
				if g.LongestOwner == p {
					score += 2
				}
				if g.ArmyOwner == p {
					score += 2
				}
				if g.Rivers.Gold[p] > g.Rivers.Gold[1-p] {
					score++
				}
				if g.Rivers.Gold[p] <= g.Rivers.Gold[1-p] {
					score -= 2
				}
				if score != player.Score {
					t.Fatal("incorrect combined score", p, score, player.Score)
				}
			}
			b, _ := json.Marshal(s)
			twoRiverCheck(t, s)
			after, _ := json.Marshal(s)
			if !reflect.DeepEqual(b, after) {
				t.Fatal("finished restore")
			}
			t.Logf("steps=%d round=%d gold=%v tokens=%v", steps, s.Round, s.Catan.Rivers.Gold, s.Catan.Two.Tokens)
		})
	}
	for _, key := range []string{"catan_bridge", "catan_two_build", "catan_two_trade", "catan_two_return", "catan_coin_buy", "catan_coin_sell"} {
		if coverage[key] == 0 {
			t.Fatal("missing natural coverage", key, coverage)
		}
	}
	t.Log(coverage)
}

func TestCatanTwoRiversRuleVersionCompatibility(t *testing.T) {
	s := twoRiverFixture(t)
	if s.Catan.Rivers.Rules != CatanRiversRules {
		t.Fatal("new river save omitted rule version")
	}
	s.Catan.Rivers.Rules = "unsupported"
	helperReject(t, s, s.Turn, Action{Type: "catan_end"})
	// Old internal river saves had no field; their2025map/inventory stay valid.
	s.Catan.Rivers.Rules = ""
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	twoRiverCheck(t, s)
}
