package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func explorerInventionPlay(t *testing.T, s *State, left, right int) {
	t.Helper()
	ckProgressGive(t, s, s.Turn, 3)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 3, Tile: left, Target: right})
}

// Controlled discoveries use the board primitive, not ship actions. The full
// combination remains gated; this tests number provenance and action recovery.
func explorerInventionReveal(t *testing.T, s *State) {
	t.Helper()
	g, b := s.Catan, s.Catan.Explorer.Board
	for _, h := range slices.Clone(b.Hidden) {
		if h.Resource < CatanDesert && !h.Revealed {
			if _, err := b.reveal(g, h.Tile); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestCatanExplorerCityInventionRegionsRestoreAndPrivacy(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, scenario := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scenario), func(t *testing.T) {
				s := explorerCityScenarioFixture(t, n, scenario)
				if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
					t.Fatal(err)
				}
				explorerInventionReveal(t, s)
				g, b := s.Catan, s.Catan.Explorer.Board
				ids, values := []int{}, []int{}
				for _, region := range [][]int{b.Starting, b.Regions[0], b.Regions[1]} {
					for _, id := range region {
						if catanInventionNumber(g.Tiles[id].Number) && !slices.Contains(values, g.Tiles[id].Number) {
							ids, values = append(ids, id), append(values, g.Tiles[id].Number)
							break
						}
					}
				}
				if len(ids) != 3 {
					t.Fatal("fixture lacks three distinct public numbers")
				}
				original := clone(*g)
				for _, pair := range [][2]int{{0, 1}, {1, 2}, {1, 2}, {0, 1}} {
					left, right := ids[pair[0]], ids[pair[1]]
					oldLeft, oldRight := s.Catan.Tiles[left].Number, s.Catan.Tiles[right].Number
					explorerInventionPlay(t, s, left, right)
					g, b = s.Catan, s.Catan.Explorer.Board
					if g.Tiles[left].Number != oldRight || g.Tiles[right].Number != oldLeft || !reflect.DeepEqual(b.Hidden, original.Explorer.Board.Hidden) || !reflect.DeepEqual(b.Numbers, original.Explorer.Board.Numbers) || !reflect.DeepEqual(b.Liberated, original.Explorer.Board.Liberated) || !reflect.DeepEqual(g.Explorer.Economy, original.Explorer.Economy) || !reflect.DeepEqual(g.Explorer.Cargo, original.Explorer.Cargo) || !reflect.DeepEqual(g.Explorer.Pirate, original.Explorer.Pirate) {
						t.Fatal("swap changed provenance, production, cargo or pirate")
					}
					for viewer := -1; viewer < n; viewer++ {
						view := s.View(viewer)
						raw, err := json.Marshal(view)
						if err != nil || strings.Contains(string(raw), "numberSwaps") || strings.Contains(string(raw), "\"hidden\"") || strings.Contains(string(raw), "\"liberated\"") {
							t.Fatal("internal board provenance leaked", err)
						}
					}
				}
				if len(b.NumberSwaps) != 0 || !reflect.DeepEqual(g.Tiles, original.Tiles) {
					t.Fatal("reverse swaps did not restore original map")
				}
				// Same-value discs can legally be exchanged without persistent edits.
				left, right := -1, -1
				for _, a := range g.inventionTiles() {
					for _, z := range g.inventionTiles() {
						if a != z && g.Tiles[a].Number == g.Tiles[z].Number {
							left, right = a, z
						}
					}
				}
				if left < 0 {
					t.Fatal("fixture lacks equal discs")
				}
				explorerInventionPlay(t, s, left, right)
				if len(s.Catan.Explorer.Board.NumberSwaps) != 0 {
					t.Fatal("equal discs left noncanonical edits")
				}
			})
		}
	}
}

