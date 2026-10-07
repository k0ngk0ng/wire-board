package game

import (
	"reflect"
	"testing"
)

// Guaranteed mission-action coverage, independent of whether a natural match
// chooses to finish through buildings, fish, spices or lair rewards first.
// These are valid controlled midgames, not claims of naturally reached states.
func TestCatanExplorerFullBotMissionPrivacyCoverage(t *testing.T) {
	for _, kind := range []string{"fish_roll", "fish_load", "fish_deliver", "spice_land", "spice_deliver"} {
		t.Run(kind, func(t *testing.T) {
			s := explorerFullRevealed(t)
			actor := s.Turn
			ship := actor * 3
			g, x := s.Catan, s.Catan.Explorer
			loc := catanExplorerCargoLocation{"ship", ship}
			// Make the opponent-hand permutation non-vacuous and conserve each card.
			p, q := (actor+1)%3, (actor+2)%3
			g.Bank[0]--
			g.Players[p].Resources[0]++
			g.Bank[1]--
			g.Players[q].Resources[1]++
			if kind != "fish_roll" {
				x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
			}
			var cargoID int
			switch kind {
			case "fish_load":
				shoal := x.Board.publicView().Shoals[0].Tile
				x.Cargo.Fish[0] = catanExplorerCargoLocation{"shoal", shoal}
				explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return catanExplorerTouches(g, e.ID, shoal) })
			case "fish_deliver":
				x.Cargo.Fish[0] = loc
				anchor := x.Board.Council.Anchors[0]
				explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == anchor || e.B == anchor })
			case "spice_land":
				farm := x.Board.publicView().Farms[0].Tile
				x.Cargo.Units[actor*11+2] = loc
				explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return catanExplorerTouches(g, e.ID, farm) })
			case "spice_deliver":
				// Use a non-swift farm so entering movement has the ordinary speed budget.
				for _, farm := range x.Board.publicView().Farms {
					if farm.Ability != "swift" {
						cargoID = explorerSpiceClaimFixture(t, s, actor, farm.Tile, 2, loc)
						break
					}
				}
				anchor := x.Board.Council.Anchors[0]
				explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == anchor || e.B == anchor })
			}
			if err := s.validateCatanExplorer(); err != nil {
				t.Fatal("invalid mission coverage fixture", err)
			}
			explorerSpiceApply(t, s, Action{Type: "catan_explorer_begin_move"})
			first, err := s.BotAction(actor)
			if err != nil {
				t.Fatal(err)
			}
			if first.Type != "catan_explorer_"+kind {
				t.Fatal("required mission action missing", kind, first)
			}
			before := clone(*s)
			assertExplorerFullBotPrivate(t, s, actor, first)
			if !reflect.DeepEqual(*s, before) {
				t.Fatal("privacy check mutated original state")
			}
			explorerSpiceApply(t, s, first)
			x = s.Catan.Explorer
			switch kind {
			case "fish_load":
				if x.Cargo.Fish[first.Card] != loc {
					t.Fatal("fish was not loaded")
				}
			case "fish_deliver":
				if len(x.Fish.Deliveries) != 1 || x.Cargo.Fish[0].Kind != "supply" {
					t.Fatal("fish was not delivered")
				}
			case "spice_land":
				if !x.Cargo.farmFriend(actor, first.Target) || len(x.Cargo.spiceContents(loc)) != 1 {
					t.Fatal("farm claim did not create cargo")
				}
			case "spice_deliver":
				if len(x.Spice.Deliveries) != 1 || x.Cargo.Spice[cargoID].At.Kind != "supply" {
					t.Fatal("spice was not delivered")
				}
			}
			if s.Finished {
				t.Fatal("fixture unexpectedly finished")
			}
			explorerStateRestore(t, s)
		})
	}
}
