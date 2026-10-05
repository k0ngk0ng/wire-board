package game

import (
	"encoding/json"
	"math"
	"testing"
)

func pairedGame(t *testing.T, n int, helpers bool) *State {
	t.Helper()
	s, err := NewCatan(n, CatanOptions{FiveSix: true, Helpers: helpers, AllHelpers: helpers})
	if err != nil {
		t.Fatal(err)
	}
	for s.Catan.setup() {
		actor := s.Turn
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, actor, a)
	}
	return s
}

func TestCatanFiveSixMapComponentsAndSetup(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := pairedGame(t, n, true)
		g := s.Catan
		if len(g.Tiles) != 30 || len(g.Vertices) != 80 || len(g.Edges) != 109 || len(g.Ports) != 11 {
			t.Fatalf("map topology: %d/%d/%d/%d", len(g.Tiles), len(g.Vertices), len(g.Edges), len(g.Ports))
		}
		terrain := make([]int, 6)
		numbers := make([]int, 13)
		cards := make([]int, 5)
		ports := make([]int, 6)
		for _, tile := range g.Tiles {
			terrain[tile.Resource]++
			numbers[tile.Number]++
		}
		for i, want := range []int{6, 5, 6, 6, 5, 2} {
			if terrain[i] != want {
				t.Fatal("terrain inventory", terrain)
			}
		}
		for _, num := range []int{2, 3, 4, 5, 6, 8, 9, 10, 11, 12} {
			want := 3
			if num == 2 || num == 12 {
				want = 2
			}
			if numbers[num] != want {
				t.Fatal("number inventory", numbers)
			}
		}
		for _, c := range g.DevDeck {
			cards[c]++
		}
		for i, want := range []int{20, 3, 3, 3, 5} {
			if cards[i] != want {
				t.Fatal("development inventory", cards)
			}
		}
		portVertices := map[int]bool{}
		for _, p := range g.Ports {
			ports[p.Resource+1]++
			for _, v := range []int{g.Edges[p.Edge].A, g.Edges[p.Edge].B} {
				if portVertices[v] {
					t.Fatal("overlapping ports")
				}
				portVertices[v] = true
			}
		}
		for i, want := range []int{5, 1, 1, 2, 1, 1} {
			if ports[i] != want {
				t.Fatal("port inventory", ports)
			}
		}
		for i, tile := range g.Tiles {
			if tile.Number != 6 && tile.Number != 8 {
				continue
			}
			for _, other := range g.Tiles[i+1:] {
				if (other.Number == 6 || other.Number == 8) && math.Hypot(tile.X-other.X, tile.Y-other.Y) < math.Sqrt(3)*g.HexSize+1 {
					t.Fatal("adjacent high production")
				}
			}
		}
		for i, p := range g.Players {
			roads, settlements, cities := g.pieces(i)
			if roads != 2 || settlements != 2 || cities != 0 || p.Helper.ID != (i-g.StartPlayer+n)%n+1 {
				t.Fatal("setup order or helper assignment", i, p.Helper)
			}
		}
		for c, count := range g.Bank {
			for _, p := range g.Players {
				count += p.Resources[c]
			}
			if count != 24 {
				t.Fatal("resource inventory", c, count)
			}
		}
		if s.Turn != g.StartPlayer || g.Paired.Primary != s.Turn || g.Paired.Secondary != (s.Turn+3)%n || g.Paired.Second {
			t.Fatal("initial markers")
		}
	}
}

func TestCatanPairedTurnOrderAndNoProductionOrPlayerTrade(t *testing.T) {
	s := pairedGame(t, 6, false)
	g := s.Catan
	start := s.Turn
	serial := g.TurnSerial
	s.Phase = "catan_turn"
	g.Players[g.Paired.Secondary].Dev[2] = 1
	g.Players[g.Paired.Secondary].NewDev[2] = 1
	g.PlayedDev = true
	helperApply(t, s, start, Action{Type: "catan_end"})
	g = s.Catan
	if s.Turn != (start+3)%6 || !g.Paired.Second || s.Phase != "catan_turn" || g.TurnSerial != serial || s.Round != 1 || g.PlayedDev || g.Players[s.Turn].NewDev[2] != 0 {
		t.Fatal("incorrect secondary phase")
	}
	helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
	helperGrant(s, s.Turn, []int{4, 0, 0, 0, 0})
	helperReject(t, s, s.Turn, Action{Type: "catan_trade_offer", Give: []int{1, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}})
	// Random setup can grant a 2:1/3:1 harbor. This test verifies the second
	// player's bank-trade permission, not a fixed no-harbor exchange rate.
	rate := s.Catan.rates(s.Turn)[0]
	helperApply(t, s, s.Turn, Action{Type: "catan_bank", Give: []int{rate, 0, 0, 0, 0}, Take: []int{0, 1, 0, 0, 0}})
	helperApply(t, s, s.Turn, Action{Type: "catan_dev", Card: 2, Take: []int{0, 0, 0, 1, 1}})
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	g = s.Catan
	if s.Turn != (start+1)%6 || g.Paired.Second || s.Phase != "catan_roll" || g.Paired.Secondary != (start+4)%6 || g.TurnSerial != serial+1 || g.PlayedDev {
		t.Fatal("paired markers did not rotate")
	}
	// One round contains six production turns and twelve action phases.
	for step := 1; step < 6; step++ {
		s.Phase = "catan_turn"
		helperApply(t, s, s.Turn, Action{Type: "catan_end"})
		helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	}
	if s.Round != 2 || s.Turn != start {
		t.Fatal("paired round drift")
	}
}

