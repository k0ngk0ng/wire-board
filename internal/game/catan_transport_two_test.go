package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func transportTwoState(t *testing.T, setup bool) *State {
	t.Helper()
	s, err := newCatanTransportState(2, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; setup && s.Catan.setup() && i < 20; i++ {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, s.Turn, a)
	}
	if setup && s.Catan.setup() {
		t.Fatal("unfinished setup")
	}
	return s
}

// Uses the real production resolver and atomic after-action hook, with an
// explicit server dice stream. There is no client-controlled dice action.
func transportTwoRoll(t *testing.T, s *State, dice ...[2]int) int {
	t.Helper()
	next := clone(*s)
	used := 0
	err := next.catanTransportRoll(func() [2]int {
		if used >= len(dice) {
			t.Fatal("unexpected extra roll")
		}
		result := dice[used]
		used++
		return result
	})
	if err == nil {
		err = next.catanTwoAfterAction(s, Action{Type: "catan_roll"})
	}
	if err == nil {
		err = next.validateCatanTransport()
	}
	if err != nil {
		t.Fatal(err)
	}
	*s = next
	return used
}

func transportTwoActionPhase(t *testing.T, s *State) {
	t.Helper()
	transportTwoRoll(t, s, [2]int{1, 2})
	transportTwoRoll(t, s, [2]int{5, 6})
	if s.Phase != "catan_turn" {
		t.Fatal(s.Phase)
	}
}

func TestCatanTransportTwoSetupAndDoubleProduction(t *testing.T) {
	s := transportTwoState(t, false)
	g := s.Catan
	for i, pair := range [][2]int{{1, 1}, {17, 4}} {
		v := g.Vertices[g.Tiles[pair[0]].Vertices[pair[1]]]
		r, n, c := g.pieces(-2 - i)
		if v.Owner != -2-i || v.Level != 1 || r != 0 || n != 1 || c != 0 {
			t.Fatal("neutral setup", v, r, n, c)
		}
	}
	// Give the first settlement a commodity reward, without mistaking an
	// internal path (one incident hex) for a coastal boundary.
	vertex := -1
	for _, site := range g.Transport.Map.Sites {
		for _, v := range g.Tiles[site.Tile].Vertices {
			if !g.canSettlement(s.Turn, v, true) {
				continue
			}
			coastal := false
			for _, edge := range g.touching(v) {
				if edge < 72 && len(g.edgeTiles(edge)) == 1 {
					coastal = true
				}
			}
			want := 1
			if coastal {
				want++
			}
			if g.twoSettlementTokens(s.Turn, v) != want {
				t.Fatal("commodity/coastal token reward", v, want)
			}
			if !coastal {
				vertex = v
			}
		}
	}
	if vertex < 0 {
		t.Fatal("no inland commodity corner")
	}
	p := s.Turn
	helperApply(t, s, p, Action{Type: "catan_settlement", Vertex: vertex})
	if s.Catan.Two.Tokens[p] != 6 {
		t.Fatal("setup commodity token missing")
	}
	for s.Catan.setup() {
		before := s.Catan.Two.Tokens[s.Turn]
		actor := s.Turn
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
		if a.Type == "catan_city" && before != s.Catan.Two.Tokens[actor] {
			t.Fatal("initial city incorrectly paid settlement chips")
		}
	}
	for p, w := range s.Catan.Transport.Wagons {
		r, n, c := s.Catan.pieces(p)
		if r != 2 || n != 1 || c != 1 || s.Catan.Vertices[w.Position].Owner != p {
			t.Fatal("player setup")
		}
	}
	for _, dice := range [][][2]int{{{1, 1}, {6, 6}, {2, 3}}, {{2, 3}, {6, 6}, {3, 3}}} {
		before := clone(*s)
		n := dice[len(dice)-1][0] + dice[len(dice)-1][1]
		expected := make([][]int, 2)
		for p := range expected {
			expected[p] = slices.Clone(s.Catan.Players[p].Resources)
		}
		for _, tile := range s.Catan.Tiles {
			if tile.Number != n {
				continue
			}
			for _, id := range tile.Vertices {
				v := s.Catan.Vertices[id]
				if v.Owner >= 0 {
					expected[v.Owner][tile.Resource] += v.Level
				}
			}
		}
		if used := transportTwoRoll(t, s, dice...); used != len(dice) {
			t.Fatal("did not reroll", used)
		}
		if s.Catan.RollID != before.Catan.RollID+1 {
			t.Fatal("extra roll ID")
		}
		for p, want := range expected {
			if !slices.Equal(want, s.Catan.Players[p].Resources) {
				t.Fatal("production differs", p, want, s.Catan.Players[p].Resources)
			}
		}
		s = transportRestoreState(t, s)
	}
	if s.Phase != "catan_turn" || !slices.Equal(s.Catan.Two.Rolls, []int{5, 6}) {
		t.Fatal("double production", s.Phase, s.Catan.Two.Rolls)
	}
	helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	seq := int(s.Catan.Transport.Sequence)
	helperApply(t, s, s.Turn, Action{Type: "catan_transport_stop", Offer: seq})
	if s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 0 || s.Catan.Two.Spent {
		t.Fatal("next-turn reset")
	}
}

