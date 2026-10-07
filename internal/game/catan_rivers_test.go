package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func riversFixture(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanRivers(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	s.Turn = 0
	s.Catan.StartPlayer = 0
	if s.Catan.Paired != nil {
		s.Catan.Paired.Primary, s.Catan.Paired.Secondary = 0, 3
	}
	if err = s.Apply(0, Action{Type: "catan_rivers_start", Tile: s.Catan.Rivers.Map.Swamps[0]}); err != nil {
		t.Fatal(err)
	}
	s.Catan.SetupStep, s.Catan.TurnSerial = s.Catan.SetupLimit(), 1
	s.Phase = "catan_turn"
	return s
}
func riverGold(g *Catan, p, n int) { g.Rivers.Bank += g.Rivers.Gold[p] - n; g.Rivers.Gold[p] = n }
func riverReject(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	before, _ := json.Marshal(s)
	err := s.Apply(p, a)
	after, _ := json.Marshal(s)
	if err == nil || string(before) != string(after) {
		t.Fatalf("invalid %s accepted or mutated state: %v", a.Type, err)
	}
}
func riverConserved(t *testing.T, s *State) {
	t.Helper()
	g := s.Catan
	if err := g.validateRivers(); err != nil {
		t.Fatal(err)
	}
	for c, bank := range g.Bank {
		total := bank
		for _, p := range g.Players {
			if p.Resources[c] < 0 {
				t.Fatal("negative resource")
			}
			total += p.Resources[c]
		}
		want := 19
		if len(g.Players) > 4 {
			want = 24
		}
		if total != want {
			t.Fatal("resource conservation", c, total)
		}
	}
}

func TestCatanRiversSetupRewardsAndRestore(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := NewCatanRivers(n, CatanOptions{FiveSix: n > 4})
			if err != nil {
				t.Fatal(err)
			}
			first := s.Turn
			g := s.Catan
			for _, p := range g.Players {
				if p.Score != -2 {
					t.Fatal("initial poorest ties")
				}
			}
			riverReject(t, s, (first+1)%n, Action{Type: "catan_rivers_start", Tile: g.Rivers.Map.Swamps[1]})
			riverReject(t, s, first, Action{Type: "catan_rivers_start", Tile: 0})
			s.AutoCatanPending()
			if s.Phase != "catan_setup_settlement" || s.Catan.SetupStep != 0 || s.Turn != first || !slices.Contains(s.Catan.Rivers.Map.Swamps, s.Catan.Robber) {
				t.Fatal("start timeout")
			}
			expected := make([]int, n)
			steps := 0
			for s.Catan.setup() && steps < 4*n+2 {
				p := s.Turn
				a, err := s.catanBot(p)
				if err != nil {
					t.Fatal(err)
				}
				if a.Type == "catan_settlement" && s.Catan.riverVertex(a.Vertex) || a.Type == "catan_road" && s.Catan.riverEdge(a.Edge) {
					expected[p]++
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(s.Catan.Rivers.Gold, expected) {
					t.Fatal("starting gold repeated or missing")
				}
				s = &[]State{clone(*s)}[0]
				riverConserved(t, s)
				steps++
			}
			if s.Catan.setup() || s.Phase != "catan_roll" || s.Turn != first || sum(expected) == 0 {
				t.Fatal("setup did not complete")
			}
		})
	}
}

