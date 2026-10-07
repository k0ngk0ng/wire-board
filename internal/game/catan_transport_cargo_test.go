package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func transportCargoFixture(t *testing.T, n int) (*Catan, *catanTransport) {
	t.Helper()
	g, m := transportBoard(t, n)
	stock := 19
	if n > 4 {
		stock = 24
	}
	g.Bank = []int{stock, stock, stock, stock, stock}
	for p := range g.Players {
		g.Players[p].Resources = make([]int, 5)
	}
	c, err := newCatanTransportPieces(g, m)
	if err != nil {
		t.Fatal(err)
	}
	// Explicit setup fixture: cities obey separation, but this is not a full
	// game's setup (no ordinary settlements/roads or starting production).
	for p := range g.Players {
		placed := false
		for v := range g.Vertices {
			if m.siteAt(v) < 0 && g.canSettlement(p, v, true) {
				g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
				if err = c.placeWagon(g, p, v); err != nil {
					t.Fatal(err)
				}
				placed = true
				break
			}
		}
		if !placed {
			t.Fatal("no setup city")
		}
	}
	return g, c
}
func transportGive(g *Catan, p int, cards []int) { catanMove(g.Bank, g.Players[p].Resources, cards) }
func transportSnapshot(g *Catan, c *catanTransport) string {
	b, _ := json.Marshal([]any{g, c})
	return string(b)
}
func transportRoundTrip(t *testing.T, g *Catan, c *catanTransport) (*Catan, *catanTransport) {
	t.Helper()
	old := struct {
		Board     *Catan
		Transport *catanTransport
	}{g, c}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var next struct {
		Board     *Catan
		Transport *catanTransport
	}
	if err = json.Unmarshal(data, &next); err != nil {
		t.Fatal(err)
	}
	if err = next.Transport.validate(next.Board); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old, next) {
		t.Fatal("state changed on restore")
	}
	return next.Board, next.Transport
}

// Places the wagon beside a plaza only to isolate arrival rules. The load,
// move, payment and repeated-request checks still use actual controller methods.
func transportVisitFixture(t *testing.T, g *Catan, c *catanTransport, p, site int) {
	t.Helper()
	if err := c.beginTurn(g, p, c.TurnSerial+1); err != nil {
		t.Fatal(err)
	}
	path := c.Map.Sites[site].Paths[0]
	c.Wagons[p].Position = g.Edges[path].A
	if err := c.beginTravel(g, p); err != nil {
		t.Fatal(err)
	}
	if _, err := c.move(g, p, c.Sequence, path); err != nil {
		t.Fatal(err)
	}
}

