package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Fresh independent source7 sessions are specified before session admission.
func TestFighter7FreshSessions(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name        string
		characters  []uint8
		team, human bool
	}{
		{"seven-seven", []uint8{7, 7}, false, false},
		{"seven-zero", []uint8{7, 0}, false, false},
		{"zero-seven", []uint8{0, 7}, false, false},
		{"seven-kurama", []uint8{7, 1}, false, false},
		{"kurama-seven", []uint8{1, 7}, false, false},
		{"seven-kuwabara", []uint8{7, 2}, false, false},
		{"kuwabara-seven", []uint8{2, 7}, false, false},
		{"seven-three", []uint8{7, 3}, false, false},
		{"three-seven", []uint8{3, 7}, false, false},
		{"seven-hiei", []uint8{7, 4}, false, false},
		{"hiei-seven", []uint8{4, 7}, false, false},
		{"seven-sensui", []uint8{7, 5}, false, false},
		{"sensui-seven", []uint8{5, 7}, false, false},
		{"seven-jin", []uint8{7, 6}, false, false},
		{"jin-seven", []uint8{6, 7}, false, false},
		{"seven-elder", []uint8{7, 8}, false, false},
		{"elder-seven", []uint8{8, 7}, false, false},
		{"seven-chu", []uint8{7, 9}, false, false},
		{"chu-seven", []uint8{9, 7}, false, false},
		{"seven-ten", []uint8{7, 10}, false, false},
		{"ten-seven", []uint8{10, 7}, false, false},
		{"three", []uint8{7, 0, 3}, false, false},
		{"four", []uint8{7, 0, 3, 9}, false, false},
		{"team", []uint8{7, 0, 7, 10}, true, false},
		{"human-idle", []uint8{7, 9}, false, true},
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
				t.Fatal("source7 fresh creation", err)
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
					t.Fatal("source7 fresh frame", tick, err)
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
						t.Fatal("source7 restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					raw, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(raw, &restored) != nil {
						t.Fatal("source7 save", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !ko || resumed != 30 {
				t.Fatal("source7 match did not complete naturally", state.MatchComplete, state.Match.WinnerCode, damage, ko, resumed)
			}
		})
	}
}
