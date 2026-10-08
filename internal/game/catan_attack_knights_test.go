package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func attackCityCore(t *testing.T, n int) *State {
	t.Helper()
	s, err := newCatanAttackCityCore(n)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func attackCityHand(s *State, p int, hand []int) {
	g := s.Catan
	for c, n := range g.Players[p].Resources {
		g.Bank[c] += n
	}
	g.Players[p].Resources = slices.Clone(hand)
	for c, n := range hand {
		g.Bank[c] -= n
	}
}
func TestCatanAttackCityCoreIsolationAndRestore(t *testing.T) {
	for n := 3; n <= 6; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := attackCityCore(t, n)
			if len(s.Catan.Bank) != 8 || s.Catan.CitiesKnights.BarbarianPosition != 0 || len(s.Catan.Attack.Deck) != 0 {
				t.Fatal("wrong combined inventory")
			}
			b, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			var q State
			if err = json.Unmarshal(b, &q); err != nil {
				t.Fatal(err)
			}
			if err = q.Catan.Attack.City.validate(q.Catan); err != nil {
				t.Fatal(err)
			}
			// Invalid phase action must not expose ordinary end semantics.
			if err = q.Apply(q.Turn, Action{Type: "catan_end"}); err == nil {
				t.Fatal("unfinished combination exposed")
			}
			base, err := NewCatanAttack(n)
			if err != nil {
				t.Fatal(err)
			}
			if base.Catan.Attack.City != nil || len(base.Catan.Bank) != 5 || len(base.Catan.Attack.Deck) != 26 || base.Catan.victoryTarget() != 12 {
				t.Fatal("ordinary attack changed")
			}
		})
	}
}
func TestCatanAttackCityRecruitActivatePromote(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	p := s.Turn
	c := g.Attack.City
	s.Phase = "catan_turn"
	attackCityHand(s, p, []int{0, 0, 5, 5, 5, 0, 0, 0})
	edges := c.recruitEdges(g, p)
	if len(edges) == 0 {
		t.Fatal("castle choices")
	}
	edge := edges[0]
	for _, kind := range []string{"catan_attack_knight_recruit", "catan_attack_knight_activate", "catan_attack_knight_promote"} {
		if err := s.catanAttackCityKnightAction(p, Action{Type: kind, Edge: edge}); err != nil {
			t.Fatal(kind, err)
		}
	}
	if c.Knights[0].Strength != 2 || !c.Knights[0].Active || c.Knights[0].ActivatedAt != g.CitiesKnights.ActionSerial {
		t.Fatal("knight state")
	}
	before := clone(*s)
	if s.catanAttackCityKnightAction(p, Action{Type: "catan_attack_knight_promote", Edge: edge}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("promotion lock")
	}
	g.CitiesKnights.ActionSerial++
	if s.catanAttackCityKnightAction(p, Action{Type: "catan_attack_knight_promote", Edge: edge}) == nil {
		t.Fatal("mighty without politics")
	}
	g.CitiesKnights.Players[p].Improvements[CatanPolitics] = 3
	if err := s.catanAttackCityKnightAction(p, Action{Type: "catan_attack_knight_promote", Edge: edge}); err != nil {
		t.Fatal(err)
	}
	if c.Knights[0].Strength != 3 || !c.Knights[0].Active {
		t.Fatal("promotion lost activation")
	}
}
func TestCatanAttackCityUnlimitedLandingAndMerchant(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := attackCityCore(t, n)
			g := s.Catan
			a := g.Attack
			p := s.Turn
			tile := a.Map.Coast[0]
			total := g.Tiles[tile].Number
			a.Barbarians[tile] = 2
			a.Prisoners[p] = a.Map.Barbarians - sum(a.Barbarians)
			g.CitiesKnights.Merchant = &CatanMerchant{Owner: p, Tile: tile}
			red := max(1, total-6)
			yellow := total - red
			got, err := s.catanAttackCityLanding([2]int{red, yellow})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(got, tile) || a.Barbarians[tile] != 3 || a.City.Issued != len(got) || g.CitiesKnights.Merchant != nil {
				t.Fatal("landing supply or merchant", got, a.City.Issued)
			}
			before := clone(*s)
			if got, err = s.catanAttackCityLanding([2]int{1, 6}); err != nil || len(got) != 0 || !reflect.DeepEqual(before, *s) {
				t.Fatal("seven landed")
			}
			if _, err = s.catanAttackCityLanding([2]int{0, 6}); err == nil || !reflect.DeepEqual(before, *s) {
				t.Fatal("invalid dice mutated")
			}
		})
	}
}
func attackCityBattleFixture(t *testing.T) *State {
	t.Helper()
	s := attackCityCore(t, 3)
	s.Phase = "catan_turn"
	g := s.Catan
	a := g.Attack
	for i := range a.Barbarians {
		a.Barbarians[i] = 0
	}
	tile := a.Map.Coast[0]
	a.Barbarians[tile] = 3
	a.City.Knights = []catanAttackCityKnight{
		{Owner: 0, Edge: catanFishingSide(g, tile, 0), Strength: 3, Active: true},
		{Owner: 1, Edge: catanFishingSide(g, tile, 1), Strength: 2, Active: true},
		{Owner: 2, Edge: catanFishingSide(g, tile, 2), Strength: 3, Active: false},
	}
	return s
}
func TestCatanAttackCityBattleStrengthAndDowngrade(t *testing.T) {
	for die := 1; die <= 6; die++ {
		t.Run(fmt.Sprint(die), func(t *testing.T) {
			s := attackCityBattleFixture(t)
			g := s.Catan
			tile := g.Attack.Map.Coast[0]
			before := slices.Clone(g.Attack.City.Knights)
			b, err := s.catanAttackCityBattle(tile, func() int { return die })
			if err != nil {
				t.Fatal(err)
			}
			if b == nil || !slices.Equal(b.Strength, []int{3, 2, 0}) || !slices.Equal(b.Prisoners, []int{2, 1, 0}) || len(b.Knights) != 2 {
				t.Fatal("inactive or unweighted battle", b)
			}
			if s.Catan.Attack.Barbarians[tile] != 0 {
				t.Fatal("coast not freed")
			}
			c := s.Catan.Attack.City
			for _, k := range before {
				i := c.at(k.Edge)
				if i < 0 {
					t.Fatal("unexpected loss with reserve")
				}
				want := k.Strength
				if k.Active && g.Attack.Map.edgeOrientation(g, k.Edge) == catanAttackLossOrientation(die) {
					want--
				}
				if c.Knights[i].Strength != want || c.Knights[i].Active != k.Active {
					t.Fatal("rank downgrade", c.Knights[i], want)
				}
			}
		})
	}
}
func TestCatanAttackCityBattleShortageAndAtomicFailure(t *testing.T) {
	s := attackCityBattleFixture(t)
	g := s.Catan
	a := g.Attack
	tile := a.Map.Coast[0]
	die := 1
	for catanAttackLossOrientation(die) != a.Map.edgeOrientation(g, a.City.Knights[0].Edge) {
		die++
	}
	for _, rank := range []int{1, 1, 2, 2} {
		for _, e := range g.Edges {
			if a.City.at(e.ID) < 0 && !slices.Contains(e.Tiles, tile) {
				a.City.Knights = append(a.City.Knights, catanAttackCityKnight{Owner: 0, Edge: e.ID, Strength: rank})
				break
			}
		}
	}
	before := clone(*s)
	if _, err := s.catanAttackCityBattle(tile, func() int { return 0 }); err == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("invalid casualty die mutated state")
	}
	b, err := s.catanAttackCityBattle(tile, func() int { return die })
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(b.Lost, func(k catanAttackCityKnight) bool { return k.Owner == 0 && k.Strength == 3 }) || b.Gold[0] != 3 {
		t.Fatal("no replacement must return mighty to stock", b)
	}
}
func TestCatanAttackCityMovementAndDisplacementTargets(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	c := g.Attack.City
	from := c.recruitEdges(g, 0)[0]
	c.Knights = []catanAttackCityKnight{{Owner: 0, Edge: from, Strength: 3, Active: true}}
	far := -1
	for to, d := range c.distances(g, from, 5) {
		if d == 5 && !g.Attack.castleEdge(g, to) {
			far = to
			break
		}
	}
	if far < 0 {
		t.Fatal("no distant path")
	}
	if err := c.move(g, 0, from, far); err != nil {
		t.Fatal(err)
	}
	if c.Knights[0].Active {
		t.Fatal("moved knight stayed active")
	}
	c.Knights[0].Edge = from
	if err := c.move(g, 0, from, far); err == nil {
		t.Fatal("inactive knight moved five")
	}
	c.Knights[0].Active = true
	near := -1
	for to, d := range c.distances(g, from, 3) {
		if d > 0 && !g.Attack.castleEdge(g, to) {
			near = to
			break
		}
	}
	c.Knights = append(c.Knights, catanAttackCityKnight{Owner: 1, Edge: near, Strength: 1, Active: true})
	if !slices.Contains(c.displacementTargets(g, from), near) || len(c.retreatEdges(g, near)) == 0 {
		t.Fatal("missing displacement response")
	}
	c.Knights[0].ActivatedAt = g.CitiesKnights.ActionSerial
	if len(c.displacementTargets(g, from)) != 0 {
		t.Fatal("new activation displaced")
	}
}

