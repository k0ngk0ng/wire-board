package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func attackCityPlanFixture(t *testing.T, displace bool) *State {
	t.Helper()
	s := attackCityCore(t, 3)
	g := s.Catan
	g.SetupStep = 6
	g.TurnSerial = 1
	s.Phase = "catan_turn"
	s.Turn = 0
	c := g.Attack.City
	from := c.recruitEdges(g, 0)[0]
	c.Knights = []catanAttackCityKnight{{Owner: 0, Edge: from, Strength: 3, Active: true}}
	if displace {
		to := -1
		for edge, d := range c.distances(g, from, 3) {
			if d == 2 && !g.Attack.castleEdge(g, edge) {
				to = edge
				break
			}
		}
		if to < 0 {
			t.Fatal("no target")
		}
		c.Knights = append(c.Knights, catanAttackCityKnight{Owner: 1, Edge: to, Strength: 1, Active: true})
	}
	if err := s.catanAttackCityBeginPlan(); err != nil {
		t.Fatal(err)
	}
	return s
}
func attackCityPlanRestore(t *testing.T, s *State) {
	t.Helper()
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if err = restored.Catan.Attack.City.validate(restored.Catan); err != nil {
		t.Fatal(err)
	}
	if err = restored.validateAttackCityTreason(); err != nil {
		t.Fatal(err)
	}
	if err = restored.validateAttackCityPlan(); err != nil {
		t.Fatal(err)
	}
	*s = restored
}
func attackCityPlanReject(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	before := clone(*s)
	if err := s.catanAttackCityPlanAction(p, a); err == nil {
		t.Fatal("accepted invalid command", a)
	}
	if !reflect.DeepEqual(before, *s) {
		t.Fatal("invalid command changed state")
	}
}
func TestCatanAttackCityPlanMoveUndoRestore(t *testing.T) {
	s := attackCityPlanFixture(t, false)
	g := s.Catan
	c := g.Attack.City
	from := c.Knights[0].Edge
	to := c.destinations(g, from)[0]
	id := c.Sequence
	a := Action{Type: "catan_attack_city_move", Prompt: id, Choice: "move", Edge: from, Target: to}
	attackCityPlanReject(t, s, 1, a)
	bad := a
	bad.Prompt--
	attackCityPlanReject(t, s, 0, bad)
	bad = a
	bad.Choice = "displace"
	attackCityPlanReject(t, s, 0, bad)
	if err := s.catanAttackCityPlanAction(0, a); err != nil {
		t.Fatal(err)
	}
	attackCityPlanRestore(t, s)
	if s.Catan.Attack.City.Knights[0].Active {
		t.Fatal("move did not deactivate")
	}
	attackCityPlanReject(t, s, 0, a)
	if err := s.catanAttackCityPlanAction(0, Action{Type: a.Type, Prompt: id, Choice: "undo"}); err != nil {
		t.Fatal(err)
	}
	if !s.Catan.Attack.City.Knights[0].Active || s.Catan.Attack.City.Knights[0].Edge != from {
		t.Fatal("undo lost activation")
	}
	attackCityPlanReject(t, s, 0, Action{Type: a.Type, Prompt: id, Choice: "confirm"})
	if err := s.catanAttackCityPlanAction(0, a); err != nil {
		t.Fatal(err)
	}
	if err := s.catanAttackCityPlanAction(0, Action{Type: a.Type, Prompt: id, Choice: "confirm"}); err != nil {
		t.Fatal(err)
	}
	if s.Turn == 0 || s.Catan.Attack.City.Plan != nil || s.Catan.Attack.City.End == nil {
		t.Fatal("end did not persist and advance")
	}
	attackCityPlanRestore(t, s)
	attackCityPlanReject(t, s, 0, a)
}
func TestCatanAttackCityPlanRetreatOwnerAndLock(t *testing.T) {
	s := attackCityPlanFixture(t, true)
	c := s.Catan.Attack.City
	id := c.Sequence
	from, to := c.Knights[0].Edge, c.Knights[1].Edge
	a := Action{Type: "catan_attack_city_move", Prompt: id, Choice: "displace", Edge: from, Target: to}
	if err := s.catanAttackCityPlanAction(0, a); err != nil {
		t.Fatal(err)
	}
	attackCityPlanRestore(t, s)
	q := s.Catan.Attack.City.Plan
	if s.Phase != catanAttackCityRetreatPhase || q.Pending.Player != 1 {
		t.Fatal("missing opponent response")
	}
	reply := Action{Type: a.Type, Prompt: id, Choice: "retreat", Edge: q.Pending.Targets[0]}
	attackCityPlanReject(t, s, 0, reply)
	attackCityPlanReject(t, s, 2, reply)
	bad := reply
	bad.Edge = to
	attackCityPlanReject(t, s, 1, bad)
	if err := s.catanAttackCityPlanAction(1, reply); err != nil {
		t.Fatal(err)
	}
	attackCityPlanRestore(t, s)
	if s.Catan.Attack.City.Plan.Locked != 1 || s.Phase != catanAttackCityMovePhase {
		t.Fatal("response not locked")
	}
	attackCityPlanReject(t, s, 0, Action{Type: a.Type, Prompt: id, Choice: "undo"})
	attackCityPlanReject(t, s, 1, reply)
	if err := s.catanAttackCityPlanAction(0, Action{Type: a.Type, Prompt: id, Choice: "confirm"}); err != nil {
		t.Fatal(err)
	}
	attackCityPlanRestore(t, s)
}
func TestCatanAttackCityPlanRejectsCorruptSave(t *testing.T) {
	s := attackCityPlanFixture(t, true)
	c := s.Catan.Attack.City
	if err := s.catanAttackCityPlanAction(0, Action{Type: "catan_attack_city_move", Prompt: c.Sequence, Choice: "displace", Edge: c.Knights[0].Edge, Target: c.Knights[1].Edge}); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Attack.City.Plan.Pending.Player = 2 },
		func(s *State) { s.Catan.Attack.City.Plan.Pending.Targets = []int{0} },
		func(s *State) { s.Catan.Attack.City.Plan.Pending.Knight.Strength = 3 },
		func(s *State) { s.Catan.Attack.City.Plan.ID++ },
		func(s *State) { s.Catan.Attack.City.Plan.Turn++ },
		func(s *State) { s.Catan.Attack.City.Plan.Locked = 1 },
		func(s *State) { s.Catan.Attack.City.Knights[0].Active = true },
		func(s *State) { s.Phase = catanAttackCityMovePhase },
		func(s *State) {
			s.Catan.Attack.City.Plan.Orders = append(s.Catan.Attack.City.Plan.Orders, catanAttackCityOrder{})
		},
		func(s *State) { s.Catan.Attack.City.Plan.Before = slices.Clone(s.Catan.Attack.City.Knights) },
	} {
		bad := clone(*s)
		mutate(&bad)
		if bad.validateAttackCityPlan() == nil {
			t.Fatal("corrupted pending accepted")
		}
	}
}

