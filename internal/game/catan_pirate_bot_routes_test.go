package game

import (
	"encoding/json"
	"os"
	"testing"
)

// Failure scenarios captured from a natural six-player HTTP game:
//   - after capturing its fortress, a bot builds a pointless road on a pirate
//     island and permanently blocks another player's last expedition edge;
//   - Road Building bypasses that strategy and forces the same blockade;
//   - avoiding the blockade must not revoke a human's otherwise legal action.
func pirateHarborStalled(t *testing.T) *State {
	t.Helper()
	raw, err := os.ReadFile("testdata/catan-pirate-harbor-stalled.json")
	if err != nil {
		t.Fatal(err)
	}
	var s State
	if err = json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return &s
}
func TestCatanPirateBotAvoidsPointlessOffshoreRoad(t *testing.T) {
	s := pirateHarborStalled(t)
	g := s.Catan
	s.Turn, s.Phase = 1, "catan_turn"
	g.Edges[0].Owner = -1
	if !g.canRoad(1, 0) {
		t.Fatal("captured blockade edge must remain legal for a human")
	}
	a, err := s.BotAction(1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Type == "catan_road" && (!g.pirateHomeVertex(g.Edges[a.Edge].A) || !g.pirateHomeVertex(g.Edges[a.Edge].B)) {
		t.Fatal("bot recreates the permanent blockade", a)
	}
}

// Knight modules can give roads additional uses; the ordinary pirate policy
// must not remove those modules' road candidates without evaluating them.
func TestCatanPirateRoadPolicyPreservesKnightModules(t *testing.T) {
	for _, attack := range []bool{false, true} {
		s := pirateHarborStalled(t)
		if attack {
			s.Catan.Attack = &catanAttack{}
		} else {
			s.Catan.CitiesKnights = &CatanCitiesKnights{}
		}
		if !s.Catan.pirateBotRoadUseful(0) {
			t.Fatal("ordinary pirate heuristic removed a knight-module road")
		}
	}
}
func TestCatanPirateRoadBuildingAvoidsForcedBlockade(t *testing.T) {
	s := pirateHarborStalled(t)
	g := s.Catan
	s.Turn, s.Phase = 1, "catan_turn"
	g.Edges[0].Owner = -1
	// Move one real discarded Road Building card back to its owner for this
	// isolated decision; do not create an extra development card.
	found := false
	for i, kind := range g.DevDiscard {
		if kind == 1 {
			g.DevDiscard = append(g.DevDiscard[:i], g.DevDiscard[i+1:]...)
			g.Players[1].Dev[1]++
			found = true
			break
		}
	}
	if !found {
		t.Fatal("fixture needs a road building card")
	}
	g.PlayedDev = false
	trial := clone(*s)
	if err := trial.Apply(1, Action{Type: "catan_dev", Card: 1}); err != nil {
		t.Fatal("fixture cannot legally play Road Building", err)
	}
	for step := 0; step < 4; step++ {
		a, err := s.BotAction(1)
		if err != nil {
			t.Fatal(err)
		}
		if a.Type == "catan_road" && (!g.pirateHomeVertex(g.Edges[a.Edge].A) || !g.pirateHomeVertex(g.Edges[a.Edge].B)) {
			t.Fatal("Road Building recreates blockade", a)
		}
		if a.Type != "catan_dev" && s.Phase != "catan_roads" {
			return
		}
		if err = s.Apply(1, a); err != nil {
			t.Fatal(err)
		}
		g = s.Catan
		if s.Phase != "catan_roads" {
			return
		}
	}
	t.Fatal("free routes did not finish")
}
