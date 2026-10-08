package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func riverKnightGame(t *testing.T, n int, events bool) *State {
	t.Helper()
	s, e := NewCatanRiversCitiesKnights(n, CatanOptions{FiveSix: n > 4})
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
func riverKnightRestore(t *testing.T, s *State) {
	t.Helper()
	b, e := json.Marshal(s)
	if e != nil {
		t.Fatal(e)
	}
	var next State
	if e = json.Unmarshal(b, &next); e != nil {
		t.Fatal(e)
	}
	if e = next.Catan.validateRivers(); e != nil {
		t.Fatal(e)
	}
	if next.Catan.Two != nil {
		if e = next.validateCatanTwo(); e != nil {
			t.Fatal(e)
		}
	}
	if e = next.validateCatanEventSession(); e != nil {
		t.Fatal(e)
	}
	if e = next.validateCityProgressInventory(); e != nil {
		t.Fatal(e)
	}
	*s = next
}
func TestCatanRiversKnightsNaturalGames(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, events := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/events%t", n, events), func(t *testing.T) {
				s := riverKnightGame(t, n, events)
				seen := map[string]int{}
				for step := 0; step < 16000 && !s.Finished; step++ {
					p := twoFullActor(s)
					a, e := s.BotAction(p)
					if e != nil {
						trial := clone(*s)
						end := trial.Apply(p, Action{Type: "catan_end"})
						t.Fatal(step, s.Phase, e, "end", end)
					}
					if e = s.Apply(p, a); e != nil {
						t.Fatal(step, s.Phase, a, e)
					}
					seen[a.Type]++
					if step%79 == 0 {
						riverKnightRestore(t, s)
					}
				}
				if !s.Finished {
					t.Fatal("unfinished", s.Round, s.Phase, seen)
				}
				riverKnightRestore(t, s)
				if s.Catan.victoryTarget() != 13 {
					t.Fatal("target")
				}
				t.Log("rounds", s.Round, seen)
			})
		}
	}
}

