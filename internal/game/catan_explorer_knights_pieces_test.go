package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Legal map/setup, then a controlled three-vertex road network. Piece and
// resource inventories remain conserved; this is not a naturally reached game.
func explorerKnightFixture(t *testing.T, n int) (*State, [3]int, [2]int) {
	t.Helper()
	s := explorerCityProductionFixture(t, n)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	for p := range g.Players {
		for r := range g.Bank {
			want := 0
			if r < 5 {
				want = 3
			}
			g.Bank[r] += g.Players[p].Resources[r] - want
			g.Players[p].Resources[r] = want
		}
	}
	for i := range g.Edges {
		g.Edges[i].Owner = -1
	}
	for _, a := range g.Edges {
		if !catanExplorerLandEdge(g, a.ID) || !g.explorerKnightSite(a.A) || !g.explorerKnightSite(a.B) || g.Vertices[a.A].Level > 0 || g.Vertices[a.B].Level > 0 {
			continue
		}
		for _, b := range g.Edges {
			if b.ID == a.ID || !catanExplorerLandEdge(g, b.ID) {
				continue
			}
			for _, v := range []int{a.A, a.B} {
				end := -1
				if b.A == v {
					end = b.B
				}
				if b.B == v {
					end = b.A
				}
				if end < 0 || end == a.A || end == a.B || g.Vertices[end].Level > 0 || !g.explorerKnightSite(end) {
					continue
				}
				start := a.A
				if start == v {
					start = a.B
				}
				g.Edges[a.ID].Owner, g.Edges[b.ID].Owner = 0, 0
				explorerCityRestore(t, s)
				return s, [3]int{start, v, end}, [2]int{a.ID, b.ID}
			}
		}
	}
	t.Fatal("missing three empty land vertices")
	return nil, [3]int{}, [2]int{}
}

func explorerKnightAct(t *testing.T, s *State, a Action) {
	t.Helper()
	a.Prompt = int(s.Catan.TurnSerial)
	if err := s.catanExplorerCityAction(s.Turn, a); err != nil {
		t.Fatal(a, err)
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerKnightLifecycleAndSegmentLocks(t *testing.T) {
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, v, _ := explorerKnightFixture(t, n)
			bank := slices.Clone(s.Catan.Bank)
			explorerKnightAct(t, s, Action{Type: "catan_knight_recruit", Vertex: v[0]})
			if s.Catan.Bank[2] != bank[2]+1 || s.Catan.Bank[4] != bank[4]+1 || s.Catan.knightAt(v[0]).Active {
				t.Fatal("recruit cost/activity")
			}
			explorerKnightAct(t, s, Action{Type: "catan_knight_activate", Vertex: v[0]})
			explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: v[0], Target: v[1], Prompt: 1})
			explorerKnightAct(t, s, Action{Type: "catan_knight_promote", Vertex: v[0]})
			explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: v[0], Prompt: 1})
			for steps := 0; steps < 2*n; steps++ {
				explorerCityFinishAction(t, s)
				if s.Phase == "catan_roll" {
					if err := s.catanExplorerCityRoll(1, 1, 0); err != nil {
						t.Fatal(err)
					}
				}
				if s.Turn == 0 {
					break
				}
			}
			if s.Turn != 0 || s.Catan.CitiesKnights.ActionSerial <= 1 {
				t.Fatal("no next action segment")
			}
			explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: v[0], Prompt: int(s.Catan.TurnSerial)})
			s.Catan.CitiesKnights.Players[0].Improvements[CatanPolitics] = 3
			explorerKnightAct(t, s, Action{Type: "catan_knight_promote", Vertex: v[0]})
			cargo, fleet := clone(*s.Catan.Explorer.Cargo), clone(*s.Catan.Explorer.Fleet)
			explorerKnightAct(t, s, Action{Type: "catan_knight_move", Vertex: v[0], Target: v[2]})
			piece := s.Catan.knightAt(v[2])
			if piece == nil || piece.Strength != 3 || piece.Active || s.Catan.knightAt(v[0]) != nil || !reflect.DeepEqual(cargo, *s.Catan.Explorer.Cargo) || !reflect.DeepEqual(fleet, *s.Catan.Explorer.Fleet) {
				t.Fatal("knight movement changed cargo or lost locks")
			}
			explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: v[2], Target: v[0], Prompt: int(s.Catan.TurnSerial)})
			explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_move", Vertex: v[2], Target: v[0], Choice: "ship", Prompt: int(s.Catan.TurnSerial)})
		})
	}
}

