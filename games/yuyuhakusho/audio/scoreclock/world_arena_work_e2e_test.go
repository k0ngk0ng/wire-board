package scoreclock_test

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
	"reflect"
	"testing"
)

// The threshold is deliberately synthetic. Native tests independently prove
// every arena work boundary; this tests actual audio composition/rollback only.
func TestArenaWorkWorldAudioInteriorTransaction(t *testing.T) {
	resources := observedEnvelopeLibrary(t)
	_, priorities := soundIRQNatives(t)
	rules, err := engine.DecodeChuWorldRules(parserClockUnpack(t, "../../data/chu-world-rules.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	for _, failure := range []string{"none", "late-object", "late-palette"} {
		t.Run(failure, func(t *testing.T) {
			live, err := session.New(resources, cfg)
			if err != nil {
				t.Fatal(err)
			}
			initial := live.Snapshot()
			direct, err := session.Restore(resources, cfg, initial)
			if err != nil {
				t.Fatal(err)
			}
			world := engine.ChuWorldState{Objects: engine.WorldObjects{0x8180: {ObjectID: 0x8180, Header: engine.MatchObjectHeader{Behavior: 0x20918, Size: 64, Stage: 2}}}, Camera: engine.DefaultArenaCamera{X: 864, Y: 16}, SoundQueue: [4]uint8{4}}
			if failure == "late-object" {
				world.Objects[0x9fc0] = engine.WorldObject{ObjectID: 0x9fc0, Header: engine.MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
			}
			localRules := rules
			if failure == "late-palette" {
				world.Palette.Flags = 1
				world.Palette.Mode = 0xffff
				localRules.Arena.FadeAdjust = nil
			}
			before, _ := json.Marshal(world)
			clock := scoreclock.Clock{Cycle: 10000, Refresh: 10896}
			var entry scoreclock.Clock
			calls, work := 0, 0
			var actual scoreclock.SoundIRQResult
			got, events, _, err := live.WorldFrameWithHooks(world, [8]engine.Buttons{}, localRules, session.WorldFrameHooks{Work: func(branch *session.Runtime, op engine.WorldWorkOperation, owners engine.WorldSoundOwners) error {
				work++
				if calls == 0 && clock.Cycle >= 20000 {
					calls++
					entry = clock
					var err error
					actual, err = branch.Interrupt(clock, owners.Queue, owners.AudioPause, priorities, scoreclock.Limits{MaxOperations: 65536})
					if err != nil {
						return err
					}
					clock = actual.Clock
				}
				cycle, refresh, err := op.Work.Advance(clock.Cycle, clock.Refresh)
				clock = scoreclock.Clock{Cycle: cycle, Refresh: refresh}
				return err
			}})
			if calls != 1 || work < 500 || entry.Cycle < 20000 {
				t.Fatal("did not enter real audio inside arena", calls, work, entry)
			}
			if failure != "none" {
				if err == nil || events != nil || !reflect.DeepEqual(got, world) || !reflect.DeepEqual(live.Snapshot(), initial) {
					t.Fatal("partial world/audio work commit", failure, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				queue := world.SoundQueue
				pause := uint8(0)
				expected, err := direct.Interrupt(entry, &queue, &pause, priorities, scoreclock.Limits{MaxOperations: 65536})
				if err != nil || !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(live.Snapshot(), direct.Snapshot()) || got.SoundQueue != queue || got.Clock.AudioPause != pause {
					t.Fatal("interior audio differs", err)
				}
			}
			after, _ := json.Marshal(world)
			if string(before) != string(after) {
				t.Fatal("caller world mutated")
			}
		})
	}
}
