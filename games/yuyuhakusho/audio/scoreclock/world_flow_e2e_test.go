package scoreclock_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
)

func TestWorldFlowOriginalPrefixWrapperOneClockAndBudget(t *testing.T) {
	prefixes := map[string]battleWorldNative{}
	for _, p := range battleWorldNatives(t) {
		prefixes[p.ID] = p
	}
	cases := 0
	for _, row := range worldClockNatives(t) {
		if !strings.HasPrefix(row.ID, "natural-") {
			continue
		}
		t.Run(row.ID, func(t *testing.T) {
			prefix, ok := prefixes[row.ID]
			if !ok || prefix.End.Clock != row.Clock || !reflect.DeepEqual(prefix.End.Context, row.Context.IRQ) {
				t.Fatal("frozen prefix/wrapper boundaries do not join")
			}
			joined := row
			joined.Frontier, joined.Held, joined.Blocked = prefix.Frontier, prefix.Held, prefix.Blocked
			joined.Auxiliary = nil
			joined.Bus = nil
			for _, a := range prefix.Auxiliary {
				joined.Auxiliary = append(joined.Auxiliary, struct{ Target, Delay uint64 }{a.Target, a.Delay})
			}
			joined.Auxiliary = append(joined.Auxiliary, row.Auxiliary...)
			for _, b := range prefix.Bus {
				joined.Bus = append(joined.Bus, struct {
					At            uint64
					Asserted      bool
					Delay         uint64
					HeldBefore    bool
					FrontierAfter uint64
				}{b.At, b.Asserted, b.Delay, b.GrantedBefore, b.FrontierAfter})
			}
			joined.Bus = append(joined.Bus, row.Bus...)
			device := observedWorldClock(joined)
			world, graphics, flags := row.World, row.Graphics, prefix.Flags
			world.Clock.AudioPause = prefix.Pause
			graphics.CachedDisplay = prefix.Context.Video.DisplayEnabled
			cpus, ios := 0, 0
			got, err := scoreclock.AdvanceBattleWorldIRQToLogic(prefix.Clock, scoreclock.BattleMainContext{IRQ: prefix.Context}, scoreclock.BattleWorldInput{DisplayReady: prefix.DisplayReady}, &graphics, &flags, &world, device, func(e scoreclock.BattleWorldObservation) error {
				if e.Stage == "cpu" {
					if cpus < len(prefix.CPU) {
						want := prefix.CPU[cpus]
						if e.Clock != want.Clock || !reflect.DeepEqual(e.Context, want.Context) || e.Flags != want.Flags {
							t.Fatal("combined original prefix differs", cpus, e, want)
						}
					} else {
						index := cpus - len(prefix.CPU)
						if index >= len(row.CPU) {
							t.Fatal("extra combined work")
						}
						want := row.CPU[index]
						if e.Clock != want.Clock || !reflect.DeepEqual(e.Context, want.Context) || e.Flags != want.Flags {
							t.Fatal("combined original wrapper differs", index, e, want)
						}
					}
					cpus++
				} else if e.Stage == "io" {
					var want scoreclock.BattleSampleIO
					if ios < len(prefix.IO) {
						want = prefix.IO[ios]
					} else {
						index := ios - len(prefix.IO)
						if index >= len(row.IO) {
							t.Fatal("extra combined IO")
						}
						want = row.IO[index]
					}
					if e.IO == nil || *e.IO != want {
						t.Fatal("combined original IO differs", ios, e.IO, want)
					}
					ios++
				} else {
					t.Fatal("unknown combined stage", e.Stage)
				}
				return nil
			}, scoreclock.Limits{MaxOperations: 65536})
			if err != nil || got.Prefix.Clock != prefix.End.Clock || got.Dispatch.Clock != row.End.Clock || !reflect.DeepEqual(got.Dispatch.Context.IRQ, row.End.Context) || flags != row.End.Flags || cpus != len(prefix.CPU)+len(row.CPU) || ios != len(prefix.IO)+len(row.IO) || int(got.Dispatch.Operations) != cpus || device.auxIndex != len(joined.Auxiliary) || device.busIndex != len(joined.Bus) || len(got.Dispatch.Frames) != row.End.Frames {
				t.Fatal("whole original World IRQ flow differs", err, got, row.End, cpus, ios)
			}
			cases++
		})
	}
	if cases != 11 {
		t.Fatal("missing whole natural World IRQ flows", cases)
	}
}