func TestCatanExplorerKnightDisplacementRetreatAndNoEscape(t *testing.T) {
	for _, escape := range []bool{true, false} {
		t.Run(fmt.Sprint(escape), func(t *testing.T) {
			s, v, e := explorerKnightFixture(t, 3)
			g := s.Catan
			g.Edges[e[1]].Owner = -1
			if escape {
				g.Edges[e[1]].Owner = 1
			}
			g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: v[0], Strength: 2, Active: true}, {Owner: 1, Vertex: v[1], Strength: 1, Active: true}}
			explorerCityRestore(t, s)
			explorerKnightAct(t, s, Action{Type: "catan_knight_move", Vertex: v[0], Target: v[1]})
			if !escape {
				if s.Phase != "catan_turn" || s.Catan.knightCount(1, 1) != 0 {
					t.Fatal("trapped knight not returned to supply")
				}
				return
			}
			if s.Phase != "catan_knight_retreat" || s.CatanPendingActor() != 1 || s.Catan.knightCount(1, 1) != 1 {
				t.Fatal("displacement did not preserve inventory/pending actor")
			}
			explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_activate", Vertex: v[1], Prompt: 1})
			explorerCityReject(t, s, 0, Action{Type: "catan_knight_retreat", Vertex: v[2], Prompt: 1})
			explorerCityReject(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: v[0], Prompt: 1})
			explorerCityReject(t, s, 1, Action{Type: "catan_knight_retreat", Vertex: v[2], Prompt: 0})
			cargo := clone(*s.Catan.Explorer.Cargo)
			if err := s.catanExplorerCityRespond(1, Action{Type: "catan_knight_retreat", Vertex: v[2], Prompt: 1}); err != nil {
				t.Fatal(err)
			}
			if s.Phase != "catan_turn" || s.Turn != 0 || s.Catan.knightAt(v[2]) == nil || !s.Catan.knightAt(v[2]).Active || s.Catan.knightAt(v[1]).Active || !reflect.DeepEqual(cargo, *s.Catan.Explorer.Cargo) {
				t.Fatal("retreat state or actor changed")
			}
			explorerCityRestore(t, s)
		})
	}
}