func TestCatanTransportCargoOfficialInventories(t *testing.T) {
	base := catanTransportTokens(false)
	expanded := catanTransportTokens(true)
	if len(base) != 36 || len(expanded) != 54 || !reflect.DeepEqual(base, expanded[:36]) {
		t.Fatal("stable original component IDs")
	}
	for _, extended := range []bool{false, true} {
		counts := map[string]int{}
		for _, token := range catanTransportTokens(extended) {
			counts[token.Origin+":"+token.Cargo]++
		}
		n := 6
		if extended {
			n = 9
		}
		want := map[string]int{"quarry:marble": n, "quarry:sand": n, "glassworks:glass": n, "glassworks:tools": n, "castle:sand": n, "castle:tools": n}
		if !reflect.DeepEqual(counts, want) {
			t.Fatal("printed six fronts/backs", counts)
		}
	}
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			orders := map[string]bool{}
			for range 12 {
				g, c := transportCargoFixture(t, n)
				size := 12
				total := 100
				if n > 4 {
					size = 18
					total = 152
				}
				for i, stack := range c.Stacks {
					if len(stack) != size {
						t.Fatal("shared stack size", i)
					}
				}
				if c.GoldBank != total-5*n || sum(c.Gold) != 5*n {
					t.Fatal("each player starts with five gold")
				}
				if err := c.placeWagon(g, 0, c.Wagons[0].Position); err == nil {
					t.Fatal("placed wagon twice")
				}
				transportRoundTrip(t, g, c)
				orders[fmt.Sprint(c.Stacks)] = true
			}
			if len(orders) < 2 {
				t.Fatal("commodity stacks not shuffled")
			}
		})
	}
}
func TestCatanTransportCargoDeliveryAtEveryLevel(t *testing.T) {
	for level := 0; level <= 4; level++ {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			g, c := transportCargoFixture(t, 4)
			if err := c.beginTurn(g, 0, 1); err != nil {
				t.Fatal(err)
			}
			for next := 0; next < level; next++ {
				cost := catanTransportUpgradeCost(next)
				transportGive(g, 0, cost)
				if err := c.upgrade(g, 0); err != nil {
					t.Fatal(err)
				}
				if sum(g.Players[0].Resources) != 0 {
					t.Fatal("upgrade cost not paid")
				}
			}
			if level == 4 && c.extraPoints(0) != 1 {
				t.Fatal("full upgrade must be worth one VP")
			}
			if err := c.beginTravel(g, 0); err != nil {
				t.Fatal(err)
			}
			if err := c.stop(g, 0, c.Sequence); err != nil {
				t.Fatal(err)
			}
			transportVisitFixture(t, g, c, 0, 0)
			before, _ := c.token(c.Stacks[0][0])
			res, err := c.resolveArrival(g, 0, c.Sequence, false)
			if err != nil || res.Loaded != before.ID || res.Delivered != 0 || res.Gold != 0 {
				t.Fatal("empty wagon draws top token", res, err)
			}
			if len(c.Stacks[0]) != 11 || c.Wagons[0].Cargo != before.ID {
				t.Fatal("cargo not removed from supply")
			}
			dest := -1
			for i := range c.Map.Sites {
				if c.Map.accepts(i, before.Cargo) {
					dest = i
					break
				}
			}
			transportVisitFixture(t, g, c, 0, dest)
			if !c.canDeliver() {
				t.Fatal("matching commodity refused")
			}
			g, c = transportRoundTrip(t, g, c)
			oldGold, oldBank, oldPoints := c.Gold[0], c.GoldBank, c.extraPoints(0)
			origin := catanTransportOriginIndex(c.Map.Sites[dest].Kind)
			next := c.Stacks[origin][0]
			res, err = c.resolveArrival(g, 0, c.Sequence, true)
			if err != nil || res.Delivered != before.ID || res.Loaded != next || res.Gold != level+1 || c.Gold[0] != oldGold+level+1 || c.GoldBank != oldBank-level-1 || c.extraPoints(0) != oldPoints+1 {
				t.Fatal("delivery reward", res, err)
			}
			g, c = transportRoundTrip(t, g, c)
			snapshot := transportSnapshot(g, c)
			if _, err = c.resolveArrival(g, 0, c.Sequence, true); err == nil || transportSnapshot(g, c) != snapshot {
				t.Fatal("replayed delivery")
			}
			if err = c.validate(g); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestCatanTransportCargoWrongDestinationDeclineAndSharedStack(t *testing.T) {
	g, c := transportCargoFixture(t, 6)
	transportVisitFixture(t, g, c, 0, 0)
	first := c.Stacks[0][0]
	if _, err := c.resolveArrival(g, 0, c.Sequence, false); err != nil {
		t.Fatal(err)
	}
	transportVisitFixture(t, g, c, 0, 4) // a different quarry uses the SAME stack.
	snapshot := transportSnapshot(g, c)
	if _, err := c.resolveArrival(g, 0, c.Sequence, true); err == nil || transportSnapshot(g, c) != snapshot {
		t.Fatal("quarry accepted its own cargo")
	}
	res, err := c.resolveArrival(g, 0, c.Sequence, false)
	if err != nil || res.Loaded != 0 || res.Delivered != 0 || c.Wagons[0].Cargo != first || len(c.Stacks[0]) != 17 {
		t.Fatal("full wagon swapped cargo", res, err)
	}
	transportVisitFixture(t, g, c, 1, 5) // third quarry, empty second player's wagon.
	second := c.Stacks[0][0]
	if _, err = c.resolveArrival(g, 1, c.Sequence, false); err != nil || c.Wagons[1].Cargo != second || len(c.Stacks[0]) != 16 {
		t.Fatal("shared quarry stack", err)
	}
	cargo, _ := c.token(first)
	dest := -1
	for i := range c.Map.Sites {
		if c.Map.accepts(i, cargo.Cargo) {
			dest = i
			break
		}
	}
	transportVisitFixture(t, g, c, 0, dest)
	points := c.extraPoints(0)
	res, err = c.resolveArrival(g, 0, c.Sequence, false)
	if err != nil || res.Delivered != 0 || res.Loaded != 0 || c.Wagons[0].Cargo != first || c.extraPoints(0) != points {
		t.Fatal("delivery is optional", res, err)
	}
}
func TestCatanTransportCargoStageAndRequestGuards(t *testing.T) {
	g, c := transportCargoFixture(t, 3)
	if err := c.beginTurn(g, 0, 1); err != nil {
		t.Fatal(err)
	}
	for _, p := range []int{-1, 1, 3} {
		snapshot := transportSnapshot(g, c)
		if err := c.beginTravel(g, p); err == nil || transportSnapshot(g, c) != snapshot {
			t.Fatal("other player started travel")
		}
	}
	if err := c.beginTurn(g, 1, 2); err == nil {
		t.Fatal("skipped unfinished turn")
	}
	if err := c.beginTravel(g, 0); err != nil {
		t.Fatal(err)
	}
	if err := c.upgrade(g, 0); err == nil {
		t.Fatal("upgrade after action phase")
	}
	if err := c.buyResource(g, 0, 0); err == nil {
		t.Fatal("trade during movement")
	}
	if err := c.beginTravel(g, 0); err == nil {
		t.Fatal("restarted same travel")
	}
	if err := c.stop(g, 0, c.Sequence); err != nil {
		t.Fatal(err)
	}
	if err := c.beginTurn(g, 1, 1); err == nil {
		t.Fatal("replayed turn serial")
	}
	transportVisitFixture(t, g, c, 1, 0)
	if err := c.beginTurn(g, 2, c.TurnSerial+1); err == nil {
		t.Fatal("skipped arrival choice")
	}
	for _, args := range []struct {
		p int
		s uint64
	}{{0, c.Sequence}, {-1, c.Sequence}, {1, c.Sequence - 1}, {1, c.Sequence + 1}} {
		snapshot := transportSnapshot(g, c)
		if _, err := c.resolveArrival(g, args.p, args.s, false); err == nil || transportSnapshot(g, c) != snapshot {
			t.Fatal("wrong/stale arrival responder", args)
		}
	}
}

func TestCatanTransportCargoGoldTrade(t *testing.T) {
	for _, rate := range []int{2, 3, 4} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			g, c := transportCargoFixture(t, 4)
			if err := c.beginTurn(g, 0, 1); err != nil {
				t.Fatal(err)
			}
			// Keep the official port inventory; move fixture buildings off ports.
			for _, port := range g.Ports {
				e := g.Edges[port.Edge]
				g.Vertices[e.A].Owner, g.Vertices[e.A].Level = -1, 0
				g.Vertices[e.B].Owner, g.Vertices[e.B].Level = -1, 0
			}
			if rate < 4 {
				for _, port := range g.Ports {
					if port.Resource == 0 && rate == 2 || port.Resource == -1 && rate == 3 {
						v := g.Edges[port.Edge].A
						g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
						break
					}
				}
			}
			if g.rates(0)[0] != rate {
				t.Fatal("trade fixture rate", g.rates(0))
			}
			transportGive(g, 0, []int{rate, 0, 0, 0, 0})
			if err := c.sellResource(g, 0, 0); err != nil || c.Gold[0] != 6 || sum(g.Players[0].Resources) != 0 {
				t.Fatal("resource for gold", err)
			}
			for color := range 2 {
				if err := c.buyResource(g, 0, color); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := transportSnapshot(g, c)
			if err := c.buyResource(g, 0, 2); err == nil || transportSnapshot(g, c) != snapshot {
				t.Fatal("third gold purchase")
			}
			if c.Gold[0] != 2 || c.Bought != 2 {
				t.Fatal("gold purchase payment")
			}
			if err := c.beginTravel(g, 0); err != nil {
				t.Fatal(err)
			}
			if err := c.stop(g, 0, c.Sequence); err != nil {
				t.Fatal(err)
			}
			if err := c.beginTurn(g, 0, 2); err != nil {
				t.Fatal(err)
			}
			if err := c.buyResource(g, 0, 2); err != nil || c.Bought != 1 {
				t.Fatal("new action phase must reset purchase cap", err)
			}
		})
	}
}

