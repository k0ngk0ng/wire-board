package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustGame(t *testing.T, kind string, n int) *State {
	t.Helper()
	s, e := New(kind, n)
	if e != nil {
		t.Fatal(e)
	}
	// Rule fixtures use a fixed seat order; random starts are tested separately.
	if s.Splendor != nil {
		s.Turn, s.Splendor.StartPlayer = 0, 0
	}
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
func playingRail(t *testing.T, n int) *State {
	s := mustGame(t, "rail", n)
	s.AutoChooseRailSetup()
	return s
}
func gemInvariant(t *testing.T, s *State) {
	t.Helper()
	g := s.Splendor
	total := append([]int{}, g.Bank...)
	cards := 0
	for i := 0; i < 3; i++ {
		cards += len(g.Decks[i]) + len(g.Market[i])
	}
	for _, p := range g.Players {
		cards += len(p.Cards) + len(p.Reserved)
		for i, v := range p.Tokens {
			if v < 0 {
				t.Fatal("negative tokens")
			}
			total[i] += v
		}
		if len(p.Reserved) > 3 {
			t.Fatal("reservation overflow")
		}
	}
	if cards != 90 {
		t.Fatal("card conservation", cards)
	}
	want := 7
	if len(g.Players) < 4 {
		want = len(g.Players) + 2
	}
	for i, n := range total {
		w := want
		if i == 5 {
			w = 5
		}
		if n != w {
			t.Fatal("token conservation", total)
		}
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
func TestBaseData(t *testing.T) {
	cards := Cards()
	if len(cards) != 90 || len(Nobles()) != 10 {
		t.Fatalf("incorrect Splendor data: %d cards", len(cards))
	}
	counts := [3]int{}
	colors := [5]int{}
	for _, c := range cards {
		counts[c.Tier-1]++
		colors[c.Color]++
		if c.Points < 0 || c.Color < 0 || len(c.Cost) != 5 {
			t.Fatal(c)
		}
	}
	if counts != [3]int{40, 30, 20} || colors != [5]int{18, 18, 18, 18, 18} {
		t.Fatal(counts, colors)
	}
	d := MapData()
	if len(d.Cities) != 36 || len(d.Routes) != 100 || len(d.Tickets) != 30 {
		t.Fatalf("bad rail data: %d %d %d", len(d.Cities), len(d.Routes), len(d.Tickets))
	}
	for _, r := range d.Routes {
		if r.Length < 1 || r.Length > 6 || r.A >= 36 || r.B >= 36 || r.Color > 7 || r.Color < -1 {
			t.Fatal(r)
		}
	}
}
func clone[T any](v T) T { b, _ := json.Marshal(v); var c T; _ = json.Unmarshal(b, &c); return c }
func sum(v []int) int {
	n := 0
	for _, x := range v {
		n += x
	}
	return n
}

func TestSplendorEliminationLeavesOneWinner(t *testing.T) {
	s := mustGame(t, "splendor", 2)
	if err := s.EliminateSplendor(1); err == nil {
		t.Fatal("non-current player eliminated")
	}
	if err := s.EliminateSplendor(0); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Phase != "finished" || !reflect.DeepEqual(s.Winners, []int{1}) {
		t.Fatal("sole remaining player did not win")
	}
	if err := s.EliminateSplendor(1); err == nil {
		t.Fatal("eliminated player after game finished")
	}
	r := mustGame(t, "rail", 2)
	if err := r.EliminateSplendor(0); err == nil {
		t.Fatal("Splendor elimination changed rail game")
	}
}
