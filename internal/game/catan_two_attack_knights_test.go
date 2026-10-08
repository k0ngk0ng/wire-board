package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanTwoAttackCityNaturalEngine(t *testing.T) {
	for _, events := range []bool{false, true} {
		t.Run(fmt.Sprint(events), func(t *testing.T) {
			s := attackCityCore(t, 2)
			if err := s.validateAttackCityState(); err != nil {
				t.Fatal(err)
			}
			if events {
				if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
					t.Fatal(err)
				}
			}
			phases := map[string]int{}
			for step := 0; step < 16000 && !s.Finished; step++ {
				p := twoFullActor(s)
				phases[s.Phase]++
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				if err = s.Apply(p, a); err != nil {
					t.Fatal(step, s.Phase, a, err)
				}
				if step%83 == 0 {
					for viewer := -1; viewer < 2; viewer++ {
						s.View(viewer)
					}
					attackCityPlanRestore(t, s)
				}
			}
			if !s.Finished {
				t.Fatal("unfinished", s.Round, s.Phase)
			}
			if phases["catan_two_build"] == 0 || phases[catanAttackCityMovePhase] == 0 {
				t.Fatal("missing paths", phases)
			}
			t.Log("rounds", s.Round, "phases", phases)
		})
	}
}

func twoAttackCityActionFixture(t *testing.T) *State {
	t.Helper()
	s := attackCityCore(t, 2)
	for step := 0; step < 100 && (s.Catan.setup() || s.Phase != "catan_turn"); step++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	if s.Phase != "catan_turn" {
		t.Fatal("no action phase")
	}
	attackCityHand(s, s.Turn, []int{2, 2, 4, 3, 4, 0, 0, 0})
	return s
}
func applyTwoAttack(t *testing.T, s *State, p int, a Action) {
	t.Helper()
	if err := s.Apply(p, a); err != nil {
		t.Fatal(a, err)
	}
	attackCityPlanRestore(t, s)
	if err := s.validateAttackCityState(); err != nil {
		t.Fatal(err)
	}
}
func TestCatanTwoAttackCityRecruitPromoteTokensAndIsolation(t *testing.T) {
	s := twoAttackCityActionFixture(t)
	p := s.Turn
	edge := s.Catan.Attack.City.recruitEdges(s.Catan, p)[0]
	applyTwoAttack(t, s, p, Action{Type: "catan_attack_knight_recruit", Edge: edge})
	if s.Phase != "catan_two_build" || s.Catan.Two.Pending.Kind != "knight" {
		t.Fatal("missing neutral recruit")
	}
	choices := s.Catan.twoNeutralChoices("knight")
	ch := choices[0]
	a := Action{Type: "catan_two_build", Target: 0, Vertex: -1, Edge: ch.Edge}
	before := clone(*s)
	if s.Apply(1-p, a) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("wrong actor mutated game")
	}
	applyTwoAttack(t, s, p, a)
	applyTwoAttack(t, s, p, Action{Type: "catan_attack_knight_promote", Edge: edge})
	if s.Catan.Two.Pending == nil || s.Catan.Two.Pending.Kind != "knight_promote" {
		t.Fatal("missing neutral promotion")
	}
	applyTwoAttack(t, s, p, a)
	neutral := s.Catan.Attack.City.Knights[s.Catan.Attack.City.at(ch.Edge)]
	if neutral.Strength != 2 || neutral.Active || neutral.Owner != -2 {
		t.Fatal(neutral)
	}
	before = clone(*s)
	if s.Apply(p, Action{Type: "catan_attack_knight_activate", Edge: ch.Edge}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("activated neutral")
	}
	tokens := s.Catan.Two.Tokens[p]
	applyTwoAttack(t, s, p, Action{Type: "catan_two_knight", Edge: edge})
	if s.Catan.Two.Tokens[p] != tokens+2 || s.Catan.Two.TokensIssued != 0 || s.Catan.Attack.City.at(edge) >= 0 {
		t.Fatal("exchange")
	}
	before = clone(*s)
	if s.Apply(p, Action{Type: "catan_two_knight", Edge: ch.Edge}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("repeat or neutral exchange")
	}
	ordinary, e := NewCatanTwoAttack(2, CatanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !ordinary.Catan.twoAttack() || ordinary.Catan.twoAttackKnights() || ordinary.Catan.CitiesKnights != nil || len(ordinary.Catan.Attack.Deck) == 0 {
		t.Fatal("ordinary changed")
	}
	old := twoKnightsFixture(t)
	if old.Catan.Attack != nil {
		t.Fatal("ordinary knights changed")
	}
}

