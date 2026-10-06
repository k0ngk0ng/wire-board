package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Deliberately small coastal fixture, with abundant paid resources. This is
// neither a scenario constructor nor a natural complete game.
func explorerCargoFixture(t *testing.T) (*Catan, *catanExplorerSailing, *catanExplorerCargo, []int) {
	t.Helper()
	g := &Catan{Players: make([]CatanPlayer, 2), Bank: []int{9, 9, 9, 9, 9}}
	g.Players[0].Resources = []int{10, 10, 10, 10, 10}
	g.Players[1].Resources = make([]int, 5)
	if err := g.makeScenarioMap([]CatanHexSpec{{Resource: 0, Number: 6}}); err != nil {
		t.Fatal(err)
	}
	v := slices.Clone(g.Tiles[0].Vertices)
	g.Vertices[v[0]].Owner, g.Vertices[v[0]].Level = 0, 2
	g.Players[0].Score = 2
	f, _ := newCatanExplorerSailing(2)
	f.Positions[0] = catanFishingSide(g, 0, 0)
	c, err := newCatanExplorerCargo(g, f, "pirate-lairs")
	if err != nil {
		t.Fatal(err)
	}
	if err = c.beginAction(g, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	return g, f, c, v
}

func explorerCargoSnapshot(t *testing.T, g *Catan, f *catanExplorerSailing, c *catanExplorerCargo) string {
	t.Helper()
	b, err := json.Marshal([]any{g, f, c})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func explorerCargoReject(t *testing.T, g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, action func() error) {
	t.Helper()
	before := explorerCargoSnapshot(t, g, f, c)
	if err := action(); err == nil {
		t.Fatal("invalid cargo action accepted")
	}
	if explorerCargoSnapshot(t, g, f, c) != before {
		t.Fatal("rejected cargo action partially mutated state")
	}
}

func explorerCargoRestore(t *testing.T, g *Catan, f *catanExplorerSailing, c *catanExplorerCargo) {
	t.Helper()
	for _, value := range []any{g, f, c} {
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, value); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(value)
		if err != nil || string(out) != string(b) {
			t.Fatal("cargo JSON restore changed state", err)
		}
	}
	if err := c.validate(g, f); err != nil {
		t.Fatal(err)
	}
}

func TestCatanExplorerCargoPrintedLandHoSetup(t *testing.T) {
	wantHands := [][]int{{0, 0, 1, 1, 1}, {1, 1, 0, 0, 1}, {0, 0, 1, 1, 1}, {0, 1, 1, 0, 0}}
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			g, board, f, c, err := newCatanExplorerLandHo(n)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.Units) != n*11 || len(f.Positions) != n*3 || c.Turn != nil || f.Turn != nil {
				t.Fatal("initial inventories or phase")
			}
			bank := []int{19, 19, 19, 19, 19}
			for p, hand := range wantHands[:n] {
				if !reflect.DeepEqual(g.Players[p].Resources, hand) || g.Players[p].Score != 3 {
					t.Fatal("printed initial resources/score", p)
				}
				for r, amount := range hand {
					bank[r] -= amount
				}
				if f.Positions[p*3] != board.Opening[p].Ship || f.Positions[p*3+1] != -1 || f.Positions[p*3+2] != -1 {
					t.Fatal("one starting ship per active player")
				}
				for u := 0; u < 11; u++ {
					want := catanExplorerCargoLocation{"supply", -1}
					if u == 0 {
						want = catanExplorerCargoLocation{"ship", p * 3}
					}
					if c.Units[p*11+u] != want {
						t.Fatal("printed settler/supply inventory")
					}
				}
			}
			if !reflect.DeepEqual(g.Bank, bank) {
				t.Fatal("opening resources must come from bank")
			}
			for color, o := range board.Opening {
				owner := color
				if color >= n {
					if n == 2 {
						owner = -color
					} else {
						owner = -1
					}
				}
				if g.Vertices[o.Settlement].Owner != owner || g.Vertices[o.Harbor].Owner != owner || g.Edges[o.Road].Owner != owner {
					t.Fatal("active/neutral/absent printed pieces", color)
				}
			}
			explorerCargoRestore(t, g, f, c)
			if err := c.beginAction(g, f, 0, 1); err != nil {
				t.Fatal(err)
			}
			explorerCargoReject(t, g, f, c, func() error {
				return c.buildUnit(g, f, 0, 1, 2, catanExplorerCargoLocation{"harbor", board.Opening[0].Harbor}, nil)
			})
		})
	}
	for _, n := range []int{0, 1, 5, 6} {
		if _, _, _, _, err := newCatanExplorerLandHo(n); err == nil {
			t.Fatal("uninstalled player setup accepted", n)
		}
	}
}

