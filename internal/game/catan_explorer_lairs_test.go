package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type explorerLairsFixture struct {
	explorerPirateFixture
	L    *catanExplorerLairs
	Tile int
}

func newExplorerLairsFixture(t *testing.T, n int) explorerLairsFixture {
	t.Helper()
	q := newExplorerPirateFixture(t, n)
	// Deliberately artificial test numbers. This is not a claimed physical
	// component list; the verified 2025 inventory remains a release gate.
	l, err := newCatanExplorerLairs(n, []int{2, 3, 4, 5, 6, 8})
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range q.B.Hidden {
		if h.Resource == CatanGold {
			if err = l.discover(q.G, q.B, h.Tile); err != nil {
				t.Fatal(err)
			}
		}
	}
	tile := -1
	var edges []int
	for _, s := range l.Sites {
		edges = nil
		for _, e := range q.G.Edges {
			if catanExplorerSeaEdge(q.G, e.ID) && catanExplorerTouches(q.G, e.ID, s.Tile) {
				edges = append(edges, e.ID)
			}
		}
		prospective := clone(*q.G)
		prospective.Tiles[s.Tile].Number = s.Number
		buildable := false
		for _, v := range prospective.Tiles[s.Tile].Vertices {
			buildable = buildable || catanExplorerLandVertex(&prospective, v)
		}
		if len(edges) >= n && buildable {
			tile = s.Tile
			break
		}
	}
	if tile < 0 {
		t.Fatal("no coastal gold fixture")
	}
	for p := 0; p < n; p++ {
		q.F.Positions[p*3] = edges[p]
		for unit := 2; unit < 4; unit++ {
			q.C.Units[p*11+unit] = catanExplorerCargoLocation{"ship", p * 3}
		}
	}
	return explorerLairsFixture{q, l, tile}
}
func (q explorerLairsFixture) start(t *testing.T, player int, seq uint64) {
	t.Helper()
	if seq > 1 {
		if err := q.E.beginProduction(q.G, q.F, q.C, player, seq); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := q.E.resolveProduction(q.G, q.F, q.C, player, seq, [2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	if err := q.C.beginMovement(q.G, q.F, player, seq, 0); err != nil {
		t.Fatal(err)
	}
}
func (q explorerLairsFixture) end(t *testing.T) {
	t.Helper()
	if err := q.C.endMovement(q.G, q.F, q.E.Turn.Player, q.E.Turn.Sequence); err != nil {
		t.Fatal(err)
	}
}
func (q explorerLairsFixture) act(kind string, ship int, units []int, rolls ...int) error {
	i := 0
	return q.L.apply(q.G, q.B, q.F, q.C, q.E, q.E.Turn.Player, q.E.Turn.Sequence, kind, q.Tile, ship, units, func(int) int {
		if i >= len(rolls) {
			return -1
		}
		r := rolls[i] - 1
		i++
		return r
	})
}
func (q explorerLairsFixture) must(t *testing.T, kind string, ship int, units []int, rolls ...int) {
	t.Helper()
	if err := q.act(kind, ship, units, rolls...); err != nil {
		t.Fatal(kind, err)
	}
}
func (q explorerLairsFixture) bytes() string { data, _ := json.Marshal(q); return string(data) }
func (q explorerLairsFixture) reject(t *testing.T, kind string, ship int, units []int, rolls ...int) {
	t.Helper()
	before := q.bytes()
	if err := q.act(kind, ship, units, rolls...); err == nil {
		t.Fatal("accepted", kind)
	}
	if before != q.bytes() {
		t.Fatal("failed mission action mutated snapshot")
	}
}
func (q explorerLairsFixture) restore(t *testing.T) {
	t.Helper()
	before := q.bytes()
	for _, v := range []any{q.G, q.B, q.F, q.C, q.E, q.L} {
		data, _ := json.Marshal(v)
		if err := json.Unmarshal(data, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.L.validate(q.G, q.B, q.F, q.C, q.E); err != nil {
		t.Fatal(err)
	}
	if q.bytes() != before {
		t.Fatal("restore differs")
	}
}

func TestCatanExplorerLairsLandEndPhaseRewardsHeroAndPickup(t *testing.T) {
	q := newExplorerLairsFixture(t, 2)
	q.start(t, 0, 1)
	q.reject(t, "land", 0, []int{2, 2})
	q.reject(t, "land", 0, []int{13})
	q.reject(t, "land", 3, []int{2})
	beforeMove := clone(*q.F.Turn)
	q.must(t, "land", 0, []int{2, 3})
	if !reflect.DeepEqual(beforeMove, *q.F.Turn) {
		t.Fatal("landing spent movement")
	}
	q.reject(t, "pickup", 0, []int{2})
	q.reject(t, "begin", 0, nil)
	q.end(t)
	q.reject(t, "begin", 0, nil)
	q.restore(t)
	q.start(t, 1, 2)
	q.reject(t, "land", 3, []int{13, 14})
	q.must(t, "land", 3, []int{13})
	q.reject(t, "land", 3, []int{14})
	q.reject(t, "begin", 3, nil)
	site := q.L.site(q.Tile)
	if q.L.Sites[site].Ready != 2 || q.G.Tiles[q.Tile].Number != 0 || !slices.Equal(q.L.Progress, []int{0, 0}) {
		t.Fatal("capture settled before movement ended")
	}
	q.end(t)
	gold := slices.Clone(q.E.Gold)
	bank := q.E.GoldBank
	q.must(t, "begin", 3, nil)
	q.restore(t)
	if q.E.GoldBank != bank-4 || q.E.Gold[0] != gold[0]+2 || q.E.Gold[1] != gold[1]+2 || q.L.leader() != 1 {
		t.Fatal("participants rewarded out of active-player clockwise order")
	}
	q.reject(t, "begin", 3, nil)
	q.reject(t, "roll", 3, nil, 0, 6)
	// Player1 has 1 crew, player0 has 2. Both totals are 6; more crew wins.
	q.must(t, "roll", 3, nil, 5, 4)
	s := q.L.Sites[site]
	if s.Hero != 0 || s.Resolved != 2 || s.Number != q.G.Tiles[q.Tile].Number || q.B.Liberated[q.Tile] != s.Number || q.L.leader() != 0 || !slices.Equal(q.L.scores(), []int{2, 1}) {
		t.Fatal("wrong hero or score", s, q.L.scores())
	}
	if q.C.Units[2].Kind != "supply" || q.C.Units[3].Kind != "lair" || q.C.Units[13].Kind != "lair" {
		t.Fatal("hero must return exactly one own crew")
	}
	q.reject(t, "pickup", 3, []int{13})
	q.restore(t)
	q.start(t, 0, 3)
	q.must(t, "pickup", 0, []int{3})
	q.reject(t, "land", 0, []int{3})
	q.restore(t)
	if q.C.Units[3] != (catanExplorerCargoLocation{"ship", 0}) {
		t.Fatal("later-turn pickup failed")
	}
}
func TestCatanExplorerLairsThreeWayTieRerollsOnlyTiedPlayers(t *testing.T) {
	q := newExplorerLairsFixture(t, 3)
	for p := 0; p < 3; p++ {
		q.start(t, p, uint64(p+1))
		q.must(t, "land", p*3, []int{p*11 + 2})
		q.end(t)
	}
	q.must(t, "begin", 6, nil)
	if q.L.leader() != 2 {
		t.Fatal("active first")
	}
	q.must(t, "roll", 6, nil, 1, 5, 5) // Order2,0,1: player2 drops out.
	if !slices.Equal(q.L.Battle.Candidates, []int{0, 1}) {
		t.Fatal("wrong reroll candidates", q.L.Battle)
	}
	q.restore(t)
	gold := slices.Clone(q.E.Gold)
	q.must(t, "roll", 6, nil, 3, 3)
	q.restore(t)
	q.must(t, "roll", 6, nil, 2, 6)
	s := q.L.Sites[q.L.site(q.Tile)]
	if s.Hero != 1 || len(s.Rounds) != 3 || !slices.Equal(s.Rounds[1], []int{3, 3, 0}) || !slices.Equal(q.E.Gold, gold) || q.L.Battle != nil {
		t.Fatal("tie resolution repeated rewards or rolled eliminated contender", s)
	}
	q.restore(t)
}
func TestCatanExplorerLairsGoldShortageAtomicAndSoloHero(t *testing.T) {
	q := newExplorerLairsFixture(t, 2)
	q.start(t, 0, 1)
	q.must(t, "land", 0, []int{2, 3})
	q.end(t)
	q.start(t, 1, 2)
	q.end(t)
	// Load the third own crew in a later-turn rule fixture.
	q.C.Units[4] = catanExplorerCargoLocation{"ship", 0}
	q.start(t, 0, 3)
	q.must(t, "land", 0, []int{4})
	q.end(t)
	remaining := q.E.GoldBank
	q.E.Gold[1] += remaining
	q.E.GoldBank = 0
	q.reject(t, "begin", 0, nil)
	q.E.Gold[1] -= 2
	q.E.GoldBank = 2
	q.must(t, "begin", 0, nil)
	s := q.L.Sites[q.L.site(q.Tile)]
	if s.Hero != 0 || len(s.Rounds) != 0 || q.L.Battle != nil || !slices.Equal(q.L.Progress, []int{2, 0}) || q.E.GoldBank != 0 {
		t.Fatal("solo hero or atomic reward failure", s)
	}
	q.restore(t)
}
func TestCatanExplorerLairsHiddenNumbersAndPublicCopies(t *testing.T) {
	q := newExplorerLairsFixture(t, 2)
	before, _ := json.Marshal(q.L.publicView())
	q.L.Sites[0].Number, q.L.Sites[1].Number = q.L.Sites[1].Number, q.L.Sites[0].Number
	after, _ := json.Marshal(q.L.publicView())
	if string(before) != string(after) {
		t.Fatal("hidden token numbers leaked")
	}
	q.restore(t)
	beforeState := q.bytes()
	view := q.L.publicView()
	view.Progress[0] = 7
	view.Sites[0].Tile = -1
	raw, _ := json.Marshal(q.L.publicView())
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v["deck"] != nil || v["inventory"] != nil || v["arrival"] != nil {
		t.Fatal("private inventory exposed")
	}
	for _, s := range v["sites"].([]any) {
		if s.(map[string]any)["number"] != nil {
			t.Fatal("unresolved lair number exposed")
		}
	}
	if q.bytes() != beforeState {
		t.Fatal("view mutated state")
	}
}
func TestCatanExplorerLairTrackPrintedPointsAndStack(t *testing.T) {
	l, err := newCatanExplorerLairs(3, []int{2, 3, 4, 5, 6, 8})
	if err != nil {
		t.Fatal(err)
	}
	if l.leader() != -1 {
		t.Fatal("start space has no award")
	}
	l.advance(1)
	l.advance(0)
	if l.leader() != 1 {
		t.Fatal("first arrival must win tie")
	}
	l.advance(0)
	if l.leader() != 0 {
		t.Fatal("new leader")
	}
	l.advance(1)
	if l.leader() != 0 {
		t.Fatal("later tie stole award")
	}
	want := []int{0, 1, 1, 2, 2, 2, 3, 3}
	for step := 0; step < 8; step++ {
		if catanExplorerLairPoints[step] != want[step] {
			t.Fatal("wrong printed point", step)
		}
	}
	for n := 0; n < 12; n++ {
		l.advance(0)
	}
	arrival := l.Arrival[0]
	for n := 0; n < 12; n++ {
		l.advance(1)
	}
	l.advance(0)
	if l.Progress[0] != 7 || l.Progress[1] != 7 || l.Arrival[0] != arrival || l.leader() != 0 || !slices.Equal(l.scores(), []int{4, 3, 0}) {
		t.Fatal("end-space tie was reordered")
	}
}

func TestCatanExplorerLairHeroAllDiceOutcomes(t *testing.T) {
	for _, counts := range [][]int{{1, 1, 1}, {2, 1, 0}, {1, 2, 0}} {
		candidates := []int{}
		for p, n := range counts {
			if n > 0 {
				candidates = append(candidates, p)
			}
		}
		for a := 1; a <= 6; a++ {
			for b := 1; b <= 6; b++ {
				for c := 1; c <= 6; c++ {
					dice := []int{a, b, c}
					if counts[2] == 0 {
						dice[2] = 0
						if c > 1 {
							continue
						}
					}
					want := []int{}
					for _, p := range candidates {
						beaten := false
						for _, other := range candidates {
							if dice[other]+counts[other] > dice[p]+counts[p] || dice[other]+counts[other] == dice[p]+counts[p] && counts[other] > counts[p] {
								beaten = true
							}
						}
						if !beaten {
							want = append(want, p)
						}
					}
					got, err := catanExplorerLairWinners(counts, candidates, dice)
					if err != nil || !slices.Equal(got, want) {
						t.Fatal(counts, dice, want, got, err)
					}
				}
			}
		}
	}
}
func TestCatanExplorerLairsDamagedHistoryAndGoldNumberRejected(t *testing.T) {
	q := newExplorerLairsFixture(t, 3)
	for p := 0; p < 3; p++ {
		q.start(t, p, uint64(p+1))
		q.must(t, "land", p*3, []int{p*11 + 2})
		q.end(t)
	}
	q.must(t, "begin", 6, nil)
	q.must(t, "roll", 6, nil, 2, 5, 5)
	for _, damage := range []func(*catanExplorerLairs){
		func(l *catanExplorerLairs) { l.Progress[0]++ },
		func(l *catanExplorerLairs) { l.Serial++ },
		func(l *catanExplorerLairs) { l.Sites[l.site(q.Tile)].Contributions[0] = -1 },
		func(l *catanExplorerLairs) { l.Sites[l.site(q.Tile)].Rounds[0][0] = 7 },
		func(l *catanExplorerLairs) { l.Battle.Candidates = []int{0, 2} },
		func(l *catanExplorerLairs) { l.Sites[l.site(q.Tile)].Captor = 0 },
		func(l *catanExplorerLairs) { l.Sites[0].Tile = l.Sites[1].Tile },
	} {
		l := clone(*q.L)
		damage(&l)
		if err := l.validate(q.G, q.B, q.F, q.C, q.E); err == nil {
			t.Fatal("damaged lair history accepted")
		}
	}
	q.must(t, "roll", 6, nil, 6, 1)
	q.restore(t)
	number := q.G.Tiles[q.Tile].Number
	q.G.Tiles[q.Tile].Number = 7
	if err := q.B.validate(q.G); err == nil {
		t.Fatal("bad liberated number accepted")
	}
	q.G.Tiles[q.Tile].Number = number
	delete(q.B.Liberated, q.Tile)
	if err := q.B.validate(q.G); err == nil {
		t.Fatal("gold production without liberation accepted")
	}
}
func TestCatanExplorerLairsProductionOnlyAfterLiberation(t *testing.T) {
	q := newExplorerLairsFixture(t, 2)
	q.start(t, 0, 1)
	q.must(t, "land", 0, []int{2, 3})
	q.end(t)
	q.start(t, 1, 2)
	q.end(t)
	q.C.Units[4] = catanExplorerCargoLocation{"ship", 0}
	q.start(t, 0, 3)
	q.must(t, "land", 0, []int{4})
	q.end(t)
	q.must(t, "begin", 0, nil)
	vertex := -1
	for _, v := range q.G.Tiles[q.Tile].Vertices {
		if catanExplorerLandVertex(q.G, v) {
			vertex = v
			break
		}
	}
	if vertex < 0 {
		t.Fatal("fixture has no legal settlement point")
	}
	// Production fixture: actual legal vertex with one ordinary settlement.
	q.G.Vertices[vertex].Owner, q.G.Vertices[vertex].Level = 0, 1
	number := q.G.Tiles[q.Tile].Number
	a, b := 1, number-1
	if b > 6 {
		a, b = number-6, 6
	}
	if err := q.E.beginProduction(q.G, q.F, q.C, 1, 4); err != nil {
		t.Fatal(err)
	}
	result, err := q.E.resolveProduction(q.G, q.F, q.C, 1, 4, [2]int{a, b})
	if err != nil {
		t.Fatal(err)
	}
	expected := 2
	if sum(result.Resources[0]) == 0 {
		expected++
	}
	if result.Gold[0] != expected {
		t.Fatal("liberated gold must pay two plus no-resource compensation when applicable", result.Gold)
	}
	q.restore(t)
}
