package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func friendlyGame(t *testing.T) *State {
	t.Helper()
	s, err := NewCatanFriendlyRobber(3, CatanOptions{}, CatanBaseConfiguration{})
	if err != nil {
		t.Fatal(err)
	}
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	g.TurnSerial = 1
	s.Turn, s.Phase, g.ResumePhase = 0, "catan_robber", "catan_turn"
	return s
}

// Explicit minimal topologies isolate protection and fallback cases. They are
// not presented as legal map layouts or complete playthroughs.
func friendlyTopology(t *testing.T) *State {
	s := friendlyGame(t)
	g := s.Catan
	g.Tiles = []CatanTile{
		{ID: 0, Resource: CatanDesert, Vertices: []int{0}},
		{ID: 1, Resource: 0, Number: 6, Vertices: []int{1}},
		{ID: 2, Resource: 1, Number: 8, Vertices: []int{2}},
		{ID: 3, Resource: 2, Number: 5, Vertices: []int{3}},
	}
	g.Robber = 0
	for p := range g.Players {
		g.Players[p].Score = 2
	}
	for i := 0; i < 4; i++ {
		g.Vertices[i].Owner, g.Vertices[i].Level = i%3, 1
	}
	return s
}

func TestCatanFriendlyRobberVisiblePointsAndOwnBuildings(t *testing.T) {
	s := friendlyTopology(t)
	g := s.Catan
	if g.robberAllowed(1) || g.robberAllowed(2) || g.robberAllowed(3) {
		t.Fatal("two-point player affected")
	}
	// A face-down VP does not remove protection or change hints for any viewer.
	g.Players[1].Score = 3
	g.Players[1].Dev[4] = 1
	if g.robberAllowed(1) || !g.friendlyProtected(1) {
		t.Fatal("hidden VP removed protection")
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		v := s.View(viewer)["catan"].(map[string]any)
		if !reflect.DeepEqual(v["friendlyRobber"].(map[string]any)["protectedPlayers"], []int{0, 1, 2}) {
			t.Fatal("public protection differs", viewer)
		}
		if viewer != 1 {
			p := v["players"].([]any)[1].(map[string]any)
			for _, key := range []string{"resources", "dev", "newDev"} {
				if _, leak := p[key]; leak {
					t.Fatal("private cards leaked")
				}
			}
		}
	}
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 1})
	// Everyone else is protected; FAQ55 still requires blocking your own land.
	g.Players[0].Score = 3
	if !g.robberAllowed(3) || g.robberAllowed(0) {
		t.Fatal("own unprotected land must be used before desert fallback")
	}
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: 3})
	if s.Phase != "catan_turn" || len(s.Catan.Victims) != 0 {
		t.Fatal("own land should not steal")
	}
}

func TestCatanFriendlyRobberDesertFallbackAndSafeVictims(t *testing.T) {
	for _, alreadyDesert := range []bool{false, true} {
		s := friendlyTopology(t)
		g := s.Catan
		// The fallback desert may adjoin protected and unprotected players.
		g.Tiles[0].Vertices = []int{1, 2}
		g.Players[2].Score = 3
		// Every other land still touches a protected player.
		g.Tiles[2].Vertices = []int{1}
		if !alreadyDesert {
			g.Robber = 3
		}
		helperGrant(s, 1, []int{1, 0, 0, 0, 0})
		helperGrant(s, 2, []int{0, 1, 0, 0, 0})
		for id := range g.Tiles {
			if g.robberAllowed(id) != (id == 0) {
				t.Fatal("wrong fallback", id, alreadyDesert)
			}
		}
		restored := clone(*s)
		s = &restored
		a, err := s.BotAction(0)
		if err != nil {
			t.Fatal(err)
		}
		if a.Type != "catan_robber" || a.Tile != 0 {
			t.Fatal("bot did not use fallback", a)
		}
		helperApply(t, s, 0, a)
		if s.Phase != "catan_turn" || s.Catan.Robber != 0 || sum(s.Catan.Players[1].Resources) != 1 || sum(s.Catan.Players[2].Resources) != 0 || s.Catan.Players[0].Resources[1] != 1 {
			t.Fatal("fallback stole protected hand or stalled")
		}
		fleetSupply(t, s.Catan)
	}
	// If everyone is protected, a knight may still send the robber back without stealing.
	s := friendlyTopology(t)
	s.Phase = "catan_turn"
	s.Catan.Robber = 3
	catanCard(s.Catan, 0, 0)
	helperApply(t, s, 0, Action{Type: "catan_dev", Card: 0})
	helperApply(t, s, 0, Action{Type: "catan_robber", Tile: 0})
	if s.Phase != "catan_turn" || s.Catan.Players[0].Knights != 1 {
		t.Fatal("knight without theft")
	}
}