func TestCatanTwoAttackCityNeutralRetreatAndTreason(t *testing.T) {
	s := twoAttackCityActionFixture(t)
	p := s.Turn
	g := s.Catan
	c := g.Attack.City
	from := c.recruitEdges(g, p)[0]
	to := -1
	for edge, d := range c.distances(g, from, 3) {
		if d == 2 && !g.Attack.castleEdge(g, edge) {
			to = edge
			break
		}
	}
	if to < 0 {
		t.Fatal("no displacement target")
	}
	c.Knights = []catanAttackCityKnight{{Owner: p, Edge: from, Strength: 3, Active: true}, {Owner: -2, Edge: to, Strength: 1}}
	applyTwoAttack(t, s, p, Action{Type: "catan_end"})
	id := s.Catan.Attack.City.Sequence
	applyTwoAttack(t, s, p, Action{Type: "catan_attack_city_move", Prompt: id, Choice: "displace", Edge: from, Target: to})
	q := s.Catan.Attack.City.Plan.Pending
	if q == nil || q.Player != p || s.CatanPendingActor() != p {
		t.Fatal("neutral actor", q)
	}
	before := clone(*s)
	a := Action{Type: "catan_attack_city_move", Prompt: id, Choice: "retreat", Edge: q.Targets[0]}
	if s.Apply(1-p, a) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("wrong retreat actor")
	}
	applyTwoAttack(t, s, p, a)
	applyTwoAttack(t, s, p, Action{Type: a.Type, Prompt: id, Choice: "confirm"})
	s = twoAttackCityActionFixture(t)
	p = s.Turn
	g = s.Catan
	c = g.Attack.City
	c.Knights = []catanAttackCityKnight{{Owner: -2, Edge: 0, Strength: 2}, {Owner: -2, Edge: 1, Strength: 1}}
	ckProgressGive(t, s, p, 22)
	applyTwoAttack(t, s, p, Action{Type: "catan_progress", Card: 22, Target: -2})
	id = s.Catan.Attack.City.Sequence
	if s.CatanPendingActor() != p {
		t.Fatal("neutral treason actor")
	}
	before = clone(*s)
	a = Action{Type: "catan_attack_city_treason", Prompt: id, Choice: "remove", Edge: 0}
	if s.Apply(p, a) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("removed stronger neutral")
	}
	a.Edge = 1
	applyTwoAttack(t, s, p, a)
	applyTwoAttack(t, s, p, Action{Type: a.Type, Prompt: id, Choice: "place", Edge: 1, Color: 1})
	if s.Phase != "catan_turn" || s.Catan.Two.Pending != nil {
		t.Fatal("treason recruited extra neutral")
	}
}
func TestCatanTwoAttackCityRejectCorruptNeutral(t *testing.T) {
	s := twoAttackCityActionFixture(t)
	s.Catan.Attack.City.Knights = []catanAttackCityKnight{{Owner: -2, Edge: 0, Strength: 1}}
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Attack.City.Knights[0].Active = true },
		func(s *State) { s.Catan.Attack.City.Knights[0].Strength = 3 },
		func(s *State) { s.Catan.Attack.TwoRules = CatanTwoAttackRules },
		func(s *State) { s.Catan.Two.TokensIssued++; s.Catan.Two.Bank++ },
	} {
		bad := clone(*s)
		mutate(&bad)
		if bad.validateAttackCityState() == nil {
			t.Fatal("accepted corrupt save")
		}
	}
}