func riverKnightReady(t *testing.T, n int) *State {
	t.Helper()
	s := riverKnightGame(t, n, false)
	finishFishHelperSetup(t, s)
	for s.Phase != "catan_turn" {
		twoSeaStep(t, s)
	}
	riverKnightRestore(t, s)
	return s
}
func riverKnightGold(s *State, p, amount int) {
	g := s.Catan
	g.Rivers.Bank += g.Rivers.Gold[p] - amount
	g.Rivers.Gold[p] = amount
	s.catanScores()
}
func TestCatanRiversKnightsOpeningGoldAndFirstRobber(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riverKnightGame(t, n, false)
			cities := 0
			for s.Catan.setup() {
				g := s.Catan
				p := s.Turn
				a, e := s.BotAction(p)
				if e != nil {
					t.Fatal(e)
				}
				before := g.Rivers.Gold[p]
				delta := 0
				if (a.Type == "catan_settlement" || a.Type == "catan_city") && g.riverVertex(a.Vertex) {
					delta = 1
					if a.Type == "catan_city" {
						cities++
					}
				}
				if a.Type == "catan_road" && g.riverEdge(a.Edge) {
					delta = 1
				}
				helperApply(t, s, p, a)
				if s.Catan.Rivers.Gold[p] != before+delta {
					t.Fatal("opening river gold", a, before, s.Catan.Rivers.Gold[p])
				}
				if s.Catan.Robber != -1 {
					t.Fatal("robber entered before invasion")
				}
				riverKnightRestore(t, s)
			}
			t.Log("river starting cities", cities)
		})
	}
}
func TestCatanRiversKnightsCommoditiesAndInvention(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riverKnightReady(t, n)
			p := s.Turn
			g := s.Catan
			riverKnightGold(s, p, 8)
			g.CitiesKnights.Players[p].Improvements[CatanCommerce] = 3
			hand := []int{0, 0, 0, 0, 0, 0, 2, 0}
			twoKnightsHand(s, p, hand)
			if g.rates(p)[6] != 2 {
				t.Fatal("commodity rate")
			}
			helperApply(t, s, p, Action{Type: "catan_coin_sell", Color: 6})
			if s.Catan.Rivers.Gold[p] != 9 || s.Catan.Players[p].Resources[6] != 0 {
				t.Fatal("commodity sale")
			}
			helperReject(t, s, p, Action{Type: "catan_coin_buy", Color: 6})
			for range 2 {
				helperApply(t, s, p, Action{Type: "catan_coin_buy", Color: 0})
			}
			helperReject(t, s, p, Action{Type: "catan_coin_buy", Color: 1})
			riverKnightRestore(t, s)
			g = s.Catan
			choices := g.inventionNumbers()
			left, right := choices[0], choices[1]
			for _, c := range choices {
				if c.Number != left.Number {
					right = c
					break
				}
			}
			ckProgressGive(t, s, p, 3)
			helperApply(t, s, p, Action{Type: "catan_progress", Card: 3, Tile: left.Tile, Target: right.Tile})
			riverKnightRestore(t, s)
			if s.Catan.Tiles[left.Tile].Number != right.Number {
				t.Fatal("numbers not swapped")
			}
			bad := clone(*s)
			bad.Catan.Rivers.Map.NumberSwaps = nil
			if bad.Catan.validateRivers() == nil {
				t.Fatal("missing swap ledger accepted")
			}
		})
	}
}
func TestCatanRiversKnightsGoldPreventsPillage(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, save := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/save%t", n, save), func(t *testing.T) {
				s := riverKnightGame(t, n, false)
				finishFishHelperSetup(t, s)
				s.Catan.CitiesKnights.BarbarianPosition = 6
				if e := s.catanCityRoll(1, 2, 3); e != nil {
					t.Fatal(e)
				}
				p := s.CatanPendingActor()
				if s.Phase != "catan_pillage" {
					t.Fatal("missing pillage", s.Phase)
				}
				site := s.Catan.pillageSites(p)[0]
				s.Catan.CitiesKnights.Walls = append(s.Catan.CitiesKnights.Walls, site)
				riverKnightGold(s, p, 4)
				helperReject(t, s, p, Action{Type: "catan_pillage", Choice: "gold"})
				riverKnightGold(s, p, 5)
				riverKnightRestore(t, s)
				for _, viewer := range []int{-1, p, (p + 1) % n} {
					v := s.View(viewer)["catan"].(map[string]any)["rivers"].(map[string]any)
					if v["canProtectCity"] != (viewer == p) {
						t.Fatal("response ownership")
					}
				}
				if save {
					helperApply(t, s, p, Action{Type: "catan_pillage", Choice: "gold"})
				} else {
					helperApply(t, s, p, Action{Type: "catan_pillage", Vertex: site})
				}
				want := 1
				gold := 5
				if save {
					want = 2
					gold = 0
				}
				if s.Catan.Vertices[site].Level != want || s.Catan.Rivers.Gold[p] != gold {
					t.Fatal("protection payment or city")
				}
				riverKnightRestore(t, s)
				for s.Phase == "catan_pillage" {
					twoSeaStep(t, s)
				}
				if s.Catan.CitiesKnights.Invasions != 1 || s.Catan.Robber != s.Catan.CitiesKnights.RobberStart {
					t.Fatal("first invasion continuation")
				}
				riverKnightRestore(t, s)
			})
		}
	}
}