func TestCatanAttackCityMetropolisAndInnerNumbers(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	a := g.Attack
	p := s.Turn
	if len(g.attackCityInventionTiles()) != 5 {
		t.Fatal("official five inner invention discs")
	}
	for _, id := range g.attackCityInventionTiles() {
		if slices.Contains(a.Map.Coast, id) {
			t.Fatal("coastal number allowed")
		}
	}
	vertex := -1
	for _, v := range g.Vertices {
		all := true
		touch := false
		for _, tile := range g.Tiles {
			if slices.Contains(tile.Vertices, v.ID) {
				touch = true
				all = all && slices.Contains(a.Map.Coast, tile.ID)
			}
		}
		if touch && all {
			vertex = v.ID
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no coastal corner")
	}
	g.Vertices[vertex].Owner = p
	g.Vertices[vertex].Level = 2
	g.CitiesKnights.Metropolises[0] = vertex
	for _, tile := range g.Tiles {
		if slices.Contains(tile.Vertices, vertex) {
			a.Barbarians[tile.ID] = 3
		}
	}
	if !g.attackCityMetropolis(vertex) || !g.attackCityImprovementBlocked(p, 0) || g.attackCityImprovementBlocked(p, 1) {
		t.Fatal("metropolis track block")
	}
	for _, tile := range g.Tiles {
		if slices.Contains(tile.Vertices, vertex) {
			a.Barbarians[tile.ID] = 2
			break
		}
	}
	if g.attackCityImprovementBlocked(p, 0) {
		t.Fatal("freed metropolis track still blocked")
	}
}

func TestCatanAttackCityDisplacementOwnerChoice(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	c := g.Attack.City
	from := c.recruitEdges(g, 0)[0]
	near := -1
	for to, d := range c.distances(g, from, 3) {
		if d == 2 && !g.Attack.castleEdge(g, to) {
			near = to
			break
		}
	}
	c.Knights = []catanAttackCityKnight{{Owner: 0, Edge: from, Strength: 3, Active: true}, {Owner: 1, Edge: near, Strength: 2, Active: true}}
	q, err := c.beginDisplacement(g, 0, from, near)
	if err != nil || q == nil {
		t.Fatal(err)
	}
	if q.Player != 1 || q.Knight.Active || c.Knights[c.at(near)].Active || len(q.Targets) == 0 {
		t.Fatal("displacement state")
	}
	before := clone(*s)
	if c.completeDisplacement(g, q, 0, q.Targets[0]) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("attacker chose retreat")
	}
	if err = c.completeDisplacement(g, q, 1, q.Targets[0]); err != nil {
		t.Fatal(err)
	}
	knight := c.Knights[c.at(q.Targets[0])]
	if knight.Owner != 1 || knight.Strength != 2 || knight.Active {
		t.Fatal("retreat changed rank or left active")
	}
	if c.completeDisplacement(g, q, 1, q.Targets[0]) == nil {
		t.Fatal("duplicate retreat")
	}
	if err = c.validate(g); err != nil {
		t.Fatal(err)
	}
}

