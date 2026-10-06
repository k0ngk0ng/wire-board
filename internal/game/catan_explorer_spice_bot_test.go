package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerSpiceNaturalBotMatches(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			// Official spice map and normal setup; no injected resources, pieces or scores.
			s, err := newCatanExplorerSpiceState(n)
			if err != nil {
				t.Fatal(err)
			}
			actions := map[string]int{}
			for step := 0; step < 9000 && !s.Finished; step++ {
				p := s.Turn
				if s.Phase == "catan_discard" {
					for i, due := range s.Catan.DiscardDue {
						if due > 0 {
							p = i
							break
						}
					}
				}
				a, err := s.BotAction(p)
				if err != nil {
					t.Fatal(step, s.Phase, err)
				}
				beforeFish := slices.Clone(s.Catan.Explorer.Cargo.Fish)
				if err = s.Apply(p, a); err != nil {
					t.Fatalf("step %d phase %s action %+v: %v", step, s.Phase, a, err)
				}
				assertExplorerFishMotion(t, beforeFish, s, a)
				actions[a.Type]++
				if step%37 == 0 {
					s = explorerStateRestore(t, s)
				}
			}
			scores := []int{}
			for _, p := range s.Catan.Players {
				scores = append(scores, p.Score)
			}
			t.Log("round", s.Round, "scores", scores, "fish", s.Catan.Explorer.Fish.publicView(n).Progress, "spice", s.Catan.Explorer.Spice.publicView(s.Catan).Progress, "actions", actions)
			if !s.Finished {
				t.Fatal("natural spice mission did not finish")
			}
			if actions["catan_explorer_fish_roll"] == 0 || actions["catan_explorer_fish_load"] == 0 || actions["catan_explorer_fish_deliver"] == 0 || actions["catan_explorer_spice_land"] == 0 || actions["catan_explorer_spice_deliver"] == 0 {
				t.Fatal("both mission logistics must be exercised")
			}
			if len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 15 {
				t.Fatal("wrong natural result")
			}
			explorerStateRestore(t, s)
		})
	}
}

func TestCatanExplorerSpiceBotIgnoresHiddenFarmsAndOpponentHands(t *testing.T) {
	s := explorerSpiceStarted(t, 3)
	seen := map[string]bool{}
	for step := 0; step < 2400 && !s.Finished; step++ {
		actor := s.Turn
		if s.Phase == "catan_discard" {
			for p, due := range s.Catan.DiscardDue {
				if due > 0 {
					actor = p
					break
				}
			}
		}
		first, err := s.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if step%13 == 0 || !seen[first.Type] {
			next := clone(*s)
			x := next.Catan.Explorer
			for region := 0; region < 2; region++ {
				slices.Reverse(x.Board.Numbers[region])
				ids := []int{}
				for i, h := range x.Board.Hidden {
					if !h.Revealed && h.Region == region {
						ids = append(ids, i)
					}
				}
				for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
					a, b := ids[i], ids[j]
					x.Board.Hidden[a].Resource, x.Board.Hidden[b].Resource = x.Board.Hidden[b].Resource, x.Board.Hidden[a].Resource
					x.Board.Hidden[a].Fish, x.Board.Hidden[b].Fish = x.Board.Hidden[b].Fish, x.Board.Hidden[a].Fish
					x.Board.Hidden[a].Farm, x.Board.Hidden[b].Farm = x.Board.Hidden[b].Farm, x.Board.Hidden[a].Farm
					x.Board.Hidden[a].PirateDie, x.Board.Hidden[b].PirateDie = x.Board.Hidden[b].PirateDie, x.Board.Hidden[a].PirateDie
				}
			}
			p, q := (actor+1)%3, (actor+2)%3
			swapped := false
			for a, na := range next.Catan.Players[p].Resources {
				for b, nb := range next.Catan.Players[q].Resources {
					if na > 0 && nb > 0 && a != b {
						next.Catan.Players[p].Resources[a]--
						next.Catan.Players[p].Resources[b]++
						next.Catan.Players[q].Resources[b]--
						next.Catan.Players[q].Resources[a]++
						swapped = true
						break
					}
				}
				if swapped {
					break
				}
			}
			if err := next.validateCatanExplorer(); err != nil {
				t.Fatal("invalid private permutation", err)
			}
			if !reflect.DeepEqual(s.View(actor), next.View(actor)) {
				t.Fatal("privacy fixture changed actor's visible state")
			}
			second, err := next.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatal("fish bot read secret information", first, second)
			}
		}
		seen[first.Type] = true
		if err = s.Apply(actor, first); err != nil {
			t.Fatal(err)
		}
		if seen["catan_explorer_fish_roll"] && seen["catan_explorer_fish_load"] && seen["catan_explorer_fish_deliver"] && seen["catan_explorer_spice_land"] && seen["catan_explorer_spice_deliver"] && seen["catan_explorer_spice_gold"] {
			return
		}
	}
	t.Fatal("privacy run did not exercise fishing, delivery and crews", seen)
}

