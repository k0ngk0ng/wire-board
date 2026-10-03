package game

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func catanGame(t *testing.T, n int) *State {
	t.Helper()
	s, e := New("catan", n)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func catanCheck(t *testing.T, s *State) {
	t.Helper()
	g := s.Catan
	for c, b := range g.Bank {
		total := b
		if b < 0 {
			t.Fatal("negative bank")
		}
		for _, p := range g.Players {
			if p.Resources[c] < 0 {
				t.Fatal("negative hand")
			}
			total += p.Resources[c]
		}
		if total != 19 {
			t.Fatalf("resource %d conservation: %d", c, total)
		}
	}
	dev := len(g.DevDeck) + len(g.DevDiscard)
	for i, p := range g.Players {
		dev += sum(p.Dev)
		r, v, c := g.pieces(i)
		if r > 15 || v > 5 || c > 4 {
			t.Fatal("pieces exhausted")
		}
		for k, n := range p.NewDev {
			if n > p.Dev[k] || n < 0 {
				t.Fatal("invalid new cards")
			}
		}
	}
	if dev != 25 {
		t.Fatalf("development conservation %d", dev)
	}
	for _, e := range g.Edges {
		if g.Vertices[e.A].Level > 0 && g.Vertices[e.B].Level > 0 {
			t.Fatal("adjacent settlements")
		}
	}
}
func catanFinishSetup(t *testing.T, s *State) {
	t.Helper()
	for s.Catan.setup() {
		a, e := s.BotAction(s.Turn)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(s.Turn, a); e != nil {
			t.Fatal(e)
		}
		catanCheck(t, s)
	}
}
func catanGive(g *Catan, p, c, n int) { g.Players[p].Resources[c] += n; g.Bank[c] -= n }
func catanCard(g *Catan, p, kind int) {
	for i, c := range g.DevDeck {
		if c == kind {
			g.DevDeck = append(g.DevDeck[:i], g.DevDeck[i+1:]...)
			g.Players[p].Dev[kind]++
			return
		}
	}
}
func TestCatanMapSetupAndLimits(t *testing.T) {
	for _, n := range []int{2, 5} {
		if _, e := New("catan", n); e == nil {
			t.Fatal("accepted player count")
		}
	}
	for attempt := 0; attempt < 20; attempt++ {
		s := catanGame(t, 3+attempt%2)
		g := s.Catan
		if len(g.Tiles) != 19 || len(g.Vertices) != 54 || len(g.Edges) != 72 || len(g.Ports) != 9 {
			t.Fatalf("bad topology %d %d %d", len(g.Tiles), len(g.Vertices), len(g.Edges))
		}
		for i, a := range g.Tiles {
			if a.Number != 6 && a.Number != 8 {
				continue
			}
			for _, b := range g.Tiles[i+1:] {
				if (b.Number == 6 || b.Number == 8) && math.Hypot(a.X-b.X, a.Y-b.Y) < 110 {
					t.Fatal("adjacent red numbers")
				}
			}
		}
		expected := []int{}
		for i := range g.Players {
			expected = append(expected, i)
		}
		for i := len(g.Players) - 1; i >= 0; i-- {
			expected = append(expected, i)
		}
		for _, p := range expected {
			if s.Turn != p {
				t.Fatal("snake turn")
			}
			step := g.SetupStep
			s.AutoCatanPending()
			if g.SetupStep != step+1 {
				t.Fatal("setup timer did not complete exactly one pair")
			}
		}
		if s.Turn != 0 || s.Phase != "catan_roll" {
			t.Fatal("setup transition")
		}
		for i := range g.Players {
			r, v, c := g.pieces(i)
			if r != 2 || v != 2 || c != 0 {
				t.Fatal("setup pieces")
			}
		}
		catanCheck(t, s)
	}
}
func TestCatanRobberDiscardAndPrivacy(t *testing.T) {
	s := catanGame(t, 3)
	catanFinishSetup(t, s)
	g := s.Catan
	// Put hands into a known legal distribution, then trigger a seven.
	for i := range g.Players {
		catanMove(g.Players[i].Resources, g.Bank, append([]int{}, g.Players[i].Resources...))
	}
	catanGive(g, 0, 0, 9)
	catanGive(g, 1, 1, 8)
	catanGive(g, 2, 2, 7)
	s.catanRoll(7)
	if !reflect.DeepEqual(g.DiscardDue, []int{4, 4, 0}) {
		t.Fatal(g.DiscardDue)
	}
	if e := s.Apply(1, Action{Type: "catan_discard", Tokens: []int{0, 3, 0, 0, 0}}); e == nil {
		t.Fatal("wrong discard")
	}
	if e := s.Apply(1, Action{Type: "catan_discard", Tokens: []int{0, 4, 0, 0, 0}}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_discard" {
		t.Fatal("early robber")
	}
	s.AutoCatanPending()
	if s.Phase != "catan_robber" {
		t.Fatal(s.Phase)
	}
	catanCard(g, 0, 4)
	s.catanScores()
	view := s.View(-1)["catan"].(map[string]any)
	if _, ok := view["devDeck"]; ok {
		t.Fatal("deck exposed")
	}
	for _, raw := range view["players"].([]any) {
		p := raw.(map[string]any)
		for _, k := range []string{"resources", "dev", "newDev"} {
			if _, ok := p[k]; ok {
				t.Fatal("secret exposed", k)
			}
		}
	}
	p := view["players"].([]any)[0].(map[string]any)
	if int(p["score"].(int)) != g.Players[0].Score-1 {
		t.Fatal("secret victory point")
	}
	before := sum(g.Players[0].Resources)
	target := -1
	for _, tile := range g.Tiles {
		if tile.ID == g.Robber {
			continue
		}
		for _, v := range tile.Vertices {
			if g.Vertices[v].Owner == 1 {
				target = tile.ID
			}
		}
	}
	if target < 0 {
		t.Fatal("no robber target")
	}
	if e := s.Apply(0, Action{Type: "catan_robber", Tile: target}); e != nil {
		t.Fatal(e)
	}
	if s.Phase == "catan_steal" {
		if e := s.Apply(0, Action{Type: "catan_steal", Target: 1}); e != nil {
			t.Fatal(e)
		}
	}
	if sum(g.Players[0].Resources) != before+1 {
		t.Fatal("steal missing")
	}
	catanCheck(t, s)
}
func TestCatanTradesAndDev(t *testing.T) {
	s := catanGame(t, 3)
	catanFinishSetup(t, s)
	g := s.Catan
	s.Phase = "catan_turn"
	for i := range g.Players {
		catanMove(g.Players[i].Resources, g.Bank, append([]int{}, g.Players[i].Resources...))
	}
	catanGive(g, 0, 0, 5)
	catanGive(g, 1, 1, 3)
	give, take := []int{1, 0, 0, 0, 0}, []int{0, 1, 0, 0, 0}
	if e := s.Apply(1, Action{Type: "catan_trade_offer", Give: give, Take: take}); e == nil {
		t.Fatal("off-turn offer")
	}
	if e := s.Apply(0, Action{Type: "catan_trade_offer", Give: give, Take: take}); e != nil {
		t.Fatal(e)
	}
	id := g.Trade.ID
	if e := s.Apply(1, Action{Type: "catan_trade_accept", Offer: id - 1}); e == nil {
		t.Fatal("stale offer")
	}
	if e := s.Apply(1, Action{Type: "catan_trade_accept", Offer: id}); e != nil {
		t.Fatal(e)
	}
	if g.Players[0].Resources[0] != 5 {
		t.Fatal("accept committed trade without host choice")
	}
	if e := s.Apply(0, Action{Type: "catan_trade_complete", Offer: id, Target: 1}); e != nil {
		t.Fatal(e)
	}
	if g.Trade != nil || g.Players[0].Resources[1] != 1 {
		t.Fatal("trade result")
	}
	rate := g.rates(0)[0]
	give[0] = rate
	take = []int{0, 0, 1, 0, 0}
	if e := s.Apply(0, Action{Type: "catan_bank", Give: give, Take: take}); e != nil {
		t.Fatal(e)
	}
	catanCard(g, 0, 0)
	g.Players[0].NewDev[0] = 1
	if e := s.Apply(0, Action{Type: "catan_dev", Card: 0}); e == nil {
		t.Fatal("new development played")
	}
	g.Players[0].NewDev[0] = 0
	if e := s.Apply(0, Action{Type: "catan_dev", Card: 0}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_robber" || g.Players[0].Knights != 1 {
		t.Fatal("knight state")
	}
	a, e := s.BotAction(0)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Apply(0, a); e != nil {
		t.Fatal(e)
	}
	if s.Phase == "catan_steal" {
		a, _ = s.BotAction(0)
		_ = s.Apply(0, a)
	}
	catanCard(g, 0, 2)
	if e = s.Apply(0, Action{Type: "catan_dev", Card: 2, Take: []int{0, 0, 0, 1, 1}}); e == nil {
		t.Fatal("two dev cards in a turn")
	}
	catanCheck(t, s)
}
func TestCatanBotsFinishAndPreservePrivacy(t *testing.T) {
	for _, n := range []int{3, 4} {
		s := catanGame(t, n)
		steps := 0
		for !s.Finished && steps < 1800 {
			p := s.Turn
			if s.Phase == "catan_discard" {
				for i, d := range s.Catan.DiscardDue {
					if d > 0 {
						p = i
						break
					}
				}
			}
			before, _ := json.Marshal(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("bot mutated state")
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatalf("step %d phase %s action %+v: %v", steps, s.Phase, a, e)
			}
			catanCheck(t, s)
			steps++
		}
		if !s.Finished {
			t.Fatalf("%d-player bots did not finish in %d actions, round %d scores %+v", n, steps, s.Round, s.Catan.Players)
		}
		if len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 10 {
			t.Fatal("winner")
		}
		t.Logf("%d players finished in %d actions / round %d", n, steps, s.Round)
	}
	s := catanGame(t, 3)
	catanFinishSetup(t, s)
	s.Phase = "catan_turn"
	a, _ := s.BotAction(0)
	other := clone(*s)
	other.Catan.Players[1].Resources = []int{9, 0, 0, 0, 0}
	other.Catan.Players[1].Dev = []int{0, 0, 0, 0, 5}
	for i, j := 0, len(other.Catan.DevDeck)-1; i < j; i, j = i+1, j-1 {
		other.Catan.DevDeck[i], other.Catan.DevDeck[j] = other.Catan.DevDeck[j], other.Catan.DevDeck[i]
	}
	b, _ := other.BotAction(0)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("bot used hidden cards")
	}
}
func TestCatanPublicLogDoesNotRevealDevelopment(t *testing.T) {
	s := catanGame(t, 3)
	catanFinishSetup(t, s)
	s.Phase = "catan_turn"
	for c := 2; c < 5; c++ {
		catanGive(s.Catan, 0, c, 1)
	}
	other := clone(*s)
	s.Catan.DevDeck[len(s.Catan.DevDeck)-1] = 0
	other.Catan.DevDeck[len(other.Catan.DevDeck)-1] = 4
	_ = s.Apply(0, Action{Type: "catan_buy_dev"})
	_ = other.Apply(0, Action{Type: "catan_buy_dev"})
	if !reflect.DeepEqual(s.Log, other.Log) {
		t.Fatal("development leaked in log")
	}
	if strings.Contains(strings.Join(s.Log, " "), "购买一张骑士") {
		t.Fatal("hidden type")
	}
}

