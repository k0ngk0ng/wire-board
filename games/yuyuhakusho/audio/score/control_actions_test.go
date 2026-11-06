package score_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

func controlHooks() score.ObservedHooks {
	return score.ObservedHooks{Operation: func(score.Operation) error { return nil }, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }}
}

func controlSeed(t *testing.T) (*score.Catalog, *score.Driver) {
	t.Helper()
	c := catalog(t)
	d, err := score.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Advance(score.Input{SoundID: 0x82}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Advance(score.Input{}); err != nil {
		t.Fatal(err)
	}
	return c, d
}

func TestControlStopDACFlagsKeepsParametersAndPCMSeparate(t *testing.T) {
	c, d := controlSeed(t)
	before := d.Snapshot()
	v := &before.Voices[0]
	v.Active = true
	v.NoteOff = true
	v.Claimed = true
	v.Tie = true
	d, err := score.Restore(c, before)
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.ApplyMusicControlObserved(score.ActionStopMusicDACFlags, controlHooks())
	if err != nil {
		t.Fatal(err)
	}
	want := before
	want.Voices[0].Active = false
	want.Voices[0].NoteOff = false
	want.Voices[0].Claimed = false
	want.Voices[0].Tie = false
	if !reflect.DeepEqual(d.Snapshot(), want) || len(out.Intents) != 0 || out.ClearPrimaryPriority {
		t.Fatal("DAC flags action changed parameters, time, Root or PCM")
	}
}

func TestControlFadeStepByteBoundariesAndNoClockAdvance(t *testing.T) {
	c, d := controlSeed(t)
	before := d.Snapshot()
	for _, test := range []struct {
		slot  int
		level uint8
	}{{1, 126}, {2, 127}, {3, 255}, {4, 128}, {5, 0}, {7, 14}, {8, 15}, {9, 255}} {
		v := &before.Voices[test.slot]
		v.Active = true
		v.NoteOff = false
		v.Claimed = false
		v.Tie = false
		v.BaseLevel = test.level
	}
	before.Voices[5].Claimed = true
	before.Voices[7].VolumeEnvelope = 1
	before.Voices[7].VolumeCursor = param.EnvelopeCursor{Index: 3}
	d, err := score.Restore(c, before)
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.ApplyMusicControlObserved(score.ActionFadeMusicStep, controlHooks())
	if err != nil {
		t.Fatal(err)
	}
	want := before
	for _, test := range []struct {
		slot   int
		level  uint8
		active bool
	}{{1, 127, true}, {2, 128, false}, {3, 0, true}, {4, 129, false}, {5, 1, true}, {7, 15, true}, {8, 16, false}, {9, 0, true}} {
		want.Voices[test.slot].BaseLevel = test.level
		want.Voices[test.slot].Active = test.active
	}
	if !reflect.DeepEqual(d.Snapshot(), want) {
		t.Fatal("fade moved another owner/cursor or used wrong byte/signed threshold")
	}
	psg := []uint8{}
	for _, intent := range out.Intents {
		if intent.Kind == "psg" {
			psg = append(psg, intent.Value)
		}
		if intent.Kind == "dac" || intent.Kind == "dac-pan" || intent.Kind == "queue" || intent.Kind == "ym" && intent.Register == 0x28 {
			t.Fatal("fade emitted a keyoff/restart/queue effect")
		}
	}
	if !reflect.DeepEqual(psg, []uint8{0x9f, 0xf0}) {
		t.Fatalf("expected direct base attenuation without envelope tick: %x", psg)
	}
}

func TestControlResetIsStateOnlyAndRetainsTickCount(t *testing.T) {
	_, d := controlSeed(t)
	before := d.Snapshot()
	out, err := d.ApplyMusicControlObserved(score.ActionResetScore, controlHooks())
	if err != nil {
		t.Fatal(err)
	}
	if d.Snapshot() != (score.State{Ticks: before.Ticks}) || len(out.Intents) != 0 || out.ClearPrimaryPriority {
		t.Fatal("score reset duplicated Root/chip04 or reset its diagnostic clock")
	}
}

func TestControlActionRejectionAndCallbackFailureAreAtomic(t *testing.T) {
	_, d := controlSeed(t)
	before := d.Snapshot()
	if out, err := d.ApplyMusicControlObserved(score.MusicControlAction(255), controlHooks()); err == nil || len(out.Intents) != 0 || d.Snapshot() != before {
		t.Fatal("unknown control action committed")
	}
	failed := false
	sentinel := errors.New("control callback failed")
	hooks := controlHooks()
	hooks.Operation = func(score.Operation) error {
		if failed {
			t.Fatal("callback repeated after failure")
		}
		failed = true
		return sentinel
	}
	if out, err := d.ApplyMusicControlObserved(score.ActionFadeMusicStep, hooks); err == nil || len(out.Intents) != 0 || d.Snapshot() != before {
		t.Fatal("failed control committed")
	}
}