func TestCatanRiversBridgesAndRoadRewards(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riversFixture(t, n)
			g := s.Catan
			riverGold(g, 1, g.Rivers.Bank)
			id := g.Rivers.Map.Bridges[0]
			e := g.Edges[id]
			g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
			catanGive(g, 0, 0, 3)
			catanGive(g, 0, 1, 5)
			riverReject(t, s, 0, Action{Type: "catan_road", Edge: id})
			s.Phase = "catan_roads"
			g.FreeRoads = 2
			g.ResumePhase = "catan_turn"
			riverReject(t, s, 0, Action{Type: "catan_bridge", Edge: id})
			s.Phase = "catan_turn"
			g.FreeRoads = 0
			if err := s.Apply(0, Action{Type: "catan_bridge", Edge: id}); err != nil {
				t.Fatal(err)
			}
			g = s.Catan
			roads, _, _ := g.pieces(0)
			if !g.Edges[id].Bridge || roads != 0 || g.bridgeCount(0) != 1 || g.roadLength(0) != 1 || g.Rivers.Gold[0] != 3 || g.Players[0].Resources[0] != 2 || g.Players[0].Resources[1] != 3 {
				t.Fatal("bridge payment/reward/inventory")
			}
			riverReject(t, s, 0, Action{Type: "catan_bridge", Edge: id})
			road := -1
			for _, r := range g.Edges {
				if g.canRoad(0, r.ID) && g.riverEdge(r.ID) {
					road = r.ID
					break
				}
			}
			if road < 0 {
				t.Fatal("no adjacent riverbank road")
			}
			if err := s.Apply(0, Action{Type: "catan_road", Edge: road}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Rivers.Gold[0] != 4 || s.Catan.bridgeCount(0) != 1 {
				t.Fatal("road earns once")
			}
			// A free road earns its usual river reward too, without charging resources.
			g = s.Catan
			s.Phase = "catan_roads"
			g.FreeRoads = 1
			g.ResumePhase = "catan_turn"
			road = -1
			for _, r := range g.Edges {
				if g.canRoad(0, r.ID) && g.riverEdge(r.ID) {
					road = r.ID
					break
				}
			}
			before := slices.Clone(g.Players[0].Resources)
			if road < 0 {
				t.Fatal("no free river road")
			}
			if err := s.Apply(0, Action{Type: "catan_road", Edge: road}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Rivers.Gold[0] != 5 || !slices.Equal(before, s.Catan.Players[0].Resources) || s.Phase != "catan_turn" {
				t.Fatal("free reward/continuation")
			}
			// Upgrading the source settlement must not award a second building coin.
			g = s.Catan
			catanGive(g, 0, 3, 2)
			catanGive(g, 0, 4, 3)
			if err := s.Apply(0, Action{Type: "catan_city", Vertex: e.A}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Rivers.Gold[0] != 5 || s.Catan.Rivers.GoldIssued != 5 {
				t.Fatal("city minted gold")
			}
			riverConserved(t, s)
		})
	}
}

func TestCatanRiversBridgeConnectionsLimitAndLedger(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	id := g.Rivers.Map.Bridges[0]
	e := g.Edges[id]
	road := -1
	for _, r := range g.touching(e.A) {
		if r != id && !slices.Contains(g.Rivers.Map.Bridges, r) {
			road = r
			break
		}
	}
	g.Edges[road].Owner = 0
	if !g.canBridge(0, id) {
		t.Fatal("bridge cannot continue road")
	}
	g.Edges[road].Damaged = true
	if g.canBridge(0, id) {
		t.Fatal("damaged road connected bridge")
	}
	g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 1
	if !g.canBridge(0, id) {
		t.Fatal("own building must permit bridge despite unrelated damaged road")
	}
	g.Vertices[e.A].Owner = 1
	if g.canBridge(0, id) {
		t.Fatal("crossed opponent building")
	}
	g.Vertices[e.A].Owner = 0
	g.Edges[road].Damaged = false
	for _, other := range g.Rivers.Map.Bridges[1:4] {
		g.Edges[other].Owner, g.Edges[other].Bridge = 0, true
	}
	if g.canBridge(0, id) {
		t.Fatal("fourth bridge")
	}
	for _, other := range g.Rivers.Map.Bridges[1:4] {
		g.Edges[other].Owner, g.Edges[other].Bridge = -1, false
	}
	catanGive(g, 0, 0, 1)
	catanGive(g, 0, 1, 2)
	riverGold(g, 1, 99)
	if err := s.Apply(0, Action{Type: "catan_bridge", Edge: id}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.GoldIssued != 2 || s.Catan.Rivers.Gold[0] != 3 || s.Catan.Rivers.Bank != 0 {
		t.Fatal("partial physical supply must pay the full bridge reward")
	}
	riverConserved(t, s)
}

func TestCatanRiversCoinPurchaseLimitsAndPair(t *testing.T) {
	for _, n := range []int{3, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riversFixture(t, n)
			riverGold(s.Catan, 0, 10)
			riverReject(t, s, 1, Action{Type: "catan_coin_buy", Color: 0})
			s.Phase = "catan_roll"
			riverReject(t, s, 0, Action{Type: "catan_coin_buy", Color: 0})
			s.Phase = "catan_turn"
			for _, c := range []int{0, 4} {
				if err := s.Apply(0, Action{Type: "catan_coin_buy", Color: c}); err != nil {
					t.Fatal(err)
				}
				*s = clone(*s)
			}
			if s.Catan.Rivers.Bought != 2 || s.Catan.Rivers.Gold[0] != 6 || s.Catan.Players[0].Resources[0] != 1 || s.Catan.Players[0].Resources[4] != 1 {
				t.Fatal("purchases")
			}
			riverReject(t, s, 0, Action{Type: "catan_coin_buy", Color: 1})
			next := 1
			if n > 4 {
				next = 3
			}
			riverGold(s.Catan, next, 4)
			if err := s.Apply(0, Action{Type: "catan_end"}); err != nil {
				t.Fatal(err)
			}
			if s.Catan.Rivers.Bought != 0 || s.Turn != next {
				t.Fatal("purchase cap did not reset once")
			}
			if n > 4 {
				if s.Phase != "catan_turn" || !s.Catan.Paired.Second {
					t.Fatal("pair")
				}
				if err := s.Apply(next, Action{Type: "catan_coin_buy", Color: 2}); err != nil {
					t.Fatal(err)
				}
				riverReject(t, s, next, Action{Type: "catan_trade_offer", Give: make([]int, 5), Take: []int{1, 0, 0, 0, 0}, GoldGive: 1})
			}
			riverConserved(t, s)
		})
	}
}