func TestCatanTwoAttackCityFiniteTokensAndNeutralFallback(t *testing.T) {
	s := twoAttackCityActionFixture(t)
	g := s.Catan
	p := s.Turn
	c := g.Attack.City
	c.Knights = []catanAttackCityKnight{{Owner: p, Edge: 0, Strength: 2}}
	g.Two.Bank = 1
	g.Two.Tokens = []int{10, 9}
	before := clone(*s)
	if s.Apply(p, Action{Type: "catan_two_knight", Edge: 0}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("partial knight payout accepted")
	}
	if err := s.catanTwoEarn(p, 3); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Two.Bank != 0 || s.Catan.Two.TokensIssued != 0 {
		t.Fatal("finite supply expanded")
	}
	c.Knights = nil
	for _, owner := range catanTwoNeutralOwners {
		for i := 0; i < 2; i++ {
			edge := c.recruitEdges(g, owner)[0]
			c.Knights = append(c.Knights, catanAttackCityKnight{Owner: owner, Edge: edge, Strength: 1})
		}
	}
	choices := g.twoNeutralChoices("knight")
	if len(choices) == 0 {
		t.Fatal("missing fallback road")
	}
	ch := choices[0]
	count := len(c.Knights)
	if err := g.placeTwoNeutral("knight", ch); err != nil {
		t.Fatal(err)
	}
	if len(c.Knights) != count || g.Edges[ch.Edge].Owner != ch.Owner {
		t.Fatal("fallback did not build road")
	}
	if err := s.validateAttackCityState(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanTwoAttackCityNeutralNoBattleAndPrivateChoices(t *testing.T) {
	s := twoAttackCityActionFixture(t)
	g := s.Catan
	c := g.Attack.City
	p := s.Turn
	tile := g.Attack.Map.Coast[0]
	edge := -1
	for _, e := range g.Edges {
		if slices.Contains(e.Tiles, tile) {
			edge = e.ID
			break
		}
	}
	c.Knights = []catanAttackCityKnight{{Owner: -2, Edge: edge, Strength: 2}}
	g.Attack.Barbarians[tile] = 1
	b, err := s.catanAttackCityBattle(tile, func() int { return 1 })
	if err != nil || b != nil || s.Catan.Attack.Barbarians[tile] != 1 {
		t.Fatal("inactive neutral fought", b, err)
	}
	ckProgressGive(t, s, p, 22)
	applyTwoAttack(t, s, p, Action{Type: "catan_progress", Card: 22, Target: -2})
	for _, viewer := range []int{-1, p, 1 - p} {
		view := s.catanAttackCityPlanView(viewer)
		if (view["choices"] != nil) != (viewer == p) {
			t.Fatal("private neutral choices", viewer)
		}
	}
	s.AutoCatanPending()
	s.AutoCatanPending()
	if s.Phase != "catan_turn" || s.Catan.Two.Pending != nil {
		t.Fatal("auto treason continuation")
	}
}

func TestCatanTwoAttackCityEventProductionAndPublicGate(t *testing.T) {
	s := attackCityCore(t, 2)
	if err := s.EnableCatanEvents(CatanEventCatalogue); err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		applyTwoAttack(t, s, p, a)
	}
	twoReferenceTop(t, s, 22, 23) // Two printed eights are allowed with event cards.
	for draw := 0; draw < 2; draw++ {
		applyTwoAttack(t, s, s.Turn, Action{Type: "catan_roll"})
		for step := 0; s.Phase != "catan_roll" && s.Phase != "catan_turn"; step++ {
			if step > 40 {
				t.Fatal("production stuck", s.Phase)
			}
			p := twoFullActor(s)
			a, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			applyTwoAttack(t, s, p, a)
		}
		if len(s.Catan.Two.Rolls) != draw+1 || s.Catan.Two.Rolls[draw] != 8 {
			t.Fatal("draw count")
		}
		if draw == 0 && s.Phase != "catan_roll" || draw == 1 && s.Phase != "catan_turn" {
			t.Fatal("production continuation", s.Phase)
		}
	}
	if _, err := NewCatanAttackCitiesKnights(2); err == nil {
		t.Fatal("unverified public entry opened")
	}
}

func TestCatanTwoAttackCitySmithingTwoNeutralPromotions(t *testing.T) {
	s := twoAttackCityActionFixture(t)
	p := s.Turn
	s.Catan.Attack.City.Knights = []catanAttackCityKnight{{Owner: p, Edge: 0, Strength: 1}, {Owner: p, Edge: 1, Strength: 1}, {Owner: -2, Edge: 2, Strength: 1}, {Owner: -3, Edge: 3, Strength: 1}}
	ckProgressGive(t, s, p, 8)
	applyTwoAttack(t, s, p, Action{Type: "catan_progress", Card: 8, Targets: []int{0, 1}})
	for step := 0; step < 2; step++ {
		if s.Phase != "catan_two_build" || s.Catan.Two.Pending.Kind != "knight_promote" {
			t.Fatal("missing promotion", step)
		}
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		applyTwoAttack(t, s, p, a)
	}
	if s.Phase != "catan_turn" {
		t.Fatal("smithing did not finish")
	}
	for _, k := range s.Catan.Attack.City.Knights {
		if k.Strength != 2 {
			t.Fatal("missing rank", k)
		}
	}
}
