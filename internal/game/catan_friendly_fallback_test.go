package game

import (
	"reflect"
	"slices"
	"testing"
)

// Small, explicitly controlled topology for the all-protected boundary;
// natural complete matches separately cover the actual scenario maps.
func friendlyOutsideFixture(t *testing.T) *State {
	s := seaTestGame(t)
	s.enableCatanFriendlyRobber()
	g := s.Catan
	s.Phase, g.ResumePhase = "catan_robber", "catan_turn"
	for _, tile := range g.Tiles {
		if tile.Resource == CatanSea {
			for _, e := range g.Edges {
				if slices.Contains(e.Tiles, tile.ID) {
					g.Edges[e.ID].Owner, g.Edges[e.ID].Ship = 1, true
					break
				}
			}
		} else {
			v := tile.Vertices[0]
			g.Vertices[v].Owner, g.Vertices[v].Level = 1, 1
		}
	}
	g.Players[1].Score = 2
	helperGrant(s, 1, []int{2, 1, 0, 0, 0})
	return s
}

func TestCatanFriendlyOutsideFallbackActionsHintsBotsAndRestore(t *testing.T) {
	for _, kind := range []string{"catan_robber", "catan_pirate"} {
		t.Run(kind, func(t *testing.T) {
			s := friendlyOutsideFixture(t)
			g := s.Catan
			if !g.robberAllowed(-1) || !g.pirateDestinationAllowed(0, -1) {
				t.Fatal("all-protected retreat missing")
			}
			view := s.View(0)["catan"].(map[string]any)
			for _, key := range []string{"robber", "pirate"} {
				if !slices.Equal(view["legal"].(map[string][]int)[key], []int{-1}) {
					t.Fatal("unsafe hints", key)
				}
			}
			if view["friendlyRobber"].(map[string]any)["fallback"] != CatanFriendlySeaFallbackRules {
				t.Fatal("missing public source")
			}
			bot, err := s.BotAction(0)
			if err != nil || bot.Tile != -1 || (bot.Type != "catan_robber" && bot.Type != "catan_pirate") {
				t.Fatal("bot stalled", bot, err)
			}
			before := clone(*s)
			restored := clone(*s)
			helperApply(t, &restored, 0, Action{Type: kind, Tile: -1})
			if restored.Phase != "catan_turn" || len(restored.Catan.Victims) != 0 || restored.Catan.FriendlyRobber.Fallback != CatanFriendlySeaFallbackRules {
				t.Fatal("retreat did not resume")
			}
			for p := range g.Players {
				if !slices.Equal(before.Catan.Players[p].Resources, restored.Catan.Players[p].Resources) {
					t.Fatal("retreat stole a card")
				}
			}
			if kind == "catan_robber" && restored.Catan.Robber != -1 {
				t.Fatal("robber not outside")
			}
		})
	}
}

func TestCatanFriendlyOutsideFallbackPriorityAndLegacyIsolation(t *testing.T) {
	s := friendlyOutsideFixture(t)
	g := s.Catan
	// A normal legal destination must be used before retreating outside.
	for _, v := range g.Tiles[3].Vertices {
		g.Vertices[v].Owner, g.Vertices[v].Level = -1, 0
	}
	if !g.robberAllowed(3) || g.robberAllowed(-1) {
		t.Fatal("outside shortcut despite legal land")
	}
	helperReject(t, s, 0, Action{Type: "catan_robber", Tile: -1})
	s = friendlyOutsideFixture(t)
	g = s.Catan
	g.Tiles[0].Resource, g.Tiles[0].Number = CatanDesert, 0
	if !g.robberAllowed(0) || g.robberAllowed(-1) {
		t.Fatal("desert no longer has priority")
	}
	// Tribe's non-numbered outer islands cannot become a desert escape route.
	g.Seafarers.Tribe = &CatanTribeState{}
	if g.robberAllowed(0) || !g.robberAllowed(-1) {
		t.Fatal("tribe land restriction bypassed")
	}
	for _, e := range g.Edges {
		if slices.Contains(e.Tiles, 1) {
			g.Edges[e.ID].Owner, g.Edges[e.ID].Ship = -1, false
		}
	}
	if !g.pirateDestinationAllowed(0, 1) || g.pirateDestinationAllowed(0, -1) {
		t.Fatal("pirate stayed outside despite legal sea")
	}
	s = friendlyOutsideFixture(t)
	s.Catan.Robber = -1
	if !s.Catan.robberAllowed(-1) {
		t.Fatal("already outside cannot finish")
	}
	s.Catan.FriendlyRobber.Fallback = ""
	if s.Catan.robberAllowed(-1) || s.Catan.pirateDestinationAllowed(0, -1) {
		t.Fatal("old save acquired new fallback")
	}
	base := friendlyTopology(t)
	if base.Catan.robberAllowed(-1) || base.Catan.FriendlyRobber.Fallback != "" {
		t.Fatal("base variant changed")
	}
}

func TestCatanFriendlyFallbackRejectsUnknownSavedRecipe(t *testing.T) {
	for _, change := range []func(*Catan){
		func(g *Catan) { g.FriendlyRobber.Fallback = "future" },
		func(g *Catan) { g.FriendlyRobber.Rules = "future" },
		func(g *Catan) { g.Seafarers = nil },
	} {
		s := friendlyOutsideFixture(t)
		change(s.Catan)
		before := clone(*s)
		if s.Apply(0, Action{Type: "catan_robber", Tile: -1}) == nil || !reflect.DeepEqual(before, *s) {
			t.Fatal("corrupt fallback changed state")
		}
	}
}
