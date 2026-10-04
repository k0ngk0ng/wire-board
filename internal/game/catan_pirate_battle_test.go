package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func navyComplete(t *testing.T, s *State) {
	t.Helper()
	for tries := 0; tries < 15; tries++ {
		if s.Catan.pirateFortressReady(s.Turn) {
			return
		}
		navyExtend(t, s)
	}
	t.Fatal("fortress not reached")
}
func pirateGiveDev(t *testing.T, g *Catan, player, kind int) {
	t.Helper()
	i := slices.Index(g.DevDeck, kind)
	if i < 0 {
		t.Fatal("missing card")
	}
	g.DevDeck = append(g.DevDeck[:i], g.DevDeck[i+1:]...)
	g.Players[player].Dev[kind]++
}
func TestCatanPirateDevelopmentDeckAndKnightUpgrade(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := navyFixture(t, n, 0)
			g := s.Catan
			counts := make([]int, 5)
			for _, card := range g.DevDeck {
				counts[card]++
			}
			want := []int{14, 2, 2, 2, 0}
			if n == 4 {
				want[4] = 5
			}
			if n > 4 {
				want = []int{20, 3, 3, 3, 5}
			}
			if !reflect.DeepEqual(counts, want) {
				t.Fatal("scenario development composition", counts)
			}
			navyExtend(t, s)
			g = s.Catan
			route := append([]int{}, g.pirateIslands().Fortresses[0].Route...)
			pirateGiveDev(t, g, 0, 0)
			pirateGiveDev(t, g, 0, 0)
			s.Phase = "catan_roll"
			helperApply(t, s, 0, Action{Type: "catan_dev", Card: 0})
			g = s.Catan
			if !g.Edges[route[0]].Warship || g.Edges[route[1]].Warship || g.Players[0].Knights != 0 || s.Phase != "catan_roll" || g.Robber != -1 || g.ArmyOwner != -1 {
				t.Fatal("knight did not upgrade nearest normal ship")
			}
			helperReject(t, s, 0, Action{Type: "catan_dev", Card: 0})
			g.PlayedDev = false
			helperApply(t, s, 0, Action{Type: "catan_dev", Card: 0})
			g = s.Catan
			if !g.Edges[route[1]].Warship || len(g.DevDiscard) != 2 {
				t.Fatal("second conversion skipped ship")
			}
			g.PlayedDev = false
			pirateGiveDev(t, g, 0, 0)
			helperReject(t, s, 0, Action{Type: "catan_dev", Card: 0})
			s.Phase = "catan_turn"
			navyExtend(t, s)
			g = s.Catan
			g.Players[0].NewDev[0] = 1
			helperReject(t, s, 0, Action{Type: "catan_dev", Card: 0})
		})
	}
}
func TestCatanPirateFortressBattleOutcomesAndReservedPiece(t *testing.T) {
	for _, mode := range []string{"loss", "tie", "win", "capture"} {
		t.Run(mode, func(t *testing.T) {
			s := navyFixture(t, 4, 1)
			navyComplete(t, s)
			g := s.Catan
			p := g.pirateIslands()
			f := &p.Fortresses[1]
			route := append([]int{}, f.Route...)
			for i, id := range route {
				g.Edges[id].Warship = i < 2
			}
			die := 3
			if mode == "tie" {
				die = 2
			}
			if mode == "win" || mode == "capture" {
				die = 1
			}
			if mode == "capture" {
				f.Strength = 1
			}
			_, beforePieces, _ := g.pieces(1)
			dice := append([]int{}, g.Dice...)
			if !s.catanAttackFortress(1, die) {
				t.Fatal("ready fortress did not attack")
			}
			if !reflect.DeepEqual(g.Dice, dice) || p.Battle == nil || p.Battle.Die != die || p.Battle.Warships != 2 {
				t.Fatal("battle die overwrote production")
			}
			switch mode {
			case "loss", "tie":
				remove := 2
				if mode == "tie" {
					remove = 1
				}
				if f.Strength != 3 || len(f.Route) != len(route)-remove || len(p.Battle.Removed) != remove {
					t.Fatal("wrong battle loss")
				}
				for i, id := range p.Battle.Removed {
					if id != route[len(route)-1-i] || g.Edges[id].Owner >= 0 || g.Edges[id].Ship || g.Edges[id].Warship {
						t.Fatal("wrong tail ship returned")
					}
				}
				if g.pirateFortressReady(1) {
					t.Fatal("lost route still ready")
				}
			case "win":
				if f.Strength != 2 || len(f.Route) != len(route) || g.Vertices[f.Vertex].Level != 0 {
					t.Fatal("ordinary win captured too early")
				}
			case "capture":
				_, afterPieces, _ := g.pieces(1)
				if f.Strength != 0 || g.Vertices[f.Vertex].Owner != 1 || g.Vertices[f.Vertex].Level != 1 || afterPieces != beforePieces {
					t.Fatal("captured settlement / piece reserve")
				}
				if s.catanAttackFortress(1, die) {
					t.Fatal("recaptured an already liberated fortress")
				}
			}
			restored := clone(*s)
			if !reflect.DeepEqual(*s, restored) {
				t.Fatal("battle persistence")
			}
		})
	}
}
func TestCatanPirateEndTurnAttacksThenHandsOffPairedPlayer(t *testing.T) {
	s := navyFixture(t, 6, 0)
	navyComplete(t, s)
	g := s.Catan
	for _, id := range g.pirateIslands().Fortresses[0].Route {
		g.Edges[id].Warship = true
	}
	g.Paired = &CatanPairedTurn{Primary: 0, Secondary: 3}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	g = s.Catan
	if g.pirateIslands().Battle == nil || g.pirateIslands().Battle.Player != 0 || s.Turn != 3 || s.Phase != "catan_turn" || !g.Paired.Second {
		t.Fatal("battle did not finish first paired action")
	}
	if g.pirateIslands().Fortresses[0].Strength != 2 {
		t.Fatal("fleet should beat any die")
	}
}
func TestCatanPirateCaptureRequiredForVictoryAndRemovesFinalFleet(t *testing.T) {
	s := navyFixture(t, 3, 0)
	navyComplete(t, s)
	g := s.Catan
	p := g.pirateIslands()
	for _, id := range p.Fortresses[0].Route {
		g.Edges[id].Warship = true
	}
	// Public score alone cannot win before the player's own fortress is captured.
	g.Players[0].Score = 12
	s.catanVictory()
	if s.Finished {
		t.Fatal("won without fortress")
	}
	for i := range p.Fortresses {
		p.Fortresses[i].Strength = 0
	}
	f := &p.Fortresses[0]
	f.Strength = 1
	g.Seafarers.Pirate = p.FleetPath[0]
	own := 0
	for i := range g.Vertices {
		if g.Vertices[i].Owner == 0 && g.Vertices[i].Level > 0 {
			g.Vertices[i].Level = 2
			own++
		}
	}
	for i := range g.Vertices {
		v := &g.Vertices[i]
		if v.ID != f.Vertex && v.ID != f.Beachhead && v.Level == 0 && own < 4 {
			v.Owner = 0
			v.Level = 2
			own++
		}
	}
	g.Vertices[f.Beachhead].Owner = 0
	g.Vertices[f.Beachhead].Level = 1
	s.catanScores()
	if g.Players[0].Score != 9 {
		t.Fatal("score fixture", g.Players[0].Score)
	}
	helperApply(t, s, 0, Action{Type: "catan_end"})
	g = s.Catan
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{0}) || g.Players[0].Score != 10 || g.Seafarers.Pirate != -1 {
		t.Fatal("capture win / fleet removal")
	}
	if g.ArmyOwner != -1 || g.LongestOwner != -1 {
		t.Fatal("disabled awards still granted")
	}
}
func TestCatanPirateBeachheadRequiresNavalArrivalAndFortressCannotBeBought(t *testing.T) {
	s := navyFixture(t, 4, 1)
	g := s.Catan
	f := g.pirateIslands().Fortresses[1]
	fleetGive(g, 1, []int{0, 2, 0, 2, 0})
	helperReject(t, s, 1, Action{Type: "catan_settlement", Vertex: f.Beachhead})
	helperReject(t, s, 1, Action{Type: "catan_settlement", Vertex: f.Vertex})
	for tries := 0; tries < 15; tries++ {
		vs, _ := s.Catan.pirateRouteVertices(1)
		if slices.Contains(vs, f.Beachhead) {
			break
		}
		navyExtend(t, s)
	}
	helperApply(t, s, 1, Action{Type: "catan_settlement", Vertex: f.Beachhead})
	g = s.Catan
	if g.Vertices[f.Beachhead].Level != 1 {
		t.Fatal("arrival did not unlock beachhead")
	}
	last := g.pirateIslands().Fortresses[1].Route[len(g.pirateIslands().Fortresses[1].Route)-1]
	g.Seafarers.BuiltShips = nil
	if g.movableShip(1, last) {
		t.Fatal("closed beachhead line allowed ship removal")
	}
	fleetSupply(t, g)
}

