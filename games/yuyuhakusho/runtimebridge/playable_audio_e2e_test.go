package runtimebridge

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

// Exercise real independent rules, decoded assets, rendering and rollback.
// The separate observer reads producer requests without replacing an oracle.
func TestPlayableAudioRequestsDoNotChangeMatch(t *testing.T) {
	a := networkAssets(t, 0)
	s, err := NewSession(networkOptions(0, []uint8{3, 9}, false, false), a)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Ready(); err != nil {
		t.Fatal(err)
	}
	plain, err := s.Fork()
	if err != nil {
		t.Fatal(err)
	}
	count, music, finite := 0, false, false
	for tick := 0; tick < 620; tick++ {
		held := [4]engine.Buttons{}
		if tick%40 < 20 {
			held[0] = engine.ButtonRight | engine.ButtonB
		}
		var input [8]engine.Buttons
		copy(input[:4], held[:])
		var want []uint16
		_, _, _, e := engine.StepChuSessionFrameObserved(s.saved.World, input, a.rules, func(op engine.WorldSoundOperation, _ engine.WorldSoundOwners, apply func() error) error {
			if err := apply(); err != nil {
				return err
			}
			if op.Request.Sequence == 0 {
				want = append(want, uint16(op.Request.Code))
			}
			return nil
		})
		if e != nil {
			t.Fatal(tick, e)
		}
		got, e := s.StepControlledWithSounds(held, 0)
		if e != nil {
			t.Fatal(tick, e)
		}
		old, e := plain.StepControlled(held, 0)
		if e != nil {
			t.Fatal(tick, e)
		}
		if !reflect.DeepEqual(got.SoundRequests, want) {
			t.Fatalf("tick %d requests %v want %v", tick, got.SoundRequests, want)
		}
		if !bytes.Equal(got.Pixels, old.Pixels) || !bytes.Equal(performanceSave(t, s), performanceSave(t, plain)) {
			t.Fatal("sound collection changed match", tick)
		}
		for _, code := range got.SoundRequests {
			count++
			music = music || code == 0x82
			finite = finite || code >= 6 && code < 127 || code >= 160 && code <= 171
		}
		if tick == 360 {
			raw, e := s.Save()
			if e != nil {
				t.Fatal(e)
			}
			s, e = RestoreSession(raw, a)
			if e != nil {
				t.Fatal(e)
			}
		}
	}
	if count < 5 || !music || !finite {
		t.Fatal("actual match lacked music/actions", count, music, finite)
	}
	t.Logf("620 real match steps, %d ordered requests, identical rules/saves/pixels", count)
}

func TestPlayableAudioFailurePublishesNothing(t *testing.T) {
	s := performanceSession(t, 0, false)
	s.saved.World.Objects[0x9fc0] = engine.WorldObject{ObjectID: 0x9fc0, Header: engine.MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
	before := performanceSave(t, s)
	frame, err := s.StepControlledWithSounds([4]engine.Buttons{engine.ButtonB}, 0)
	if err == nil || len(frame.SoundRequests) != 0 || !bytes.Equal(before, performanceSave(t, s)) {
		t.Fatal("failed transaction leaked audio/state", err)
	}
}