func TestCatanTransportTwoSevenAndNeutralResponses(t *testing.T) {
	for _, first := range []bool{true, false} {
		s := transportTwoState(t, true)
		if !first {
			transportTwoRoll(t, s, [2]int{1, 2})
		}
		for p := range 2 {
			catanMove(s.Catan.Players[p].Resources, s.Catan.Bank, slices.Clone(s.Catan.Players[p].Resources))
			transportGive(s.Catan, p, []int{4, 4, 0, 0, 0})
		}
		transportTwoRoll(t, s, [2]int{3, 4})
		if s.Phase != "catan_discard" {
			t.Fatal("missing discard")
		}
		for p := range 2 {
			helperApply(t, s, p, Action{Type: "catan_discard", Tokens: []int{2, 2, 0, 0, 0}})
		}
		if s.Phase != "catan_transport_barbarian" {
			t.Fatal("missing barbarian")
		}
		p := s.Turn
		g := s.Catan
		target := -1
		for _, e := range g.Edges {
			if e.Owner == 1-p && !slices.Contains(g.Transport.Barbarians[:], e.ID) {
				target = e.ID
				break
			}
		}
		if target < 0 {
			t.Fatal("no enemy road")
		}
		s = transportRestoreState(t, s)
		helperApply(t, s, p, Action{Type: "catan_transport_barbarian", Card: 0, Edge: target, Offer: int(s.Catan.Transport.BarbarianSequence)})
		want := "catan_turn"
		if first {
			want = "catan_roll"
		}
		if s.Phase != want || sum(s.Catan.Players[1-p].Resources) != 3 || sum(s.Catan.Players[p].Resources) != 5 {
			t.Fatal("seven continuation", s.Phase)
		}
	}
	s := transportTwoState(t, true)
	transportGiveDev(t, s, s.Turn, 1)
	helperApply(t, s, s.Turn, Action{Type: "catan_dev", Card: 1})
	for i := 0; i < 2; i++ {
		a, err := s.BotAction(s.Turn)
		if err != nil {
			t.Fatal(err)
		}
		if a.Type != "catan_road" {
			t.Fatal(a)
		}
		helperApply(t, s, s.Turn, a)
		if s.Phase != "catan_two_build" {
			t.Fatal("no neutral build")
		}
		s = transportRestoreState(t, s)
		helperReject(t, s, s.Turn, Action{Type: "catan_end"})
		helperReject(t, s, s.Turn, Action{Type: "catan_transport_upgrade"})
		s.AutoCatanPending()
	}
	if s.Phase != "catan_roll" || len(s.Catan.Two.Rolls) != 0 {
		t.Fatal("free roads did not restore preroll", s.Phase)
	}
	transportTwoActionPhase(t, s)
}

