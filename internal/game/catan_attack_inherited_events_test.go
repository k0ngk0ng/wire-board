package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func resolveAttackEvent(t *testing.T, s *State) {
	t.Helper()
	for step := 0; s.Catan.CardEvent != nil && step < len(s.Catan.Players); step++ {
		assertAttackRestored(t, s)
		actor := s.CatanPendingActor()
		action, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		attackReject(t, s, (actor+1)%len(s.Catan.Players), action)
		if step%2 == 0 {
			helperApply(t, s, actor, action)
		} else {
			s.AutoCatanPending()
		}
	}
	if s.Catan.CardEvent != nil || s.Phase != "catan_turn" || !s.Catan.RevealedEvent.ProductionStarted {
		t.Fatal("event stalled or missing production")
	}
	assertAttackRestored(t, s)
}

func TestCatanAttackEventInheritedResourceRewardsAndEmptyBank(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, kind := range []string{"plentiful_year", "calm_seas", "tournament"} {
			for _, stock := range []int{0, 1, 1000} {
				t.Run(fmt.Sprintf("%d/%s/%d", n, kind, stock), func(t *testing.T) {
					s := attackEventFixture(t, n)
					p := s.Turn
					// Map knights do not become face-up Knight cards for Tournament.
					attackBattleKnights(s, 0, p, p, p)
					if stock < 1000 {
						hand := slices.Clone(s.Catan.Bank)
						hand[0] -= stock
						attackHand(s, p, hand)
					}
					before := clone(s.Catan.Players)
					beginCardEvent(t, s, kind, 6, 0, 0)
					if stock > 0 {
						want := []int{}
						for i := range n {
							want = append(want, (p+i)%n)
						}
						if !slices.Equal(s.Catan.CardEvent.Players, want) {
							t.Fatal("zero ties did not reward everyone")
						}
					}
					resolveAttackEvent(t, s)
					for i := range n {
						gain := sum(s.Catan.Players[i].Resources) - sum(before[i].Resources)
						want := 0
						if stock == 1000 || stock == 1 && i == p {
							want = 1
						}
						if gain != want {
							t.Fatal("reward/exhaustion", i, gain, want)
						}
					}
				})
			}
		}
	}
}