func TestCatanExplorerCargoHarborAndShipCosts(t *testing.T) {
	g, f, c, v := explorerCargoFixture(t)
	g.Vertices[v[3]].Owner, g.Vertices[v[3]].Level = 0, 1
	g.Players[0].Score++
	if err := c.buildHarbor(g, f, 0, 1, v[3]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Players[0].Resources, []int{10, 10, 10, 8, 8}) || g.Players[0].Score != 4 || g.Vertices[v[3]].Level != 2 {
		t.Fatal("harbor costs 2 wheat/2 ore and adds one point")
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildHarbor(g, f, 0, 1, v[3]) })
	edge := catanFishingSide(g, 0, 5)
	g.Edges[edge].Owner = 0 // A coastal road does not occupy a ship slot.
	if err := c.buildShip(g, f, 0, 1, 1, edge); err != nil {
		t.Fatal(err)
	}
	if f.Positions[1] != edge || g.Edges[edge].Owner != 0 || !reflect.DeepEqual(g.Players[0].Resources, []int{9, 10, 9, 8, 8}) {
		t.Fatal("ship cost/coastal road")
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildShip(g, f, 0, 1, 2, edge) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildShip(g, f, 0, 1, 0, catanFishingSide(g, 0, 3)) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildShip(g, f, 0, 1, 3, catanFishingSide(g, 0, 3)) })
	if err := c.buildShip(g, f, 0, 1, 2, catanFishingSide(g, 0, 3)); err != nil {
		t.Fatal(err)
	}
	c.Units[0] = catanExplorerCargoLocation{"ship", 0}
	if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.endMovement(g, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.beginAction(g, f, 0, 2); err != nil {
		t.Fatal(err)
	}
	if err := c.buildShip(g, f, 0, 2, 0, catanFishingSide(g, 0, 2)); err != nil {
		t.Fatal(err)
	}
	if c.Units[0] != (catanExplorerCargoLocation{"supply", -1}) || f.Turn.Ships[0] != (catanExplorerShipMove{Closed: true}) {
		t.Fatal("recycling must return cargo and clear old MPs")
	}
	explorerCargoRestore(t, g, f, c)
	if err := c.beginMovement(g, f, 0, 2, 0); err != nil {
		t.Fatal(err)
	}
	if f.Turn.Ships[0].Remaining != 4 {
		t.Fatal("new ship gets only fresh turn budget")
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildShip(g, f, 0, 2, 0, catanFishingSide(g, 0, 0)) })
}

func TestCatanExplorerCargoUnitInventoryAndPaidReplacement(t *testing.T) {
	g, f, c, v := explorerCargoFixture(t)
	h, s := catanExplorerCargoLocation{"harbor", v[0]}, catanExplorerCargoLocation{"ship", 0}
	if err := c.buildUnit(g, f, 0, 1, 0, s, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.buildUnit(g, f, 0, 1, 2, h, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Players[0].Resources, []int{9, 9, 8, 9, 9}) {
		t.Fatal("settler and crew costs")
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildUnit(g, f, 0, 1, 1, h, []int{2}) }) // A half-full harbor cannot discard.
	if err := c.buildUnit(g, f, 0, 1, 3, h, nil); err != nil {
		t.Fatal(err)
	}
	explorerCargoReject(t, g, f, c, func() error { return c.buildUnit(g, f, 0, 1, 4, h, nil) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildUnit(g, f, 0, 1, 4, h, []int{2, 3}) }) // One crew needs only one slot.
	explorerCargoReject(t, g, f, c, func() error { return c.buildUnit(g, f, 0, 1, 4, h, []int{2, 2}) })
	explorerCargoReject(t, g, f, c, func() error { return c.buildUnit(g, f, 0, 1, 11, h, []int{2, 3}) })
	before := slices.Clone(g.Players[0].Resources)
	if err := c.buildUnit(g, f, 0, 1, 2, h, []int{2}); err != nil {
		t.Fatal("may pay to rebuild same returned piece", err)
	}
	if g.Players[0].Resources[2] != before[2]-1 || g.Players[0].Resources[4] != before[4]-1 {
		t.Fatal("replacement must not be free")
	}
	explorerCargoRestore(t, g, f, c)
	// A piece already on a ship cannot be copied into the harbor.
	c.Units[2], c.Units[3] = catanExplorerCargoLocation{"supply", -1}, catanExplorerCargoLocation{"supply", -1}
	c.Units[1] = h
	explorerCargoReject(t, g, f, c, func() error { return c.buildUnit(g, f, 0, 1, 0, h, []int{1}) })
	// Tampered money totals reject before returning a piece or charging.
	g.Bank[0]++
	explorerCargoReject(t, g, f, c, func() error { return c.buildUnit(g, f, 0, 1, 4, h, []int{1}) })
}