func TestCatanTransportTwoTradeChipBarbarianRetreat(t *testing.T) {
	s := transportTwoState(t, true)
	p := s.Turn
	// Artificial leading-player fixture with a delivered physical token,
	// transferred from supply. Hidden victory cards do not raise this cost.
	tr := s.Catan.Transport
	tr.Wagons[p].Delivered = append(tr.Wagons[p].Delivered, tr.Stacks[0][0])
	tr.Stacks[0] = tr.Stacks[0][1:]
	s.catanScores()
	if s.Catan.twoTokenCost(p) != 2 {
		t.Fatal("fixture not leading")
	}
	g := s.Catan
	q := g.Two
	q.Bank += q.Tokens[p] - 1
	q.Tokens[p] = 1
	edges := g.twoTransportRetreatEdges()
	if len(edges) == 0 || len(g.twoRetreatTiles()) != 0 {
		t.Fatal("retreat must choose roadless edges")
	}
	for _, e := range g.Edges {
		if e.Owner != -1 || slices.Contains(g.Transport.Barbarians[:], e.ID) {
			helperReject(t, s, p, Action{Type: "catan_two_robber", Card: 0, Edge: e.ID})
		}
	}
	helperReject(t, s, 1-p, Action{Type: "catan_two_robber", Card: 0, Edge: edges[0]})
	helperReject(t, s, p, Action{Type: "catan_two_robber", Card: 3, Edge: edges[0]})
	before := clone(*s)
	helperApply(t, s, p, Action{Type: "catan_two_robber", Card: 0, Edge: edges[0]})
	if s.Catan.Two.Tokens[p] != 0 || s.Catan.Two.Bank != before.Catan.Two.Bank+1 || !s.Catan.Two.Spent || s.Catan.Transport.Barbarians[0] != edges[0] || s.Catan.Robber != -1 || s.Phase != "catan_roll" {
		t.Fatal("retreat payment/state")
	}
	for player := range 2 {
		if !reflect.DeepEqual(before.Catan.Players[player], s.Catan.Players[player]) || !reflect.DeepEqual(before.Catan.Transport.Wagons[player], s.Catan.Transport.Wagons[player]) {
			t.Fatal("retreat stole resources or spent delivered cargo")
		}
	}
	helperReject(t, s, p, Action{Type: "catan_two_robber", Card: 1, Edge: edges[1]})
	s = transportRestoreState(t, s)
	transportTwoActionPhase(t, s)
	helperApply(t, s, p, Action{Type: "catan_end"})
	helperReject(t, s, p, Action{Type: "catan_two_robber", Card: 1, Edge: edges[1]})
}

