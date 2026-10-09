package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

func caravansAttackRestore(t *testing.T, s *State) {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var next State
	if err = json.Unmarshal(raw, &next); err != nil {
		t.Fatal(err)
	}
	for _, check := range []func() error{next.validateCatanAttack, next.validateCaravans, next.validateCatanTwo, next.validateCatanEventSession} {
		if err = check(); err != nil {
			t.Fatal(err)
		}
	}
	*s = next
}
func TestCatanCaravansAttackNaturalEngine(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for _, knights := range []bool{false, true} {
			for _, events := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/knights%t/events%t", n, knights, events), func(t *testing.T) {
					s, err := NewCatanCaravansAttack(n, knights)
					if err != nil {
						t.Fatal(err)
					}
					t.Log("starts", s.Catan.Caravans.Map.Starts)
					if events {
						if err = s.EnableCatanEvents(CatanEventCatalogue); err != nil {
							t.Fatal(err)
						}
					}
					for step := 0; step < 15000 && !s.Finished; step++ {
						p := twoFullActor(s)
						a, err := s.BotAction(p)
						if err != nil {
							t.Fatal(step, s.Phase, err)
						}
						if err = s.Apply(p, a); err != nil {
							t.Fatal(step, s.Phase, a, err)
						}
						if step%41 == 0 {
							caravansAttackRestore(t, s)
							for viewer := -1; viewer < n; viewer++ {
								s.View(viewer)
							}
						}
					}
					if !s.Finished {
						t.Fatal("unfinished", s.Round)
					}
					if s.Catan.Caravans.Sequence == 0 {
						t.Fatal("no caravans")
					}
					caravansAttackRestore(t, s)
					t.Log("round", s.Round, "votes", s.Catan.Caravans.Sequence, "wagons", len(s.Catan.Caravans.Wagons))
				})
			}
		}
	}
}