func TestCatanAttackCityPlanChoicesAndBot(t *testing.T) {
	for _, displace := range []bool{false, true} {
		s := attackCityPlanFixture(t, displace)
		c := s.Catan.Attack.City
		if displace {
			if err := s.catanAttackCityPlanAction(0, Action{Type: "catan_attack_city_move", Prompt: c.Sequence, Choice: "displace", Edge: c.Knights[0].Edge, Target: c.Knights[1].Edge}); err != nil {
				t.Fatal(err)
			}
		}
		first := s.CatanPendingActor()
		if first != 0 && !displace || first != 1 && displace {
			t.Fatal("wrong pending actor", first)
		}
		for step := 0; step < 20 && s.Catan.Attack.City.Plan != nil; step++ {
			actor := s.CatanPendingActor()
			for _, viewer := range []int{-1, 0, 1, 2} {
				view := s.catanAttackCityPlanView(viewer)
				if (view["choices"] != nil) != (viewer == actor) {
					t.Fatal("private choices leaked")
				}
				raw, _ := json.Marshal(view)
				var data map[string]any
				_ = json.Unmarshal(raw, &data)
				if _, ok := data["before"]; ok {
					t.Fatal("internal plan leaked")
				}
			}
			a, err := s.catanAttackCityPlanBot(actor)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.catanAttackCityPlanAction(actor, a); err != nil {
				t.Fatal(a, err)
			}
			attackCityPlanRestore(t, s)
		}
		if s.Catan.Attack.City.Plan != nil {
			t.Fatal("bot failed to finish end phase")
		}
	}
}

func TestCatanAttackCityNaturalEngine(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s := attackCityCore(t, n)
				if events {
					if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
						t.Fatal(err)
					}
				}
				for step := 0; step < 16000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, err := s.BotAction(p)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(p, a); err != nil {
						t.Fatal(step, s.Phase, a, err)
					}
					if step%83 == 0 {
						attackCityPlanRestore(t, s)
						if err = s.validateAttackCityState(); err != nil {
							t.Fatal(step, err)
						}
					}
				}
				if !s.Finished {
					t.Fatal("not finished", s.Round, s.Phase)
				}
			})
		}
	}

}

