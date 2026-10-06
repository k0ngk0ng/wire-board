package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type explorerSpiceFixture struct {
	Game    *Catan
	Board   *catanExplorerBoard
	Fleet   *catanExplorerSailing
	Cargo   *catanExplorerCargo
	Economy *catanExplorerEconomy
	Mission *catanExplorerSpice
	Harbor  int
}

func explorerSpiceWorld(t *testing.T) *explorerSpiceFixture {
	t.Helper()
	g, b, err := newCatanExplorerBoard(3, "spices-for-catan", "variable")
	if err != nil {
		t.Fatal(err)
	}
	// A legal explicit shuffle places all farms on the outer coastline. It is
	// a rule fixture, not a naturally played or randomly accessible whole game.
	for region, tiles := range [2][]int{{4, 5, 6}, {61, 62, 63}} {
		for j, ability := range []string{"swift", "pirate", "gold"} {
			from, to := -1, -1
			for i, h := range b.Hidden {
				if h.Region == region && h.Farm == ability {
					from = i
				}
				if h.Tile == tiles[j] {
					to = i
				}
			}
			a, z := b.Hidden[from], b.Hidden[to]
			a.Tile, z.Tile = z.Tile, a.Tile
			b.Hidden[from], b.Hidden[to] = z, a
		}
	}
	if err = b.validate(g); err != nil {
		t.Fatal(err)
	}
	f, _ := newCatanExplorerSailing(3)
	g.Bank = []int{19, 19, 19, 19, 19}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
	}
	c, err := newCatanExplorerCargo(g, f, "spices-for-catan")
	if err != nil {
		t.Fatal(err)
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
	}
	harbor := b.HarborStarts[0]
	g.Vertices[harbor].Owner, g.Vertices[harbor].Level = 0, 2
	id := 0
	for _, edge := range g.Edges {
		if catanExplorerSeaEdge(g, edge.ID) {
			f.Positions[id] = edge.ID
			id++
			if id == 3 {
				break
			}
		}
	}
	e, err := newCatanExplorerEconomy(g, f, c)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.beginProduction(g, f, c, 0, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = e.resolveProduction(g, f, c, 0, 1, [2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	w := &explorerSpiceFixture{g, b, f, c, e, &catanExplorerSpice{Deliveries: []catanExplorerSpiceDelivery{}}, harbor}
	w.check(t)
	return w
}
func (w *explorerSpiceFixture) check(t *testing.T) {
	t.Helper()
	if err := w.Mission.validate(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy); err != nil {
		t.Fatal(err)
	}
}
func (w *explorerSpiceFixture) restore(t *testing.T) *explorerSpiceFixture {
	t.Helper()
	raw, _ := json.Marshal(w)
	var next explorerSpiceFixture
	if err := json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	next.check(t)
	if !reflect.DeepEqual(w, &next) {
		t.Fatal("spice snapshot changed")
	}
	return &next
}
func (w *explorerSpiceFixture) movePhase(t *testing.T) {
	t.Helper()
	p, seq := w.Cargo.Turn.Player, w.Cargo.Turn.Sequence
	if err := w.Cargo.beginMovement(w.Game, w.Fleet, p, seq, w.Cargo.farmCount(w.Board, p, "swift")); err != nil {
		t.Fatal(err)
	}
}
func (w *explorerSpiceFixture) next(t *testing.T, p int) {
	t.Helper()
	if w.Cargo.Turn.Phase == "action" {
		w.movePhase(t)
	}
	old := w.Cargo.Turn
	if err := w.Cargo.endMovement(w.Game, w.Fleet, old.Player, old.Sequence); err != nil {
		t.Fatal(err)
	}
	if err := w.Economy.beginProduction(w.Game, w.Fleet, w.Cargo, p, old.Sequence+1); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Economy.resolveProduction(w.Game, w.Fleet, w.Cargo, p, old.Sequence+1, [2]int{1, 1}); err != nil {
		t.Fatal(err)
	}
	w.check(t)
}
func (w *explorerSpiceFixture) position(t *testing.T, ship, tile int) {
	t.Helper()
	for _, edge := range w.Game.Edges {
		if !catanExplorerSeaEdge(w.Game, edge.ID) || !catanExplorerTouches(w.Game, edge.ID, tile) {
			continue
		}
		n := 0
		for id, at := range w.Fleet.Positions {
			if id != ship && at == edge.ID {
				n++
			}
		}
		if n < 2 {
			w.Fleet.Positions[ship] = edge.ID
			return
		}
	}
	t.Fatal("fixture farm has no sea access", tile)
}
func (w *explorerSpiceFixture) anchor(t *testing.T, ship int) {
	t.Helper()
	a := w.Board.Council.Anchors[0]
	for _, e := range w.Game.Edges {
		if (e.A == a || e.B == a) && catanExplorerSeaEdge(w.Game, e.ID) {
			n := 0
			for id, at := range w.Fleet.Positions {
				if id != ship && at == e.ID {
					n++
				}
			}
			if n < 2 {
				w.Fleet.Positions[ship] = e.ID
				return
			}
		}
	}
	t.Fatal("anchor")
}
func (w *explorerSpiceFixture) farm(ability string, region int) int {
	for _, h := range w.Board.Hidden {
		if h.Farm == ability && h.Region == region {
			return h.Tile
		}
	}
	return -1
}
func (w *explorerSpiceFixture) act(kind string, tile, ship, piece int) error {
	return w.Mission.apply(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy, w.Cargo.Turn.Player, w.Cargo.Turn.Sequence, kind, tile, ship, piece)
}
func (w *explorerSpiceFixture) land(t *testing.T, tile, ship, unit int) int {
	t.Helper()
	if w.Cargo.Units[unit] != (catanExplorerCargoLocation{"supply", -1}) {
		t.Fatal("fixture reuses deployed unit")
	}
	w.Cargo.Units[unit] = catanExplorerCargoLocation{"ship", ship}
	w.position(t, ship, tile)
	if err := w.act("land", tile, ship, unit); err != nil {
		t.Fatal(err)
	}
	for id, s := range w.Cargo.Spice {
		if s.Origin == tile && s.Owner == unit/11 {
			return id
		}
	}
	t.Fatal("land did not allocate sack")
	return -1
}
func (w *explorerSpiceFixture) reject(t *testing.T, act func() error) {
	t.Helper()
	raw, _ := json.Marshal(w)
	if act() == nil {
		t.Fatal("invalid spice action accepted")
	}
	after, _ := json.Marshal(w)
	if string(raw) != string(after) {
		t.Fatal("failed action partially changed spice state")
	}
}

func TestCatanExplorerSpiceDiscoveryInventory(t *testing.T) {
	for n := 2; n <= 4; n++ {
		g, b, err := newCatanExplorerBoard(n, "spices-for-catan", "variable")
		if err != nil {
			t.Fatal(err)
		}
		f, _ := newCatanExplorerSailing(n)
		c, err := newCatanExplorerCargo(g, f, b.Scenario)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Spice) != 24 || len(c.Fish) != 6 {
			t.Fatal("physical cargo stock")
		}
		farms := 0
		for _, h := range slices.Clone(b.Hidden) {
			if h.Farm != "" && c.discoverSpice(g, b, f, h.Tile) == nil {
				t.Fatal("loaded an undiscovered farm")
			}
			if _, err = b.reveal(g, h.Tile); err != nil {
				t.Fatal(err)
			}
			if h.Farm == "" {
				if c.discoverSpice(g, b, f, h.Tile) == nil {
					t.Fatal("ordinary terrain spawned sacks")
				}
				continue
			}
			if err = c.discoverSpice(g, b, f, h.Tile); err != nil {
				t.Fatal(err)
			}
			farms++
			count := 0
			for _, s := range c.Spice {
				if s.Origin >= 0 {
					count++
				}
				if s.Owner != -1 {
					t.Fatal("discovery claimed a sack")
				}
			}
			if count != farms*n || len(c.spiceContents(catanExplorerCargoLocation{"farm", h.Tile})) != n {
				t.Fatal("discovery allocation")
			}
			before, _ := json.Marshal(c)
			if c.discoverSpice(g, b, f, h.Tile) == nil {
				t.Fatal("duplicate discovery accepted")
			}
			after, _ := json.Marshal(c)
			if string(before) != string(after) {
				t.Fatal("duplicate mutated cargo")
			}
		}
		if farms != 6 || c.validate(g, f) != nil {
			t.Fatal("final physical cargo invariant")
		}
	}
}

func TestCatanExplorerSpiceLandDeliverSixFarmsAndRestore(t *testing.T) {
	w := explorerSpiceWorld(t)
	w.movePhase(t)
	// An already closed ship does not regain movement from a new Swift farm.
	w.Fleet.Turn.Ships[1].Closed = true
	w.Fleet.Turn.Ships[1].Remaining = 0
	expectedPoints := []int{1, 1, 2, 2, 3, 3}
	for i, tile := range []int{4, 5, 6, 61, 62, 63} {
		oldMP := w.Fleet.Turn.Ships[2].Remaining
		sack := w.land(t, tile, 0, 2+i)
		if w.Cargo.Units[2+i] != (catanExplorerCargoLocation{"farm", tile}) || w.Cargo.used(catanExplorerCargoLocation{"ship", 0}) != 1 {
			t.Fatal("crew/sack swap")
		}
		if (tile == 4 || tile == 61) && w.Fleet.Turn.Ships[2].Remaining != oldMP+1 {
			t.Fatal("swift bonus did not reach another ship")
		}
		if !w.Fleet.Turn.Ships[1].Closed || w.Fleet.Turn.Ships[1].Remaining != 0 {
			t.Fatal("swift reopened completed ship")
		}
		w.reject(t, func() error { return w.act("land", tile, 0, 2+i) })
		w = w.restore(t)
		w.anchor(t, 0)
		if err := w.act("deliver", tile, 0, sack); err != nil {
			t.Fatal(err)
		}
		v := w.Mission.publicView(w.Game)
		if v.Progress[0] != i+1 || v.Scores[0] != expectedPoints[i]+1 || v.Leader != 0 {
			t.Fatal("six-space mission scoring", i, v)
		}
		w.reject(t, func() error { return w.act("deliver", tile, 0, sack) })
		if !w.Cargo.farmFriend(0, tile) || w.Cargo.Spice[sack].At != (catanExplorerCargoLocation{"supply", -1}) {
			t.Fatal("delivery lost permanent farm identity")
		}
		w = w.restore(t)
	}
	if w.Fleet.Turn.Speed != 2 || w.Cargo.farmCount(w.Board, 0, "gold") != 2 || !slices.Equal(w.Cargo.pirateFarmDice(w.Board, 0), []int{4, 5}) {
		t.Fatal("six permanent abilities")
	}
	w.next(t, 0)
	w.movePhase(t)
	if w.Fleet.Turn.Ships[0].Remaining != 6 {
		t.Fatal("speed bonus missing on next turn")
	}
}

func (w *explorerSpiceFixture) dock(t *testing.T, ship int) {
	t.Helper()
	for _, e := range w.Game.Edges {
		if (e.A == w.Harbor || e.B == w.Harbor) && catanExplorerSeaEdge(w.Game, e.ID) {
			n := 0
			for id, at := range w.Fleet.Positions {
				if id != ship && at == e.ID {
					n++
				}
			}
			if n < 2 {
				w.Fleet.Positions[ship] = e.ID
				return
			}
		}
	}
	t.Fatal("dock")
}
func (w *explorerSpiceFixture) stock(p, r, n int) {
	w.Game.Players[p].Resources[r] += n
	w.Game.Bank[r] -= n
}
func TestCatanExplorerSpiceMixedTransferAndPermanentCrew(t *testing.T) {
	w := explorerSpiceWorld(t)
	w.movePhase(t)
	a := w.land(t, w.farm("gold", 0), 0, 2)
	b := w.land(t, w.farm("gold", 1), 0, 3)
	if w.Cargo.used(catanExplorerCargoLocation{"ship", 0}) != 2 {
		t.Fatal("two sacks should fit")
	}
	w.dock(t, 0)
	ship, harbor := catanExplorerCargoLocation{"ship", 0}, catanExplorerCargoLocation{"harbor", w.Harbor}
	w.Cargo.Units[0] = harbor // One large settler fills the other container.
	transfer := func(load, unload, loadFish, unloadFish, loadSpice, unloadSpice []int) error {
		return w.Cargo.transferAllFreight(w.Game, w.Fleet, 0, 1, 0, w.Harbor, load, unload, loadFish, unloadFish, loadSpice, unloadSpice)
	}
	w.reject(t, func() error { return transfer(nil, nil, nil, nil, nil, []int{a}) })
	w.reject(t, func() error { return transfer([]int{0}, nil, nil, nil, nil, []int{a, a}) })
	if err := transfer([]int{0}, nil, nil, nil, nil, []int{a, b}); err != nil {
		t.Fatal(err)
	}
	if w.Cargo.Units[0] != ship || w.Cargo.Spice[a].At != harbor || w.Cargo.Spice[b].At != harbor {
		t.Fatal("simultaneous two-sack/settler swap")
	}
	if err := transfer(nil, []int{0}, nil, nil, []int{a, b}, nil); err != nil {
		t.Fatal(err)
	}
	// Return the settler in this isolated fixture, then swap one sack for crew.
	w.Cargo.Units[0] = catanExplorerCargoLocation{"supply", -1}
	w.Cargo.Units[4] = harbor
	if err := transfer([]int{4}, nil, nil, nil, nil, []int{a}); err != nil {
		t.Fatal(err)
	}
	if w.Cargo.used(ship) != 2 || w.Cargo.used(harbor) != 1 {
		t.Fatal("small cargo capacity")
	}
	w.reject(t, func() error { return transfer(nil, nil, nil, nil, []int{a}, nil) })
	// A farm crew cannot be unloaded through a harbor action or bought again.
	w.reject(t, func() error { return transfer(nil, []int{2}, nil, nil, nil, nil) })
	w.Cargo.Units[4] = catanExplorerCargoLocation{"supply", -1}
	if err := transfer(nil, nil, nil, nil, []int{a}, nil); err != nil {
		t.Fatal(err)
	}
	// Fish occupy both slots; exchanging two sacks for a haul is legal atomically.
	w.Cargo.Fish[0] = harbor
	w.check(t)
	if err := transfer(nil, nil, []int{0}, nil, nil, []int{a, b}); err != nil {
		t.Fatal(err)
	}
	if w.Cargo.Fish[0] != ship || w.Cargo.used(ship) != 2 || w.Cargo.used(harbor) != 2 {
		t.Fatal("fish/spice shared capacity")
	}
	w.reject(t, func() error { return transfer(nil, nil, nil, nil, []int{a}, nil) })
	if err := transfer(nil, nil, nil, []int{0}, []int{a, b}, nil); err != nil {
		t.Fatal(err)
	}
	// Transfer via the real harbor into another ship; direct ship-to-ship is absent.
	w.Cargo.Fish[0] = catanExplorerCargoLocation{"supply", -1}
	if err := transfer(nil, nil, nil, nil, nil, []int{a, b}); err != nil {
		t.Fatal(err)
	}
	w.dock(t, 1)
	if err := w.Cargo.transferAllFreight(w.Game, w.Fleet, 0, 1, 1, w.Harbor, nil, nil, nil, nil, []int{a, b}, nil); err != nil {
		t.Fatal(err)
	}
	w = w.restore(t)
	w.anchor(t, 1)
	if err := w.act("deliver", 0, 1, a); err != nil {
		t.Fatal(err)
	}
	if err := w.act("deliver", 0, 1, b); err != nil {
		t.Fatal(err)
	}
	w.check(t)
}
func TestCatanExplorerSpiceGoldLimitsAndLostCargo(t *testing.T) {
	w := explorerSpiceWorld(t)
	w.movePhase(t)
	a := w.land(t, w.farm("gold", 0), 0, 2)
	w.stock(0, 0, 4)
	w.reject(t, func() error { return w.act("gold", 0, 0, 0) }) // Movement phase, despite the newly gained ability.
	w.next(t, 0)
	gold := w.Economy.Gold[0]
	if err := w.act("gold", 0, 0, 0); err != nil {
		t.Fatal(err)
	}
	if w.Economy.Gold[0] != gold+1 || w.Mission.GoldUse.Count != 1 {
		t.Fatal("gold exchange")
	}
	w = w.restore(t)
	w.reject(t, func() error { return w.act("gold", 0, 0, 0) })
	w.movePhase(t)
	b := w.land(t, w.farm("gold", 1), 0, 3)
	w.next(t, 0)
	for i := 0; i < 2; i++ {
		if err := w.act("gold", 0, 0, 0); err != nil {
			t.Fatal(err)
		}
	}
	w.reject(t, func() error { return w.act("gold", 0, 0, 0) })
	if w.Economy.Turn.Bought != 0 {
		t.Fatal("fast gold incorrectly consumed ordinary gold-buy quota")
	}
	// Recycle a deployed ship while all three slots are in use. Lost sacks keep
	// their origin/claim; they cannot reappear as a second farm entitlement.
	w.stock(0, 0, 1)
	w.stock(0, 2, 1)
	w.dock(t, 0)
	edge := -1
	for _, e := range w.Game.Edges {
		if (e.A == w.Harbor || e.B == w.Harbor) && catanExplorerSeaEdge(w.Game, e.ID) && !slices.Contains(w.Fleet.Positions, e.ID) {
			edge = e.ID
			break
		}
	}
	if edge < 0 {
		t.Fatal("fixture missing empty rebuild berth")
	}
	if err := w.Cargo.buildShip(w.Game, w.Fleet, 0, w.Cargo.Turn.Sequence, 0, edge); err != nil {
		t.Fatal(err)
	}
	if w.Cargo.Spice[a].At != (catanExplorerCargoLocation{"supply", -1}) || w.Cargo.Spice[b].At != (catanExplorerCargoLocation{"supply", -1}) || w.Mission.publicView(w.Game).Progress[0] != 0 {
		t.Fatal("lost cargo must not score")
	}
	w.movePhase(t)
	tile := w.farm("gold", 0)
	w.position(t, 0, tile)
	w.Cargo.Units[4] = catanExplorerCargoLocation{"ship", 0}
	w.reject(t, func() error { return w.act("land", tile, 0, 4) })
	w = w.restore(t)
	w.next(t, 0)
	// Existing unresolved empty-gold-bank rule remains an atomic rejection.
	w.Economy.Gold[1] += w.Economy.GoldBank
	w.Economy.GoldBank = 0
	w.stock(0, 1, 1)
	w.reject(t, func() error { return w.act("gold", 0, 0, 1) })
}
func TestCatanExplorerSpicePermissionsAndInvalidRequests(t *testing.T) {
	w := explorerSpiceWorld(t)
	w.movePhase(t)
	tile := w.farm("pirate", 0)
	w.Cargo.Units[2] = catanExplorerCargoLocation{"ship", 0}
	w.position(t, 0, tile)
	for _, player := range []int{-1, 1, 3} {
		p := player
		w.reject(t, func() error {
			return w.Mission.apply(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy, p, 1, "land", tile, 0, 2)
		})
	}
	w.reject(t, func() error {
		return w.Mission.apply(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy, 0, 2, "land", tile, 0, 2)
	})
	w.reject(t, func() error { return w.act("land", tile, -1, 2) })
	w.reject(t, func() error { return w.act("land", tile, 0, 0) })
	w.reject(t, func() error { return w.act("land", w.Board.Starting[0], 0, 2) })
	w.reject(t, func() error { return w.act("pickup", tile, 0, 2) })
	edge := -1
	vertex := -1
	for _, e := range w.Game.Edges {
		if slices.Contains(e.Tiles, tile) && catanExplorerLandEdge(w.Game, e.ID) {
			edge = e.ID
			break
		}
	}
	for _, v := range w.Game.Tiles[tile].Vertices {
		if catanExplorerLandVertex(w.Game, v) {
			vertex = v
			break
		}
	}
	if edge < 0 || vertex < 0 {
		t.Fatal("farm building fixture")
	}
	if w.Cargo.landEdge(w.Game, 0, edge) || w.Cargo.landVertex(w.Game, 0, vertex) {
		t.Fatal("farm grants building before crew")
	}
	if err := w.act("land", tile, 0, 2); err != nil {
		t.Fatal(err)
	}
	// Some corners border another farm, requiring friendship with both. Choose
	// the farm's outer-frame edge/corner, which borders no second farm.
	granted := false
	for _, e := range w.Game.Edges {
		if slices.Contains(e.Tiles, tile) && w.Cargo.landEdge(w.Game, 0, e.ID) {
			granted = true
			if w.Cargo.landEdge(w.Game, 1, e.ID) {
				t.Fatal("opponent gained farm access")
			}
		}
	}
	if !granted {
		t.Fatal("crew did not unlock own farm roads")
	}
	w = w.restore(t)
	// Corrupting claim, permanent crew, capacity or duplicate deliveries rejects.
	for _, mutate := range []func(*explorerSpiceFixture){
		func(q *explorerSpiceFixture) { q.Cargo.Units[2] = catanExplorerCargoLocation{"supply", -1} },
		func(q *explorerSpiceFixture) { q.Cargo.Units[3] = q.Cargo.Units[2] },
		func(q *explorerSpiceFixture) { q.Cargo.Units[0] = catanExplorerCargoLocation{"ship", 0} },
		func(q *explorerSpiceFixture) {
			q.Cargo.Spice[q.Cargo.spiceContents(catanExplorerCargoLocation{"ship", 0})[0]].Owner = 1
		},
	} {
		bad := clone(*w)
		mutate(&bad)
		if bad.Mission.validate(bad.Game, bad.Board, bad.Fleet, bad.Cargo, bad.Economy) == nil {
			t.Fatal("invalid spice snapshot accepted")
		}
	}
}

func TestCatanExplorerSpicePirateBonusDiceAndHistoricRoll(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		for die := 1; die <= 6; die++ {
			w := explorerSpiceWorld(t)
			w.movePhase(t)
			unit := 2
			for region := 0; region < 2; region++ {
				if mask&(1<<region) != 0 {
					w.land(t, w.farm("pirate", region), 0, unit)
					unit++
				}
			}
			pirate := newCatanExplorerPirate()
			pirate.Owner = 1
			for _, h := range w.Game.Tiles {
				if catanExplorerPirateTile(w.Game, w.Board, h.ID) {
					pirate.Tile = h.ID
					break
				}
			}
			if pirate.Tile < 0 {
				t.Fatal("pirate fixture")
			}
			w.position(t, 0, pirate.Tile)
			_, err := pirate.apply(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy, 0, 1, "chase", 0, false, func(int) int { return die - 1 })
			if err != nil {
				t.Fatal(err)
			}
			want := die == 6 || mask&1 != 0 && die == 5 || mask&2 != 0 && die == 4
			if pirate.LastChase.Success != want || (pirate.Pending != nil) != want {
				t.Fatal("farm die bonus", mask, die, pirate.LastChase)
			}
			saved := clone(*pirate)
			if err = saved.validate(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy); err != nil {
				t.Fatal("saved chase", err)
			}
			if !reflect.DeepEqual(*pirate, saved) {
				t.Fatal("historic chase changed across save")
			}
			before, _ := json.Marshal(pirate)
			if _, err = pirate.apply(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy, 0, 1, "chase", 0, false, func(int) int { return 5 }); err == nil {
				t.Fatal("same ship chased twice")
			}
			after, _ := json.Marshal(pirate)
			if string(before) != string(after) {
				t.Fatal("repeat chase mutated")
			}
		}
	}
	// A later friendship must not turn a prior failed five into a success.
	w := explorerSpiceWorld(t)
	w.movePhase(t)
	pirate := newCatanExplorerPirate()
	pirate.Owner = 1
	for _, h := range w.Game.Tiles {
		if catanExplorerPirateTile(w.Game, w.Board, h.ID) {
			pirate.Tile = h.ID
			break
		}
	}
	w.position(t, 0, pirate.Tile)
	if _, err := pirate.apply(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy, 0, 1, "chase", 0, false, func(int) int { return 4 }); err != nil {
		t.Fatal(err)
	}
	w.land(t, w.farm("pirate", 0), 0, 2)
	if pirate.LastChase.Success || len(pirate.LastChase.Bonus) != 0 {
		t.Fatal("old roll retroactively gained bonus")
	}
	if err := pirate.validate(w.Game, w.Board, w.Fleet, w.Cargo, w.Economy); err != nil {
		t.Fatal(err)
	}
}

