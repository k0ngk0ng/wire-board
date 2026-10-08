package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestCatanFishingAttackKnightsNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s, e := newCatanFishingAttack(n, true)
				if e != nil {
					t.Fatal(e)
				}
				if events {
					if e = s.EnableCatanEvents(CatanEventCatalogue); e != nil {
						t.Fatal(e)
					}
				}
				actions := map[string]int{}
				for step := 0; step < 20000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						t.Fatal(step, s.Phase, e)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, a, e)
					}
					actions[a.Type+":"+a.Choice]++
					if step%83 == 0 {
						fishingAttackRestore(t, s)
						for viewer := -1; viewer < n; viewer++ {
							s.View(viewer)
						}
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round, s.Phase)
				}
				t.Log("rounds", s.Round, "fish moves", actions["catan_attack_city_move:fish"], "progress", actions["catan_fish_progress:"])
			})
		}
	}
}
func fishingAttackCityFixture(t *testing.T, n int) *State {
	t.Helper()
	s, e := newCatanFishingAttack(n, true)
	if e != nil {
		t.Fatal(e)
	}
	for s.Catan.setup() || s.Phase != "catan_turn" {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e != nil {
			t.Fatal(e)
		}
		if e = s.Apply(p, a); e != nil {
			t.Fatal(e)
		}
	}
	return s
}
func TestCatanFishingAttackKnightsMovePaymentUndoAndPrivacy(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := fishingAttackCityFixture(t, n)
			g := s.Catan
			p := s.Turn
			c := g.Attack.City
			fishingAttackHand(t, s, p, 4)
			from := catanFishingSide(g, g.Attack.Map.Castles[0], 0)
			c.Knights = []catanAttackCityKnight{{Owner: p, Edge: from, Strength: 1}}
			target := -1
			for _, edge := range c.fishDestinations(g, p, from) {
				if !slices.Contains(c.destinations(g, from), edge) {
					target = edge
					break
				}
			}
			if target < 0 {
				t.Fatal("no long target")
			}
			if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
				t.Fatal(e)
			}
			initialHand := slices.Clone(s.Catan.Fishing.Tokens.Hands[p])
			before, _ := json.Marshal(s)
			q := s.Catan.Attack.City.Plan
			cost := s.Catan.fishActionCost(p, "catan_fish_knight")
			payment := s.Catan.fishPayment(p, cost)
			action := Action{Type: "catan_attack_city_move", Choice: "fish", Prompt: q.ID, Edge: from, Target: target, Tokens: payment}
			attackReject(t, s, (p+1)%n, action)
			bad := action
			bad.Tokens = nil
			attackReject(t, s, p, bad)
			bad = action
			bad.Choice = "move"
			attackReject(t, s, p, bad)
			if e := s.Apply(p, action); e != nil {
				t.Fatal(e)
			}
			c = s.Catan.Attack.City
			if c.Knights[0].Active || c.Knights[0].Edge != target {
				t.Fatal("fish activated/lost knight")
			}
			if !slices.Equal(s.Catan.Fishing.Tokens.Hands[p], initialHand) {
				t.Fatal("draft spent fish")
			}
			for viewer := -1; viewer < n; viewer++ {
				pub := s.catanAttackCityPlanView(viewer)
				raw, _ := json.Marshal(pub["plan"])
				if strings.Contains(string(raw), "tokens") {
					t.Fatal("draft payment leaked", string(raw))
				}
				if viewer != p && pub["choices"] != nil {
					t.Fatal("private choices leaked")
				}
			}
			fishingAttackRestore(t, s)
			if e := s.Apply(p, Action{Type: "catan_attack_city_move", Choice: "undo", Prompt: q.ID}); e != nil {
				t.Fatal(e)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("undo changed inventory/knights")
			}
			if e := s.Apply(p, action); e != nil {
				t.Fatal(e)
			}
			if e := s.Apply(p, Action{Type: "catan_attack_city_move", Choice: "confirm", Prompt: q.ID}); e != nil {
				t.Fatal(e)
			}
			if len(s.Catan.Fishing.Tokens.Hands[p]) != len(initialHand)-len(payment) {
				t.Fatal("wrong committed payment")
			}
			fishingAttackRestore(t, s)
			for viewer := -1; viewer < n; viewer++ {
				pub := s.catanAttackCityPlanView(viewer)
				raw, _ := json.Marshal(pub["end"])
				if strings.Contains(string(raw), "tokens") {
					t.Fatal("payment identities leaked in record")
				}
			}
		})
	}
}
func TestCatanFishingAttackKnightsInventionAndProgress(t *testing.T) {
	for _, n := range []int{2, 6} {
		s := fishingAttackCityFixture(t, n)
		g := s.Catan
		p := s.Turn
		choices := g.attackCityInventionTiles()
		if len(choices) < 2 {
			t.Fatal("no invention")
		}
		if e := s.catanAttackCityInvention(choices[0], choices[1]); e != nil {
			t.Fatal(e)
		}
		fishingAttackRestore(t, s)
		damaged := clone(*s)
		damaged.Catan.Attack.City.NumberSwaps = nil
		if damaged.Catan.Tiles[choices[0]].Number != damaged.Catan.Tiles[choices[1]].Number && damaged.Catan.validateFishing() == nil {
			t.Fatal("missing swap receipt accepted")
		}
		if s.Catan.victoryTargetFor(p) != 13 {
			t.Fatal("wrong target")
		}
		fishingAttackHand(t, s, p, 7)
		if e := s.Apply(p, Action{Type: "catan_fish_progress", Color: 0, Tokens: s.Catan.fishPayment(p, s.Catan.fishActionCost(p, "catan_fish_progress"))}); e != nil {
			t.Fatal(e)
		}
		fishingAttackRestore(t, s)
		if n == 2 && (sum(s.Catan.Two.Tokens) != 0 || s.catanTwoCanExchangeKnight(p)) {
			t.Fatal("trade token exchange active")
		}
	}
}

