package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

type explorerFishFixture struct {
	Game    *Catan
	Board   *catanExplorerBoard
	Fleet   *catanExplorerSailing
	Cargo   *catanExplorerCargo
	Mission *catanExplorerFish
	Harbor  int
}

func explorerFishWorld(t *testing.T) *explorerFishFixture {
	t.Helper()
	g, b, err := newCatanExplorerBoard(3, "fish-for-catan", "variable")
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range slices.Clone(b.Hidden) {
		if _, err = b.reveal(g, h.Tile); err != nil {
			t.Fatal(err)
		}
	}
	f, err := newCatanExplorerSailing(3)
	if err != nil {
		t.Fatal(err)
	}
	g.Bank = []int{19, 19, 19, 19, 19}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
	}
	c, err := newCatanExplorerCargo(g, f, "fish-for-catan")
	if err != nil {
		t.Fatal(err)
	}
	harbor := b.HarborStarts[0]
	g.Vertices[harbor].Owner, g.Vertices[harbor].Level = 0, 2
	for _, edge := range g.Edges {
		if (edge.A == harbor || edge.B == harbor) && catanExplorerSeaEdge(g, edge.ID) {
			f.Positions[0] = edge.ID
			break
		}
	}
	if err = c.beginAction(g, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err = c.beginMovement(g, f, 0, 1, 0); err != nil {
		t.Fatal(err)
	}
	w := &explorerFishFixture{g, b, f, c, &catanExplorerFish{Deliveries: []catanExplorerFishDelivery{}}, harbor}
	w.check(t)
	return w
}
func (w *explorerFishFixture) check(t *testing.T) {
	t.Helper()
	if err := w.Mission.validate(w.Game, w.Board, w.Fleet, w.Cargo); err != nil {
		t.Fatal(err)
	}
}
func (w *explorerFishFixture) restore(t *testing.T) *explorerFishFixture {
	t.Helper()
	raw, _ := json.Marshal(w)
	var next explorerFishFixture
	if err := json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	next.check(t)
	if !reflect.DeepEqual(w, &next) {
		t.Fatal("fish save changed on restore")
	}
	return &next
}
func (w *explorerFishFixture) act(kind string, ship, fish, pirate, die int) error {
	return w.Mission.apply(w.Game, w.Board, w.Fleet, w.Cargo, w.Cargo.Turn.Player, w.Cargo.Turn.Sequence, kind, ship, fish, pirate, func(int) int { return die - 1 })
}
func (w *explorerFishFixture) next(t *testing.T, player int) {
	t.Helper()
	seq := w.Cargo.Turn.Sequence
	if err := w.Cargo.endMovement(w.Game, w.Fleet, w.Cargo.Turn.Player, seq); err != nil {
		t.Fatal(err)
	}
	if err := w.Cargo.beginAction(w.Game, w.Fleet, player, seq+1); err != nil {
		t.Fatal(err)
	}
	if err := w.Cargo.beginMovement(w.Game, w.Fleet, player, seq+1, 0); err != nil {
		t.Fatal(err)
	}
}
func (w *explorerFishFixture) firstShoal() catanExplorerShoal { return w.Board.publicView().Shoals[0] }
func (w *explorerFishFixture) position(ship, tile int) {
	for _, edge := range w.Game.Edges {
		if catanExplorerSeaEdge(w.Game, edge.ID) && slices.Contains(edge.Tiles, tile) {
			w.Fleet.Positions[ship] = edge.ID
			return
		}
	}
}

