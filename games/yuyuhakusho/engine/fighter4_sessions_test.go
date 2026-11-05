package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Fresh independent source4 sessions are specified before session admission.
func TestFighter4FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"four-four", []uint8{4, 4}, false, false},
		{"four-zero", []uint8{4, 0}, false, false},
		{"zero-four", []uint8{0, 4}, false, false},
		{"four-kurama", []uint8{4, 1}, false, false},
		{"kurama-four", []uint8{1, 4}, false, false},
		{"four-kuwabara", []uint8{4, 2}, false, false},
		{"kuwabara-four", []uint8{2, 4}, false, false},
		{"four-three", []uint8{4, 3}, false, false},
		{"three-four", []uint8{3, 4}, false, false},
		{"four-jin", []uint8{4, 6}, false, false},
		{"jin-four", []uint8{6, 4}, false, false},
		{"four-elder", []uint8{4, 8}, false, false},
		{"elder-four", []uint8{8, 4}, false, false},
		{"four-chu", []uint8{4, 9}, false, false},
		{"chu-four", []uint8{9, 4}, false, false},
		{"four-ten", []uint8{4, 10}, false, false},
		{"ten-four", []uint8{10, 4}, false, false},
		{"three", []uint8{4, 0, 3}, false, false},
		{"four", []uint8{4, 0, 3, 9}, false, false},
		{"team", []uint8{4, 0, 4, 10}, true, false},
		{"human-idle", []uint8{4, 9}, false, true},
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
				t.Fatal("source4 fresh creation", err)
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
					t.Fatal("source4 fresh frame", tick, err)
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
						t.Fatal("source4 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source4 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source4 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