func TestCatanExplorerKnightFogAndShipEdgesForbidden(t *testing.T) {
	s, v, e := explorerKnightFixture(t, 3)
	g := s.Catan
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: v[0], Strength: 1, Active: true}}
	// A corrupted/artificial sea-route bridge must not create knight movement.
	g.Edges[e[0]].Ship = true
	if slices.Contains(g.knightDestinations(*g.knightAt(v[0]), false), v[1]) {
		t.Fatal("knight used a ship route")
	}
	g.Edges[e[0]].Ship = false
	// Reveal only ordinary terrain in this controlled midgame, leaving a real
	// fog frontier. No gold/lair or other mission inventories are fabricated.
	for _, h := range slices.Clone(g.Explorer.Board.Hidden) {
		if h.Resource >= 0 && h.Resource < 5 {
			if _, err := g.Explorer.Board.reveal(g, h.Tile); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, edge := range g.Edges {
		if !catanExplorerLandEdge(g, edge.ID) {
			continue
		}
		for _, target := range []int{edge.A, edge.B} {
			if g.Vertices[target].Level != 0 || g.explorerKnightSite(target) {
				continue
			}
			g.Edges[edge.ID].Owner = 0
			if g.knightPlaceable(0, target) {
				t.Fatal("fog frontier is recruitable")
			}
			explorerCityRestore(t, s)
			explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_recruit", Vertex: target, Prompt: 1})
			return
		}
	}
	t.Fatal("fixture lacks fog frontier land edge")
}

func TestCatanExplorerKnightBlocksRoadsAndSettlements(t *testing.T) {
	s, v, e := explorerKnightFixture(t, 3)
	g, x := s.Catan, s.Catan.Explorer
	g.Edges[e[1]].Owner = -1
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 1, Vertex: v[1], Strength: 1}}
	before := clone(*s)
	if err := x.Cargo.buildRoad(g, x.Fleet, 0, g.TurnSerial, e[1]); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("road passed opponent knight")
	}
	for _, owner := range []int{0, 1} {
		g.CitiesKnights.Knights[0].Owner = owner
		before = clone(*s)
		if err := x.Cargo.buildSettlement(g, x.Fleet, 0, g.TurnSerial, v[1]); err == nil || !reflect.DeepEqual(*s, before) {
			t.Fatal("settlement covered a knight")
		}
		if catanExplorerBotSite(g, 0, v[1], false) {
			t.Fatal("bot offered occupied site")
		}
	}
	// An own knight permits extending the connected road.
	g.CitiesKnights.Knights[0].Owner = 0
	if err := x.Cargo.buildRoad(g, x.Fleet, 0, g.TurnSerial, e[1]); err != nil {
		t.Fatal(err)
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerKnightInvalidStateAndActionRollback(t *testing.T) {
	base, v, _ := explorerKnightFixture(t, 3)
	for _, reason := range []string{"owner", "vertex", "strength", "duplicate", "inventory", "future_activation", "future_promotion", "serial", "building"} {
		t.Run(reason, func(t *testing.T) {
			s := clone(*base)
			k := s.Catan.CitiesKnights
			k.Knights = []CatanKnight{{Owner: 0, Vertex: v[0], Strength: 1}}
			switch reason {
			case "owner":
				k.Knights[0].Owner = 3
			case "vertex":
				k.Knights[0].Vertex = -1
			case "strength":
				k.Knights[0].Strength = 4
			case "duplicate":
				k.Knights = append(k.Knights, k.Knights[0])
			case "inventory":
				k.Knights = append(k.Knights, CatanKnight{Owner: 0, Vertex: v[1], Strength: 1}, CatanKnight{Owner: 0, Vertex: v[2], Strength: 1})
			case "future_activation":
				k.Knights[0].ActivatedAt = 2
			case "future_promotion":
				k.Knights[0].PromotedAt = 2
			case "serial":
				k.ActionSerial = 2
			case "building":
				k.Knights[0].Vertex = explorerDevelopmentCity(t, &s, 0)
			}
			if err := s.validateExplorerCityProduction(); err == nil {
				t.Fatal("invalid knight snapshot accepted")
			}
			explorerCityActionReject(t, &s, 0, Action{Type: "catan_knight_activate", Vertex: v[0], Prompt: 1})
		})
	}
	explorerKnightAct(t, base, Action{Type: "catan_knight_recruit", Vertex: v[0]})
	explorerKnightAct(t, base, Action{Type: "catan_knight_recruit", Vertex: v[1]})
	explorerCityActionReject(t, base, 0, Action{Type: "catan_knight_recruit", Vertex: v[2], Prompt: 1})
	explorerCityActionReject(t, base, 1, Action{Type: "catan_knight_activate", Vertex: v[0], Prompt: 1})
	explorerCityActionReject(t, base, 0, Action{Type: "catan_knight_activate", Vertex: v[0], Prompt: 0})
}

func TestCatanExplorerKnightBlocksSettlerLanding(t *testing.T) {
	s := explorerCityProductionFixture(t, 3)
	if err := s.catanExplorerCityRoll(1, 1, 3); err != nil {
		t.Fatal(err)
	}
	g, x := s.Catan, s.Catan.Explorer
	vertex, berth := -1, -1
	for _, v := range g.Vertices {
		if !catanExplorerBotSite(g, 0, v.ID, false) {
			continue
		}
		for _, e := range g.Edges {
			if (e.A == v.ID || e.B == v.ID) && catanExplorerSeaEdge(g, e.ID) && !slices.Contains(x.Fleet.Positions, e.ID) {
				vertex, berth = v.ID, e.ID
				break
			}
		}
		if vertex >= 0 {
			break
		}
	}
	if vertex < 0 {
		t.Fatal("missing legal coastal landing fixture")
	}
	x.Fleet.Positions[0] = berth
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 1, Vertex: vertex, Strength: 1}}
	if err := x.Cargo.beginMovement(g, x.Fleet, 0, g.TurnSerial, 0); err != nil {
		t.Fatal(err)
	}
	s.Phase = "catan_explorer_move"
	explorerCityRestore(t, s)
	g, x = s.Catan, s.Catan.Explorer
	before := clone(*s)
	if err := x.Cargo.settle(g, x.Fleet, 0, g.TurnSerial, 0, vertex); err == nil || !reflect.DeepEqual(*s, before) {
		t.Fatal("settler covered enemy knight or lost cargo")
	}
	g.CitiesKnights.Knights = nil
	if err := x.Cargo.settle(g, x.Fleet, 0, g.TurnSerial, 0, vertex); err != nil {
		t.Fatal("same coast should allow landing after knight leaves", err)
	}
	if g.Vertices[vertex].Owner != 0 || x.Fleet.Positions[0] != -1 {
		t.Fatal("settler landing not completed")
	}
	explorerCityRestore(t, s)
}

func TestCatanExplorerKnightSmithingAtomicPair(t *testing.T) {
	s, v, _ := explorerKnightFixture(t, 3)
	g := s.Catan
	g.CitiesKnights.Players[0].Improvements[CatanPolitics] = 3
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: v[0], Strength: 2, Active: true}, {Owner: 0, Vertex: v[1], Strength: 1}}
	ckProgressGive(t, s, 0, 8)
	// Failure on the second target must undo the first promotion and card use.
	explorerCityActionReject(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{v[0], v[2]}, Prompt: 1})
	explorerCityActionReject(t, s, 0, Action{Type: "catan_progress", Card: 8, Targets: []int{v[0], v[0]}, Prompt: 1})
	before := clone(*s.Catan)
	explorerKnightAct(t, s, Action{Type: "catan_progress", Card: 8, Targets: []int{v[0], v[1]}})
	if s.Catan.knightAt(v[0]).Strength != 3 || s.Catan.knightAt(v[1]).Strength != 2 || !s.Catan.knightAt(v[0]).Active || s.Catan.knightAt(v[1]).Active || !slices.Equal(s.Catan.Bank, before.Bank) || !slices.Equal(s.Catan.Players[0].Resources, before.Players[0].Resources) || len(s.Catan.CitiesKnights.Players[0].Progress) != 0 {
		t.Fatal("Smithing changed resources, activity or failed to consume card")
	}
	explorerCityActionReject(t, s, 0, Action{Type: "catan_knight_promote", Vertex: v[1], Prompt: 1})
}