func TestCatanExplorerFishRollConditionsAndPersistence(t *testing.T) {
	for die := 1; die <= 6; die++ {
		w := explorerFishWorld(t)
		tile := -1
		for _, h := range w.Board.Hidden {
			if h.Fish == die {
				tile = h.Tile
			}
		}
		if err := w.act("roll", 0, 0, -1, die); err != nil {
			t.Fatal(err)
		}
		if (w.Mission.LastRoll.Spawned >= 0) != (tile >= 0) {
			t.Fatal("roll does not match printed die face")
		}
		if tile >= 0 && w.Cargo.Fish[0] != (catanExplorerCargoLocation{"shoal", tile}) {
			t.Fatal("fish appeared at wrong shoal")
		}
		w = w.restore(t)
		before, _ := json.Marshal(w)
		if err := w.act("roll", 0, 0, -1, die); err == nil {
			t.Fatal("rolled twice after restore")
		}
		after, _ := json.Marshal(w)
		if string(before) != string(after) {
			t.Fatal("rejected repeat mutated")
		}
		w.next(t, 0)
		if err := w.act("roll", 0, 0, -1, die); err != nil {
			t.Fatal(err)
		}
		if w.Mission.LastRoll.Spawned != -1 {
			t.Fatal("spawned second haul on occupied/absent shoal")
		}
	}
	for _, reason := range []string{"pirate", "hidden", "empty-supply"} {
		t.Run(reason, func(t *testing.T) {
			w := explorerFishWorld(t)
			shoal := w.firstShoal()
			pirate := -1
			if reason == "pirate" {
				pirate = shoal.Tile
			}
			if reason == "hidden" {
				for i, h := range w.Board.Hidden {
					if h.Tile == shoal.Tile {
						w.Board.Hidden[i].Revealed = false
						w.Game.Tiles[h.Tile].Resource = CatanFog
					}
				}
			}
			if reason == "empty-supply" {
				// Six legal deployed ships consume all six shared hauls. No extra
				// resource/mission score is granted to fabricate a successful catch.
				ships := []int{0, 1, 2, 3, 4, 5}
				edge := 0
				for id, ship := range ships {
					for !catanExplorerSeaEdge(w.Game, edge) {
						edge++
					}
					w.Fleet.Positions[ship] = edge
					if ship > 0 && ship < 3 {
						w.Fleet.Turn.Ships[ship] = catanExplorerShipMove{Remaining: 4}
					}
					w.Cargo.Fish[id] = catanExplorerCargoLocation{"ship", ship}
					edge++
				}
			}
			w.check(t)
			if err := w.act("roll", 0, 0, pirate, shoal.Number); err != nil {
				t.Fatal(err)
			}
			if w.Mission.LastRoll.Spawned != -1 {
				t.Fatal("blocked catch spawned")
			}
			w.restore(t)
		})
	}
}

func TestCatanExplorerFishLoadExhaustedShipAndSharedCapacity(t *testing.T) {
	w := explorerFishWorld(t)
	shoal := w.firstShoal()
	w.position(0, shoal.Tile)
	w.Fleet.Turn.Current = 0
	w.Fleet.Turn.Ships[0] = catanExplorerShipMove{Spent: 4}
	if err := w.act("roll", 0, 0, -1, shoal.Number); err != nil {
		t.Fatal(err)
	}
	before := clone(*w.Fleet)
	// Even one crew prevents a size-two fish haul from fitting.
	w.Cargo.Units[2] = catanExplorerCargoLocation{"ship", 0}
	if err := w.act("load", 0, 0, -1, 0); err == nil {
		t.Fatal("fish squeezed beside crew")
	}
	w.Cargo.Units[2] = catanExplorerCargoLocation{"supply", -1}
	if err := w.act("load", 0, 0, -1, 0); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, *w.Fleet) || w.Cargo.used(catanExplorerCargoLocation{"ship", 0}) != 2 || len(w.Mission.Deliveries) != 0 {
		t.Fatal("loading spent MPs or awarded progress")
	}
	w = w.restore(t)
	if err := w.act("load", 0, 0, -1, 0); err == nil {
		t.Fatal("loaded same fish twice")
	}
	// Pirate placement clears only uncollected fish, never cargo on that coast.
	w.Cargo.Fish[1] = catanExplorerCargoLocation{"shoal", shoal.Tile}
	w.Cargo.removeShoalFish(shoal.Tile)
	if w.Cargo.Fish[0] != (catanExplorerCargoLocation{"ship", 0}) || w.Cargo.Fish[1] != (catanExplorerCargoLocation{"supply", -1}) {
		t.Fatal("pirate discarded loaded fish")
	}
	w.check(t)
}