func TestCatanExplorerSpiceBotRecoversStoredSacks(t *testing.T) {
	s := explorerSpiceRevealed(t)
	g, x := s.Catan, s.Catan.Explorer
	actor := s.Turn
	ship := actor * 3
	harbor := -1
	for _, v := range g.Vertices {
		if v.Owner == actor && v.Level == 2 {
			harbor = v.ID
			break
		}
	}
	// Human left two sacks at port; the arriving ship still carries a settler.
	sacks := []int{}
	for i, farm := range x.Board.publicView().Farms[:2] {
		sacks = append(sacks, explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+2, catanExplorerCargoLocation{"harbor", harbor}))
	}
	explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == harbor || e.B == harbor })
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	// Avoid a fish roll preceding this cargo decision; it is otherwise legal.
	g, x = s.Catan, s.Catan.Explorer
	if err := x.Fish.apply(g, x.Board, x.Fleet, x.Cargo, actor, g.TurnSerial, "roll", 0, 0, -1, func(int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	before := clone(*s)
	a, err := s.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, *s) {
		t.Fatal("bot planning mutated state")
	}
	if a.Type != "catan_explorer_transfer" || !slices.Equal(a.SpiceLoad, sacks) || !slices.Equal(a.Take, []int{actor * 11}) {
		t.Fatal("bot failed simultaneous settler/spice swap", a)
	}
	explorerSpiceApply(t, s, a)
	// The loaded vessel should head for the council, never drift back to farms.
	g, x = s.Catan, s.Catan.Explorer
	for _, e := range g.Edges {
		if catanExplorerSeaEdge(g, e.ID) {
			value := catanExplorerMissionGoal(g, actor, ship, e.ID)
			anchor := slices.Contains(x.Board.Council.Anchors, e.A) || slices.Contains(x.Board.Council.Anchors, e.B)
			if (value > 0) != anchor {
				t.Fatal("loaded spice vessel has wrong goal", e.ID, value)
			}
		}
	}
	if len(catanExplorerMissionVoyage(g, actor, ship)) == 0 {
		t.Fatal("no route for stored spice delivery")
	}
}

func TestCatanExplorerSpiceBotPermanentCrewAndGoldReserve(t *testing.T) {
	s := explorerSpiceRevealed(t)
	g, x := s.Catan, s.Catan.Explorer
	actor := s.Turn
	// Permanent crews must not consume the mobile recruit quota.
	for i, farm := range x.Board.publicView().Farms[:4] {
		explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+2, catanExplorerCargoLocation{"supply", -1})
	}
	wanted := false
	for _, plan := range s.catanExplorerMissionPlans(actor) {
		if plan.action.Type == "catan_explorer_unit" && plan.action.Card%11 >= 2 {
			wanted = true
		}
	}
	if !wanted {
		t.Fatal("permanent crews prevented remaining missions")
	}
	for i, farm := range x.Board.publicView().Farms[4:] {
		explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+6, catanExplorerCargoLocation{"supply", -1})
	}
	for _, plan := range s.catanExplorerMissionPlans(actor) {
		if plan.action.Type == "catan_explorer_unit" && plan.action.Card%11 >= 2 {
			t.Fatal("recruited after all farms claimed")
		}
	}
	reserve := slices.Clone(g.Players[actor].Resources)
	if _, ok := s.catanExplorerSpiceBotGold(actor, reserve); ok {
		t.Fatal("sold reserved build resource")
	}
	g.Bank[0]--
	g.Players[actor].Resources[0]++
	a, ok := s.catanExplorerSpiceBotGold(actor, reserve)
	if !ok || a.Type != "catan_explorer_spice_gold" || a.Card != 0 {
		t.Fatal("missed surplus exchange", a)
	}
	explorerSpiceApply(t, s, a)
}
