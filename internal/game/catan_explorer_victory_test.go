package game

import (
	"encoding/json"
	"slices"
	"testing"
)

// Explicit boundary fixture, not a natural full game: normal setup and reveal
// transactions followed by three captured lairs with physical crew assigned.
// The six numbers remain artificial private acceptance components.
func explorerVictoryFixture(t *testing.T, shared bool) (*State, []int) {
	t.Helper()
	s := explorerLairsStarted(t, 3)
	if err := s.catanExplorerRoll([2]int{1, 2}); err != nil {
		t.Fatal(err)
	}
	g, actor := s.Catan, s.Turn
	x := g.Explorer
	for _, h := range slices.Clone(x.Board.Hidden) {
		if _, err := x.discover(g, actor, []int{h.Tile}); err != nil {
			t.Fatal(err)
		}
	}
	tiles := []int{}
	next := make([]int, 3)
	for p := range next {
		next[p] = p*11 + 2
	}
	for i := 0; i < 3; i++ {
		site := &x.Lairs.Sites[i]
		tiles = append(tiles, site.Tile)
		site.Ready, site.Captor = g.TurnSerial, actor
		for j := 0; j < 3; j++ {
			p := actor
			if shared && i == 0 && j > 0 {
				p = (actor + 1) % 3
			}
			x.Cargo.Units[next[p]] = catanExplorerCargoLocation{"lair", site.Tile}
			next[p]++
		}
	}
	for _, kind := range []string{"catan_explorer_begin_move", "catan_end"} {
		if err := s.Apply(actor, Action{Type: kind, Prompt: int(g.TurnSerial)}); err != nil {
			t.Fatal(err)
		}
	}
	return explorerStateRestore(t, s), tiles
}

func explorerVictoryBuildings(t *testing.T, s *State, player, target int) {
	t.Helper()
	g := s.Catan
	points, harbors, settlements := 0, 0, 0
	for _, v := range g.Vertices {
		if v.Owner == player {
			points += v.Level
			if v.Level == 2 {
				harbors++
			} else {
				settlements++
			}
		}
	}
	for i, v := range g.Vertices {
		if points >= target {
			break
		}
		if v.Level != 0 || !catanExplorerLandVertex(g, i) {
			continue
		}
		clear := true
		for _, e := range g.Edges {
			if e.A == i && g.Vertices[e.B].Level > 0 || e.B == i && g.Vertices[e.A].Level > 0 {
				clear = false
			}
		}
		if !clear {
			continue
		}
		level := 1
		if harbors < 4 && target-points >= 2 && catanExplorerCoast(g, i) {
			level = 2
		}
		if level == 1 && settlements >= 4 {
			continue
		} // One settler remains aboard.
		g.Vertices[i].Owner, g.Vertices[i].Level = player, level
		points += level
		if level == 2 {
			harbors++
		} else {
			settlements++
		}
	}
	if points != target {
		t.Fatalf("could only place %d of %d fixture building points", points, target)
	}
	s.catanExplorerMissionScore()
	if err := s.validateCatanExplorer(); err != nil {
		t.Fatal(err)
	}
}

