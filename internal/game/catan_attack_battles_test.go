package game

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func attackDice(t *testing.T, values ...int) func() int {
	t.Helper()
	i := 0
	return func() int {
		t.Helper()
		if i >= len(values) {
			t.Fatal("unexpected die roll", i)
		}
		v := values[i]
		i++
		return v
	}
}
func attackBattleKnights(s *State, tile int, owners ...int) {
	s.Catan.Attack.Knights = nil
	for side, p := range owners {
		s.Catan.Attack.Knights = append(s.Catan.Attack.Knights, catanAttackKnight{p, catanFishingSide(s.Catan, tile, side)})
	}
}
func attackEndReject(t *testing.T, s *State, moves []catanAttackMove, die func() int) {
	t.Helper()
	before, _ := json.Marshal(s)
	if err := s.catanAttackResolveEnd(moves, die); err == nil {
		t.Fatal("invalid end phase accepted")
	}
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("rejected end phase mutated state")
	}
}
func TestCatanAttackPrisonerDistribution(t *testing.T) {
	tests := []struct {
		name             string
		strength         []int
		barb             int
		dice, want, gold []int
		contests         int
	}{
		{"sole", []int{4, 0, 0}, 3, nil, []int{3, 0, 0}, []int{0, 0, 0}, 0},
		{"majority", []int{3, 1, 0}, 3, nil, []int{2, 1, 0}, []int{0, 0, 0}, 0},
		{"equalShare", []int{2, 1, 1, 0}, 3, nil, []int{1, 1, 1, 0}, []int{0, 0, 0, 0}, 0},
		{"extraTieCompensation", []int{2, 2, 0}, 3, []int{4, 4, 6, 1}, []int{2, 1, 0}, []int{0, 3, 0}, 2},
		{"shortageOnlyBoundaryRerolls", []int{1, 1, 1, 1}, 2, []int{6, 4, 4, 1, 2, 5}, []int{1, 0, 1, 0}, []int{0, 3, 0, 3}, 2},
		{"irrelevantLowTie", []int{1, 1, 1, 1}, 1, []int{6, 1, 1, 1}, []int{1, 0, 0, 0}, []int{0, 3, 3, 3}, 1},
		{"sixPlayers", []int{1, 1, 1, 1, 1, 1}, 3, []int{1, 2, 3, 4, 5, 6}, []int{0, 0, 0, 1, 1, 1}, []int{3, 3, 3, 0, 0, 0}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := catanAttackBattle{Barbarians: tc.barb, Prisoners: make([]int, len(tc.strength)), Gold: make([]int, len(tc.strength))}
			if err := b.distribute(tc.strength, attackDice(t, tc.dice...)); err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(b.Prisoners, tc.want) || !slices.Equal(b.Gold, tc.gold) || len(b.Contests) != tc.contests {
				t.Fatal(b)
			}
			if tc.name == "shortageOnlyBoundaryRerolls" && !slices.Equal(b.Contests[1].Players, []int{1, 2}) {
				t.Fatal("unrelated ranks rerolled", b)
			}
		})
	}
}
func TestCatanAttackEndMovementOrders(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, false)
		g, a := s.Catan, s.Catan.Attack
		p := s.Turn
		from := catanFishingSide(g, a.Map.Castles[0], 0)
		a.Knights = []catanAttackKnight{{p, from}}
		target3, target5 := -1, -1
		for edge, d := range a.knightDestinations(g, 0, 5) {
			if d == 3 {
				target3 = edge
			}
			if d == 5 {
				target5 = edge
			}
		}
		if target3 < 0 || target5 < 0 {
			t.Fatal("fixture unreachable")
		}
		attackEndReject(t, s, nil, attackDice(t))
		attackEndReject(t, s, []catanAttackMove{{from, target5, false}}, attackDice(t))
		attackEndReject(t, s, []catanAttackMove{{from, target5, true}}, attackDice(t))
		attackHand(s, p, []int{0, 0, 0, 2, 0})
		attackEndReject(t, s, []catanAttackMove{{from, target3, false}, {from, target5, true}}, attackDice(t))
		saved := clone(*s)
		if err := s.catanAttackResolveEnd([]catanAttackMove{{from, target5, true}}, attackDice(t)); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[p].Resources[3] != 1 || s.Catan.Attack.Knights[0].Edge != target5 || s.Catan.Attack.End.Player != p || s.Catan.Attack.EndSequence != 1 {
			t.Fatal("move or payment lost")
		}
		if s.Turn == p {
			t.Fatal("no turn handoff")
		}
		assertAttackRestored(t, s)
		s = &saved
		if err := s.catanAttackResolveEnd([]catanAttackMove{{from, target3, false}}, attackDice(t)); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[p].Resources[3] != 2 {
			t.Fatal("free move charged")
		}
	}
}
func TestCatanAttackEndSoleBattleCasualtiesAndRecord(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for die := 1; die <= 6; die++ {
			s := newAttackState(t, n, false)
			a := s.Catan.Attack
			tile := a.Map.Coast[3]
			p := s.Turn
			a.Barbarians = make([]int, len(a.Barbarians))
			a.Barbarians[tile] = 3
			attackBattleKnights(s, tile, p, p, p, p, p, p)
			expected := []catanAttackKnight{}
			for _, k := range a.Knights {
				if a.Map.edgeOrientation(s.Catan, k.Edge) == catanAttackLossOrientation(die) {
					expected = append(expected, k)
				}
			}
			if len(expected) != 2 {
				t.Fatal("orientation fixture")
			}
			if err := s.catanAttackResolveEnd(nil, attackDice(t, die)); err != nil {
				t.Fatal(err)
			}
			a = s.Catan.Attack
			b := a.End.Battles[0]
			if a.Prisoners[p] != 3 || a.Gold[p] != 6 || len(a.Knights) != 4 || a.Barbarians[tile] != 0 || !reflect.DeepEqual(b.Lost, expected) || b.LossDie != die || s.Catan.Players[p].Score != 1 {
				t.Fatal("battle result", b)
			}
			if a.supply() != a.Map.Barbarians-3 {
				t.Fatal("captives were returned to supply")
			}
			assertAttackRestored(t, s)
			for viewer := -1; viewer < n; viewer++ {
				v := s.View(viewer)["catan"].(map[string]any)["attack"].(map[string]any)
				if v["end"] == nil || v["deck"] != nil {
					t.Fatal("record not public or deck leaked")
				}
			}
		}
	}
}
func TestCatanAttackEndOrderedLossBeforeNextBattle(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, false)
		g, a := s.Catan, s.Catan.Attack
		p := s.Turn
		first, second, shared := -1, -1, -1
		for i, one := range a.Map.Coast {
			for _, two := range a.Map.Coast[i+1:] {
				for _, e := range g.Edges {
					if slices.Contains(e.Tiles, one) && slices.Contains(e.Tiles, two) {
						first, second, shared = one, two, e.ID
						break
					}
					if shared >= 0 {
						break
					}
				}
				if shared >= 0 {
					break
				}
			}
			if shared >= 0 {
				break
			}
		}
		if shared < 0 {
			t.Fatal("no adjacent coast")
		}
		orientation := a.Map.edgeOrientation(g, shared)
		loss := []int{1, 2, 3}[orientation]
		a.Barbarians = make([]int, len(a.Barbarians))
		a.Barbarians[first] = 1
		a.Barbarians[second] = 1
		a.Knights = []catanAttackKnight{{p, shared}}
		for _, tile := range []int{first, second} {
			for _, e := range g.Edges {
				if e.ID != shared && slices.Contains(e.Tiles, tile) && a.Map.edgeOrientation(g, e.ID) != orientation && !a.castleEdge(g, e.ID) {
					a.Knights = append(a.Knights, catanAttackKnight{p, e.ID})
					break
				}
			}
		}
		if len(a.Knights) != 3 {
			t.Fatal("fixture")
		}
		if err := s.catanAttackResolveEnd(nil, attackDice(t, loss)); err != nil {
			t.Fatal(err)
		}
		a = s.Catan.Attack
		if len(a.End.Battles) != 1 || a.End.Battles[0].Tile != first || a.Barbarians[second] != 1 || len(a.Knights) != 2 {
			t.Fatal("lost shared knight fought again", a.End)
		}
		assertAttackRestored(t, s)
	}
}
func TestCatanAttackEndRejectsDiceGoldAndWrongOrdersAtomically(t *testing.T) {
	s := newAttackState(t, 3, false)
	a := s.Catan.Attack
	tile := a.Map.Coast[3]
	p := s.Turn
	a.Barbarians = make([]int, len(a.Barbarians))
	a.Barbarians[tile] = 1
	attackBattleKnights(s, tile, p, p)
	attackEndReject(t, s, nil, attackDice(t, 7))
	other := (p + 1) % 3
	attackCoins(s, other, a.GoldBank)
	// At least one of the two occupied edges is removed for this die.
	die := []int{1, 2, 3}[a.Map.edgeOrientation(s.Catan, a.Knights[0].Edge)]
	attackEndReject(t, s, nil, attackDice(t, die))
	attackCoins(s, other, 0)
	a.Knights[0].Player = other
	attackEndReject(t, s, []catanAttackMove{{a.Knights[0].Edge, a.Knights[1].Edge, false}}, attackDice(t))
	attackEndReject(t, s, nil, func() int { return 2 }) // Equal distribution rolls never fabricate a winner.
}
func TestCatanAttackEndVictoryAndNonCurrentScores(t *testing.T) {
	for _, activeWins := range []bool{true, false} {
		s := newAttackState(t, 3, false)
		a := s.Catan.Attack
		p := s.Turn
		owner := p
		if !activeWins {
			owner = (p + 2) % 3
		} // not the next player
		tile := a.Map.Coast[3]
		a.Barbarians = make([]int, len(a.Barbarians))
		a.Barbarians[tile] = 1
		a.Prisoners[owner] = 23
		attackBattleKnights(s, tile, owner, owner)
		dice := []int{1}
		if activeWins {
			dice = nil
		}
		if err := s.catanAttackResolveEnd(nil, attackDice(t, dice...)); err != nil {
			t.Fatal(err)
		}
		if s.Finished != activeWins || s.Catan.Players[owner].Score != 12 {
			t.Fatal("wrong victory timing")
		}
		if activeWins && (!slices.Equal(s.Winners, []int{owner}) || s.Catan.Attack.End.Battles[0].LossDie != 0) {
			t.Fatal("continued after win")
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackEndVacatedOriginsAndIndependentWheat(t *testing.T) {
	s := newAttackState(t, 4, false)
	g, a := s.Catan, s.Catan.Attack
	p := s.Turn
	a.Barbarians = make([]int, len(a.Barbarians))
	first := catanFishingSide(g, a.Map.Coast[3], 0)
	second := catanFishingSide(g, a.Map.Coast[3], 1)
	a.Knights = []catanAttackKnight{{p, first}, {p, second}}
	destination := -1
	for edge := range a.knightDestinations(g, 0, 3) {
		if edge != first && edge != second {
			destination = edge
			break
		}
	}
	if destination < 0 {
		t.Fatal("fixture")
	}
	attackHand(s, p, []int{0, 0, 0, 1, 0})
	attackEndReject(t, s, []catanAttackMove{{first, destination, true}, {second, first, true}}, attackDice(t))
	attackHand(s, p, []int{0, 0, 0, 2, 0})
	// A vacated origin cannot be used to move the first knight a second time.
	attackEndReject(t, s, []catanAttackMove{{first, destination, false}, {second, first, false}, {first, second, false}}, attackDice(t))
	if err := s.catanAttackResolveEnd([]catanAttackMove{{first, destination, true}, {second, first, true}}, attackDice(t)); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Players[p].Resources[3] != 0 || s.Catan.Attack.Knights[0].Edge != destination || s.Catan.Attack.Knights[1].Edge != first {
		t.Fatal("movement identity or separate payment")
	}
	assertAttackRestored(t, s)
}
func TestCatanAttackEndSurvivorsFightAgainAndProductionDiceStay(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, false)
		g, a := s.Catan, s.Catan.Attack
		p := s.Turn
		// All knights are owned by a different player, including a castle guard:
		// only the active player's castle knights are forced to move.
		owner := (p + 1) % n
		first, second, shared := -1, -1, -1
		for i, one := range a.Map.Coast {
			for _, two := range a.Map.Coast[i+1:] {
				for _, e := range g.Edges {
					if slices.Contains(e.Tiles, one) && slices.Contains(e.Tiles, two) {
						first, second, shared = one, two, e.ID
						break
					}
					if shared >= 0 {
						break
					}
				}
				if shared >= 0 {
					break
				}
			}
			if shared >= 0 {
				break
			}
		}
		if shared < 0 {
			t.Fatal("fixture")
		}
		a.Barbarians = make([]int, len(a.Barbarians))
		a.Barbarians[first] = 1
		a.Barbarians[second] = 1
		a.Knights = []catanAttackKnight{{owner, shared}}
		for _, tile := range []int{first, second} {
			for _, e := range g.Edges {
				if e.ID != shared && slices.Contains(e.Tiles, tile) {
					a.Knights = append(a.Knights, catanAttackKnight{owner, e.ID})
					break
				}
			}
		}
		occupied := map[int]bool{}
		for _, k := range a.Knights {
			occupied[k.Edge] = true
		}
		for side := 0; side < 6; side++ {
			e := catanFishingSide(g, a.Map.Castles[0], side)
			if !occupied[e] {
				a.Knights = append(a.Knights, catanAttackKnight{owner, e})
				break
			}
		}
		// Pick an orientation absent from BOTH knights on the first hex.
		used := map[int]bool{}
		for _, k := range a.Knights {
			if slices.Contains(g.Edges[k.Edge].Tiles, first) {
				used[a.Map.edgeOrientation(g, k.Edge)] = true
			}
		}
		safe := -1
		for v := 0; v < 3; v++ {
			if !used[v] {
				safe = []int{1, 2, 3}[v]
				break
			}
		}
		if safe < 0 {
			t.Fatal("fixture has all orientations")
		}
		g.Dice = []int{2, 6}
		g.RollID = 27
		if err := s.catanAttackResolveEnd(nil, attackDice(t, safe, 1)); err != nil {
			t.Fatal(err)
		}
		a = s.Catan.Attack
		if len(a.End.Battles) != 2 || a.End.Battles[0].Tile != first || a.End.Battles[1].Tile != second || a.Prisoners[owner] != 2 || !slices.Equal(s.Catan.Dice, []int{2, 6}) || s.Catan.RollID != 27 {
			t.Fatal("ordered survivor battles or independent dice", a.End)
		}
		assertAttackRestored(t, s)
	}
}
func TestCatanAttackEndRecordValidation(t *testing.T) {
	s := newAttackState(t, 3, false)
	a := s.Catan.Attack
	p := s.Turn
	tile := a.Map.Coast[3]
	a.Barbarians = make([]int, len(a.Barbarians))
	a.Barbarians[tile] = 1
	attackBattleKnights(s, tile, p, p)
	if err := s.catanAttackResolveEnd(nil, attackDice(t, 1)); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*catanAttack){
		func(a *catanAttack) { a.EndSequence++ }, func(a *catanAttack) { a.End.Player = 99 }, func(a *catanAttack) { a.End.Battles[0].Tile = -1 }, func(a *catanAttack) { a.End.Battles[0].LossDie = 7 }, func(a *catanAttack) { a.End.Battles[0].Prisoners[p]++ }, func(a *catanAttack) { a.End.Battles = append(a.End.Battles, a.End.Battles[0]) }, func(a *catanAttack) { a.End.Battles[0].Knights[0].Edge = -1 },
	} {
		bad := clone(*s)
		mutate(bad.Catan.Attack)
		if err := bad.validateCatanAttack(); err == nil {
			t.Fatal("bad end record accepted")
		}
	}
}

