package game

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestCatanRiversTransportBridgeMovement(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		g, m, r, err := newCatanRiversTransportBoard(n)
		if err != nil {
			t.Fatal(err)
		}
		if n == 2 {
			g.Two = &CatanTwo{Rules: CatanTwoRules}
		}
		edge := r.Bridges[0]
		position := g.Edges[edge].A
		barbarians := m.Barbarians
		for i := range barbarians {
			if barbarians[i] == edge {
				for _, e := range g.Edges {
					if e.ID != edge && !slices.Contains(barbarians[:], e.ID) {
						barbarians[i] = e.ID
						break
					}
				}
			}
		}
		gold := make([]int, n)
		gold[0] = 10
		for _, tc := range []struct {
			owner, mp, toll, pay int
			bridge               bool
		}{{-1, 3, 0, -1, false}, {0, 1, 0, -1, true}, {1, 1, 2, 1, true}} {
			e := &g.Edges[edge]
			e.Owner, e.Bridge = tc.owner, tc.bridge
			q, err := newCatanTransportTravel(g, m, 0, position, 0)
			if err != nil {
				t.Fatal(err)
			}
			step, err := q.quote(g, m, barbarians, gold, edge)
			if err != nil {
				t.Fatal(err)
			}
			if step.MP != tc.mp || step.Toll != tc.toll || step.Pay != tc.pay {
				t.Fatal("wrong bridge quotation", step)
			}
			before := slices.Clone(gold)
			if _, err = q.move(g, m, barbarians, gold, edge); err != nil {
				t.Fatal(err)
			}
			if gold[0] != before[0]-tc.toll || tc.pay >= 0 && gold[tc.pay] != before[tc.pay]+tc.toll {
				t.Fatal("wrong toll transfer")
			}
			if err = m.validate(g); err != nil {
				t.Fatal("legal bridge rejected", err)
			}
		}
		// A barbarian adds two movement points, including to a bridge. Without
		// enough movement or gold, neither the wagon nor either balance changes.
		g.Edges[edge].Owner, g.Edges[edge].Bridge = 1, true
		barbarians[0] = edge
		q, _ := newCatanTransportTravel(g, m, 0, position, 0)
		step, err := q.quote(g, m, barbarians, gold, edge)
		if err != nil || step.MP != 3 {
			t.Fatal("barbarian bridge cost", step, err)
		}
		q.Points = 2
		before, _ := json.Marshal(struct {
			Q *catanTransportTravel
			G []int
		}{q, gold})
		if _, err = q.move(g, m, barbarians, gold, edge); err == nil {
			t.Fatal("insufficient movement accepted")
		}
		after, _ := json.Marshal(struct {
			Q *catanTransportTravel
			G []int
		}{q, gold})
		if string(before) != string(after) {
			t.Fatal("rejected move mutated state")
		}
		q.Points = 4
		gold[0] = 1
		if _, err = q.move(g, m, barbarians, gold, edge); err == nil {
			t.Fatal("insufficient toll accepted")
		}
	}
}

func TestCatanRiversTransportNeutralBridgeTolls(t *testing.T) {
	g, m, r, err := newCatanRiversTransportBoard(2)
	if err != nil {
		t.Fatal(err)
	}
	g.Two = &CatanTwo{Rules: CatanTwoRules}
	edge := r.Bridges[0]
	g.Edges[edge].Owner = -2
	g.Edges[edge].Bridge = true
	gold := []int{10, 0}
	barbarians := m.Barbarians
	for i := range barbarians {
		if barbarians[i] == edge {
			for _, e := range g.Edges {
				if e.ID != edge && !slices.Contains(barbarians[:], e.ID) {
					barbarians[i] = e.ID
					break
				}
			}
		}
	}
	q, _ := newCatanTransportTravel(g, m, 0, g.Edges[edge].A, 0)
	bank := 0
	for range 4 {
		step, e := q.move(g, m, barbarians, gold, edge)
		if e != nil {
			t.Fatal(e)
		}
		bank += step.Bank
		if step.MP != 1 || step.Toll != 2 || step.Bank != 1 || !step.Neutral {
			t.Fatal(step)
		}
	}
	if gold[0] != 2 || gold[1] != 4 || bank != 4 || q.NeutralTolls != 8 {
		t.Fatal("neutral split", gold, bank, q)
	}
	if err = q.validate(g, m, barbarians, gold); err != nil {
		t.Fatal("valid bridge toll history rejected", err)
	}
}
