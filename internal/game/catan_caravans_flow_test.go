package game

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func caravanFixture(t *testing.T, n int) *State {
	t.Helper()
	s, err := NewCatanCaravans(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		t.Fatal(err)
	}
	s.Turn, s.Catan.StartPlayer = 0, 0
	if s.Catan.Paired != nil {
		s.Catan.Paired.Primary, s.Catan.Paired.Secondary = 0, 3
	}
	s.Catan.SetupStep = s.Catan.SetupLimit()
	s.Phase = "catan_turn"
	return s
}
func caravanConserved(t *testing.T, s *State) {
	t.Helper()
	if err := s.validateCaravans(); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	stock, devStock := 19, 25
	if len(g.Players) > 4 {
		stock, devStock = 24, 34
	}
	for color, bank := range g.Bank {
		total := bank
		if bank < 0 {
			t.Fatal("negative bank")
		}
		for p, seat := range g.Players {
			if seat.Resources[color] < 0 {
				t.Fatal("negative hand")
			}
			total += seat.Resources[color]
			if q := g.Caravans.Pending; q != nil && q.Bids[p] != nil {
				total += q.Bids[p][color]
			}
		}
		if total != stock {
			t.Fatal("resource conservation", color, total)
		}
	}
	dev := len(g.DevDeck) + len(g.DevDiscard)
	for p, seat := range g.Players {
		dev += sum(seat.Dev)
		roads, villages, cities := g.pieces(p)
		if roads > 15 || villages > 5 || cities > 4 {
			t.Fatal("pieces exhausted")
		}
	}
	if dev != devStock {
		t.Fatal("development conservation")
	}
}
func caravanRestore(t *testing.T, s *State) {
	t.Helper()
	before, _ := json.Marshal(s)
	var restored State
	if err := json.Unmarshal(before, &restored); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(restored)
	if string(before) != string(after) {
		t.Fatal("restore changed state")
	}
	*s = restored
	caravanConserved(t, s)
}

func TestCatanCaravansVoteOutcomesAndPrivacy(t *testing.T) {
	for _, tc := range []struct {
		name            string
		bids            []int
		votes           []int
		chooser, winner int
	}{
		{"all pass", []int{0, 0, 0, 0}, nil, 0, 1},
		{"strict majority", []int{1, 3, 0, 0}, nil, 1, 2},
		{"coalition", []int{3, 2, 2, 0}, []int{0, 1, 1, -1}, -1, 1},
		{"location tie individual leader", []int{2, 1, 1, 0}, []int{0, 1, 1, -1}, 0, 2},
		{"location and player ties", []int{0, 1, 1, 0}, []int{-1, 0, 1, -1}, 0, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := caravanFixture(t, 4)
			choices := s.Catan.Caravans.choices(s.Catan)
			for p, n := range tc.bids {
				catanGive(s.Catan, p, 2, n)
			}
			s.Catan.Caravans.Built = true
			helperApply(t, s, 0, Action{Type: "catan_end"})
			for p, n := range tc.bids {
				if s.Turn != 0 || s.CatanPendingActor() != p {
					t.Fatal("wrong bidder/turn")
				}
				helperReject(t, s, (p+1)%4, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 0, 0, 0}})
				helperReject(t, s, p, Action{Type: "catan_end"})
				helperReject(t, s, p, Action{Type: "catan_caravan_bid", Tokens: []int{1, 0, 0, 0, 0}})
				helperReject(t, s, p, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, n + 1, 0, 0}})
				helperApply(t, s, p, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, n, 0, 0}})
				caravanRestore(t, s)
				for viewer := -1; viewer < 4; viewer++ {
					v := s.View(viewer)["catan"].(map[string]any)
					c := v["caravans"].(map[string]any)
					if c["canAct"] != (viewer >= 0 && viewer == s.CatanPendingActor()) {
						t.Fatal("response permission")
					}
					if int(c["pending"].(map[string]any)["bids"].([]any)[p].([]any)[2].(float64)) != n {
						t.Fatal("bid not public")
					}
					for other, raw := range v["players"].([]any) {
						if other != viewer && raw.(map[string]any)["resources"] != nil {
							t.Fatal("hand leak")
						}
					}
				}
			}
			for p, index := range tc.votes {
				if index < 0 {
					continue
				}
				if s.CatanPendingActor() != p || s.Phase != "catan_caravan_vote" {
					t.Fatal("wrong voter", p, s.Phase)
				}
				w := choices[index]
				helperReject(t, s, p, Action{Type: "catan_caravan_vote", Edge: w.Edge, Vertex: -1})
				helperApply(t, s, p, Action{Type: "catan_caravan_vote", Edge: w.Edge, Vertex: w.From})
				caravanRestore(t, s)
			}
			if tc.chooser >= 0 {
				if s.CatanPendingActor() != tc.chooser || s.Phase != "catan_caravan_place" {
					t.Fatal("wrong decider", s.CatanPendingActor())
				}
				w := choices[tc.winner]
				helperApply(t, s, tc.chooser, Action{Type: "catan_caravan_place", Edge: w.Edge, Vertex: w.From})
			}
			if len(s.Catan.Caravans.Wagons) != 1 || s.Catan.Caravans.Wagons[0] != choices[tc.winner] || s.Catan.Caravans.Pending != nil || s.Turn != 1 || s.Phase != "catan_roll" || s.Catan.Bank[2] != 19 {
				t.Fatal("vote completion", s.Phase, s.Turn)
			}
			caravanRestore(t, s)
		})
	}
}