func TestCatanFriendlyRobberNoShortcutToDesertOrNeutralProtection(t *testing.T) {
	s := friendlyTopology(t)
	g := s.Catan
	g.Robber = 1
	g.Vertices[2].Owner = -1 // Neutral structures do not belong to a protected player.
	if !g.robberAllowed(2) || g.robberAllowed(0) {
		t.Fatal("neutral building blocks / premature fallback")
	}
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: 0})
	g.Vertices[2].Owner = 2
	g.Players[2].Eliminated = true
	if !g.robberAllowed(2) {
		t.Fatal("eliminated player blocks friendly robber")
	}
	// A revealed but empty land is a valid target even when nobody can be robbed.
	g.Tiles[3].Vertices = nil
	if !g.robberAllowed(3) {
		t.Fatal("empty land rejected")
	}
}

func TestCatanFriendlyRobberBuildChangesProtectionImmediately(t *testing.T) {
	s := friendlyGame(t)
	g := s.Catan
	s.Phase = "catan_turn"
	village := g.Edges[g.Ports[0].Edge].A
	other := g.Edges[g.Ports[3].Edge].A
	g.Vertices[village].Owner, g.Vertices[village].Level = 0, 1
	g.Vertices[other].Owner, g.Vertices[other].Level = 0, 1
	s.catanScores()
	if !g.friendlyProtected(0) {
		t.Fatal("two public points")
	}
	catanCard(g, 0, 4)
	s.catanScores()
	if !g.friendlyProtected(0) {
		t.Fatal("secret point removed protection")
	}
	helperGrant(s, 0, []int{0, 0, 0, 2, 3})
	helperApply(t, s, 0, Action{Type: "catan_city", Vertex: village})
	if s.Catan.friendlyProtected(0) || s.Catan.Players[0].Score != 4 {
		t.Fatal("third public point did not remove protection")
	}
	// Harbor award counts as visible score, while its harbor points do not.
	s = harborsGame(t)
	g = s.Catan
	harborBuildings(g, []int{3, 2, 0})
	s.enableCatanFriendlyRobber()
	s.catanScores()
	if g.friendlyProtected(0) || !g.friendlyProtected(1) {
		t.Fatal("harbor award visibility")
	}
}

func TestCatanFriendlyPirateProtectsShipsNotLandBuildings(t *testing.T) {
	s := seaTestGame(t)
	s.enableCatanFriendlyRobber()
	g := s.Catan
	edge := seaEdge(t, g, 0, 1)
	g.Edges[edge].Owner, g.Edges[edge].Ship = 1, true
	g.Players[1].Score = 2
	g.Vertices[g.Edges[edge].A].Owner, g.Vertices[g.Edges[edge].A].Level = 2, 1
	g.Players[2].Score = 2
	s.Phase = "catan_robber"
	g.ResumePhase = "catan_turn"
	helperGrant(s, 1, []int{1, 0, 0, 0, 0})
	if g.pirateDestinationAllowed(0, 1) {
		t.Fatal("protected ship destination allowed")
	}
	helperReject(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	legal := s.View(0)["catan"].(map[string]any)["legal"].(map[string][]int)["pirate"]
	if slices.Contains(legal, 1) {
		t.Fatal("hint exposes illegal protected pirate destination")
	}
	for _, choice := range g.pirateBotChoices(0) {
		if choice.action.Tile == 1 {
			t.Fatal("bot considers protected pirate destination")
		}
	}
	g.Players[1].Score = 3
	g.Players[1].Dev[4] = 1
	if g.pirateDestinationAllowed(0, 1) {
		t.Fatal("pirate counted hidden VP")
	}
	g.Players[1].Score = 4
	if !g.pirateDestinationAllowed(0, 1) {
		t.Fatal("shore settlement incorrectly blocks pirate")
	}
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: 1})
	if s.Phase != "catan_turn" || sum(s.Catan.Players[1].Resources) != 0 || sum(s.Catan.Players[0].Resources) != 1 {
		t.Fatal("pirate theft")
	}
	s.Phase = "catan_robber"
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: -1})
	if s.Phase != "catan_turn" || s.Catan.Seafarers.Pirate != -1 {
		t.Fatal("sea rim escape")
	}
}