func TestCatanAttackEndNoBattleAtEqualStrengthAndLiberationWins(t *testing.T) {
	s := newAttackState(t, 3, false)
	g, a := s.Catan, s.Catan.Attack
	p := s.Turn
	tile := a.Map.Coast[3]
	a.Barbarians = make([]int, len(a.Barbarians))
	a.Barbarians[tile] = 2
	attackBattleKnights(s, tile, p, p)
	if err := s.catanAttackResolveEnd(nil, attackDice(t)); err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.Attack.End.Battles) != 0 || s.Catan.Attack.Barbarians[tile] != 2 {
		t.Fatal("equal strength incorrectly won")
	}
	s = newAttackState(t, 3, false)
	g, a = s.Catan, s.Catan.Attack
	p = s.Turn
	vertex := -1
	tile = -1
	for _, v := range g.Vertices {
		touch := []int{}
		for _, h := range g.Tiles {
			if slices.Contains(h.Vertices, v.ID) {
				touch = append(touch, h.ID)
			}
		}
		if len(touch) == 1 && slices.Contains(a.Map.Coast, touch[0]) {
			vertex = v.ID
			tile = touch[0]
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no coastal corner")
	}
	a.Barbarians = make([]int, len(a.Barbarians))
	a.Barbarians[tile] = 3
	a.Prisoners[p] = 22
	g.Vertices[vertex].Owner = p
	g.Vertices[vertex].Level = 1
	owner := (p + 1) % 3
	attackBattleKnights(s, tile, owner, owner, owner, owner)
	attackCoins(s, owner, a.GoldBank)
	s.catanScores()
	if g.Players[p].Score != 11 {
		t.Fatal("conquered settlement gave a point")
	}
	if err := s.catanAttackResolveEnd(nil, attackDice(t)); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || s.Catan.Players[p].Score != 12 || !slices.Equal(s.Winners, []int{p}) || len(s.Catan.Attack.Knights) != 4 || s.Catan.Attack.GoldBank != 0 {
		t.Fatal("liberation should win before post-game losses", s.Catan.Attack.End)
	}
	assertAttackRestored(t, s)
}