func TestCatanCaravansBuildingTriggerPairedAndRobber(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := NewCatanCaravans(n, CatanOptions{FiveSix: n > 4})
			if err != nil {
				t.Fatal(err)
			}
			for s.Catan.setup() {
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, s.Turn, a)
				if s.Catan.Caravans.Built || s.Catan.Caravans.Pending != nil {
					t.Fatal("setup triggered wagon")
				}
				caravanRestore(t, s)
			}
			if s.Catan.Robber != -1 {
				t.Fatal("robber starts on board")
			}
			catanCard(s.Catan, s.Turn, 0)
			helperApply(t, s, s.Turn, Action{Type: "catan_dev", Card: 0})
			if s.Phase != "catan_robber" {
				t.Fatal("knight phase")
			}
			helperApply(t, s, s.Turn, Action{Type: "catan_robber", Tile: s.Catan.Caravans.Map.WateringHoles[0]})
			for s.Phase != "catan_roll" {
				a, err := s.BotAction(s.Turn)
				if err != nil {
					t.Fatal(err)
				}
				helperApply(t, s, s.Turn, a)
			}
			s.Phase = "catan_turn" // Deterministic action fixture, no production grant.
			build := func() {
				g := s.Catan
				p := s.Turn
				for _, v := range g.Vertices {
					if v.Owner == p && v.Level == 1 {
						for color, count := range catanPrices["catan_city"] {
							catanGive(s.Catan, p, color, max(0, count-s.Catan.Players[p].Resources[color]))
						}
						helperApply(t, s, p, Action{Type: "catan_city", Vertex: v.ID})
					}
				}
				if !s.Catan.Caravans.Built || s.Catan.Caravans.Pending != nil {
					t.Fatal("building phase marker")
				}
			}
			build()
			active := s.Turn
			oldSerial := s.Catan.TurnSerial
			helperApply(t, s, active, Action{Type: "catan_end"})
			if s.Turn != active || s.Catan.Caravans.Sequence != 1 {
				t.Fatal("vote moved action owner")
			}
			for steps := 0; s.CatanPendingActor() >= 0 && steps < 20; steps++ {
				s.AutoCatanPending()
				caravanRestore(t, s)
			}
			if len(s.Catan.Caravans.Wagons) != 1 || s.CatanPendingActor() >= 0 {
				t.Fatal("two buildings must place one wagon")
			}
			if n > 4 {
				if !s.Catan.Paired.Second || s.Phase != "catan_turn" || s.Catan.TurnSerial != oldSerial {
					t.Fatal("paired transition")
				}
				secondary := s.Turn
				build()
				helperApply(t, s, secondary, Action{Type: "catan_end"})
				if s.Catan.Caravans.Pending.Active != secondary || s.CatanPendingActor() != secondary {
					t.Fatal("secondary vote not independent")
				}
				for steps := 0; s.CatanPendingActor() >= 0 && steps < 20; steps++ {
					s.AutoCatanPending()
					caravanRestore(t, s)
				}
				if s.Catan.Paired.Second || s.Catan.TurnSerial != oldSerial+1 || len(s.Catan.Caravans.Wagons) != 2 {
					t.Fatal("secondary completion")
				}
			}
			if s.Catan.Caravans.Built || s.Catan.Caravans.Pending != nil {
				t.Fatal("stale vote")
			}
		})
	}
}

