package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func twoAttackGame(t *testing.T, events bool) *State {
	t.Helper()
	s, e := NewCatanTwoAttack(2, CatanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if events {
		if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
			t.Fatal(e)
		}
	}
	return s
}
func twoAttackRestore(t *testing.T, s *State) {
	t.Helper()
	raw, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var q State
	if e = json.Unmarshal(raw, &q); e != nil {
		t.Fatal(e)
	}
	if e = q.validateCatanAttack(); e != nil {
		t.Fatal("attack restore", s.Phase, e)
	}
	if e = q.validateCatanTwo(); e != nil {
		t.Fatal("two restore", s.Phase, e)
	}
	*s = q
}
func TestCatanTwoAttackNaturalGames(t *testing.T) {
	for _, events := range []bool{false, true} {
		t.Run(fmt.Sprint(events), func(t *testing.T) {
			s := twoAttackGame(t, events)
			seen := map[string]int{}
			for step := 0; step < 12000 && !s.Finished; step++ {
				actor := s.CatanPendingActor()
				if actor < 0 {
					actor = s.Turn
				}
				if s.Phase == "catan_discard" {
					for p, n := range s.Catan.DiscardDue {
						if n > 0 {
							actor = p
							break
						}
					}
				}
				a, e := s.BotAction(actor)
				if e != nil {
					t.Fatal(step, s.Phase, e, seen)
				}
				if e = s.Apply(actor, a); e != nil {
					t.Fatal(step, s.Phase, a, e, seen)
				}
				seen[a.Type]++
				if step%23 == 0 {
					twoAttackRestore(t, s)
				}
			}
			if !s.Finished {
				t.Fatal("stalled", s.Round, s.Phase, seen)
			}
			t.Log(s.Round, seen)
		})
	}
}

