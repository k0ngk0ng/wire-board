package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func harborsGame(t *testing.T) *State {
	t.Helper()
	s, err := NewCatanHarbors(3, CatanOptions{}, CatanBaseConfiguration{})
	if err != nil {
		t.Fatal(err)
	}
	s.Catan.SetupStep = s.Catan.SetupLimit()
	s.Turn, s.Phase = 0, "catan_turn"
	return s
}

// Explicit board fixture for award changes, not a claimed legal playthrough.
func harborBuildings(g *Catan, points []int) {
	for i := range g.Vertices {
		g.Vertices[i].Owner, g.Vertices[i].Level = -1, 0
	}
	for p, n := range points {
		for j := 0; n > 0; j++ {
			v := g.Edges[g.Ports[p*3+j].Edge].A
			level := min(2, n)
			g.Vertices[v].Owner, g.Vertices[v].Level = p, level
			n -= level
		}
	}
}

func TestCatanHarborsOwnershipAndScore(t *testing.T) {
	s := harborsGame(t)
	for _, tc := range []struct {
		points []int
		owner  int
	}{
		{[]int{2, 2, 0}, -1}, {[]int{3, 2, 0}, 0},
		{[]int{3, 3, 0}, 0}, {[]int{3, 4, 0}, 1},
		{[]int{3, 3, 0}, 1}, {[]int{3, 2, 3}, -1},
		{[]int{4, 2, 3}, 0}, {[]int{2, 2, 2}, -1},
	} {
		harborBuildings(s.Catan, tc.points)
		s.catanScores()
		if s.Catan.Harbors.Owner != tc.owner {
			t.Fatal(tc, s.Catan.Harbors)
		}
		for p, n := range tc.points {
			if p == tc.owner {
				n += 2
			}
			if s.Catan.Players[p].Score != n {
				t.Fatal("award counted incorrectly", p, n, s.Catan.Players[p].Score)
			}
		}
		logs := len(s.Log)
		s.catanScores()
		if len(s.Log) != logs {
			t.Fatal("repeated ownership log")
		}
	}
	harborBuildings(s.Catan, []int{4, 3, 0})
	s.catanScores()
	s.Catan.Players[0].Eliminated = true
	s.catanScores()
	if s.Catan.Harbors.Owner != 1 || !reflect.DeepEqual(s.Catan.harborPoints(), []int{0, 3, 0}) {
		t.Fatal("eliminated holder")
	}
}

func TestCatanHarborsMetropolisNeutralAndDuplicatePorts(t *testing.T) {
	s := ckEmptyTurn(t)
	s.enableCatanHarbors()
	g := s.Catan
	city := g.Edges[g.Ports[0].Edge].A
	neutral := g.Edges[g.Ports[1].Edge].A
	g.Vertices[city].Owner, g.Vertices[city].Level = 0, 2
	g.Vertices[neutral].Owner, g.Vertices[neutral].Level = -1, 1
	g.CitiesKnights.Metropolises[0] = city
	g.Ports = append(g.Ports, g.Ports[0])
	s.catanScores()
	if g.harborPoints()[0] != 2 || g.Harbors.Owner != -1 || g.Players[0].Score != 4 {
		t.Fatal("metropolis/neutral/duplicate", g.harborPoints())
	}
}

func TestCatanHarborsPillageReleasesAward(t *testing.T) {
	s := ckEvent(t)
	s.enableCatanHarbors()
	g := s.Catan
	city := g.Edges[g.Ports[0].Edge].A
	village := g.Edges[g.Ports[1].Edge].A
	g.Vertices[city].Owner, g.Vertices[city].Level = 0, 2
	g.Vertices[village].Owner, g.Vertices[village].Level = 0, 1
	s.catanScores()
	if g.Harbors.Owner != 0 || g.Players[0].Score != 5 {
		t.Fatal("initial award")
	}
	g.CitiesKnights.BarbarianPosition = 6
	if err := s.catanCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	helperApply(t, s, 0, Action{Type: "catan_pillage", Vertex: city})
	if s.Catan.Harbors.Owner != -1 || s.Catan.Players[0].Score != 2 {
		t.Fatal("pillage kept award", s.Catan.Players[0].Score)
	}
}

func TestCatanHarborsPaidCityWinsAndRejectionIsAtomic(t *testing.T) {
	s := harborsGame(t)
	g := s.Catan
	village := g.Edges[g.Ports[0].Edge].A
	other := g.Edges[g.Ports[1].Edge].A
	g.Vertices[village].Owner, g.Vertices[village].Level = 0, 1
	g.Vertices[other].Owner, g.Vertices[other].Level = 0, 1
	cities := 0
	for i := range g.Vertices {
		if !g.harborVertex(i) && cities < 3 {
			g.Vertices[i].Owner, g.Vertices[i].Level = 0, 2
			cities++
		}
	}
	s.catanScores()
	if g.Players[0].Score != 8 || g.Harbors.Owner != -1 {
		t.Fatal("fixture score")
	}
	helperReject(t, s, 0, Action{Type: "catan_city", Vertex: village})
	helperGrant(s, 0, []int{0, 0, 0, 2, 3})
	helperReject(t, s, 1, Action{Type: "catan_city", Vertex: village})
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: village})
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) || s.Catan.Players[0].Score != 11 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("paid build did not win", s.Phase, s.Catan.Players[0])
	}
	fleetSupply(t, s.Catan)
}

