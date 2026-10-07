package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerFullBotKeepsCrewAfterAllFarmClaims(t *testing.T) {
	s := explorerFullRevealed(t)
	actor, ship := s.Turn, s.Turn*3
	g, x := s.Catan, s.Catan.Explorer
	for i, farm := range x.Board.publicView().Farms {
		explorerSpiceClaimFixture(t, s, actor, farm.Tile, i+2, catanExplorerCargoLocation{"supply", -1})
	}
	harbor := -1
	for _, v := range g.Vertices {
		if v.Owner == actor && v.Level == 2 {
			harbor = v.ID
			break
		}
	}
	x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Units[actor*11+8] = catanExplorerCargoLocation{"ship", ship}
	x.Cargo.Units[actor*11+9] = catanExplorerCargoLocation{"harbor", harbor}
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	if catanExplorerBotSpiceRemaining(g, actor) != 0 || !catanExplorerBotNeedsCrew(g, actor) {
		t.Fatal("unfinished lairs lost after six farm claims")
	}
	found := false
	for _, plan := range s.catanExplorerMissionPlans(actor) {
		if plan.action.Type == "catan_explorer_unit" && plan.action.Card == actor*11+10 {
			found = true
		}
	}
	if !found {
		t.Fatal("bot did not recruit third mobile crew for lairs")
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	g, x = s.Catan, s.Catan.Explorer
	// The setup ship is docked beside its own harbor. Mark this optional roll
	// done so the next cargo decision tests retaining, rather than retiring, crew.
	if err := x.Fish.apply(g, x.Board, x.Fleet, x.Cargo, actor, g.TurnSerial, "roll", 0, 0, -1, func(int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	before := clone(*s)
	a, ok := s.catanExplorerMissionCargo(actor)
	if !ok || a.Type != "catan_explorer_transfer" || !slices.Equal(a.Give, []int{actor*11 + 9}) || len(a.Take) > 0 {
		t.Fatal("bot retired needed crew", a)
	}
	if !reflect.DeepEqual(before, *s) {
		t.Fatal("cargo planning mutated state")
	}
	explorerSpiceApply(t, s, a)
	g, x = s.Catan, s.Catan.Explorer
	value := -100000
	for _, e := range g.Edges {
		if catanExplorerSeaEdge(g, e.ID) {
			value = max(value, catanExplorerMissionGoal(g, actor, ship, e.ID))
		}
	}
	if value < 1000 {
		t.Fatal("crew ship ignored visible lairs")
	}
}

func TestCatanExplorerFullBotFreightKeepsCouncilDestination(t *testing.T) {
	for _, kind := range []string{"spice", "fish"} {
		t.Run(kind, func(t *testing.T) {
			s := explorerFullRevealed(t)
			actor, ship := s.Turn, s.Turn*3
			g, x := s.Catan, s.Catan.Explorer
			x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
			loc := catanExplorerCargoLocation{"ship", ship}
			if kind == "spice" {
				explorerSpiceClaimFixture(t, s, actor, x.Board.publicView().Farms[0].Tile, 2, loc)
				x.Cargo.Units[actor*11+3] = loc // A spare crew must not divert the sack.
			} else {
				x.Cargo.Fish[0] = loc
			}
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
			g, x = s.Catan, s.Catan.Explorer
			before := clone(*s)
			for _, e := range g.Edges {
				if catanExplorerSeaEdge(g, e.ID) {
					value := catanExplorerMissionGoal(g, actor, ship, e.ID)
					anchor := slices.Contains(x.Board.Council.Anchors, e.A) || slices.Contains(x.Board.Council.Anchors, e.B)
					if (value > 0) != anchor {
						t.Fatal("loaded vessel diverted to crew mission", kind, e.ID, value)
					}
				}
			}
			if !reflect.DeepEqual(before, *s) {
				t.Fatal("destination planning mutated state")
			}
		})
	}
}

func TestCatanExplorerFullBotResolvesLairWithoutReadingToken(t *testing.T) {
	s := explorerFullRevealed(t)
	actor := s.Turn
	g, x := s.Catan, s.Catan.Explorer
	tile := x.Lairs.Sites[0].Tile
	x.Lairs.Sites[0].Ready, x.Lairs.Sites[0].Captor = g.TurnSerial, actor
	for i := 2; i < 5; i++ {
		x.Cargo.Units[actor*11+i] = catanExplorerCargoLocation{"lair", tile}
	}
	explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
	explorerSpiceApply(t, s, Action{Type: "catan_end"})
	before := clone(*s)
	a, err := s.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	if a.Type != "catan_explorer_resolve" || a.Target != tile {
		t.Fatal("full bot skipped ready lair", a)
	}
	other := clone(*s)
	l := other.Catan.Explorer.Lairs
	l.Sites[0].Number, l.Sites[1].Number = l.Sites[1].Number, l.Sites[0].Number
	if err := other.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.View(actor), other.View(actor)) {
		t.Fatal("hidden token changed visible state")
	}
	b, err := other.BotAction(actor)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || !reflect.DeepEqual(before, *s) {
		t.Fatal("lair planning used hidden token or changed state")
	}
	explorerSpiceApply(t, s, a)
	if s.Catan.Explorer.Lairs.Sites[0].Resolved == 0 {
		t.Fatal("lair resolution failed")
	}
}

func TestCatanExplorerFullNaturalBotMatches(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			// Normal full-map setup and actions; lair numbers are explicit artificial
			// acceptance components, not a verified production inventory.
			s, err := newCatanExplorerFullState(n, []int{2, 3, 4, 5, 6, 8})
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
				beforeSpice := slices.Clone(s.Catan.Explorer.Cargo.Spice)
				beforeFish := slices.Clone(s.Catan.Explorer.Cargo.Fish)
				if err = s.Apply(p, a); err != nil {
					t.Fatalf("step %d phase %s action %+v: %v", step, s.Phase, a, err)
				}
				assertExplorerSpiceMotion(t, beforeSpice, s, a)
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
			t.Log("round", s.Round, "scores", scores, "fish", s.Catan.Explorer.Fish.publicView(n).Progress, "spice", s.Catan.Explorer.Spice.publicView(s.Catan).Progress, "lairs", s.Catan.Explorer.Lairs.Progress, "actions", actions)
			if !s.Finished {
				t.Fatal("natural full mission did not finish")
			}
			if actions["catan_explorer_fish_roll"] == 0 || actions["catan_explorer_fish_load"] == 0 || actions["catan_explorer_fish_deliver"] == 0 || actions["catan_explorer_spice_land"] == 0 || actions["catan_explorer_spice_deliver"] == 0 {
				t.Fatal("fish and spice logistics must be exercised")
			}
			if len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 17 {
				t.Fatal("wrong natural result")
			}
			explorerStateRestore(t, s)
		})
	}
}

