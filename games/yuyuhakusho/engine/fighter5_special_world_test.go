package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func fighter5SpecialWorld(t *testing.T, continuous bool) {
	t.Helper()
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, draws, head, saved, last := true, 0, 0, 0, 0, 0
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if first || !continuous {
			state = decodeChuWorldTest(t, row.Before)
		} else {
			begin, out, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed, SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || out.ResetRequested {
				t.Fatal("source5 special begin", row.Frame, err)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		var snapshot []byte
		if row.Frame == 2056 || row.Frame == 3179 || row.Frame == 3419 {
			var err error
			snapshot, err = json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
		}
		next, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source5 special world", row.Frame, err)
		}
		compareChuWorldOrderTest(t, row, events)
		compareChuWorldTest(t, next, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, next, row, events)
		var actual []WorldDraw
		for _, event := range events {
			if event.Kind == "draw" {
				if event.Draw == nil {
					t.Fatal("missing callback-time draw snapshot")
				}
				v := *event.Draw
				v.Cache = [4]uint32{}
				actual = append(actual, v)
			}
		}
		expected := append([]WorldDraw(nil), row.Draws...)
		for i := range expected {
			expected[i].Cache = [4]uint32{}
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatal("source5 special ordered draw values differ", row.Frame, len(actual), len(expected))
		}
		if snapshot != nil {
			var restored ChuWorldState
			if err := json.Unmarshal(snapshot, &restored); err != nil {
				t.Fatal(err)
			}
			other, replay, err := StepChuWorldObjects(restored, ChuWorldView{StreamBase: 0xff6000}, library)
			if err != nil || !reflect.DeepEqual(next, other) || !reflect.DeepEqual(events, replay) {
				t.Fatal("source5 special world restore diverged", row.Frame, err)
			}
			saved++
		}
		state = next
		if continuous {
			state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
			if err != nil {
				t.Fatal(err)
			}
		}
		callbacks += len(row.Calls)
		draws += len(row.Draws)
		last = row.Frame
		for _, call := range row.Calls {
			if call.Behavior == 0x1414e {
				head++
			}
		}
	}, "chu-world-fighter5-special")
	if count != 1801 || callbacks != 24007 || draws != 12058 || head != 139 || saved != 3 || last != 3600 {
		t.Fatal("source5 special world coverage lost", count, callbacks, draws, head, saved, last)
	}
	t.Logf("source5 special continuous=%v passes=%d callbacks=%d orderedDraws=%d head=%d restores=%d", continuous, count, callbacks, draws, head, saved)
}

func TestFighter5SpecialWorldOriginalPasses(t *testing.T)     { fighter5SpecialWorld(t, false) }
func TestFighter5SpecialWorldSingleInitialState(t *testing.T) { fighter5SpecialWorld(t, true) }
