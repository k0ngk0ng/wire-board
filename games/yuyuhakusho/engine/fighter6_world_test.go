package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Original source6 worlds and the full failure list precede integration.
func fighter6WorldOracle(t *testing.T, name string, continuous bool) {
	t.Helper()
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, last, source6 := true, 0, 0, 0
	seen := make(map[uint32]int)
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if !continuous || first {
			state = decodeChuWorldTest(t, row.Before)
		} else {
			begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || outcome.ResetRequested {
				t.Fatal("source6 world begin", row.Frame, err, outcome)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		if object := state.Objects[0x8400]; object.Tag == 6 && object.Header.Behavior == 0x12668 {
			source6++
		}
		got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source6 world pass", row.Frame, err)
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
	if name == "chu-world-fighter6" {
		if count != 2685 || callbacks != 35242 || last != 4485 || source6 < 2000 || !state.MatchComplete {
			t.Fatal("source6 original complete world coverage lost", count, callbacks, last, source6, state.MatchComplete)
		}
	} else if name == "chu-world-fighter6-ai" {
		if count != 2280 || callbacks != 26366 || last != 4813 || !state.MatchComplete {
			t.Fatal("source6 true AI world coverage lost", count, callbacks, last)
		}
	} else {
		if count != 2401 || last != 4200 || source6 < 1000 {
			t.Fatal("source6 special world missed natural parent/child composition", count, last, source6, seen)
		}
	}
	t.Logf("source6 original %s continuous=%v passes=%d callbacks=%d child kinds=%v", name, continuous, count, callbacks, seen)
}

func TestFighter6WorldOriginalPasses(t *testing.T) {
	for _, name := range []string{"chu-world-fighter6", "chu-world-fighter6-special", "chu-world-fighter6-ai"} {
		t.Run(name, func(t *testing.T) { fighter6WorldOracle(t, name, false) })
	}
}

func TestFighter6WorldSingleInitialState(t *testing.T) {
	for _, name := range []string{"chu-world-fighter6", "chu-world-fighter6-special", "chu-world-fighter6-ai"} {
		t.Run(name, func(t *testing.T) { fighter6WorldOracle(t, name, true) })
	}
}

func TestFighter6WorldCounterSaveRestore(t *testing.T) {
	library := sessionLibrary(t)
	checked := map[string]int{}
	visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		state := decodeChuWorldTest(t, row.Before)
		fighter := state.Objects[0x8400]
		group := ""
		if fighter.Body.ActionKind == 0x38 && fighter.Body.Pose == 0xaa && fighter.StunTicks < 255 {
			group = "active"
		}
		if fighter.Body.ActionKind == 0x38 && fighter.StunTicks >= 255 {
			group = "outro"
		}
		if row.Frame > 3760 && fighter.Body.ActionKind == 12 && fighter.Body.Pose == 0x2b {
			group = "recovery"
		}
		if group == "" || checked[group] >= 10 {
			return
		}
		raw, err := json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		var resumed ChuWorldState
		if err := json.Unmarshal(raw, &resumed); err != nil || !reflect.DeepEqual(state, resumed) {
			t.Fatal("source6 counter save", row.Frame, err)
		}
		next, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source6 counter world", row.Frame, err)
		}
		other, replay, err := StepChuWorldObjects(resumed, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil || !reflect.DeepEqual(next, other) || !reflect.DeepEqual(events, replay) {
			t.Fatal("source6 counter resume", row.Frame, err)
		}
		compareChuWorldTest(t, other, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, other, row, replay)
		checked[group]++
	}, "chu-world-fighter6-special")
	for _, group := range []string{"active", "outro", "recovery"} {
		if checked[group] != 10 {
			t.Fatal("missing source6 counter restore", group, checked)
		}
	}
	t.Logf("source6 counter save/resume states=%v", checked)
}

func TestFighter6PracticeWaitBoundary(t *testing.T) {
	library := sessionLibrary(t)
	checked := false
	visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if checked {
			return
		}
		state := decodeChuWorldTest(t, row.Before)
		main := state.Objects[0x8000]
		if main.Header.Behavior != 0xa01e || main.Header.Stage != 32 {
			t.Fatal("not original practice container")
		}
		before, _ := json.Marshal(state)
		cursor := uint16(0)
		events, err := stepChuWorldObject(&state, main, ChuWorldView{}, library, &cursor)
		after, _ := json.Marshal(state)
		if err != nil || len(events) != 0 || string(before) != string(after) {
			t.Fatal("byte-identical original practice wait changed state", err)
		}
		checked = true
	}, "chu-world-fighter6-ai")
	if !checked {
		t.Fatal("missing actual practice world")
	}
}

func TestFighter6FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"six-six", []uint8{6, 6}, false, false},
		{"six-zero", []uint8{6, 0}, false, false},
		{"zero-six", []uint8{0, 6}, false, false},
		{"six-kurama", []uint8{6, 1}, false, false},
		{"kurama-six", []uint8{1, 6}, false, false},
		{"six-kuwabara", []uint8{6, 2}, false, false},
		{"kuwabara-six", []uint8{2, 6}, false, false},
		{"six-three", []uint8{6, 3}, false, false},
		{"three-six", []uint8{3, 6}, false, false},
		{"six-elder", []uint8{6, 8}, false, false},
		{"elder-six", []uint8{8, 6}, false, false},
		{"six-chu", []uint8{6, 9}, false, false},
		{"chu-six", []uint8{9, 6}, false, false},
		{"six-ten", []uint8{6, 10}, false, false},
		{"ten-six", []uint8{10, 6}, false, false},
		{"three", []uint8{6, 0, 3}, false, false},
		{"four", []uint8{6, 0, 3, 9}, false, false},
		{"team", []uint8{6, 0, 6, 10}, true, false},
		{"human-idle", []uint8{6, 9}, false, true},
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
				t.Fatal("source6 fresh creation", err)
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
					t.Fatal("source6 fresh frame", tick, err)
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
						t.Fatal("source6 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source6 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source6 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
