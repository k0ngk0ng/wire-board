package scoreclock_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

type worldLogicNative struct {
	worldClockNative
	Input engine.ControllerNormalizationInput
	End   struct {
		Clock    scoreclock.Clock
		Context  scoreclock.BattleSampleContext
		Flags    scoreclock.BattleWorldFlags
		World    engine.ChuWorldState
		Graphics scoreclock.BattleGraphics
		Frames   int
	}
}

func worldLogicNatives(t *testing.T) []worldLogicNative {
	t.Helper()
	var fixture struct {
		Version int
		Cases   []json.RawMessage
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/world-logic-begin/portable-logic-begin.json.gz"), &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 16 {
		t.Fatal("whole original logical prelude missing")
	}
	var rows []worldLogicNative
	for _, raw := range fixture.Cases {
		var row worldLogicNative
		if err := json.Unmarshal(raw, &row.worldClockNative); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	return rows
}
func TestWorldLogicBeginOriginalPreludeAndNestedVertical(t *testing.T) {
	steps, ios, frames := 0, 0, 0
	for _, row := range worldLogicNatives(t) {
		t.Run(row.ID, func(t *testing.T) {
			device := observedWorldClock(row.worldClockNative)
			graphics, flags, world := row.Graphics, row.Flags, row.World
			cpus, accesses := 0, 0
			got, err := scoreclock.AdvanceBattleWorldLogicBegin(row.Clock, row.Context, &graphics, &flags, &world, row.Input, device, func(e scoreclock.BattleWorldObservation) error {
				if e.Stage == "cpu" {
					if cpus >= len(row.CPU) {
						t.Fatal("extra logical prelude work", e)
					}
					want := row.CPU[cpus]
					if e.Clock != want.Clock || !reflect.DeepEqual(e.Context, want.Context) || e.Flags != want.Flags {
						t.Fatal("original logical prelude work differs", cpus, e, want)
					}
					cpus++
				} else if e.Stage == "io" {
					if accesses >= len(row.IO) || e.IO == nil || *e.IO != row.IO[accesses] {
						t.Fatal("original logical prelude IO differs", accesses, e.IO, row.IO[minimumIndex(accesses, len(row.IO))])
					}
					accesses++
				} else {
					t.Fatal(e.Stage)
				}
				return nil
			}, scoreclock.Limits{MaxOperations: 65536})
			if err != nil || got.Clock != row.End.Clock || !reflect.DeepEqual(got.Context.IRQ, row.End.Context) || flags != row.End.Flags || !reflect.DeepEqual(world, row.End.World) || !reflect.DeepEqual(graphics, row.End.Graphics) || !got.AwaitingObjects || got.Outcome.ResetRequested || !got.Outcome.InputConsumed || cpus != len(row.CPU) || accesses != len(row.IO) || device.auxIndex != len(row.Auxiliary) || device.busIndex != len(row.Bus) || len(got.Frames) != row.End.Frames || int(got.Operations) != cpus {
				t.Fatal("whole original logical prelude differs", err, got, row.End, world, cpus, accesses, device)
			}
			steps += cpus
			ios += accesses
			frames += len(got.Frames)
		})
	}
	if steps != 5935 || ios != 460 || frames != 10 {
		t.Fatal("missing original logical prefix cases", steps, ios, frames)
	}
}
