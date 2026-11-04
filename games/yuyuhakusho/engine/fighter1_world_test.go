package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Original source1 worlds and the full failure list precede integration.
func fighter1WorldOracle(t *testing.T, name string, continuous bool) {
	t.Helper()
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, last, source1 := true, 0, 0, 0
	seen := make(map[uint32]int)
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if !continuous || first {
			state = decodeChuWorldTest(t, row.Before)
		} else {
			begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || outcome.ResetRequested {
				t.Fatal("source1 world begin", row.Frame, err, outcome)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		if object := state.Objects[0x8400]; object.Tag == 1 && object.Header.Behavior == 0x12668 {
			source1++
		}
		got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source1 world pass", row.Frame, err)
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
	if name == "chu-world-fighter1" {
		if count != 2955 || callbacks != 38770 || last != 4755 || source1 < 2000 || !state.MatchComplete {
			t.Fatal("source1 original complete world coverage lost", count, callbacks, last, source1, state.MatchComplete)
		}
	} else {
		if count < 1700 || last != 3600 || source1 < 1000 || seen[0x12dd4] == 0 || seen[0x12ee8] == 0 || seen[0x1300c] == 0 {
			t.Fatal("source1 special world missed natural parent/child composition", count, last, source1, seen)
		}
	}
	t.Logf("source1 original %s continuous=%v passes=%d callbacks=%d child kinds=%v", name, continuous, count, callbacks, seen)
}

func TestFighter1WorldOriginalPasses(t *testing.T) {
	for _, name := range []string{"chu-world-fighter1", "chu-world-fighter1-special"} {
		t.Run(name, func(t *testing.T) { fighter1WorldOracle(t, name, false) })
	}
}

func TestFighter1WorldSingleInitialState(t *testing.T) {
	for _, name := range []string{"chu-world-fighter1", "chu-world-fighter1-special"} {
		t.Run(name, func(t *testing.T) { fighter1WorldOracle(t, name, true) })
	}
}

func TestFighter1WorldActiveChildSaveRestore(t *testing.T) {
	library := sessionLibrary(t)
	checked := map[uint32]int{}
	visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		state := decodeChuWorldTest(t, row.Before)
		for _, id := range []uint16{0x8500, 0x8600, 0x8700} {
			child := state.Objects[id]
			kind := child.Header.Behavior
			if (kind != 0x12dd4 && kind != 0x12ee8 && kind != 0x1300c) || checked[kind] >= 10 || child.Header.Stage == 0 {
				continue
			}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var restored ChuWorldState
			if err := json.Unmarshal(raw, &restored); err != nil || !reflect.DeepEqual(state, restored) {
				t.Fatal("source1 active child save omitted canonical state", row.Frame, err)
			}
			next, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
			if err != nil {
				t.Fatal("source1 child frame", row.Frame, err)
			}
			resumed, replay, err := StepChuWorldObjects(restored, ChuWorldView{StreamBase: 0xff6000}, library)
			if err != nil || !reflect.DeepEqual(next, resumed) || !reflect.DeepEqual(events, replay) {
				t.Fatal("source1 active child save/resume diverged", row.Frame, err)
			}
			compareChuWorldTest(t, resumed, decodeChuWorldTest(t, row.After), row.Frame)
			compareChuWorldBytesTest(t, resumed, row, replay)
			checked[kind]++
		}
	}, "chu-world-fighter1-special")
	for _, kind := range []uint32{0x12dd4, 0x12ee8, 0x1300c} {
		if checked[kind] != 10 {
			t.Fatal("missing natural source1 active child restore states", kind, checked)
		}
	}
	t.Logf("source1 three natural child kinds save/resume=%v", checked)
}

func TestFighter1FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"one-one", []uint8{1, 1}, false, false},
		{"one-zero", []uint8{1, 0}, false, false},
		{"zero-one", []uint8{0, 1}, false, false},
		{"one-kuwabara", []uint8{1, 2}, false, false},
		{"kuwabara-one", []uint8{2, 1}, false, false},
		{"one-three", []uint8{1, 3}, false, false},
		{"three-one", []uint8{3, 1}, false, false},
		{"one-elder", []uint8{1, 8}, false, false},
		{"elder-one", []uint8{8, 1}, false, false},
		{"one-chu", []uint8{1, 9}, false, false},
		{"chu-one", []uint8{9, 1}, false, false},
		{"one-ten", []uint8{1, 10}, false, false},
		{"ten-one", []uint8{10, 1}, false, false},
		{"three", []uint8{1, 0, 3}, false, false},
		{"four", []uint8{1, 0, 3, 9}, false, false},
		{"team", []uint8{1, 0, 1, 10}, true, false},
		{"human-idle", []uint8{1, 9}, false, true},
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
				t.Fatal("source1 fresh creation", err)
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
					t.Fatal("source1 fresh frame", tick, err)
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
						t.Fatal("source1 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source1 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source1 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
