package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Component-controller fixture, not a complete setup or paired-turn game.
// Ships/crew are explicitly placed to isolate expanded IDs and transactions.
type explorerStockFixture struct {
	G     *Catan
	B     *catanExplorerBoard
	F     *catanExplorerSailing
	C     *catanExplorerCargo
	E     *catanExplorerEconomy
	L     *catanExplorerLairs
	Fish  *catanExplorerFish
	Spice *catanExplorerSpice
}

func explorerStockWorld(t *testing.T, n int, scene string) *explorerStockFixture {
	t.Helper()
	g, b, err := newCatanExplorerBoard(n, scene, "variable")
	if err != nil {
		t.Fatal(err)
	}
	// Place farms on outer edges using a legal within-region shuffle, so every
	// farm can be reached by the isolated cargo tests without fabricating land.
	if catanExplorerSpiceScenario(scene) {
		for region := 0; region < 2; region++ {
			var targets []int
			for i, h := range b.Hidden {
				if h.Region != region {
					continue
				}
				for _, edge := range g.Edges {
					if len(edge.Tiles) == 1 && edge.Tiles[0] == h.Tile {
						targets = append(targets, i)
						break
					}
				}
			}
			if len(targets) < 3 {
				t.Fatal("fixture coast")
			}
			for j, ability := range []string{"swift", "pirate", "gold"} {
				from := slices.IndexFunc(b.Hidden, func(h catanExplorerHidden) bool { return h.Region == region && h.Farm == ability })
				to := targets[j]
				a, z := b.Hidden[from].Tile, b.Hidden[to].Tile
				b.Hidden[from], b.Hidden[to] = b.Hidden[to], b.Hidden[from]
				b.Hidden[from].Tile, b.Hidden[to].Tile = a, z
			}
		}
	}
	f, err := newCatanExplorerSailing(n)
	if err != nil {
		t.Fatal(err)
	}
	bank := 19
	if n > 4 {
		bank = 24
	}
	g.Bank = []int{bank, bank, bank, bank, bank}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
	}
	c, err := newCatanExplorerCargo(g, f, scene)
	if err != nil {
		t.Fatal(err)
	}
	e, err := newCatanExplorerEconomy(g, f, c)
	if err != nil {
		t.Fatal(err)
	}
	w := &explorerStockFixture{G: g, B: b, F: f, C: c, E: e}
	if catanExplorerMissionScenario(scene) {
		// Artificial input, NOT the unverified official six/eight token faces.
		numbers := []int{2, 3, 4, 5, 6, 8}
		if n > 4 {
			numbers = append(numbers, 9, 10)
		}
		w.L, err = newCatanExplorerLairs(n, numbers)
		if err != nil {
			t.Fatal(err)
		}
	}
	if catanExplorerFishScenario(scene) {
		w.Fish = &catanExplorerFish{Deliveries: []catanExplorerFishDelivery{}}
	}
	if catanExplorerSpiceScenario(scene) {
		w.Spice = &catanExplorerSpice{Deliveries: []catanExplorerSpiceDelivery{}}
	}
	for _, h := range slices.Clone(b.Hidden) {
		if _, err = b.reveal(g, h.Tile); err != nil {
			t.Fatal(err)
		}
		if h.Farm != "" {
			if err = c.discoverSpice(g, b, f, h.Tile); err != nil {
				t.Fatal(err)
			}
		}
		if h.Resource == CatanGold {
			if err = w.L.discover(g, b, h.Tile); err != nil {
				t.Fatal(err)
			}
		}
	}
	w.check(t)
	return w
}
func (w *explorerStockFixture) check(t *testing.T) {
	t.Helper()
	if err := w.B.validate(w.G); err != nil {
		t.Fatal(err)
	}
	if err := w.E.validate(w.G, w.F, w.C); err != nil {
		t.Fatal(err)
	}
	if w.L != nil {
		if err := w.L.validate(w.G, w.B, w.F, w.C, w.E); err != nil {
			t.Fatal(err)
		}
	}
	if w.Fish != nil {
		if err := w.Fish.validate(w.G, w.B, w.F, w.C); err != nil {
			t.Fatal(err)
		}
	}
	if w.Spice != nil {
		if err := w.Spice.validate(w.G, w.B, w.F, w.C, w.E); err != nil {
			t.Fatal(err)
		}
	}
}
func (w *explorerStockFixture) restore(t *testing.T) *explorerStockFixture {
	t.Helper()
	raw, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	var next explorerStockFixture
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	next.check(t)
	if !reflect.DeepEqual(w, &next) {
		t.Fatal("expanded inventory changed on restore")
	}
	return &next
}
func (w *explorerStockFixture) start(t *testing.T, p int, seq uint64) {
	t.Helper()
	if err := w.E.beginProduction(w.G, w.F, w.C, p, seq); err != nil {
		t.Fatal(err)
	}
	if _, err := w.E.resolveProduction(w.G, w.F, w.C, p, seq, [2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
}
func (w *explorerStockFixture) movement(t *testing.T) {
	t.Helper()
	p, seq := w.C.Turn.Player, w.C.Turn.Sequence
	if err := w.C.beginMovement(w.G, w.F, p, seq, w.C.farmCount(w.B, p, "swift")); err != nil {
		t.Fatal(err)
	}
}
func (w *explorerStockFixture) position(t *testing.T, ship int, accept func(CatanEdge) bool) {
	t.Helper()
	for _, edge := range w.G.Edges {
		if !catanExplorerSeaEdge(w.G, edge.ID) || !accept(edge) {
			continue
		}
		count := 0
		for id, at := range w.F.Positions {
			if id != ship && at == edge.ID {
				count++
			}
		}
		if count < 2 {
			w.F.Positions[ship] = edge.ID
			return
		}
	}
	t.Fatal("no fixture edge")
}

func TestCatanExplorerSixInventoryConservationAndRestore(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, scene := range []string{"pirate-lairs", "fish-for-catan", "spices-for-catan", "explorers-and-pirates"} {
			t.Run(fmt.Sprintf("%d/%s", n, scene), func(t *testing.T) {
				w := explorerStockWorld(t, n, scene)
				fish, spice, lairs, gold := 6, 24, 6, 148
				if n > 4 {
					fish, spice, lairs, gold = 8, 36, 8, 172
				}
				if len(w.C.Units) != 11*n || len(w.F.Positions) != 3*n || w.E.GoldBank != gold-2*n || sum(w.E.Gold) != 2*n {
					t.Fatal("physical stock")
				}
				if w.Fish != nil && len(w.C.Fish) != fish || w.Spice != nil && len(w.C.Spice) != spice || w.L != nil && (len(w.L.Inventory) != lairs || len(w.L.Sites)+len(w.L.Deck) != lairs) {
					t.Fatal("mission stock")
				}
				if w.Spice != nil {
					allocated := 0
					for _, s := range w.C.Spice {
						if s.Origin >= 0 {
							allocated++
						}
					}
					if allocated != 6*n {
						t.Fatal("farm allocation", allocated)
					}
				}
				w = w.restore(t)
				for _, mutate := range []func(*explorerStockFixture){
					func(q *explorerStockFixture) { q.E.GoldBank++ },
					func(q *explorerStockFixture) { q.G.Bank[4]-- },
					func(q *explorerStockFixture) { q.C.Units = q.C.Units[:len(q.C.Units)-1] },
					func(q *explorerStockFixture) { q.F.Positions = append(q.F.Positions, -1) },
				} {
					q := clone(*w)
					mutate(&q)
					if q.E.validate(q.G, q.F, q.C) == nil {
						t.Fatal("accepted broken conservation")
					}
				}
				if w.Fish != nil {
					q := clone(*w)
					q.C.Fish = q.C.Fish[:len(q.C.Fish)-1]
					if q.C.validate(q.G, q.F) == nil {
						t.Fatal("missing fish accepted")
					}
				}
				if w.Spice != nil {
					q := clone(*w)
					q.C.Spice = append(q.C.Spice, q.C.Spice[0])
					if q.C.validate(q.G, q.F) == nil {
						t.Fatal("extra spice accepted")
					}
				}
				if w.L != nil {
					q := clone(*w)
					q.L.Sites = q.L.Sites[:len(q.L.Sites)-1]
					if q.L.validate(q.G, q.B, q.F, q.C, q.E) == nil {
						t.Fatal("missing lair accepted")
					}
				}
			})
		}
	}
}

func TestCatanExplorerSixLastSeatEconomyAndFish(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			w := explorerStockWorld(t, n, "fish-for-catan")
			p, ship := n-1, n*3-1
			harbor := w.B.HarborStarts[0]
			w.G.Vertices[harbor].Owner, w.G.Vertices[harbor].Level = p, 2
			w.position(t, ship, func(e CatanEdge) bool { return e.A == harbor || e.B == harbor })
			w.start(t, p, 1)
			beforeGold := w.E.Gold[p]
			if err := w.E.bankTrade(w.G, w.F, w.C, p, 1, -1, 4); err != nil {
				t.Fatal(err)
			}
			if w.E.Gold[p] != beforeGold-2 || w.G.Players[p].Resources[4] != 1 || w.G.Bank[4] != 23 {
				t.Fatal("last seat economy")
			}
			w.movement(t)
			// Exercise the eighth fish and final two crew IDs through an actual full
			// harbor swap. IDs remain distinct across cargo types.
			unit := n*11 - 2
			w.C.Fish[7] = catanExplorerCargoLocation{"ship", ship}
			w.C.Units[unit], w.C.Units[unit+1] = catanExplorerCargoLocation{"harbor", harbor}, catanExplorerCargoLocation{"harbor", harbor}
			if err := w.C.transferFreight(w.G, w.F, p, 1, ship, harbor, []int{unit, unit + 1}, nil, nil, []int{7}); err != nil {
				t.Fatal(err)
			}
			w = w.restore(t)
			if err := w.C.transferFreight(w.G, w.F, p, 1, ship, harbor, nil, []int{unit, unit + 1}, []int{7}, nil); err != nil {
				t.Fatal(err)
			}
			w.position(t, ship, func(e CatanEdge) bool {
				return slices.Contains(w.B.Council.Anchors, e.A) || slices.Contains(w.B.Council.Anchors, e.B)
			})
			if err := w.Fish.apply(w.G, w.B, w.F, w.C, p, 1, "deliver", ship, 7, -1, nil); err != nil {
				t.Fatal(err)
			}
			if w.C.Fish[7].Kind != "supply" || w.Fish.publicView(n).Scores[p] != 2 {
				t.Fatal("eighth fish delivery")
			}
			w.restore(t)
		})
	}
}