func TestCatanPairedHelpersAcquiredDuringProductionCannotPlayImmediately(t *testing.T) {
	s := pairedGame(t, 6, true)
	g := s.Catan
	secondary := g.Paired.Secondary
	g.Players[secondary].Helper = &CatanHelperSeat{ID: 11, AcquiredTurn: g.TurnSerial, UsedTurn: g.TurnSerial}
	s.Phase = "catan_turn"
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	helperReject(t, s, secondary, Action{Type: "catan_helper", Color: 0})
	helperApply(t, s, secondary, Action{Type: "catan_end"})
	if !s.Catan.helperReady(secondary, 11) {
		t.Fatal("helper stayed locked beyond paired turn")
	}
}

func TestCatanPairedVictoryAndElimination(t *testing.T) {
	for _, second := range []bool{false, true} {
		s := pairedGame(t, 5, false)
		s.Phase = "catan_turn"
		if second {
			helperApply(t, s, s.Turn, Action{Type: "catan_end"})
		}
		winner := s.Turn
		s.Catan.Players[winner].Score = 10
		helperApply(t, s, winner, Action{Type: "catan_end"})
		if !s.Finished || s.Winners[0] != winner {
			t.Fatal("missing immediate victory", second)
		}
	}
	s := pairedGame(t, 5, false)
	primary := s.Turn
	secondary := s.Catan.Paired.Secondary
	if err := s.EliminateCatan(primary); err != nil {
		t.Fatal(err)
	}
	if s.Turn != secondary || !s.Catan.Paired.Second {
		t.Fatal("primary departure skipped partner")
	}
	if err := s.EliminateCatan(secondary); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Paired.Second || s.Catan.Players[s.Turn].Eliminated || s.Phase != "catan_roll" {
		t.Fatal("secondary departure lost normal turn")
	}
}

func TestCatanPairedSaveRestoresMarkersAndBotsFinish(t *testing.T) {
	for _, n := range []int{5, 6} {
		s := pairedGame(t, n, true)
		for step := 0; step < 8000 && !s.Finished; step++ {
			actor := s.Turn
			if q := s.Catan.HelperPending; q != nil {
				actor = q.Player
			} else if s.Phase == "catan_discard" {
				for i, due := range s.Catan.DiscardDue {
					if due > 0 {
						actor = i
						break
					}
				}
			}
			a, err := s.BotAction(actor)
			if err != nil {
				t.Fatalf("n=%d step=%d phase=%s %v", n, step, s.Phase, err)
			}
			helperApply(t, s, actor, a)
			if step%50 == 0 {
				data, _ := json.Marshal(s)
				var restored State
				if err = json.Unmarshal(data, &restored); err != nil {
					t.Fatal(err)
				}
				s = &restored
			}
			for color, count := range s.Catan.Bank {
				for _, p := range s.Catan.Players {
					count += p.Resources[color]
				}
				if count != 24 {
					t.Fatal("resource conservation", color, count)
				}
			}
		}
		if !s.Finished {
			t.Fatal("paired bot game did not finish", n)
		}
	}
}

func TestCatanPairedDigurChoosesEitherDesert(t *testing.T) {
	s := pairedGame(t, 5, true)
	g := s.Catan
	actor := s.Turn
	g.Players[actor].Helper = &CatanHelperSeat{ID: 10}
	desert := -1
	for _, tile := range g.Tiles {
		if tile.Resource == 5 {
			desert = tile.ID
		} else {
			g.Robber = tile.ID
		}
	}
	helperReject(t, s, actor, Action{Type: "catan_helper", Choice: "desert", Tile: g.Robber})
	helperApply(t, s, actor, Action{Type: "catan_helper", Choice: "desert", Tile: desert})
	if s.Catan.Robber != desert {
		t.Fatal("Digur ignored selected desert")
	}
}