func TestCatanTransportCargoPrivacyAndSupplyCorruption(t *testing.T) {
	g, c := transportCargoFixture(t, 4)
	transportVisitFixture(t, g, c, 0, 0)
	if _, err := c.resolveArrival(g, 0, c.Sequence, false); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(c.publicView())
	c.Stacks[0][0], c.Stacks[0][1] = c.Stacks[0][1], c.Stacks[0][0]
	after, _ := json.Marshal(c.publicView())
	if string(before) != string(after) {
		t.Fatal("public view leaks hidden supply order")
	}
	view := c.publicView()
	token, _ := c.token(c.Wagons[0].Cargo)
	if view.Wagons[0].Cargo == nil || *view.Wagons[0].Cargo != token || view.Supply[0] != 11 {
		t.Fatal("face-up cargo must be visible")
	}
	view.Travel.Position = -1
	view.Wagons[0].Cargo.Cargo = "tampered"
	if c.Travel.Position < 0 {
		t.Fatal("view aliases game state")
	}
	for name, corrupt := range map[string]func(*catanTransport){
		"duplicate":              func(c *catanTransport) { c.Stacks[0][0] = c.Stacks[0][1] },
		"missing":                func(c *catanTransport) { c.Stacks[1] = c.Stacks[1][1:] },
		"wrong-origin":           func(c *catanTransport) { c.Stacks[0][0], c.Stacks[1][0] = c.Stacks[1][0], c.Stacks[0][0] },
		"double-load":            func(c *catanTransport) { c.Wagons[1].Cargo = c.Wagons[0].Cargo },
		"phantom-gold":           func(c *catanTransport) { c.Gold[0]++ },
		"reopen-arrival":         func(c *catanTransport) { c.ArrivalResolved = false },
		"missing-arrival-record": func(c *catanTransport) { c.LastArrival = nil },
		"wrong-position":         func(c *catanTransport) { c.Wagons[0].Position++ },
		"wrong-level":            func(c *catanTransport) { c.Wagons[0].Level++ },
	} {
		t.Run(name, func(t *testing.T) {
			data, _ := json.Marshal(c)
			var bad catanTransport
			json.Unmarshal(data, &bad)
			corrupt(&bad)
			if bad.validate(g) == nil {
				t.Fatal("corrupt inventory accepted")
			}
		})
	}
}