func TestCatanExplorerSixAllSpiceClaimsAndDeliveries(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			w := explorerStockWorld(t, n, "spices-for-catan")
			var farms []int
			for _, h := range w.B.Hidden {
				if h.Farm != "" {
					farms = append(farms, h.Tile)
				}
			}
			for p := 0; p < n; p++ {
				ship, seq := p*3+2, uint64(p+1)
				w.position(t, ship, func(e CatanEdge) bool { return catanExplorerTouches(w.G, e.ID, farms[0]) })
				w.start(t, p, seq)
				w.movement(t)
				for i, tile := range farms {
					unit := p*11 + 5 + i // includes the final crew ID of the sixth color.
					w.C.Units[unit] = catanExplorerCargoLocation{"ship", ship}
					w.position(t, ship, func(e CatanEdge) bool { return catanExplorerTouches(w.G, e.ID, tile) })
					if err := w.Spice.apply(w.G, w.B, w.F, w.C, w.E, p, seq, "land", tile, ship, unit); err != nil {
						t.Fatal(err)
					}
					sack := slices.IndexFunc(w.C.Spice, func(s catanExplorerSpiceSack) bool { return s.Owner == p && s.Origin == tile })
					if sack < 0 {
						t.Fatal("missing claim")
					}
					w = w.restore(t)
					w.position(t, ship, func(e CatanEdge) bool {
						return slices.Contains(w.B.Council.Anchors, e.A) || slices.Contains(w.B.Council.Anchors, e.B)
					})
					if err := w.Spice.apply(w.G, w.B, w.F, w.C, w.E, p, seq, "deliver", tile, ship, sack); err != nil {
						t.Fatal(err)
					}
					raw, _ := json.Marshal(w)
					if w.Spice.apply(w.G, w.B, w.F, w.C, w.E, p, seq, "deliver", tile, ship, sack) == nil {
						t.Fatal("duplicate delivery")
					}
					after, _ := json.Marshal(w)
					if string(raw) != string(after) {
						t.Fatal("duplicate mutated inventory")
					}
				}
				if w.Spice.publicView(w.G).Progress[p] != 6 {
					t.Fatal("six farm score")
				}
				if err := w.C.endMovement(w.G, w.F, p, seq); err != nil {
					t.Fatal(err)
				}
				w.F.Positions[ship] = -1 // Remove fixture vessel so later seats can dock.
				w = w.restore(t)
			}
			if len(w.Spice.Deliveries) != 6*n {
				t.Fatal("missing deliveries")
			}
			if n == 6 && w.C.Spice[35].Owner != 5 {
				t.Fatal("last physical sack not exercised")
			}
		})
	}
}

