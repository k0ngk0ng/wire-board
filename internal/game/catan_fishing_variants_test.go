package game

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
)

func fishingVariantGame(t *testing.T, n int, scene string, knights, friendly, harbors bool) *State {
	t.Helper()
	options := CatanOptions{FiveSix: n > 4}
	var s *State
	var err error
	switch {
	case knights:
		s, err = NewCatanFishingCitiesKnights(n, options)
	case scene == "":
		s, err = NewCatanFishing(n, options)
	case scene == "new_world":
		var world *CatanNewWorldMap
		world, err = GenerateCatanFishingNewWorldMap(n)
		if err == nil {
			s, err = NewCatanFishingNewWorld(n, options, world)
		}
	default:
		s, err = NewCatanFishingSeafarers(n, options, CatanSeafarersSetup{Scenario: scene}, nil)
	}
	if err != nil {
		t.Fatal(n, scene, err)
	}
	if friendly {
		if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	if harbors {
		if err = s.ConfigureCatanHarbors(CatanHarborsSetup{Enabled: true}); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestCatanFishingVariantsConfigurationMatrix(t *testing.T) {
	for n := 3; n <= 6; n++ {
		for _, scene := range []string{"", "islands", "fog", "desert", "tribe", "cloth", "wonders", "new_world", "knights"} {
			if n > 4 && slices.Contains([]string{"islands", "desert", "tribe", "cloth"}, scene) {
				continue
			}
			for mode := 1; mode <= 3; mode++ {
				t.Run(fmt.Sprintf("%d/%s/%d", n, scene, mode), func(t *testing.T) {
					s := fishingVariantGame(t, n, scene, scene == "knights", mode&1 != 0, mode&2 != 0)
					g := s.Catan
					if err := g.validateFishing(); err != nil {
						t.Fatal(err)
					}
					if err := s.validateCatanFriendlyFallback(); err != nil {
						t.Fatal(err)
					}
					target := 10
					if scene == "knights" {
						target = 13
					}
					if g.Seafarers != nil {
						target = g.Seafarers.VictoryPoints
					}
					if mode&2 != 0 {
						target++
					}
					if g.victoryTarget() != target {
						t.Fatal("variant target", g.victoryTarget(), target)
					}
					if mode&1 != 0 && g.FriendlyRobber.Fallback != CatanFriendlyFishingFallbackRules {
						t.Fatal("missing source")
					}
					if scene == "knights" && mode&1 != 0 && g.FriendlyRobber.Knights != CatanFriendlyKnightsRules {
						t.Fatal("missing knights source")
					}
					restored := clone(*s)
					if !reflect.DeepEqual(*s, restored) {
						t.Fatal("restore changed combination")
					}
					if _, err := restored.BotAction(restored.Turn); err != nil {
						t.Fatal("opening bot", err)
					}
				})
			}
		}
	}
}

// Synthetic all-protected coverage on an otherwise intact Fishing map. This
// isolates the no-desert boundary, not a claim about reachable building counts.
func fishingProtectedMap(t *testing.T, knights bool) *State {
	s := fishingVariantGame(t, 3, "", knights, true, false)
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Turn, s.Phase, g.ResumePhase = 0, "catan_robber", "catan_turn"
	for _, tile := range g.Tiles {
		v := tile.Vertices[0]
		g.Vertices[v].Owner, g.Vertices[v].Level = 1, 1
	}
	g.Players[1].Score = 2
	helperGrant(s, 1, []int{1, 1, 0, 0, 0})
	if knights {
		g.CitiesKnights.Invasions = 1
	}
	return s
}

func TestCatanFishingFriendlyFallbackAndLakeProtection(t *testing.T) {
	for _, knights := range []bool{false, true} {
		s := fishingProtectedMap(t, knights)
		g := s.Catan
		if !g.robberAllowed(-1) {
			t.Fatal("no-desert fallback missing")
		}
		lake := g.Fishing.Map.Lakes[0].Tile
		if g.robberAllowed(lake) {
			t.Fatal("protected lake treated as desert")
		}
		legal := s.View(0)["catan"].(map[string]any)["legal"].(map[string][]int)["robber"]
		if !slices.Equal(legal, []int{-1}) {
			t.Fatal("unsafe legal choices", legal)
		}
		action, err := s.BotAction(0)
		if err != nil || action.Type != "catan_robber" || action.Tile != -1 {
			t.Fatal("stalled bot", action, err)
		}
		before := slices.Clone(g.Players[1].Resources)
		restored := clone(*s)
		helperApply(t, &restored, 0, action)
		if restored.Catan.Robber != -1 || restored.Phase != "catan_turn" || !slices.Equal(restored.Catan.Players[1].Resources, before) {
			t.Fatal("retreat stole or stalled")
		}
		s = fishingProtectedMap(t, knights)
		g = s.Catan
		lake = g.Fishing.Map.Lakes[0].Tile
		for _, id := range g.Tiles[lake].Vertices {
			g.Vertices[id].Owner, g.Vertices[id].Level = -1, 0
		}
		if !g.robberAllowed(lake) || g.robberAllowed(-1) {
			t.Fatal("unprotected lake must be used before outside")
		}
		helperReject(t, s, 0, Action{Type: "catan_robber", Tile: -1})
	}
	s := fishingProtectedMap(t, true)
	s.Catan.CitiesKnights.Invasions = 0
	if s.Catan.robberAllowed(-1) || len(s.Catan.taxationTiles()) != 0 {
		t.Fatal("fallback bypassed dormancy")
	}
	s.Catan.CitiesKnights.Invasions = 1
	s.Phase = "catan_turn"
	ckProgressGive(t, s, 0, 21)
	if !slices.Equal(s.Catan.taxationTiles(), []int{-1}) {
		t.Fatal("missing Taxation retreat")
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 21, Tile: -1})
	if s.Catan.Robber != -1 || len(s.Catan.CitiesKnights.Players[0].Progress) != 0 || sum(s.Catan.Players[0].Resources) != 0 {
		t.Fatal("outside Taxation effect")
	}
}

func TestCatanFishingFriendlyPaymentAndHarborBoot(t *testing.T) {
	for _, knights := range []bool{false, true} {
		var s *State
		p := 0
		if knights {
			s = fishCityActionGame(t, 6, "catan_turn")
		} else {
			s, p = fishActionFixture(t, 6, "catan_turn")
		}
		s.enableCatanFriendlyRobber()
		g := s.Catan
		target := (p + 1) % 6
		helperGrant(s, target, []int{1, 0, 0, 0, 0})
		if !g.friendlyProtected(target) {
			t.Fatal("fixture target not protected")
		}
		helperApply(t, s, p, Action{Type: "catan_fish_steal", Target: target, Tokens: g.fishPayment(p, 3)})
		g = s.Catan
		if g.Players[target].Resources[0] != 0 || g.Players[p].Resources[0] != 1 {
			t.Fatal("paid theft incorrectly blocked")
		}
	}
	s, p := fishActionFixture(t, 3, "catan_turn")
	s.enableCatanFriendlyRobber()
	helperApply(t, s, p, Action{Type: "catan_fish_robber", Tokens: s.Catan.fishPayment(p, 2)})
	if s.Catan.Robber != -1 {
		t.Fatal("paid removal failed")
	}
	for _, knights := range []bool{false, true} {
		s := fishingVariantGame(t, 3, "", knights, true, true)
		g := s.Catan
		g.SetupStep = g.SetupLimit()
		s.Turn, s.Phase = 0, "catan_turn"
		baseTarget := 11
		if knights {
			baseTarget = 14
		}
		// Two actual port buildings earn the award; ties do not replace the owner.
		for i, level := range []int{2, 1} {
			v := g.Edges[g.Ports[i].Edge].A
			g.Vertices[v].Owner, g.Vertices[v].Level = 0, level
		}
		s.catanScores()
		if g.Harbors.Owner != 0 || g.Players[0].Score != 5 || g.friendlyProtected(0) {
			t.Fatal("harbor award/protection")
		}
		fishTop(&g.Fishing.Tokens, catanFishBoot)
		if err := g.Fishing.Tokens.beginDraw(0, []int{1, 0, 0}); err != nil {
			t.Fatal(err)
		}
		if g.victoryTargetFor(0) != baseTarget+1 || g.victoryTargetFor(1) != baseTarget {
			t.Fatal("boot did not stack")
		}
		g.Players[0].Score = baseTarget
		g.Players[1].Score = baseTarget
		s.catanVictory()
		if s.Finished {
			t.Fatal("boot bypassed")
		}
		helperApply(t, s, 0, Action{Type: "catan_fish_boot", Target: 1})
		if !s.Finished || !slices.Equal(s.Winners, []int{0}) {
			t.Fatal("passing boot did not win")
		}
	}
}

func TestCatanFishingFallbackMetadataIsolation(t *testing.T) {
	for _, change := range []func(*Catan){
		func(g *Catan) { g.FriendlyRobber.Fallback = "unknown" },
		func(g *Catan) { g.Fishing = nil },
		func(g *Catan) { g.FriendlyRobber.Rules = "unknown" },
	} {
		s := fishingProtectedMap(t, false)
		change(s.Catan)
		helperReject(t, s, 0, Action{Type: "catan_robber", Tile: -1})
	}
	s := fishingProtectedMap(t, false)
	s.Catan.FriendlyRobber.Fallback = ""
	if err := s.validateCatanFriendlyFallback(); err != nil {
		t.Fatal(err)
	}
	if s.Catan.robberAllowed(-1) {
		t.Fatal("legacy silently received new fallback")
	}
	base := friendlyTopology(t)
	if base.Catan.robberAllowed(-1) || base.Catan.FriendlyRobber.Fallback != "" {
		t.Fatal("base changed")
	}
}

func TestCatanFishingFriendlyPirateRetreat(t *testing.T) {
	s := fishingVariantGame(t, 3, "islands", false, true, false)
	g := s.Catan
	g.SetupStep = g.SetupLimit()
	s.Turn, s.Phase, g.ResumePhase = 0, "catan_robber", "catan_turn"
	for _, tile := range g.Tiles {
		if tile.Resource != CatanSea {
			continue
		}
		for _, e := range g.Edges {
			if slices.Contains(e.Tiles, tile.ID) {
				g.Edges[e.ID].Owner, g.Edges[e.ID].Ship = 1, true
				break
			}
		}
	}
	g.Players[1].Score = 2
	g.Seafarers.Pirate = -1
	if !g.pirateDestinationAllowed(0, -1) {
		t.Fatal("already-outside pirate stalled")
	}
	helperApply(t, s, 0, Action{Type: "catan_pirate", Tile: -1})
	if s.Phase != "catan_turn" || s.Catan.Seafarers.Pirate != -1 || len(s.Catan.Victims) != 0 {
		t.Fatal("pirate retreat stole or stalled")
	}
}