func caravansAttackPlaying(t *testing.T, n int, knights bool) *State {
	t.Helper()
	s, err := NewCatanCaravansAttack(n, knights)
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 200 && (s.Catan.setup() || s.Phase != "catan_turn"); step++ {
		p := twoFullActor(s)
		a, e := s.BotAction(p)
		if e == nil {
			e = s.Apply(p, a)
		}
		if e != nil {
			t.Fatal(e)
		}
	}
	if s.Catan.setup() || s.Phase != "catan_turn" {
		t.Fatal("no action phase")
	}
	return s
}
func TestCatanCaravansAttackMapAndGuards(t *testing.T) {
	for n := 2; n <= 6; n++ {
		for repeat := 0; repeat < 12; repeat++ {
			s, err := NewCatanCaravansAttack(n, repeat%2 == 0)
			if err != nil {
				t.Fatal(err)
			}
			g := s.Catan
			m := g.Caravans.Map
			starts, supply := 2, 22
			if n > 4 {
				starts, supply = 5, 33
			}
			if len(m.Starts) != starts || m.Supply != supply {
				t.Fatal(n, "wrong starts/supply", m)
			}
			for _, id := range m.WateringHoles {
				if g.Tiles[id].Resource != catanWateringHole || g.Tiles[id].Number != 0 || g.Attack.Barbarians[id] != 0 {
					t.Fatal("waterhole terrain or production")
				}
			}
			caravansAttackRestore(t, s)
			for _, mutate := range []func(*State){
				func(q *State) { q.Catan.Caravans.Attack = "" }, func(q *State) { q.Catan.Attack.Map.Caravans = "" },
				func(q *State) { q.Catan.Caravans.Map.Starts[0].From++ }, func(q *State) { q.Catan.Caravans.Map.Supply++ },
				func(q *State) { q.Catan.Tiles[q.Catan.Caravans.Map.WateringHoles[0]].Resource = CatanDesert },
				func(q *State) { q.Catan.Caravans.Map.WateringHoles[0]++ },
			} {
				q := clone(*s)
				mutate(&q)
				if q.validateCatanAttack() == nil {
					t.Fatal("corrupt combination accepted")
				}
			}
		}
	}
	for _, knights := range []bool{false, true} {
		var s *State
		var err error
		if knights {
			s, err = NewCatanAttackCitiesKnights(3)
		} else {
			s, err = NewCatanAttack(3)
		}
		if err != nil {
			t.Fatal(err)
		}
		s.Phase = "catan_caravan_bid"
		if s.validateCatanAttack() == nil {
			t.Fatal("ordinary attack acquired caravan phase")
		}
	}
}
func TestCatanCaravansAttackTurnOrderAndEscrow(t *testing.T) {
	for _, n := range []int{2, 3, 6} {
		for _, knights := range []bool{false, true} {
			s := caravansAttackPlaying(t, n, knights)
			p, serial := s.Turn, s.Catan.TurnSerial
			s.Catan.Caravans.Built = true
			if err := s.Apply(p, Action{Type: "catan_end"}); err != nil {
				t.Fatal(err)
			}
			for step := 0; step < 60 && s.Phase != "catan_caravan_bid"; step++ {
				actor := twoFullActor(s)
				a, e := s.BotAction(actor)
				if e == nil {
					e = s.Apply(actor, a)
				}
				if e != nil {
					t.Fatal(e)
				}
			}
			if s.Phase != "catan_caravan_bid" || s.Turn != p || s.Catan.TurnSerial != serial {
				t.Fatal("vote missing or turn advanced before vote")
			}
			if knights {
				if s.Catan.Attack.City.End == nil {
					t.Fatal("vote before knight battles")
				}
			} else if s.Catan.Attack.End == nil {
				t.Fatal("vote before battles")
			}
			// Ensure a nonzero bid is held outside both the player's hand and bank,
			// with restoration accepting exactly this escrow and rejecting corruption.
			actor := twoFullActor(s)
			color := s.Catan.caravanBidColors()[0]
			if s.Catan.Bank[color] == 0 {
				t.Fatal("fixture bank empty")
			}
			s.Catan.Bank[color]--
			s.Catan.Players[actor].Resources[color]++
			bid := make([]int, 5)
			bid[color] = 1
			if err := s.Apply(actor, Action{Type: "catan_caravan_bid", Tokens: bid}); err != nil {
				t.Fatal(err)
			}
			caravansAttackRestore(t, s)
			broken := clone(*s)
			broken.Catan.Caravans.Pending.Bids[actor][color]++
			if broken.validateCatanAttack() == nil {
				t.Fatal("corrupt escrow accepted")
			}
			sequence := s.Catan.Caravans.Sequence
			for step := 0; step < 60 && s.Catan.TurnSerial == serial; step++ {
				actor := twoFullActor(s)
				a, e := s.BotAction(actor)
				if e == nil {
					e = s.Apply(actor, a)
				}
				if e != nil {
					t.Fatal(e)
				}
			}
			if s.Catan.TurnSerial != serial+1 || s.Catan.Caravans.Pending != nil || s.Catan.Caravans.Sequence != sequence {
				t.Fatal("turn skipped, vote repeated or vote pending")
			}
			if len(s.Catan.Caravans.Wagons) != map[bool]int{true: 2, false: 1}[n == 2] {
				t.Fatal("wrong wagon count")
			}
			caravansAttackRestore(t, s)
		}
	}
}
func TestCatanCaravansAttackConquestAndInvention(t *testing.T) {
	for _, knights := range []bool{false, true} {
		s := caravansAttackPlaying(t, 3, knights)
		g := s.Catan
		c := g.Caravans
		// Occupation blocks new ordinary construction, but cannot remove a legal
		// caravan placement or a previously placed wagon.
		choices := c.choices(g)
		for _, id := range g.Attack.Map.Coast {
			g.Attack.Barbarians[id] = 3
		}
		after := c.choices(g)
		if !slices.Equal(choices, after) {
			t.Fatal("barbarians changed wagon choices")
		}
		for i := 0; i < 3; i++ {
			if err := c.place(g, c.choices(g)[0]); err != nil {
				t.Fatal(err)
			}
		}
		for _, id := range g.Attack.Map.Coast {
			g.Attack.Barbarians[id] = 0
		}
		// Two incident wagons give +1 only while the associated building counts.
		vertex := -1
		for _, v := range g.Vertices {
			coastal, total := 0, 0
			for _, tile := range g.Tiles {
				if slices.Contains(tile.Vertices, v.ID) {
					total++
					if slices.Contains(g.Attack.Map.Coast, tile.ID) {
						coastal++
					}
				}
			}
			if coastal == 1 && total == 1 && len(g.touching(v.ID)) >= 2 {
				vertex = v.ID
				break
			}
		}
		if vertex < 0 {
			t.Fatal("no coastal corner")
		}
		for i := range g.Vertices {
			g.Vertices[i].Owner = -1
			g.Vertices[i].Level = 0
		}
		edges := g.touching(vertex)
		c.Wagons = []catanCaravanWagon{{Edge: edges[0], From: vertex}, {Edge: edges[1], From: vertex}}
		g.Vertices[vertex].Owner = 0
		g.Vertices[vertex].Level = 1
		s.catanScores()
		before := g.Players[0].Score
		for _, id := range g.Attack.Map.Coast {
			g.Attack.Barbarians[id] = 3
		}
		s.catanScores()
		if g.Players[0].Score != before-2 {
			t.Fatal("conquered village retained caravan bonus")
		}
	}
	for _, n := range []int{2, 6} {
		s := caravansAttackPlaying(t, n, true)
		g := s.Catan
		tiles := g.attackCityInventionTiles()
		if len(tiles) < 2 {
			t.Fatal("missing inland invention targets")
		}
		if err := s.catanAttackCityInvention(tiles[0], tiles[1]); err != nil {
			t.Fatal(err)
		}
		caravansAttackRestore(t, s)
	}
}
