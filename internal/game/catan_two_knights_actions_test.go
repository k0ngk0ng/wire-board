package game

import (
	"reflect"
	"slices"
	"testing"
)

// Find a real, empty three-vertex chain without replacing the map/topology.
func twoCityKnightChain(t *testing.T, s *State) [3]int {
	t.Helper()
	g := s.Catan
	for _, a := range g.Edges {
		for _, b := range g.Edges {
			if a.ID == b.ID {
				continue
			}
			for _, middle := range []int{a.A, a.B} {
				if b.A != middle && b.B != middle {
					continue
				}
				left, right := a.A+a.B-middle, b.A+b.B-middle
				if g.Vertices[left].Level+g.Vertices[middle].Level+g.Vertices[right].Level != 0 {
					continue
				}
				for i := range g.Edges {
					g.Edges[i].Owner = -1
				}
				g.Edges[a.ID].Owner = s.Turn
				g.Edges[b.ID].Owner = -2
				return [3]int{left, middle, right}
			}
		}
	}
	t.Fatal("no empty chain")
	return [3]int{}
}

func TestCatanTwoKnightsNeutralDisplacement(t *testing.T) {
	for _, intrigue := range []bool{false, true} {
		s := twoKnightsFixture(t)
		p := s.Turn
		v := twoCityKnightChain(t, s)
		k := s.Catan.CitiesKnights
		k.ActionSerial = 4
		k.Knights = []CatanKnight{{Owner: p, Vertex: v[0], Strength: 2, Active: true, ActivatedAt: 2}, {Owner: -2, Vertex: v[1], Strength: 1, PromotedAt: 1}}
		s.catanScores()
		if intrigue {
			ckProgressGive(t, s, p, 19)
			helperApply(t, s, p, Action{Type: "catan_progress", Card: 19, Vertex: v[1]})
		} else {
			helperApply(t, s, p, Action{Type: "catan_knight_move", Vertex: v[0], Target: v[1]})
		}
		if s.CatanPendingActor() != p || s.Phase != "catan_knight_retreat" {
			t.Fatal("neutral retreat has no real controller")
		}
		twoCoreRestore(t, s)
		helperReject(t, s, 1-p, Action{Type: "catan_knight_retreat", Vertex: v[2]})
		helperReject(t, s, p, Action{Type: "catan_knight_retreat", Vertex: v[0]})
		bad := clone(*s)
		bad.Catan.CitiesKnights.Pending.Players = []int{-2}
		if bad.validateCatanTwo() == nil {
			t.Fatal("negative response seat restored")
		}
		s.AutoCatanPending()
		n := s.Catan.knightAt(v[2])
		if s.Phase != "catan_turn" || n == nil || n.Owner != -2 || n.Active || n.Strength != 1 || n.PromotedAt != 1 {
			t.Fatal("neutral retreat changed piece or stalled")
		}
		twoCoreRestore(t, s)
		// No neutral retreat route: the next displacement returns the piece.
		for i := range s.Catan.Edges {
			if s.Catan.Edges[i].Owner == -2 {
				s.Catan.Edges[i].Owner = p
			}
		}
		ckProgressGive(t, s, p, 19)
		helperApply(t, s, p, Action{Type: "catan_progress", Card: 19, Vertex: v[2]})
		if s.Phase != "catan_turn" || s.Catan.knightAt(v[2]) != nil {
			t.Fatal("blocked neutral retreat not removed")
		}
	}
}