func TestCatanExplorerFishDeliveryOnlyAtPrintedAnchors(t *testing.T) {
	base := explorerFishWorld(t)
	eligible := 0
	for _, edge := range base.Game.Edges {
		if !catanExplorerSeaEdge(base.Game, edge.ID) {
			continue
		}
		w := clone(*base)
		w.Fleet.Positions[0] = edge.ID
		w.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", 0}
		// Independent geometric oracle: north/south tips of the council hex.
		island := w.Game.Tiles[w.Board.Council.Tile]
		want := false
		for _, id := range []int{edge.A, edge.B} {
			v := w.Game.Vertices[id]
			if catanExplorerCoordinate(v.X, island.X) && (catanExplorerCoordinate(v.Y, island.Y-w.Game.HexSize) || catanExplorerCoordinate(v.Y, island.Y+w.Game.HexSize)) {
				want = true
			}
		}
		before, _ := json.Marshal(w)
		err := w.act("deliver", 0, 0, -1, 0)
		if (err == nil) != want {
			t.Fatal("anchor delivery legality", edge.ID, err)
		}
		if want {
			eligible++
			view := w.Mission.publicView(3)
			if !slices.Equal(view.Progress, []int{1, 0, 0}) || !slices.Equal(view.Scores, []int{2, 0, 0}) || view.Leader != 0 || w.Cargo.Fish[0] != (catanExplorerCargoLocation{"supply", -1}) {
				t.Fatal("delivery did not return fish and score")
			}
			w.restore(t)
		} else {
			after, _ := json.Marshal(w)
			if string(before) != string(after) {
				t.Fatal("invalid delivery partially scored")
			}
		}
	}
	if eligible != 6 {
		t.Fatal("two three-way anchor vertices expected", eligible)
	}
}

func TestCatanExplorerFishFreightSwapsAndHarborTransshipment(t *testing.T) {
	w := explorerFishWorld(t)
	c, g, f := w.Cargo, w.Game, w.Fleet
	ship, harbor := catanExplorerCargoLocation{"ship", 0}, catanExplorerCargoLocation{"harbor", w.Harbor}
	c.Fish[0] = ship
	c.Units[2], c.Units[3] = harbor, harbor
	if err := c.transferFreight(g, f, 0, 1, 0, w.Harbor, []int{2, 3}, nil, nil, []int{0}); err != nil {
		t.Fatal(err)
	}
	if c.Fish[0] != harbor || c.Units[2] != ship || c.Units[3] != ship {
		t.Fatal("full cargo swap did not occur")
	}
	if err := c.transferFreight(g, f, 0, 1, 0, w.Harbor, nil, []int{2, 3}, []int{0}, nil); err != nil {
		t.Fatal(err)
	}
	c.Units[2], c.Units[3] = catanExplorerCargoLocation{"supply", -1}, catanExplorerCargoLocation{"supply", -1}
	c.Units[0] = harbor
	if err := c.transferFreight(g, f, 0, 1, 0, w.Harbor, []int{0}, nil, nil, []int{0}); err != nil {
		t.Fatal("fish and unit ID namespaces must remain independent", err)
	}
	c.Units[0] = catanExplorerCargoLocation{"supply", -1}
	f.Positions[1] = f.Positions[0]
	f.Turn.Ships[1] = catanExplorerShipMove{Remaining: 4}
	if err := c.transferFreight(g, f, 0, 1, 1, w.Harbor, nil, nil, []int{0}, nil); err != nil {
		t.Fatal(err)
	}
	if c.Fish[0] != (catanExplorerCargoLocation{"ship", 1}) {
		t.Fatal("harbor did not transship to second ship")
	}
	before, _ := json.Marshal(w)
	if err := c.transferFreight(g, f, 0, 1, 1, w.Harbor, nil, nil, nil, []int{0, 0}); err == nil {
		t.Fatal("duplicate fish accepted")
	}
	after, _ := json.Marshal(w)
	if string(before) != string(after) {
		t.Fatal("invalid swap partially unloaded")
	}
	w.restore(t)
}

