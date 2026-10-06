package game

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"testing"
)

type explorerPirateFixture struct {
	G *Catan
	B *catanExplorerBoard
	F *catanExplorerSailing
	C *catanExplorerCargo
	E *catanExplorerEconomy
	P *catanExplorerPirate
}

// Official map topology plus explicit boat/hand fixtures, not a complete
// Pirate Lairs starting position or an end-to-end natural match.
func newExplorerPirateFixture(t *testing.T, n int) explorerPirateFixture {
	t.Helper()
	g, b, err := newCatanExplorerBoard(n, "pirate-lairs", "fixed")
	if err != nil {
		t.Fatal(err)
	}
	g.Bank = []int{19, 19, 19, 19, 19}
	for i := range g.Players {
		g.Players[i].Resources = make([]int, 5)
	}
	// Reveal using the real per-region number stacks; no boat is placed against
	// unresolved fog in this rule fixture. Gold fields remain uncaptured.
	for _, tile := range slices.Clone(b.Hidden) {
		if _, err = b.reveal(g, tile.Tile); err != nil {
			t.Fatal(err)
		}
	}
	f, _ := newCatanExplorerSailing(n)
	c, err := newCatanExplorerCargo(g, f, "pirate-lairs")
	if err != nil {
		t.Fatal(err)
	}
	e, err := newCatanExplorerEconomy(g, f, c)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.beginProduction(g, f, c, 0, 1); err != nil {
		t.Fatal(err)
	}
	q := explorerPirateFixture{g, b, f, c, e, newCatanExplorerPirate()}
	if err = q.P.validate(g, b, f, c, e); err != nil {
		t.Fatal(err)
	}
	return q
}
func (q explorerPirateFixture) snapshot() string { data, _ := json.Marshal(q); return string(data) }
func (q explorerPirateFixture) action(kind string, target int, decline bool, roll int) (catanExplorerTheft, error) {
	return q.P.apply(q.G, q.B, q.F, q.C, q.E, 0, q.E.Turn.Sequence, kind, target, decline, func(int) int { return roll })
}
func (q explorerPirateFixture) must(t *testing.T, kind string, target int, decline bool, roll int) catanExplorerTheft {
	t.Helper()
	r, err := q.action(kind, target, decline, roll)
	if err != nil {
		t.Fatal(kind, err)
	}
	return r
}
func (q explorerPirateFixture) reject(t *testing.T, kind string, target int, decline bool, roll int) {
	t.Helper()
	before := q.snapshot()
	if _, err := q.action(kind, target, decline, roll); err == nil {
		t.Fatal("accepted", kind, target)
	}
	if q.snapshot() != before {
		t.Fatal("failed action mutated pirate, hand or turn")
	}
}
func (q explorerPirateFixture) restore(t *testing.T) {
	t.Helper()
	before := q.snapshot()
	for _, v := range []any{q.G, q.B, q.F, q.C, q.E, q.P} {
		data, _ := json.Marshal(v)
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.P.validate(q.G, q.B, q.F, q.C, q.E); err != nil {
		t.Fatal(err)
	}
	if q.snapshot() != before {
		t.Fatal("restore differs")
	}
}
func (q explorerPirateFixture) edge(tile int) int {
	for _, e := range q.G.Edges {
		if slices.Contains(e.Tiles, tile) {
			return e.ID
		}
	}
	return -1
}
func (q explorerPirateFixture) give(player, color, n int) {
	q.G.Bank[color] -= n
	q.G.Players[player].Resources[color] += n
}

func TestCatanExplorerPiratePlacementAndSevenResume(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			q := newExplorerPirateFixture(t, n)
			if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{3, 4}); err != nil {
				t.Fatal(err)
			}
			q.must(t, "seven", 0, false, 0)
			q.restore(t)
			q.reject(t, "seven", 0, false, 0)
			for _, tile := range q.G.Tiles {
				// Independent hex-coordinate neighbor rule, not the edge helper under test.
				adjacent := false
				for _, start := range q.B.Starting {
					a := q.G.Tiles[start]
					adjacent = adjacent || math.Abs(math.Hypot(tile.X-a.X, tile.Y-a.Y)-math.Sqrt(3)*q.G.HexSize) < 0.01
				}
				legal := tile.Resource == CatanSea && tile.ID != q.B.FrameSea && !adjacent
				if catanExplorerPirateTile(q.G, q.B, tile.ID) != legal {
					t.Fatal("wrong legal tile", tile.ID)
				}
				if !legal {
					q.reject(t, "place", tile.ID, false, 0)
				}
			}
			target := q.P.destinations(q.G, q.B)[0]
			q.must(t, "place", target, false, 0)
			if q.P.Owner != 0 || q.P.Tile != target || q.P.Pending != nil || q.E.Turn.Phase != "ready" || q.C.Turn.Phase != "action" {
				t.Fatal("seven did not resume action phase")
			}
			q.restore(t)
			q.reject(t, "place", target, false, 0)
		})
	}
}
func TestCatanExplorerPirateTheftIsPerPlayerAndResourceWeighted(t *testing.T) {
	for draw := 0; draw < 4; draw++ {
		q := newExplorerPirateFixture(t, 4)
		tile := q.P.destinations(q.G, q.B)[0]
		edge := q.edge(tile)
		q.F.Positions[3], q.F.Positions[4] = edge, edge
		// Only touches a vertex: battle-ready, but not on an edge for theft.
		outsider := -1
		for _, e := range q.G.Edges {
			if catanExplorerSeaEdge(q.G, e.ID) && catanExplorerTouches(q.G, e.ID, tile) && !slices.Contains(e.Tiles, tile) {
				outsider = e.ID
				break
			}
		}
		if outsider < 0 {
			t.Fatal("missing endpoint-only fixture")
		}
		q.F.Positions[6] = outsider
		q.give(1, 0, 3)
		q.give(1, 4, 1)
		q.give(2, 2, 1)
		if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{3, 4}); err != nil {
			t.Fatal(err)
		}
		q.must(t, "seven", 0, false, 0)
		q.must(t, "place", tile, false, 0)
		q.restore(t)
		if !slices.Equal(q.P.victims(q.G, q.F, q.E), []int{1}) {
			t.Fatal("duplicate boat or endpoint-only victim")
		}
		q.reject(t, "steal", 2, false, 0)
		q.reject(t, "steal", 1, true, 0)
		q.reject(t, "steal", 1, false, 4)
		result := q.must(t, "steal", 1, false, draw)
		color := 0
		if draw == 3 {
			color = 4
		}
		if result.Target != 1 || result.Resource != color || result.Gold || q.G.Players[0].Resources[color] != 1 || sum(q.G.Players[1].Resources) != 3 || q.P.Pending != nil {
			t.Fatal("wrong weighted resource theft", result)
		}
		q.restore(t)
	}
}
func TestCatanExplorerPirateEmptyHandGoldOptionalAndNoGold(t *testing.T) {
	for _, decline := range []bool{false, true} {
		q := newExplorerPirateFixture(t, 2)
		tile := q.P.destinations(q.G, q.B)[0]
		q.F.Positions[3] = q.edge(tile)
		if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{3, 4}); err != nil {
			t.Fatal(err)
		}
		q.must(t, "seven", 0, false, 0)
		q.must(t, "place", tile, false, 0)
		result := q.must(t, "steal", 1, decline, 0)
		want := 3
		if decline {
			want = 2
		}
		if q.E.Gold[0] != want || q.E.Gold[1] != 4-want || q.E.GoldBank != 144 || result.Gold == decline {
			t.Fatal("wrong optional coin theft")
		}
		q.restore(t)
	}
	q := newExplorerPirateFixture(t, 2)
	tile := q.P.destinations(q.G, q.B)[0]
	q.F.Positions[3] = q.edge(tile)
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{3, 4}); err != nil {
		t.Fatal(err)
	}
	q.E.GoldBank += q.E.Gold[1]
	q.E.Gold[1] = 0
	q.must(t, "seven", 0, false, 0)
	q.must(t, "place", tile, false, 0)
	if q.P.Pending != nil || q.E.Turn.Phase != "ready" {
		t.Fatal("empty victim must not block")
	}
}
func TestCatanExplorerPirateWaitsForAllDiscardsAndChecksActor(t *testing.T) {
	q := newExplorerPirateFixture(t, 3)
	q.give(1, 0, 8)
	q.give(2, 1, 9)
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{3, 4}); err != nil {
		t.Fatal(err)
	}
	q.reject(t, "seven", 0, false, 0)
	if err := q.E.discard(q.G, q.F, q.C, 2, 1, []int{0, 4, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	q.reject(t, "seven", 0, false, 0)
	if err := q.E.discard(q.G, q.F, q.C, 1, 1, []int{4, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	before := q.snapshot()
	for _, request := range [][2]int{{1, 1}, {-1, 1}, {0, 2}} {
		if _, err := q.P.apply(q.G, q.B, q.F, q.C, q.E, request[0], uint64(request[1]), "seven", 0, false, nil); err == nil {
			t.Fatal("wrong actor/sequence accepted")
		}
	}
	if q.snapshot() != before {
		t.Fatal("bad actor mutated")
	}
	q.must(t, "seven", 0, false, 0)
	q.restore(t)
}
func TestCatanExplorerPirateChaseOnceNoMovementSpentAndResume(t *testing.T) {
	q := newExplorerPirateFixture(t, 2)
	tile := q.P.destinations(q.G, q.B)[0]
	q.P.Owner, q.P.Tile = 1, tile
	edges := []int{}
	for _, edge := range q.G.Edges {
		if catanExplorerSeaEdge(q.G, edge.ID) && catanExplorerTouches(q.G, edge.ID, tile) {
			edges = append(edges, edge.ID)
		}
	}
	for i := 0; i < 3; i++ {
		q.F.Positions[i] = edges[i]
	}
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	q.reject(t, "chase", 0, false, 5)
	if err := q.C.beginMovement(q.G, q.F, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	beforeFleet, _ := json.Marshal(q.F)
	q.must(t, "chase", 0, false, 4)
	q.reject(t, "chase", 0, false, 5)
	if q.P.LastChase.Die != 5 || q.P.LastChase.Success || q.P.Pending != nil {
		t.Fatal("failed chase outcome")
	}
	q.must(t, "chase", 1, false, 5)
	q.restore(t)
	if q.P.Pending == nil || q.P.Pending.Resume != "movement" {
		t.Fatal("successful chase must activate pirate")
	}
	q.reject(t, "chase", 2, false, 5)
	q.reject(t, "place", tile, false, 0)
	nextTile := q.P.destinations(q.G, q.B)[0]
	q.must(t, "place", nextTile, false, 0)
	if q.P.Pending != nil || q.C.Turn.Phase != "movement" || q.P.Owner != 0 || q.P.Tile != nextTile {
		t.Fatal("did not resume movement")
	}
	afterFleet, _ := json.Marshal(q.F)
	if string(beforeFleet) != string(afterFleet) {
		t.Fatal("chase consumed movement or reordered ships")
	}
	q.reject(t, "chase", 2, false, 5)
	q.restore(t)
}

func TestCatanExplorerPirateFailedChaseStillPaysTributeAndNextTurnRetries(t *testing.T) {
	q := newExplorerPirateFixture(t, 2)
	tile := q.P.destinations(q.G, q.B)[0]
	q.P.Owner, q.P.Tile = 1, tile
	edge := q.edge(tile)
	q.F.Positions[0] = edge
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := q.C.beginMovement(q.G, q.F, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	q.must(t, "chase", 0, false, 0)
	next := -1
	for _, candidate := range q.G.Edges {
		if candidate.ID != edge && slices.Contains(candidate.Tiles, tile) && catanExplorerAdjacentEdges(q.G.Edges[edge], candidate) {
			next = candidate.ID
			break
		}
	}
	if next < 0 {
		t.Fatal("missing sea edge")
	}
	gold, bank := q.E.Gold[0], q.E.GoldBank
	for _, path := range [][]int{{next}, {edge}, {next}} {
		if _, err := q.F.sail(q.G, 0, 1, 0, path, q.P.Owner, q.P.Tile, q.E.Gold, &q.E.GoldBank); err != nil {
			t.Fatal(err)
		}
	}
	if q.E.Gold[0] != gold-1 || q.E.GoldBank != bank+1 {
		t.Fatal("tribute not paid exactly once after failed chase")
	}
	q.reject(t, "chase", 0, false, 5)
	if err := q.C.endMovement(q.G, q.F, 0, 1); err != nil {
		t.Fatal(err)
	}
	for seq := uint64(2); seq <= 3; seq++ {
		player := 1
		if seq == 3 {
			player = 0
		}
		if err := q.E.beginProduction(q.G, q.F, q.C, player, seq); err != nil {
			t.Fatal(err)
		}
		if _, err := q.E.resolveProduction(q.G, q.F, q.C, player, seq, [2]int{1, 2}); err != nil {
			t.Fatal(err)
		}
		if err := q.C.beginMovement(q.G, q.F, player, seq, 0); err != nil {
			t.Fatal(err)
		}
		if seq == 2 {
			if err := q.C.endMovement(q.G, q.F, player, seq); err != nil {
				t.Fatal(err)
			}
		}
	}
	q.must(t, "chase", 0, false, 5)
	if !slices.Equal(q.P.Attempted, []int{0}) || q.P.ChaseSequence != 3 || q.P.LastChase.Die != 6 {
		t.Fatal("attempt was not reset for next own turn")
	}
	q.restore(t)
}
func TestCatanExplorerPirateChaseDiceAndMovedShip(t *testing.T) {
	for die := 1; die <= 6; die++ {
		q := newExplorerPirateFixture(t, 2)
		tile := q.P.destinations(q.G, q.B)[0]
		q.P.Owner, q.P.Tile = 1, tile
		q.F.Positions[0] = q.edge(tile)
		if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{1, 2}); err != nil {
			t.Fatal(err)
		}
		if err := q.C.beginMovement(q.G, q.F, 0, 1, 0); err != nil {
			t.Fatal(err)
		}
		q.reject(t, "chase", -1, false, 0)
		q.reject(t, "chase", 3, false, 0)
		q.reject(t, "chase", 0, false, -1)
		q.reject(t, "chase", 0, false, 6)
		q.must(t, "chase", 0, false, die-1)
		if q.P.LastChase.Success != (die == 6) || (q.P.Pending != nil) != (die == 6) {
			t.Fatal("wrong chase success threshold", die)
		}
	}
	q := newExplorerPirateFixture(t, 2)
	tile := q.P.destinations(q.G, q.B)[0]
	q.P.Owner, q.P.Tile = 1, tile
	edge := q.edge(tile)
	q.F.Positions[0] = edge
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := q.C.beginMovement(q.G, q.F, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	for _, e := range q.G.Edges {
		if e.ID != edge && slices.Contains(e.Tiles, tile) && catanExplorerAdjacentEdges(q.G.Edges[edge], e) {
			if _, err := q.F.sail(q.G, 0, 1, 0, []int{e.ID}, q.P.Owner, q.P.Tile, q.E.Gold, &q.E.GoldBank); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	if q.F.Turn.Ships[0].Spent != 1 {
		t.Fatal("fixture did not move")
	}
	q.reject(t, "chase", 0, false, 5)
}
func TestCatanExplorerPirateRejectsDamagedSave(t *testing.T) {
	q := newExplorerPirateFixture(t, 2)
	tile := q.P.destinations(q.G, q.B)[0]
	q.P.Owner, q.P.Tile = 1, tile
	q.F.Positions[0] = q.edge(tile)
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := q.C.beginMovement(q.G, q.F, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	q.must(t, "chase", 0, false, 5)
	for _, damage := range []func(*catanExplorerPirate){
		func(p *catanExplorerPirate) { p.Owner = -1 },
		func(p *catanExplorerPirate) { p.Tile = q.B.FrameSea },
		func(p *catanExplorerPirate) { p.ChaseSequence = 2 },
		func(p *catanExplorerPirate) { p.Attempted = append(p.Attempted, 0) },
		func(p *catanExplorerPirate) { p.LastChase.Die = 5 },
		func(p *catanExplorerPirate) { p.Pending.Sequence = 2 },
		func(p *catanExplorerPirate) { p.Pending.Resume = "action" },
		func(p *catanExplorerPirate) { p.Pending.Stage = "steal" },
	} {
		p := clone(*q.P)
		damage(&p)
		if err := p.validate(q.G, q.B, q.F, q.C, q.E); err == nil {
			t.Fatal("accepted damaged save", p)
		}
	}
}

func TestCatanExplorerPirateEndpointChaseNeedsNoCrewOrMovementPoints(t *testing.T) {
	q := newExplorerPirateFixture(t, 2)
	tile := q.P.destinations(q.G, q.B)[0]
	q.P.Owner, q.P.Tile = 1, tile
	edge := -1
	for _, e := range q.G.Edges {
		if catanExplorerSeaEdge(q.G, e.ID) && catanExplorerTouches(q.G, e.ID, tile) && !slices.Contains(e.Tiles, tile) {
			edge = e.ID
			break
		}
	}
	if edge < 0 {
		t.Fatal("missing endpoint-only boat fixture")
	}
	q.F.Positions[0] = edge
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, 0, 1, [2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	// A construction discovery stopped this boat before movement began. The
	// printed battle-ready definition asks only for no movement and contact,
	// not remaining MP or a crew aboard.
	q.C.Turn.BuildStopped = []int{0}
	if err := q.C.beginMovement(q.G, q.F, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if q.F.Turn.Ships[0].Remaining != 0 || q.F.Turn.Ships[0].Spent != 0 {
		t.Fatal("wrong stopped boat fixture")
	}
	q.must(t, "chase", 0, false, 5)
	if !q.P.LastChase.Success {
		t.Fatal("endpoint-only empty boat cannot chase")
	}
	q.restore(t)
}
