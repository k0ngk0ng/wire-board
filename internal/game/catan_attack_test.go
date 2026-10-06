package game

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"
)

func attackFixture(t *testing.T, n int) (*Catan, *catanAttack) {
	t.Helper()
	g, m, e := newCatanAttackBoard(n)
	if e != nil {
		t.Fatal(e)
	}
	a, e := newCatanAttackPieces(g, m)
	if e != nil {
		t.Fatal(e)
	}
	return g, a
}
func TestCatanAttackOfficialBoardsAndComponents(t *testing.T) {
	for n := 2; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			terrainOrders, deckOrders := map[string]bool{}, map[string]bool{}
			for trial := 0; trial < 24; trial++ {
				g, a := attackFixture(t, n)
				tileCount, vertices, edges, remaining, gold := 19, 54, 72, 34, 100
				if n > 4 {
					tileCount, vertices, edges, remaining, gold = 30, 80, 109, 46, 152
				}
				if len(g.Tiles) != tileCount || len(g.Vertices) != vertices || len(g.Edges) != edges || a.supply() != remaining || a.GoldBank != gold || g.Robber != -1 {
					t.Fatal("official pieces or graph")
				}
				numbers, terrain := []int{}, []int{}
				coastalNums := map[int]int{}
				for _, tile := range g.Tiles {
					numbers = append(numbers, tile.Number)
					terrain = append(terrain, tile.Resource)
					if slices.Contains(a.Map.Coast, tile.ID) {
						coastalNums[tile.Number]++
					}
					if (tile.Number == 2 || tile.Number == 12) != (a.Barbarians[tile.ID] == 1) {
						t.Fatal("initial 2/12 barbarians", tile)
					}
				}
				// Independent row transcription from the official printed setup diagram.
				expected := [][]int{{3, 8, 0}, {9, 4, 5, 10}, {12, 6, 10, 8, 11}, {4, 9, 3, 5}, {0, 6, 2}}
				if n > 4 {
					expected = [][]int{{12, 9, 3}, {4, 5, 10, 8}, {5, 11, 6, 3, 0}, {0, 8, 10, 4, 6, 0}, {0, 3, 8, 11, 9}, {6, 4, 9, 10}, {2, 5, 11}}
				}
				flat := []int{}
				for _, row := range expected {
					flat = append(flat, row...)
				}
				if !slices.Equal(numbers, flat) {
					t.Fatal("printed numeric faces", numbers)
				}
				for _, number := range []int{2, 3, 4, 5, 6, 8, 9, 10, 11, 12} {
					want := 1
					if n > 4 && (number == 5 || number == 9) {
						want = 2
					}
					if coastalNums[number] != want {
						t.Fatal("landing dice targets", number, coastalNums)
					}
				}
				// Clockwise traversal in screen coordinates wraps once, including gaps
				// occupied by desert/castle, and begins immediately after the west castle.
				cx, cy := 0.0, 0.0
				for _, tile := range g.Tiles {
					cx += tile.X
					cy += tile.Y
				}
				cx /= float64(tileCount)
				cy /= float64(tileCount)
				rotation := 0.0
				for i, id := range a.Map.Coast {
					p, q := g.Tiles[id], g.Tiles[a.Map.Coast[(i+1)%len(a.Map.Coast)]]
					delta := math.Atan2(q.Y-cy, q.X-cx) - math.Atan2(p.Y-cy, p.X-cx)
					if delta <= 0 {
						delta += 2 * math.Pi
					}
					rotation += delta
				}
				if math.Abs(rotation-2*math.Pi) > 1e-8 {
					t.Fatal("battle order not clockwise once", rotation)
				}
				first := 4
				if n > 4 {
					first = 5
				}
				if g.Tiles[a.Map.Coast[0]].Number != first {
					t.Fatal("wrong first battle")
				}
				if len(a.Deck) != 26 || len(a.Discard) != 0 || len(a.Knights) != 0 || sum(a.Gold) != 0 || sum(a.Prisoners) != 0 {
					t.Fatal("starting components")
				}
				counts := map[string]int{}
				for _, card := range a.Deck {
					counts[card]++
				}
				if !reflect.DeepEqual(counts, map[string]int{"capture": 4, "knighthood": 14, "swift_knight": 4, "treason": 4}) {
					t.Fatal("official special card deck")
				}
				terrainOrders[fmt.Sprint(terrain)] = true
				deckOrders[fmt.Sprint(a.Deck)] = true
				var restored struct {
					Board  *Catan
					Attack *catanAttack
				}
				data, _ := json.Marshal(struct {
					Board  *Catan
					Attack *catanAttack
				}{g, a})
				if e := json.Unmarshal(data, &restored); e != nil {
					t.Fatal(e)
				}
				if e := restored.Attack.validate(restored.Board); e != nil {
					t.Fatal(e)
				}
				after, _ := json.Marshal(restored)
				if string(data) != string(after) {
					t.Fatal("save/restore changed map or hidden deck")
				}
			}
			if len(terrainOrders) < 2 || len(deckOrders) < 2 {
				t.Fatal("terrain/cards not shuffled")
			}
		})
	}
	for _, n := range []int{0, 1, 7} {
		if _, _, e := newCatanAttackBoard(n); e == nil {
			t.Fatal("bad player count", n)
		}
	}
}

