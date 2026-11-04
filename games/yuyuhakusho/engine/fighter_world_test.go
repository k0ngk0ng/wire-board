package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Original mixed-character world capture and the failure list preceded the
// production source10 action/actor/world integration.
func TestFighterWorldSource10OriginalPasses(t *testing.T) {
	library := sessionLibrary(t)
	source10, callbacks := 0, 0
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		state := decodeChuWorldTest(t, row.Before)
		if state.Objects[0x8400].Tag == 10 {
			source10++
		}
		got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatalf("mixed original frame%d: %v", row.Frame, err)
		}
		compareChuWorldOrderTest(t, row, events)
		compareChuWorldTest(t, got, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, got, row, events)
		callbacks += len(row.Calls)
	}, "chu-world-fighter10")
	if count != 2789 || callbacks != 36871 || source10 < 2000 {
		t.Fatal("source10 original world coverage missing", count, callbacks, source10)
	}
}

func TestFighterWorldSource10SingleInitialState(t *testing.T) {
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, last := true, 0, 0
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if first {
			state = decodeChuWorldTest(t, row.Before)
			first = false
		} else {
			begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || outcome.ResetRequested {
				t.Fatal("mixed continuous begin", row.Frame, err, outcome)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		got, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("mixed continuous object pass", row.Frame, err)
		}
		compareChuWorldOrderTest(t, row, events)
		compareChuWorldTest(t, got, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, got, row, events)
		state = got
		state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
		if err != nil {
			t.Fatal(err)
		}
		callbacks += len(row.Calls)
		last = row.Frame
	}, "chu-world-fighter10")
	if count != 2789 || callbacks != 36871 || last != 4590 || !state.MatchComplete {
		t.Fatal("mixed original single-initial match did not reach original clear", count, callbacks, last)
	}
}

func TestFighterSessionFreshMixedMatches(t *testing.T) {
	library := sessionLibrary(t)
	for _, test := range []struct {
		name       string
		characters []uint8
		team       bool
		humanIdle  bool
	}{
		{"ten-ten", []uint8{10, 10}, false, false},
		{"ten-chu", []uint8{10, 9}, false, false},
		{"chu-ten", []uint8{9, 10}, false, false},
		{"three", []uint8{10, 9, 10}, false, false},
		{"four", []uint8{10, 9, 10, 9}, false, false},
		{"team", []uint8{10, 9, 10, 9}, true, false},
		{"human-idle", []uint8{10, 9}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			slots := []uint8{0, 3}
			if len(test.characters) == 3 {
				slots = []uint8{0, 1, 3}
			} else if len(test.characters) == 4 {
				slots = []uint8{0, 1, 2, 3}
			}
			options := ChuSessionOptions{Seed: 0x12345678, Difficulty: 6, TeamBattle: test.team}
			for i, character := range test.characters {
				options.Players = append(options.Players, ChuSessionPlayer{Slot: slots[i], CharacterID: character, Color: uint8(i), Computer: !test.humanIdle || i != 0, HumanChannel: 0})
			}
			state, _, err := NewChuSession(options, library)
			if err != nil {
				t.Fatal("mixed fresh configuration", err)
			}
			state, err = CompleteChuSessionResource(state, DefaultChuSessionResource())
			if err != nil {
				t.Fatal(err)
			}
			var restored ChuWorldState
			resumed, damage, knockout := 0, false, false
			kinds := make(map[uint8]int)
			for tick := 1; tick <= 20000 && !state.MatchComplete; tick++ {
				var events []ChuWorldEvent
				state, events, _, err = StepChuSessionFrame(state, [8]Buttons{}, library)
				if err != nil {
					t.Fatalf("mixed fresh tick%d: %v", tick, err)
				}
				for i, slot := range slots {
					o := state.Objects[0x8400+uint16(slot)*0x400]
					if o.Header.Behavior != 0x12668 {
						continue
					}
					if o.Tag != test.characters[i] {
						t.Fatal("fresh constructor silently replaced selected character")
					}
					if test.humanIdle && i == 0 && o.InputChannel != 0 {
						t.Fatal("idle human was taken over")
					}
					kinds[o.Body.ActionKind]++
					damage = damage || o.Body.Health < 223
					knockout = knockout || o.Body.Health < 0
				}
				if tick > 500 && tick <= 530 {
					var otherEvents []ChuWorldEvent
					restored, otherEvents, _, err = StepChuSessionFrame(restored, [8]Buttons{}, library)
					if err != nil || !reflect.DeepEqual(restored, state) || !reflect.DeepEqual(otherEvents, events) {
						t.Fatal("mixed fresh mid-match restore diverged", tick, err)
					}
					resumed++
				}
				if tick == 500 {
					encoded, encodeErr := json.Marshal(state)
					if encodeErr != nil || json.Unmarshal(encoded, &restored) != nil {
						t.Fatal("mixed fresh save failed", encodeErr)
					}
				}
			}
			if !state.MatchComplete || state.Match.WinnerCode == 0 || !damage || !knockout || resumed != 30 {
				t.Fatal("fresh mixed match did not naturally finish", state.MatchComplete, state.Match.WinnerCode, damage, knockout, resumed)
			}
			t.Logf("mixed fresh %s winner=%d seen actions=%v", test.name, state.Match.WinnerCode, kinds)
		})
	}
}
