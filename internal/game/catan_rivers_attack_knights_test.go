package game

import (
	"fmt"
	"slices"
	"testing"
)

func TestCatanRiversAttackKnightsNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, err := NewCatanRiversAttack(n, true)
				if err != nil {
					t.Fatal(err)
				}
				if events {
					if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 10000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%37 == 0 {
						riversAttackRestore(t, s)
						for viewer := -1; viewer < n; viewer++ {
							s.View(viewer)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round)
				}
				riversAttackRestore(t, s)
				t.Log("round", s.Round, "winner", s.Winners, "landings", s.Catan.Attack.Sequence)
			})
		}
	}
}

func TestCatanRiversAttackKnightsInventionRestore(t *testing.T) {
	for _, n := range []int{2, 6} {
		s, err := NewCatanRiversAttack(n, true)
		if err != nil {
			t.Fatal(err)
		}
		for step := 0; s.Catan.setup() && step < 100; step++ {
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e == nil {
				e = s.Apply(p, a)
			}
			if e != nil {
				t.Fatal(e)
			}
		}
		g := s.Catan
		tiles := g.attackCityInventionTiles()
		left, right := -1, -1
		for _, a := range tiles {
			for _, b := range tiles {
				if g.Tiles[a].Number != g.Tiles[b].Number {
					left, right = a, b
				}
			}
		}
		if left < 0 {
			t.Fatal("no invention pair")
		}
		if err := s.catanAttackCityInvention(left, right); err != nil {
			t.Fatal(err)
		}
		riversAttackRestore(t, s)
		if s.Catan.victoryTarget() != 13 {
			t.Fatal("wrong victory target")
		}
		bad := clone(*s)
		bad.Catan.Attack.City.NumberSwaps = nil
		if bad.validateAttackCityState() == nil {
			t.Fatal("unrecorded number changes accepted")
		}
		if s.Catan.canRiverPillageGold(s.Turn) {
			t.Fatal("ordinary city pillage leaked into attack combination")
		}
	}
}

func TestCatanRiversAttackKnightsDiplomacySharedGold(t *testing.T) {
	s, err := NewCatanRiversAttack(3, true)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; s.Catan.setup() && step < 100; step++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e == nil {
			e = s.Apply(p, a)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	s.Phase = "catan_turn"
	g, p := s.Catan, s.Turn
	for i := range g.Edges {
		g.Edges[i].Owner = -1
		g.Edges[i].Bridge = false
	}
	road := -1
	for _, e := range g.Edges {
		if !slices.Contains(g.Rivers.Map.Bridges, e.ID) && g.riverEdge(e.ID) && g.Vertices[e.A].Level == 0 && g.Vertices[e.B].Level == 0 {
			road = e.ID
			break
		}
	}
	if road < 0 {
		t.Fatal("no open riverbank road")
	}
	g.Edges[road].Owner = (p + 1) % 3
	g.Attack.GoldBank += g.Attack.Gold[p]
	g.Attack.Gold[p] = 0
	ckProgressGive(t, s, p, 16)
	helperReject(t, s, p, Action{Type: "catan_progress", Card: 16, Edge: road})
	g = s.Catan
	g.Attack.GoldBank--
	g.Attack.Gold[p]++
	bank := g.Attack.GoldBank
	helperApply(t, s, p, Action{Type: "catan_progress", Card: 16, Edge: road})
	g = s.Catan
	if g.Attack.Gold[p] != 0 || g.Attack.GoldBank != bank+1 || g.Edges[road].Owner != -1 || len(g.Rivers.Gold) != 0 {
		t.Fatal("diplomacy did not use shared ledger")
	}
	riversAttackRestore(t, s)
}