func TestCatanTransportTwoNeutralTollsAggregateAndSwift(t *testing.T) {
	s := transportTwoState(t, true)
	transportTwoActionPhase(t, s)
	p := s.Turn
	g := s.Catan
	// Explicit movement-cost fixture: one neutral road, no barbarian; crossing
	// back and forth must pay 1/2/3 total, not round separately at each edge.
	edge := -1
	for _, e := range g.Edges {
		if e.Owner == -1 && g.Transport.Map.canBuildRoad(g, e.ID) && !slices.Contains(g.Transport.Barbarians[:], e.ID) && e.A < 54 && e.B < 54 {
			edge = e.ID
			break
		}
	}
	if edge < 0 {
		t.Fatal("no fixture edge")
	}
	g.Edges[edge].Owner = -2
	g.Transport.Wagons[p].Position = g.Edges[edge].A
	transportGiveDev(t, s, p, 2)
	helperApply(t, s, p, Action{Type: "catan_dev", Card: 2})
	helperApply(t, s, p, Action{Type: "catan_end"})
	initial := clone(*s.Catan.Transport)
	for i := 1; i <= 3; i++ {
		tr := s.Catan.Transport
		step, err := tr.Travel.quote(s.Catan, tr.Map, tr.Barbarians, tr.Gold, edge)
		if err != nil || !step.Neutral || step.MP != 1 || step.Toll != 1 || step.Bank != i%2 {
			t.Fatal("neutral quote", i, step, err)
		}
		helperApply(t, s, p, Action{Type: "catan_transport_step", Offer: int(tr.Sequence), Edge: edge})
		tr = s.Catan.Transport
		if tr.Gold[p] != initial.Gold[p]-i || tr.Gold[1-p] != initial.Gold[1-p]+i/2 || tr.GoldBank != initial.GoldBank+(i+1)/2 || tr.Travel.NeutralTolls != i {
			t.Fatal("aggregate split", i, tr.Gold, tr.GoldBank)
		}
		s = transportRestoreState(t, s)
	}
	tr := s.Catan.Transport
	helperApply(t, s, p, Action{Type: "catan_transport_stop", Offer: int(tr.Sequence)})
	tr = s.Catan.Transport
	if tr.Moves != 2 || tr.Travel.NeutralTolls != 0 {
		t.Fatal("Swift is a separate movement")
	}
	step, err := tr.Travel.quote(s.Catan, tr.Map, tr.Barbarians, tr.Gold, edge)
	if err != nil || step.Bank != 1 {
		t.Fatal("Swift bank rounding", step, err)
	}
	helperApply(t, s, p, Action{Type: "catan_transport_step", Offer: int(tr.Sequence), Edge: edge})
	// Zero balance must reject both odd/even neutral crossings atomically.
	tr = s.Catan.Transport
	tr.GoldBank += tr.Gold[p]
	tr.Gold[p] = 0
	helperReject(t, s, p, Action{Type: "catan_transport_step", Offer: int(tr.Sequence), Edge: edge})
	if err = s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
	// Corrupt persisted aggregate and multiplayer neutral ownership rejected.
	bad := clone(*s)
	bad.Catan.Transport.Travel.NeutralTolls = 20
	if bad.validateCatanTransport() == nil {
		t.Fatal("corrupt total accepted")
	}
	multi := transportState(t, true)
	multi.Catan.Edges[0].Owner = -2
	if multi.validateCatanTransport() == nil {
		t.Fatal("neutral road in multiplayer")
	}
}

func TestCatanTransportTwoForcedTradePrivacy(t *testing.T) {
	s := transportTwoState(t, true)
	p := s.Turn
	transportGive(s.Catan, 1-p, []int{1, 1, 1, 1, 1})
	helperApply(t, s, p, Action{Type: "catan_two_trade"})
	if s.Phase != "catan_two_trade" {
		t.Fatal("no forced trade")
	}
	for _, viewer := range []int{-1, 0, 1} {
		v := s.View(viewer)["catan"].(map[string]any)
		trade := v["two"].(map[string]any)["trade"].(map[string]any)
		if (trade["drawn"] != nil) != (viewer == p) {
			t.Fatal("private forced draw", viewer)
		}
		for seat, raw := range v["players"].([]any) {
			if seat != viewer && raw.(map[string]any)["resources"] != nil {
				t.Fatal("private resources", viewer, seat)
			}
		}
		if _, ok := v["transport"].(map[string]any)["stacks"]; ok {
			t.Fatal("hidden cargo supply")
		}
	}
	helperReject(t, s, p, Action{Type: "catan_end"})
	helperReject(t, s, 1-p, Action{Type: "catan_two_return", Give: []int{2, 0, 0, 0, 0}})
	s = transportRestoreState(t, s)
	before, _ := json.Marshal(s)
	s.AutoCatanPending()
	after, _ := json.Marshal(s)
	if string(before) == string(after) || s.Phase != "catan_roll" {
		t.Fatal("forced trade timeout stuck")
	}
}