func TestCatanExplorerSpiceLeaderTieAndPublicView(t *testing.T) {
	w := explorerSpiceWorld(t)
	w.movePhase(t)
	a := w.land(t, 6, 0, 2)
	w.anchor(t, 0)
	if err := w.act("deliver", 0, 0, a); err != nil {
		t.Fatal(err)
	}
	// A second real claimant of the same farm; each has a distinct physical sack.
	w.position(t, 3, 6)
	w.next(t, 1)
	w.movePhase(t)
	b := w.land(t, 6, 3, 13)
	w.anchor(t, 3)
	if err := w.act("deliver", 0, 3, b); err != nil {
		t.Fatal(err)
	}
	v := w.Mission.publicView(w.Game)
	if v.Leader != 0 || !slices.Equal(v.Progress, []int{1, 1, 0}) || !slices.Equal(v.Scores, []int{2, 1, 0}) {
		t.Fatal("earliest arrival keeps tied bonus", v)
	}
	c := w.land(t, 63, 3, 14)
	w.anchor(t, 3)
	if err := w.act("deliver", 0, 3, c); err != nil {
		t.Fatal(err)
	}
	v = w.Mission.publicView(w.Game)
	if v.Leader != 1 || !slices.Equal(v.Scores, []int{1, 2, 0}) {
		t.Fatal("strict lead moves bonus", v)
	}
	w = w.restore(t)
	v.Progress[0] = 99
	v.Scores[1] = 99
	if w.Mission.publicView(w.Game).Progress[0] != 1 {
		t.Fatal("public progress aliases state")
	}
	// Platform retirement excludes the player from leadership, retaining earned
	// track points. Full resource/ship retirement integration is a later stage.
	w.Game.Players[1].Eliminated = true
	if v = w.Mission.publicView(w.Game); v.Leader != 0 || v.Scores[1] != 1 {
		t.Fatal("retired player kept leader bonus", v)
	}
}