func TestCatanExplorerSixEighthFishSpawn(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			w := explorerStockWorld(t, n, "fish-for-catan")
			p, ship := n-1, n*3-1
			for id := 0; id < 7; id++ {
				w.position(t, id, func(CatanEdge) bool { return true })
				w.C.Fish[id] = catanExplorerCargoLocation{"ship", id}
			}
			shoal := w.B.publicView().Shoals[0]
			w.position(t, ship, func(e CatanEdge) bool { return catanExplorerTouches(w.G, e.ID, shoal.Tile) })
			w.start(t, p, 1)
			w.movement(t)
			if err := w.Fish.apply(w.G, w.B, w.F, w.C, p, 1, "roll", ship, 0, -1, func(int) int { return shoal.Number - 1 }); err != nil {
				t.Fatal(err)
			}
			if w.Fish.LastRoll.Spawned != 7 {
				t.Fatal("eighth fish did not spawn")
			}
			w = w.restore(t)
			if err := w.Fish.apply(w.G, w.B, w.F, w.C, p, 1, "load", ship, 7, -1, nil); err != nil {
				t.Fatal(err)
			}
			if w.C.Fish[7] != (catanExplorerCargoLocation{"ship", ship}) {
				t.Fatal("last fish load")
			}
			w.restore(t)
		})
	}
}