func TestCatanTransportTwoMixedRoadCostsAndBotBudget(t *testing.T) {
	s := transportTwoState(t, true)
	transportTwoActionPhase(t, s)
	p := s.Turn
	g := s.Catan
	// Independent mixed-road fixture. Choose a real five-edge path avoiding
	// centers and barbarians, then assign road owners and an upgraded wagon.
	var path []int
	var start int
	var walk func(int, []int, map[int]bool) bool
	walk = func(at int, edges []int, seen map[int]bool) bool {
		if len(edges) == 5 {
			path = slices.Clone(edges)
			return true
		}
		for _, id := range g.touching(at) {
			e := g.Edges[id]
			if id >= 72 || !g.Transport.Map.canBuildRoad(g, id) || slices.Contains(g.Transport.Barbarians[:], id) {
				continue
			}
			to := e.A
			if to == at {
				to = e.B
			}
			if seen[to] {
				continue
			}
			seen[to] = true
			if walk(to, append(edges, id), seen) {
				return true
			}
			delete(seen, to)
		}
		return false
	}
	for v := 0; v < 54; v++ {
		if walk(v, nil, map[int]bool{v: true}) {
			start = v
			break
		}
	}
	if len(path) != 5 {
		t.Fatal("missing path")
	}
	owners := []int{-2, p, 1 - p, -3, -1}
	for i, id := range path {
		g.Edges[id].Owner = owners[i]
	}
	g.Transport.Wagons[p].Position = start
	g.Transport.Wagons[p].Level = 4
	s.catanScores()
	helperApply(t, s, p, Action{Type: "catan_end"})
	initial := clone(*s.Catan.Transport)
	for _, edge := range path {
		helperApply(t, s, p, Action{Type: "catan_transport_step", Edge: edge, Offer: int(s.Catan.Transport.Sequence)})
	}
	tr := s.Catan.Transport
	if tr.Travel.Points != 1 || tr.Travel.NeutralTolls != 2 || tr.Gold[p] != initial.Gold[p]-3 || tr.Gold[1-p] != initial.Gold[1-p]+2 || tr.GoldBank != initial.GoldBank+1 {
		t.Fatal("normal toll affected neutral aggregate", tr)
	}
	s = transportRestoreState(t, s)
	// A public path planner cannot treat a neutral road as a free, empty edge.
	g = s.Catan
	for i := range g.Edges {
		g.Edges[i].Owner = -1
	}
	from := -1
	for v := 0; v < 54; v++ {
		ok := true
		for _, e := range g.touching(v) {
			ok = ok && g.Transport.Map.canBuildRoad(g, e)
		}
		if ok {
			from = v
			break
		}
	}
	if from < 0 {
		t.Fatal("no planner start")
	}
	for _, id := range g.touching(from) {
		g.Edges[id].Owner = -2
	}
	to := g.Transport.Map.Sites[0].Center
	if path := catanTransportPath(g, g.Transport.Map, from, to, p, 0); path != nil {
		t.Fatal("planned unaffordable neutral departure", path)
	}
	if path := catanTransportPath(g, g.Transport.Map, from, to, p, 1); len(path) == 0 {
		t.Fatal("affordable neutral departure rejected")
	}
}

func TestCatanTransportTwoBotRetreatAndCorruptPhase(t *testing.T) {
	s := transportTwoState(t, true)
	g := s.Catan
	p := s.Turn
	from := g.Transport.Wagons[p].Position
	edge := -1
	for _, id := range g.touching(from) {
		if !slices.Contains(g.Transport.Barbarians[:], id) {
			edge = id
			break
		}
	}
	if edge < 0 {
		t.Fatal("missing adjacent edge")
	}
	g.Transport.Barbarians[0] = edge
	a, ok := s.catanTwoOptionalBot(p)
	if !ok || a.Type != "catan_two_robber" || a.Card != 0 {
		t.Fatal("bot did not remove wagon obstruction", a)
	}
	helperApply(t, s, p, a)
	if s.Catan.Transport.Barbarians[0] != a.Edge || s.Catan.Two.Tokens[p] != g.Two.Tokens[p]-1 {
		t.Fatal("bot retreat failed")
	}
	for _, phase := range []string{"catan_two_build", "catan_two_trade"} {
		multi := transportState(t, true)
		multi.Phase = phase
		if multi.validateCatanTransport() == nil {
			t.Fatal("multiplayer accepted two-player phase", phase)
		}
	}
	transportTwoActionPhase(t, s)
	helperApply(t, s, p, Action{Type: "catan_end"})
	s.Catan.Two.Rolls = nil
	if s.validateCatanTransport() == nil {
		t.Fatal("movement bypassed double production")
	}
}
