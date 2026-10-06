package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerFishNaturalBotMatches(t *testing.T) {
	for n := 2; n <= 4; n++ {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			// Explicit artificial lair numbers; normal setup and all subsequent play
			// use production rules, with no injected resources, pieces or scores.
			s, err := newCatanExplorerFishState(n, []int{2, 3, 4, 5, 6, 8})
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
				if err = s.Apply(p, a); err != nil {
					t.Fatalf("step %d phase %s action %+v: %v", step, s.Phase, a, err)
				}
				actions[a.Type]++
				if step%37 == 0 {
					s = explorerStateRestore(t, s)
				}
			}
			scores := []int{}
			for _, p := range s.Catan.Players {
				scores = append(scores, p.Score)
			}
			t.Log("round", s.Round, "scores", scores, "fish", s.Catan.Explorer.Fish.publicView(n).Progress, "lairs", s.Catan.Explorer.Lairs.Progress, "actions", actions)
			if !s.Finished {
				t.Fatal("natural fish mission did not finish")
			}
			if actions["catan_explorer_fish_roll"] == 0 || actions["catan_explorer_fish_load"] == 0 || actions["catan_explorer_fish_deliver"] == 0 || actions["catan_explorer_resolve"] == 0 {
				t.Fatal("both mission logistics must be exercised")
			}
			if len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < 15 {
				t.Fatal("wrong natural result")
			}
			explorerStateRestore(t, s)
		})
	}
}

func TestCatanExplorerFishBotFiveLairsAndFishingHold(t *testing.T) {
	s := explorerFishStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFishRevealExcept(t, s, -1)
	g, x := s.Catan, s.Catan.Explorer
	// The scene contains five lairs. This helper's purpose is to select mission
	// goals; resolved metadata is supplied explicitly, not a valid game save.
	for i := range x.Lairs.Sites {
		x.Lairs.Sites[i].Resolved = 1
	}
	if catanExplorerMissionUnfinished(g) {
		t.Fatal("waiting for a nonexistent sixth lair")
	}
	x.Lairs.Sites[0].Resolved = 0
	if !catanExplorerMissionUnfinished(g) {
		t.Fatal("unfinished visible lair ignored")
	}
	// Reserve the third ship for fishing without inspecting hidden components.
	for ship := s.Turn * 3; ship < (s.Turn+1)*3; ship++ {
		if catanExplorerBotFishingShip(g, s.Turn, ship) != (ship == s.Turn*3+2) {
			t.Fatal("wrong fishing ship")
		}
	}
}

func TestCatanExplorerFishBotLoadedShipHeadsToCouncil(t *testing.T) {
	s := explorerFishStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	explorerFishRevealExcept(t, s, -1)
	g, x := s.Catan, s.Catan.Explorer
	ship := s.Turn * 3
	shoal := x.Board.publicView().Shoals[0]
	for _, e := range g.Edges {
		if catanExplorerSeaEdge(g, e.ID) && catanExplorerTouches(g, e.ID, shoal.Tile) && !slices.Contains(x.Fleet.Positions, e.ID) {
			x.Fleet.Positions[ship] = e.ID
			break
		}
	}
	x.Cargo.Units[s.Turn*11] = catanExplorerCargoLocation{"supply", -1}
	x.Cargo.Fish[0] = catanExplorerCargoLocation{"ship", ship}
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	g, x = s.Catan, s.Catan.Explorer
	before := clone(*x)
	path := catanExplorerMissionVoyage(g, s.Turn, ship)
	if len(path) == 0 {
		t.Fatal("loaded fish vessel has no delivery route")
	}
	if _, err := x.Fleet.quote(g, s.Turn, g.TurnSerial, ship, path, -1, -1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, *x) {
		t.Fatal("planning mutated authoritative pieces")
	}
	for _, e := range g.Edges {
		if !catanExplorerSeaEdge(g, e.ID) {
			continue
		}
		value := catanExplorerMissionGoal(g, s.Turn, ship, e.ID)
		anchor := slices.Contains(x.Board.Council.Anchors, e.A) || slices.Contains(x.Board.Council.Anchors, e.B)
		if (value > 0) != anchor {
			t.Fatal("fish vessel seeks non-council goal", e.ID, value)
		}
	}
}

func TestCatanExplorerFishBotIgnoresHiddenFishAndOpponentHands(t *testing.T) {
	s := explorerFishStarted(t, 3)
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
				t.Fatal("fish bot read secret information", first, second)
			}
		}
		seen[first.Type] = true
		if err = s.Apply(actor, first); err != nil {
			t.Fatal(err)
		}
		if seen["catan_explorer_fish_roll"] && seen["catan_explorer_fish_load"] && seen["catan_explorer_fish_deliver"] && seen["catan_explorer_land"] {
			return
		}
	}
	t.Fatal("privacy run did not exercise fishing, delivery and crews", seen)
}

func TestCatanExplorerFishBotKeepsFishingHoldFreeAndCollectsHarborCargo(t *testing.T) {
	s := explorerFishStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	g, x := s.Catan, s.Catan.Explorer
	p := s.Turn
	fishing := p*3 + 2
	x.Fleet.Positions[fishing] = x.Fleet.Positions[p*3]
	harbor := -1
	for _, v := range []int{g.Edges[x.Fleet.Positions[fishing]].A, g.Edges[x.Fleet.Positions[fishing]].B} {
		if g.Vertices[v].Owner == p && g.Vertices[v].Level == 2 {
			harbor = v
			break
		}
	}
	if harbor < 0 {
		t.Fatal("initial vessel not at harbor")
	}
	x.Cargo.Units[p*11+2], x.Cargo.Units[p*11+3] = catanExplorerCargoLocation{"harbor", harbor}, catanExplorerCargoLocation{"harbor", harbor}
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
	for _, plan := range s.catanExplorerMissionPlans(p) {
		if plan.action.Type == "catan_explorer_unit" && plan.action.Choice == "ship" && plan.action.Target == fishing {
			t.Fatal("recruits filled reserved fishing hold")
		}
	}
	explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
	if a, ok := s.catanExplorerMissionCargo(p); ok && a.Type == "catan_explorer_transfer" && a.Slot == fishing && len(a.Give) > 0 {
		t.Fatal("auto-loading crew filled fishing hold")
	}
	g, x = s.Catan, s.Catan.Explorer
	x.Cargo.Units[p*11+2], x.Cargo.Units[p*11+3] = catanExplorerCargoLocation{"supply", -1}, catanExplorerCargoLocation{"supply", -1}
	// An explicit human-to-autoplay handoff fixture: fish was left in the port.
	x.Cargo.Fish[0] = catanExplorerCargoLocation{"harbor", harbor}
	a, err := s.BotAction(p)
	if err != nil {
		t.Fatal(err)
	}
	if a.Type != "catan_explorer_transfer" || a.Slot != fishing || !slices.Equal(a.Cards, []int{0}) {
		t.Fatal("bot ignored stored harbor fish", a)
	}
	if err = s.Apply(p, a); err != nil {
		t.Fatal(err)
	}
	if s.Catan.Explorer.Cargo.Fish[0] != (catanExplorerCargoLocation{"ship", fishing}) {
		t.Fatal("fish did not reach fishing vessel")
	}
	explorerStateRestore(t, s)
}