func TestCatanExplorerCityInventionGuardsAndCorruption(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	ckProgressGive(t, s, 0, 3)
	ids := s.Catan.inventionTiles()
	a := Action{Type: "catan_progress", Card: 3, Tile: ids[0], Target: ids[1], Prompt: 1}
	explorerCityActionReject(t, s, 0, a) // Before production.
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	explorerCityActionReject(t, s, 1, a)
	bad := a
	bad.Prompt = 0
	explorerCityActionReject(t, s, 0, bad)
	for _, target := range []int{-1, len(s.Catan.Tiles), a.Tile} {
		bad = a
		bad.Target = target
		explorerCityActionReject(t, s, 0, bad)
	}
	for _, tile := range s.Catan.Tiles {
		if !catanInventionNumber(tile.Number) {
			bad = a
			bad.Target = tile.ID
			explorerCityActionReject(t, s, 0, bad)
		}
	}
	// Use distinct starting numbers for a real two-entry permutation.
	for _, id := range ids {
		if s.Catan.Tiles[id].Number != s.Catan.Tiles[a.Tile].Number {
			a.Target = id
			break
		}
	}
	if err := s.catanExplorerCityAction(0, a); err != nil {
		t.Fatal(err)
	}
	explorerCityActionReject(t, s, 0, a) // Card was consumed.
	explorerCityRestore(t, s)
	for _, kind := range []string{"negative", "outside", "fog", "protected", "unbalanced", "missing", "redundant", "tile-mismatch", "without-combination"} {
		t.Run(kind, func(t *testing.T) {
			q := clone(*s)
			g, b := q.Catan, q.Catan.Explorer.Board
			switch kind {
			case "negative":
				b.NumberSwaps[-1] = 3
			case "outside":
				b.NumberSwaps[len(g.Tiles)] = 3
			case "fog":
				b.NumberSwaps[b.Hidden[0].Tile] = 3
			case "protected":
				b.NumberSwaps[a.Tile], g.Tiles[a.Tile].Number = 6, 6
			case "unbalanced":
				b.NumberSwaps[a.Tile], g.Tiles[a.Tile].Number = g.Tiles[a.Target].Number, g.Tiles[a.Target].Number
			case "missing":
				delete(b.NumberSwaps, a.Tile)
			case "redundant":
				b.NumberSwaps[b.FramePasture] = g.Tiles[b.FramePasture].Number
			case "tile-mismatch":
				g.Tiles[a.Tile].Number = 7
			case "without-combination":
				b.CitiesKnights = false
			}
			before := clone(q)
			if err := q.validateExplorerCityProduction(); err == nil || !reflect.DeepEqual(q, before) {
				t.Fatal("corruption accepted or validation mutated state")
			}
		})
	}
}

func TestCatanExplorerCityInventionFollowingProduction(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityProductionFixture(t, n)
			if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
				t.Fatal(err)
			}
			left, right, number := -1, -1, 0
			for _, a := range s.Catan.inventionTiles() {
				for _, z := range s.Catan.inventionTiles() {
					g := clone(*s.Catan)
					num := g.Tiles[z].Number
					before := explorerCityClaims(&g, num)
					g.Tiles[a].Number, g.Tiles[z].Number = g.Tiles[z].Number, g.Tiles[a].Number
					if !reflect.DeepEqual(before, explorerCityClaims(&g, num)) {
						left, right, number = a, z, num
						break
					}
				}
				if left >= 0 {
					break
				}
			}
			if left < 0 {
				t.Fatal("fixture has no production-changing swap")
			}
			explorerInventionPlay(t, s, left, right)
			explorerCityFinishAction(t, s)
			if n == 6 {
				explorerCityFinishAction(t, s)
			}
			claims := explorerCityClaims(s.Catan, number)
			before := clone(*s.Catan)
			red, yellow := max(1, number-6), min(6, number-1)
			if err := s.catanExplorerCityRoll(red, yellow, 3); err != nil {
				t.Fatal(err)
			}
			for p, amounts := range claims {
				for color, amount := range amounts {
					if s.Catan.Players[p].Resources[color] != before.Players[p].Resources[color]+amount {
						t.Fatal("production ignored swapped number", p, color)
					}
				}
			}
			explorerCityRestore(t, s)
		})
	}
}

