package game

import (
	"reflect"
	"slices"
	"testing"
)

func ckKnightGraph(t *testing.T, owners []int) *State {
	t.Helper()
	s := ckEmptyTurn(t)
	g := s.Catan
	g.Vertices = make([]CatanVertex, len(owners)+1)
	g.Edges = make([]CatanEdge, len(owners))
	g.Tiles = []CatanTile{{ID: 0, Resource: 0, Number: 6, Vertices: []int{0, 1}}}
	g.Ports = nil
	for i := range g.Vertices {
		g.Vertices[i] = CatanVertex{ID: i, Owner: -1}
	}
	for i, owner := range owners {
		g.Edges[i] = CatanEdge{ID: i, A: i, B: i + 1, Owner: owner}
	}
	g.CitiesKnights.ActionSerial = 5
	return s
}
func ckKnightStock(t *testing.T, g *Catan) {
	t.Helper()
	positions := map[int]bool{}
	for _, n := range g.CitiesKnights.Knights {
		if n.Strength < 1 || n.Strength > 3 || positions[n.Vertex] || g.Vertices[n.Vertex].Level > 0 {
			t.Fatal("invalid knight position/strength", n)
		}
		positions[n.Vertex] = true
	}
	for p := range g.Players {
		for level := 1; level <= 3; level++ {
			if g.knightCount(p, level) > 2 {
				t.Fatal("knight piece inventory", p, level)
			}
		}
	}
	ckSupply(t, g)
}
func TestCatanKnightsRecruitPromoteInventoryAndActionLocks(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 0})
	helperGrant(s, 0, []int{0, 0, 8, 3, 8, 0, 0, 0})
	helperReject(t, s, 1, Action{Type: "catan_knight_recruit", Vertex: 0})
	for _, v := range []int{0, 2} {
		helperApply(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: v})
	}
	helperReject(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: 4})
	helperReject(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: 0})
	helperApply(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 0})
	helperReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 0})
	helperApply(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: 4})
	helperApply(t, s, 0, Action{Type: "catan_knight_activate", Vertex: 0})
	helperReject(t, s, 0, Action{Type: "catan_knight_activate", Vertex: 0})
	helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 1})
	restored := clone(*s)
	s = &restored
	helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 1})
	s.Catan.CitiesKnights.ActionSerial++
	helperReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 0}) // Politics gate.
	s.Catan.CitiesKnights.Players[0].Improvements[CatanPolitics] = 3
	helperApply(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 0})
	if !s.Catan.knightAt(0).Active || s.Catan.knightAt(0).Strength != 3 {
		t.Fatal("promotion changed active status")
	}
	helperApply(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 1})
	helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 1, Target: 0})
	helperApply(t, s, 0, Action{Type: "catan_knight_activate", Vertex: 1})
	helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 1, Target: 0})
	if s.Catan.knightAt(1).PromotedAt != s.Catan.CitiesKnights.ActionSerial {
		t.Fatal("moving reset promotion lock")
	}
	// Three recruits, two promotions and two activations pay five wool/ore
	// pairs and two grain. Movement and rejected requests charge nothing.
	if !reflect.DeepEqual(s.Catan.Players[0].Resources, []int{0, 0, 3, 1, 3, 0, 0, 0}) {
		t.Fatal("incorrect knight construction/activation costs", s.Catan.Players[0].Resources)
	}
	ckKnightStock(t, s.Catan)
}
func TestCatanKnightsTierInventoryAndNoOwnSettlementOverlap(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 0})
	k := s.Catan.CitiesKnights
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 2}, {Owner: 0, Vertex: 2, Strength: 2}, {Owner: 0, Vertex: 4, Strength: 1}}
	helperGrant(s, 0, []int{0, 0, 1, 0, 1, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 4})
	if s.Catan.canSettlement(0, 4, false) {
		t.Fatal("settlement replaced own knight")
	}
	if !s.Catan.canSettlement(0, 3, false) {
		t.Fatal("knights incorrectly imposed distance rule")
	}
	helperReject(t, s, 0, Action{Type: "catan_settlement", Vertex: 4})
	s.Catan.Vertices[6].Owner, s.Catan.Vertices[6].Level = 0, 1
	helperReject(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: 6})
	ckKnightStock(t, s.Catan)
}
func TestCatanKnightsBlockRoadAndLongestRoute(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 0})
	g := s.Catan
	// Player 1 can recruit at the centre through a branching road.
	g.Vertices = append(g.Vertices, CatanVertex{ID: 7, Owner: -1}, CatanVertex{ID: 8, Owner: -1})
	g.Edges = append(g.Edges, CatanEdge{ID: 6, A: 3, B: 7, Owner: 1}, CatanEdge{ID: 7, A: 3, B: 8, Owner: -1})
	s.catanScores()
	if g.roadLength(0) != 6 || g.LongestOwner != 0 {
		t.Fatal("fixture longest route")
	}
	s.Turn = 1
	helperGrant(s, 1, []int{0, 0, 1, 0, 1, 0, 0, 0})
	helperApply(t, s, 1, Action{Type: "catan_knight_recruit", Vertex: 3})
	g = s.Catan
	if g.roadLength(0) != 3 || g.LongestOwner != -1 || g.canRoad(0, 7) || !g.canRoad(1, 7) {
		t.Fatal("knight failed to interrupt opponent route")
	}
	// Activation is irrelevant to blocking.
	g.knightAt(3).Active = true
	if g.roadLength(0) != 3 || g.canRoad(0, 7) {
		t.Fatal("active knight blocking")
	}
	helperApply(t, s, 1, Action{Type: "catan_knight_move", Vertex: 3, Target: 7})
	if s.Catan.roadLength(0) != 6 || s.Catan.LongestOwner != 0 {
		t.Fatal("moving knight did not restore route award")
	}
	ckKnightStock(t, s.Catan)
}
func TestCatanKnightsMovementGraphAndBlockers(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 1})
	g := s.Catan
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 2, Active: true}, {Owner: 0, Vertex: 2, Strength: 1}, {Owner: 1, Vertex: 4, Strength: 2}}
	g.Vertices[1].Owner, g.Vertices[1].Level = 0, 1
	if got := g.knightDestinations(*g.knightAt(0), false); !reflect.DeepEqual(got, []int{3}) {
		t.Fatal("own pieces traversal or equal strength block", got)
	}
	helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 4})
	helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 5})
	helperReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 6})
	s.Catan.Vertices[1].Owner = 1
	if len(s.Catan.knightDestinations(*s.Catan.knightAt(0), false)) != 0 {
		t.Fatal("passed enemy building")
	}
}
func ckDisplacement(t *testing.T) *State {
	s := ckKnightGraph(t, []int{0, 0, 1, 1, 1})
	g := s.Catan
	g.Vertices[3].Owner, g.Vertices[3].Level = 1, 1
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 2, Active: true}, {Owner: 1, Vertex: 2, Strength: 1, Active: true, ActivatedAt: 3, PromotedAt: 4}}
	return s
}
func TestCatanKnightsDisplacementChoiceRestoreAndTimeout(t *testing.T) {
	for _, auto := range []bool{false, true} {
		s := ckDisplacement(t)
		helperApply(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 2})
		if s.Phase != "catan_knight_retreat" || s.CatanPendingActor() != 1 || s.Catan.knightAt(2).Owner != 0 || s.Catan.knightAt(2).Active {
			t.Fatal("displacement continuation")
		}
		ckKnightStock(t, s.Catan)
		helperReject(t, s, 0, Action{Type: "catan_knight_retreat", Vertex: 4})
		helperReject(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: 3})
		helperReject(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: 0})
		helperReject(t, s, 0, Action{Type: "catan_end"})
		for _, p := range []int{-1, 0, 1, 2} {
			legal := s.View(p)["catan"].(map[string]any)["legal"].(map[string][]int)
			if (len(legal["knightRetreat"]) > 0) != (p == 1) {
				t.Fatal("retreat view actor")
			}
		}
		restored := clone(*s)
		s = &restored
		if auto {
			s.AutoCatanPending()
		} else {
			helperApply(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: 4})
		}
		if s.Phase != "catan_turn" || s.Turn != 0 || s.CatanPendingActor() != -1 {
			t.Fatal("failed to resume attacker")
		}
		n := s.Catan.knightAt(4)
		if n == nil || n.Owner != 1 || !n.Active || n.ActivatedAt != 3 || n.PromotedAt != 4 {
			t.Fatal("retreat lost piece status", n)
		}
		ckKnightStock(t, s.Catan)
	}
	s := ckDisplacement(t)
	s.Catan.Edges[2].Owner = 0 // No route owned by the displaced knight at its location.
	helperApply(t, s, 0, Action{Type: "catan_knight_move", Vertex: 0, Target: 2})
	if s.CatanPendingActor() != -1 || s.Catan.knightCount(1, 1) != 0 {
		t.Fatal("no escape must return knight")
	}
}
func TestCatanKnightsChaseAndPairedActionSerial(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0})
	g := s.Catan
	g.Tiles = append(g.Tiles, CatanTile{ID: 1, Resource: 2, Number: 8, Vertices: []int{2, 3}})
	g.Robber = 0
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 1, Active: true}}
	helperReject(t, s, 0, Action{Type: "catan_knight_chase", Vertex: 0})
	s.Catan.CitiesKnights.Invasions = 1
	helperApply(t, s, 0, Action{Type: "catan_knight_chase", Vertex: 0})
	if s.Phase != "catan_robber" || s.Catan.knightAt(0).Active {
		t.Fatal("chase phase")
	}
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: 1})
	if s.Phase != "catan_turn" {
		t.Fatal("chase must resume action")
	}
	s = ckGame(t, 5)
	g = s.Catan
	g.SetupStep = 10
	g.TurnSerial = 1
	s.Turn, s.Phase = 0, "catan_turn"
	g.Paired.Primary, g.Paired.Secondary = 0, 3
	g.CitiesKnights.ActionSerial = 4
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 3, Vertex: 0, Strength: 1, Active: true, ActivatedAt: 3, PromotedAt: 3}}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	g = s.Catan
	if s.Turn != 3 || g.TurnSerial != 1 || g.CitiesKnights.ActionSerial != 5 || !g.knightCanAct(g.knightAt(0)) || !g.knightCanPromote(g.knightAt(0)) {
		t.Fatal("paired action incorrectly shares knight locks")
	}
	helperGrant(s, 3, []int{0, 0, 1, 0, 1, 0, 0, 0})
	helperApply(t, s, 3, Action{Type: "catan_knight_promote", Vertex: 0})
	helperReject(t, s, 3, Action{Type: "catan_knight_promote", Vertex: 0})
	helperApply(t, s, 3, Action{Type: "catan_end"})
	if s.Catan.CitiesKnights.ActionSerial != 6 {
		t.Fatal("next primary action serial")
	}
}
func TestCatanKnightsBotAndViewRespectActivation(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0})
	s.Catan.Vertices[3].Owner, s.Catan.Vertices[3].Level = 0, 2
	helperGrant(s, 0, []int{0, 0, 1, 1, 1, 0, 0, 0})
	a, err := s.BotAction(0)
	if err != nil || a.Type != "catan_knight_recruit" {
		t.Fatal("bot did not recruit defender", a, err)
	}
	helperApply(t, s, 0, a)
	a, err = s.BotAction(0)
	if err != nil || a.Type != "catan_knight_activate" {
		t.Fatal("bot did not activate defender", a, err)
	}
	helperApply(t, s, 0, a)
	v := s.View(0)["catan"].(map[string]any)
	if len(v["knightMoves"].(map[int][]int)) > 0 {
		t.Fatal("newly active knight can move in view")
	}
	if slices.Contains(v["legal"].(map[string][]int)["knightActivate"], a.Vertex) {
		t.Fatal("active knight remains activatable")
	}
	ckKnightStock(t, s.Catan)
}

