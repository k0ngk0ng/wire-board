package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

func fighter4DuoWorld(t *testing.T, continuous bool) {
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
				t.Fatal("source4 duo begin", row.Frame, err)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		if state.Objects[0x9000].Tag != 4 || state.Objects[0x9000].InputChannel != 1 || state.Objects[0x8400].InputChannel != 0 {
			t.Fatal("Duo real source4/slot/port identity changed")
		}
		var snapshot []byte
		if row.Frame == 3479 || row.Frame == 3500 || row.Frame == 3589 || row.Frame == 3600 || row.Frame == 4298 || row.Frame == 4299 || row.Frame == 4379 {
			var err error
			snapshot, err = json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
		}
		next, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source4 duo world", row.Frame, err)
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
			t.Fatal("source4 duo ordered draw values differ", row.Frame, len(actual), len(expected))
		}
		if snapshot != nil {
			var restored ChuWorldState
			if err := json.Unmarshal(snapshot, &restored); err != nil {
				t.Fatal(err)
			}
			other, replay, err := StepChuWorldObjects(restored, ChuWorldView{StreamBase: 0xff6000}, library)
			if err != nil || !reflect.DeepEqual(next, other) || !reflect.DeepEqual(events, replay) {
				t.Fatal("source4 duo world restore diverged", row.Frame, err)
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
			if call.Behavior == 0x13bb6 {
				head++
			}
		}
	}, "chu-world-fighter4-duo")
	if count != 3001 || callbacks != 34236 || draws != 18901 || head != 111 || saved != 7 || last != 5500 {
		t.Fatal("source4 duo world coverage lost", count, callbacks, draws, head, saved, last)
	}
	parent, child := state.Objects[0x9000], state.Objects[0x9100]
	if parent.MovementSource != 0x12578 || parent.Energy>>16 != -1 || parent.Body.ActionKind != 0 || child.Header.Behavior != 0 || state.DragonBankCursor == 0 {
		t.Fatal("Duo natural boost did not restore original movement and clear child", parent.MovementSource, parent.Energy, parent.Body.ActionKind, child.Header, state.DragonBankCursor)
	}
	t.Logf("source4 duo continuous=%v passes=%d callbacks=%d orderedDraws=%d head=%d restores=%d", continuous, count, callbacks, draws, head, saved)
}

func TestFighter4DuoWorldOriginalPasses(t *testing.T)     { fighter4DuoWorld(t, false) }
func TestFighter4DuoWorldSingleInitialState(t *testing.T) { fighter4DuoWorld(t, true) }
