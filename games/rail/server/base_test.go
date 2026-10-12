package rail

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func mustGame(t *testing.T, kind string, n int) *State {
	t.Helper()
	s, e := New(kind, n)
	if e != nil {
		t.Fatal(e)
	}
	// Rule fixtures use a fixed seat order; random starts are tested separately.
	return s
}
func apply(t *testing.T, s *State, a Action) {
	t.Helper()
	if e := s.Apply(s.Turn, a); e != nil {
		t.Fatalf("%+v: %v", a, e)
	}
}
func reject(t *testing.T, s *State, a Action) {
	t.Helper()
	c := clone(*s)
	if e := c.Apply(c.Turn, a); e == nil {
		t.Fatalf("accepted invalid action %+v", a)
	}
}
func routeID(t *testing.T, a, b string) int {
	t.Helper()
	d := MapData()
	for _, r := range d.Routes {
		if (d.Cities[r.A].Name == a && d.Cities[r.B].Name == b) || (d.Cities[r.A].Name == b && d.Cities[r.B].Name == a) {
			return r.ID
		}
	}
	t.Fatal("missing route", a, b)
	return 0
}
func TestRailInitialTicketSelectionAndPrivacy(t *testing.T) {
	s := mustGame(t, "rail", 2)
	if sum(s.Rail.Players[0].Hand) != 4 || s.Phase != "tickets" {
		t.Fatal("setup")
	}
	pending := s.Rail.SetupPending[1]
	if err := s.Apply(1, Action{Type: "keep", Keep: []int{pending[0].ID}}); err == nil {
		t.Fatal("accepted too few")
	}
	if err := s.Apply(1, Action{Type: "keep", Keep: []int{pending[0].ID, pending[0].ID}}); err == nil {
		t.Fatal("duplicate accepted")
	}
	if err := s.Apply(1, Action{Type: "keep", Keep: []int{pending[0].ID, pending[1].ID}}); err != nil {
		t.Fatal(err)
	}
	if !s.Rail.Setup {
		t.Fatal("setup ended before everyone selected")
	}
	v := s.View(1)["rail"].(map[string]any)
	for _, k := range []string{"deck", "discard", "ticketDeck", "pending", "setupPending"} {
		if _, ok := v[k]; ok {
			t.Fatal("leak", k)
		}
	}
	op := v["players"].([]any)[0].(map[string]any)
	if _, ok := op["hand"]; ok {
		t.Fatal("hand leaked")
	}
	if _, ok := op["tickets"]; ok {
		t.Fatal("tickets leaked")
	}
	pending = s.Rail.SetupPending[0]
	apply(t, s, Action{Type: "keep", Keep: []int{pending[0].ID, pending[1].ID}})
	if s.Rail.Setup || s.Turn != 0 || s.Phase != "turn" {
		t.Fatal("setup completion")
	}
	railInvariant(t, s)
}
func playingRail(t *testing.T, n int) *State {
	s := mustGame(t, "rail", n)
	s.AutoChooseRailSetup()
	return s
}
func TestRailDrawWildRules(t *testing.T) {
	s := playingRail(t, 2)
	g := s.Rail
	g.Face = []int{8, 1, 2, 3, 4}
	apply(t, s, Action{Type: "draw", Slot: 0})
	if s.Turn != 1 {
		t.Fatal("face-up locomotive must end turn")
	}
	g.Deck = append([]int{8, 0}, g.Deck...)
	apply(t, s, Action{Type: "draw", Slot: -1})
	if s.Turn != 1 || g.Drawn != 1 {
		t.Fatal("blind locomotive uses only one draw")
	}
	g.Face = []int{8, 1, 2, 3, 4}
	reject(t, s, Action{Type: "draw", Slot: 0})
	reject(t, s, Action{Type: "tickets"})
	apply(t, s, Action{Type: "draw", Slot: 1})
	if s.Turn != 0 {
		t.Fatal("second draw must end turn")
	}
}
func TestRailRefillReshuffleAndFiniteMarket(t *testing.T) {
	g := Rail{Face: []int{8, 8, 8, 0, 1}, Deck: []int{2, 3, 4, 5}, Discard: []int{}}
	g.refill()
	wild := 0
	for _, c := range g.Face {
		if c == 8 {
			wild++
		}
	}
	if wild >= 3 || len(g.Face) != 5 {
		t.Fatal("market not reset")
	}
	if len(g.Deck)+len(g.Discard)+len(g.Face) != 9 {
		t.Fatal("lost cards")
	}
	g = Rail{Face: []int{}, Discard: []int{8, 8, 8, 8, 8}}
	g.refill()
	if len(g.Face) != 5 {
		t.Fatal("finite degenerate market")
	}
	g = Rail{Face: []int{8, 8, 8}, Deck: nil, Discard: nil}
	if g.canDrawSecond() {
		t.Fatal("cannot draw a second face-up wild")
	}
}
func TestRailClaimAndDoubleRoutes(t *testing.T) {
	for _, n := range []int{2, 3, 4, 5} {
		s := playingRail(t, n)
		g := s.Rail
		id := routeID(t, "Vancouver", "Seattle")
		p := &g.Players[0]
		p.Hand = make([]int, 9)
		p.Hand[0] = 3
		p.Hand[8] = 3
		apply(t, s, Action{Type: "claim", Route: id, Color: 0})
		if p.Trains != 44 || p.Score != 1 || g.Owners[id] != 0 {
			t.Fatal("route payment")
		}
		g.Players[1].Hand = make([]int, 9)
		g.Players[1].Hand[0] = 3
		c := clone(*s)
		err := c.Apply(1, Action{Type: "claim", Route: id + 1, Color: 0})
		if (err == nil) != (n >= 4) {
			t.Fatalf("double line wrong for %d", n)
		}
		s.Turn = 0
		reject(t, s, Action{Type: "claim", Route: id + 1, Color: 0})
	}
}
func TestRailClaimColorAndGold(t *testing.T) {
	s := playingRail(t, 2)
	id := routeID(t, "Seattle", "Helena")
	s.Rail.Players[0].Hand = []int{6, 6, 6, 4, 6, 6, 6, 6, 2}
	reject(t, s, Action{Type: "claim", Route: id, Color: 0})
	reject(t, s, Action{Type: "claim", Route: id, Color: 3, Wild: 1})
	apply(t, s, Action{Type: "claim", Route: id, Color: 3, Wild: 2})
	p := s.Rail.Players[0]
	if p.Trains != 39 || p.Score != 15 || p.Hand[3] != 0 || p.Hand[8] != 0 {
		t.Fatal(p)
	}
}
func TestRailLastRoundIncludesTrigger(t *testing.T) {
	s := playingRail(t, 3)
	s.Rail.Players[0].Trains = 2
	s.railNext()
	if s.Rail.LastRemaining != 3 || s.Turn != 1 {
		t.Fatal("trigger")
	}
	s.railNext()
	s.railNext()
	if s.Finished || s.Turn != 0 {
		t.Fatal("trigger must get final turn")
	}
	s.railNext()
	if !s.Finished {
		t.Fatal("did not finish")
	}
}
func TestLongestAllowsCyclesAndRejectsReuse(t *testing.T) {
	owners := map[int]int{}
	for _, pair := range [][2]string{{"Vancouver", "Seattle"}, {"Vancouver", "Calgary"}, {"Seattle", "Calgary"}, {"Seattle", "Helena"}} {
		owners[routeID(t, pair[0], pair[1])] = 0
	}
	if length := Longest(owners, 0); length != 14 {
		t.Fatalf("want 14, got %d", length)
	}
	if Longest(owners, 1) != 0 {
		t.Fatal("opponent track counted")
	}
	d := MapData()
	r := d.Routes[routeID(t, "Seattle", "Helena")-1]
	if !Connected(owners, 0, r.A, r.B) || Connected(owners, 1, r.A, r.B) {
		t.Fatal("connectivity")
	}
}
func TestRailFinalScoresAndSharedBonus(t *testing.T) {
	s := playingRail(t, 2)
	r := MapData().Routes[0]
	s.Rail.Owners = map[int]int{1: 0, 4: 1}
	s.Rail.Players[0].Tickets = []Ticket{{ID: 100, A: r.A, B: r.B, Points: 7}}
	s.Rail.Players[1].Tickets = []Ticket{{ID: 101, A: 0, B: 35, Points: 10}}
	s.Rail.Players[0].RouteScore = 4
	s.Rail.Players[1].RouteScore = 7
	s.railFinish()
	a, b := s.Rail.Players[0], s.Rail.Players[1]
	if a.Score != 11 || a.Completed != 1 || b.Score != 7 || b.Bonus != 10 || !reflect.DeepEqual(s.Winners, []int{0}) {
		t.Fatal(a, b, s.Winners)
	}
	raw, _ := json.Marshal(s.View(0))
	if !strings.Contains(string(raw), `"tickets"`) {
		t.Fatal("no revealed tickets")
	}
}
func railInvariant(t *testing.T, s *State) {
	t.Helper()
	g := s.Rail
	cards := len(g.Deck) + len(g.Discard)
	for _, c := range g.Face {
		if c >= 0 {
			cards++
		}
	}
	tickets := len(g.TicketDeck) + len(g.Pending)
	for _, pending := range g.SetupPending {
		tickets += len(pending)
	}
	for i, p := range g.Players {
		cards += sum(p.Hand)
		tickets += len(p.Tickets)
		trains := p.Trains
		for _, r := range MapData().Routes {
			if owner, ok := g.Owners[r.ID]; ok && owner == i {
				trains += r.Length
			}
		}
		if trains != 45 {
			t.Fatal("train conservation")
		}
	}
	if cards != 110 || tickets != 30 {
		t.Fatal("rail card conservation", cards, tickets)
	}
}
func TestFullRailGame(t *testing.T) {
	for _, n := range []int{2, 5} {
		s := playingRail(t, n)
		for step := 0; step < 1600 && !s.Finished; step++ {
			railInvariant(t, s)
			if s.Phase == "tickets" {
				keep := []int{}
				minimum := 1
				if s.Rail.Setup {
					minimum = 2
				}
				for _, t := range s.Rail.Pending[:minimum] {
					keep = append(keep, t.ID)
				}
				apply(t, s, Action{Type: "keep", Keep: keep})
				continue
			}
			if s.Phase == "draw" {
				apply(t, s, Action{Type: "draw", Slot: -1})
				continue
			}
			best := Action{}
			bestLength := 0
			for _, r := range MapData().Routes {
				if _, occupied := s.Rail.Owners[r.ID]; occupied || r.Length <= bestLength || r.Length > s.Rail.Players[s.Turn].Trains {
					continue
				}
				p := s.Rail.Players[s.Turn]
				for c := 0; c < 8; c++ {
					if r.Color >= 0 && r.Color != c {
						continue
					}
					wild := max(0, r.Length-p.Hand[c])
					if wild > p.Hand[8] {
						continue
					}
					candidate := Action{Type: "claim", Route: r.ID, Color: c, Wild: wild}
					copy := clone(*s)
					if copy.Apply(copy.Turn, candidate) == nil {
						best = candidate
						bestLength = r.Length
						break
					}
				}
			}
			if bestLength > 0 {
				apply(t, s, best)
			} else if len(s.Rail.Deck)+len(s.Rail.Discard) > 0 {
				apply(t, s, Action{Type: "draw", Slot: -1})
			} else if len(s.Rail.Face) > 0 {
				apply(t, s, Action{Type: "draw", Slot: 0})
			} else if len(s.Rail.TicketDeck) > 0 {
				apply(t, s, Action{Type: "tickets"})
			} else {
				apply(t, s, Action{Type: "pass"})
			}
		}
		railInvariant(t, s)
		if !s.Finished {
			t.Fatalf("%d player game failed to finish", n)
		}
	}
}