func twoAttackFixture(t *testing.T) *State {
	t.Helper()
	s := twoAttackGame(t, false)
	finishAttackSetup(t, s)
	s.Phase = "catan_turn"
	s.Catan.Two.Rolls = []int{4, 5}
	s.Catan.RollID = 2
	s.Catan.Dice = []int{2, 3}
	return s
}
func TestCatanTwoAttackFirstKnightAndMovement(t *testing.T) {
	for _, card := range []string{"knighthood", "swift_knight"} {
		t.Run(card, func(t *testing.T) {
			s := twoAttackFixture(t)
			p := s.Turn
			buyAttackCard(t, s, card)
			first, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			helperApply(t, s, p, first)
			if !s.Catan.Attack.Pending.Neutral || len(s.Catan.Attack.Knights) != 1 {
				t.Fatal("neutral recruitment missing")
			}
			twoAttackRestore(t, s)
			a := s.Catan.Attack
			targets := a.recruitEdges(s.Catan, catanAttackNeutral, card)
			if len(targets) == 0 {
				t.Fatal("missing neutral placement")
			}
			helperReject(t, s, 1-p, Action{Type: "catan_attack_card", Prompt: a.Pending.ID, Choice: card, Edge: targets[0]})
			helperApply(t, s, p, Action{Type: "catan_attack_card", Prompt: a.Pending.ID, Choice: card, Edge: targets[0]})
			twoAttackRestore(t, s)
			if s.Catan.Attack.Pending != nil || len(s.Catan.Attack.Knights) != 2 || s.Catan.Attack.Knights[1].Player != catanAttackNeutral {
				t.Fatal("wrong neutral owner")
			}
			buyAttackCard(t, s, card)
			next, e := s.BotAction(p)
			if e != nil {
				t.Fatal(e)
			}
			helperApply(t, s, p, next)
			if s.Catan.Attack.Pending != nil || len(s.Catan.Attack.Knights) != 3 {
				t.Fatal("repeated neutral recruitment")
			}
			helperApply(t, s, p, Action{Type: "catan_end"})
			if s.Catan.Attack.EndPlan == nil {
				t.Fatal("no knight plan")
			}
			for step := 0; step < 15 && s.Catan.Attack.EndPlan != nil; step++ {
				act, e := s.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				helperApply(t, s, p, act)
				twoAttackRestore(t, s)
			}
			if s.Catan.Attack.EndPlan != nil {
				t.Fatal("move plan stalled")
			}
		})
	}
}
func TestCatanTwoAttackNeutralContestCompensationAndImmunity(t *testing.T) {
	s := twoAttackFixture(t)
	g := s.Catan
	a := g.Attack
	p := s.Turn
	tile, one, two, loss := -1, -1, -1, 0
	for _, id := range a.Map.Coast {
		for _, e := range g.Edges {
			for _, f := range g.Edges {
				if e.ID != f.ID && !a.castleEdge(g, e.ID) && !a.castleEdge(g, f.ID) && slices.Contains(e.Tiles, id) && slices.Contains(f.Tiles, id) && a.Map.edgeOrientation(g, e.ID) == a.Map.edgeOrientation(g, f.ID) {
					tile, one, two = id, e.ID, f.ID
					break
				}
			}
			if tile >= 0 {
				break
			}
		}
		if tile >= 0 {
			break
		}
	}
	if tile < 0 {
		t.Fatal("fixture lacks opposite coastal sides")
	}
	for d := 1; d <= 6; d++ {
		if catanAttackLossOrientation(d) == a.Map.edgeOrientation(g, one) {
			loss = d
			break
		}
	}
	a.Barbarians = make([]int, len(g.Tiles))
	a.Barbarians[tile] = 1
	a.Knights = []catanAttackKnight{{Player: p, Edge: one}, {Player: catanAttackNeutral, Edge: two}}
	a.GoldBank = 0
	a.Gold = []int{100, 0}
	g.Two.Bank = 0
	g.Two.Tokens = []int{10, 10}
	g.Two.TokensIssued = 0
	beforeGold := a.Gold[p]
	dice := []int{3, 2, loss}
	calls := 0
	if e := s.catanAttackResolveEnd(nil, func() int {
		if calls >= len(dice) {
			t.Fatal("neutral consumed real die")
		}
		d := dice[calls]
		calls++
		return d
	}); e != nil {
		t.Fatal(e)
	}
	twoAttackRestore(t, s)
	a = s.Catan.Attack
	g = s.Catan
	if calls != 3 || a.NeutralPrisoners != 1 || a.Prisoners[p] != 0 || a.Gold[p] != beforeGold+4 || a.GoldIssued != 4 || g.Two.Tokens[p] != 12 || g.Two.TokensIssued != 2 {
		t.Fatal("wrong neutral/ledger compensation", calls, a, g.Two)
	}
	if len(a.Knights) != 1 || a.Knights[0].Player != catanAttackNeutral {
		t.Fatal("neutral lost or real casualty remained")
	}
	battle := a.End.Battles[0]
	if len(battle.Contests) != 2 || battle.Tokens[p] != 2 || len(battle.Lost) != 1 {
		t.Fatal("battle record")
	}
	for _, r := range battle.Contests {
		for i, seat := range r.Players {
			if seat == 2 && r.Dice[i] != 3 {
				t.Fatal("neutral die")
			}
		}
	}
}
func TestCatanTwoAttackMoveBarbarianWindowAndRollback(t *testing.T) {
	for rolls := 0; rolls <= 2; rolls++ {
		t.Run(fmt.Sprint(rolls), func(t *testing.T) {
			s := twoAttackFixture(t)
			g := s.Catan
			p := s.Turn
			g.Two.Rolls = []int{4, 5}[:rolls]
			if rolls < 2 {
				s.Phase = "catan_roll"
			}
			a := g.Attack
			from, to := a.Map.Coast[0], a.Map.Coast[1]
			a.Barbarians = make([]int, len(g.Tiles))
			a.Barbarians[from] = 3
			a.Barbarians[to] = 2
			s.catanScores()
			cost := g.twoTokenCost(p)
			tokens := g.Two.Tokens[p]
			helperReject(t, s, 1-p, Action{Type: "catan_two_robber", Card: from, Tile: to})
			helperReject(t, s, p, Action{Type: "catan_two_robber", Card: from, Tile: from})
			helperApply(t, s, p, Action{Type: "catan_two_robber", Card: from, Tile: to})
			twoAttackRestore(t, s)
			g = s.Catan
			if g.Attack.Barbarians[from] != 2 || g.Attack.Barbarians[to] != 3 || g.Robber != -1 || g.Two.Tokens[p] != tokens-cost || !g.Two.Spent || len(g.Two.Rolls) != rolls {
				t.Fatal("barbarian movement state")
			}
			helperReject(t, s, p, Action{Type: "catan_two_robber", Card: to, Tile: from})
		})
	}
}