func TestCatanCaravansScoresVictoryAndElimination(t *testing.T) {
	for _, owner := range []int{0, 2} {
		t.Run(fmt.Sprint(owner), func(t *testing.T) {
			s := caravanFixture(t, 4)
			g := s.Catan
			c := g.Caravans
			first := c.Map.Starts[0]
			if err := c.place(g, first); err != nil {
				t.Fatal(err)
			}
			edge := g.Edges[first.Edge]
			target := edge.A
			if target == first.From {
				target = edge.B
			}
			var second catanCaravanWagon
			for _, w := range c.choices(g) {
				if w.From == target {
					second = w
					break
				}
			}
			g.Vertices[target].Owner, g.Vertices[target].Level = owner, 1
			count := 1
			for i := range g.Vertices {
				if count < 7 && g.canSettlement(owner, i, true) {
					g.Vertices[i].Owner = owner
					g.Vertices[i].Level = 1
					if count <= 4 {
						g.Vertices[i].Level = 2
					}
					count++
				}
			}
			if count != 7 {
				t.Fatal("score fixture")
			}
			s.catanScores()
			if g.Players[owner].Score != 11 || g.victoryTarget() != 12 {
				t.Fatal("pre-wagon score", g.Players[owner].Score)
			}
			c.Built = true
			helperApply(t, s, 0, Action{Type: "catan_end"})
			before := clone(*s)
			if s.EliminateCatan(0) == nil || !reflect.DeepEqual(*s, before) {
				t.Fatal("removed during required vote")
			}
			for p := 0; p < 4; p++ {
				helperApply(t, s, p, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 0, 0, 0}})
			}
			helperApply(t, s, 0, Action{Type: "catan_caravan_place", Edge: second.Edge, Vertex: second.From})
			if s.Catan.Players[owner].Score != 12 {
				t.Fatal("wagon building bonus")
			}
			if owner == 0 {
				if !s.Finished || !slices.Equal(s.Winners, []int{0}) {
					t.Fatal("missed own-turn victory")
				}
			} else {
				if s.Finished || s.Turn != 1 {
					t.Fatal("out of turn victory")
				}
				s.Phase = "catan_turn"
				helperApply(t, s, 1, Action{Type: "catan_end"})
				if !s.Finished || !slices.Equal(s.Winners, []int{2}) {
					t.Fatal("next-own-turn victory")
				}
			}
			caravanRestore(t, s)
		})
	}
	s := caravanFixture(t, 3)
	s.Catan.Caravans.Built = true
	if err := s.EliminateCatan(0); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Caravans.Built || s.Catan.Caravans.Pending != nil || s.Turn != 1 {
		t.Fatal("elimination left queued wagon")
	}
	base := catanGame(t, 3)
	helperReject(t, base, 0, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 0, 0, 0}})
}