func TestCatanFriendlyRobberStaleVictimRejectedAndBotPrivateIndependence(t *testing.T) {
	s := friendlyTopology(t)
	s.Phase = "catan_steal"
	s.Catan.Victims = []int{1}
	helperGrant(s, 1, []int{1, 0, 0, 0, 0})
	helperReject(t, s, 0, Action{Type: "catan_steal", Target: 1})
	s.Phase = "catan_robber"
	s.Catan.Victims = nil
	a, err := s.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	other := clone(*s)
	other.Catan.Players[1].Dev[4] = 2
	other.Catan.Players[1].Score += 2
	other.Catan.Players[1].Resources = []int{0, 9, 0, 2, 0}
	b, err := other.BotAction(0)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("private opponent cards changed friendly target")
	}
	if _, err := NewCatanFriendlyRobber(3, CatanOptions{Helpers: true}, CatanBaseConfiguration{}); err != nil {
		t.Fatal(err)
	}
}

func TestCatanFriendlyRobberBotsComplete(t *testing.T) {
	for _, n := range []int{3, 4, 5, 6} {
		for _, layout := range CatanBaseLayouts(n) {
			for _, harbors := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/%s/harbors=%v", n, layout, harbors), func(t *testing.T) {
					s, err := NewCatanFriendlyRobber(n, CatanOptions{FiveSix: n > 4}, CatanBaseConfiguration{Layout: layout})
					if err != nil {
						t.Fatal(err)
					}
					target := 10
					if harbors {
						s.enableCatanHarbors()
						target++
					}
					steps, moves := 0, 0
					for ; steps < 10000 && !s.Finished; steps++ {
						actor := ckActor(s)
						a, err := s.BotAction(actor)
						if err != nil {
							t.Fatal(steps, s.Phase, err)
						}
						if a.Type == "catan_robber" {
							moves++
							// Independently ensure that an ordinary target has no protected owner.
							tile := s.Catan.Tiles[a.Tile]
							if tile.Resource != CatanDesert {
								for _, id := range tile.Vertices {
									v := s.Catan.Vertices[id]
									if v.Level > 0 && v.Owner >= 0 && !s.Catan.Players[v.Owner].Eliminated {
										p := s.Catan.Players[v.Owner]
										if p.Score-p.Dev[4] < 3 {
											t.Fatal("bot blocked protected player")
										}
									}
								}
							}
						}
						helperApply(t, s, actor, a)
						fleetSupply(t, s.Catan)
						dev := len(s.Catan.DevDeck) + len(s.Catan.DevDiscard)
						for p, seat := range s.Catan.Players {
							dev += sum(seat.Dev)
							r, v, c := s.Catan.pieces(p)
							if r > 15 || v > 5 || c > 4 {
								t.Fatal("piece inventory")
							}
						}
						want := 25
						if n > 4 {
							want = 34
						}
						if dev != want {
							t.Fatal("development inventory", dev)
						}
						if steps%31 == 0 {
							restored := clone(*s)
							if !reflect.DeepEqual(*s, restored) {
								t.Fatal("save roundtrip")
							}
							s = &restored
						}
					}
					if !s.Finished || len(s.Winners) != 1 || s.Catan.Players[s.Winners[0]].Score < target || moves == 0 {
						t.Fatal("incomplete friendly game", steps, moves)
					}
					t.Logf("steps=%d robberMoves=%d score=%d", steps, moves, s.Catan.Players[s.Winners[0]].Score)
				})
			}
		}
	}
}
