package game

import "testing"

// frozenCaravans fills both seats' building and route inventories, exhausts the
// caravan supply and empties the development deck, which is the frozen board
// the natural two-player games reached before the site rule existed.
func frozenCaravans(t *testing.T) *State {
	t.Helper()
	s, err := NewCatanCaravansShoresSeafarers(2)
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Phase = "catan_turn"
	usedVertex := map[int]bool{}
	for p := 0; p < 2; p++ {
		settlements, cities := 0, 0
		for _, v := range g.Vertices {
			if usedVertex[v.ID] || v.Harbor {
				continue
			}
			usedVertex[v.ID] = true
			if settlements < 5 {
				g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = p, 1
				settlements++
				continue
			}
			if cities < 4 {
				g.Vertices[v.ID].Owner, g.Vertices[v.ID].Level = p, 2
				cities++
				continue
			}
			break
		}
		if settlements != 5 || cities != 4 {
			t.Fatal("fixture could not place building inventory", settlements, cities)
		}
	}
	usedEdge := map[int]bool{}
	for p := 0; p < 2; p++ {
		roads, ships := 0, 0
		for _, e := range g.Edges {
			if usedEdge[e.ID] {
				continue
			}
			usedEdge[e.ID] = true
			if roads < 15 {
				g.Edges[e.ID].Owner, g.Edges[e.ID].Ship, g.Edges[e.ID].Bridge = p, false, false
				roads++
				continue
			}
			if ships < 15 {
				g.Edges[e.ID].Owner, g.Edges[e.ID].Ship = p, true
				ships++
				continue
			}
			break
		}
		if roads != 15 || ships != 15 {
			t.Fatal("fixture could not place route inventory", roads, ships)
		}
	}
	g.DevDeck = []int{}
	g.Caravans.Wagons = make([]catanCaravanWagon, g.Caravans.Map.Supply)
	return s
}

func TestCatanCaravansDeadEndRule(t *testing.T) {
	s := frozenCaravans(t)
	g := s.Catan
	if !s.catanCaravansDeadEnd() {
		t.Fatal("frozen board did not end")
	}
	if !s.Finished || s.Phase != "finished" || len(s.Winners) == 0 {
		t.Fatal("frozen finish state", s.Finished, s.Phase, s.Winners)
	}
	if !caravansDeadEnded(s) {
		t.Fatal("missing rule marker")
	}
	best := -1
	for _, seat := range g.Players {
		if seat.Score > best {
			best = seat.Score
		}
	}
	for _, winner := range s.Winners {
		if g.Players[winner].Score != best {
			t.Fatal("winner is not leading", winner, g.Players[winner].Score, best)
		}
	}

	// The supply is still open.
	open := frozenCaravans(t)
	open.Catan.Caravans.Wagons = []catanCaravanWagon{}
	if open.catanCaravansDeadEnd() {
		t.Fatal("ended with caravans left")
	}
	// A hidden victory card can still decide the game.
	card := frozenCaravans(t)
	card.Catan.DevDeck = []int{4}
	if card.catanCaravansDeadEnd() {
		t.Fatal("ended while a victory card remained")
	}
	// An open board keeps playing.
	board := frozenCaravans(t)
	board.Catan.Vertices = append([]CatanVertex{}, board.Catan.Vertices...)
	for i := range board.Catan.Vertices {
		board.Catan.Vertices[i].Owner, board.Catan.Vertices[i].Level = -1, 0
	}
	if board.catanCaravansDeadEnd() {
		t.Fatal("ended with an open board")
	}
}