func TestCatanRiversWealthAndCoinTrade(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	for _, tc := range []struct {
		gold   []int
		rich   int
		poor   []int
		points []int
	}{
		{[]int{0, 0, 0}, -1, []int{0, 1, 2}, []int{-2, -2, -2}},
		{[]int{5, 2, 2}, 0, []int{1, 2}, []int{1, -2, -2}},
		{[]int{5, 5, 2}, -1, []int{2}, []int{0, 0, -2}},
		{[]int{1, 2, 3}, 2, []int{0}, []int{-2, 0, 1}},
	} {
		for p, n := range tc.gold {
			riverGold(g, p, n)
		}
		s.catanScores()
		rich, poor := g.riverWealth()
		if rich != tc.rich || !slices.Equal(poor, tc.poor) {
			t.Fatal("wealth tie", rich, poor)
		}
		for p, points := range tc.points {
			if g.Players[p].Score != points {
				t.Fatal("wealth points")
			}
		}
	}
	riverGold(g, 0, 5)
	riverGold(g, 1, 2)
	riverGold(g, 2, 1)
	catanGive(g, 1, 0, 2)
	offer := Action{Type: "catan_trade_offer", Give: make([]int, 5), Take: []int{2, 0, 0, 0, 0}, GoldGive: 3}
	if err := s.Apply(0, offer); err != nil {
		t.Fatal(err)
	}
	id := s.Catan.Trade.ID
	if err := s.Apply(1, Action{Type: "catan_trade_accept", Offer: id}); err != nil {
		t.Fatal(err)
	}
	*s = clone(*s)
	if err := s.Apply(0, Action{Type: "catan_trade_complete", Offer: id, Target: 1}); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	if !slices.Equal(g.Rivers.Gold, []int{2, 5, 1}) || g.Players[0].Resources[0] != 2 || g.Players[1].Resources[0] != 0 || g.Players[0].Score != 0 || g.Players[1].Score != 1 {
		t.Fatal("trade settlement")
	}
	riverReject(t, s, 0, Action{Type: "catan_trade_complete", Offer: id, Target: 1})
	offer.GoldGive = -1
	riverReject(t, s, 0, offer)
	offer.GoldGive = 1
	offer.GoldTake = 1
	riverReject(t, s, 0, offer)
	offer.GoldTake = 153
	offer.GoldGive = 0
	riverReject(t, s, 0, offer)
	riverConserved(t, s)
}