func TestCatanKnightsUnavailableResourcesTierThreeAndElimination(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: 0})
	k := s.Catan.CitiesKnights
	k.Knights = []CatanKnight{{Owner: 0, Vertex: 0, Strength: 3, Active: true}, {Owner: 0, Vertex: 2, Strength: 3}, {Owner: 0, Vertex: 4, Strength: 2}, {Owner: 1, Vertex: 6, Strength: 1, Active: true}}
	k.Players[0].Improvements[CatanPolitics] = 3
	helperGrant(s, 0, []int{0, 0, 2, 1, 2, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 4})
	helperReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: 0})
	if err := s.EliminateCatan(0); err != nil {
		t.Fatal(err)
	}
	for _, n := range s.Catan.CitiesKnights.Knights {
		if n.Owner == 0 {
			t.Fatal("departed player's knight remains as potential pending responder")
		}
	}
	if s.Catan.knightAt(6) == nil {
		t.Fatal("another player's knight removed")
	}
	ckKnightStock(t, s.Catan)
	base, err := NewCatan(3, CatanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	base.Catan.SetupStep = 6
	base.Phase = "catan_turn"
	helperReject(t, base, base.Turn, Action{Type: "catan_knight_recruit", Vertex: 0})
}

func TestCatanKnightsRandomFirstPlayerRoundAndActionSerial(t *testing.T) {
	s := ckEmptyTurn(t)
	s.Catan.StartPlayer = 2
	s.Turn = 2
	s.Round = 1
	serial := s.Catan.CitiesKnights.ActionSerial
	for step, want := range []int{0, 1, 2} {
		// Event dice remain under implementation; only advance completed action
		// phases here, without pretending that a full production turn was played.
		s.Phase = "catan_turn"
		helperApply(t, s, s.Turn, Action{Type: "catan_end"})
		wantRound := 1
		if step == 2 {
			wantRound = 2
		}
		if s.Turn != want || s.Round != wantRound || s.Catan.CitiesKnights.ActionSerial != serial+uint64(step+1) {
			t.Fatal("random first player round/action sequence", s.Turn, s.Round)
		}
	}
}