func TestCatanExplorerCargoTransfersAreAtomicAndDoNotRefreshMovement(t *testing.T) {
	g, f, c, v := explorerCargoFixture(t)
	h, s := catanExplorerCargoLocation{"harbor", v[0]}, catanExplorerCargoLocation{"ship", 0}
	c.Units[0], c.Units[2], c.Units[3] = s, h, h
	explorerCargoReject(t, g, f, c, func() error { return c.transfer(g, f, 0, 1, 0, v[0], nil, []int{0}) })
	if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	for _, action := range []func() error{
		func() error { return c.transfer(g, f, 0, 1, 0, v[0], []int{2}, nil) },
		func() error { return c.transfer(g, f, 0, 1, 0, v[0], []int{2, 2}, []int{0}) },
		func() error { return c.transfer(g, f, 0, 1, 0, v[0], []int{0}, []int{2}) },
		func() error { return c.transfer(g, f, 0, 2, 0, v[0], []int{2, 3}, []int{0}) },
		func() error { return c.transfer(g, f, 1, 1, 0, v[0], []int{2, 3}, []int{0}) },
		func() error { return c.transfer(g, f, 0, 1, 0, v[3], []int{2, 3}, []int{0}) },
	} {
		explorerCargoReject(t, g, f, c, action)
	}
	move := f.Turn.Ships[0]
	if err := c.transfer(g, f, 0, 1, 0, v[0], []int{2, 3}, []int{0}); err != nil {
		t.Fatal(err)
	}
	if c.Units[0] != h || c.Units[2] != s || c.Units[3] != s || move != f.Turn.Ships[0] {
		t.Fatal("simultaneous full swap must preserve MPs")
	}
	explorerCargoRestore(t, g, f, c)
	// Complete a four-edge trip returning to the same harbor, without cheating
	// the MP budget. Loading/unloading is legal even with zero MPs left.
	path := []int{catanFishingSide(g, 0, 1), catanFishingSide(g, 0, 0), catanFishingSide(g, 0, 5), catanFishingSide(g, 0, 0)}
	gold, bank := []int{0, 0}, 0
	if _, err := f.sail(g, 0, 1, 0, path, -1, -1, gold, &bank); err != nil {
		t.Fatal(err)
	}
	move = f.Turn.Ships[0]
	if move.Remaining != 0 || move.Spent != 4 {
		t.Fatal("fixture must consume all MPs")
	}
	if err := c.transfer(g, f, 0, 1, 0, v[0], []int{0}, []int{2, 3}); err != nil {
		t.Fatal(err)
	}
	if f.Turn.Ships[0] != move {
		t.Fatal("transfer refreshed movement")
	}
	f.Turn.Ships[0].Closed = true
	if err := c.transfer(g, f, 0, 1, 0, v[0], []int{2, 3}, []int{0}); err != nil {
		t.Fatal("stopped ship still loads", err)
	}
	explorerCargoRestore(t, g, f, c)
	if err := c.endMovement(g, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	explorerCargoReject(t, g, f, c, func() error { return c.transfer(g, f, 0, 1, 0, v[0], []int{0}, []int{2, 3}) })
	explorerCargoReject(t, g, f, c, func() error { return c.beginAction(g, f, 0, 1) })
}

func TestCatanExplorerCargoSettlingReturnsPhysicalPieces(t *testing.T) {
	g, f, c, v := explorerCargoFixture(t)
	c.Units[0] = catanExplorerCargoLocation{"ship", 0}
	if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	explorerCargoReject(t, g, f, c, func() error { return c.settle(g, f, 0, 1, 0, v[1]) }) // Adjacent harbor.
	explorerCargoReject(t, g, f, c, func() error { return c.settle(g, f, 0, 1, 0, v[3]) }) // Not ship endpoint.
	gold, bank := []int{0, 0}, 0
	if _, err := f.sail(g, 0, 1, 0, []int{catanFishingSide(g, 0, 1)}, -1, -1, gold, &bank); err != nil {
		t.Fatal(err)
	}
	hand, supply := slices.Clone(g.Players[0].Resources), slices.Clone(g.Bank)
	if err := c.settle(g, f, 0, 1, 0, v[2]); err != nil {
		t.Fatal(err)
	}
	if g.Vertices[v[2]].Owner != 0 || g.Vertices[v[2]].Level != 1 || g.Players[0].Score != 3 || f.Positions[0] != -1 || c.Units[0] != (catanExplorerCargoLocation{"supply", -1}) || f.Turn.Current != -1 {
		t.Fatal("settling must award village and return ship/settler")
	}
	if !reflect.DeepEqual(hand, g.Players[0].Resources) || !reflect.DeepEqual(supply, g.Bank) {
		t.Fatal("settler founding must not charge again")
	}
	explorerCargoRestore(t, g, f, c)
	explorerCargoReject(t, g, f, c, func() error { return c.settle(g, f, 0, 1, 0, v[2]) })
	if err := c.endMovement(g, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.beginAction(g, f, 0, 2); err != nil {
		t.Fatal(err)
	}
	if err := c.buildShip(g, f, 0, 2, 0, catanFishingSide(g, 0, 0)); err != nil {
		t.Fatal("returned ship reusable", err)
	}
	if err := c.buildUnit(g, f, 0, 2, 0, catanExplorerCargoLocation{"ship", 0}, nil); err != nil {
		t.Fatal("returned settler reusable", err)
	}
	explorerCargoRestore(t, g, f, c)
}

func TestCatanExplorerCargoCorruptInventoryAndPhase(t *testing.T) {
	for name, damage := range map[string]func(*Catan, *catanExplorerSailing, *catanExplorerCargo, []int){
		"missing piece": func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) {
			c.Units = c.Units[:len(c.Units)-1]
		},
		"other ship": func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) {
			f.Positions[3] = f.Positions[0]
			c.Units[0] = catanExplorerCargoLocation{"ship", 3}
		},
		"supply index": func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) { c.Units[0].Index = 0 },
		"overfull": func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) {
			c.Units[0] = catanExplorerCargoLocation{"ship", 0}
			c.Units[2] = c.Units[0]
		},
		"undeployed ship": func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) {
			c.Units[0] = catanExplorerCargoLocation{"ship", 1}
		},
		"ordinary village cargo": func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) {
			g.Vertices[v[0]].Level = 1
			c.Units[0] = catanExplorerCargoLocation{"harbor", v[0]}
		},
		"helper combination": func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) { g.Options.Helpers = true },
		"phase mismatch":     func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) { c.Turn.Phase = "movement" },
		"unknown location":   func(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, v []int) { c.Units[0].Kind = "island" },
	} {
		t.Run(name, func(t *testing.T) {
			g, f, c, v := explorerCargoFixture(t)
			damage(g, f, c, v)
			if err := c.validate(g, f); err == nil {
				t.Fatal("corrupt cargo accepted")
			}
			explorerCargoReject(t, g, f, c, func() error { return c.beginMovement(g, f, 0, 1, 0) })
		})
	}
}