func TestCatanExplorerSpiceFarmSettlementRoadAndHarbor(t *testing.T) {
	w := explorerSpiceWorld(t)
	w.movePhase(t)
	tile := w.farm("gold", 0)
	// Choose a coastal corner touching only this farm; adjacent farms would
	// each require the builder's own permanent crew as well.
	vertex, shipEdge := -1, -1
	for _, v := range w.Game.Tiles[tile].Vertices {
		if !catanExplorerLandVertex(w.Game, v) {
			continue
		}
		other := false
		for _, h := range w.Game.Tiles {
			if h.ID != tile && h.Resource == CatanDesert && slices.Contains(h.Vertices, v) {
				other = true
			}
		}
		if other {
			continue
		}
		for _, e := range w.Game.Edges {
			if (e.A == v || e.B == v) && catanExplorerSeaEdge(w.Game, e.ID) {
				vertex, shipEdge = v, e.ID
				break
			}
		}
		if vertex >= 0 {
			break
		}
	}
	if vertex < 0 {
		t.Fatal("coastal farm fixture")
	}
	w.Fleet.Positions[1] = shipEdge
	w.Cargo.Units[0] = catanExplorerCargoLocation{"ship", 1}
	w.reject(t, func() error { return w.Cargo.settle(w.Game, w.Fleet, 0, 1, 1, vertex) })
	w.land(t, tile, 0, 2)
	if err := w.Cargo.settle(w.Game, w.Fleet, 0, 1, 1, vertex); err != nil {
		t.Fatal(err)
	}
	if w.Game.Vertices[vertex].Owner != 0 || w.Cargo.Units[2] != (catanExplorerCargoLocation{"farm", tile}) || w.Fleet.Positions[1] != -1 {
		t.Fatal("farm settlement consumed permanent crew or retained settler ship")
	}
	w.check(t)
	w.next(t, 0)
	road := -1
	for _, e := range w.Game.Edges {
		if (e.A == vertex || e.B == vertex) && e.Owner == -1 && w.Cargo.landEdge(w.Game, 0, e.ID) {
			road = e.ID
			break
		}
	}
	if road < 0 {
		t.Fatal("farm road fixture")
	}
	w.stock(0, 0, 1)
	w.stock(0, 1, 1)
	if err := w.Cargo.buildRoad(w.Game, w.Fleet, 0, w.Cargo.Turn.Sequence, road); err != nil {
		t.Fatal(err)
	}
	w.stock(0, 3, 2)
	w.stock(0, 4, 2)
	if err := w.Cargo.buildHarbor(w.Game, w.Fleet, 0, w.Cargo.Turn.Sequence, vertex); err != nil {
		t.Fatal(err)
	}
	if w.Game.Vertices[vertex].Level != 2 || w.Game.Edges[road].Owner != 0 {
		t.Fatal("farm construction did not persist")
	}
	w = w.restore(t)
}