func TestCatanAttackCityProgressVariants(t *testing.T) {
	t.Run("intrigue and prisoner score", func(t *testing.T) {
		s := attackCityCore(t, 3)
		g := s.Catan
		a := g.Attack
		p := s.Turn
		tile := a.Map.Coast[0]
		a.Prisoners[p] = 2
		a.Barbarians[tile] = 1
		if err := s.catanAttackCityIntrigue(p, tile); err != nil {
			t.Fatal(err)
		}
		s.catanScores()
		if a.Prisoners[p] != 3 || a.Barbarians[tile] != 0 || g.Players[p].Score != 1 || g.victoryTarget() != 13 {
			t.Fatal("intrigue score")
		}
	})
	t.Run("taxation commodity and conquered building", func(t *testing.T) {
		s := attackCityCore(t, 3)
		g := s.Catan
		p := s.Turn
		other := (p + 1) % 3
		tile := g.Attack.Map.Coast[0]
		v := g.Tiles[tile].Vertices[0]
		g.Vertices[v].Owner = other
		g.Vertices[v].Level = 2
		attackCityHand(s, other, []int{0, 0, 0, 0, 0, 0, 2, 0})
		for _, hex := range g.Tiles {
			if slices.Contains(hex.Vertices, v) {
				g.Attack.Barbarians[hex.ID] = 3
			}
		}
		before := clone(*s)
		if err := s.catanAttackCityTaxation(p, tile, func(n int) int { return 0 }); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, *s) {
			t.Fatal("conquered city taxed")
		}
		s.Catan.CitiesKnights.Metropolises[0] = v
		if err := s.catanAttackCityTaxation(p, tile, func(n int) int { return 0 }); err != nil {
			t.Fatal(err)
		}
		if s.Catan.Players[p].Resources[6] != 1 || s.Catan.Players[other].Resources[6] != 1 || s.Catan.Robber != -1 {
			t.Fatal("metropolis taxation/no robber")
		}
	})
	t.Run("treason exact original edge", func(t *testing.T) {
		s := attackCityCore(t, 3)
		g := s.Catan
		c := g.Attack.City
		c.Knights = []catanAttackCityKnight{{Owner: 1, Edge: 0, Strength: 3, Active: true}}
		q, err := c.treasonRemove(g, 0, 1, 0)
		if err != nil || q == nil {
			t.Fatal(err)
		}
		if err = c.treasonPlace(g, q, 1, 2); err == nil {
			t.Fatal("wrong placement player")
		}
		if err = c.treasonPlace(g, q, 0, 2); err != nil {
			t.Fatal(err)
		}
		if c.Knights[0].Edge != 0 || c.Knights[0].Owner != 0 || c.Knights[0].Strength != 2 || !c.Knights[0].Active {
			t.Fatal("treason placement")
		}
		if err = c.treasonPlace(g, q, 0, 1); err == nil {
			t.Fatal("duplicate treason")
		}
	})
}