func TestCatanExplorerCityInventionLaterDiscovery(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	g, b := s.Catan, s.Catan.Explorer.Board
	start := g.inventionTiles()[0]
	discovered := -1
	for _, h := range slices.Clone(b.Hidden) {
		if h.Region == 0 && h.Resource < CatanDesert {
			if _, err := b.reveal(g, h.Tile); err != nil {
				t.Fatal(err)
			}
			if catanInventionNumber(g.Tiles[h.Tile].Number) && g.Tiles[h.Tile].Number != g.Tiles[start].Number {
				discovered = h.Tile
				break
			}
		}
	}
	if discovered < 0 {
		t.Fatal("missing eligible discovery")
	}
	explorerInventionPlay(t, s, start, discovered)
	g, b = s.Catan, s.Catan.Explorer.Board
	swaps, stacks := clone(b.NumberSwaps), clone(b.Numbers)
	for _, h := range slices.Clone(b.Hidden) {
		if h.Region == 1 && h.Resource < CatanDesert && !h.Revealed {
			next, err := b.reveal(g, h.Tile)
			if err != nil {
				t.Fatal(err)
			}
			if next.Number != stacks[1][0] || g.Tiles[h.Tile].Number != stacks[1][0] || !slices.Equal(b.Numbers[0], stacks[0]) || !slices.Equal(b.Numbers[1], stacks[1][1:]) || !reflect.DeepEqual(b.NumberSwaps, swaps) {
				t.Fatal("later discovery used swapped disc provenance")
			}
			explorerCityRestore(t, s)
			return
		}
	}
	t.Fatal("missing later discovery")
}

func TestCatanExplorerCityInventionLiberatedLair(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := explorerCityProductionFixture(t, n)
			if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
				t.Fatal(err)
			}
			g, x := s.Catan, s.Catan.Explorer
			// Explicit synthetic component fixture, not the unverified official
			// six/eight lair face inventory. History represents one solo conquest.
			numbers := []int{3, 4, 5, 9, 10, 11}
			if n == 6 {
				numbers = append(numbers, 3, 4)
			}
			l, err := newCatanExplorerLairs(n, numbers)
			if err != nil {
				t.Fatal(err)
			}
			for _, h := range slices.Clone(x.Board.Hidden) {
				if h.Resource == CatanGold && len(l.Sites) < 2 {
					if _, err := x.Board.reveal(g, h.Tile); err != nil {
						t.Fatal(err)
					}
					if err := l.discover(g, x.Board, h.Tile); err != nil {
						t.Fatal(err)
					}
				}
			}
			x.Lairs = l
			site := &l.Sites[0]
			site.Ready, site.Resolved, site.Captor, site.Hero = 1, 1, 0, 0
			site.Contributions = make([]int, n)
			site.Contributions[0] = 3
			l.Progress[0], l.Arrival[0], l.Serial = 2, 2, 2
			x.Cargo.Units[2], x.Cargo.Units[3] = catanExplorerCargoLocation{"lair", site.Tile}, catanExplorerCargoLocation{"lair", site.Tile}
			x.Economy.Gold[0] += 2
			x.Economy.GoldBank -= 2
			x.Board.Liberated = map[int]int{site.Tile: site.Number}
			g.Tiles[site.Tile].Number = site.Number
			if err := l.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
				t.Fatal(err)
			}
			tile, original := site.Tile, site.Number
			other := -1
			for _, id := range g.inventionTiles() {
				if id != tile && g.Tiles[id].Number != original {
					other = id
					break
				}
			}
			if other < 0 {
				t.Fatal("missing distinct number")
			}
			wanted, history := g.Tiles[other].Number, clone(*l)
			explorerInventionPlay(t, s, tile, other)
			g, x = s.Catan, s.Catan.Explorer
			if err := x.Lairs.validate(g, x.Board, x.Fleet, x.Cargo, x.Economy); err != nil {
				t.Fatal(err)
			}
			view := x.Lairs.publicView(x.Board)
			if g.Tiles[tile].Number != wanted || view.Sites[0].Number != wanted || view.Sites[1].Number != 0 || x.Board.Liberated[tile] != original || !reflect.DeepEqual(*x.Lairs, history) {
				t.Fatal("swapped gold changed history or revealed unresolved token")
			}
			for viewer := -1; viewer < n; viewer++ {
				v := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)["lairs"].(catanExplorerLairsView)
				if v.Sites[0].Number != wanted || v.Sites[1].Number != 0 {
					t.Fatal("lair projection number mismatch")
				}
			}
			// Unliberated tokens cannot be exchanged even if their hidden face
			// happens to be an eligible number.
			ckProgressGive(t, s, s.Turn, 3)
			explorerCityActionReject(t, s, s.Turn, Action{Type: "catan_progress", Card: 3, Tile: other, Target: x.Lairs.Sites[1].Tile, Prompt: 1})
		})
	}
}