func TestCatanRiversCoinSalesPortsAndImmediateVictory(t *testing.T) {
	for _, rate := range []int{4, 3, 2} {
		s := riversFixture(t, 3)
		g := s.Catan
		color := 0
		if rate < 4 {
			for _, port := range g.Ports {
				if rate == 3 && port.Resource < 0 || rate == 2 && port.Resource == color {
					v := g.Edges[port.Edge].A
					g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
					break
				}
			}
		}
		if g.rates(0)[color] != rate {
			t.Fatal("port fixture")
		}
		catanGive(g, 0, color, rate)
		if err := s.Apply(0, Action{Type: "catan_coin_sell", Color: color}); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Rivers.Gold[0] != 1 || s.Catan.Players[0].Resources[color] != 0 || s.Catan.Rivers.Bought != 0 {
			t.Fatal("sale rate or buy limit")
		}
		riverConserved(t, s)
	}
	s := riversFixture(t, 3)
	g := s.Catan
	built := 0
	for _, v := range g.Vertices {
		if built < 5 && g.canSettlement(0, v.ID, true) {
			g.Vertices[v.ID].Owner = 0
			g.Vertices[v.ID].Level = 2
			if built == 4 {
				g.Vertices[v.ID].Level = 1
			}
			built++
		}
	}
	if built != 5 {
		t.Fatal("nine-point legal buildings fixture")
	}
	for p, n := range []int{34, 34, 32} {
		riverGold(g, p, n)
	}
	s.catanScores()
	if g.Players[0].Score != 9 {
		t.Fatal("score fixture")
	}
	catanGive(g, 0, 0, g.rates(0)[0])
	if err := s.Apply(0, Action{Type: "catan_coin_sell", Color: 0}); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || !slices.Equal(s.Winners, []int{0}) || s.Catan.Players[0].Score != 10 || s.Catan.Rivers.GoldIssued != 1 {
		t.Fatal("wealth victory delayed")
	}
}

func TestCatanRiversDoubleProductionRobberAndCoinPrivacy(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	tile := g.Tiles[g.Rivers.Map.DoubleNumberTile]
	g.Vertices[tile.Vertices[0]].Owner, g.Vertices[tile.Vertices[0]].Level = 0, 2
	riverGold(g, 0, 12)
	for _, number := range []int{2, 12} {
		next := clone(*s)
		if err := next.catanRollProduction(number); err != nil {
			t.Fatal(err)
		}
		if next.Catan.Players[0].Resources[tile.Resource] != 2 {
			t.Fatal("double production missing")
		}
		riverConserved(t, &next)
	}
	g.Robber = tile.ID
	if err := s.catanRollProduction(2); err != nil {
		t.Fatal(err)
	}
	if sum(g.Players[0].Resources) != 0 || g.Rivers.Gold[0] != 12 {
		t.Fatal("robber or coins")
	}
	if err := s.catanRollProduction(7); err != nil {
		t.Fatal(err)
	}
	if s.Phase == "catan_discard" || sum(g.DiscardDue) != 0 {
		t.Fatal("coins counted as hand")
	}
	for _, viewer := range []int{-1, 0, 1} {
		v := s.View(viewer)["catan"].(map[string]any)
		r := v["rivers"].(map[string]any)
		if int(r["gold"].([]any)[0].(float64)) != 12 {
			t.Fatal("coins must be public")
		}
		if viewer != 0 {
			if _, ok := v["players"].([]any)[0].(map[string]any)["resources"]; ok {
				t.Fatal("private resource leak")
			}
		}
	}
}

func TestCatanRiversEliminationAndOldGameIsolation(t *testing.T) {
	s := riversFixture(t, 6)
	g := s.Catan
	riverGold(g, 0, 20)
	riverGold(g, 1, 5)
	g.Rivers.Bought = 2
	if err := s.EliminateCatan(0); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.Gold[0] != 0 || s.Catan.Rivers.Bought != 0 || s.Catan.Rivers.Bank != 147 {
		t.Fatal("departing coins or next cap")
	}
	rich, poor := s.Catan.riverWealth()
	if rich != 1 || slices.Contains(poor, 0) {
		t.Fatal("eliminated wealth")
	}
	riverConserved(t, s)
	base, _ := NewCatan(3, CatanOptions{})
	base.Catan.SetupStep = 6
	base.Phase = "catan_turn"
	riverReject(t, base, 0, Action{Type: "catan_trade_offer", Give: make([]int, 5), Take: []int{1, 0, 0, 0, 0}, GoldGive: 1})
	riverReject(t, base, 0, Action{Type: "catan_bridge", Edge: 0})
	if !reflect.DeepEqual(base.Catan.Rivers, (*CatanRivers)(nil)) {
		t.Fatal("base acquired expansion")
	}
}