func TestCatanExplorerCargoUnexploredAndLairCoast(t *testing.T) {
	for _, resource := range []int{CatanFog, CatanGold} {
		t.Run(fmt.Sprint(resource), func(t *testing.T) {
			g, f, c, _ := explorerCargoFixture(t)
			// Three separate hexes isolate the cargo rule from movement/map setup:
			// the home harbor, a blocked shore, and a clear landing shore.
			if err := g.makeScenarioMap([]CatanHexSpec{{Resource: 0, Number: 6}, {Q: 3, Resource: resource}, {Q: 6, Resource: 0, Number: 8}}); err != nil {
				t.Fatal(err)
			}
			home := g.Tiles[0].Vertices[0]
			g.Vertices[home].Owner, g.Vertices[home].Level = 0, 2
			f.Positions[0] = catanFishingSide(g, 0, 0)
			c.Units[0] = catanExplorerCargoLocation{"ship", 0}
			if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
				t.Fatal(err)
			}
			// Can't found on an unknown shore (and also cannot teleport to it).
			explorerCargoReject(t, g, f, c, func() error { return c.settle(g, f, 0, 1, 0, g.Tiles[1].Vertices[0]) })
			if resource == CatanGold {
				f.Positions[0] = catanFishingSide(g, 1, 0)
				explorerCargoReject(t, g, f, c, func() error { return c.settle(g, f, 0, 1, 0, g.Tiles[1].Vertices[0]) })
				// Stand-in for the not-yet-installed lair controller. Its reward,
				// mission and map checks are intentionally not claimed here.
				g.Tiles[1].Number = 5
				if err := c.settle(g, f, 0, 1, 0, g.Tiles[1].Vertices[0]); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestCatanExplorerCargoComponentLimits(t *testing.T) {
	for _, kind := range []string{"settlements", "harbors"} {
		t.Run(kind, func(t *testing.T) {
			g, f, c, _ := explorerCargoFixture(t)
			specs := []CatanHexSpec{}
			for i := 0; i < 6; i++ {
				specs = append(specs, CatanHexSpec{Q: 3 * i, Resource: 0, Number: 6})
			}
			if err := g.makeScenarioMap(specs); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 5; i++ {
				level := 1
				if kind == "harbors" && i < 4 {
					level = 2
				}
				v := g.Tiles[i].Vertices[0]
				g.Vertices[v].Owner, g.Vertices[v].Level = 0, level
			}
			f.Positions[0] = catanFishingSide(g, 5, 0)
			if kind == "harbors" {
				explorerCargoReject(t, g, f, c, func() error { return c.buildHarbor(g, f, 0, 1, g.Tiles[4].Vertices[0]) })
			} else {
				c.Units[0] = catanExplorerCargoLocation{"ship", 0}
				if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
					t.Fatal(err)
				}
				explorerCargoReject(t, g, f, c, func() error { return c.settle(g, f, 0, 1, 0, g.Tiles[5].Vertices[0]) })
			}
		})
	}
}

func TestCatanExplorerCargoCannotSkipMandatoryDiscovery(t *testing.T) {
	g, f, c, _ := explorerCargoFixture(t)
	// One known land, one known sea and one fog hex. Enumerate a coastal edge
	// which actually touches the fog, then make a legal single-edge discovery.
	if err := g.makeScenarioMap([]CatanHexSpec{{Resource: 0, Number: 6}, {Q: 1, Resource: CatanSea}, {Q: 1, R: 1, Resource: CatanFog}}); err != nil {
		t.Fatal(err)
	}
	from, to := -1, -1
	for _, a := range g.Edges {
		if !catanExplorerSeaEdge(g, a.ID) || catanExplorerTouches(g, a.ID, 2) {
			continue
		}
		for _, b := range g.Edges {
			if catanExplorerSeaEdge(g, b.ID) && catanExplorerAdjacentEdges(a, b) && catanExplorerTouches(g, b.ID, 2) {
				from, to = a.ID, b.ID
				break
			}
		}
		if from >= 0 {
			break
		}
	}
	if from < 0 {
		t.Fatal("missing discovery route in fixture")
	}
	f.Positions[0] = from
	c.Units[0] = catanExplorerCargoLocation{"ship", 0}
	if err := c.beginMovement(g, f, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	gold, bank := []int{0, 0}, 0
	if _, err := f.sail(g, 0, 1, 0, []int{to}, -1, -1, gold, &bank); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.Turn.Exploring, []int{2}) {
		t.Fatal("discovery must pause movement")
	}
	explorerCargoRestore(t, g, f, c)
	explorerCargoReject(t, g, f, c, func() error { return c.settle(g, f, 0, 1, 0, g.Edges[to].A) })
	explorerCargoReject(t, g, f, c, func() error { return c.endMovement(g, f, 0, 1) })
	explorerCargoReject(t, g, f, c, func() error { return c.beginAction(g, f, 0, 2) })
	explorerCargoReject(t, g, f, c, func() error { return c.transfer(g, f, 0, 1, 0, g.Edges[to].A, nil, []int{0}) })
}
