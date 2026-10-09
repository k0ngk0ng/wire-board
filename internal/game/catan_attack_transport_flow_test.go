package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func attackTransportStarted(t *testing.T, n int) *State {
	t.Helper()
	s, err := newCatanAttackTransportState(n)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; s.Catan.setup() && i < 60; i++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	if s.Catan.setup() {
		t.Fatal("setup incomplete")
	}
	for i := 0; s.Phase != "catan_turn" && i < 40; i++ {
		p := twoFullActor(s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.Apply(p, a); err != nil {
			t.Fatal(err)
		}
	}
	if s.Phase != "catan_turn" {
		t.Fatal("production incomplete")
	}
	return s
}
func TestCatanAttackTransportBattleThenDelivery(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackTransportStarted(t, n)
		g := s.Catan
		actor := s.Turn
		tile := 9
		if n > 4 {
			tile = 18
		}
		// Inland fighting must free the same blockers the cargo wagon observes.
		pieces := &g.AttackTransport.Pieces
		for i := range pieces.Barbarians {
			pieces.Barbarians[i] = catanAttackTransportBarbarian{-1, -1, -1}
		}
		edges := pieces.edges(g, tile, -1)
		for i := range 3 {
			pieces.Barbarians[i] = catanAttackTransportBarbarian{tile, edges[i], -1}
		}
		g.syncAttackTransportCounts()
		attackBattleKnights(s, tile, actor, actor, actor, actor, actor, actor)
		tr := g.Transport
		other := (actor + 1) % n
		tr.Gold[other] += tr.GoldBank
		tr.GoldBank = 0
		before := slices.Clone(tr.Gold)
		if err := s.catanAttackResolveEnd(nil, attackDice(t, 1)); err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		tr = g.Transport
		if s.Turn != actor || s.Phase != "catan_transport_move" || tr.Travel == nil || g.Attack.Prisoners[actor] != 3 || g.Attack.Barbarians[tile] != 0 {
			t.Fatal("combat did not continue to same player's wagon")
		}
		reward := g.Attack.End.Battles[0].Gold[actor]
		if reward <= 0 || tr.Gold[actor] != before[actor]+reward || tr.GoldIssued != reward || len(g.Attack.Gold) != 0 {
			t.Fatal("battle gold not shared", reward, tr.Gold)
		}
		for _, e := range edges[:3] {
			if g.AttackTransport.Pieces.blocking(e) != -1 {
				t.Fatal("captives block route")
			}
		}
		if err := s.Apply(actor, Action{Type: "catan_transport_stop", Offer: int(tr.Sequence)}); err != nil {
			t.Fatal(err)
		}
		if s.Turn == actor || s.Phase == "catan_transport_move" {
			t.Fatal("wagon did not finish turn")
		}
		if err := s.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCatanAttackTransportConqueredDeliveryAndGold(t *testing.T) {
	s := attackTransportStarted(t, 3)
	g := s.Catan
	actor := s.Turn
	tport := g.Transport
	site := 1
	tile := tport.Map.Sites[site].Tile
	for g.AttackTransport.Pieces.counts(g)[tile] < 3 {
		if _, err := g.AttackTransport.Pieces.land(g, g.attackTransportBoard(), tile); err != nil {
			t.Fatal(err)
		}
	}
	g.syncAttackTransportCounts()
	if g.tileProduces(g.Tiles[tile], g.Tiles[tile].Number) {
		t.Fatal("conquered commodity produces")
	}
	// Load a compatible real token and move along an actual depot path.
	token := 0
	for i, stack := range tport.Stacks {
		for j, id := range stack {
			c, _ := tport.token(id)
			if tport.Map.accepts(site, c.Cargo) {
				token = id
				tport.Stacks[i] = slices.Delete(stack, j, j+1)
				break
			}
		}
		if token != 0 {
			break
		}
	}
	if token == 0 {
		t.Fatal("no cargo")
	}
	path := tport.Map.Sites[site].Paths[0]
	e := g.Edges[path]
	pos := e.A
	if pos == tport.Map.Sites[site].Center {
		pos = e.B
	}
	tport.Wagons[actor].Cargo = token
	tport.Wagons[actor].Position = pos
	tport.Wagons[actor].Level = 4
	if err := s.catanTransportBeginTravel(actor); err != nil {
		t.Fatal(err)
	}
	if _, err := tport.move(g, actor, tport.Sequence, path); err != nil {
		t.Fatal(err)
	}
	other := (actor + 1) % 3
	tport.Gold[other] += tport.GoldBank
	tport.GoldBank = 0
	before := tport.Gold[actor]
	result, err := tport.resolveArrival(g, actor, tport.Sequence, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Delivered != token || result.Loaded == 0 || result.Gold != 5 || tport.Gold[actor] != before+5 || tport.GoldIssued != 5 || !g.Attack.conquered(tile) {
		t.Fatal("conquest affected delivery", result)
	}
	if err = s.catanAfterTransportTravel(); err != nil {
		t.Fatal(err)
	}
	if err = s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
}
func TestCatanAttackTransportTreasonAndCoins(t *testing.T) {
	for _, n := range []int{2, 6} {
		s := attackTransportStarted(t, n)
		g := s.Catan
		p := s.Turn
		plans := g.attackTreasonPlans()
		if len(plans[0].Sources) != 2 {
			t.Fatal("no full treason")
		}
		before := g.Transport.Gold[p]
		if err := s.catanAttackTreason(p, plans[0].Sources, plans[0].Destinations[:2]); err != nil {
			t.Fatal(err)
		}
		if g.Transport.Gold[p] != before+2 || len(g.Attack.Gold) != 0 {
			t.Fatal("treason separate ledger")
		}
		if err := s.validateCatanTransport(); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := s.Apply(p, Action{Type: "catan_coin_buy", Color: 0}); err != nil {
				t.Fatal(err)
			}
		}
		raw, _ := json.Marshal(s)
		if err := s.Apply(p, Action{Type: "catan_coin_buy", Color: 0}); err == nil {
			t.Fatal("third coin purchase")
		}
		after, _ := json.Marshal(s)
		if string(raw) != string(after) {
			t.Fatal("failed purchase mutated")
		}
		if g.Attack.Bought != 0 {
			t.Fatal("duplicate purchase ledger")
		}
	}
}
func TestCatanAttackTransportCorruptSaveAndDeparture(t *testing.T) {
	s := attackTransportStarted(t, 3)
	for _, bad := range []func(*Catan){
		func(g *Catan) { g.AttackTransport.Rules = "" }, func(g *Catan) { g.Attack.Map.Transport = "" }, func(g *Catan) { g.Transport.Map.Attack = "" },
		func(g *Catan) { g.Attack.Gold = []int{0, 0, 0} }, func(g *Catan) { g.Attack.Barbarians[0]++ }, func(g *Catan) { g.Attack.Prisoners[0]++ },
		func(g *Catan) { g.Transport.Swift = true }, func(g *Catan) { g.Transport.GoldBank++ }, func(g *Catan) { g.Transport.Barbarians = [3]int{0, 1, 2} },
	} {
		copy := clone(*s)
		bad(copy.Catan)
		if copy.validateCatanTransport() == nil {
			t.Fatal("bad save accepted")
		}
	}
	before := sum(s.Catan.Transport.Gold) + s.Catan.Transport.GoldBank
	actor := s.Turn
	if err := s.EliminateCatan(actor); err != nil {
		t.Fatal(err)
	}
	if !s.Catan.Players[actor].Eliminated || s.Catan.Transport.Gold[actor] != 0 || sum(s.Catan.Transport.Gold)+s.Catan.Transport.GoldBank != before {
		t.Fatal("departure ledger")
	}
	if err := s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
}
func TestCatanAttackTransportSevenDoesNotMove(t *testing.T) {
	s := attackTransportStarted(t, 3)
	g := s.Catan
	before := clone(g.AttackTransport.Pieces)
	g.Dice = []int{3, 4}
	g.RollID++
	if err := s.catanRollProduction(7); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, g.AttackTransport.Pieces) || s.Phase == "catan_transport_barbarian" || g.Transport.BarbarianPending {
		t.Fatal("seven moved invaders")
	}
}

func TestCatanAttackTransportTimeoutAndPrivateState(t *testing.T) {
	s := attackTransportStarted(t, 3)
	actor := s.Turn
	if err := s.Apply(actor, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_transport_move" {
		t.Fatal("end did not reach wagon")
	}
	// Reject stale sequence without changing state, then continue with autoplay.
	before, _ := json.Marshal(s)
	if err := s.Apply(actor, Action{Type: "catan_transport_stop", Offer: int(s.Catan.Transport.Sequence) + 1}); err == nil {
		t.Fatal("stale move accepted")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("stale move mutated state")
	}
	for i := 0; s.Turn == actor && i < 30; i++ {
		s.AutoCatanPending()
	}
	if s.Turn == actor {
		t.Fatal("timeout did not complete transportation")
	}
	if err := s.validateCatanTransport(); err != nil {
		t.Fatal(err)
	}
	for viewer := -1; viewer < 3; viewer++ {
		v := s.View(viewer)["catan"].(map[string]any)
		attack := v["attack"].(map[string]any)
		if attack["deck"] != nil {
			t.Fatal("attack deck leaked")
		}
		raw, err := json.Marshal(v["transport"])
		if err != nil {
			t.Fatal(err)
		}
		var tr map[string]any
		if err = json.Unmarshal(raw, &tr); err != nil {
			t.Fatal(err)
		}
		state := tr["state"].(map[string]any)
		if state["stacks"] != nil {
			t.Fatal("cargo stack order leaked")
		}
	}
}

func TestCatanAttackTransportSharedBorderReassignment(t *testing.T) {
	s := attackTransportStarted(t, 2)
	g := s.Catan
	p := &g.AttackTransport.Pieces
	edge, one, two := -1, -1, -1
	for _, e := range g.Edges {
		if len(e.Tiles) == 2 && g.Tiles[e.Tiles[0]].Resource != catanCastle && g.Tiles[e.Tiles[1]].Resource != catanCastle {
			edge, one, two = e.ID, e.Tiles[0], e.Tiles[1]
			break
		}
	}
	if edge < 0 {
		t.Fatal("no shared border")
	}
	id := len(p.Barbarians) - 1
	p.Barbarians[id] = catanAttackTransportBarbarian{one, edge, -1}
	if err := p.relocate(g, g.attackTransportBoard(), id, two, edge); err != nil {
		t.Fatal(err)
	}
	if p.Barbarians[id].Tile != two || p.blocking(edge) != id {
		t.Fatal("shared border changed blocker identity")
	}
	before := clone(*p)
	if err := p.relocate(g, g.attackTransportBoard(), id, two, edge); err == nil || !reflect.DeepEqual(before, *p) {
		t.Fatal("no-op relocation accepted")
	}
}

func TestCatanAttackTransportProductionExtremeAndImmediateWin(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackTransportStarted(t, n)
		s.Phase = "catan_roll"
		g := s.Catan
		before := sum(g.AttackTransport.Pieces.counts(g))
		calls := 0
		if err := s.catanTransportRoll(func() [2]int { calls++; return [2]int{6, 6} }); err != nil {
			t.Fatal(err)
		}
		want := 1
		if n > 4 {
			want = 2
		}
		if calls != 1 || sum(g.AttackTransport.Pieces.counts(g)) != before+want {
			t.Fatal("12 rerolled or failed invasion")
		}
		// Winning with prisoners must end before creating a wagon response.
		s = attackTransportStarted(t, n)
		g = s.Catan
		p := s.Turn
		for i := range g.AttackTransport.Pieces.Barbarians {
			g.AttackTransport.Pieces.Barbarians[i] = catanAttackTransportBarbarian{-1, -1, -1}
		}
		g.syncAttackTransportCounts()
		s.catanScores()
		needed := 2*(14-g.Players[p].Score) - 1
		if needed < 0 || needed+1 >= len(g.AttackTransport.Pieces.Barbarians) {
			t.Fatal("bad victory fixture")
		}
		for i := 0; i < needed; i++ {
			g.AttackTransport.Pieces.Barbarians[i].Captor = p
		}
		tile := 9
		if n > 4 {
			tile = 18
		}
		edge := g.AttackTransport.Pieces.edges(g, tile, -1)[0]
		g.AttackTransport.Pieces.Barbarians[needed] = catanAttackTransportBarbarian{tile, edge, -1}
		g.syncAttackTransportCounts()
		attackBattleKnights(s, tile, p, p)
		s.catanScores()
		if g.Players[p].Score != 13 {
			t.Fatal("pre-win score", g.Players[p].Score)
		}
		if err := s.catanAttackResolveEnd(nil, func() int { t.Fatal("victory should precede loss die"); return 1 }); err != nil {
			t.Fatal(err)
		}
		if !s.Finished || s.Phase != "finished" || s.Catan.Transport.Travel != nil || !slices.Equal(s.Winners, []int{p}) {
			t.Fatal("victory allowed transportation")
		}
	}
}