func TestCatanRiversBotsComplete(t *testing.T) {
	bridges, buys, sales := 0, 0, 0
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := NewCatanRivers(n, CatanOptions{FiveSix: n > 4})
			if err != nil {
				t.Fatal(err)
			}
			steps := 0
			for !s.Finished && steps < 4000 {
				actor := s.Turn
				if s.Phase == "catan_discard" {
					for p, due := range s.Catan.DiscardDue {
						if due > 0 {
							actor = p
							break
						}
					}
				}
				a, err := s.catanBot(actor)
				if err != nil {
					t.Fatal(err)
				}
				if err = s.Apply(actor, a); err != nil {
					t.Fatalf("step %d phase %s action %+v: %v", steps, s.Phase, a, err)
				}
				if a.Type == "catan_bridge" {
					bridges++
				}
				if a.Type == "catan_coin_buy" {
					buys++
				}
				if a.Type == "catan_coin_sell" {
					sales++
				}
				riverConserved(t, s)
				if steps%29 == 0 {
					*s = clone(*s)
				}
				steps++
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 10 {
				t.Fatal("rivers did not finish", steps)
			}
			t.Logf("players=%d steps=%d rounds=%d winner=%d", n, steps, s.Round, s.Winners[0])
		})
	}
	if bridges == 0 || buys == 0 || sales == 0 {
		t.Fatal("batch missed river actions", bridges, buys, sales)
	}
	t.Log("river actions", bridges, buys, sales)
}

func TestCatanRiversBridgeEarnsLongestRoute(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	bridge := g.Rivers.Map.Bridges[0]
	e := g.Edges[bridge]
	path := []int{}
	seen := map[int]bool{e.A: true, e.B: true}
	var walk func(int) bool
	walk = func(v int) bool {
		if len(path) == 4 {
			return true
		}
		for _, id := range g.touching(v) {
			if slices.Contains(g.Rivers.Map.Bridges, id) {
				continue
			}
			edge := g.Edges[id]
			next := edge.A
			if next == v {
				next = edge.B
			}
			if seen[next] {
				continue
			}
			seen[next] = true
			path = append(path, id)
			if walk(next) {
				return true
			}
			path = path[:len(path)-1]
			delete(seen, next)
		}
		return false
	}
	if !walk(e.A) {
		t.Fatal("no simple riverbank approach")
	}
	for _, id := range path {
		g.Edges[id].Owner = 0
	}
	s.catanScores()
	if g.LongestOwner != -1 || g.Players[0].RoadLength != 4 {
		t.Fatal("initial route")
	}
	catanGive(g, 0, 0, 1)
	catanGive(g, 0, 1, 2)
	if err := s.Apply(0, Action{Type: "catan_bridge", Edge: bridge}); err != nil {
		t.Fatal(err)
	}
	g = s.Catan
	roads, _, _ := g.pieces(0)
	if g.LongestOwner != 0 || g.Players[0].RoadLength != 5 || roads != 4 || g.bridgeCount(0) != 1 {
		t.Fatal("bridge route award")
	}
	// An opponent's building at the junction breaks the route at that vertex.
	g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 1, 1
	s.catanScores()
	if g.LongestOwner != -1 || g.Players[0].RoadLength != 4 {
		t.Fatal("opponent did not split mixed road/bridge route")
	}
}

