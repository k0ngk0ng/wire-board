package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func attackTransportTravelFixture(t *testing.T, n int) (*Catan, *catanAttackTransportBoard, *catanAttackTransportPieces, *catanAttackTransportTravel, []int, int) {
	t.Helper()
	g, b, err := newCatanAttackTransportBoard(n)
	if err != nil {
		t.Fatal(err)
	}
	if n == 2 {
		s, e := NewCatanTransport(2)
		if e != nil {
			t.Fatal(e)
		}
		g.Two = s.Catan.Two
	}
	p, err := newCatanAttackTransportPieces(g, b)
	if err != nil {
		t.Fatal(err)
	}
	// ID deliberately exceeds the ordinary transport's three-piece array.
	id := len(p.Barbarians) - 1
	tile := b.Attack.Coast[1]
	edge := p.edges(g, tile, -1)[0]
	p.Barbarians[id] = catanAttackTransportBarbarian{tile, edge, -1}
	q, err := newCatanAttackTransportTravel(g, b, p, 0, g.Edges[edge].A, 1)
	if err != nil {
		t.Fatal(err)
	}
	gold := make([]int, n)
	for i := range gold {
		gold[i] = 5
	}
	return g, b, p, q, gold, id
}
func TestCatanAttackTransportDriveOff(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		g, b, p, q, gold, id := attackTransportTravelFixture(t, n)
		edge := p.Barbarians[id].Edge
		step, err := q.quote(g, b, p, gold, edge)
		if err != nil || step.MP != 4 {
			t.Fatal(n, "blocking", step, err)
		}
		before := clone(*q)
		if _, err = q.driveOff(g, b, p, gold, 0, 6); err == nil || !reflect.DeepEqual(before, *q) {
			t.Fatal("center barbarian driven off")
		}
		ok, err := q.driveOff(g, b, p, gold, id, 6)
		if err != nil || !ok || q.Travel.Pending != id {
			t.Fatal("drive", err)
		}
		before = clone(*q)
		pieces := clone(*p)
		if _, err = q.move(g, b, p, gold, edge); err == nil || !reflect.DeepEqual(before, *q) {
			t.Fatal("move with pending response")
		}
		if err = q.stop(g, b, p, gold); err == nil {
			t.Fatal("stop with pending response")
		}
		if err = q.relocate(g, b, p, gold, p.Barbarians[id].Tile, edge); err == nil || !reflect.DeepEqual(before, *q) || !reflect.DeepEqual(pieces, *p) {
			t.Fatal("invalid relocation mutated")
		}
		raw, _ := json.Marshal(q)
		var restored catanAttackTransportTravel
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if err = restored.validate(g, b, p, gold); err != nil {
			t.Fatal("pending restore", err)
		}
		*q = restored
		inland := 9
		if n > 4 {
			inland = 18
		}
		dest := p.edges(g, inland, id)[0]
		if err = q.relocate(g, b, p, gold, inland, dest); err != nil {
			t.Fatal(err)
		}
		if q.Travel.Pending != -1 || !q.Attempted[id] || p.blocking(edge) != -1 || p.blocking(dest) != id {
			t.Fatal("relocate desync")
		}
		step, err = q.quote(g, b, p, gold, edge)
		if err != nil || step.MP != 2 {
			t.Fatal("old route remains blocked", step, err)
		}
		if _, err = q.driveOff(g, b, p, gold, id, 6); err == nil {
			t.Fatal("repeat attempt")
		}
		q.Travel.Position = g.Edges[dest].A
		fresh, e := newCatanAttackTransportTravel(g, b, p, 0, q.Travel.Position, 1)
		if e != nil {
			t.Fatal(e)
		}
		ok, err = fresh.driveOff(g, b, p, gold, id, 1)
		if err != nil || ok || !fresh.Attempted[id] || fresh.Travel.Pending != -1 {
			t.Fatal("failed attempt", err)
		}
		if _, err = fresh.driveOff(g, b, p, gold, id, 6); err == nil {
			t.Fatal("failed attempt retried")
		}
		if err = p.capture(g, b, id, 0); err != nil {
			t.Fatal(err)
		}
		step, err = fresh.quote(g, b, p, gold, dest)
		if err != nil || step.MP != 2 {
			t.Fatal("captive blocks route", step, err)
		}
	}
}
func TestCatanAttackTransportMoveTollsAndArrival(t *testing.T) {
	for _, n := range []int{2, 4, 6} {
		g, b, p, q, gold, id := attackTransportTravelFixture(t, n)
		edge := p.Barbarians[id].Edge
		g.Edges[edge].Owner = 1
		step, err := q.move(g, b, p, gold, edge)
		if err != nil || step.MP != 3 || gold[0] != 4 || gold[1] != 6 || q.Travel.Points != 2 {
			t.Fatal("toll/blocker", step, err, gold)
		}
		site := b.Transport.Sites[0]
		edge = site.Paths[0]
		q, err = newCatanAttackTransportTravel(g, b, p, 0, g.Edges[edge].A, 1)
		if err != nil {
			t.Fatal(err)
		}
		if q.Travel.Position == site.Center {
			q.Travel.Position = g.Edges[edge].B
		}
		if _, err = q.move(g, b, p, gold, edge); err != nil {
			t.Fatal(err)
		}
		if q.Travel.Arrived != 0 || q.Travel.Position != site.Center || !q.Travel.Ended || q.Travel.Points != 0 {
			t.Fatal("delivery does not stop")
		}
		if n == 2 {
			q, err = newCatanAttackTransportTravel(g, b, p, 0, g.Edges[0].A, 4)
			if err != nil {
				t.Fatal(err)
			}
			g.Edges[0].Owner = -2
			before := slices.Clone(gold)
			first, e := q.move(g, b, p, gold, 0)
			if e != nil {
				t.Fatal(e)
			}
			second, e := q.move(g, b, p, gold, 0)
			if e != nil {
				t.Fatal(e)
			}
			if !first.Neutral || first.Bank != 1 || second.Bank != 0 || second.Pay != 1 || gold[0] != before[0]-2 || gold[1] != before[1]+1 || q.Travel.NeutralTolls != 2 {
				t.Fatal("neutral toll", first, second, gold)
			}
		}
	}
}
func TestCatanAttackTransportTravelGuards(t *testing.T) {
	g, b, p, q, gold, id := attackTransportTravelFixture(t, 3)
	for _, corrupt := range []func(*catanAttackTransportTravel){
		func(v *catanAttackTransportTravel) { v.Attempted = v.Attempted[:3] },
		func(v *catanAttackTransportTravel) { v.Travel.Pending = 0; v.Attempted[0] = true },
		func(v *catanAttackTransportTravel) { v.Travel.Pending = id },
		func(v *catanAttackTransportTravel) { v.Travel.Attempted[0] = true },
		func(v *catanAttackTransportTravel) { v.Travel.Points = 99 },
	} {
		v := clone(*q)
		corrupt(&v)
		if v.validate(g, b, p, gold) == nil {
			t.Fatal("corrupt save accepted", v)
		}
	}
	g.Bank = []int{19, 19, 19, 18, 19}
	g.Players[0].Resources = []int{0, 0, 0, 1, 0}
	if err := q.wheat(g, b, p, gold); err != nil {
		t.Fatal(err)
	}
	if g.Bank[3] != 19 || g.Players[0].Resources[3] != 0 || q.Travel.Points != 7 {
		t.Fatal("wheat accounting")
	}
	if err := q.wheat(g, b, p, gold); err == nil {
		t.Fatal("double wheat")
	}
	if err := q.stop(g, b, p, gold); err != nil {
		t.Fatal(err)
	}
	if err := q.validate(g, b, p, gold); err != nil {
		t.Fatal(err)
	}
}