func TestCatanLongestRoadAndAwardTies(t *testing.T) {
	s := catanGame(t, 3)
	g := s.Catan
	g.Edges = nil
	g.Vertices = make([]CatanVertex, 8)
	for i := range g.Vertices {
		g.Vertices[i] = CatanVertex{ID: i, Owner: -1}
	}
	for i := 0; i < 6; i++ {
		g.Edges = append(g.Edges, CatanEdge{ID: i, A: i, B: i + 1, Owner: 0})
	}
	if g.roadLength(0) != 6 {
		t.Fatal("chain")
	}
	g.Vertices[3].Owner = 1
	g.Vertices[3].Level = 1
	if g.roadLength(0) != 3 {
		t.Fatal("road crosses opponent settlement")
	}
	g.Vertices[3].Level = 0
	g.Vertices[3].Owner = -1
	g.Edges[5].B = 0
	g.Edges = append(g.Edges, CatanEdge{ID: 6, A: 0, B: 6, Owner: 0})
	if g.roadLength(0) != 7 {
		t.Fatal("cycle and branch trail")
	}
	for _, x := range []struct {
		old    int
		values []int
		want   int
	}{{0, []int{5, 5, 4}, 0}, {0, []int{4, 6, 6}, -1}, {0, []int{5, 6, 4}, 1}, {-1, []int{4, 4, 4}, -1}} {
		if got := g.awardHolder(x.old, 5, x.values); got != x.want {
			t.Fatal("award tie", got, x)
		}
	}
}
func TestCatanProductionShortageAndCity(t *testing.T) {
	s := catanGame(t, 3)
	g := s.Catan
	t0 := &g.Tiles[0]
	t0.Resource = 0
	t0.Number = 4
	g.Robber = 1
	for i := 1; i < len(g.Tiles); i++ {
		g.Tiles[i].Number = 3
	}
	g.Vertices[t0.Vertices[0]].Owner = 0
	g.Vertices[t0.Vertices[0]].Level = 2
	g.SetupStep = 6
	s.catanRoll(4)
	if g.Players[0].Resources[0] != 2 {
		t.Fatal("city production")
	}
	g.Vertices[t0.Vertices[2]].Owner = 1
	g.Vertices[t0.Vertices[2]].Level = 1
	catanGive(g, 2, 0, g.Bank[0]-1)
	before := clone(g.Players)
	s.catanRoll(4)
	if !reflect.DeepEqual(before, g.Players) {
		t.Fatal("shortage paid multiple players")
	}
	g.Vertices[t0.Vertices[2]].Level = 0
	s.catanRoll(4)
	if g.Bank[0] != 0 || g.Players[0].Resources[0] != 3 {
		t.Fatal("sole claimant should receive remaining stock")
	}
	catanCheck(t, s)
}
func TestCatanAllDevelopmentsVictoryAndElimination(t *testing.T) {
	s := catanGame(t, 3)
	catanFinishSetup(t, s)
	g := s.Catan
	s.Phase = "catan_turn"
	catanCard(g, 0, 2)
	before := g.Players[0].Resources[3]
	if e := s.Apply(0, Action{Type: "catan_dev", Card: 2, Take: []int{0, 0, 0, 2, 0}}); e != nil {
		t.Fatal(e)
	}
	if g.Players[0].Resources[3] != before+2 {
		t.Fatal("plenty")
	}
	g.PlayedDev = false
	catanCard(g, 0, 3)
	want := g.Players[0].Resources[0] + g.Players[1].Resources[0] + g.Players[2].Resources[0]
	if e := s.Apply(0, Action{Type: "catan_dev", Card: 3, Color: 0}); e != nil {
		t.Fatal(e)
	}
	if g.Players[0].Resources[0] != want || g.Players[1].Resources[0] != 0 {
		t.Fatal("monopoly")
	}
	g.PlayedDev = false
	catanCard(g, 0, 1)
	beforeHand := append([]int{}, g.Players[0].Resources...)
	if e := s.Apply(0, Action{Type: "catan_dev", Card: 1}); e != nil {
		t.Fatal(e)
	}
	for s.Phase == "catan_roads" {
		a, e := s.BotAction(0)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(0, a); e != nil {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(beforeHand, g.Players[0].Resources) {
		t.Fatal("free roads charged")
	}
	catanCheck(t, s)
	if e := s.EliminateCatan(0); e != nil {
		t.Fatal(e)
	}
	if sum(g.Players[0].Resources) != 0 || !g.Players[0].Eliminated {
		t.Fatal("elimination")
	}
	catanCheck(t, s)
	if e := s.EliminateCatan(1); e != nil {
		t.Fatal(e)
	}
	if !s.Finished || s.Winners[0] != 2 {
		t.Fatal("last player")
	}
	s = catanGame(t, 3)
	catanFinishSetup(t, s)
	g = s.Catan
	for range 5 {
		catanCard(g, 0, 4)
	}
	for i := range g.Vertices {
		if g.Vertices[i].Owner == 0 {
			g.Vertices[i].Level = 2
		}
	}
	g.Players[0].Knights = 3
	s.catanScores()
	s.Turn = 2
	s.Phase = "catan_turn"
	if e := s.Apply(2, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	if !s.Finished || s.Winners[0] != 0 {
		t.Fatal("victory at own turn start")
	}
}