func TestCatanCaravansFullBots(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s, err := NewCatanCaravans(n, CatanOptions{FiveSix: n > 4})
			if err != nil {
				t.Fatal(err)
			}
			counts := map[string]int{}
			steps := 0
			for ; steps < 5000 && !s.Finished; steps++ {
				p := s.Turn
				if actor := s.CatanPendingActor(); actor >= 0 {
					p = actor
				} else if s.Phase == "catan_discard" {
					for i, due := range s.Catan.DiscardDue {
						if due > 0 {
							p = i
							break
						}
					}
				}
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatalf("step %d: %v", steps, err)
				}
				counts[a.Type]++
				helperApply(t, s, p, a)
				caravanConserved(t, s)
				if steps%31 == 0 || s.CatanPendingActor() >= 0 {
					caravanRestore(t, s)
				}
			}
			if !s.Finished || s.Catan.Caravans.Sequence == 0 || len(s.Catan.Caravans.Wagons) == 0 || s.Catan.Caravans.Pending != nil || s.Catan.Players[s.Turn].Score < 12 {
				t.Fatal("incomplete game", steps, s.Phase)
			}
			t.Logf("steps=%d wagons=%d bids=%d votes=%d choices=%d", steps, len(s.Catan.Caravans.Wagons), counts["catan_caravan_bid"], counts["catan_caravan_vote"], counts["catan_caravan_place"])
		})
	}
}

func TestCatanCaravansWeightedRoadAndNoWagonSupply(t *testing.T) {
	s := caravanFixture(t, 3)
	g := s.Catan
	c := g.Caravans
	first := c.Map.Starts[0]
	if err := c.place(g, first); err != nil {
		t.Fatal(err)
	}
	e := g.Edges[first.Edge]
	joint := e.A
	if joint == first.From {
		joint = e.B
	}
	var second catanCaravanWagon
	for _, w := range c.choices(g) {
		if w.From == joint {
			second = w
			break
		}
	}
	if err := c.place(g, second); err != nil {
		t.Fatal(err)
	}
	e = g.Edges[second.Edge]
	end := e.A
	if end == second.From {
		end = e.B
	}
	third := -1
	for _, id := range g.touching(end) {
		if id != second.Edge && id != first.Edge {
			third = id
			break
		}
	}
	if third < 0 {
		t.Fatal("three-road fixture")
	}
	for _, id := range []int{first.Edge, second.Edge, third} {
		g.Edges[id].Owner = 0
	}
	s.catanScores()
	if g.Players[0].RoadLength != 5 || g.LongestOwner != 0 || g.Players[0].Score != 2 {
		t.Fatal("three roads and two wagons must count five", g.Players[0])
	}
	g.Vertices[joint].Owner, g.Vertices[joint].Level = 1, 1
	s.catanScores()
	if g.Players[0].RoadLength != 3 || g.LongestOwner != -1 || g.Players[1].Score != 2 {
		t.Fatal("opponent cuts road but receives one wagon bonus")
	}
	g.Vertices[joint].Level = 2
	s.catanScores()
	if g.Players[1].Score != 3 {
		t.Fatal("city wagon bonus repeated")
	}
	// No construction: no bid or wagon, even with already existing buildings.
	helperApply(t, s, 0, Action{Type: "catan_end"})
	if s.CatanPendingActor() != -1 || s.Catan.Caravans.Sequence != 0 {
		t.Fatal("no-building turn opened vote")
	}
	// Fill or block a real directed network; no fictitious reduced supply.
	g = s.Catan
	c = g.Caravans
	for len(c.choices(g)) > 0 {
		if err := c.place(g, c.choices(g)[0]); err != nil {
			t.Fatal(err)
		}
	}
	s.Phase = "catan_turn"
	c.Built = true
	count := len(c.Wagons)
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	if s.CatanPendingActor() != -1 || s.Catan.Caravans.Built || len(s.Catan.Caravans.Wagons) != count {
		t.Fatal("no-space vote charged cards or stalled")
	}
	caravanRestore(t, s)
}