func TestCatanTwoAttackBuildingLandingCounts(t *testing.T) {
	for _, kind := range []string{"city", "settlement-road", "settlement-village"} {
		t.Run(kind, func(t *testing.T) {
			s := twoAttackFixture(t)
			g, p := s.Catan, s.Turn
			if g.Attack.Sequence != 0 {
				t.Fatal("setup triggered landing")
			}
			attackHand(s, p, []int{5, 5, 5, 5, 5})
			vertex := -1
			if kind == "city" {
				for _, v := range g.Vertices {
					if v.Owner == p && v.Level == 1 {
						vertex = v.ID
						break
					}
				}
				helperApply(t, s, p, Action{Type: "catan_city", Vertex: vertex})
				if s.Catan.Attack.Sequence != 1 {
					t.Fatal("city must trigger one landing")
				}
				twoAttackRestore(t, s)
				return
			}
			for _, v := range g.Vertices {
				if !g.canSettlement(p, v.ID, true) {
					continue
				}
				for _, e := range g.Edges {
					if e.Owner == -1 && (e.A == v.ID || e.B == v.ID) {
						g.Edges[e.ID].Owner = p
						vertex = v.ID
						break
					}
				}
				if vertex >= 0 {
					break
				}
			}
			if vertex < 0 {
				t.Fatal("no real site")
			}
			if kind == "settlement-village" {
				found := false
				for _, v := range g.Vertices {
					if v.ID == vertex || !g.canSettlement(-2, v.ID, true) {
						continue
					}
					// Keep the future real settlement's distance exclusion clear.
					adjacent := false
					for _, e := range g.Edges {
						if e.A == vertex && e.B == v.ID || e.B == vertex && e.A == v.ID {
							adjacent = true
						}
					}
					if adjacent {
						continue
					}
					for _, e := range g.Edges {
						if e.Owner == -1 && (e.A == v.ID || e.B == v.ID) {
							g.Edges[e.ID].Owner = -2
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				if !found {
					t.Fatal("no neutral site")
				}
			}
			helperApply(t, s, p, Action{Type: "catan_settlement", Vertex: vertex})
			if s.Catan.Attack.Sequence != 0 || !s.Catan.Attack.TwoLanding || s.Phase != "catan_two_build" {
				t.Fatal("landing was not deferred")
			}
			twoAttackRestore(t, s)
			choices := s.Catan.twoNeutralChoices("settlement")
			if len(choices) == 0 {
				t.Fatal("no neutral choices")
			}
			c := choices[0]
			if (c.Vertex >= 0) != (kind == "settlement-village") {
				t.Fatal("wrong fallback", c)
			}
			target := 0
			if c.Owner == -3 {
				target = 1
			}
			action := Action{Type: "catan_two_build", Prompt: s.Catan.Two.Sequence, Target: target, Edge: c.Edge, Vertex: c.Vertex}
			helperReject(t, s, 1-p, action)
			helperApply(t, s, p, action)
			want := 1
			if c.Vertex >= 0 {
				want = 2
			}
			if s.Catan.Attack.Sequence != want || s.Catan.Attack.TwoLanding {
				t.Fatal("wrong landing count", s.Catan.Attack.Sequence, want)
			}
			twoAttackRestore(t, s)
		})
	}
}

func TestCatanTwoAttackMovementOrderAndConflict(t *testing.T) {
	s := twoAttackFixture(t)
	g, p := s.Catan, s.Turn
	edges := []int{}
	for _, e := range g.Edges {
		if !g.Attack.castleEdge(g, e.ID) {
			edges = append(edges, e.ID)
		}
	}
	g.Attack.Knights = []catanAttackKnight{{Player: p, Edge: edges[0]}, {Player: catanAttackNeutral, Edge: edges[len(edges)/2]}}
	if g.attackConflictLeader() != -1 {
		t.Fatal("neutral tied strength must prevent sole leader")
	}
	g.Attack.Knights = append(g.Attack.Knights, catanAttackKnight{Player: p, Edge: edges[len(edges)-1]})
	if g.attackConflictLeader() != p {
		t.Fatal("two knights must beat one neutral")
	}
	helperApply(t, s, p, Action{Type: "catan_end"})
	preview, err := s.catanAttackPlanPreview()
	if err != nil {
		t.Fatal(err)
	}
	choices, _ := s.catanAttackPlanChoices(preview)
	own, neutral := catanAttackMoveChoice{}, catanAttackMoveChoice{}
	for _, c := range choices {
		if len(c.Normal) == 0 {
			continue
		}
		if c.From == edges[len(edges)/2] {
			neutral = c
		} else {
			own = c
		}
	}
	if len(own.Normal) == 0 || len(neutral.Normal) == 0 {
		t.Fatal("no move choices")
	}
	id := s.Catan.Attack.EndPlan.ID
	helperApply(t, s, p, Action{Type: "catan_attack_move", Prompt: id, Choice: "normal", Edge: neutral.From, Target: neutral.Normal[0]})
	helperReject(t, s, p, Action{Type: "catan_attack_move", Prompt: id, Choice: "normal", Edge: own.From, Target: own.Normal[0]})
	twoAttackRestore(t, s)
	helperApply(t, s, p, Action{Type: "catan_attack_move", Prompt: id, Choice: "undo"})
	helperApply(t, s, p, Action{Type: "catan_attack_move", Prompt: id, Choice: "normal", Edge: own.From, Target: own.Normal[0]})
	twoAttackRestore(t, s)
}

func TestCatanTwoAttackRejectCorruptStateAndIsolation(t *testing.T) {
	s := twoAttackFixture(t)
	for _, mutate := range []func(*State){
		func(q *State) { q.Catan.Attack.TwoRules = "unknown" },
		func(q *State) { q.Catan.Two = nil },
		func(q *State) { q.Catan.Attack.NeutralPrisoners = -1 },
		func(q *State) { q.Catan.Attack.NeutralPrisoners = 99 },
		func(q *State) { q.Catan.Attack.TwoLanding = true },
		func(q *State) { q.Catan.Attack.Knights = []catanAttackKnight{{Player: -3, Edge: 0}} },
		func(q *State) {
			q.Catan.Attack.Knights = []catanAttackKnight{{Player: -2, Edge: 0}, {Player: -2, Edge: 1}}
		},
		func(q *State) { q.Catan.Attack.Knights = []catanAttackKnight{{Player: 0, Edge: 0}} },
	} {
		q := clone(*s)
		mutate(&q)
		if q.validateCatanAttack() == nil {
			t.Fatal("corrupt attack accepted")
		}
	}
	for _, n := range []int{3, 4, 5, 6} {
		q, e := NewCatanAttack(n)
		if e != nil {
			t.Fatal(e)
		}
		q.Catan.Attack.NeutralPrisoners = 1
		if q.validateCatanAttack() == nil {
			t.Fatal("neutral leaked to multiplayer")
		}
	}
	for _, opt := range []CatanOptions{{Helpers: true}, {FiveSix: true}} {
		if _, e := NewCatanTwoAttack(2, opt); e == nil {
			t.Fatal("invalid combination")
		}
	}
}