func TestCatanExplorerFullBotIgnoresPrivateInformation(t *testing.T) {
	s := explorerFullStarted(t, 3)
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
			assertExplorerFullBotPrivate(t, s, actor, first)
		}
		seen[first.Type] = true
		if err = s.Apply(actor, first); err != nil {
			t.Fatal(err)
		}
		if seen["catan_explorer_fish_roll"] && seen["catan_explorer_fish_load"] && seen["catan_explorer_fish_deliver"] && seen["catan_explorer_spice_land"] && seen["catan_explorer_spice_deliver"] {
			return
		}
	}
	// Required mission actions are covered independently by deterministic states.
	// A valid natural game may finish before choosing every optional mission.
	t.Log("natural privacy trajectory finished", s.Finished, "actions", seen)
}

func assertExplorerFullBotPrivate(t *testing.T, s *State, actor int, first Action) {
	t.Helper()
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
	slices.Reverse(x.Lairs.Deck)
	hidden := []int{}
	for i, site := range x.Lairs.Sites {
		if site.Resolved == 0 {
			hidden = append(hidden, i)
		}
	}
	for i, j := 0, len(hidden)-1; i < j; i, j = i+1, j-1 {
		a, b := hidden[i], hidden[j]
		x.Lairs.Sites[a].Number, x.Lairs.Sites[b].Number = x.Lairs.Sites[b].Number, x.Lairs.Sites[a].Number
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
		t.Fatal("full mission bot read secret information", first, second)
	}
}