func TestCatanKnightsRetreatCompletesBeforeRouteVictory(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 0, 0, 0, 0})
	g := s.Catan
	g.Vertices = append(g.Vertices, CatanVertex{ID: 7, Owner: -1}, CatanVertex{ID: 8, Owner: -1})
	g.Edges = append(g.Edges, CatanEdge{ID: 6, A: 3, B: 7, Owner: 0}, CatanEdge{ID: 7, A: 3, B: 8, Owner: 1}, CatanEdge{ID: 8, A: 8, B: 4, Owner: 1})
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 7, Strength: 2, Active: true}, {Owner: 1, Vertex: 3, Strength: 1}}
	g.CitiesKnights.Players[0].ProgressPoints = 11
	s.catanScores()
	if g.Players[0].Score != 11 || g.LongestOwner != -1 {
		t.Fatal("fixture route award")
	}
	helperApply(t, s, 0, Action{Type: "catan_knight_move", Vertex: 7, Target: 3})
	if s.Finished || s.Phase != "catan_knight_retreat" || s.Catan.Players[0].Score != 11 {
		t.Fatal("transient knight absence incorrectly ended match")
	}
	helperApply(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: 4})
	if s.Finished || s.Catan.LongestOwner != -1 || s.Catan.Players[0].Score != 11 || s.Catan.roadLength(0) != 4 {
		t.Fatal("final blocking knight did not determine route award")
	}
}