// Shortest public path, without going through another plaza (arrival ends the
// movement). No hidden token IDs/order or other players' resource hands used.
func transportPublicPath(g *Catan, m *catanTransportMap, from, to int) []int {
	queue := []int{from}
	parents := map[int][2]int{from: {-1, -1}}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		if v == to {
			break
		}
		for _, e := range g.Edges {
			next := -1
			if e.A == v {
				next = e.B
			} else if e.B == v {
				next = e.A
			}
			if next < 0 || next != to && m.siteAt(next) >= 0 {
				continue
			}
			if _, seen := parents[next]; seen {
				continue
			}
			parents[next] = [2]int{v, e.ID}
			queue = append(queue, next)
		}
	}
	if _, ok := parents[to]; !ok {
		return nil
	}
	path := []int{}
	for at := to; at != from; at = parents[at][0] {
		path = append(path, parents[at][1])
	}
	slices.Reverse(path)
	return path
}
func transportCheckInventory(t *testing.T, g *Catan, c *catanTransport) {
	t.Helper()
	if err := c.validate(g); err != nil {
		t.Fatal(err)
	}
	ids := []int{}
	for _, stack := range c.Stacks {
		ids = append(ids, stack...)
	}
	for p, w := range c.Wagons {
		ids = append(ids, w.Delivered...)
		if w.Cargo != 0 {
			ids = append(ids, w.Cargo)
		}
		want := len(w.Delivered)
		if w.Level == 4 {
			want++
		}
		if c.extraPoints(p) != want {
			t.Fatal("scenario score drift")
		}
	}
	slices.Sort(ids)
	total := 36
	if len(g.Players) > 4 {
		total = 54
	}
	if len(ids) != total {
		t.Fatal("physical token count", len(ids))
	}
	for i, id := range ids {
		if id != i+1 {
			t.Fatal("token lost/duplicated", ids)
		}
	}
	if c.GoldBank+sum(c.Gold) != c.Map.Gold {
		t.Fatal("gold conservation")
	}
}
func TestCatanTransportCargoCompleteInventoryJourneys(t *testing.T) {
	// Complete transport-only inventory runs, NOT complete CATAN games:
	// no dice, construction, development cards, or 13-point victory yet.
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			g, c := transportCargoFixture(t, n)
			operations, deliveries := 0, 0
			total := 36
			if n > 4 {
				total = 54
			}
			for turn := 0; turn < 8000 && deliveries < total; turn++ {
				p := turn % n
				if err := c.beginTurn(g, p, c.TurnSerial+1); err != nil {
					t.Fatal(err)
				}
				if err := c.beginTravel(g, p); err != nil {
					t.Fatal(err)
				}
				view := c.publicView()
				w := view.Wagons[p]
				var path []int
				for i, site := range c.Map.Sites {
					if site.Center == w.Position {
						continue
					}
					if w.Cargo != nil {
						if !c.Map.accepts(i, w.Cargo.Cargo) {
							continue
						}
					} else if view.Supply[catanTransportOriginIndex(site.Kind)] == 0 {
						continue
					}
					candidate := transportPublicPath(g, c.Map, w.Position, site.Center)
					if len(candidate) > 0 && (path == nil || len(candidate) < len(path)) {
						path = candidate
					}
				}
				for _, edge := range path {
					if _, err := c.Travel.quote(g, c.Map, c.Barbarians, c.Gold, edge); err != nil {
						break
					}
					if _, err := c.move(g, p, c.Sequence, edge); err != nil {
						t.Fatal(err)
					}
					operations++
					transportCheckInventory(t, g, c)
					if operations%17 == 0 {
						g, c = transportRoundTrip(t, g, c)
					}
					if c.Travel.Ended {
						break
					}
				}
				if c.Travel.Arrived >= 0 {
					res, err := c.resolveArrival(g, p, c.Sequence, c.canDeliver())
					if err != nil {
						t.Fatal(err)
					}
					if res.Delivered != 0 {
						deliveries++
					}
				} else if !c.Travel.Ended {
					if err := c.stop(g, p, c.Sequence); err != nil {
						t.Fatal(err)
					}
				}
				transportCheckInventory(t, g, c)
			}
			if deliveries != total {
				t.Fatal("transport stock not exhausted", deliveries, total)
			}
			for _, stack := range c.Stacks {
				if len(stack) != 0 {
					t.Fatal("undelivered stock")
				}
			}
			for _, w := range c.Wagons {
				if w.Cargo != 0 {
					t.Fatal("undelivered wagon cargo")
				}
			}
			points := 0
			for p := range c.Wagons {
				points += c.extraPoints(p)
			}
			if points != total {
				t.Fatal("delivered score", points)
			}
			// Return to an exhausted source: no reshuffle, new token or VP.
			transportVisitFixture(t, g, c, 0, 0)
			old := c.extraPoints(0)
			res, err := c.resolveArrival(g, 0, c.Sequence, false)
			if err != nil || res.Loaded != 0 || res.Delivered != 0 || c.extraPoints(0) != old {
				t.Fatal("exhausted stack regenerated token", res, err)
			}
			transportCheckInventory(t, g, c)
			t.Logf("%d players: %d movement steps, %d deliveries, %d retained VPs", n, operations, deliveries, points)
		})
	}
}

