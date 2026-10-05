package game

import (
	"reflect"
	"testing"
)

func ckCycle(t *testing.T, tail bool) *State {
	s := ckKnightGraph(t, []int{0, 0, 0, 0})
	g := s.Catan
	g.Edges = []CatanEdge{{ID: 0, A: 0, B: 1, Owner: 0}, {ID: 1, A: 1, B: 2, Owner: 0}, {ID: 2, A: 2, B: 3, Owner: 0}, {ID: 3, A: 3, B: 0, Owner: 0}}
	if tail {
		g.Edges[0] = CatanEdge{ID: 0, A: 0, B: 1, Owner: 0}
		g.Edges[3] = CatanEdge{ID: 3, A: 3, B: 1, Owner: 0}
	}
	return s
}
func TestCatanOpenRoutesOfficialCyclesKnightsAndEnemyInterruptions(t *testing.T) {
	for _, ship := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			anchors []int
			tail    bool
			want    []int
		}{
			{"unanchored circle", nil, false, []int{0, 1, 2, 3}},
			{"one anchor", []int{0}, false, []int{0, 3}},
			{"two anchors", []int{0, 2}, false, []int{}},
			{"lollipop", []int{0}, true, []int{1, 2, 3}},
		} {
			t.Run(tc.name+map[bool]string{false: "/road", true: "/ship"}[ship], func(t *testing.T) {
				s := ckCycle(t, tc.tail)
				g := s.Catan
				for _, v := range tc.anchors {
					g.Vertices[v].Owner, g.Vertices[v].Level = 0, 1
				}
				for i := range g.Edges {
					g.Edges[i].Ship = ship
				}
				g.Seafarers = &CatanSeafarers{Pirate: -1}
				got := []int{}
				for i := range g.Edges {
					if g.openRoute(0, i) {
						got = append(got, i)
					}
					if ship && g.movableShip(0, i) != g.openRoute(0, i) {
						t.Fatal("ship movement does not use official openness")
					}
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Fatal("wrong open edges", got, tc.want)
				}
				if len(tc.anchors) == 2 {
					g.Vertices[1].Owner, g.Vertices[1].Level = 1, 1
					for i := range g.Edges {
						if g.openRoute(0, i) {
							t.Fatal("enemy interruption opened closed line")
						}
					}
				}
				if ship {
					g.Seafarers.MovedShip = true
					if g.movableShip(0, 0) {
						t.Fatal("second movement permitted")
					}
					g.Seafarers.MovedShip = false
					g.Seafarers.BuiltShips = []int{0}
					if g.movableShip(0, 0) {
						t.Fatal("new ship movable")
					}
				}
			})
		}
	}
	s := ckKnightGraph(t, []int{0, 0, 0})
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 1
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 3, Strength: 1}}
	for i := range s.Catan.Edges {
		if s.Catan.openRoute(0, i) {
			t.Fatal("diplomacy may not detach knight from its city")
		}
	}
	s.Catan.CitiesKnights.Knights = nil
	if !s.Catan.openRoute(0, 2) || s.Catan.openRoute(0, 0) {
		t.Fatal("only unanchored tail is open")
	}
}
func TestCatanProgressPoliticsDiplomacyAtomicRelocationAndAward(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, -1, 1, 1, 1, 1, 1})
	g := s.Catan
	g.Vertices[0].Owner, g.Vertices[0].Level = 0, 1
	g.Vertices[6].Owner, g.Vertices[6].Level = 1, 1
	g.LongestOwner = 0
	s.catanScores()
	ckProgressGive(t, s, 0, 16, 16)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: 1})
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: 4})
	if s.Phase != "catan_diplomacy" || s.Catan.LongestOwner != 0 || s.Catan.Edges[4].Owner != -1 {
		t.Fatal("diplomacy changed longest holder in the middle")
	}
	helperReject(t, s, 1, Action{Type: "catan_diplomacy", Edge: 4})
	helperReject(t, s, 0, Action{Type: "catan_diplomacy", Edge: 0})
	helperReject(t, s, 0, Action{Type: "catan_diplomacy", Edge: 5})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_diplomacy", Edge: 4})
	if s.Phase != "catan_turn" || s.Catan.LongestOwner != 0 || s.Catan.Players[0].RoadLength != 5 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("relocation lost incumbent tie or charged cost")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: 10})
	if s.Phase != "catan_turn" || s.CatanPendingActor() != -1 || s.Catan.Edges[10].Owner != -1 {
		t.Fatal("enemy road removal gave free replacement")
	}
	ckProgressStock(t, s.Catan)
	s = ckKnightGraph(t, []int{0, 0})
	s.Catan.Vertices[0].Owner, s.Catan.Vertices[0].Level = 0, 1
	ckProgressGive(t, s, 0, 16)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 16, Edge: 1})
	helperApply(t, s, 0, Action{Type: "catan_diplomacy", Choice: "skip"})
	if s.Catan.Edges[1].Owner != -1 || s.CatanPendingActor() != -1 {
		t.Fatal("optional relocation skip")
	}
	ckProgressStock(t, s.Catan)
}