func TestCatanCaravansCorruptVoteAndBotPrivacy(t *testing.T) {
	s := caravanFixture(t, 4)
	for p := range s.Catan.Players {
		catanGive(s.Catan, p, 2, 3)
	}
	s.Catan.Caravans.Built = true
	helperApply(t, s, 0, Action{Type: "catan_end"})
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Caravans.Pending.Active = 2 },
		func(s *State) { s.Catan.Caravans.Pending.Cursor = 9 },
		func(s *State) { s.Catan.Caravans.Pending.Order = []int{0, 1, 1, 3} },
		func(s *State) { s.Catan.Caravans.Pending.Bids[2] = []int{0, 0, 1, 0, 0} },
		func(s *State) { s.Catan.Caravans.Pending.Votes[0] = &s.Catan.Caravans.Map.Starts[0] },
		func(s *State) { s.Phase = "catan_turn" },
		func(s *State) { s.Catan.Bank = append(s.Catan.Bank, 1) },
		func(s *State) { s.Catan.Players[2].Resources = []int{0} },
	} {
		bad := clone(*s)
		mutate(&bad)
		helperReject(t, &bad, 0, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 0, 0, 0}})
	}
	for stage := 0; stage < 2; stage++ {
		actor := s.CatanPendingActor()
		before := clone(*s)
		a, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(*s, before) {
			t.Fatal("bot mutated state")
		}
		other := clone(*s)
		slices.Reverse(other.Catan.DevDeck)
		for p := range other.Catan.Players {
			if p != actor {
				other.Catan.Players[p].Resources = []int{9, 8, 7, 6, 5}
				other.Catan.Players[p].Dev = []int{1, 2, 3, 4, 5}
			}
		}
		b, err := other.BotAction(actor)
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("bot used rivals' hidden information")
		}
		if stage == 0 {
			for p := 0; p < 4; p++ {
				helperApply(t, s, p, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 1, 0, 0}})
			}
		}
	}
}

func TestCatanCaravansMixedBidsAndDepartedSeats(t *testing.T) {
	s := caravanFixture(t, 4)
	s.Catan.Players[1].Eliminated = true
	s.Turn = 2
	s.Catan.Caravans.Built = true
	catanGive(s.Catan, 2, 2, 1)
	catanGive(s.Catan, 2, 3, 2)
	helperApply(t, s, 2, Action{Type: "catan_end"})
	if !slices.Equal(s.Catan.Caravans.Pending.Order, []int{2, 3, 0}) {
		t.Fatal("clockwise active-seat order")
	}
	helperApply(t, s, 2, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 1, 2, 0}})
	if s.Catan.Bank[2] != 18 || s.Catan.Bank[3] != 17 || sum(s.Catan.Players[2].Resources) != 0 {
		t.Fatal("bids not held aside")
	}
	for _, p := range []int{3, 0} {
		helperApply(t, s, p, Action{Type: "catan_caravan_bid", Tokens: []int{0, 0, 0, 0, 0}})
	}
	q := s.Catan.Caravans.Pending
	if q.Chooser != 2 || s.CatanPendingActor() != 2 {
		t.Fatal("mixed resources not counted")
	}
	for _, mutate := range []func(*State){
		func(s *State) { s.Catan.Caravans.Pending.Chooser = 0 },
		func(s *State) {
			s.Catan.Caravans.Pending.Kind = "vote"
			s.Phase = "catan_caravan_vote"
			s.Catan.Caravans.Pending.Cursor = 0
			s.Catan.Caravans.Pending.Chooser = -1
		},
		func(s *State) { s.Catan.Caravans.Pending.Votes[2] = &s.Catan.Caravans.Map.Starts[0] },
	} {
		bad := clone(*s)
		mutate(&bad)
		w := bad.Catan.Caravans.Map.Starts[0]
		helperReject(t, &bad, bad.CatanPendingActor(), Action{Type: bad.Phase, Tokens: []int{0, 0, 0, 0, 0}, Edge: w.Edge, Vertex: w.From})
	}
	w := s.Catan.Caravans.Map.Starts[0]
	helperApply(t, s, 2, Action{Type: "catan_caravan_place", Edge: w.Edge, Vertex: w.From})
	if s.Catan.Bank[2] != 19 || s.Catan.Bank[3] != 19 || s.Turn != 3 {
		t.Fatal("mixed bid settlement")
	}
	caravanRestore(t, s)
}
