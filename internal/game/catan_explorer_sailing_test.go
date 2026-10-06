package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Minimal hex geometry deliberately tests the sailing kernel, not an official
// scenario, setup, full game, or mission reward controller.
func explorerSailingFixture(t *testing.T, n int, specs ...CatanHexSpec) (*Catan, *catanExplorerSailing, []int, *int) {
	t.Helper()
	if len(specs) == 0 {
		specs = []CatanHexSpec{{Resource: CatanSea}}
	}
	g := &Catan{Players: make([]CatanPlayer, n), Bank: []int{19, 19, 19, 19, 19}}
	if n > 4 {
		g.Bank = []int{24, 24, 24, 24, 24}
	}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
	}
	g.Players[0].Resources[2] = 3
	g.Bank[2] -= 3
	if err := g.makeScenarioMap(specs); err != nil {
		t.Fatal(err)
	}
	f, err := newCatanExplorerSailing(n)
	if err != nil {
		t.Fatal(err)
	}
	f.Positions[0], f.Positions[1], f.Positions[2] = 0, 3, 5
	if err = f.begin(g, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	gold := make([]int, n)
	for p := range gold {
		gold[p] = 2
	}
	bank := 100 // A payment-only fixture, not the official E&P gold inventory.
	return g, f, gold, &bank
}
func explorerSailingSnapshot(t *testing.T, g *Catan, f *catanExplorerSailing, gold []int, bank *int) string {
	t.Helper()
	b, err := json.Marshal([]any{g, f, gold, *bank})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func explorerSailingRestore(t *testing.T, g *Catan, f *catanExplorerSailing) *catanExplorerSailing {
	t.Helper()
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	var out catanExplorerSailing
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f, &out) {
		t.Fatal("movement persistence differs")
	}
	if err = out.validate(g); err != nil {
		t.Fatal(err)
	}
	return &out
}
func TestCatanExplorerSailingPointsWoolAndPerShipTurns(t *testing.T) {
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			g, f, gold, bank := explorerSailingFixture(t, n)
			if len(f.Positions) != n*3 || f.Turn.Ships[0].Remaining != 4 {
				t.Fatal("initial ship allotment")
			}
			if _, err := f.sail(g, 0, 1, 0, []int{1, 2, 3, 4}, -1, -1, gold, bank); err != nil {
				t.Fatal(err)
			}
			if f.Turn.Ships[0].Remaining != 0 || f.Positions[0] != 4 {
				t.Fatal("four edges must use four MPs")
			}
			f = explorerSailingRestore(t, g, f)
			oldWool, oldBank := g.Players[0].Resources[2], g.Bank[2]
			if err := f.wool(g, 0, 1, 0); err != nil {
				t.Fatal(err)
			}
			if f.Turn.Ships[0].Remaining != 2 || g.Players[0].Resources[2] != oldWool-1 || g.Bank[2] != oldBank+1 {
				t.Fatal("wool transfer or bonus")
			}
			if _, err := f.sail(g, 0, 1, 0, []int{5, 0}, -1, -1, gold, bank); err != nil {
				t.Fatal(err)
			}
			if err := f.wool(g, 0, 1, 0); err == nil {
				t.Fatal("cannot pay twice for same ship")
			}
			if err := f.wool(g, 0, 1, 1); err != nil || f.Turn.Current != 0 {
				t.Fatal("prepaying another ship must not pretend it moved", err)
			}
			if _, err := f.sail(g, 0, 1, 1, []int{2}, -1, -1, gold, bank); err != nil {
				t.Fatal(err)
			}
			if !f.Turn.Ships[0].Closed || f.Turn.Ships[1].Remaining != 5 || f.Turn.Ships[2].Remaining != 4 {
				t.Fatal("independent movement limits")
			}
			f = explorerSailingRestore(t, g, f)
			if _, err := f.sail(g, 0, 1, 0, []int{1}, -1, -1, gold, bank); err == nil {
				t.Fatal("cannot alternate ships")
			}
			if err := f.end(g, 0, 1); err != nil {
				t.Fatal(err)
			}
			f = explorerSailingRestore(t, g, f)
			if err := f.begin(g, 0, 1, 0); err == nil {
				t.Fatal("stale begin must not restore movement points")
			}
			if err := f.begin(g, 0, 2, 0); err != nil || f.Turn.Ships[0].Wool || f.Turn.Ships[0].Remaining != 4 {
				t.Fatal("fresh movement phase resets each ship", err)
			}
		})
	}
}
func TestCatanExplorerSailingPassesFullEdgeButCannotStop(t *testing.T) {
	for _, owners := range [][2]int{{1, 2}, {3, 4}, {1, 3}} {
		g, f, gold, bank := explorerSailingFixture(t, 3)
		f.Positions[1], f.Positions[2] = -1, -1
		for _, id := range owners {
			f.Positions[id] = 1
		}
		f.Turn = nil
		if err := f.begin(g, 0, 1, 0); err != nil {
			t.Fatal(err)
		}
		before := explorerSailingSnapshot(t, g, f, gold, bank)
		if _, err := f.sail(g, 0, 1, 0, []int{1}, -1, -1, gold, bank); err == nil || explorerSailingSnapshot(t, g, f, gold, bank) != before {
			t.Fatal("third ship cannot stop, and rejection must be atomic")
		}
		if _, err := f.sail(g, 0, 1, 0, []int{1, 2}, -1, -1, gold, bank); err != nil {
			t.Fatal("must be able to pass own/other/mixed pair", err)
		}
		if f.Positions[0] != 2 || f.Turn.Ships[0].Spent != 2 {
			t.Fatal("passing a full edge still costs one MP per edge")
		}
		if err := f.validate(g); err != nil {
			t.Fatal(err)
		}
	}
}
func TestCatanExplorerSailingPiratePaysSupplyOncePerShip(t *testing.T) {
	for _, pirateOwner := range []int{-1, 0, 1} {
		g, f, gold, bank := explorerSailingFixture(t, 3)
		tile := 0
		if pirateOwner < 0 {
			tile = -1
		}
		for _, step := range []struct{ ship, edge int }{{0, 1}, {0, 2}, {1, 4}} {
			if _, err := f.sail(g, 0, 1, step.ship, []int{step.edge}, pirateOwner, tile, gold, bank); err != nil {
				t.Fatal(err)
			}
			f = explorerSailingRestore(t, g, f)
		}
		want := 0
		if pirateOwner == 1 {
			want = 2
		}
		if gold[0] != 2-want || gold[1] != 2 || *bank != 100+want {
			t.Fatal("tribute belongs to supply, once for each ship", gold, *bank)
		}
	}
}
func TestCatanExplorerSailingInvalidActionsAreAtomic(t *testing.T) {
	type action struct {
		name                    string
		change                  func(*Catan, *catanExplorerSailing, []int, *int)
		player, ship            int
		sequence                uint64
		path                    []int
		pirateOwner, pirateTile int
	}
	cases := []action{
		{name: "wrong actor", player: 1, ship: 0, sequence: 1, path: []int{1}, pirateOwner: -1, pirateTile: -1},
		{name: "stale response", ship: 0, sequence: 0, path: []int{1}, pirateOwner: -1, pirateTile: -1},
		{name: "other ship", ship: 3, sequence: 1, path: []int{1}, pirateOwner: -1, pirateTile: -1},
		{name: "missing ship", ship: 99, sequence: 1, path: []int{1}, pirateOwner: -1, pirateTile: -1},
		{name: "empty path", ship: 0, sequence: 1, pirateOwner: -1, pirateTile: -1},
		{name: "not adjacent", ship: 0, sequence: 1, path: []int{3}, pirateOwner: -1, pirateTile: -1},
		{name: "stationary step", ship: 0, sequence: 1, path: []int{0}, pirateOwner: -1, pirateTile: -1},
		{name: "off board", ship: 0, sequence: 1, path: []int{77}, pirateOwner: -1, pirateTile: -1},
		{name: "over budget", ship: 0, sequence: 1, path: []int{1, 2, 3, 4, 5}, pirateOwner: -1, pirateTile: -1},
		{name: "no tribute", ship: 0, sequence: 1, path: []int{1}, pirateOwner: 1, pirateTile: 0, change: func(_ *Catan, _ *catanExplorerSailing, gold []int, _ *int) { gold[0] = 0 }},
		{name: "invalid pirate", ship: 0, sequence: 1, path: []int{1}, pirateOwner: 1, pirateTile: 99},
		{name: "eliminated", ship: 0, sequence: 1, path: []int{1}, pirateOwner: -1, pirateTile: -1, change: func(g *Catan, _ *catanExplorerSailing, _ []int, _ *int) { g.Players[0].Eliminated = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, f, gold, bank := explorerSailingFixture(t, 3)
			if tc.change != nil {
				tc.change(g, f, gold, bank)
			}
			before := explorerSailingSnapshot(t, g, f, gold, bank)
			if _, err := f.sail(g, tc.player, tc.sequence, tc.ship, tc.path, tc.pirateOwner, tc.pirateTile, gold, bank); err == nil {
				t.Fatal("must reject")
			}
			if explorerSailingSnapshot(t, g, f, gold, bank) != before {
				t.Fatal("failed path modified state or paid part of the toll")
			}
		})
	}
}
func TestCatanExplorerSailingDiscoveryStopsAndPersists(t *testing.T) {
	g, f, gold, bank := explorerSailingFixture(t, 3, CatanHexSpec{Resource: CatanSea}, CatanHexSpec{Q: 1, Resource: CatanFog})
	start, arrival, next := -1, -1, -1
	for i, a := range g.Edges {
		for j, b := range g.Edges {
			if i != j && catanExplorerSeaEdge(g, i) && catanExplorerSeaEdge(g, j) && !catanExplorerTouches(g, i, 1) && catanExplorerTouches(g, j, 1) && catanExplorerAdjacentEdges(a, b) {
				start, arrival = i, j
			}
		}
	}
	if start < 0 {
		t.Fatal("fixture has no approach")
	}
	for i, e := range g.Edges {
		if i != arrival && catanExplorerSeaEdge(g, i) && catanExplorerAdjacentEdges(e, g.Edges[arrival]) {
			next = i
			break
		}
	}
	f.Positions[0], f.Positions[1], f.Positions[2] = start, start, -1
	f.Turn = nil
	if err := f.begin(g, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	before := explorerSailingSnapshot(t, g, f, gold, bank)
	if _, err := f.sail(g, 0, 1, 0, []int{arrival, next}, -1, -1, gold, bank); err == nil || explorerSailingSnapshot(t, g, f, gold, bank) != before {
		t.Fatal("cannot sail past discovery")
	}
	if _, err := f.sail(g, 0, 1, 0, []int{arrival}, -1, -1, gold, bank); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.Turn.Exploring, []int{1}) || !f.Turn.Ships[0].Closed || f.Turn.Ships[0].Remaining != 0 {
		t.Fatal("discovery must stop this ship")
	}
	f = explorerSailingRestore(t, g, f)
	if err := f.end(g, 0, 1); err == nil {
		t.Fatal("cannot skip reveal")
	}
	if err := f.discovered(g, 0, 1); err == nil {
		t.Fatal("cannot acknowledge a still-hidden hex")
	}
	if _, err := f.sail(g, 0, 1, 1, []int{arrival}, -1, -1, gold, bank); err == nil {
		t.Fatal("must resolve reveal before another ship")
	}
	g.Tiles[1].Resource = 0 // Explicit stand-in for the not-yet-built discovery controller.
	if err := f.discovered(g, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := f.wool(g, 0, 1, 0); err == nil {
		t.Fatal("wool cannot reopen explored ship")
	}
	if err := f.swiftVoyage(g, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if f.Turn.Ships[0].Remaining != 0 || f.Turn.Ships[1].Remaining != 5 {
		t.Fatal("farm bonus must not reopen explored ship")
	}
	if _, err := f.sail(g, 0, 1, 1, []int{arrival}, -1, -1, gold, bank); err != nil {
		t.Fatal(err)
	}
	if err := f.end(g, 0, 1); err != nil {
		t.Fatal(err)
	}
	explorerSailingRestore(t, g, f)
}
func TestCatanExplorerSailingSwiftVoyageAndCoast(t *testing.T) {
	g, f, gold, bank := explorerSailingFixture(t, 3, CatanHexSpec{Resource: CatanSea}, CatanHexSpec{Q: 1, Resource: 0}, CatanHexSpec{Q: 1, R: 1, Resource: 1})
	for id, edge := range g.Edges {
		sea := false
		for _, tile := range edge.Tiles {
			sea = sea || tile == 0
		}
		if catanExplorerSeaEdge(g, id) != sea {
			t.Fatal("land/land and land/frame are not ocean", id)
		}
	}
	if _, err := f.sail(g, 0, 1, 0, []int{1}, -1, -1, gold, bank); err != nil {
		t.Fatal(err)
	}
	if err := f.swiftVoyage(g, 0, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := f.wool(g, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.swiftVoyage(g, 0, 1, 2); err != nil {
		t.Fatal(err)
	}
	if f.Turn.Ships[0].Remaining != 7 || f.Turn.Ships[0].Spent != 1 || f.Turn.Ships[1].Remaining != 6 {
		t.Fatal("four base + two farms + wool, independently per ship")
	}
	if err := f.swiftVoyage(g, 0, 1, 2); err == nil {
		t.Fatal("cannot duplicate farm reward")
	}
	if err := f.swiftVoyage(g, 0, 1, 3); err == nil {
		t.Fatal("only two swift farms")
	}
	explorerSailingRestore(t, g, f)
}
func TestCatanExplorerSailingRejectsDamagedSave(t *testing.T) {
	for _, change := range []func(*catanExplorerSailing){
		func(f *catanExplorerSailing) { f.Rules = "other" },
		func(f *catanExplorerSailing) { f.Positions = f.Positions[:8] },
		func(f *catanExplorerSailing) { f.Positions[0] = -2 },
		func(f *catanExplorerSailing) { f.Positions[1], f.Positions[2] = 0, 0 },
		func(f *catanExplorerSailing) { f.Turn.Ships[0].Remaining++ },
		func(f *catanExplorerSailing) { f.Turn.Ships[3].Remaining = 4 },
		func(f *catanExplorerSailing) { f.Turn.Speed = 3 },
		func(f *catanExplorerSailing) { f.Turn.Current = 0 },
		func(f *catanExplorerSailing) { f.Turn.Exploring = []int{0} },
		func(f *catanExplorerSailing) { f.Turn.Open = false },
	} {
		g, f, _, _ := explorerSailingFixture(t, 3)
		change(f)
		if err := f.validate(g); err == nil {
			t.Fatal("accepted damaged movement state")
		}
	}
}

func TestCatanExplorerSailingPirateEntryExitAndVertexOnlyContact(t *testing.T) {
	g, f, gold, bank := explorerSailingFixture(t, 3,
		CatanHexSpec{Resource: CatanSea}, CatanHexSpec{Q: 1, Resource: CatanSea}, CatanHexSpec{Q: 0, R: 1, Resource: CatanSea})
	outside, inside := -1, -1
	for a, edge := range g.Edges {
		if slices.Contains(edge.Tiles, 1) || !catanExplorerTouches(g, a, 1) {
			continue
		}
		for b, target := range g.Edges {
			if slices.Contains(target.Tiles, 1) && catanExplorerAdjacentEdges(edge, target) {
				outside, inside = a, b
			}
		}
	}
	if outside < 0 {
		t.Fatal("fixture has no vertex-only pirate contact")
	}
	for id := range f.Positions {
		f.Positions[id] = -1
	}
	f.Positions[0] = outside
	f.Turn = nil
	if err := f.begin(g, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	q, err := f.sail(g, 0, 1, 0, []int{inside}, 1, 1, gold, bank)
	if err != nil || q.Gold != 1 || gold[0] != 1 || *bank != 101 {
		t.Fatal("entering pirate edge must pay", q, err)
	}
	f = explorerSailingRestore(t, g, f)
	q, err = f.sail(g, 0, 1, 0, []int{outside, inside}, 1, 1, gold, bank)
	if err != nil || q.Gold != 0 || gold[0] != 1 || *bank != 101 {
		t.Fatal("leaving and entering again in same turn must not pay twice", q, err)
	}
	if err = f.end(g, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err = f.begin(g, 0, 2, 0); err != nil {
		t.Fatal(err)
	}
	q, err = f.sail(g, 0, 2, 0, []int{outside}, 1, 1, gold, bank)
	if err != nil || q.Gold != 1 || gold[0] != 0 || *bank != 102 {
		t.Fatal("leaving pirate on a new turn must pay again", q, err)
	}
	for next, edge := range g.Edges {
		if next == outside || slices.Contains(edge.Tiles, 1) || !catanExplorerAdjacentEdges(edge, g.Edges[outside]) {
			continue
		}
		// A fresh turn on an edge touching a pirate vertex, but not its side,
		// is battle-ready and still does not owe a toll for sailing away.
		if err = f.end(g, 0, 2); err != nil {
			t.Fatal(err)
		}
		if err = f.begin(g, 0, 3, 0); err != nil {
			t.Fatal(err)
		}
		q, err = f.sail(g, 0, 3, 0, []int{next}, 1, 1, gold, bank)
		if err != nil || q.Gold != 0 {
			t.Fatal("vertex-only contact must not be charged", q, err)
		}
		return
	}
	t.Fatal("fixture has no uncharged departure")
}

func TestCatanExplorerSailingExhaustiveShortPaths(t *testing.T) {
	// Enumerate every path of up to four steps on a six-edge sea hex, with
	// a pair of ships blocking one edge. Independent adjacency and occupancy
	// expectations catch teleports, free steps and stopping on a third ship.
	g, original, _, _ := explorerSailingFixture(t, 3)
	original.Positions[1], original.Positions[2] = 2, 2
	var visit func([]int)
	checked := 0
	visit = func(path []int) {
		if len(path) > 0 {
			legal, current := true, 0
			for _, edge := range path {
				if (edge-current+6)%6 != 1 && (current-edge+6)%6 != 1 {
					legal = false
				}
				current = edge
			}
			legal = legal && current != 2
			f := explorerSailingRestore(t, g, original)
			gold, bank := []int{2, 2, 2}, 100
			before := explorerSailingSnapshot(t, g, f, gold, &bank)
			_, err := f.sail(g, 0, 1, 0, path, -1, -1, gold, &bank)
			if (err == nil) != legal {
				t.Fatalf("path %v expected legal=%t: %v", path, legal, err)
			}
			if legal {
				if f.Positions[0] != current || f.Turn.Ships[0].Remaining != 4-len(path) || f.Turn.Ships[0].Spent != len(path) {
					t.Fatal("wrong endpoint or cost", path)
				}
				explorerSailingRestore(t, g, f)
			} else if explorerSailingSnapshot(t, g, f, gold, &bank) != before {
				t.Fatal("invalid path changed state", path)
			}
			checked++
		}
		if len(path) < 4 {
			for edge := range 6 {
				visit(append(slices.Clone(path), edge))
			}
		}
	}
	visit(nil)
	if checked != 1554 {
		t.Fatal("incomplete enumeration", checked)
	}
}