func TestCatanAttackCityBlockedCastleContinues(t *testing.T) {
	s := attackCityCore(t, 6)
	g := s.Catan
	g.SetupStep = 12
	g.Paired.Primary = 0
	g.Paired.Second = false
	g.TurnSerial = 1
	s.Turn = 0
	s.Phase = "catan_turn"
	c := g.Attack.City
	from := c.recruitEdges(g, 0)[0]
	c.Knights = []catanAttackCityKnight{{Owner: 0, Edge: from, Strength: 1}}
	exits := c.destinations(g, from)
	if len(exits) == 0 || len(exits) > 30 {
		t.Fatal("invalid blocked fixture", len(exits))
	}
	for i, edge := range exits {
		c.Knights = append(c.Knights, catanAttackCityKnight{Owner: 1 + i/6, Edge: edge, Strength: 1 + (i%6)/2})
	}
	if err := s.catanAttackCityBeginPlan(); err != nil {
		t.Fatal(err)
	}
	choices := s.catanAttackCityMoveChoices(0)
	if len(choices) != 1 || choices[0].Required {
		t.Fatal("blocked castle still mandatory", choices)
	}
	a, err := s.catanAttackCityPlanBot(0)
	if err != nil || a.Choice != "confirm" {
		t.Fatal("blocked bot", a, err)
	}
	if err = s.Apply(0, a); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Attack.City.Plan != nil || s.Turn == 0 {
		t.Fatal("blocked turn did not advance")
	}
	// Once an exit is free, castle departure is compulsory again.
	s.Turn = 0
	s.Catan.Paired.Primary = 0
	s.Catan.Paired.Second = false
	s.Phase = "catan_turn"
	s.Catan.Attack.City.Knights = slices.Delete(s.Catan.Attack.City.Knights, 1, 2)
	if err = s.catanAttackCityBeginPlan(); err != nil {
		t.Fatal(err)
	}
	choices = s.catanAttackCityMoveChoices(0)
	if len(choices) != 1 || !choices[0].Required {
		t.Fatal("free castle exit ignored")
	}
	attackCityPlanReject(t, s, 0, Action{Type: "catan_attack_city_move", Prompt: s.Catan.Attack.City.Sequence, Choice: "confirm"})
}

func TestCatanAttackCityAutoPending(t *testing.T) {
	for _, displace := range []bool{false, true} {
		s := attackCityPlanFixture(t, displace)
		c := s.Catan.Attack.City
		if displace {
			if err := s.catanAttackCityPlanAction(0, Action{Type: "catan_attack_city_move", Prompt: c.Sequence, Choice: "displace", Edge: c.Knights[0].Edge, Target: c.Knights[1].Edge}); err != nil {
				t.Fatal(err)
			}
		}
		for step := 0; step < 10 && s.Catan.Attack.City.Plan != nil; step++ {
			before := clone(*s)
			s.AutoCatanPending()
			if reflect.DeepEqual(before, *s) {
				t.Fatal("timeout did not advance", s.Phase)
			}
			attackCityPlanRestore(t, s)
		}
		if s.Catan.Attack.City.Plan != nil {
			t.Fatal("timeout plan stuck")
		}
	}
	s := attackCityCore(t, 3)
	s.Catan.SetupStep = 6
	s.Catan.TurnSerial = 1
	s.Turn = 0
	s.Phase = "catan_turn"
	s.Catan.Attack.City.Knights = []catanAttackCityKnight{{Owner: 1, Edge: 0, Strength: 2}}
	ckProgressGive(t, s, 0, 22)
	if err := s.Apply(0, Action{Type: "catan_progress", Card: 22, Target: 1}); err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 2; step++ {
		before := clone(*s)
		s.AutoCatanPending()
		if reflect.DeepEqual(before, *s) {
			t.Fatal("treason timeout stuck")
		}
		attackCityPlanRestore(t, s)
	}
	if s.Catan.Attack.City.Treason != nil || s.Phase != "catan_turn" {
		t.Fatal("treason timeout incomplete")
	}
}

func TestCatanAttackCityEventKnightStrength(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	g.Attack.City.Knights = []catanAttackCityKnight{
		{Owner: 0, Edge: 0, Strength: 1, Active: true},
		{Owner: 0, Edge: 1, Strength: 3, Active: false},
		{Owner: 1, Edge: 2, Strength: 2, Active: true},
	}
	if g.attackConflictLeader() != 1 || !slices.Equal(g.cardEventLeaders("tournament", 0), []int{1}) {
		t.Fatal("events ignored active road knight strength")
	}
	g.Attack.City.Knights[2].Strength = 1
	if g.attackConflictLeader() != -1 || !slices.Equal(g.cardEventLeaders("tournament", 1), []int{1, 0}) {
		t.Fatal("tied knight events")
	}
}

func TestCatanAttackCityLeavingReturnsRoadKnights(t *testing.T) {
	s := attackCityCore(t, 3)
	s.Catan.SetupStep = 6
	s.Catan.TurnSerial = 1
	s.Turn = 0
	s.Phase = "catan_turn"
	s.Catan.Attack.City.Knights = []catanAttackCityKnight{{Owner: 0, Edge: 0, Strength: 1}, {Owner: 1, Edge: 1, Strength: 1}}
	if err := s.EliminateCatan(0); err != nil {
		t.Fatal(err)
	}
	if len(s.Catan.Attack.City.Knights) != 1 || s.Catan.Attack.City.Knights[0].Owner != 1 {
		t.Fatal("departed road knights remain")
	}
	if err := s.validateAttackCityState(); err != nil {
		t.Fatal(err)
	}
	a, err := s.BotAction(s.Turn)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Apply(s.Turn, a); err != nil {
		t.Fatal(err)
	}
}
