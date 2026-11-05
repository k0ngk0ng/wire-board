package engine

import "testing"

func fighter7OrdinaryWorld(t *testing.T, continuous bool) {
	t.Helper()
	library := sessionLibrary(t)
	var state ChuWorldState
	first, callbacks, source7, last := true, 0, 0, 0
	count := visitChuWorldTest(t, func(row chuWorldOriginalPass) {
		if first && len(row.DriverEntries) != 0 {
			t.Fatal("source7 initial pass gained an unobserved driver entry")
		}
		if first || !continuous {
			state = decodeChuWorldTest(t, row.Before)
		} else {
			begin, outcome, err := BeginWorldFrame(WorldFrameState{Clock: state.Clock, Controllers: state.Controllers, RandomSeed: state.RandomSeed,
				SlowGate: state.SlowGate, ResetEnabled: state.ResetEnabled}, row.Before.Controllers)
			if err != nil || outcome.ResetRequested {
				t.Fatal("source7 frame begin", row.Frame, err)
			}
			state.Clock, state.Controllers, state.RandomSeed = begin.Clock, begin.Controllers, begin.RandomSeed
		}
		first = false
		if o := state.Objects[0x8400]; o.Header.Behavior == 0x12668 && o.Tag == 7 {
			source7++
		}
		next, events, err := StepChuWorldObjects(state, ChuWorldView{StreamBase: 0xff6000}, library)
		if err != nil {
			t.Fatal("source7 original world", row.Frame, err)
		}
		compareChuWorldOrderTest(t, row, events)
		compareChuWorldTest(t, next, decodeChuWorldTest(t, row.After), row.Frame)
		compareChuWorldBytesTest(t, next, row, events)
		state = next
		if continuous {
			state.Palette, err = FinishWorldPalette(state.Palette, library.Arena)
			if err != nil {
				t.Fatal(err)
			}
		}
		callbacks += len(row.Calls)
		last = row.Frame
	}, "chu-world-fighter7")
	if count != 3144 || callbacks != 41797 || last != 4944 || source7 < 3000 || !state.MatchComplete {
		t.Fatal("source7 original world coverage lost", count, callbacks, last, source7, state.MatchComplete)
	}
	t.Logf("source7 ordinary continuous=%v passes=%d callbacks=%d cleared=%v", continuous, count, callbacks, state.MatchComplete)
}

func TestFighter7WorldOriginalPasses(t *testing.T) { fighter7OrdinaryWorld(t, false) }

func TestFighter7WorldSingleInitialState(t *testing.T) { fighter7OrdinaryWorld(t, true) }