func TestCatanTwoKnightsNeutralTreason(t *testing.T) {
	s := twoKnightsFixture(t)
	p := s.Turn
	v := twoCityKnightChain(t, s)
	g := s.Catan
	g.CitiesKnights.Knights = []CatanKnight{{Owner: -2, Vertex: v[1], Strength: 2}, {Owner: -2, Vertex: v[2], Strength: 1}}
	s.catanScores()
	ckProgressGive(t, s, p, 22)
	helperApply(t, s, p, Action{Type: "catan_progress", Card: 22, Target: -2})
	if s.CatanPendingActor() != p {
		t.Fatal("neutral Treason lacks controller")
	}
	twoCoreRestore(t, s)
	helperReject(t, s, p, Action{Type: "catan_treason_remove", Vertex: v[1]})
	helperReject(t, s, 1-p, Action{Type: "catan_treason_remove", Vertex: v[2]})
	a, err := s.BotAction(p)
	if err != nil || a.Vertex != v[2] {
		t.Fatal("bot did not choose weakest neutral", a, err)
	}
	bad := clone(*s)
	bad.Catan.CitiesKnights.Pending.Color = -1
	if bad.validateCatanTwo() == nil {
		t.Fatal("invalid neutral owner restored")
	}
	helperApply(t, s, p, a)
	if s.Phase != "catan_treason_place" {
		t.Fatal("missing own replacement")
	}
	twoCoreRestore(t, s)
	helperApply(t, s, p, Action{Type: "catan_treason_place", Vertex: v[0], Color: 1})
	if s.Phase != "catan_turn" || s.Catan.Two.Pending != nil || s.Catan.knightAt(v[0]).Owner != p || s.Catan.knightAt(v[2]) != nil {
		t.Fatal("replacement recruited an extra neutral")
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanTwoKnightsKnightTokenExchange(t *testing.T) {
	s := twoKnightsFixture(t)
	p := s.Turn
	v := twoCityKnightChain(t, s)
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: p, Vertex: v[0], Strength: 2}, {Owner: -2, Vertex: v[1], Strength: 1}}
	s.Catan.Two.Bank = 1
	s.Catan.Two.Tokens = []int{9, 10}
	helperReject(t, s, p, Action{Type: "catan_two_knight", Vertex: v[1]})
	helperReject(t, s, p, Action{Type: "catan_two_knight", Vertex: v[0]})
	s.Catan.Two.Bank++
	s.Catan.Two.Tokens[p]--
	before := s.Catan.Two.Tokens[p]
	helperApply(t, s, p, Action{Type: "catan_two_knight", Vertex: v[0]})
	if s.Catan.knightAt(v[0]) != nil || s.Catan.Two.Bank != 0 || s.Catan.Two.Tokens[p] != before+2 || !s.Catan.Two.KnightExchanged {
		t.Fatal("knight exchange payment")
	}
	twoCoreRestore(t, s)
	// The AI may exchange spare inactive strength, retaining city defense.
	s = twoKnightsFixture(t)
	p = s.Turn
	v = twoCityKnightChain(t, s)
	s.Catan.CitiesKnights.Knights = []CatanKnight{{Owner: p, Vertex: v[0], Strength: 1}, {Owner: p, Vertex: v[2], Strength: 2}}
	s.Catan.Two.Bank += s.Catan.Two.Tokens[p]
	s.Catan.Two.Tokens[p] = 0
	a, ok := s.catanTwoOptionalBot(p)
	if !ok || a.Type != "catan_two_knight" {
		t.Fatal("bot never exchanges spare knight")
	}
	helperApply(t, s, p, a)
}

func TestCatanTwoKnightsRoadCardsAndRestoreGuards(t *testing.T) {
	s := twoKnightsFixture(t)
	p := s.Turn
	ckProgressGive(t, s, p, 7) // Road Building
	helperApply(t, s, p, Action{Type: "catan_progress", Card: 7})
	responses := 0
	for step := 0; s.Phase != "catan_turn" && step < 8; step++ {
		if s.Phase == "catan_two_build" {
			responses++
		}
		twoCoreRestore(t, s)
		a, err := s.BotAction(p)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, p, a)
	}
	if responses != 2 || s.Phase != "catan_turn" {
		t.Fatal("Road Building missing neutral roads", responses)
	}
	ckProgressGive(t, s, p, 16)
	road := -1
	for _, edge := range s.Catan.diplomacyRoads() {
		if s.Catan.Edges[edge].Owner == p {
			road = edge
			break
		}
	}
	if road < 0 {
		t.Fatal("no own open road")
	}
	helperApply(t, s, p, Action{Type: "catan_progress", Card: 16, Edge: road})
	if s.Phase != "catan_diplomacy" {
		t.Fatal("no relocation")
	}
	twoCoreRestore(t, s)
	helperApply(t, s, p, Action{Type: "catan_diplomacy", Edge: s.Catan.diplomacyPlacements(p)[0]})
	if s.Phase != "catan_turn" || s.Catan.Two.Pending != nil {
		t.Fatal("Diplomacy must not add neutral road")
	}
	for name, mutate := range map[string]func(*State){
		"progress stock": func(s *State) {
			s.Catan.CitiesKnights.ProgressDecks[0] = append(s.Catan.CitiesKnights.ProgressDecks[0], 0)
		},
		"foreign card": func(s *State) {
			s.Catan.CitiesKnights.Players[0].Progress = append(s.Catan.CitiesKnights.Players[0].Progress, -1)
		},
		"phantom response": func(s *State) { s.Phase = "catan_knight_retreat" },
		"infinite tokens":  func(s *State) { s.Catan.Two.TokensIssued = 1; s.Catan.Two.Bank++ },
		"city level":       func(s *State) { s.Catan.CitiesKnights.Players[0].Improvements[0] = 6 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := clone(*s)
			mutate(&bad)
			if bad.validateCatanTwo() == nil {
				t.Fatal("accepted invalid saved game")
			}
		})
	}
	before := clone(*s)
	if err := s.catanTwoRoll(1, 2); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("base dice bypassed city event")
	}
	if !slices.Equal(s.Catan.Two.Rolls, []int{2, 3}) {
		t.Fatal("card actions altered production")
	}
}
