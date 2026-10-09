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