func TestCatanFishingAttackKnightsRetreatLocksReservedFish(t *testing.T) {
	s := fishingAttackCityFixture(t, 3)
	g := s.Catan
	p := s.Turn
	other := (p + 1) % 3
	c := g.Attack.City
	fishingAttackHand(t, s, p, 4)
	// One inactive knight makes a paid move; another displaces an opponent.
	first := catanFishingSide(g, g.Attack.Map.Castles[0], 0)
	c.Knights = []catanAttackCityKnight{{Owner: p, Edge: first, Strength: 1}}
	chosen := false
	paidTarget, second, opponent := -1, -1, -1
	for _, to := range c.fishDestinations(g, p, first) {
		for _, edge := range g.Edges {
			if edge.ID == first || edge.ID == to {
				continue
			}
			trial := clone(*g)
			tc := trial.Attack.City
			tc.Knights[0].Edge = to
			tc.Knights = append(tc.Knights, catanAttackCityKnight{Owner: p, Edge: edge.ID, Strength: 3, Active: true})
			for dest, d := range tc.distances(&trial, edge.ID, 3) {
				if d < 1 || dest == to || dest == first || trial.Attack.castleEdge(&trial, dest) {
					continue
				}
				tc.Knights = append(tc.Knights, catanAttackCityKnight{Owner: other, Edge: dest, Strength: 1, Active: true})
				if slices.Contains(tc.displacementTargets(&trial, edge.ID), dest) {
					q, e := tc.beginDisplacement(&trial, p, edge.ID, dest)
					if e == nil && q != nil && len(q.Targets) > 0 {
						paidTarget, second, opponent, chosen = to, edge.ID, dest, true
						break
					}
				}
				tc.Knights = tc.Knights[:2]
			}
			if chosen {
				break
			}
		}
		if chosen {
			break
		}
	}
	if !chosen {
		t.Fatal("no retreat fixture")
	}
	c.Knights = append(c.Knights, catanAttackCityKnight{Owner: p, Edge: second, Strength: 3, Active: true}, catanAttackCityKnight{Owner: other, Edge: opponent, Strength: 1, Active: true})
	if e := s.Apply(p, Action{Type: "catan_end"}); e != nil {
		t.Fatal(e)
	}
	q := s.Catan.Attack.City.Plan
	ids := s.Catan.fishPayment(p, 2)
	initial := len(s.Catan.Fishing.Tokens.Hands[p])
	if e := s.Apply(p, Action{Type: "catan_attack_city_move", Choice: "fish", Prompt: q.ID, Edge: first, Target: paidTarget, Tokens: ids}); e != nil {
		t.Fatal(e)
	}
	attackReject(t, s, p, Action{Type: "catan_attack_city_move", Choice: "fish", Prompt: q.ID, Edge: second, Target: opponent, Tokens: ids})
	if e := s.Apply(p, Action{Type: "catan_attack_city_move", Choice: "displace", Prompt: q.ID, Edge: second, Target: opponent}); e != nil {
		t.Fatal(e)
	}
	fishingAttackRestore(t, s)
	q = s.Catan.Attack.City.Plan
	if q.Pending == nil || q.Pending.Player != other {
		t.Fatal("no opponent response")
	}
	if e := s.Apply(other, Action{Type: "catan_attack_city_move", Choice: "retreat", Prompt: q.ID, Edge: q.Pending.Targets[0]}); e != nil {
		t.Fatal(e)
	}
	fishingAttackRestore(t, s)
	if s.Catan.Attack.City.Plan.Locked != 2 || len(s.Catan.Fishing.Tokens.Hands[p]) != initial {
		t.Fatal("wrong lock/reservation")
	}
	attackReject(t, s, p, Action{Type: "catan_attack_city_move", Choice: "undo", Prompt: q.ID})
	if e := s.Apply(p, Action{Type: "catan_attack_city_move", Choice: "confirm", Prompt: q.ID}); e != nil {
		t.Fatal(e)
	}
	if len(s.Catan.Fishing.Tokens.Hands[p]) != initial-len(ids) {
		t.Fatal("locked fish double charged")
	}
	fishingAttackRestore(t, s)
}

func TestCatanFishingAttackKnightsNeutralAndActiveCannotSpendFish(t *testing.T) {
	s := fishingAttackCityFixture(t, 2)
	g := s.Catan
	p := s.Turn
	fishingAttackHand(t, s, p, 4)
	edge := catanFishingSide(g, g.Attack.Map.Castles[0], 0)
	for _, k := range []catanAttackCityKnight{{Owner: p, Edge: edge, Strength: 1, Active: true}, {Owner: catanTwoNeutralOwners[0], Edge: edge, Strength: 1}} {
		trial := clone(*s)
		trial.Catan.Attack.City.Knights = []catanAttackCityKnight{k}
		if len(trial.Catan.Attack.City.fishDestinations(trial.Catan, p, edge)) != 0 {
			t.Fatal("illegal fish movement offered")
		}
		before, _ := json.Marshal(trial)
		_, e := trial.catanAttackCityResolveEnd([]catanAttackCityOrder{{From: edge, To: 0, Retreat: -1, Fish: true, Tokens: g.fishPayment(p, 2)}}, func() int { return 1 })
		if e == nil {
			t.Fatal("active/neutral paid fish")
		}
		after, _ := json.Marshal(trial)
		if string(before) != string(after) {
			t.Fatal("invalid fish movement mutated state")
		}
	}
}
