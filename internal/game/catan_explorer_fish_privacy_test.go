package game

import (
	"fmt"
	"testing"
)

// Controlled valid midgames guarantee mission coverage independently of the
// winning strategy in a natural match. Lair faces remain synthetic fixtures.
func TestCatanExplorerFishBotMissionPrivacyCoverage(t *testing.T) {
	for n := 2; n <= 4; n++ {
		for _, kind := range []string{"fish_roll", "fish_load", "fish_deliver", "land", "resolve"} {
			t.Run(fmt.Sprintf("%d/%s", n, kind), func(t *testing.T) {
				s := explorerFishStarted(t, n)
				if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
					t.Fatal(err)
				}
				explorerFishRevealExcept(t, s, -1)
				actor := s.Turn
				ship := actor * 3
				g, x := s.Catan, s.Catan.Explorer
				loc := catanExplorerCargoLocation{"ship", ship}
				if n >= 3 {
					// Conserve cards and make opponent-hand permutations non-vacuous.
					for resource := 0; resource < 2; resource++ {
						g.Bank[resource]--
						g.Players[(actor+resource+1)%n].Resources[resource]++
					}
				}
				if kind != "fish_roll" {
					x.Cargo.Units[actor*11] = catanExplorerCargoLocation{"supply", -1}
				}
				tile := x.Lairs.Sites[0].Tile
				switch kind {
				case "fish_load":
					shoal := x.Board.publicView().Shoals[0].Tile
					x.Cargo.Fish[0] = catanExplorerCargoLocation{"shoal", shoal}
					explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return catanExplorerTouches(g, e.ID, shoal) })
				case "fish_deliver":
					x.Cargo.Fish[0] = loc
					anchor := x.Board.Council.Anchors[0]
					explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return e.A == anchor || e.B == anchor })
				case "land":
					x.Cargo.Units[actor*11+2] = loc
					explorerSpiceDock(t, s, ship, func(e CatanEdge) bool { return catanExplorerTouches(g, e.ID, tile) })
				case "resolve":
					x.Lairs.Sites[0].Ready, x.Lairs.Sites[0].Captor = g.TurnSerial, actor
					for i := 2; i < 5; i++ {
						x.Cargo.Units[actor*11+i] = catanExplorerCargoLocation{"lair", tile}
					}
				}
				if err := s.validateCatanExplorer(); err != nil {
					t.Fatal("invalid mission fixture", err)
				}
				explorerFishApply(t, s, Action{Type: "catan_explorer_begin_move"})
				if kind == "land" {
					explorerFishApply(t, s, Action{Type: "catan_explorer_fish_roll"})
				}
				if kind == "resolve" {
					explorerFishApply(t, s, Action{Type: "catan_end"})
				}
				s = explorerStateRestore(t, s)
				first, err := s.BotAction(actor)
				if err != nil {
					t.Fatal(err)
				}
				if first.Type != "catan_explorer_"+kind {
					t.Fatal("required mission action missing", kind, first)
				}
				assertExplorerFishBotPrivate(t, s, actor, first)
				explorerFishApply(t, s, first)
				x = s.Catan.Explorer
				switch kind {
				case "fish_roll":
					if x.Fish.LastRoll == nil || x.Fish.LastRoll.Sequence != g.TurnSerial {
						t.Fatal("fish roll missing")
					}
				case "fish_load":
					if x.Cargo.Fish[first.Card] != loc {
						t.Fatal("fish not loaded")
					}
				case "fish_deliver":
					if len(x.Fish.Deliveries) != 1 || x.Cargo.Fish[0].Kind != "supply" {
						t.Fatal("fish not delivered")
					}
				case "land":
					if x.Cargo.Units[actor*11+2] != (catanExplorerCargoLocation{"lair", tile}) {
						t.Fatal("crew not landed")
					}
				case "resolve":
					if x.Lairs.Sites[0].Resolved == 0 {
						t.Fatal("lair not resolved")
					}
				}
				if s.Finished {
					t.Fatal("fixture unexpectedly finished")
				}
				explorerStateRestore(t, s)
			})
		}
	}
}
