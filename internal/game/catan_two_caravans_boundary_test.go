package game

import (
	"fmt"
	"math/rand"
	"slices"
	"testing"
)

func singleCaravanOrigin(mask int) bool { return mask > 0 && mask&(mask-1) == 0 }

// The map topology is fixed; a seeded walk builds legal chronological networks.
// Only completed even prefixes are used, compatible with pre-supplement saves.
func twoCaravanNetwork(t *testing.T, accept func(*Catan, *catanCaravans) bool) *State {
	t.Helper()
	s := twoCaravanFixture(t)
	g, c := s.Catan, s.Catan.Caravans
	rng := rand.New(rand.NewSource(3357))
	for attempt := 0; attempt < 4000; attempt++ {
		c.Wagons = nil
		for len(c.Wagons) <= 20 {
			if len(c.Wagons)%2 == 0 && accept(g, c) {
				t.Logf("boundary network: %+v", c.Wagons)
				twoCaravanCheck(t, s)
				return s
			}
			choices := c.choices(g)
			if len(choices) == 0 {
				break
			}
			c.Wagons = append(c.Wagons, choices[rng.Intn(len(choices))])
		}
	}
	t.Fatal("seeded legal network did not reach required boundary")
	return nil
}

func TestCatanTwoCaravansMergedTrains(t *testing.T) {
	for _, newlyMerged := range []bool{false, true} {
		t.Run(fmt.Sprint("new=", newlyMerged), func(t *testing.T) {
			var first catanCaravanWagon
			s := twoCaravanNetwork(t, func(g *Catan, c *catanCaravans) bool {
				choices, count := c.firstChoices(g, true)
				if count != 2 {
					return false
				}
				for _, w := range choices {
					after := *c
					after.Wagons = append(slices.Clone(c.Wagons), w)
					mask := after.trainOrigins(g, w.From)
					if !singleCaravanOrigin(mask) && (mask != c.trainOrigins(g, w.From)) == newlyMerged {
						first = w
						return true
					}
				}
				return false
			})
			twoCaravanBid(t, s, []int{2, 1})
			helperApply(t, s, 0, Action{Type: "catan_caravan_place", Edge: first.Edge, Vertex: first.From})
			twoCaravanCheck(t, s)
			c, g := s.Catan.Caravans, s.Catan
			train := c.Pending.Two.Train
			if singleCaravanOrigin(train) || train != c.trainOrigins(g, first.From) || g.Bank[2] != 16 {
				t.Fatal("merged identity or escrow was not retained")
			}
			for _, w := range c.choices(g) {
				if c.trainOrigins(g, w.From) == train {
					helperReject(t, s, 0, Action{Type: "catan_caravan_place", Edge: w.Edge, Vertex: w.From})
				}
			}
			a, err := s.BotAction(0)
			if err != nil || c.trainOrigins(g, a.Vertex) == train {
				t.Fatal("second wagon did not choose another train", err)
			}
			helperApply(t, s, 0, a)
			twoCaravanCheck(t, s)
			if s.Catan.Caravans.Pending != nil || s.Catan.Bank[2] != 19 || s.Turn != 1 {
				t.Fatal("merged round did not finish")
			}
		})
	}
}

func TestCatanTwoCaravansMustMaximizePlacements(t *testing.T) {
	var blocked catanCaravanWagon
	s := twoCaravanNetwork(t, func(g *Catan, c *catanCaravans) bool {
		good, count := c.firstChoices(g, true)
		if count != 2 {
			return false
		}
		for _, w := range c.choices(g) {
			if !slices.Contains(good, w) {
				blocked = w
				return true
			}
		}
		return false
	})
	twoCaravanBid(t, s, []int{2, 1})
	helperReject(t, s, 0, Action{Type: "catan_caravan_place", Edge: blocked.Edge, Vertex: blocked.From})
	view := s.View(0)["catan"].(map[string]any)["caravans"].(map[string]any)
	if view["placementLimit"] != 2 {
		t.Fatal("missing two-wagon public limit")
	}
	for i := 0; i < 2; i++ {
		a, err := s.BotAction(0)
		if err != nil {
			t.Fatal(err)
		}
		helperApply(t, s, 0, a)
		twoCaravanCheck(t, s)
	}
	if len(s.Catan.Caravans.ShortRounds) != 0 || s.Catan.Caravans.Pending != nil {
		t.Fatal("avoidable short round")
	}
	// A forged receipt for a normal pair cannot excuse losing a second wagon.
	s.Catan.Caravans.ShortRounds = []catanTwoCaravanShortRound{{Start: 0, Different: true}}
	if s.validateCaravans() == nil {
		t.Fatal("forged short receipt accepted")
	}
}

func TestCatanTwoCaravansForcedShortRound(t *testing.T) {
	for _, different := range []bool{false, true} {
		t.Run(fmt.Sprint("different=", different), func(t *testing.T) {
			s := twoCaravanNetwork(t, func(g *Catan, c *catanCaravans) bool {
				_, count := c.firstChoices(g, different)
				return count == 1
			})
			bids := []int{2, 2}
			if different {
				bids[1] = 1
			}
			start := len(s.Catan.Caravans.Wagons)
			twoCaravanBid(t, s, bids)
			view := s.View(0)["catan"].(map[string]any)["caravans"].(map[string]any)
			if view["placementLimit"] != 1 {
				t.Fatal("missing short public limit")
			}
			a, err := s.BotAction(0)
			if err != nil {
				t.Fatal(err)
			}
			helperApply(t, s, 0, a)
			twoCaravanCheck(t, s)
			c := s.Catan.Caravans
			if len(c.Wagons) != start+1 || len(c.ShortRounds) != 1 || c.ShortRounds[0].Different != different || c.Pending != nil || s.Catan.Bank[2] != 19 || s.Turn != 1 || s.Phase != "catan_roll" {
				t.Fatal("short round did not return bids and advance")
			}
			for _, corrupt := range []func(*catanCaravans){
				func(c *catanCaravans) { c.ShortRounds = nil },
				func(c *catanCaravans) { c.ShortRounds[0].Start++ },
				func(c *catanCaravans) { c.ShortRounds = append(c.ShortRounds, c.ShortRounds[0]) },
			} {
				bad := clone(*s)
				corrupt(bad.Catan.Caravans)
				helperReject(t, &bad, bad.Turn, Action{Type: "catan_roll"})
			}
			// More rounds may start on an odd wagon count. Run tie rounds to the
			// actual end of this network/supply, restoring after every placement.
			for len(s.Catan.Caravans.choices(s.Catan)) > 0 {
				s.Phase = "catan_turn"
				s.Catan.Two.Rolls = []int{2, 12}
				twoCaravanBid(t, s, []int{1, 1})
				for s.Catan.Caravans.Pending != nil {
					actor := s.CatanPendingActor()
					a, err := s.BotAction(actor)
					if err != nil {
						t.Fatal(err)
					}
					helperApply(t, s, actor, a)
					twoCaravanCheck(t, s)
				}
			}
			t.Logf("completed with %d wagons, receipts %+v", len(s.Catan.Caravans.Wagons), s.Catan.Caravans.ShortRounds)
			if different && (len(s.Catan.Caravans.Wagons) != 22 || len(s.Catan.Caravans.ShortRounds) != 2) {
				t.Fatal("final physical wagon was not placed in a new short round")
			}
		})
	}
}