func TestCatanExplorerParticipationVictoryStopsBeforeHeroAndOtherRewards(t *testing.T) {
	for _, shared := range []bool{false, true} {
		name := "solo"
		if shared {
			name = "shared"
		}
		t.Run(name, func(t *testing.T) {
			s, tiles := explorerVictoryFixture(t, shared)
			actor, serial := s.Turn, s.Catan.TurnSerial
			explorerVictoryBuildings(t, s, actor, 10)
			x := s.Catan.Explorer
			// Only the winner's reward remains in the bank. Later rewards will never
			// occur, so they must not prevent an otherwise valid immediate victory.
			other := (actor + 1) % 3
			x.Economy.Gold[other] += x.Economy.GoldBank - 2
			x.Economy.GoldBank = 2
			beforeGold := slices.Clone(x.Economy.Gold)
			if err := s.Apply(actor, Action{Type: "catan_explorer_resolve", Target: tiles[0], Prompt: int(serial)}); err != nil {
				t.Fatal(err)
			}
			s = explorerStateRestore(t, s)
			x = s.Catan.Explorer
			if !s.Finished || s.Phase != "finished" || !slices.Equal(s.Winners, []int{actor}) || s.Turn != actor || s.Catan.TurnSerial != serial || s.Catan.Players[actor].Score != 12 {
				t.Fatal("participation did not immediately win", s.Phase, s.Winners)
			}
			if x.Lairs.Battle != nil || x.Lairs.RewardVictory == nil || x.Lairs.Progress[actor] != 1 || x.Economy.GoldBank != 0 {
				t.Fatal("hero reward or later transaction ran")
			}
			for p := range beforeGold {
				want := beforeGold[p]
				if p == actor {
					want += 2
				}
				if x.Economy.Gold[p] != want || p != actor && x.Lairs.Progress[p] != 0 {
					t.Fatal("later contributor received a reward")
				}
			}
			for _, tile := range tiles {
				site := x.Lairs.Sites[x.Lairs.site(tile)]
				if site.Resolved != 0 || site.Hero != -1 || len(site.Rounds) != 0 || s.Catan.Tiles[tile].Number != 0 || len(x.Cargo.contents(catanExplorerCargoLocation{"lair", tile})) != 3 {
					t.Fatal("unfinished battle was completed")
				}
			}
			for viewer := -1; viewer < 3; viewer++ {
				view := s.View(viewer)["catan"].(map[string]any)["explorer"].(map[string]any)
				if len(view["choices"].([]map[string]any)) != 0 {
					t.Fatal("finished game offers actions")
				}
				for _, site := range view["lairs"].(catanExplorerLairsView).Sites {
					if site.Number != 0 {
						t.Fatal("unliberated number revealed")
					}
				}
			}
			before, _ := json.Marshal(s)
			if err := s.Apply(actor, Action{Type: "catan_explorer_resolve", Target: tiles[1], Prompt: int(serial)}); err == nil {
				t.Fatal("continued after victory")
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) {
				t.Fatal("postgame action mutated state")
			}
			bad := clone(*s)
			bad.Finished, bad.Phase, bad.Winners = false, "catan_explorer_resolve", nil
			if err := bad.validateCatanExplorer(); err == nil {
				t.Fatal("unfinished reward victory accepted")
			}
			bad = clone(*s)
			bad.Catan.Explorer.Lairs.RewardVictory.Player = other
			if err := bad.validateCatanExplorer(); err == nil {
				t.Fatal("wrong victory claimant accepted")
			}
			bad = clone(*s)
			bad.Catan.Explorer.Lairs.RewardVictory = nil
			if err := bad.validateCatanExplorer(); err == nil {
				t.Fatal("partial rewards without victory ledger accepted")
			}
		})
	}
}

func TestCatanExplorerHeroVictoryDoesNotWaitForRemainingLair(t *testing.T) {
	s, tiles := explorerVictoryFixture(t, true)
	actor, serial := s.Turn, s.Catan.TurnSerial
	if err := s.Apply(actor, Action{Type: "catan_explorer_resolve", Target: tiles[0], Prompt: int(serial)}); err != nil {
		t.Fatal(err)
	}
	// The first battle awards its hero to the other contributor, leaving the
	// active player at step 1 and the other at step 2. Dice are server-controlled.
	g := s.Catan
	x := g.Explorer
	if err := x.Lairs.apply(g, x.Board, x.Fleet, x.Cargo, x.Economy, actor, serial, "roll", tiles[0], 0, nil, func(int) int { return 0 }); err != nil {
		t.Fatal(err)
	}
	s.catanExplorerMissionScore()
	s.catanExplorerSyncPhase()
	explorerVictoryBuildings(t, s, actor, 10)
	if g.Players[actor].Score != 11 {
		t.Fatal("wrong pre-victory score")
	}
	// Solo capture first advances to step 2 (still 11 points), then the hero
	// reaches step 3 and takes leadership. A third captured lair remains pending.
	if err := s.Apply(actor, Action{Type: "catan_explorer_resolve", Target: tiles[1], Prompt: int(serial)}); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	x = s.Catan.Explorer
	if !s.Finished || !slices.Equal(s.Winners, []int{actor}) || s.Catan.TurnSerial != serial || s.Catan.Players[actor].Score != 13 || x.Lairs.RewardVictory != nil {
		t.Fatal("hero win waited for another lair")
	}
	if x.Lairs.Sites[x.Lairs.site(tiles[1])].Hero != actor || x.Lairs.Sites[x.Lairs.site(tiles[2])].Resolved != 0 {
		t.Fatal("incorrect battle completion")
	}
}

func TestCatanExplorerOtherContributorCannotWinOutOfTurn(t *testing.T) {
	s, tiles := explorerVictoryFixture(t, true)
	actor, other := s.Turn, (s.Turn+1)%3
	explorerVictoryBuildings(t, s, other, 11)
	if err := s.Apply(actor, Action{Type: "catan_explorer_resolve", Target: tiles[0], Prompt: int(s.Catan.TurnSerial)}); err != nil {
		t.Fatal(err)
	}
	s = explorerStateRestore(t, s)
	if s.Finished || s.Phase != "catan_explorer_battle" || s.Catan.Players[other].Score != 12 || s.Catan.Explorer.Lairs.RewardVictory != nil {
		t.Fatal("non-active contributor won out of turn")
	}
}