func TestWorldFlowActualAudioAndBudgetAfterPrefixRollback(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	prefix := battleWorldNatives(t)[0]
	row := worldClockNatives(t)[0]
	live, err := session.New(library, config)
	if err != nil {
		t.Fatal(err)
	}
	start := prefix.Context.FrameOrigin + uint64(prefix.Context.Video.Line)*3420
	if _, err := live.AdvancePCM(start, nil); err != nil {
		t.Fatal(err)
	}
	initial := live.Snapshot()
	direct, err := session.Restore(library, config, initial)
	if err != nil {
		t.Fatal(err)
	}
	wantGraphics, wantFlags, wantWorld := row.Graphics, prefix.Flags, row.World
	wantGraphics.CachedDisplay = prefix.Context.Video.DisplayEnabled
	physical := scoreclock.BattleWorldInput{DisplayReady: prefix.DisplayReady}
	wantWorld.Clock.AudioPause = prefix.Pause
	wantPrefix, err := direct.BeginBattleWorldIRQ(prefix.Clock, prefix.Context, physical, &wantFlags, &wantWorld.Clock.AudioPause, scoreclock.Limits{MaxOperations: 65536})
	if err != nil {
		t.Fatal(err)
	}
	// The prefix's real cache write is included by the combined API. In this
	// separate legacy prefix call, transfer that known borrowed latch explicitly.
	if physical.DisplayReady {
		wantGraphics.CachedDisplay = false
	}
	wantDispatch, err := direct.PrepareBattleWorldLogic(wantPrefix.Clock, scoreclock.BattleMainContext{IRQ: wantPrefix.Context}, &wantGraphics, &wantFlags, &wantWorld, scoreclock.Limits{MaxOperations: 65536 - wantPrefix.Operations})
	if err != nil {
		t.Fatal(err)
	}
	graphics, flags, world := row.Graphics, prefix.Flags, row.World
	graphics.CachedDisplay = prefix.Context.Video.DisplayEnabled
	world.Clock.AudioPause = prefix.Pause
	beforeGraphics, beforeFlags, beforeWorld := graphics, flags, world
	_, err = live.PrepareBattleWorldIRQLogic(prefix.Clock, scoreclock.BattleMainContext{IRQ: prefix.Context}, physical, &graphics, &flags, &world, scoreclock.Limits{MaxOperations: wantPrefix.Operations + 1})
	if !errors.Is(err, scoreclock.ErrBudget) || !reflect.DeepEqual(live.Snapshot(), initial) || !reflect.DeepEqual(graphics, beforeGraphics) || flags != beforeFlags || !reflect.DeepEqual(world, beforeWorld) {
		t.Fatal("combined prefix budget leaked real owners", err)
	}
	got, err := live.PrepareBattleWorldIRQLogic(prefix.Clock, scoreclock.BattleMainContext{IRQ: prefix.Context}, physical, &graphics, &flags, &world, scoreclock.Limits{MaxOperations: 65536})
	if err != nil || got.Prefix.Clock != wantPrefix.Clock || got.Dispatch.Clock != wantDispatch.Clock || got.Dispatch.Operations != wantPrefix.Operations+wantDispatch.Operations || !reflect.DeepEqual(got.Dispatch.Context, wantDispatch.Context) || !reflect.DeepEqual(got.Dispatch.Frames, wantDispatch.Frames) || !reflect.DeepEqual(live.Snapshot(), direct.Snapshot()) || !reflect.DeepEqual(graphics, wantGraphics) || flags != wantFlags || !reflect.DeepEqual(world, wantWorld) {
		t.Fatal("one-budget real World IRQ flow differs", err, got, wantPrefix, wantDispatch)
	}
}
