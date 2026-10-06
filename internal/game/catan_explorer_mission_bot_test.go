package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func TestCatanExplorerLairsNaturalBotMatch(t *testing.T) {
	for _, layout := range []string{"fixed", "variable"} {
		for _, n := range []int{2, 3, 4} {
			t.Run(fmt.Sprintf("%s/%d", layout, n), func(t *testing.T) {
				// Artificial token inventory remains explicit; all play starts from the
				// normal private constructor with no injected resources, pieces or score.
				s, err := newCatanExplorerLairsState(n, layout, []int{2, 3, 4, 5, 6, 8})
				if err != nil {
					t.Fatal(err)
				}
				actions := map[string]int{}
				for step := 0; step < 6000 && !s.Finished; step++ {
					player := s.Turn
					if s.Phase == "catan_discard" {
						for p, due := range s.Catan.DiscardDue {
							if due > 0 {
								player = p
								break
							}
						}
					}
					a, err := s.BotAction(player)
					if err != nil {
						t.Fatal(step, s.Phase, err)
					}
					if err = s.Apply(player, a); err != nil {
						t.Fatalf("step %d phase %s action %+v: %v", step, s.Phase, a, err)
					}
					actions[a.Type]++
					if step%31 == 0 {
						s = explorerStateRestore(t, s)
					}
				}
				scores := []int{}
				for _, p := range s.Catan.Players {
					scores = append(scores, p.Score)
				}
				t.Log("actions", actions, "scores", scores, "progress", s.Catan.Explorer.Lairs.Progress, "round", s.Round)
				if !s.Finished {
					t.Fatal("natural mission game did not finish")
				}
				if actions["catan_explorer_land"] == 0 || actions["catan_explorer_resolve"] == 0 || actions["catan_explorer_unit"] == 0 || actions["catan_explorer_ship"] == 0 {
					t.Fatal("mission logistics not exercised")
				}
				explorerStateRestore(t, s)
			})
		}
	}

}

func TestCatanExplorerLairsBotDoesNotReadHiddenMissionComponents(t *testing.T) {
	s := explorerLairsStarted(t, 3)
	paid, sailed, mission := false, false, false
	for step := 0; step < 1800 && !s.Finished; step++ {
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
		next := clone(*s)
		x := next.Catan.Explorer
		for r := range x.Board.Numbers {
			slices.Reverse(x.Board.Numbers[r])
		}
		for region := 0; region < 2; region++ {
			indices := []int{}
			for i, h := range x.Board.Hidden {
				if !h.Revealed && h.Region == region {
					indices = append(indices, i)
				}
			}
			for i, j := 0, len(indices)-1; i < j; i, j = i+1, j-1 {
				a, b := indices[i], indices[j]
				x.Board.Hidden[a].Resource, x.Board.Hidden[b].Resource = x.Board.Hidden[b].Resource, x.Board.Hidden[a].Resource
			}
		}
		slices.Reverse(x.Lairs.Deck)
		unresolved := []int{}
		for i, site := range x.Lairs.Sites {
			if site.Resolved == 0 {
				unresolved = append(unresolved, i)
			}
		}
		for i, j := 0, len(unresolved)-1; i < j; i, j = i+1, j-1 {
			a, b := unresolved[i], unresolved[j]
			x.Lairs.Sites[a].Number, x.Lairs.Sites[b].Number = x.Lairs.Sites[b].Number, x.Lairs.Sites[a].Number
		}
		// Swap two opponents' differently colored cards, keeping both public hand
		// counts, the resource bank, and the actor's actual hand unchanged.
		p, q := (actor+1)%3, (actor+2)%3
		swapped := false
		for a, na := range next.Catan.Players[p].Resources {
			if na == 0 {
				continue
			}
			for b, nb := range next.Catan.Players[q].Resources {
				if nb > 0 && a != b {
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
		if err = next.validateCatanExplorer(); err != nil {
			t.Fatal("privacy permutation invalid", err)
		}
		if !reflect.DeepEqual(s.View(actor), next.View(actor)) {
			t.Fatal("privacy test changed something visible")
		}
		second, err := next.BotAction(actor)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatal("bot used hidden information", first, second)
		}
		paid = paid || first.Type == "catan_explorer_unit"
		sailed = sailed || first.Type == "catan_explorer_sail"
		mission = mission || first.Type == "catan_explorer_land"
		if err = s.Apply(actor, first); err != nil {
			t.Fatal(err)
		}
		if paid && sailed && mission {
			return
		}
	}
	t.Fatal("privacy run did not exercise recruitment, navigation and landing")
}

func TestCatanExplorerLairsBotCanSwapReturningCrewForHarborSettler(t *testing.T) {
	s := explorerLairsStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	x := g.Explorer
	p := s.Turn
	ship := p * 3
	edge := g.Edges[x.Fleet.Positions[ship]]
	harbor := -1
	for _, v := range []int{edge.A, edge.B} {
		if g.Vertices[v].Owner == p && g.Vertices[v].Level == 2 {
			harbor = v
		}
	}
	if harbor < 0 {
		t.Fatal("initial ship must be docked")
	}
	x.Cargo.Units[p*11] = catanExplorerCargoLocation{"harbor", harbor}
	x.Cargo.Units[p*11+2] = catanExplorerCargoLocation{"ship", ship}
	x.Cargo.Units[p*11+3] = catanExplorerCargoLocation{"ship", ship}
	if err := s.Apply(p, Action{Type: "catan_explorer_begin_move", Prompt: 1}); err != nil {
		t.Fatal(err)
	}
	load, unload, ok := catanExplorerBotRetireCargo(s.Catan, ship, harbor)
	if !ok {
		t.Fatal("bot cannot swap a full returning ship for a settler")
	}
	if err := s.Apply(p, Action{Type: "catan_explorer_transfer", Prompt: 1, Slot: ship, Vertex: harbor, Give: load, Take: unload}); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	x = s.Catan.Explorer
	if !slices.Equal(x.Cargo.contents(catanExplorerCargoLocation{"ship", ship}), []int{p * 11}) || !slices.Equal(x.Cargo.contents(catanExplorerCargoLocation{"harbor", harbor}), []int{p*11 + 2, p*11 + 3}) {
		t.Fatal("cargo not exchanged")
	}
	if _, _, ok = catanExplorerBotRetireCargo(s.Catan, ship, harbor); ok {
		t.Fatal("bot would reverse the productive transfer")
	}
}
