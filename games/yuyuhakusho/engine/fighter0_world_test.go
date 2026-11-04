package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Original source0 worlds and the full failure list precede integration.
func fighter0WorldOracle(t *testing.T, name string, continuous bool) {
	t.Helper()
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, last, source0 := true, 0, 0, 0
	seen := make(map[uint32]int)
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if !continuous || first {
			state = decodeChuWorldTest(t, row.Before)
		} else {
			begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || outcome.ResetRequested {
				t.Fatal("source0 world begin", row.Frame, err, outcome)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		if object := state.Objects[0x8400]; object.Tag == 0 && object.Header.Behavior == 0x12668 {
			source0++
		}
		got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source0 world pass", row.Frame, err)
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
	if name == "chu-world-fighter0" {
		if count != 3156 || callbacks != 42212 || last != 4957 || source0 < 2500 || !state.MatchComplete {
			t.Fatal("source0 original complete world coverage lost", count, callbacks, last, source0, state.MatchComplete)
		}
	} else {
		if count < 1700 || last != 3600 || source0 < 1000 || seen[0x12c96] == 0 || seen[0x134c2] == 0 || seen[0x1313e] == 0 || seen[0x132de] == 0 {
			t.Fatal("source0 special world missed natural parent/child composition", count, last, source0, seen)
		}
	}
	t.Logf("source0 original %s continuous=%v passes=%d callbacks=%d child kinds=%v", name, continuous, count, callbacks, seen)
}

func TestFighter0WorldOriginalPasses(t *testing.T) {
	for _, name := range []string{"chu-world-fighter0", "chu-world-fighter0-special"} {
		t.Run(name, func(t *testing.T) { fighter0WorldOracle(t, name, false) })
	}
}

func TestFighter0WorldSingleInitialState(t *testing.T) {
	for _, name := range []string{"chu-world-fighter0", "chu-world-fighter0-special"} {
		t.Run(name, func(t *testing.T) { fighter0WorldOracle(t, name, true) })
	}
}

func TestFighter0FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"zero-zero", []uint8{0, 0}, false, false},
		{"zero-chu", []uint8{0, 9}, false, false},
		{"chu-zero", []uint8{9, 0}, false, false},
		{"zero-ten", []uint8{0, 10}, false, false},
		{"ten-zero", []uint8{10, 0}, false, false},
		{"three", []uint8{0, 9, 10}, false, false},
		{"four", []uint8{0, 0, 9, 10}, false, false},
		{"team", []uint8{0, 9, 0, 10}, true, false},
		{"human-idle", []uint8{0, 9}, false, true},
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
				t.Fatal("source0 fresh creation", err)
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
					t.Fatal("source0 fresh frame", tick, err)
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
						t.Fatal("source0 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source0 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source0 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