func TestCatanRiversOutOfTurnWealthVictoryAndBotTrade(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	built := 0
	for _, v := range g.Vertices {
		if built < 5 && g.canSettlement(1, v.ID, true) {
			g.Vertices[v.ID].Owner = 1
			g.Vertices[v.ID].Level = 2
			if built == 4 {
				g.Vertices[v.ID].Level = 1
			}
			built++
		}
	}
	if built != 5 {
		t.Fatal("buildings fixture")
	}
	for p, n := range []int{5, 4, 1} {
		riverGold(g, p, n)
	}
	s.catanScores()
	catanGive(g, 1, 0, 1)
	offer := Action{Type: "catan_trade_offer", Give: make([]int, 5), Take: []int{1, 0, 0, 0, 0}, GoldGive: 2}
	if err := s.Apply(0, offer); err != nil {
		t.Fatal(err)
	}
	a, err := s.catanBot(1)
	if err != nil || a.Type != "catan_trade_accept" {
		t.Fatal("bot ignored fair coin payment", a, err)
	}
	if err = s.Apply(1, a); err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(0, Action{Type: "catan_trade_complete", Offer: s.Catan.Trade.ID, Target: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Finished || s.Catan.Players[1].Score != 10 {
		t.Fatal("non-active player victory timing")
	}
	if err = s.Apply(0, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || !slices.Equal(s.Winners, []int{1}) {
		t.Fatal("next-turn wealth victory missing")
	}
	s = riversFixture(t, 3)
	g = s.Catan
	catanGive(g, 0, 0, 1)
	if err = s.Apply(0, Action{Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0}, Take: make([]int, 5), GoldTake: 1}); err != nil {
		t.Fatal(err)
	}
	a, err = s.catanBot(1)
	if err != nil || a.Type != "catan_trade_reject" {
		t.Fatal("bot accepted unaffordable coin offer")
	}
}

func TestCatanRiversMonopolyAndRobberNeverTakeGold(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	riverGold(g, 1, 9)
	catanGive(g, 1, 0, 3)
	at := slices.Index(g.DevDeck, 3)
	g.DevDeck = slices.Delete(g.DevDeck, at, at+1)
	g.Players[0].Dev[3]++
	if err := s.Apply(0, Action{Type: "catan_dev", Card: 3, Color: 0}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.Gold[1] != 9 || s.Catan.Players[1].Resources[0] != 0 {
		t.Fatal("monopoly coins")
	}
	g = s.Catan
	catanGive(g, 1, 2, 1)
	s.Phase = "catan_steal"
	g.Victims = []int{1}
	g.ResumePhase = "catan_turn"
	if err := s.Apply(0, Action{Type: "catan_steal", Target: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.Gold[1] != 9 || s.Catan.Players[1].Resources[2] != 0 {
		t.Fatal("robber coins")
	}
	riverConserved(t, s)
}

func TestCatanRiversRequestedCoinsAndChangedHoldings(t *testing.T) {
	s := riversFixture(t, 3)
	g := s.Catan
	catanGive(g, 0, 0, 2)
	riverGold(g, 1, 4)
	if err := s.Apply(0, Action{Type: "catan_trade_offer", Give: []int{2, 0, 0, 0, 0}, Take: make([]int, 5), GoldTake: 3}); err != nil {
		t.Fatal(err)
	}
	id := s.Catan.Trade.ID
	if err := s.Apply(1, Action{Type: "catan_trade_accept", Offer: id}); err != nil {
		t.Fatal(err)
	}
	riverGold(s.Catan, 1, 2)
	riverReject(t, s, 0, Action{Type: "catan_trade_complete", Offer: id, Target: 1})
	riverGold(s.Catan, 1, 4)
	if err := s.Apply(0, Action{Type: "catan_trade_complete", Offer: id, Target: 1}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Rivers.Gold[0] != 3 || s.Catan.Rivers.Gold[1] != 1 || s.Catan.Players[1].Resources[0] != 2 {
		t.Fatal("requested coins settlement")
	}
	// Empty resource supply rejects the whole purchase and keeps its allowance.
	g = s.Catan
	amount := g.Bank[4]
	g.Players[2].Resources[4] += amount
	g.Bank[4] = 0
	riverReject(t, s, 0, Action{Type: "catan_coin_buy", Color: 4})
	if g.Rivers.Bought != 0 {
		t.Fatal("failed purchase used allowance")
	}
	riverConserved(t, s)
}