func TestCatanRiversKnightsBridgeTravelAndDiplomacy(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := riverKnightReady(t, n)
			g := s.Catan
			p := s.Turn
			for i := range g.Edges {
				g.Edges[i].Owner = -1
				g.Edges[i].Bridge = false
			}
			bridge := -1
			for _, id := range g.Rivers.Map.Bridges {
				e := g.Edges[id]
				if g.Vertices[e.A].Level == 0 && g.Vertices[e.B].Level == 0 {
					bridge = id
					break
				}
			}
			if bridge < 0 {
				t.Fatal("no empty bridge")
			}
			e := g.Edges[bridge]
			g.Edges[bridge].Owner = p
			g.Edges[bridge].Bridge = true
			g.CitiesKnights.Knights = []CatanKnight{{Owner: p, Vertex: e.A, Strength: 2, Active: true}, {Owner: (p + 1) % n, Vertex: e.B, Strength: 1}}
			if slices.Contains(g.diplomacyRoadsFor(p), bridge) {
				t.Fatal("bridge offered for diplomacy")
			}
			ckProgressGive(t, s, p, 16)
			helperReject(t, s, p, Action{Type: "catan_progress", Card: 16, Edge: bridge})
			helperApply(t, s, p, Action{Type: "catan_knight_move", Vertex: e.A, Target: e.B})
			if s.Catan.knightAt(e.B) == nil || s.Catan.knightAt(e.B).Owner != p || s.Catan.knightAt(e.B).Active {
				t.Fatal("bridge displacement")
			}
			riverKnightRestore(t, s)
		})
	}
	for _, ownerSelf := range []bool{false, true} {
		t.Run(fmt.Sprint("own", ownerSelf), func(t *testing.T) {
			s := riverKnightReady(t, 3)
			g := s.Catan
			p := s.Turn
			for i := range g.Edges {
				g.Edges[i].Owner = -1
				g.Edges[i].Bridge = false
			}
			road := -1
			for _, e := range g.Edges {
				if !slices.Contains(g.Rivers.Map.Bridges, e.ID) && g.riverEdge(e.ID) && g.Vertices[e.A].Level == 0 && g.Vertices[e.B].Level == 0 {
					road = e.ID
					break
				}
			}
			if road < 0 {
				t.Fatal("no river road")
			}
			owner := (p + 1) % 3
			if ownerSelf {
				owner = p
			}
			g.Edges[road].Owner = owner
			riverKnightGold(s, p, 0)
			ckProgressGive(t, s, p, 16)
			helperReject(t, s, p, Action{Type: "catan_progress", Card: 16, Edge: road})
			riverKnightGold(s, p, 1)
			helperApply(t, s, p, Action{Type: "catan_progress", Card: 16, Edge: road})
			if s.Catan.Rivers.Gold[p] != 0 || s.Catan.Edges[road].Owner != -1 {
				t.Fatal("river diplomacy fee")
			}
			if ownerSelf && s.Catan.CitiesKnights.Pending != nil {
				g = s.Catan
				choices := g.diplomacyPlacements(p)
				id := choices[0]
				for _, e := range choices {
					if g.riverEdge(e) {
						id = e
						break
					}
				}
				reward := 0
				if g.riverEdge(id) {
					reward = 1
				}
				helperApply(t, s, p, Action{Type: "catan_diplomacy", Edge: id})
				if s.Catan.Rivers.Gold[p] != reward {
					t.Fatal("rebuilt river road reward")
				}
			}
			riverKnightRestore(t, s)
		})
	}
}

func TestCatanRiversKnightsPillageWealthVictory(t *testing.T) {
	s := riverKnightReady(t, 3)
	g := s.Catan
	p := s.Turn
	for i := range g.Players {
		riverKnightGold(s, i, 8)
	}
	g.CitiesKnights.Players[p].DefenderPoints = 11
	for _, v := range g.Vertices {
		if g.knightRecruitable(p, v.ID) {
			g.CitiesKnights.Knights = []CatanKnight{{Owner: p, Vertex: v.ID, Strength: 1, Active: true}}
			break
		}
	}
	if len(g.CitiesKnights.Knights) != 1 {
		t.Fatal("missing legal knight site")
	}
	s.catanScores()
	if g.Players[p].Score != 12 {
		t.Fatal("wealth fixture", g.Players[p].Score)
	}
	s.Phase = "catan_roll"
	g.CitiesKnights.BarbarianPosition = 6
	if err := s.catanCityRoll(1, 2, 3); err != nil {
		t.Fatal(err)
	}
	q := s.CatanPendingActor()
	if s.Phase != "catan_pillage" || q == p {
		t.Fatal("wrong pillage responder")
	}
	last := (q + 1) % 3
	lastCity := g.pillageSites(last)[0]
	helperApply(t, s, q, Action{Type: "catan_pillage", Choice: "gold"})
	if !s.Finished || !slices.Equal(s.Winners, []int{p}) || s.Catan.Vertices[lastCity].Level != 2 {
		t.Fatal("wealth victory must precede the remaining pillage and production")
	}
	if s.Catan.CitiesKnights.Event != nil || s.Catan.CitiesKnights.Pending != nil || s.Catan.CitiesKnights.Invasions != 1 {
		t.Fatal("invasion cleanup at victory")
	}
	riverKnightRestore(t, s)
}