func TestCatanExplorerSixLairLastSeatRewardsAndPrivacy(t *testing.T) {
	for _, n := range []int{5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			w := explorerStockWorld(t, n, "pirate-lairs")
			p, ship, unit := n-1, n*3-2, n*11-3
			tile := -1
			for _, site := range w.L.Sites {
				for _, edge := range w.G.Edges {
					if catanExplorerSeaEdge(w.G, edge.ID) && catanExplorerTouches(w.G, edge.ID, site.Tile) {
						tile = site.Tile
						break
					}
				}
				if tile >= 0 {
					break
				}
			}
			if tile < 0 {
				t.Fatal("fixture coastal lair")
			}
			// All eight site faces must stay secret, including after restore. Swapping
			// secret token assignments conserves inventory and changes no public view.
			other := clone(*w.L)
			other.Sites[0].Number, other.Sites[7].Number = other.Sites[7].Number, other.Sites[0].Number
			if err := other.validate(w.G, w.B, w.F, w.C, w.E); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(w.L.publicView(), other.publicView()) {
				t.Fatal("eighth number leaked")
			}
			for _, id := range []int{ship, ship + 1} {
				w.position(t, id, func(e CatanEdge) bool { return catanExplorerTouches(w.G, e.ID, tile) })
			}
			w.C.Units[unit], w.C.Units[unit+1] = catanExplorerCargoLocation{"ship", ship}, catanExplorerCargoLocation{"ship", ship}
			w.C.Units[unit+2] = catanExplorerCargoLocation{"ship", ship + 1}
			w.start(t, p, 1)
			w.movement(t)
			act := func(kind string, s int, ids []int) error {
				return w.L.apply(w.G, w.B, w.F, w.C, w.E, p, 1, kind, tile, s, ids, nil)
			}
			if err := act("land", ship, []int{unit, unit + 1}); err != nil {
				t.Fatal(err)
			}
			if err := act("land", ship+1, []int{unit + 2}); err != nil {
				t.Fatal(err)
			}
			w = w.restore(t)
			if err := w.C.endMovement(w.G, w.F, p, 1); err != nil {
				t.Fatal(err)
			}
			beforeGold := w.E.Gold[p]
			if err := act("begin", ship, nil); err != nil {
				t.Fatal(err)
			}
			site := w.L.Sites[w.L.site(tile)]
			if site.Hero != p || site.Resolved != 1 || len(site.Contributions) != n || site.Contributions[p] != 3 || w.E.Gold[p] != beforeGold+2 || w.G.Tiles[tile].Number != site.Number || w.L.Progress[p] != 2 || w.C.Units[unit].Kind != "supply" {
				t.Fatal("last seat lair settlement", site)
			}
			w = w.restore(t)
			before, _ := json.Marshal(w)
			if act("begin", ship, nil) == nil {
				t.Fatal("duplicate settlement")
			}
			after, _ := json.Marshal(w)
			if string(before) != string(after) {
				t.Fatal("duplicate settlement mutated")
			}
		})
	}
	for _, n := range []int{5, 6} {
		if _, err := newCatanExplorerLairs(n, []int{2, 3, 4, 5, 6, 8}); err == nil {
			t.Fatal("base tokens accepted for expanded game")
		}
	}
}