func TestCatanAttackRestoreRejectsCorruptBoardOrInventory(t *testing.T) {
	for _, mutate := range []func(*Catan, *catanAttack){
		func(g *Catan, a *catanAttack) { a.Rules = "other-edition" },
		func(g *Catan, a *catanAttack) { a.Map = nil },
		func(g *Catan, a *catanAttack) { a.Map.Coast[0], a.Map.Coast[1] = a.Map.Coast[1], a.Map.Coast[0] },
		func(g *Catan, a *catanAttack) { a.Map.Castles[0] = 0 },
		func(g *Catan, a *catanAttack) { g.Tiles[0].Number = 11 },
		func(g *Catan, a *catanAttack) { g.Tiles[16].Resource = 0 },
		func(g *Catan, a *catanAttack) { g.Tiles[2].Resource = 0 },
		func(g *Catan, a *catanAttack) { g.Tiles[0].Resource = (g.Tiles[0].Resource + 1) % 5 },
		func(g *Catan, a *catanAttack) { g.Tiles[0].Vertices[0] = -1 },
		func(g *Catan, a *catanAttack) { g.Vertices[0].X = math.NaN() },
		func(g *Catan, a *catanAttack) { g.Edges[0] = g.Edges[1] },
		func(g *Catan, a *catanAttack) { g.Edges[0].Tiles = nil },
		func(g *Catan, a *catanAttack) { g.Ports[0].Edge = g.Ports[1].Edge },
		func(g *Catan, a *catanAttack) { a.Map.Barbarians++ },
		func(g *Catan, a *catanAttack) { a.GoldBank++ },
		func(g *Catan, a *catanAttack) { a.Barbarians[0] = 4 },
		func(g *Catan, a *catanAttack) { a.Barbarians[4] = 1 },
		func(g *Catan, a *catanAttack) { a.Prisoners[0] = 36 },
		func(g *Catan, a *catanAttack) { a.Deck[0] = "knight" },
		func(g *Catan, a *catanAttack) { a.Deck = a.Deck[1:] },
		func(g *Catan, a *catanAttack) { a.Knights = []catanAttackKnight{{0, 0}, {1, 0}} },
		func(g *Catan, a *catanAttack) { a.Knights = []catanAttackKnight{{4, 0}} },
		func(g *Catan, a *catanAttack) {
			for edge := 0; edge < 7; edge++ {
				a.Knights = append(a.Knights, catanAttackKnight{0, edge})
			}
		},
	} {
		g, a := attackFixture(t, 4)
		mutate(g, a)
		if e := a.validate(g); e == nil {
			t.Fatal("accepted corrupted state")
		}
	}
}

func TestCatanAttackConquestDistinguishesBuildingFromConstructionBan(t *testing.T) {
	g, a := attackFixture(t, 4)
	a.Barbarians[0] = 3
	// The north corner touches only coastal hex 0; the southern corner touches
	// additional inland hexes, so that building remains active despite the ban.
	outer, inner := g.Tiles[0].Vertices[4], g.Tiles[0].Vertices[1]
	if !a.conqueredBuilding(g, outer) || a.conqueredBuilding(g, inner) {
		t.Fatal("frame versus unconquered inland adjacency")
	}
	for _, v := range g.Tiles[0].Vertices {
		if !a.buildBlocked(g, -1, v) {
			t.Fatal("new construction allowed", v)
		}
	}
	for side := 0; side < 6; side++ {
		if !a.buildBlocked(g, catanFishingSide(g, 0, side), -1) {
			t.Fatal("road on conquered coast")
		}
	}
	if a.buildBlocked(g, -1, g.Tiles[9].Vertices[0]) {
		t.Fatal("inland construction blocked")
	}
	a.Barbarians[0] = 2
	if a.conquered(0) || a.conqueredBuilding(g, outer) || a.buildBlocked(g, -1, outer) {
		t.Fatal("capture/treason did not liberate")
	}
	// Shared coast corner requires both hexes conquered, not merely one.
	a.Barbarians[0], a.Barbarians[1] = 3, 3
	shared := -1
	for _, v := range g.Tiles[0].Vertices {
		if !slices.Contains(g.Tiles[1].Vertices, v) {
			continue
		}
		count := 0
		for _, tile := range g.Tiles {
			if slices.Contains(tile.Vertices, v) {
				count++
			}
		}
		if count == 2 {
			shared = v
		}
	}
	if shared < 0 || !a.conqueredBuilding(g, shared) {
		t.Fatal("shared coastal corner")
	}
	a.Barbarians[1] = 2
	if a.conqueredBuilding(g, shared) {
		t.Fatal("one liberated neighbor restores building")
	}
	if e := a.validate(g); e != nil {
		t.Fatal(e)
	}
}