func TestCatanAttackEventCalmSeasCountsConqueredBuildingsOnce(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackEventFixture(t, n)
		g := s.Catan
		winner := (s.Turn + 1) % n
		other := (s.Turn + 2) % n
		outer := g.Tiles[0].Vertices[4]
		if !g.harborVertex(outer) {
			t.Fatal("fixture outer port")
		}
		g.Vertices[outer].Owner, g.Vertices[outer].Level = winner, 1
		used := map[int]bool{outer: true}
		sites := []int{}
		for _, port := range g.Ports {
			e := g.Edges[port.Edge]
			if !used[e.A] && e.A != outer && e.B != outer {
				sites = append(sites, e.A)
				used[e.A] = true
				if len(sites) == 2 {
					break
				}
			}
		}
		if len(sites) != 2 {
			t.Fatal("port fixture")
		}
		g.Vertices[sites[0]].Owner, g.Vertices[sites[0]].Level = winner, 1
		g.Vertices[sites[1]].Owner, g.Vertices[sites[1]].Level = other, 2
		g.Attack.Barbarians[0] = 3
		if !g.Attack.conqueredBuilding(g, outer) {
			t.Fatal("fixture should include conquered port building")
		}
		s.catanScores()
		beginCardEvent(t, s, "calm_seas", 7, 0, 0)
		if !slices.Equal(s.Catan.CardEvent.Players, []int{winner}) {
			t.Fatal("counted port trading availability or city VP instead of buildings")
		}
		helperApply(t, s, winner, Action{Type: "catan_event_resource", Take: []int{1, 0, 0, 0, 0}})
		// The event's chosen resource precedes seven; it is the sole steal target.
		if s.Catan.Players[s.Turn].Resources[0] != 1 || s.Catan.Players[winner].Resources[0] != 0 || s.Catan.Robber != -1 {
			t.Fatal("event-before-seven continuation")
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackEventEarthquakeRepairAndKnightMovement(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := newAttackState(t, n, true)
		finishAttackSetup(t, s)
		p := s.Turn
		beginCardEvent(t, s, "earthquake", 6, 0, 0)
		resolveAttackEvent(t, s)
		g := s.Catan
		damaged := -1
		for seat := range n {
			count := 0
			for _, e := range g.Edges {
				if e.Owner == seat && e.Damaged {
					count++
					if seat == p {
						damaged = e.ID
					}
				}
			}
			if count != 1 {
				t.Fatal("earthquake did not damage exactly one road", seat, count)
			}
		}
		attackHand(s, p, []int{2, 2, 0, 0, 0})
		for _, e := range g.Edges {
			if g.canRoad(p, e.ID) {
				t.Fatal("new road offered with unrepaired damage")
			}
		}
		// Knights ignore roads, including their damage; traversable distances match.
		g.Attack.Knights = []catanAttackKnight{{Player: p, Edge: damaged}}
		moves := g.Attack.knightDestinations(g, 0, 3)
		undamaged := clone(*s)
		undamaged.Catan.Edges[damaged].Damaged = false
		if !reflect.DeepEqual(moves, undamaged.Catan.Attack.knightDestinations(undamaged.Catan, 0, 3)) {
			t.Fatal("damaged road blocked knight")
		}
		beforeRoads, _, _ := g.pieces(p)
		helperApply(t, s, p, Action{Type: "catan_repair_road", Edge: damaged})
		afterRoads, _, _ := s.Catan.pieces(p)
		if s.Catan.Edges[damaged].Damaged || beforeRoads != afterRoads || !slices.Equal(s.Catan.Players[p].Resources, []int{1, 1, 0, 0, 0}) || s.Catan.Attack.Sequence != 0 {
			t.Fatal("repair charged incorrectly, added piece, or triggered landing")
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackEventEpidemicRespectsConquestAndExpires(t *testing.T) {
	for _, n := range []int{3, 6} {
		for _, conquered := range []bool{false, true} {
			s := attackEventFixture(t, n)
			g := s.Catan
			p := s.Turn
			tile := g.Tiles[0]
			g.Vertices[tile.Vertices[4]].Owner, g.Vertices[tile.Vertices[4]].Level = p, 2
			if conquered {
				g.Attack.Barbarians[0] = 3
			}
			s.catanScores()
			beginCardEvent(t, s, "epidemic", tile.Number, 0, 0)
			want := 1
			if conquered {
				want = 0
			}
			if sum(s.Catan.Players[p].Resources) != want {
				t.Fatal("epidemic did not reduce city or produced conquered tile")
			}
			assertAttackRestored(t, s)
			// A following ordinary production has no lingering epidemic modifier.
			s.Phase = "catan_roll"
			beginCardEvent(t, s, "beautiful_day", tile.Number, 0, 0)
			if sum(s.Catan.Players[p].Resources) != want*3 {
				t.Fatal("epidemic leaked to next production")
			}
			assertAttackRestored(t, s)
		}
	}
}

func TestCatanAttackEventNeighborsKeepChoicesPrivateAndTransferTogether(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackEventFixture(t, n)
		turn := s.Turn
		for p := range n {
			hand := make([]int, 5)
			hand[p%5] = 1
			attackHand(s, p, hand)
		}
		before := clone(s.Catan.Players)
		beginCardEvent(t, s, "good_neighbors", 6, 0, 0)
		for step := range n {
			p := (turn + step) % n
			assertAttackRestored(t, s)
			for viewer := -1; viewer < n; viewer++ {
				q := s.View(viewer)["catan"].(map[string]any)["cardEvent"].(map[string]any)
				if q["gifts"] != nil {
					t.Fatal("private gift collection exposed")
				}
				if gift, ok := q["ownGift"].(CatanEventGift); ok && gift.From != viewer {
					t.Fatal("other choice exposed")
				}
			}
			helperApply(t, s, p, neighborGift(s.Catan, p%5))
			if step < n-1 && !reflect.DeepEqual(s.Catan.Players, before) {
				t.Fatal("gift transferred before everyone chose")
			}
		}
		for p := range n {
			want := make([]int, 5)
			want[(p+n-1)%n%5] = 1
			if !slices.Equal(s.Catan.Players[p].Resources, want) {
				t.Fatal("wrong left neighbor or repeated transfer", p)
			}
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackEventHelpfulNeighborUsesConquestAndPrisonerScores(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackEventFixture(t, n)
		g := s.Catan
		turn := s.Turn
		leader := (turn + 1) % n
		coLeader := (turn + 2) % n
		// The active player's conquered city is worth zero; captured barbarians
		// still give the two leaders two public VP apiece.
		tile := g.Tiles[0]
		g.Vertices[tile.Vertices[4]].Owner, g.Vertices[tile.Vertices[4]].Level = turn, 2
		g.Attack.Barbarians[0] = 3
		g.Attack.Prisoners[leader] = 4
		g.Attack.Prisoners[coLeader] = 4
		attackHand(s, leader, []int{0, 1, 0, 0, 0})
		attackHand(s, coLeader, []int{0, 0, 1, 0, 0})
		s.catanScores()
		beginCardEvent(t, s, "helpful_neighbor", 6, 0, 0)
		if !slices.Equal(s.Catan.CardEvent.Players, []int{leader, coLeader}) || !slices.Contains(s.Catan.CardEvent.Targets, turn) {
			t.Fatal("wrong helpful ranks")
		}
		for _, p := range []int{leader, coLeader} {
			assertAttackRestored(t, s)
			action, err := s.BotAction(p)
			if err != nil {
				t.Fatal(err)
			}
			action.Target = turn
			bad := action
			bad.Target = leader
			attackReject(t, s, p, bad)
			helperApply(t, s, p, action)
		}
		if !slices.Equal(s.Catan.Players[turn].Resources, []int{0, 1, 1, 0, 0}) || s.Catan.CardEvent != nil {
			t.Fatal("helpful transfer not exactly once")
		}
		assertAttackRestored(t, s)
	}
}

func TestCatanAttackEventTradeAdvantageRetainsDamagedLongestRoad(t *testing.T) {
	for _, n := range []int{3, 6} {
		s := attackEventFixture(t, n)
		g := s.Catan
		leader := (s.Turn + 1) % n
		victim := (s.Turn + 2) % n
		// Five sides of one hex form an actual five-edge path on the official map.
		for side := 0; side < 5; side++ {
			edge := catanFishingSide(g, 0, side)
			g.Edges[edge].Owner = leader
			g.Edges[edge].Damaged = true
		}
		s.catanScores()
		if g.LongestOwner != leader || g.Players[leader].RoadLength != 5 {
			t.Fatal("longest road fixture")
		}
		attackHand(s, victim, []int{0, 0, 0, 0, 2})
		beginCardEvent(t, s, "trade_advantage", 6, 0, 0)
		if !slices.Equal(s.Catan.CardEvent.Players, []int{leader}) {
			t.Fatal("wrong longest road responder")
		}
		resolveAttackEvent(t, s)
		if s.Catan.Players[leader].Resources[4] != 1 || s.Catan.Players[victim].Resources[4] != 1 || s.Catan.LongestOwner != leader {
			t.Fatal("trade advantage ignored damaged road")
		}
	}
}

func TestCatanAttackEventInheritedCorruptPendingRejected(t *testing.T) {
	for _, kind := range []string{"earthquake", "plentiful_year", "calm_seas", "tournament", "good_neighbors", "helpful_neighbor"} {
		s := attackEventFixture(t, 6)
		p := s.Turn
		for player := range 6 {
			attackHand(s, player, []int{1, 1, 0, 0, 0})
			s.Catan.Edges[player].Owner = player
		}
		s.Catan.Attack.Prisoners[p] = 4
		s.catanScores()
		beginCardEvent(t, s, kind, 6, 0, 0)
		if s.Catan.CardEvent == nil {
			t.Fatal("missing corruption fixture", kind)
		}
		for _, mutate := range []func(*CatanCardEvent){
			func(q *CatanCardEvent) { q.Players = append(q.Players, q.Players[0]) },
			func(q *CatanCardEvent) { q.Players[0] = 6 },
			func(q *CatanCardEvent) { q.Optional = true },
			func(q *CatanCardEvent) { q.Kind = "epidemic" },
		} {
			other := clone(*s)
			mutate(other.Catan.CardEvent)
			if other.validateCatanAttack() == nil {
				t.Fatal("corrupt pending accepted", kind)
			}
		}
		if kind == "good_neighbors" {
			for _, mutate := range []func(*CatanCardEvent){
				func(q *CatanCardEvent) { q.Gifts[0].To = q.Gifts[0].From },
				func(q *CatanCardEvent) { q.Gifts[0].Color = 4 },
				func(q *CatanCardEvent) { q.Gifts[1].Color = 0 },
				func(q *CatanCardEvent) { q.Gifts = q.Gifts[1:] },
			} {
				other := clone(*s)
				mutate(other.Catan.CardEvent)
				if other.validateCatanAttack() == nil {
					t.Fatal("corrupt private gift accepted")
				}
			}
		}
		resolveAttackEvent(t, s)
	}
}

// Exercise the complete ordinary match controller with supplied face effects.
// This is deliberately NOT the unverified physical deck or its draw odds.
func TestCatanAttackControlledEventMatches(t *testing.T) {
	kinds := []string{"earthquake", "plentiful_year", "good_neighbors", "calm_seas", "conflict", "epidemic", "helpful_neighbor", "tournament", "trade_advantage", "robber_attacks", "robber_flees", "beautiful_day"}
	for _, n := range []int{3, 6} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := newAttackState(t, n, true)
			revealed := 0
			for step := 0; !s.Finished && step < 5000; step++ {
				if s.Phase == "catan_roll" {
					kind := kinds[revealed%len(kinds)]
					total := 2 + (revealed*7)%11
					if kind == "robber_attacks" {
						total = 7
					}
					if err := s.catanBeginCardEvent(kind, total, 0, 0); err != nil {
						t.Fatalf("step=%d round=%d kind=%s error=%v", step, s.Round, kind, err)
					}
					revealed++
				} else {
					actor := s.CatanPendingActor()
					if actor < 0 {
						actor = s.Turn
					}
					if s.Phase == "catan_discard" {
						for p, due := range s.Catan.DiscardDue {
							if due > 0 {
								actor = p
								break
							}
						}
					}
					action, err := s.BotAction(actor)
					if err == nil {
						err = s.Apply(actor, action)
					}
					if err != nil {
						t.Fatalf("step=%d round=%d phase=%s action=%+v error=%v", step, s.Round, s.Phase, action, err)
					}
				}
				if err := s.validateCatanAttack(); err != nil {
					t.Fatal(step, err)
				}
				if step%31 == 0 {
					assertAttackRestored(t, s)
				}
			}
			if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 12 {
				t.Fatalf("no actual victory: round=%d revealed=%d phase=%s", s.Round, revealed, s.Phase)
			}
			t.Logf("controlled effects=%d, round=%d, winner=%d", revealed, s.Round, s.Winners[0])
		})
	}
}