func TestCatanAttackCityInventionRestoreAndImmediateVictory(t *testing.T) {
	t.Run("invention", func(t *testing.T) {
		s := attackCityCore(t, 3)
		g := s.Catan
		s.Phase = "catan_turn"
		g.SetupStep = 6
		choices := g.attackCityInventionTiles()
		left, right := choices[0], choices[1]
		for _, id := range choices {
			if g.Tiles[id].Number != g.Tiles[left].Number {
				right = id
				break
			}
		}
		if err := s.catanAttackCityInvention(left, right); err != nil {
			t.Fatal(err)
		}
		if err := g.Attack.City.validate(g); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(s)
		var restored State
		if err := json.Unmarshal(data, &restored); err != nil {
			t.Fatal(err)
		}
		if err := restored.Catan.Attack.City.validate(restored.Catan); err != nil {
			t.Fatal(err)
		}
		restored.Catan.Attack.City.NumberSwaps = nil
		if restored.Catan.Attack.City.validate(restored.Catan) == nil {
			t.Fatal("missing history accepted")
		}
	})
	t.Run("victory before casualty", func(t *testing.T) {
		s := attackCityBattleFixture(t)
		g := s.Catan
		s.Turn = 0
		g.SetupStep = 6
		g.Attack.Prisoners[0] = 37
		g.Attack.City.Issued = 10
		calls := 0
		b, err := s.catanAttackCityBattle(g.Attack.Map.Coast[0], func() int { calls++; return 1 })
		if err != nil {
			t.Fatal(err)
		}
		if !s.Finished || s.Catan.Players[0].Score != 13 || b.LossDie != 0 || calls != 0 {
			t.Fatal("win waited for casualty", calls, b, s.Finished)
		}
	})
}

