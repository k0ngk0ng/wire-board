package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Original source2 worlds and the full failure list precede integration.
func fighter2WorldOracle(t *testing.T, name string, continuous bool) {
	t.Helper()
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, last, source2 := true, 0, 0, 0
	seen := make(map[uint32]int)
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if !continuous || first {
			state = decodeChuWorldTest(t, row.Before)
		} else {
			begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || outcome.ResetRequested {
				t.Fatal("source2 world begin", row.Frame, err, outcome)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		if object := state.Objects[0x8400]; object.Tag == 2 && object.Header.Behavior == 0x12668 {
			source2++
		}
		got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source2 world pass", row.Frame, err)
		}
		compareChuWorldOrderTest(t, row, events)
		compareChuWorldTest(t, got, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, got, row, events)
		state = got
		if continuous {
			state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
			if err != nil {
				t.Fatal(err)
			}
		}
		callbacks += len(row.Calls)
		last = row.Frame
		for _, call := range row.Calls {
			seen[call.Behavior]++
		}
	}, name)
	if name == "chu-world-fighter2" {
		if count != 3165 || callbacks != 42007 || last != 4965 || source2 < 2000 || !state.MatchComplete {
			t.Fatal("source2 original complete world coverage lost", count, callbacks, last, source2, state.MatchComplete)
		}
	} else {
		if count < 1700 || last != 3600 || source2 < 1000 || seen[0x12dd4] == 0 || seen[0x12c96] == 0 {
			t.Fatal("source2 special world missed natural parent/child composition", count, last, source2, seen)
		}
	}
	t.Logf("source2 original %s continuous=%v passes=%d callbacks=%d child kinds=%v", name, continuous, count, callbacks, seen)
}

func TestFighter2WorldOriginalPasses(t *testing.T) {
	for _, name := range []string{"chu-world-fighter2", "chu-world-fighter2-special"} {
		t.Run(name, func(t *testing.T) { fighter2WorldOracle(t, name, false) })
	}
}

func TestFighter2WorldSingleInitialState(t *testing.T) {
	for _, name := range []string{"chu-world-fighter2", "chu-world-fighter2-special"} {
		t.Run(name, func(t *testing.T) { fighter2WorldOracle(t, name, true) })
	}
}

func TestFighter2WorldHitStopSaveRestore(t *testing.T) {
	library := sessionLibrary(t)
	checked := 0
	visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		state := decodeChuWorldTest(t, row.Before)
		child := state.Objects[0x8500]
		if child.Header.Behavior != 0x12dd4 || child.HitStopTicks == 0 {
			return
		}
		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		var restored ChuWorldState
		if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(state, restored) {
			t.Fatal("source2 active child save omitted canonical state", row.Frame, err)
		}
		next, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source2 child hit-stop frame", row.Frame, err)
		}
		resumed, replay, err := StepChuWorldObjects(restored, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil || !reflect.DeepEqual(next, resumed) || !reflect.DeepEqual(events, replay) {
			t.Fatal("source2 hit-stop save/resume diverged", row.Frame, err)
		}
		compareChuWorldTest(t, resumed, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, resumed, row, replay)
		checked++
	}, "chu-world-fighter2-special")
	if checked < 30 {
		t.Fatal("missing natural source2 child hit-stop restore states", checked)
	}
	t.Logf("source2 natural active child hit-stop save/resume states=%d", checked)
}

func TestFighter2FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"two-two", []uint8{2, 2}, false, false},
		{"two-zero", []uint8{2, 0}, false, false},
		{"zero-two", []uint8{0, 2}, false, false},
		{"two-three", []uint8{2, 3}, false, false},
		{"three-two", []uint8{3, 2}, false, false},
		{"two-chu", []uint8{2, 9}, false, false},
		{"chu-two", []uint8{9, 2}, false, false},
		{"two-ten", []uint8{2, 10}, false, false},
		{"ten-two", []uint8{10, 2}, false, false},
		{"three", []uint8{2, 0, 3}, false, false},
		{"four", []uint8{2, 0, 3, 9}, false, false},
		{"team", []uint8{2, 0, 2, 10}, true, false},
		{"human-idle", []uint8{2, 9}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			slots := []uint8{0, 3}
			if len(test.characters) == 3 {
				slots = []uint8{0, 1, 3}
			}
			if len(test.characters) == 4 {
				slots = []uint8{0, 1, 2, 3}
			}
			options := ChuSessionOptions{Seed: 0x12345678, Difficulty: 6, TeamBattle: test.team}
			for i, character := range test.characters {
				options.Players = append(options.Players, ChuSessionPlayer{Slot: slots[i], CharacterID: character, Color: uint8(i), Computer: !test.human || i != 0})
			}
			state, _, err := NewChuSession(options, library)
			if err != nil {
				t.Fatal("source2 fresh creation", err)
			}
			state, err = CompleteChuSessionResource(state, DefaultChuSessionResource())
			if err != nil {
				t.Fatal(err)
			}
			var restored ChuWorldState
			resumed, damage, ko := 0, false, false
			for tick := 1; tick <= 20000 && !state.MatchComplete; tick++ {
				var events []ChuWorldEvent
				state, events, _, err = StepChuSessionFrame(state, [8]Buttons{}, library)
				if err != nil {
					t.Fatal("source2 fresh frame", tick, err)
				}
				for i, slot := range slots {
					object := state.Objects[0x8400+uint16(slot)*0x400]
					if object.Header.Behavior != 0x12668 {
						continue
					}
					if object.Tag != test.characters[i] {
						t.Fatal("selected source silently changed")
					}
					if test.human && i == 0 && object.InputChannel != 0 {
						t.Fatal("idle human taken over")
					}
					damage = damage || object.Body.Health < 223
					ko = ko || object.Body.Health < 0
				}
				if tick > 500 && tick <= 530 {
					var other []ChuWorldEvent
					restored, other, _, err = StepChuSessionFrame(restored, [8]Buttons{}, library)
					if err != nil || !reflect.DeepEqual(state, restored) || !reflect.DeepEqual(events, other) {
						t.Fatal("source2 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source2 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source2 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