func TestCatanExplorerFishPermissionAtomicityAndScoreTies(t *testing.T) {
	w := explorerFishWorld(t)
	for _, args := range []struct {
		player int
		seq    uint64
		die    int
	}{{1, 1, 1}, {0, 2, 1}, {0, 1, 0}, {0, 1, 7}} {
		before, _ := json.Marshal(w)
		if err := w.Mission.apply(w.Game, w.Board, w.Fleet, w.Cargo, args.player, args.seq, "roll", 0, 0, -1, func(int) int { return args.die - 1 }); err == nil {
			t.Fatal("bad actor/sequence/die accepted")
		}
		after, _ := json.Marshal(w)
		if string(before) != string(after) {
			t.Fatal("rejected roll mutated")
		}
	}
	// Explicit deliveries at legal anchor berths, advancing the real cargo
	// phases each time. Fish are deliberately supplied by this boundary fixture.
	anchor := w.Board.Council.Anchors[0]
	edges := []int{}
	for _, e := range w.Game.Edges {
		if (e.A == anchor || e.B == anchor) && catanExplorerSeaEdge(w.Game, e.ID) {
			edges = append(edges, e.ID)
		}
	}
	for i, p := range []int{0, 1, 0, 1, 1} {
		if i > 0 {
			w.next(t, p)
		}
		ship := p * 3
		w.Fleet.Positions[ship] = edges[p]
		w.Fleet.Turn.Ships[ship] = catanExplorerShipMove{Remaining: 4}
		w.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", ship}
		if err := w.act("deliver", ship, 0, -1, 0); err != nil {
			t.Fatal(err)
		}
		want := 0
		if i == 4 {
			want = 1
		}
		if w.Mission.publicView(3).Leader != want {
			t.Fatal("earliest arrival did not retain tie")
		}
		w = w.restore(t)
	}
	view := w.Mission.publicView(3)
	if !slices.Equal(view.Progress, []int{2, 3, 0}) || !slices.Equal(view.Scores, []int{1, 3, 0}) {
		t.Fatal("mission scores", view)
	}
	bad := clone(*w)
	bad.Mission.Deliveries[0].Sequence = 99
	if err := bad.Mission.validate(bad.Game, bad.Board, bad.Fleet, bad.Cargo); err == nil {
		t.Fatal("future delivery accepted")
	}
	bad = clone(*w)
	bad.Cargo.Fish[0] = catanExplorerCargoLocation{"shoal", bad.Board.Council.Tile}
	if err := bad.Mission.validate(bad.Game, bad.Board, bad.Fleet, bad.Cargo); err == nil {
		t.Fatal("fish on council accepted")
	}
}

func TestCatanExplorerFishCargoCannotBeOverfilledAndRecycledShipReturnsHaul(t *testing.T) {
	w := explorerFishWorld(t)
	c, g, f := w.Cargo, w.Game, w.Fleet
	shipLoc, harborLoc := catanExplorerCargoLocation{"ship", 0}, catanExplorerCargoLocation{"harbor", w.Harbor}
	c.Fish[0] = shipLoc
	c.Units[2] = harborLoc
	before, _ := json.Marshal(w)
	if err := c.transferFreight(g, f, 0, 1, 0, w.Harbor, nil, nil, nil, []int{0}); err == nil {
		t.Fatal("fish unloaded beside a crew")
	}
	after, _ := json.Marshal(w)
	if string(before) != string(after) {
		t.Fatal("overflow mutated cargo")
	}
	// Every owned ship is deployed; the build rule can now recycle the chosen
	// ship. Keep the replacement berth empty and use actual resource payment.
	target := -1
	for _, e := range g.Edges {
		if (e.A == w.Harbor || e.B == w.Harbor) && catanExplorerSeaEdge(g, e.ID) && e.ID != f.Positions[0] {
			target = e.ID
			break
		}
	}
	if target < 0 {
		t.Fatal("fixture harbor needs two sea berths")
	}
	next := 1
	for _, e := range g.Edges {
		if next >= 3 {
			break
		}
		if catanExplorerSeaEdge(g, e.ID) && e.ID != target && e.ID != f.Positions[0] {
			f.Positions[next] = e.ID
			f.Turn.Ships[next] = catanExplorerShipMove{Remaining: 4}
			next++
		}
	}
	if err := c.endMovement(g, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.beginAction(g, f, 0, 2); err != nil {
		t.Fatal(err)
	}
	g.Players[0].Resources[0], g.Players[0].Resources[2] = 1, 1
	g.Bank[0], g.Bank[2] = 18, 18
	if err := c.buildShip(g, f, 0, 2, 0, target); err != nil {
		t.Fatal(err)
	}
	if c.Fish[0] != (catanExplorerCargoLocation{"supply", -1}) || f.Positions[0] != target || sum(g.Players[0].Resources) != 0 || g.Bank[0] != 19 || g.Bank[2] != 19 {
		t.Fatal("ship recycling lost fish or resource payment")
	}
	w.restore(t)
	bad := clone(*w)
	bad.Cargo.Fish = bad.Cargo.Fish[:5]
	if err := bad.Mission.validate(bad.Game, bad.Board, bad.Fleet, bad.Cargo); err == nil {
		t.Fatal("missing physical fish accepted")
	}
	bad = clone(*w)
	bad.Cargo.Fish[0] = shipLoc
	bad.Cargo.Fish[1] = shipLoc
	if err := bad.Mission.validate(bad.Game, bad.Board, bad.Fleet, bad.Cargo); err == nil {
		t.Fatal("two fish in one hold accepted")
	}
}
