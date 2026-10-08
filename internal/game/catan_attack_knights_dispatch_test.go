package game

import (
	"reflect"
	"slices"
	"testing"
)

func TestCatanAttackCityProgressDispatchAndInventory(t *testing.T) {
	for _, id := range []int{3, 8, 17, 19, 21} {
		s := attackCityCore(t, 3)
		g := s.Catan
		g.SetupStep = 6
		g.TurnSerial = 1
		s.Phase = "catan_turn"
		p := s.Turn
		edge := g.Attack.City.recruitEdges(g, p)[0]
		g.Attack.City.Knights = []catanAttackCityKnight{{Owner: p, Edge: edge, Strength: 1}}
		ckProgressGive(t, s, p, id)
		a := Action{Type: "catan_progress", Card: id}
		switch id {
		case 3:
			tiles := g.attackCityInventionTiles()
			a.Tile, a.Target = tiles[0], tiles[1]
		case 8:
			a.Targets = []int{edge}
		case 19:
			a.Tile = g.Attack.Map.Coast[0]
			g.Attack.Barbarians[a.Tile] = 1
		case 21:
			a.Tile = g.Attack.Map.Coast[0]
		}
		if err := s.Apply(p, a); err != nil {
			t.Fatal(id, err)
		}
		if len(s.Catan.CitiesKnights.Players[p].Progress) != 0 {
			t.Fatal("card not consumed", id)
		}
		if id == 8 && s.Catan.Attack.City.Knights[0].Strength != 2 {
			t.Fatal("smithing used vertex knight")
		}
		if id == 17 && !s.Catan.Attack.City.Knights[0].Active {
			t.Fatal("warlord used vertex knight")
		}
		before := clone(*s)
		if s.Apply(p, a) == nil || !reflect.DeepEqual(before, *s) {
			t.Fatal("card reused or failed mutation")
		}
	}
}
func TestCatanAttackCityTreasonPersistedResponses(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	g.SetupStep = 6
	s.Phase = "catan_turn"
	s.Turn = 0
	g.TurnSerial = 1
	g.Attack.City.Knights = []catanAttackCityKnight{{Owner: 1, Edge: 0, Strength: 3, Active: true}}
	ckProgressGive(t, s, 0, 22)
	if err := s.Apply(0, Action{Type: "catan_progress", Card: 22, Target: 1}); err != nil {
		t.Fatal(err)
	}
	attackCityPlanRestore(t, s)
	if err := s.validateAttackCityTreason(); err != nil {
		t.Fatal(err)
	}
	if s.CatanPendingActor() != 1 {
		t.Fatal("wrong removal actor")
	}
	id := s.Catan.Attack.City.Sequence
	remove := Action{Type: "catan_attack_city_treason", Prompt: id, Choice: "remove", Edge: 0}
	before := clone(*s)
	if s.catanAttackCityTreasonAction(0, remove) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("caster removed opponent knight")
	}
	if err := s.catanAttackCityTreasonAction(1, remove); err != nil {
		t.Fatal(err)
	}
	attackCityPlanRestore(t, s)
	if err := s.validateAttackCityTreason(); err != nil {
		t.Fatal(err)
	}
	if s.CatanPendingActor() != 0 {
		t.Fatal("wrong placement actor")
	}
	place := Action{Type: remove.Type, Prompt: id, Choice: "place", Edge: 1, Color: 2}
	before = clone(*s)
	if s.catanAttackCityTreasonAction(0, place) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("treason placed on other edge")
	}
	place.Edge = 0
	if err := s.catanAttackCityTreasonAction(0, place); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_turn" || s.Catan.Attack.City.Treason != nil || s.Catan.Attack.City.Knights[0].Owner != 0 || !s.Catan.Attack.City.Knights[0].Active {
		t.Fatal("treason continuation")
	}
	if err := s.validateCityProgressInventory(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanAttackCitySmithingPairsAndPrivateActions(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	g.SetupStep = 6
	g.TurnSerial = 1
	s.Turn = 0
	s.Phase = "catan_turn"
	g.CitiesKnights.Players[0].Improvements[CatanPolitics] = 3
	g.Attack.City.Knights = []catanAttackCityKnight{{Owner: 0, Edge: 0, Strength: 1}, {Owner: 0, Edge: 1, Strength: 1}, {Owner: 0, Edge: 2, Strength: 2}}
	// Upgrade 2->3 first frees the second rank-two piece for a later upgrade.
	options := g.smithingOptions(0)
	found := false
	for _, option := range options {
		if len(option) == 2 && option[0] == 2 && option[1] == 0 {
			found = true
		}
		if len(option) == 2 && option[0] == 0 && option[1] == 1 {
			t.Fatal("smithing exceeded rank supply")
		}
	}
	if !found {
		t.Fatal("ordered smithing pair missing", options)
	}
	attackCityHand(s, 0, []int{0, 0, 1, 1, 1, 0, 0, 0})
	for _, viewer := range []int{-1, 0, 1, 2} {
		v := s.catanAttackCityPlanView(viewer)
		if (v["choices"] != nil) != (viewer == 0) {
			t.Fatal("private knight choices leaked")
		}
	}
	ckProgressGive(t, s, 0, 8)
	if err := s.Apply(0, Action{Type: "catan_progress", Card: 8, Targets: []int{2, 0}}); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Attack.City.Knights[0].Strength != 2 || s.Catan.Attack.City.Knights[2].Strength != 3 {
		t.Fatal("smithing pair not applied")
	}
}

func TestCatanAttackCityBattleRecordRestoreGuards(t *testing.T) {
	s := attackCityBattleFixture(t)
	s.Catan.SetupStep = 6
	s.Catan.TurnSerial = 1
	s.Phase = "catan_turn"
	s.Catan.Attack.City.Sequence = 1
	tile := s.Catan.Attack.Map.Coast[0]
	die := s.Catan.Attack.Map.edgeOrientation(s.Catan, s.Catan.Attack.City.Knights[0].Edge)*2 + 1
	b, err := s.catanAttackCityBattle(tile, func() int { return die })
	if err != nil || b == nil {
		t.Fatal(err)
	}
	s.Catan.Attack.City.End = &catanAttackCityEnd{Player: s.Turn, Battles: []catanAttackCityBattle{*b}}
	if err = s.validateAttackCityEnd(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*catanAttackCityEnd){
		func(q *catanAttackCityEnd) { q.Player = -1 },
		func(q *catanAttackCityEnd) { q.Orders = []catanAttackCityOrder{{From: 0, To: 0, Retreat: -1}} },
		func(q *catanAttackCityEnd) { q.Battles[0].Strength[0]++ },
		func(q *catanAttackCityEnd) { q.Battles[0].Prisoners[0]++ },
		func(q *catanAttackCityEnd) { q.Battles[0].Knights[0].Active = false },
		func(q *catanAttackCityEnd) { q.Battles[0].Gold[0] = -3 },
		func(q *catanAttackCityEnd) { q.Battles[0].LossDie = 7 },
		func(q *catanAttackCityEnd) { q.Battles[0].Lost = nil; q.Battles[0].Downgraded = nil },
		func(q *catanAttackCityEnd) { q.Battles = append(q.Battles, q.Battles[0]) },
	} {
		bad := clone(*s)
		mutate(bad.Catan.Attack.City.End)
		if bad.validateAttackCityEnd() == nil {
			t.Fatal("corrupted battle accepted")
		}
	}
}

func TestCatanAttackCityImprovementMetropolisVictory(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackCityCore(t, n)
		g := s.Catan
		g.SetupStep = n * 2
		g.TurnSerial = 1
		s.Turn = 0
		s.Phase = "catan_turn"
		if g.Paired != nil {
			g.Paired.Primary = 0
			g.Paired.Second = false
		}
		vertex := g.Tiles[g.Attack.Map.Coast[0]].Vertices[0]
		g.Vertices[vertex].Owner = 0
		g.Vertices[vertex].Level = 2
		g.CitiesKnights.Players[0].Improvements[CatanScience] = 3
		g.Attack.Prisoners[0] = 27
		g.Attack.City.Issued = 27
		for _, tile := range g.Tiles {
			if slices.Contains(tile.Vertices, vertex) {
				g.Attack.Barbarians[tile.ID] = 0
			}
		}
		attackCityHand(s, 0, []int{0, 0, 0, 0, 0, 4, 0, 0})
		s.catanScores()
		if err := s.Apply(0, Action{Type: "catan_improvement", Color: CatanScience}); err != nil {
			t.Fatal(n, err)
		}
		if s.Phase != "catan_metropolis" || s.Finished {
			t.Fatal("improvement lost metropolis continuation")
		}
		attackCityPlanRestore(t, s)
		if err := s.Apply(0, Action{Type: "catan_metropolis", Vertex: vertex}); err != nil {
			t.Fatal(err)
		}
		if !s.Finished || s.Catan.Players[0].Score != 13 || !slices.Equal(s.Winners, []int{0}) {
			t.Fatal("metropolis did not win immediately", s.Phase, s.Catan.Players[0].Score)
		}
	}
}

func TestCatanAttackCityEndDiscardResumesKnightPlan(t *testing.T) {
	s := attackCityCore(t, 3)
	s.Catan.SetupStep = 6
	s.Catan.TurnSerial = 1
	s.Turn = 0
	s.Phase = "catan_turn"
	for _, card := range []int{1, 2, 3, 4, 5} {
		ckProgressGive(t, s, 0, card)
	}
	if err := s.Apply(0, Action{Type: "catan_end"}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != "catan_progress_end" || s.Catan.Attack.City.Plan != nil {
		t.Fatal("discard order")
	}
	attackCityPlanRestore(t, s)
	if err := s.Apply(0, Action{Type: "catan_progress_discard", Cards: []int{1}}); err != nil {
		t.Fatal(err)
	}
	if s.Phase != catanAttackCityMovePhase || s.Turn != 0 || s.Catan.Attack.City.Plan == nil {
		t.Fatal("discard skipped road knight end phase")
	}
	s.AutoCatanPending()
	if s.Catan.Attack.City.Plan != nil || s.Turn == 0 {
		t.Fatal("automatic confirmation did not advance")
	}
}