func TestCatanHarborsTargetsAndSpecialEndings(t *testing.T) {
	s := harborsGame(t)
	if s.Catan.victoryTarget() != 11 {
		t.Fatal("base target")
	}
	s.Catan.Players[0].Score = 11
	s.Turn = 1
	s.catanVictory()
	if s.Finished {
		t.Fatal("off-turn win")
	}
	s.Turn = 0
	s.catanVictory()
	if !s.Finished {
		t.Fatal("own-turn win")
	}
	s = ckEmptyTurn(t)
	s.enableCatanHarbors()
	if s.Catan.victoryTarget() != 14 {
		t.Fatal("CK target")
	}
	for _, scenario := range []string{"shores", "islands", "fog", "desert", "new_world", "wonders", "cloth"} {
		s = ckSea(t, 3, scenario)
		target := s.Catan.Seafarers.VictoryPoints
		s.enableCatanHarbors()
		s.enableCatanHarbors()
		if s.Catan.victoryTarget() != target+1 || s.Catan.Seafarers.VictoryPoints != target {
			t.Fatal("stacked/mutated scenario target", scenario)
		}
	}
	for _, ck := range []bool{false, true} {
		s = wondersGame(t, 3, false)
		target := 11
		if ck {
			s = ckSea(t, 3, "wonders")
			target = 13
		}
		s.Catan.SetupStep = s.Catan.SetupLimit()
		s.Turn, s.Phase = 0, "catan_turn"
		s.enableCatanHarbors()
		g := s.Catan
		g.wonders().Cards[0].Owner, g.wonders().Cards[0].Level = 0, 1
		g.Players[0].Score = target - 1
		s.catanVictory()
		if s.Finished {
			t.Fatal("old wonder score target")
		}
		g.Players[0].Score = target
		s.catanVictory()
		if !s.Finished {
			t.Fatal("new wonder score target")
		}
		s.Finished, s.Phase = false, "catan_turn"
		g.Players[0].Score = 2
		g.wonders().Cards[0].Level = 4
		s.catanVictory()
		if !s.Finished {
			t.Fatal("four-level wonder ending lost")
		}
	}
	s = pirateIslandsMapGame(t, 3)
	s.Catan.SetupStep = s.Catan.SetupLimit()
	s.Turn, s.Phase = 0, "catan_turn"
	s.enableCatanHarbors()
	s.Catan.Players[0].Score = s.Catan.victoryTarget()
	s.catanVictory()
	if s.Finished {
		t.Fatal("fortress bypassed")
	}
	s.Catan.pirateIslands().Fortresses[0].Strength = 0
	s.catanVictory()
	if !s.Finished {
		t.Fatal("liberated fortress ending")
	}
	s = clothTestGame(t)
	s.enableCatanHarbors()
	s.Catan.cloth().Villages = make([]CatanClothVillage, 5)
	s.catanScores()
	if !s.catanClothEnd() || !s.Finished || s.Catan.cloth().EmptyLimit != 5 {
		t.Fatal("cloth depletion ending lost")
	}
}

func TestCatanHarborsTribePortImmediateVictory(t *testing.T) {
	s, edge := tribeGame(t)
	g := s.Catan
	// Choose an existing city port sufficiently far from the newly placed port.
	g.Ports = nil
	found := false
	for _, e := range g.Edges {
		if !g.edgeTerrain(e.ID, true) || !g.edgeTerrain(e.ID, false) {
			continue
		}
		if e.A == g.Edges[edge].A || e.B == g.Edges[edge].A {
			continue
		}
		g.Ports = []CatanPort{{Edge: e.ID, Resource: -1}}
		if slices.Contains(g.tribePortEdges(0), edge) {
			g.Vertices[e.A].Owner, g.Vertices[e.A].Level = 0, 2
			found = true
			break
		}
	}
	if !found {
		t.Fatal("fixture missing separate port")
	}
	tr := g.tribe()
	tr.Points[0] = 9
	tr.Ports = []CatanPort{{Edge: edge, Resource: 0}}
	tr.HeldPorts[0] = []int{-1}
	s.enableCatanHarbors()
	if g.harborPoints()[0] != 2 || g.Players[0].Score != 12 {
		t.Fatal("held/uncollected port counted")
	}
	// Remove the uncollected reward from this edge before reusing that location.
	tr.Ports = nil
	if g.harborPortValue(0, edge) <= 35 {
		t.Fatal("relocated port's immediate award ignored by bot")
	}
	if !s.catanAskTribePort(0, "catan_free_road", &CatanRouteCompletion{}, false) {
		t.Fatal("port prompt")
	}
	helperReject(t, s, 0, Action{Type: "catan_port", Edge: edge, Slot: 99})
	restored := clone(*s)
	s = &restored
	helperApply(t, s, 0, Action{Type: "catan_port", Edge: edge, Slot: 0})
	if !s.Finished || s.Phase != "finished" || s.Catan.tribe().Pending != nil || s.Catan.Players[0].Score != 14 || s.Catan.Harbors.Owner != 0 {
		t.Fatal("port win resumed route", s.Phase, s.Catan.Players[0].Score)
	}
}

