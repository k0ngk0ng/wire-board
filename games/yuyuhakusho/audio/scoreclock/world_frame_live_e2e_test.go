package scoreclock_test

import (
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/session"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func TestWorldFrameLiveActualAudioBranchAndLateRollback(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	config := session.Config{Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	for _, failure := range []bool{false, true} {
		live, err := session.New(library, config)
		if err != nil {
			t.Fatal(err)
		}
		before := live.Snapshot()
		world := engine.ChuWorldState{Objects: engine.WorldObjects{0x8100: {ObjectID: 0x8100, Header: engine.MatchObjectHeader{Behavior: 0x1edac, Size: 64}}}, PendingSound: 4, Clock: engine.WorldClockState{ControllerCount: 4, AudioPause: 1}}
		if failure {
			world.Palette.Flags = 1
			world.Palette.Status = 128
			world.Palette.Mode = 65535
		}
		var active *engine.ChuWorldState
		advanced := false
		got, events, _, err := live.WorldFrameWithHooks(world, [8]engine.Buttons{}, engine.ChuWorldLibrary{}, session.WorldFrameHooks{
			Owners:    func(_ *session.Runtime, current *engine.ChuWorldState) error { active = current; return nil },
			Normalize: func(_ *session.Runtime, _ engine.WorldFrameOwners, _ engine.MainWorkHook) error { return nil },
			Prefix: func(branch *session.Runtime, _ engine.MainWorkOperation, owners engine.WorldFrameOwners) error {
				if active == nil || owners.Clock != &active.Clock {
					t.Fatal("joint audio used a World shadow")
				}
				active.Clock.AudioPause = 255
				if !advanced {
					advanced = true
					_, err := branch.AdvancePCM(10000, nil)
					return err
				}
				return nil
			},
		})
		active = nil
		if !advanced {
			t.Fatal("actual audio branch did not advance")
		}
		if failure {
			if err == nil || events != nil || !reflect.DeepEqual(got, world) || !reflect.DeepEqual(live.Snapshot(), before) {
				t.Fatal("late World palette failure leaked prefix/audio", err)
			}
		} else {
			if err != nil || got.Clock.AudioPause != 255 || reflect.DeepEqual(live.Snapshot(), before) {
				t.Fatal("live prefix/audio did not commit", err)
			}
		}
	}
}