func TestCatanAttackTransportInlandBattleFreesRoutes(t *testing.T) {
	for _, n := range []int{2, 4, 6} {
		g, b, p, _, _, id := attackTransportTravelFixture(t, n)
		tile := 9
		if n > 4 {
			tile = 18
		}
		edge := p.edges(g, tile, id)[0]
		if err := p.relocate(g, b, id, tile, edge); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(p.battleTiles(g), tile) {
			t.Fatal("inland invaders excluded from battle")
		}
		seats := n
		if n == 2 {
			seats++
		}
		prizes := make([]int, seats)
		before := clone(*p)
		if err := p.captureBattle(g, b, tile, prizes); err == nil || !reflect.DeepEqual(before, *p) {
			t.Fatal("incomplete capture mutated")
		}
		winner := n - 1
		captor := winner
		if n == 2 {
			winner = 2
			captor = -2
		}
		prizes[winner] = 1
		if err := p.captureBattle(g, b, tile, prizes); err != nil {
			t.Fatal(err)
		}
		if p.blocking(edge) != -1 || p.Barbarians[id].Captor != captor || slices.Contains(p.battleTiles(g), tile) {
			t.Fatal("battle leaves route blocked")
		}
		if err := p.validate(g, b); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCatanAttackTransportRelocateWithinUnconqueredHex(t *testing.T) {
	g, b, p, q, gold, id := attackTransportTravelFixture(t, 3)
	tile := p.Barbarians[id].Tile
	oldEdge := p.Barbarians[id].Edge
	initialCount := p.counts(g)[tile]
	var edge int
	for _, candidate := range p.edges(g, tile, id) {
		if candidate != oldEdge {
			edge = candidate
			break
		}
	}
	if ok, err := q.driveOff(g, b, p, gold, id, 6); err != nil || !ok {
		t.Fatal(err)
	}
	if err := q.relocate(g, b, p, gold, tile, edge); err != nil {
		t.Fatal(err)
	}
	if p.counts(g)[tile] != initialCount || p.blocking(oldEdge) != -1 || p.blocking(edge) != id {
		t.Fatal("same-hex reassignment")
	}
}
