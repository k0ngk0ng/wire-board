package game

import (
	"slices"
	"testing"
)

func TestCatanCardTradeAdvantageAwardHolderAndSeaRoute(t *testing.T) {
	for _, sea := range []bool{false, true} {
		s := earthquakeGraph(t, []int{0, 0, 0, 0, 0, 1, 1})
		s.Phase, s.Turn = "catan_roll", 1
		g := s.Catan
		if sea {
			g.Seafarers = &CatanSeafarers{Pirate: -1}
			// A road/ship route joins at an owned coastal settlement.
			g.Vertices[3].Owner, g.Vertices[3].Level = 0, 1
			g.Edges[3].Ship, g.Edges[4].Ship = true, true
		}
		s.catanScores()
		if g.LongestOwner != 0 || g.Players[0].RoadLength != 5 {
			t.Fatal("fixture must have actual longest road/sea route")
		}
		helperGrant(s, 2, []int{0, 0, 0, 0, 1})
		beginCardEvent(t, s, "trade_advantage", 2, 0, 0)
		if !slices.Equal(s.Catan.CardEvent.Players, []int{0}) || s.Catan.CardEvent.Optional || s.Turn != 1 {
			t.Fatal("wrong trade advantage actor")
		}
		for viewer := -1; viewer < 3; viewer++ {
			legal := s.View(viewer)["catan"].(map[string]any)["legal"].(map[string][]int)
			if (len(legal["eventTargets"]) == 1) != (viewer == 0) {
				t.Fatal("trade targets shown to wrong viewer")
			}
		}
		helperReject(t, s, 0, Action{Type: "catan_event_skip"})
		helperReject(t, s, 1, Action{Type: "catan_event_steal", Target: 2})
		saved := clone(*s)
		s = &saved
		s.AutoCatanPending()
		if s.Catan.Players[0].Resources[4] != 1 || sum(s.Catan.Players[2].Resources) != 0 || s.Catan.LongestOwner != 0 || s.Phase != "catan_turn" || s.Turn != 1 {
			t.Fatal("restored trade advantage failed or changed award/turn")
		}
		catanCheck(t, s)
	}
}

func TestCatanCardTradeAdvantageNoUnawardedFallback(t *testing.T) {
	for _, variant := range []string{"short_unique", "unclaimed_tie", "eliminated", "empty"} {
		s := earthquakeGraph(t, []int{0, 0, 0, 0, 1, 1, 1, 1, 1})
		s.Phase, s.Turn = "catan_roll", 0
		g := s.Catan
		switch variant {
		case "short_unique":
			g.Edges[8].Owner, g.Edges[7].Owner = -1, -1
		case "unclaimed_tie":
			g.Edges = append(g.Edges, CatanEdge{ID: 9, A: 0, B: 10, Owner: 0})
			g.Vertices = append(g.Vertices, CatanVertex{ID: 10, Owner: -1})
		case "eliminated":
			g.Players[1].Eliminated = true
		}
		s.catanScores()
		if variant != "empty" {
			helperGrant(s, 2, []int{1, 0, 0, 0, 0})
		}
		beginCardEvent(t, s, "trade_advantage", 2, 0, 0)
		if s.Catan.CardEvent != nil || s.Phase != "catan_turn" {
			t.Fatal("unawarded or empty trade advantage stalled", variant)
		}
		if variant != "empty" && s.Catan.Players[2].Resources[0] != 1 {
			t.Fatal("unawarded longest route used a legacy fallback", variant)
		}
	}
}

func TestCatanCardTradeAdvantageCityCommodityCombinationGated(t *testing.T) {
	s := ckEvent(t)
	s.Catan.LongestOwner = 0
	helperGrant(s, 1, []int{0, 0, 0, 0, 0, 1, 0, 0})
	// 2025 T&B omits a C&K paragraph for this effect. Do not silently
	// interpret commodities or expose commodity-only hands through targets.
	rejectCardEvent(t, s, "trade_advantage", 2, 6, 0)
}