func TestCatanTransportCargoGoldLedgerOverflowIsAtomic(t *testing.T) {
	g, c := transportCargoFixture(t, 4)
	transportVisitFixture(t, g, c, 0, 0)
	if _, err := c.resolveArrival(g, 0, c.Sequence, false); err != nil {
		t.Fatal(err)
	}
	token, _ := c.token(c.Wagons[0].Cargo)
	site := -1
	for i := range c.Map.Sites {
		if c.Map.accepts(i, token.Cargo) {
			site = i
			break
		}
	}
	transportVisitFixture(t, g, c, 0, site)
	// A valid exhausted ledger at the safe integer cap must reject atomically.
	c.Gold[1] += c.GoldBank + catanGoldLedgerLimit
	c.GoldIssued = catanGoldLedgerLimit
	c.GoldBank = 0
	if err := c.validate(g); err != nil {
		t.Fatal(err)
	}
	before := transportSnapshot(g, c)
	if _, err := c.resolveArrival(g, 0, c.Sequence, true); err == nil || transportSnapshot(g, c) != before {
		t.Fatal("shortage must not lose cargo or partly award VP")
	}
	if !c.canDeliver() || c.ArrivalResolved {
		t.Fatal("pending choice was lost")
	}
}

func TestCatanTransportCargoUnaffordableAndExhaustedPaymentsAreAtomic(t *testing.T) {
	g, c := transportCargoFixture(t, 4)
	if err := c.beginTurn(g, 0, 1); err != nil {
		t.Fatal(err)
	}
	before := transportSnapshot(g, c)
	if err := c.upgrade(g, 0); err == nil || transportSnapshot(g, c) != before {
		t.Fatal("unaffordable upgrade changed inventory")
	}
	for level := 0; level < 4; level++ {
		transportGive(g, 0, catanTransportUpgradeCost(level))
		if err := c.upgrade(g, 0); err != nil {
			t.Fatal(err)
		}
	}
	before = transportSnapshot(g, c)
	if err := c.upgrade(g, 0); err == nil || transportSnapshot(g, c) != before || c.extraPoints(0) != 1 {
		t.Fatal("fifth upgrade/extra VP")
	}
	transportGive(g, 1, []int{19, 0, 0, 0, 0})
	before = transportSnapshot(g, c)
	if err := c.buyResource(g, 0, 0); err == nil || transportSnapshot(g, c) != before {
		t.Fatal("empty bank purchase changed gold or cap")
	}
	c.Gold[1] += c.Gold[0]
	c.Gold[0] = 0
	before = transportSnapshot(g, c)
	if err := c.buyResource(g, 0, 1); err == nil || transportSnapshot(g, c) != before {
		t.Fatal("purchase without gold")
	}
}
