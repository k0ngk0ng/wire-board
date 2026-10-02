package game

import (
	"reflect"
	"testing"
)

func TestRailSimultaneousSetupAndPrivacy(t *testing.T) {
	s := mustGame(t, "rail", 3)
	initial := clone(*s)
	for _, i := range []int{2, 0} {
		pending := s.Rail.SetupPending[i]
		v := s.View(i)["rail"].(map[string]any)
		if _, leaked := v["setupPending"]; leaked {
			t.Fatal("other pending tickets leaked")
		}
		if len(v["pending"].([]Ticket)) != 3 {
			t.Fatal("missing own selection")
		}
		keep := []int{pending[0].ID, pending[1].ID, pending[2].ID}
		if err := s.Apply(i, Action{Type: "keep", Keep: keep}); err != nil {
			t.Fatal(err)
		}
		if err := s.Apply(i, Action{Type: "keep", Keep: keep}); err == nil {
			t.Fatal("selected twice")
		}
	}
	if !s.Rail.Setup {
		t.Fatal("setup ended early")
	}
	s.AutoChooseRailSetup()
	if s.Rail.Setup || s.Turn != 0 || s.Phase != "turn" {
		t.Fatal("setup did not finish")
	}
	if len(s.Rail.Players[2].Tickets) != 3 || !reflect.DeepEqual(s.Rail.Players[1].Tickets, initial.Rail.SetupPending[1][:2]) {
		t.Fatal("auto selection changed chosen tickets")
	}
	railInvariant(t, s)
}

func TestRailFaceReplacementKeepsSlots(t *testing.T) {
	s := mustGame(t, "rail", 2)
	s.AutoChooseRailSetup()
	g := s.Rail
	g.Face = []int{0, 1, 2, 3, 4}
	g.Deck = []int{5, 6, 7}
	g.Discard = nil
	if err := s.Apply(0, Action{Type: "draw", Slot: 1}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Face, []int{0, 5, 2, 3, 4}) {
		t.Fatal("market shifted", g.Face)
	}
	g.Deck = nil
	if err := s.Apply(0, Action{Type: "draw", Slot: 3}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Face, []int{0, 5, 2, -1, 4}) {
		t.Fatal("empty slot shifted", g.Face)
	}
}

func TestRailEliminationPreservesRoutesAndContinues(t *testing.T) {
	s := mustGame(t, "rail", 3)
	s.AutoChooseRailSetup()
	s.Rail.Owners[1] = 0
	for _, route := range MapData().Routes {
		if route.ID == 1 {
			s.Rail.Players[0].Trains -= route.Length
		}
	}
	if err := s.EliminateRail(1); err == nil {
		t.Fatal("removed noncurrent player")
	}
	if err := s.EliminateRail(0); err != nil {
		t.Fatal(err)
	}
	if s.Turn != 1 || s.Finished || s.Rail.Owners[1] != 0 || sum(s.Rail.Players[0].Hand) != 0 {
		t.Fatal("bad elimination")
	}
	railInvariant(t, s)
	if err := s.EliminateRail(1); err != nil {
		t.Fatal(err)
	}
	if !s.Finished || !reflect.DeepEqual(s.Winners, []int{2}) {
		t.Fatal("survivor not winner", s.Winners)
	}
	railInvariant(t, s)
}

func TestRailLegacySetupMigrationAndFinalRoundElimination(t *testing.T) {
	s := mustGame(t, "rail", 3)
	g := s.Rail
	// Simulate the persisted sequential setup format after player zero chose.
	g.Players[0].Tickets = append([]Ticket{}, g.SetupPending[0][:2]...)
	g.TicketDeck = append(g.TicketDeck, g.SetupPending[0][2])
	g.Pending = g.SetupPending[1]
	g.TicketDeck = append(g.SetupPending[2], g.TicketDeck...)
	g.SetupPending = nil
	s.Turn = 1
	if !s.UpgradeRailSetup() || s.UpgradeRailSetup() {
		t.Fatal("migration should happen exactly once")
	}
	if len(g.SetupPending[0]) != 0 || len(g.SetupPending[1]) != 3 || len(g.SetupPending[2]) != 3 {
		t.Fatal("migration lost pending tickets")
	}
	railInvariant(t, s)
	s.AutoChooseRailSetup()
	g.LastRemaining = 2
	if err := s.EliminateRail(0); err != nil {
		t.Fatal(err)
	}
	if s.Finished || s.Turn != 1 || g.LastRemaining != 1 {
		t.Fatal("final round advanced incorrectly")
	}
	apply(t, s, Action{Type: "draw", Slot: -1})
	apply(t, s, Action{Type: "draw", Slot: -1})
	if !s.Finished {
		t.Fatal("final round did not finish")
	}
	for _, winner := range s.Winners {
		if winner == 0 {
			t.Fatal("eliminated player won")
		}
	}
}

func TestRailPublicSlotVersionsTrackSameColorAndReset(t *testing.T) {
	s := mustGame(t, "rail", 2)
	s.AutoChooseRailSetup()
	g := s.Rail
	g.Face = []int{0, 1, 2, 3, 4}
	g.Deck = []int{1, 6, 7}
	g.Discard = nil
	before := g.FaceVersion
	if err := s.Apply(0, Action{Type: "draw", Slot: 1}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Face, []int{0, 1, 2, 3, 4}) {
		t.Fatal("unexpected color replacement")
	}
	for i := range g.FaceVersion {
		if i == 1 && g.FaceVersion[i] == before[i] {
			t.Fatal("same-color replacement not observable")
		}
		if i != 1 && g.FaceVersion[i] != before[i] {
			t.Fatal("untouched slot changed")
		}
	}
	before = g.FaceVersion
	if err := s.Apply(0, Action{Type: "draw", Slot: -1}); err != nil {
		t.Fatal(err)
	}
	if g.FaceVersion != before {
		t.Fatal("blind draw changed market versions")
	}
	g.Face = []int{8, 8, 8, 0, 1}
	g.Deck = []int{2, 3, 4, 5}
	g.Discard = nil
	before = g.FaceVersion
	g.refill()
	for i := range g.FaceVersion {
		if g.FaceVersion[i] == before[i] {
			t.Fatal("reset omitted slot", i)
		}
	}
	// Loading an old snapshot with no versions remains valid and starts tracking changes.
	g.FaceVersion = [5]uint64{}
	g.Face = []int{0, -1, 2, 3, 4}
	g.Deck = []int{1}
	g.Discard = nil
	g.refill()
	if g.FaceVersion[1] == 0 {
		t.Fatal("legacy snapshot did not track refill")
	}
}
