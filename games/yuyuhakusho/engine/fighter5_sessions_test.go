package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Fresh independent source5 sessions are specified before session admission.
func TestFighter5FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"five-five", []uint8{5, 5}, false, false},
		{"five-zero", []uint8{5, 0}, false, false},
		{"zero-five", []uint8{0, 5}, false, false},
		{"five-kurama", []uint8{5, 1}, false, false},
		{"kurama-five", []uint8{1, 5}, false, false},
		{"five-kuwabara", []uint8{5, 2}, false, false},
		{"kuwabara-five", []uint8{2, 5}, false, false},
		{"five-three", []uint8{5, 3}, false, false},
		{"three-five", []uint8{3, 5}, false, false},
		{"five-hiei", []uint8{5, 4}, false, false},
		{"hiei-five", []uint8{4, 5}, false, false},
		{"five-jin", []uint8{5, 6}, false, false},
		{"jin-five", []uint8{6, 5}, false, false},
		{"five-elder", []uint8{5, 8}, false, false},
		{"elder-five", []uint8{8, 5}, false, false},
		{"five-chu", []uint8{5, 9}, false, false},
		{"chu-five", []uint8{9, 5}, false, false},
		{"five-ten", []uint8{5, 10}, false, false},
		{"ten-five", []uint8{10, 5}, false, false},
		{"three", []uint8{5, 0, 3}, false, false},
		{"four", []uint8{5, 0, 3, 9}, false, false},
		{"team", []uint8{5, 0, 5, 10}, true, false},
		{"human-idle", []uint8{5, 9}, false, true},
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
				t.Fatal("source5 fresh creation", err)
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
					t.Fatal("source5 fresh frame", tick, err)
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
						t.Fatal("source5 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source5 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source5 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
