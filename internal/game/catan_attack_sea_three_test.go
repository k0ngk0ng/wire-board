package game

import "testing"

func TestCatanAttackShoresThreePrintedInventory(t *testing.T) {
	g, m, e := newCatanAttackShoresThreeBoard()
	if e != nil {
		t.Fatal(e)
	}
	if len(g.Tiles) != 35 || g.Tiles[31].Resource != catanCastle || g.Tiles[28].Number != 2 || len(m.Coast) != 9 || m.ExtraNumbers[0].Number != 12 {
		t.Fatal("printed recipe")
	}
	coast, inner := [5]int{}, [5]int{}
	for _, id := range m.Coast {
		coast[g.Tiles[id].Resource]++
		if g.Seafarers.Islands[id] != g.Seafarers.StartIslands[0] {
			t.Fatal("landing outside mainland")
		}
	}
	for _, id := range []int{16, 21, 22, 27} {
		inner[g.Tiles[id].Resource]++
	}
	if coast != ([5]int{2, 2, 2, 2, 1}) || inner != ([5]int{1, 0, 1, 1, 1}) {
		t.Fatal("terrain inventories")
	}
	if g.Tiles[0].Number != 12 || g.Tiles[1].Resource != CatanGold || g.Tiles[34].Number != 10 {
		t.Fatal("outer islands changed")
	}
}

func TestCatanAttackShoresThreeDoubleNumber(t *testing.T) {
	g, m, e := newCatanAttackShoresThreeBoard()
	if e != nil {
		t.Fatal(e)
	}
	g.Attack = &catanAttack{Map: m, Barbarians: make([]int, len(g.Tiles))}
	for _, n := range []int{2, 12} {
		if !g.tileProduces(g.Tiles[28], n) || !m.landingNumber(g, 28, n) {
			t.Fatal("double number", n)
		}
	}
	if g.tileNumberWeight(g.Tiles[28]) != 2 || g.tileProduces(g.Tiles[28], 6) || m.landingNumber(g, 28, 6) {
		t.Fatal("extra probability")
	}
	g.Attack.Barbarians[28] = 3
	if g.tileProduces(g.Tiles[28], 2) || g.tileProduces(g.Tiles[28], 12) {
		t.Fatal("conquest produced")
	}
}

func TestCatanAttackShoresThreeNatural(t *testing.T) {
	for _, events := range []bool{false, true} {
		s, e := newCatanAttackShores(3)
		if e != nil {
			t.Fatal(e)
		}
		if events {
			if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
				t.Fatal(e)
			}
		}
		for step := 0; step < 16000 && !s.Finished; step++ {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Apply(p, a); e != nil {
				t.Fatal(step, e)
			}
			if step%137 == 0 {
				b := clone(*s)
				s = &b
				if e = s.validateCatanAttack(); e != nil {
					t.Fatal(e)
				}
			}
		}
		if !s.Finished {
			t.Fatal("unfinished", events)
		}
	}
}

func TestCatanAttackShoresThreeLandingFaces(t *testing.T) {
	s, e := newCatanAttackShores(3)
	if e != nil {
		t.Fatal(e)
	}
	finishAttackSetup(t, s)
	g := s.Catan
	clear(g.Attack.Barbarians)
	s.Phase = "catan_turn"
	rolls := [][2]int{{1, 1}, {6, 6}, {1, 2}}
	index := 0
	if e = s.catanAttackLanding(func() [2]int { r := rolls[index]; index++; return r }, func(int) int { return 0 }); e != nil {
		t.Fatal(e)
	}
	if g.Attack.Barbarians[28] != 2 || len(g.Attack.Landing.Rolls) != 3 {
		t.Fatal("distinct dice faces should each land on shared tile")
	}
	if e = s.validateCatanAttack(); e != nil {
		t.Fatal("landing restore", e)
	}
	b := clone(*s)
	b.Catan.Attack.Map.ExtraNumbers[0].Number = 11
	if b.validateCatanAttack() == nil {
		t.Fatal("corrupt double number")
	}
}
