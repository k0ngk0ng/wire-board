package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func mapGame(t *testing.T, id string, n int) *State {
	t.Helper()
	s, e := NewRailMap(id, n)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func mapPlaying(t *testing.T, id string, n int) *State {
	s := mapGame(t, id, n)
	s.AutoChooseRailSetup()
	return s
}
func railMapInvariant(t *testing.T, s *State) {
	t.Helper()
	g := s.Rail
	counts := make([]int, 9)
	for _, pile := range [][]int{g.Deck, g.Discard, g.Face} {
		for _, c := range pile {
			if c >= 0 {
				counts[c]++
			}
		}
	}
	for i, p := range g.Players {
		for c, n := range p.Hand {
			counts[c] += n
		}
		used := 0
		for _, r := range g.data().Routes {
			if owner, ok := g.Owners[r.ID]; ok && owner == i {
				used += r.Length + r.Mountain
			}
		}
		if used+p.Trains != g.info().Trains {
			t.Fatalf("train conservation %s: %d+%d", g.Map, used, p.Trains)
		}
	}
	if x := g.Tunnel; x != nil {
		for c, n := range x.Base {
			counts[c] += n
		}
		for _, c := range x.Revealed {
			counts[c]++
		}
	}
	for c, n := range counts {
		want := 12
		if c == 8 {
			want = 14
		}
		if n != want {
			t.Fatalf("%s card %d: %d want %d", g.Map, c, n, want)
		}
	}
}
func TestRailAllMapSetupAndBots(t *testing.T) {
	for _, info := range RailMapList() {
		for _, n := range []int{info.MinPlayers, info.MaxPlayers} {
			t.Run(fmt.Sprintf("%s/%d", info.ID, n), func(t *testing.T) {
				s := mapGame(t, info.ID, n)
				g := s.Rail
				ids := map[int]bool{}
				longs := 0
				for _, cards := range g.SetupPending {
					if len(cards) != info.SetupTickets {
						t.Fatal("setup count")
					}
					lc := 0
					for _, c := range cards {
						if ids[c.ID] {
							t.Fatal("duplicate physical ticket")
						}
						ids[c.ID] = true
						if c.Long {
							lc++
							longs++
						}
					}
					if info.LongTickets && lc != 1 {
						t.Fatal("long distribution")
					}
				}
				for _, c := range g.TicketDeck {
					if c.Long || ids[c.ID] {
						t.Fatal("invalid remaining deck")
					}
					ids[c.ID] = true
				}
				if info.LongTickets && longs != n {
					t.Fatal("missing long cards")
				}
				beforeDeck := len(g.TicketDeck)
				s.AutoChooseRailSetup()
				expected := beforeDeck
				if info.InitialReturn {
					expected += n * (info.SetupTickets - 2)
				}
				if len(g.TicketDeck) != expected {
					t.Fatal("initial discard rule")
				}
				for step := 0; step < 1600 && !s.Finished; step++ {
					railMapInvariant(t, s)
					before, _ := json.Marshal(s)
					a, e := s.BotAction(s.Turn)
					after, _ := json.Marshal(s)
					if e != nil {
						t.Fatalf("step %d phase %s: %v", step, s.Phase, e)
					}
					if string(before) != string(after) {
						t.Fatal("bot changed live state")
					}
					if e = s.Apply(s.Turn, a); e != nil {
						t.Fatalf("illegal bot action %+v: %v", a, e)
					}
					if step == 50 {
						raw, _ := json.Marshal(s)
						var resumed State
						if e = json.Unmarshal(raw, &resumed); e != nil {
							t.Fatal(e)
						}
						s = &resumed
						g = s.Rail
					}
				}
				railMapInvariant(t, s)
				if !s.Finished || len(s.Winners) == 0 {
					t.Fatalf("bot game did not finish: round %d", s.Round)
				}
			})
		}
	}
}
func TestRailMapDataAndLegacyIsolation(t *testing.T) {
	counts := map[string][3]int{"usa": {36, 100, 30}, "europe": {47, 101, 46}, "india": {39, 108, 58}, "switzerland": {55, 88, 46}, "nordiccountries": {39, 81, 46}, "legendaryasia": {39, 100, 36}}
	for _, info := range RailMapList() {
		s := mapGame(t, info.ID, info.MinPlayers)
		d := s.Rail.data()
		want := counts[info.ID]
		if len(d.Cities) != want[0] || len(d.Routes) != want[1] || len(d.Tickets) != want[2] {
			t.Fatalf("%s data count: %d %d %d", info.ID, len(d.Cities), len(d.Routes), len(d.Tickets))
		}
		seen := map[int]bool{}
		for _, r := range d.Routes {
			if r.ID <= 0 || seen[r.ID] || r.A < 0 || r.B >= len(d.Cities) || r.Length != len(r.Segments) || r.Length > 9 || r.Length < 1 {
				t.Fatalf("bad route %+v", r)
			}
			seen[r.ID] = true
		}
		snapshot := clone(*d)
		_ = mapGame(t, info.ID, info.MaxPlayers)
		if !reflect.DeepEqual(snapshot, *d) {
			t.Fatal("map data mutated")
		}
	}
	legacy := mapPlaying(t, "usa", 2)
	legacy.Rail.Map = ""
	if legacy.Rail.info().ID != "usa" {
		t.Fatal("legacy map")
	}
	if _, e := NewRailMap("switzerland", 4); e == nil {
		t.Fatal("invalid player limit")
	}
	if _, e := NewRailMap("invalid", 2); e == nil {
		t.Fatal("unknown map")
	}
}
func TestRailSpecialPayments(t *testing.T) {
	full := []int{20, 20, 20, 20, 20, 20, 20, 20, 20}
	for _, id := range []string{"switzerland", "nordiccountries"} {
		g := mapPlaying(t, id, 2).Rail
		r := Route{Length: 2, Color: 2}
		pay := make([]int, 9)
		pay[2] = 1
		pay[8] = 1
		if g.validPayment(r, full, 2, pay) {
			t.Fatal(id, "wild on ordinary route")
		}
		r.Tunnel = true
		if !g.validPayment(r, full, 2, pay) {
			t.Fatal(id, "wild tunnel")
		}
	}
	g := mapPlaying(t, "nordiccountries", 2).Rail
	r := Route{Length: 3, Color: -1, Ferry: 2, Substitute: 3}
	p := make([]int, 9)
	p[2] = 1
	p[8] = 1
	p[5] = 3
	if !g.validPayment(r, full, 2, p) {
		t.Fatal("ferry substitution")
	}
	p[5] = 2
	if g.validPayment(r, full, 2, p) {
		t.Fatal("short ferry substitution")
	}
	r = Route{Length: 9, Color: -1, Substitute: 4}
	p = make([]int, 9)
	p[2] = 8
	p[5] = 3
	p[8] = 1
	if !g.validPayment(r, full, 2, p) {
		t.Fatal("9-space substitution")
	}
	p[5] = 0
	if g.validPayment(r, full, 2, p) {
		t.Fatal("wild cannot directly replace Nordic color")
	}
	for _, info := range RailMapList() {
		g := mapPlaying(t, info.ID, info.MinPlayers).Rail
		for _, r := range g.data().Routes {
			g.Players[0].Hand = full
			for _, pay := range g.paymentOptions(0, r) {
				if !g.validPayment(r, full, pay.Color, pay.Tokens) {
					t.Fatalf("invalid suggested payment %s %+v %+v", info.ID, r, pay)
				}
			}
		}
	}
}
func TestRailWildDrawAndDoubleThresholds(t *testing.T) {
	for _, id := range []string{"europe", "switzerland", "nordiccountries"} {
		s := mapPlaying(t, id, 2)
		g := s.Rail
		g.Face = []int{8, 8, 0, 1, 2}
		g.Deck = []int{0, 1, 2}
		g.Discard = nil
		if e := s.Apply(0, Action{Type: "draw", Slot: 0}); e != nil {
			t.Fatal(e)
		}
		if id == "europe" {
			if s.Turn != 1 {
				t.Fatal("Europe wild must end turn")
			}
		} else {
			if s.Turn != 0 || s.Phase != "draw" {
				t.Fatal("single wild ended turn")
			}
			if e := s.Apply(0, Action{Type: "draw", Slot: 1}); e != nil || s.Turn != 1 {
				t.Fatal("second visible wild", e)
			}
		}
	}
	for _, info := range RailMapList() {
		for _, n := range []int{2, info.MaxPlayers} {
			g := mapPlaying(t, info.ID, n).Rail
			found := false
			for _, a := range g.data().Routes {
				for _, b := range g.data().Routes {
					if a.ID != b.ID && a.A == b.A && a.B == b.B {
						g.Owners[a.ID] = 0
						if g.routeOpen(b, 1) != (n >= info.DoubleMin) {
							t.Fatal("double threshold", info.ID, n)
						}
						if g.routeOpen(b, 0) {
							t.Fatal("same player double")
						}
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}
	}
}
func TestRailTunnelEscrowResumeAndCancel(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		s := mapPlaying(t, "europe", 2)
		g := s.Rail
		var r Route
		for _, x := range g.data().Routes {
			if x.Tunnel && x.Length <= 4 {
				r = x
				break
			}
		}
		c := max(0, r.Color)
		g.Players[0].Hand = make([]int, 9)
		g.Players[0].Hand[c] = r.Length + 3
		g.Face = nil
		g.Deck = nil
		g.Discard = []int{c, 8, (c + 1) % 8}
		before := sum(g.Players[0].Hand)
		if e := s.Apply(0, Action{Type: "claim", Route: r.ID, Color: c}); e != nil {
			t.Fatal(e)
		}
		if g.Tunnel == nil || g.Tunnel.Extra != 2 || len(g.Tunnel.Revealed) != 3 || len(g.Discard) != 0 || sum(g.Players[0].Hand) != before-r.Length {
			t.Fatal("escrow/reveal", g.Tunnel, g.Discard)
		}
		raw, _ := json.Marshal(s)
		var resumed State
		if e := json.Unmarshal(raw, &resumed); e != nil {
			t.Fatal(e)
		}
		s = &resumed
		g = s.Rail
		if e := s.Apply(1, Action{Type: "tunnel_cancel"}); e == nil {
			t.Fatal("opponent resolved tunnel")
		}
		typ := "tunnel_pay"
		if cancel {
			typ = "tunnel_cancel"
		}
		if e := s.Apply(0, Action{Type: typ}); e != nil {
			t.Fatal(e)
		}
		if g.Tunnel != nil || s.Turn != 1 {
			t.Fatal("tunnel turn completion")
		}
		_, claimed := g.Owners[r.ID]
		if claimed == cancel {
			t.Fatal("tunnel route ownership")
		}
		want := before - r.Length - 2
		if cancel {
			want = before
		}
		if sum(g.Players[0].Hand) != want {
			t.Fatal("tunnel refund/payment")
		}
	}
	s := mapPlaying(t, "europe", 2)
	g := s.Rail
	var r Route
	for _, x := range g.data().Routes {
		if x.Tunnel {
			r = x
			break
		}
	}
	c := max(0, r.Color)
	g.Players[0].Hand = make([]int, 9)
	g.Players[0].Hand[8] = r.Length + 2
	g.Deck = []int{c, c, 8}
	g.Discard = nil
	if e := s.Apply(0, Action{Type: "claim", Route: r.ID, Color: c, Wild: r.Length}); e != nil {
		t.Fatal(e)
	}
	if g.Tunnel.Extra != 1 || !g.Tunnel.WildOnly {
		t.Fatal("all-wild tunnel check")
	}
	g.Players[0].Hand[c] = 3
	if e := s.Apply(0, Action{Type: "tunnel_pay", Wild: 0}); e == nil {
		t.Fatal("all-wild surcharge accepted color")
	}
	if e := s.Apply(0, Action{Type: "tunnel_pay", Wild: 1}); e != nil {
		t.Fatal(e)
	}
}
func TestRailStationConsistentBorrowAndPrivacy(t *testing.T) {
	s := mapPlaying(t, "europe", 2)
	g := s.Rail
	r1 := g.data().Routes[0]
	var r2 Route
	for _, r := range g.data().Routes {
		if r.A == r1.A && r.B != r1.B {
			r2 = r
			break
		}
		if r.B == r1.A && r.A != r1.B {
			r2 = r
			r2.A, r2.B = r.B, r.A
			break
		}
	}
	if r2.ID == 0 {
		t.Fatal("fixture requires two incident edges")
	}
	g.Owners = map[int]int{r1.ID: 1, r2.ID: 1}
	g.Players[0].Stations = []int{r1.A}
	g.Players[0].Tickets = []Ticket{{ID: 900, A: r1.A, B: r1.B, Points: 10}, {ID: 901, A: r1.A, B: r2.B, Points: 4}}
	tickets, score, complete, borrow := g.evaluateTickets(0)
	if score != 6 || complete != 1 || !tickets[0].Complete || tickets[1].Complete || len(borrow) != 1 || borrow[0] != r1.ID {
		t.Fatal("station chose inconsistent routes", tickets, score, borrow)
	}
	if longestRailRoutes(g.data().Routes, g.Owners, 0) != 0 {
		t.Fatal("borrowed route counted for longest")
	}
	for _, viewer := range []int{1, -1} {
		view := s.View(viewer)["rail"].(map[string]any)
		p := view["players"].([]any)[0].(map[string]any)
		for _, key := range []string{"tickets", "hand", "stationRoutes", "completed", "mandalaCount"} {
			if _, ok := p[key]; ok {
				t.Fatal("private key leaked", viewer, key)
			}
		}
		if viewer < 0 && view["payments"] != nil {
			t.Fatal("spectator payments")
		}
	}
	g.Players[0].Hand = []int{3, 0, 0, 0, 0, 0, 0, 0, 0}
	if e := s.Apply(0, Action{Type: "station", Vertex: r1.A, Color: 0}); e == nil {
		t.Fatal("station occupied")
	}
	s.railFinish()
	if g.Players[0].StationScore != 8 {
		t.Fatal("unused stations")
	}
}
func TestRailSwissBorderAndAlternativeTickets(t *testing.T) {
	g := mapPlaying(t, "switzerland", 2).Rail
	var a, b Route
	country := -1
	for _, c := range g.data().Cities {
		if c.Kind != "country" {
			continue
		}
		for _, x := range g.data().Routes {
			for _, y := range g.data().Routes {
				cx, cy := g.data().Cities[x.B], g.data().Cities[y.B]
				if cx.Kind == "border" && cy.Kind == "border" && cx.Country == c.Country && cy.Country == c.Country && x.B != y.B && x.A != y.A {
					a, b, country = x, y, c.ID
					break
				}
			}
			if country >= 0 {
				break
			}
		}
		if country >= 0 {
			break
		}
	}
	if country < 0 {
		t.Fatal("no border fixture")
	}
	g.Owners = map[int]int{a.ID: 0, b.ID: 0}
	if g.connected(0, a.A, b.A) {
		t.Fatal("different country entrances auto-connected")
	}
	if !g.connected(0, a.A, country) {
		t.Fatal("country endpoint incomplete")
	}
	ticket := Ticket{A: a.A, B: country, Points: 3, Options: []TicketOption{{To: country, Points: 12}, {To: b.A, Points: 3}}}
	out := g.ticketIn(g.components(0, nil), ticket)
	if !out.Complete || out.Value != 12 {
		t.Fatal(out)
	}
	g.Owners = map[int]int{}
	out = g.ticketIn(g.components(0, nil), ticket)
	if out.Complete || out.Value != -3 {
		t.Fatal(out)
	}
}
func TestRailIndiaMandalaAndAsiaMountain(t *testing.T) {
	g := mapPlaying(t, "india", 2).Rail
	// Find a cycle in the factual board, claim it, then remove one edge.
	var cycle []Route
	var dfs func(int, int, map[int]bool, []Route) bool
	dfs = func(at, start int, seen map[int]bool, path []Route) bool {
		if len(path) > 6 {
			return false
		}
		for _, r := range g.data().Routes {
			next := -1
			if r.A == at {
				next = r.B
			} else if r.B == at {
				next = r.A
			}
			if next < 0 {
				continue
			}
			if next == start && len(path) >= 2 {
				cycle = append(append([]Route{}, path...), r)
				return true
			}
			if seen[next] {
				continue
			}
			seen[next] = true
			if dfs(next, start, seen, append(path, r)) {
				return true
			}
			delete(seen, next)
		}
		return false
	}
	if !dfs(0, 0, map[int]bool{0: true}, nil) {
		t.Fatal("no cycle")
	}
	for _, r := range cycle {
		g.Owners[r.ID] = 0
	}
	groups := g.mandalaComponents(0)
	if groups[cycle[0].A] != groups[cycle[0].B] {
		t.Fatal("cycle not mandala")
	}
	delete(g.Owners, cycle[len(cycle)-1].ID)
	groups = g.mandalaComponents(0)
	if groups[cycle[0].A] == groups[cycle[0].B] {
		t.Fatal("single path counted as mandala")
	}
	s := mapPlaying(t, "legendaryasia", 2)
	g = s.Rail
	var r Route
	for _, x := range g.data().Routes {
		if x.Mountain > 0 {
			r = x
			break
		}
	}
	c := max(0, r.Color)
	g.Players[0].Hand[c] = r.Length
	g.Players[0].Hand[8] = r.Ferry
	g.Players[0].Trains = r.Length + r.Mountain - 1
	if e := s.Apply(0, Action{Type: "claim", Route: r.ID, Color: c, Wild: r.Ferry}); e == nil {
		t.Fatal("missing mountain trains accepted")
	}
	g.Players[0].Trains = g.info().Trains
	if e := s.Apply(0, Action{Type: "claim", Route: r.ID, Color: c, Wild: r.Ferry}); e != nil {
		t.Fatal(e)
	}
	p := g.Players[0]
	if p.Trains != 45-r.Length-r.Mountain || p.RouteScore != railRoutePoints(r.Length)+r.Mountain*2 || p.MountainRoutes != 1 || g.largestNetwork(0) != 2 {
		t.Fatal("mountain accounting", p)
	}
}

func TestRailMapBonusesUseTheirOwnMetric(t *testing.T) {
	t.Run("Nordic counts tickets rather than longest", func(t *testing.T) {
		s := mapPlaying(t, "nordiccountries", 2)
		g := s.Rail
		var short, long Route
		for _, r := range g.data().Routes {
			if r.Length == 1 {
				short = r
			}
			if r.Length == 9 {
				long = r
			}
		}
		g.Owners = map[int]int{short.ID: 1, long.ID: 0}
		g.Players[0].Tickets = []Ticket{{ID: 1, A: long.A, B: long.B, Points: 1}}
		g.Players[1].Tickets = []Ticket{{ID: 2, A: short.A, B: short.B, Points: 1}, {ID: 3, A: short.A, B: short.B, Points: 1}}
		s.railFinish()
		if g.Players[0].Bonus != 0 || g.Players[1].Bonus != 10 {
			t.Fatal("wrong Nordic bonus")
		}
		g.Players[0].Tickets = nil
		g.Players[1].Tickets = nil
		s.railFinish()
		if g.Players[0].Bonus != 10 || g.Players[1].Bonus != 10 {
			t.Fatal("zero completed tickets still share Nordic bonus")
		}
	})
	t.Run("Asia counts connected cities rather than train length", func(t *testing.T) {
		s := mapPlaying(t, "legendaryasia", 2)
		g := s.Rail
		var long, a, b Route
		for _, r := range g.data().Routes {
			if r.Length >= 6 {
				long = r
				break
			}
		}
		for _, r := range g.data().Routes {
			for _, other := range g.data().Routes {
				if r.ID != long.ID && other.ID != long.ID && r.A == other.A && r.B != other.B && r.Length+other.Length < long.Length {
					a, b = r, other
					break
				}
			}
			if a.ID != 0 {
				break
			}
		}
		if a.ID == 0 {
			t.Fatal("network fixture")
		}
		g.Owners = map[int]int{long.ID: 0, a.ID: 1, b.ID: 1}
		s.railFinish()
		if g.Players[0].Bonus != 0 || g.Players[1].Bonus != 10 || g.Players[1].Network != 3 {
			t.Fatal("wrong Asian Explorer award")
		}
	})
	t.Run("Europe breaks ties using unused stations", func(t *testing.T) {
		s := mapPlaying(t, "europe", 2)
		g := s.Rail
		g.Players[0].Tickets = nil
		g.Players[1].Tickets = nil
		g.Players[0].Stations = []int{0}
		g.Players[0].RouteScore = 4
		s.railFinish()
		if g.Players[0].Score != g.Players[1].Score || !reflect.DeepEqual(s.Winners, []int{1}) {
			t.Fatal("station tie break", s.Winners)
		}
	})
}
func TestRailBotsOnMapsDoNotReadOpponentsOrDeck(t *testing.T) {
	for _, info := range RailMapList() {
		s := mapPlaying(t, info.ID, info.MinPlayers)
		other := clone(*s)
		g := other.Rail
		for i, j := 0, len(g.Deck)-1; i < j; i, j = i+1, j-1 {
			g.Deck[i], g.Deck[j] = g.Deck[j], g.Deck[i]
		}
		g.Players[1].Hand = []int{4, 3, 2, 1, 0, 0, 0, 0, 0}
		g.Players[1].Tickets = nil
		a, e := s.BotAction(0)
		b, f := other.BotAction(0)
		if e != nil || f != nil || !reflect.DeepEqual(a, b) {
			t.Fatal(info.ID, "hidden information changed decision", a, b, e, f)
		}
	}
}
