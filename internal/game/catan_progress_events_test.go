package game

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCatanProgressEventsPublicDrawPlayAndPrivateTransfer(t *testing.T) {
	s := ckEmptyTurn(t)
	ckProgressTop(t, s, 1, 9)
	s.catanDrawProgress(0, 0)
	e := s.Catan.CitiesKnights.ProgressEvents[0]
	if e.Kind != "draw" || e.Card != nil || e.Track != 0 || e.Player != 0 {
		t.Fatal(e)
	}
	s.catanDrawProgress(0, 0)
	e = s.Catan.CitiesKnights.ProgressEvents[1]
	if e.Card == nil || *e.Card != 9 || !strings.Contains(strings.Join(s.Log, "\n"), "印刷术") {
		t.Fatal(e, s.Log)
	}
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 1, Choice: "skip"})
	e = s.Catan.CitiesKnights.ProgressEvents[2]
	if e.Kind != "play" || e.Card == nil || *e.Card != 1 {
		t.Fatal(e)
	}
	ckProgressGive(t, s, 0, 18)
	ckProgressGive(t, s, 1, 7)
	helperApply(t, s, 0, Action{Type: "catan_progress", Card: 18, Target: 1})
	helperApply(t, s, 0, Action{Type: "catan_espionage", Card: 7})
	e = s.Catan.CitiesKnights.ProgressEvents[4]
	if e.Kind != "transfer" || e.Card != nil || e.Track != -1 || e.Player != 0 || e.Other != 1 {
		t.Fatal(e)
	}
	for _, viewer := range []int{-1, 0, 1, 2} {
		k := s.View(viewer)["catan"].(map[string]any)["citiesKnights"].(map[string]any)
		raw, _ := json.Marshal(k["progressEvents"])
		var got []CatanProgressEvent
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, s.Catan.CitiesKnights.ProgressEvents) {
			t.Fatal("event view mismatch", viewer)
		}
		for _, ev := range got {
			if ev.Kind == "transfer" || (ev.Kind == "draw" && ev.ID == 1) {
				if ev.Card != nil {
					t.Fatal("private identity leaked", viewer, ev)
				}
			}
		}
	}
	restored := clone(*s)
	if !reflect.DeepEqual(restored.Catan.CitiesKnights.ProgressEvents, s.Catan.CitiesKnights.ProgressEvents) {
		t.Fatal("events lost during restore")
	}
	ckProgressStock(t, s.Catan)
}

func TestCatanProgressEventsDiscardAndAtomicRejectedPlay(t *testing.T) {
	s := ckEmptyTurn(t)
	ckProgressGive(t, s, 0, 0, 1, 2, 3, 4)
	s.Phase = "catan_progress_end"
	s.Catan.CitiesKnights.Pending = &CatanCityPending{Kind: "progress_discard", Players: []int{0}}
	helperApply(t, s, 0, Action{Type: "catan_progress_discard", Cards: []int{3}})
	events := s.Catan.CitiesKnights.ProgressEvents
	if len(events) != 1 || events[0].Kind != "return" || events[0].Track != -1 || events[0].Card != nil || events[0].Count != 1 {
		t.Fatal(events)
	}
	s.Turn = 0
	s.Phase = "catan_turn"
	// Consumed internally before validating the impossible construction; rollback
	// must remove the animation just as it removes the provisional payment/log.
	helperReject(t, s, 0, Action{Type: "catan_progress", Card: 2, Vertex: -1})
	if !reflect.DeepEqual(s.Catan.CitiesKnights.ProgressEvents, events) {
		t.Fatal("rejected action published an event")
	}
	ckProgressStock(t, s.Catan)
}
func TestCatanProgressEventsBoundedAndLegacy(t *testing.T) {
	s := ckEmptyTurn(t)
	if s.Catan.CitiesKnights.ProgressEventID != 0 {
		t.Fatal("new game has historical events")
	}
	// Isolate retention from rules: many effects may occur in one saved game.
	k := s.Catan.CitiesKnights
	for range 40 {
		k.recordProgress("draw", 0, -1, 0, 1, nil)
	}
	if len(k.ProgressEvents) != 18 || k.ProgressEvents[0].ID != 23 || k.ProgressEventID != 40 {
		t.Fatal(k.ProgressEvents)
	}
	var legacy CatanCitiesKnights
	if err := json.Unmarshal([]byte(`{"players":[]}`), &legacy); err != nil || legacy.ProgressEventID != 0 || len(legacy.ProgressEvents) != 0 {
		t.Fatal("legacy", err)
	}
}