func TestCatanPirateVictoryCardKeepsIdentityButActsAsKnight(t *testing.T) {
	s := navyFixture(t, 4, 0)
	g := s.Catan
	pirateGiveDev(t, g, 0, 4)
	s.catanScores()
	if g.Players[0].Score != 1 || g.hiddenVictoryPoints(0) != 0 {
		t.Fatal("converted card scored as a victory point")
	}
	for _, viewer := range []int{-1, 0, 1, 2, 3} {
		p := s.View(viewer)["catan"].(map[string]any)["players"].([]any)[0].(map[string]any)
		if p["publicScore"] != 1 || fmt.Sprint(p["score"]) != "1" {
			t.Fatal("public score disclosed/removed treated-knight count", viewer, p)
		}
	}
	g.Players[0].NewDev[4] = 1
	helperReject(t, s, 0, Action{Type: "catan_dev", Card: 4})
	g.Players[0].NewDev[4] = 0
	s.Phase = "catan_roll"
	a, err := s.catanBot(0)
	if err != nil || a.Type != "catan_dev" || a.Card != 4 {
		t.Fatal("bot ignored victory-card knight", a, err)
	}
	helperApply(t, s, 0, a)
	g = s.Catan
	if g.Players[0].Dev[4] != 0 || g.DevDiscard[len(g.DevDiscard)-1] != 4 || !g.Edges[g.pirateIslands().Fortresses[0].Route[0]].Warship || s.Phase != "catan_roll" || g.Players[0].Score != 1 {
		t.Fatal("victory-card identity/use")
	}
	// Ryan compares actual public points, without subtracting an opponent's
	// secretly held victory-card knights as if they were hidden points.
	s.Phase = "catan_turn"
	g.Options.Helpers = true
	g.TurnSerial = 3
	g.Players[0].Helper = &CatanHelperSeat{ID: 7}
	g.Vertices[g.pirateIslands().Fortresses[1].StartVertex].Level = 2
	pirateGiveDev(t, g, 1, 4)
	pirateGiveDev(t, g, 1, 4)
	fleetGive(g, 1, []int{0, 0, 0, 0, 1})
	s.catanScores()
	helperApply(t, s, 0, Action{Type: "catan_helper", Target: 1})
	if s.Catan.HelperPending == nil || s.Catan.HelperPending.Kind != "leader" {
		t.Fatal("treated knight cards broke Ryan comparison")
	}
}
func TestCatanPirateFreeRoadBotCanUseAuxiliaryShip(t *testing.T) {
	s := navyFixture(t, 4, 1)
	navyComplete(t, s)
	g := s.Catan
	count := 0
	for i := range g.Edges {
		e := &g.Edges[i]
		if e.Owner < 0 && g.edgeTerrain(i, false) && count < 15 {
			e.Owner = 1
			e.Ship = false
			count++
		}
	}
	if count != 15 {
		t.Fatal("road supply fixture")
	}
	s.Phase = "catan_roads"
	g.FreeRoads = 2
	g.ResumePhase = "catan_turn"
	a, err := s.catanBot(1)
	if err != nil || a.Type != "catan_ship" || !g.pirateHomeCoast(a.Edge) {
		t.Fatal("free road bot omitted remaining coastal ship", a, err)
	}
	before := append([]int{}, g.Players[1].Resources...)
	route := append([]int{}, g.pirateIslands().Fortresses[1].Route...)
	helperApply(t, s, 1, a)
	g = s.Catan
	if !reflect.DeepEqual(before, g.Players[1].Resources) || !reflect.DeepEqual(route, g.pirateIslands().Fortresses[1].Route) {
		t.Fatal("free auxiliary ship changed cost/navy")
	}
}