func TestCatanAttackCityEventDieUsesCoastNotShip(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := attackCityCore(t, n)
			g := s.Catan
			g.SetupStep = 2 * n
			s.Phase = "catan_roll"
			tile := g.Attack.Map.Coast[0]
			total := g.Tiles[tile].Number
			g.Attack.Barbarians[tile] = 0
			red := max(1, total-6)
			yellow := total - red
			if err := s.catanCityRoll(red, yellow, 3); err != nil {
				t.Fatal(err)
			}
			if g.Attack.Barbarians[tile] != 1 || g.CitiesKnights.BarbarianPosition != 0 || g.CitiesKnights.Invasions != 0 || g.RollID != 1 || s.Phase != "catan_turn" {
				t.Fatal("ship event used ordinary invasion", s.Phase)
			}
			s.Phase = "catan_roll"
			if err := s.catanCityRoll(1, 6, 3); err != nil {
				t.Fatal(err)
			}
			if g.Attack.Barbarians[tile] != 1 || g.CitiesKnights.BarbarianPosition != 0 {
				t.Fatal("seven ship landing")
			}
		})
	}
}

func TestCatanAttackCityMetropolisKeepsFourPointsWithoutProduction(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	p := s.Turn
	g.SetupStep = 6
	s.Phase = "catan_turn"
	vertex := -1
	for _, v := range g.Vertices {
		all, touches := true, false
		for _, tile := range g.Tiles {
			if slices.Contains(tile.Vertices, v.ID) {
				touches = true
				all = all && slices.Contains(g.Attack.Map.Coast, tile.ID)
			}
		}
		if touches && all {
			vertex = v.ID
			break
		}
	}
	if vertex < 0 {
		t.Fatal("no coast corner")
	}
	g.Vertices[vertex].Owner = p
	g.Vertices[vertex].Level = 2
	g.CitiesKnights.Metropolises[0] = vertex
	total := 0
	for _, tile := range g.Tiles {
		if slices.Contains(tile.Vertices, vertex) {
			g.Attack.Barbarians[tile.ID] = 3
			total = tile.Number
		}
	}
	s.catanScores()
	if g.Players[p].Score != 4 {
		t.Fatal("conquered metropolis lost points", g.Players[p].Score)
	}
	if err := s.catanRollProduction(total); err != nil {
		t.Fatal(err)
	}
	if sum(g.Players[p].Resources) != 0 {
		t.Fatal("conquered metropolis produced")
	}
}

func TestCatanAttackCityCommodityCoins(t *testing.T) {
	s := attackCityCore(t, 3)
	g := s.Catan
	p := s.Turn
	s.Phase = "catan_turn"
	g.SetupStep = 6
	attackCityHand(s, p, []int{0, 0, 0, 0, 0, 4, 0, 0})
	if err := s.catanCoins(p, Action{Type: "catan_coin_sell", Color: 5}); err != nil {
		t.Fatal(err)
	}
	if g.Attack.Gold[p] != 1 || g.Players[p].Resources[5] != 0 {
		t.Fatal("commodity sale")
	}
	before := clone(*s)
	if s.catanCoins(p, Action{Type: "catan_coin_buy", Color: 5}) == nil || !reflect.DeepEqual(before, *s) {
		t.Fatal("gold bought commodity")
	}
}

func TestCatanAttackCityEndTransactionAndPairedTurns(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := attackCityCore(t, n)
			g := s.Catan
			g.SetupStep = 2 * n
			g.TurnSerial = 1
			s.Phase = "catan_turn"
			p := s.Turn
			c := g.Attack.City
			from := c.recruitEdges(g, p)[0]
			c.Knights = []catanAttackCityKnight{{Owner: p, Edge: from, Strength: 1, Active: false}}
			before := clone(*s)
			if _, err := s.catanAttackCityResolveEnd(nil, func() int { return 1 }); err == nil || !reflect.DeepEqual(before, *s) {
				t.Fatal("castle departure bypass")
			}
			destinations := c.destinations(g, from)
			if len(destinations) == 0 {
				t.Fatal("no departure")
			}
			order := catanAttackCityOrder{From: from, To: destinations[0], Retreat: -1}
			if _, err := s.catanAttackCityResolveEnd([]catanAttackCityOrder{order, order}, func() int { return 1 }); err == nil || !reflect.DeepEqual(before, *s) {
				t.Fatal("duplicate moved")
			}
			result, err := s.catanAttackCityResolveEnd([]catanAttackCityOrder{order}, func() int { return 1 })
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Orders) != 1 || s.Turn == p || s.Catan.CitiesKnights.ActionSerial != 2 {
				t.Fatal("turn did not advance")
			}
			if n == 6 && (!s.Catan.Paired.Second || s.Phase != "catan_turn") {
				t.Fatal("paired second did not retain action phase")
			}
			if n == 3 && s.Phase != "catan_roll" {
				t.Fatal("ordinary next turn missing dice")
			}
		})
	}
}
