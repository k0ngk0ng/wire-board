package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Original source8 worlds and the full failure list precede integration.
func fighter8WorldOracle(t *testing.T, name string, continuous bool) {
	t.Helper()
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, last, source8 := true, 0, 0, 0
	seen := make(map[uint32]int)
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if !continuous || first {
			state = decodeChuWorldTest(t, row.Before)
		} else {
			begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || outcome.ResetRequested {
				t.Fatal("source8 world begin", row.Frame, err, outcome)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		if object := state.Objects[0x8400]; object.Tag == 8 && object.Header.Behavior == 0x12668 {
			source8++
		}
		got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source8 world pass", row.Frame, err)
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
	if name == "chu-world-fighter8" {
		if count != 3154 || callbacks != 41301 || last != 4954 || source8 < 2000 || !state.MatchComplete {
			t.Fatal("source8 original complete world coverage lost", count, callbacks, last, source8, state.MatchComplete)
		}
	} else {
		if count < 1700 || last != 3600 || source8 < 1000 || seen[0x147ae] == 0 {
			t.Fatal("source8 special world missed natural parent/child composition", count, last, source8, seen)
		}
	}
	t.Logf("source8 original %s continuous=%v passes=%d callbacks=%d child kinds=%v", name, continuous, count, callbacks, seen)
}

func TestFighter8WorldOriginalPasses(t *testing.T) {
	for _, name := range []string{"chu-world-fighter8", "chu-world-fighter8-special"} {
		t.Run(name, func(t *testing.T) { fighter8WorldOracle(t, name, false) })
	}
}

func TestFighter8WorldSingleInitialState(t *testing.T) {
	for _, name := range []string{"chu-world-fighter8", "chu-world-fighter8-special"} {
		t.Run(name, func(t *testing.T) { fighter8WorldOracle(t, name, true) })
	}
}

func TestFighter8WorldActiveChildSaveRestore(t *testing.T) {
	library := sessionLibrary(t)
	checked := 0
	visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		state := decodeChuWorldTest(t, row.Before)
		child := state.Objects[0x8500]
		if checked >= 30 || child.Header.Behavior != 0x147ae || child.Header.Stage == 0 {
			return
		}
		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		var restored ChuWorldState
		if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(state, restored) {
			t.Fatal("source8 active child save omitted canonical state", row.Frame, err)
		}
		next, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source8 child frame", row.Frame, err)
		}
		resumed, replay, err := StepChuWorldObjects(restored, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil || !reflect.DeepEqual(next, resumed) || !reflect.DeepEqual(events, replay) {
			t.Fatal("source8 active child save/resume diverged", row.Frame, err)
		}
		compareChuWorldTest(t, resumed, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, resumed, row, replay)
		checked++
	}, "chu-world-fighter8-special")
	if checked < 30 {
		t.Fatal("missing natural source8 active child restore states", checked)
	}
	t.Logf("source8 natural active child save/resume states=%d", checked)
}

func TestFighter8FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"eight-eight", []uint8{8, 8}, false, false},
		{"eight-zero", []uint8{8, 0}, false, false},
		{"zero-eight", []uint8{0, 8}, false, false},
		{"eight-kuwabara", []uint8{8, 2}, false, false},
		{"kuwabara-eight", []uint8{2, 8}, false, false},
		{"eight-three", []uint8{8, 3}, false, false},
		{"three-eight", []uint8{3, 8}, false, false},
		{"eight-chu", []uint8{8, 9}, false, false},
		{"chu-eight", []uint8{9, 8}, false, false},
		{"eight-ten", []uint8{8, 10}, false, false},
		{"ten-eight", []uint8{10, 8}, false, false},
		{"three", []uint8{8, 0, 3}, false, false},
		{"four", []uint8{8, 0, 3, 9}, false, false},
		{"team", []uint8{8, 0, 8, 10}, true, false},
		{"human-idle", []uint8{8, 9}, false, true},
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
				t.Fatal("source8 fresh creation", err)
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
					t.Fatal("source8 fresh frame", tick, err)
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
						t.Fatal("source8 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source8 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source8 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
