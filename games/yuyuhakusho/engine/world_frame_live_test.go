package engine

import (
	"errors"
	"reflect"
	"testing"
)

func TestWorldFrameLiveOwnersAcrossPrefixAndObjects(t *testing.T) {
	initial := ChuWorldState{Objects: WorldObjects{0x8100: {ObjectID: 0x8100, Header: MatchObjectHeader{Behavior: 0x1edac, Size: 64}}}, PendingSound: 4, Clock: WorldClockState{ControllerCount: 4, AudioPause: 1}, RandomSeed: 0x5eedbeef}
	var active *ChuWorldState
	bindings, normalized, sound := 0, 0, 0
	got, _, out, err := StepChuSessionFrameWithHooks(initial, [8]Buttons{}, ChuWorldLibrary{}, ChuWorldHooks{
		Owners: func(world *ChuWorldState) error { bindings++; active = world; return nil },
		Frame: &WorldFrameWorkHooks{
			Normalize: func(owners WorldFrameOwners, work MainWorkHook) error {
				normalized++
				if active == nil || owners.Clock != &active.Clock || owners.Controllers != &active.Controllers || owners.RandomSeed != &active.RandomSeed {
					t.Fatal("frame prefix used detached owner shadows")
				}
				if err := work(MainWorkOperation{Phase: "declared-normalization", CPUCycles: 16}); err != nil {
					return err
				}
				return nil
			},
			Work: func(_ MainWorkOperation, owners WorldFrameOwners) error {
				if active == nil || owners.Clock != &active.Clock {
					t.Fatal("clock owner drift")
				}
				active.Clock.AudioPause = 255
				return nil
			},
		},
		Sound: func(op WorldSoundOperation, owners WorldSoundOwners, apply func() error) error {
			sound++
			if active == nil || owners.Queue != &active.SoundQueue || owners.AudioPause != &active.Clock.AudioPause {
				t.Fatal("object transaction owner drift")
			}
			active.Palette.Flags = 128
			return apply()
		},
	})
	active = nil
	if err != nil || bindings < 3 || normalized != 1 || sound == 0 || !out.InputConsumed || got.Clock.AudioPause != 255 || got.Palette.Flags != 128 || got.PendingSound != 0 || initial.Clock.AudioPause != 1 || initial.Palette.Flags != 0 {
		t.Fatal("live world prefix/object owners differ", err, bindings, normalized, sound, out, got)
	}
}

func TestWorldFrameLiveOwnerAndLateFailureAreAtomic(t *testing.T) {
	sentinel := errors.New("discard shared owner context")
	for _, kind := range []string{"owner", "prefix", "normalize", "late-object", "late-palette"} {
		t.Run(kind, func(t *testing.T) {
			initial := ChuWorldState{Objects: WorldObjects{0x8100: {ObjectID: 0x8100, Header: MatchObjectHeader{Behavior: 0x1edac, Size: 64}}}, PendingSound: 4, Clock: WorldClockState{ControllerCount: 4, AudioPause: 1}}
			if kind == "late-object" {
				initial.Objects[0x9fc0] = WorldObject{ObjectID: 0x9fc0, Header: MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
			}
			if kind == "late-palette" {
				initial.Palette.Flags = 1
				initial.Palette.Status = 128
				initial.Palette.Mode = 65535
			}
			var active *ChuWorldState
			hooks := ChuWorldHooks{Owners: func(world *ChuWorldState) error {
				active = world
				if kind == "owner" {
					world.Clock.AudioPause = 255
					return sentinel
				}
				return nil
			}, Frame: &WorldFrameWorkHooks{Normalize: func(owners WorldFrameOwners, _ MainWorkHook) error {
				if kind == "normalize" {
					owners.Clock.AudioPause = 255
					return sentinel
				}
				return nil
			}, Work: func(_ MainWorkOperation, owners WorldFrameOwners) error {
				owners.Clock.AudioPause = 255
				if kind == "prefix" {
					return sentinel
				}
				return nil
			}}, Sound: func(_ WorldSoundOperation, _ WorldSoundOwners, apply func() error) error {
				if active == nil {
					t.Fatal("missing internal World")
				}
				return apply()
			}}
			got, events, out, err := StepChuSessionFrameWithHooks(initial, [8]Buttons{}, ChuWorldLibrary{}, hooks)
			active = nil
			if err == nil || events != nil || out != (WorldFrameOutcome{}) || !reflect.DeepEqual(got, initial) {
				t.Fatal("failed live frame leaked owners", kind, err, got, out)
			}
		})
	}
}
