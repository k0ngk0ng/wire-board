package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func newAttackState(t *testing.T, n int, setup bool) *State {
	t.Helper()
	s, e := newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
	if e != nil {
		t.Fatal(e)
	}
	if !setup {
		s.Catan.SetupStep = s.Catan.SetupLimit()
		s.Catan.TurnSerial = 1
		s.Phase = "catan_turn"
	}
	return s
}
func attackHand(s *State, p int, values []int) {
	g := s.Catan
	for c, v := range values {
		g.Bank[c] += g.Players[p].Resources[c] - v
		g.Players[p].Resources[c] = v
	}
}
func attackCoins(s *State, p, value int) {
	a := s.Catan.Attack
	a.GoldBank += a.Gold[p] - value
	a.Gold[p] = value
}
func assertAttackRestored(t *testing.T, s *State) {
	t.Helper()
	data, _ := json.Marshal(s)
	var restored State
	if e := json.Unmarshal(data, &restored); e != nil {
		t.Fatal(e)
	}
	if e := restored.validateCatanAttack(); e != nil {
		t.Fatal(e)
	}
	after, _ := json.Marshal(&restored)
	if string(data) != string(after) {
		t.Fatal("restore changed state")
	}
	for p := -1; p < len(s.Catan.Players); p++ {
		if !reflect.DeepEqual(s.View(p), restored.View(p)) {
			t.Fatal("restore changed private view", p)
		}
	}
}
func attackReject(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	before, _ := json.Marshal(s)
	if e := s.Apply(p, a); e == nil {
		t.Fatal("invalid action accepted", a)
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("rejected action mutated state", a)
	}
}
func finishAttackSetup(t *testing.T, s *State) {
	t.Helper()
	n := len(s.Catan.Players)
	steps := 0
	for s.Catan.setup() && steps < 4*n {
		actor := s.Turn
		phase := s.Phase
		g := s.Catan
		a, e := s.BotAction(actor)
		if e != nil {
			t.Fatal(e)
		}
		wantGain := make([]int, 5)
		if phase == "catan_setup_city" {
			if a.Type != "catan_city" {
				t.Fatal("bot tried second settlement")
			}
			for _, tile := range g.Tiles {
				if tile.Resource < 5 && slices.Contains(tile.Vertices, a.Vertex) {
					wantGain[tile.Resource]++
				}
			}
		}
		if e = s.Apply(actor, a); e != nil {
			t.Fatal(phase, a, e)
		}
		if phase == "catan_setup_city" && !slices.Equal(s.Catan.Players[actor].Resources, wantGain) {
			t.Fatal("starting city did not receive exactly one per resource hex")
		}
		if s.Catan.Attack.Sequence != 0 {
			t.Fatal("initial construction triggered landing")
		}
		assertAttackRestored(t, s)
		steps++
	}
	if steps != 4*n || s.Catan.setup() || s.Phase != "catan_roll" || s.Turn != s.Catan.StartPlayer {
		t.Fatal("initial placement progression", steps, s.Phase)
	}
	for p, seat := range s.Catan.Players {
		roads, villages, cities := s.Catan.pieces(p)
		if roads != 2 || villages != 1 || cities != 1 || seat.Score != 3 {
			t.Fatal("starting pieces/score", p, roads, villages, cities, seat.Score)
		}
	}
}
func TestCatanAttackActualSetupAndEditionGuards(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for trial := 0; trial < 4; trial++ {
			s := newAttackState(t, n, true)
			finishAttackSetup(t, s)
			if s.Catan.victoryTarget() != 12 || s.Catan.ArmyOwner != -1 || s.Catan.Robber != -1 || len(s.Catan.DevDeck) != 0 {
				t.Fatal("base rules leaked")
			}
		}
	}
	for _, c := range []struct {
		n int
		o CatanOptions
	}{{2, CatanOptions{}}, {7, CatanOptions{FiveSix: true}}, {4, CatanOptions{FiveSix: true}}, {5, CatanOptions{}}, {3, CatanOptions{Helpers: true}}, {3, CatanOptions{Rules: "wrong"}}} {
		if _, e := newCatanAttackState(c.n, c.o); e == nil {
			t.Fatal("accepted unsupported recipe")
		}
	}
	s := newAttackState(t, 3, false)
	s.Catan.Attack.Rules = "corrupt"
	attackReject(t, s, s.Turn, Action{Type: "catan_end"})
}
func scriptedAttackDice(t *testing.T, values ...[2]int) func() [2]int {
	t.Helper()
	i := 0
	return func() [2]int {
		if i >= len(values) {
			t.Fatal("unexpected additional landing roll")
		}
		v := values[i]
		i++
		return v
	}
}
func TestCatanAttackLandingDistinctNumbersDuplicateCoastsAndRestore(t *testing.T) {
	s := newAttackState(t, 6, false)
	g := s.Catan
	g.Dice = []int{1, 2}
	g.RollID = 21
	before := clone(g.Players)
	roll := scriptedAttackDice(t, [2]int{3, 4}, [2]int{2, 3}, [2]int{2, 3}, [2]int{3, 6}, [2]int{1, 1})
	if e := s.catanAttackLanding(roll); e != nil {
		t.Fatal(e)
	}
	a := g.Attack
	if len(a.Landing.Rolls) != 3 || a.Sequence != 1 || a.supply() != 41 || g.RollID != 21 || !slices.Equal(g.Dice, []int{1, 2}) {
		t.Fatal("landing changed production or counts", a.Landing, a.supply())
	}
	for i, count := range []int{2, 2, 1} {
		if len(a.Landing.Rolls[i].Tiles) != count {
			t.Fatal("duplicate 5/9 not resolved together")
		}
	}
	if !reflect.DeepEqual(g.Players, before) {
		t.Fatal("landing produced resources")
	}
	assertAttackRestored(t, s)
	// An already conquered target consumes the distinct number; no replacement.
	for _, id := range a.Map.Coast {
		if g.Tiles[id].Number == 5 {
			a.Barbarians[id] = 3
		}
	}
	if e := s.catanAttackLanding(scriptedAttackDice(t, [2]int{1, 4}, [2]int{4, 5}, [2]int{1, 2})); e != nil {
		t.Fatal(e)
	}
	if len(a.Landing.Rolls[0].Tiles) != 0 || len(a.Landing.Rolls) != 3 {
		t.Fatal("rerolled conquered number")
	}
	assertAttackRestored(t, s)
}
func TestCatanAttackLandingSupplyAndInvalidRollAreAtomic(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, false)
		a := s.Catan.Attack
		captives := a.supply() - 1
		for i := 0; i < captives; i++ {
			a.Prisoners[i%n]++
		}
		before, _ := json.Marshal(s)
		if n == 6 {
			if e := s.catanAttackLanding(scriptedAttackDice(t, [2]int{2, 3})); e == nil {
				t.Fatal("invented insufficient double-target priority")
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("supply guard partially mutated landing")
			}
		}
		if e := s.catanAttackLanding(scriptedAttackDice(t, [2]int{1, 1})); e != nil {
			t.Fatal(e)
		}
		if a.supply() != 0 || len(a.Landing.Rolls) != 1 {
			t.Fatal("did not stop at empty supply")
		}
		if e := s.catanAttackLanding(scriptedAttackDice(t)); e != nil {
			t.Fatal(e)
		}
		if len(a.Landing.Rolls) != 0 || a.Sequence != 2 {
			t.Fatal("empty supply should not roll")
		}
		assertAttackRestored(t, s)
	}
	s := newAttackState(t, 3, false)
	before, _ := json.Marshal(s)
	if e := s.catanAttackLanding(scriptedAttackDice(t, [2]int{1, 2}, [2]int{0, 6})); e == nil {
		t.Fatal("invalid dice accepted")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("invalid dice partly changed board")
	}
}
func TestCatanAttackActualBuildingTriggersAndImmediateVictory(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, true)
		finishAttackSetup(t, s)
		s.Phase = "catan_turn"
		p := s.Turn
		village := -1
		for _, v := range s.Catan.Vertices {
			if v.Owner == p && v.Level == 1 {
				village = v.ID
			}
		}
		attackHand(s, p, []int{0, 0, 0, 2, 3})
		attackReject(t, s, (p+1)%n, Action{Type: "catan_city", Vertex: village})
		if e := s.Apply(p, Action{Type: "catan_city", Vertex: village}); e != nil {
			t.Fatal(e)
		}
		if s.Catan.Vertices[village].Level != 2 || sum(s.Catan.Players[p].Resources) != 0 || s.Catan.Attack.Sequence != 1 || len(s.Catan.Attack.Landing.Rolls) != 3 {
			t.Fatal("upgrade/landing not completed together")
		}
		assertAttackRestored(t, s)
		// Ordinary roads never cause a landing.
		edge := -1
		for _, e := range s.Catan.Edges {
			if s.Catan.canRoad(p, e.ID) {
				edge = e.ID
				break
			}
		}
		if edge < 0 {
			t.Fatal("road fixture")
		}
		attackHand(s, p, []int{1, 1, 0, 0, 0})
		if e := s.Apply(p, Action{Type: "catan_road", Edge: edge}); e != nil {
			t.Fatal(e)
		}
		if s.Catan.Attack.Sequence != 1 {
			t.Fatal("road caused landing")
		}
	}
	s := newAttackState(t, 3, true)
	finishAttackSetup(t, s)
	s.Phase = "catan_turn"
	p := s.Turn
	s.Catan.Attack.Prisoners[p] = 16
	s.catanScores()
	if s.Catan.Players[p].Score != 11 {
		t.Fatal("victory fixture")
	}
	village := -1
	for _, v := range s.Catan.Vertices {
		if v.Owner == p && v.Level == 1 {
			village = v.ID
		}
	}
	attackHand(s, p, []int{0, 0, 0, 2, 3})
	if e := s.Apply(p, Action{Type: "catan_city", Vertex: village}); e != nil {
		t.Fatal(e)
	}
	if !s.Finished || s.Catan.Players[p].Score != 12 || s.Catan.Attack.Sequence != 0 {
		t.Fatal("immediate victory should precede further landing")
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackConquestProductionPortsScoresAndLegalActions(t *testing.T) {
	s := newAttackState(t, 3, false)
	g := s.Catan
	p := s.Turn
	outer, inner := g.Tiles[0].Vertices[4], g.Tiles[0].Vertices[1]
	g.Vertices[outer].Owner = p
	g.Vertices[outer].Level = 2
	g.Vertices[inner].Owner = (p + 1) % 3
	g.Vertices[inner].Level = 1
	// Make the printed port on hex 0 generic without changing the inventory.
	for i, port := range g.Ports {
		if port.Resource == -1 {
			g.Ports[0].Resource, g.Ports[i].Resource = g.Ports[i].Resource, g.Ports[0].Resource
			break
		}
	}
	g.Attack.Prisoners[p] = 3
	s.catanScores()
	if g.Players[p].Score != 3 || !slices.Equal(g.rates(p), []int{3, 3, 3, 3, 3}) {
		t.Fatal("active city/port or integer prisoner score")
	}
	color, total := g.Tiles[0].Resource, g.Tiles[0].Number
	if e := s.catanRollProduction(total); e != nil {
		t.Fatal(e)
	}
	if g.Players[p].Resources[color] != 2 || g.Players[(p+1)%3].Resources[color] != 1 {
		t.Fatal("unconquered production")
	}
	g.Attack.Barbarians[0] = 3
	s.catanScores()
	if g.Players[p].Score != 1 || g.Players[(p+1)%3].Score != 1 || !slices.Equal(g.rates(p), []int{4, 4, 4, 4, 4}) {
		t.Fatal("conquered-only city lost score/port but inland village stays active")
	}
	before := clone(g.Players)
	if e := s.catanRollProduction(total); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, g.Players) {
		t.Fatal("conquered hex produced resources")
	}
	attackHand(s, (p+1)%3, []int{0, 0, 0, 2, 3})
	s.Turn = (p + 1) % 3
	attackReject(t, s, s.Turn, Action{Type: "catan_city", Vertex: inner})
	legal := s.View(s.Turn)["catan"].(map[string]any)["legal"].(map[string][]int)
	if slices.Contains(legal["cities"], inner) {
		t.Fatal("blocked upgrade advertised")
	}
	public := s.View(-1)["catan"].(map[string]any)["attack"].(map[string]any)
	if !reflect.DeepEqual(public["conquered"], []int{0}) || !reflect.DeepEqual(public["conqueredBuildings"], []int{outer}) {
		t.Fatal("conquest public marks", public)
	}
	g.Attack.Barbarians[0] = 2
	s.catanScores()
	if g.Players[p].Score != 3 || !slices.Equal(g.rates(p), []int{3, 3, 3, 3, 3}) || !g.canCityUpgrade(s.Turn, inner) {
		t.Fatal("liberation did not restore score/port/build")
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackActualSettlementAndBlockedRoadAreAtomic(t *testing.T) {
	s := newAttackState(t, 3, false)
	g := s.Catan
	p := s.Turn
	v := g.Tiles[0].Vertices[4]
	edge := g.touching(v)[0]
	g.Edges[edge].Owner = p
	attackHand(s, p, []int{1, 1, 1, 1, 0})
	if e := s.Apply(p, Action{Type: "catan_settlement", Vertex: v}); e != nil {
		t.Fatal(e)
	}
	g = s.Catan
	if g.Vertices[v].Owner != p || g.Vertices[v].Level != 1 || sum(g.Players[p].Resources) != 0 || g.Attack.Sequence != 1 {
		t.Fatal("settlement/landing cost")
	}
	// Adjacent owned road connects the site, but conquest forbids new pieces.
	g.Attack.Barbarians[0] = 3
	attackHand(s, p, []int{1, 1, 0, 0, 0})
	candidate := -1
	for _, id := range g.touching(v) {
		if g.Edges[id].Owner == -1 {
			candidate = id
			break
		}
	}
	if candidate < 0 {
		t.Fatal("blocked road fixture")
	}
	attackReject(t, s, p, Action{Type: "catan_road", Edge: candidate})
	assertAttackRestored(t, s)
}

func TestCatanAttackSevenDiscardsThenStealsWithoutRobber(t *testing.T) {
	s := newAttackState(t, 3, false)
	p := s.Turn
	attackHand(s, p, []int{8, 0, 0, 0, 0})
	attackHand(s, (p+1)%3, []int{0, 2, 0, 0, 0})
	attackHand(s, (p+2)%3, []int{0, 0, 2, 0, 0})
	attackCoins(s, p, 80)
	if e := s.catanRollProduction(7); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_discard" || s.Catan.DiscardDue[p] != 4 {
		t.Fatal("coins counted as hand or missing discard")
	}
	attackReject(t, s, p, Action{Type: "catan_steal", Target: (p + 1) % 3})
	if e := s.Apply(p, Action{Type: "catan_discard", Tokens: []int{4, 0, 0, 0, 0}}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_steal" || len(s.Catan.Victims) != 2 || s.Catan.Robber != -1 {
		t.Fatal("seven requested robber placement")
	}
	assertAttackRestored(t, s)
	attackReject(t, s, p, Action{Type: "catan_robber", Tile: 0})
	attackReject(t, s, (p+1)%3, Action{Type: "catan_steal", Target: (p + 2) % 3})
	if e := s.Apply(p, Action{Type: "catan_steal", Target: (p + 1) % 3}); e != nil {
		t.Fatal(e)
	}
	if s.Phase != "catan_turn" || s.Catan.Players[p].Resources[1] != 1 || s.Catan.Attack.Gold[p] != 80 {
		t.Fatal("theft failed or stole gold")
	}
	assertAttackRestored(t, s)
	// A single eligible opponent is resolved automatically; no eligible hand ends it.
	for _, eligible := range []bool{true, false} {
		x := newAttackState(t, 3, false)
		q := (x.Turn + 1) % 3
		if eligible {
			attackHand(x, q, []int{0, 0, 1, 0, 0})
		}
		if e := x.catanRollProduction(7); e != nil {
			t.Fatal(e)
		}
		if x.Phase != "catan_turn" || x.Catan.Robber != -1 || len(x.Catan.Victims) != 0 {
			t.Fatal("empty/single theft did not finish")
		}
		if eligible && x.Catan.Players[x.Turn].Resources[2] != 1 {
			t.Fatal("single theft missing")
		}
	}
}

func TestCatanAttackCoinsTradeLimitsPairedTurnsAndNoWealthPoints(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, false)
		p := s.Turn
		attackCoins(s, p, 10)
		for _, color := range []int{0, 1} {
			if e := s.Apply(p, Action{Type: "catan_coin_buy", Color: color}); e != nil {
				t.Fatal(e)
			}
		}
		if s.Catan.Attack.Gold[p] != 6 || s.Catan.Attack.Bought != 2 || s.Catan.Players[p].Score != 0 {
			t.Fatal("coin count, limit, or invented wealth award")
		}
		attackReject(t, s, p, Action{Type: "catan_coin_buy", Color: 2})
		attackHand(s, p, []int{4, 1, 0, 0, 0})
		if e := s.Apply(p, Action{Type: "catan_coin_sell", Color: 0}); e != nil {
			t.Fatal(e)
		}
		if s.Catan.Attack.Gold[p] != 7 || s.Catan.Players[p].Resources[0] != 0 || s.Catan.Attack.Bought != 2 {
			t.Fatal("selling at 4:1 changed purchase allowance")
		}
		other := (p + 1) % n
		attackHand(s, other, []int{0, 0, 1, 0, 0})
		give, take := []int{0, 0, 0, 0, 0}, []int{0, 0, 1, 0, 0}
		if e := s.Apply(p, Action{Type: "catan_trade_offer", Give: give, Take: take, GoldGive: 2}); e != nil {
			t.Fatal(e)
		}
		id := s.Catan.Trade.ID
		if e := s.Apply(other, Action{Type: "catan_trade_accept", Offer: id}); e != nil {
			t.Fatal(e)
		}
		if e := s.Apply(p, Action{Type: "catan_trade_complete", Offer: id, Target: other}); e != nil {
			t.Fatal(e)
		}
		if s.Catan.Attack.Gold[p] != 5 || s.Catan.Attack.Gold[other] != 2 || s.Catan.Players[p].Resources[2] != 1 {
			t.Fatal("coin/player trade accounting")
		}
		assertAttackRestored(t, s)
		if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
			t.Fatal(e)
		}
		if s.Catan.Attack.Bought != 0 {
			t.Fatal("new actor inherited coin limit")
		}
		if n == 6 {
			if !s.Catan.Paired.Second || s.Phase != "catan_turn" {
				t.Fatal("paired secondary action")
			}
			second := s.Turn
			attackCoins(s, second, 4)
			if e := s.Apply(second, Action{Type: "catan_coin_buy", Color: 0}); e != nil {
				t.Fatal(e)
			}
			attackReject(t, s, second, Action{Type: "catan_trade_offer", Give: give, Take: take, GoldGive: 1})
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackHiddenDeckPublicInventoryAndUnsupportedCards(t *testing.T) {
	s := newAttackState(t, 3, false)
	attackHand(s, s.Turn, []int{0, 0, 1, 1, 1})
	for p := -1; p < 3; p++ {
		v := s.View(p)["catan"].(map[string]any)
		a := v["attack"].(map[string]any)
		if _, exists := a["deck"]; exists {
			t.Fatal("hidden special deck exposed", p)
		}
		if a["devRemaining"] != 26 || v["devRemaining"] != 26 || a["supply"] != 34 {
			t.Fatal("public supply sizes")
		}
		other := clone(*s)
		slices.Reverse(other.Catan.Attack.Deck)
		if !reflect.DeepEqual(s.View(p), other.View(p)) {
			t.Fatal("view depends on hidden deck order")
		}
		for owner, raw := range v["players"].([]any) {
			if owner != p && raw.(map[string]any)["resources"] != nil {
				t.Fatal("opponent hand leak")
			}
		}
	}
	attackReject(t, s, s.Turn, Action{Type: "catan_buy_dev"})
	attackReject(t, s, s.Turn, Action{Type: "catan_dev", Card: 0})
	s.Catan.Attack.Knights = append(s.Catan.Attack.Knights, catanAttackKnight{Player: s.Turn, Edge: 0})
	attackReject(t, s, s.Turn, Action{Type: "catan_end"}) // Do not silently skip an unfinished battle phase.
	eliminated := s.Turn
	attackCoins(s, eliminated, 7)
	if e := s.EliminateCatan(eliminated); e != nil {
		t.Fatal(e)
	}
	if s.Catan.Attack.Gold[eliminated] != 0 || s.Catan.Attack.GoldBank != 100 {
		t.Fatal("eliminated gold not returned")
	}
	if len(s.Catan.Attack.Knights) != 0 {
		t.Fatal("eliminated knight remained")
	}
	assertAttackRestored(t, s)
}