func TestCatanAttackKnightMovementAndPrintedLossOrientations(t *testing.T) {
	for _, n := range []int{3, 6} {
		g, a := attackFixture(t, n)
		start := catanFishingSide(g, a.Map.Castles[0], 0)
		a.Knights = []catanAttackKnight{{0, start}}
		near, far := a.knightDestinations(g, 0, 3), a.knightDestinations(g, 0, 5)
		if len(near) == 0 || len(far) <= len(near) {
			t.Fatal("paid movement range")
		}
		max := 0
		block := -1
		for edge, d := range far {
			if a.castleEdge(g, edge) || edge == start || d < 1 || d > 5 {
				t.Fatal("illegal destination")
			}
			if d <= 3 && near[edge] != d || d > 3 && near[edge] != 0 {
				t.Fatal("normal/paid distances")
			}
			if d == 1 {
				block = edge
			}
			if d > max {
				max = d
			}
		}
		if max != 5 || block < 0 {
			t.Fatal("missing range cases")
		}
		// Transit remains allowed through an occupied edge and enemy buildings.
		a.Knights = append(a.Knights, catanAttackKnight{1, block})
		for _, v := range []int{g.Edges[block].A, g.Edges[block].B} {
			g.Vertices[v].Owner = 1
			g.Vertices[v].Level = 1
		}
		g.Edges[block].Owner = 1
		withBlock := a.knightDestinations(g, 0, 5)
		delete(far, block)
		if !reflect.DeepEqual(far, withBlock) {
			t.Fatal("transit treated as blocked or occupied destination allowed")
		}
		for _, id := range a.Map.Coast {
			a.Barbarians[id] = 3
		}
		if !reflect.DeepEqual(withBlock, a.knightDestinations(g, 0, 5)) {
			t.Fatal("conquest incorrectly blocks knights")
		}
		if len(a.knightDestinations(g, -1, 3)) != 0 || len(a.knightDestinations(g, 0, 4)) != 0 {
			t.Fatal("invalid movement request")
		}
		orientationCounts := [3]int{}
		for _, e := range g.Edges {
			o := a.Map.edgeOrientation(g, e.ID)
			if o < 0 || o > 2 {
				t.Fatal("unclassified edge")
			}
			orientationCounts[o]++
		}
		for _, count := range orientationCounts {
			if count == 0 {
				t.Fatal("missing orientation")
			}
		}
		for _, castle := range a.Map.Castles {
			for side := 0; side < 6; side++ {
				if a.Map.edgeOrientation(g, catanFishingSide(g, castle, side)) != side%3 {
					t.Fatal("castles not oriented identically")
				}
			}
		}
	}
	for die, want := range []int{-1, 0, 1, 2, 2, 1, 0, -1} {
		if catanAttackLossOrientation(die) != want {
			t.Fatal("printed dice pairs", die)
		}
	}
}

// Independent visual reference: upright official 2025 castle on printed p15,
// and the losses example on p18. Purple 1/6 is the upper-left/lower-right pair,
// green 2/5 upper-right/lower-left, brown 3/4 the vertical pair. The artwork must
// remain upright; an internally consistent but rotated side index is not proof.
func TestCatanAttackPrintedCastleDiceMatchScreenGeometry(t *testing.T) {
	for _, n := range []int{3, 6} {
		g, a := attackFixture(t, n)
		for _, edge := range g.Edges {
			p, q := g.Vertices[edge.A], g.Vertices[edge.B]
			dx, dy := q.X-p.X, q.Y-p.Y
			var dice []int
			if dx > -0.001 && dx < 0.001 {
				dice = []int{3, 4}
			} else if dx*dy < 0 {
				dice = []int{1, 6}
			} else {
				dice = []int{2, 5}
			}
			for _, die := range dice {
				if a.Map.edgeOrientation(g, edge.ID) != catanAttackLossOrientation(die) {
					t.Fatal("printed castle die does not match actual screen orientation", n, edge.ID, die, dx, dy)
				}
			}
		}
	}
}
