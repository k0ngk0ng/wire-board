package game

import (
	"encoding/json"
	"reflect"
	"testing"
)

func assertRevealedEvent(t *testing.T, s *State, kind string, number int, started bool) {
	t.Helper()
	want := s.Catan.revealedEventView()
	if want == nil || want.Kind != kind || want.Production != number || want.ProductionStarted != started || want.RollID != s.Catan.RollID {
		t.Fatal("wrong public face", want)
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		raw := s.View(viewer)["catan"].(map[string]any)["revealedEvent"]
		if !reflect.DeepEqual(raw, *want) {
			t.Fatal("viewer-dependent public face", viewer, raw, want)
		}
		b, _ := json.Marshal(raw)
		var fields map[string]any
		_ = json.Unmarshal(b, &fields)
		if len(fields) != 6 {
			t.Fatal("face must contain no response choices or private hand data", string(b))
		}
	}
	saved := clone(*s)
	if !reflect.DeepEqual(s.Catan.RevealedEvent, saved.Catan.RevealedEvent) {
		t.Fatal("face did not survive save/restore")
	}
	copy := s.Catan.revealedEventView()
	copy.Production = 99
	if s.Catan.revealedEventView().Production != number {
		t.Fatal("public face aliases private state")
	}
}

func TestCatanRevealedEventQueueProductionAndNextRoll(t *testing.T) {
	s := cardEarthquakeFixture(t)
	beginCardEvent(t, s, "earthquake", 6, 0, 0)
	for _, p := range []int{1, 2, 0} {
		assertRevealedEvent(t, s, "earthquake", 6, false)
		helperApply(t, s, p, Action{Type: "catan_earthquake", Edge: p * 2})
	}
	assertRevealedEvent(t, s, "earthquake", 6, true)
	helperReject(t, s, s.Turn, Action{Type: "catan_roll"})
	assertRevealedEvent(t, s, "earthquake", 6, true)
	helperApply(t, s, s.Turn, Action{Type: "catan_end"})
	assertRevealedEvent(t, s, "earthquake", 6, true)
	helperApply(t, s, s.Turn, Action{Type: "catan_roll"})
	if s.Catan.RevealedEvent != nil {
		t.Fatal("ordinary roll retained old event face")
	}
	if _, ok := s.View(-1)["catan"].(map[string]any)["revealedEvent"]; ok {
		t.Fatal("old event exposed on a normal roll")
	}
}

func TestCatanRevealedEventImmediateEffectsAndLegacyQueue(t *testing.T) {
	s := cardEarthquakeFixture(t)
	beginCardEvent(t, s, "beautiful_day", 6, 0, 0)
	if s.Catan.CardEvent != nil {
		t.Fatal("beautiful day must resolve immediately")
	}
	assertRevealedEvent(t, s, "beautiful_day", 6, true)
	s.Phase = "catan_roll"
	beginCardEvent(t, s, "good_neighbors", 6, 0, 0)
	// Fixture production gave the neighbors resources. Mimic a pre-field save.
	if s.Catan.CardEvent == nil {
		t.Fatal("need a pending legacy event")
	}
	s.Catan.RevealedEvent = nil
	before, _ := json.Marshal(s)
	assertRevealedEvent(t, s, "good_neighbors", 6, false)
	after, _ := json.Marshal(s)
	if string(before) != string(after) {
		t.Fatal("view migrated state during read")
	}
	for i := 0; s.Catan.CardEvent != nil && i < 4; i++ {
		s.AutoCatanPending()
	}
	if s.Catan.CardEvent != nil {
		t.Fatal("legacy queue stalled")
	}
	assertRevealedEvent(t, s, "good_neighbors", 6, true)
}

func TestCatanRevealedEventCityResponsesAndAlchemy(t *testing.T) {
	s := ckKnightGraph(t, []int{0, 0, 1, 1, 2, 2})
	g := s.Catan
	s.Phase = "catan_roll"
	g.Tiles[0].Vertices = []int{0, 3, 6}
	for p, v := range []int{0, 3, 6} {
		g.Vertices[v].Owner, g.Vertices[v].Level = p, 2
	}
	g.CitiesKnights.BarbarianPosition = 6
	g.CitiesKnights.RobberStart = -1
	g.CitiesKnights.Knights = []CatanKnight{{Owner: 0, Vertex: 1, Strength: 1, Active: true}}
	beginCardEvent(t, s, "epidemic", 6, 2, 3)
	if s.Phase != "catan_pillage" {
		t.Fatal("need city response", s.Phase)
	}
	assertRevealedEvent(t, s, "epidemic", 6, false)
	restored := clone(*s)
	s = &restored
	for i := 0; s.CatanPendingActor() >= 0 && i < 8; i++ {
		s.AutoCatanPending()
	}
	assertRevealedEvent(t, s, "epidemic", 6, true)
	if s.Catan.RevealedEvent.Red != 2 || s.Catan.RevealedEvent.Face != 3 {
		t.Fatal("independent dice lost")
	}
	s.Phase = "catan_roll"
	s.Turn = 0
	ckProgressGive(t, s, 0, 0)
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{0, 6}})
	assertRevealedEvent(t, s, "epidemic", 6, true)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 0, Tokens: []int{2, 3}})
	if s.Catan.RevealedEvent != nil || sum(s.Catan.Dice) != 5 {
		t.Fatal("Alchemy must clear event and use its own production number")
	}
}

func TestCatanRevealedEventGoldAndRollback(t *testing.T) {
	s := cardEarthquakeFixture(t)
	g := s.Catan
	g.Seafarers = &CatanSeafarers{Pirate: -1}
	g.Tiles[0].Resource = CatanGold
	beginCardEvent(t, s, "beautiful_day", 6, 0, 0)
	if s.Phase != "catan_gold" {
		t.Fatal("need gold response", s.Phase)
	}
	assertRevealedEvent(t, s, "beautiful_day", 6, true)
	for i := 0; s.CatanPendingActor() >= 0 && i < 5; i++ {
		s.AutoCatanPending()
	}
	assertRevealedEvent(t, s, "beautiful_day", 6, true)
	s.Phase = "catan_roll"
	rejectCardEvent(t, s, "missing", 6, 0, 0)
	assertRevealedEvent(t, s, "beautiful_day", 6, true)
	beginCardEvent(t, s, "earthquake", 6, 0, 0)
	for _, p := range []int{1, 2} {
		helperApply(t, s, p, Action{Type: "catan_earthquake", Edge: p * 2})
	}
	// The final response must roll back both the road and the public record if
	// subsequent city processing is invalid.
	s.Catan.CitiesKnights = &CatanCitiesKnights{}
	helperReject(t, s, 0, Action{Type: "catan_earthquake", Edge: 0})
	if q := s.Catan.RevealedEvent; q == nil || q.Kind != "earthquake" || q.ProductionStarted {
		t.Fatal("failed continuation changed the public record", q)
	}
}