func TestCatanHarborsPrivacyPersistenceAndBotValuation(t *testing.T) {
	s := harborsGame(t)
	g := s.Catan
	harborBuildings(g, []int{2, 3, 0})
	s.catanScores()
	catanCard(g, 0, 4)
	helperGrant(s, 0, []int{1, 2, 3, 0, 0})
	s.catanScores()
	restored := clone(*s)
	if !reflect.DeepEqual(*s, restored) {
		t.Fatal("save roundtrip")
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		v := restored.View(viewer)["catan"].(map[string]any)
		h := v["harbors"].(map[string]any)
		if !reflect.DeepEqual(h["points"], []int{2, 3, 0}) || h["owner"].(float64) != 1 || v["victoryTarget"] != 11 {
			t.Fatal("public award state", h)
		}
		if _, ok := v["devDeck"]; ok {
			t.Fatal("deck leak")
		}
		p := v["players"].([]any)[0].(map[string]any)
		if viewer != 0 {
			for _, key := range []string{"resources", "dev", "newDev"} {
				if _, ok := p[key]; ok {
					t.Fatal("hand leak", key)
				}
			}
			if p["score"].(int) != g.Players[0].Score-1 {
				t.Fatal("secret VP leak")
			}
		}
	}
	site := g.Edges[g.Ports[1].Edge].A
	if g.harborVertexValue(0, site) <= 0 {
		t.Fatal("port site ignored")
	}
	// One extra point only ties the current holder; two points wins the award.
	if g.harborGainValue(0, 2) <= 2*g.harborGainValue(0, 1) {
		t.Fatal("taking award not valued")
	}
	before := g.harborVertexValue(0, site)
	g.Players[1].Resources = []int{9, 0, 0, 0, 0}
	g.Players[1].Dev[4] = 4
	if g.harborVertexValue(0, site) != before {
		t.Fatal("bot used opponent hand")
	}
	if _, err := NewCatanHarbors(3, CatanOptions{Helpers: true}, CatanBaseConfiguration{}); err != nil {
		t.Fatal(err)
	}
}

func TestCatanHarborsBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, layout := range CatanBaseLayouts(n) {
			t.Run(fmt.Sprintf("%d/%s", n, layout), func(t *testing.T) {
				s, err := NewCatanHarbors(n, CatanOptions{FiveSix: n > 4}, CatanBaseConfiguration{Layout: layout})
				if err != nil {
					t.Fatal(err)
				}
				steps, awards := 0, 0
				for ; steps < 10000 && !s.Finished; steps++ {
					actor := ckActor(s)
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(steps, s.Phase, err)
					}
					helperApply(t, s, actor, a)
					fleetSupply(t, s.Catan)
					if s.Catan.Harbors.Owner >= 0 {
						awards++
					}
					for p := range s.Catan.Players {
						r, v, c := s.Catan.pieces(p)
						if r > 15 || v > 5 || c > 4 {
							t.Fatal("piece inventory")
						}
					}
					if steps%31 == 0 {
						restored := clone(*s)
						if !reflect.DeepEqual(*s, restored) {
							t.Fatal("save roundtrip")
						}
						s = &restored
					}
				}
				if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 11 {
					t.Fatal("no legal ending", steps)
				}
				t.Logf("%d actions, %d states with harbor award, winner score %d", steps, awards, s.Catan.Players[s.Winners[0]].Score)
			})
		}
	}
}

func TestCatanHarborsCitiesKnightsBotsComplete(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := ckGame(t, n)
			s.enableCatanHarbors()
			steps := 0
			for ; steps < 10000 && !s.Finished; steps++ {
				actor := ckActor(s)
				a, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(steps, s.Phase, err)
				}
				helperApply(t, s, actor, a)
				ckSupply(t, s.Catan)
				ckProgressStock(t, s.Catan)
				ckKnightStock(t, s.Catan)
				if steps%31 == 0 {
					restored := clone(*s)
					if !reflect.DeepEqual(*s, restored) {
						t.Fatal("save roundtrip")
					}
					s = &restored
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 14 {
				t.Fatal("no CK harbor ending", steps)
			}
			t.Logf("%d actions, %d barbarian attacks, winner score %d", steps, s.Catan.CitiesKnights.Invasions, s.Catan.Players[s.Winners[0]].Score)
		})
	}
}
