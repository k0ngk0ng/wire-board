package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrObservedPCMSequence = errors.New("invalid observed PCM phase sequence")

// ObservedPCMStage connects one real score feedback/write/return interval to
// the existing timed PCM/pan leaves. The caller supplies the actual phase
// clock and closes the stage at its real return. It does not price unrelated
// score operations, choose a world timeline, or replay diagnostic outputs.
type ObservedPCMStage struct {
	read     score.FeedbackRead
	music    *MusicPublication
	pan      *PanRouting
	intent   score.Intent
	expected bool
	feedback score.DACFeedback
	consumed bool
	done     bool
	failure  error
}

func BeginObservedPCM(clock Clock, read score.FeedbackRead, panEntry string, bus Bus, chip Chip, limits Limits) (*ObservedPCMStage, error) {
	if bus == nil || read.VoiceSlot < 0 || read.VoiceSlot >= 18 || read.IntentOffset < 0 || read.Pan != (score.Stereo{Left: read.TrackPan.Left, Right: read.TrackPan.Right}) {
		return nil, ErrObservedPCMSequence
	}
	t := &ObservedPCMStage{read: read}
	switch read.Kind {
	case score.ReadBlockedPriority:
		if panEntry != "" || read.VoiceSlot != 0 || read.WritePan != read.TrackPan {
			return nil, ErrObservedPCMSequence
		}
		musicBus, ok := bus.(MusicBus)
		if !ok {
			return nil, ErrObservedPCMSequence
		}
		descriptor, ok := musicBus.MusicDescriptor(read.DescriptorID)
		if !ok || descriptor.Flags&192 != read.DescriptorPriority {
			return nil, ErrObservedPCMSequence
		}
		var err error
		t.music, err = BeginMusicPublication(clock, MusicRequest{DescriptorID: read.DescriptorID, Pan: read.TrackPan}, musicBus, limits)
		if err != nil {
			return nil, err
		}
		t.feedback = t.music.Feedback()
		if input, accepted := t.music.ExpectedInput(); accepted {
			t.expected = true
			t.intent = score.Intent{Order: read.IntentOffset, Kind: "dac", DescriptorID: input.DescriptorID, Pan: &score.Stereo{Left: read.TrackPan.Left, Right: read.TrackPan.Right}}
		}
	case score.ReadPanRedirect:
		if read.DescriptorID != "" || read.DescriptorPriority != 0 {
			return nil, ErrObservedPCMSequence
		}
		panBus, ok := bus.(PanBus)
		if !ok {
			return nil, ErrObservedPCMSequence
		}
		var err error
		t.pan, err = BeginPanRouting(clock, PanRequest{Entry: panEntry, Channel: 6, Pan: read.TrackPan, WritePan: &read.WritePan}, panBus, chip, limits)
		if err != nil {
			return nil, err
		}
		effect, ok := t.pan.ExpectedEffect()
		if !ok {
			return nil, ErrObservedPCMSequence
		}
		t.expected = true
		t.feedback.PanRedirect = effect.Kind == "dac-pan"
		if t.feedback.PanRedirect {
			t.intent = score.Intent{Order: read.IntentOffset, Kind: "dac-pan", Pan: &score.Stereo{Left: read.TrackPan.Left, Right: read.TrackPan.Right}}
		} else {
			t.intent = score.Intent{Order: read.IntentOffset, Kind: "ym", Bank: effect.Write.Bank, Register: effect.Write.Register, Value: effect.Write.Value}
		}
	default:
		return nil, ErrObservedPCMSequence
	}
	return t, nil
}

// BeginObservedPan connects an actual ordinary FM pan operation, which has no
// DAC feedback callback. stored is read from the current working transaction;
// the requested value comes directly from this canonical write operation.
// Channel six must use BeginObservedPCM at its preceding live feedback stage.
func BeginObservedPan(clock Clock, op score.Operation, stored score.Pan, panEntry string, bus PanBus, chip Chip, limits Limits) (*ObservedPCMStage, error) {
	if op.Version != 1 || op.Kind != score.OpChipWrite || op.WriteCount != 1 || op.VoiceSlot < 0 || op.VoiceSlot >= 18 || op.IntentOffset < 0 {
		return nil, ErrObservedPCMSequence
	}
	var channel uint8
	switch op.Channel {
	case "fm1":
		channel = 0
	case "fm2":
		channel = 1
	case "fm3":
		channel = 2
	case "fm4":
		channel = 4
	case "fm5":
		channel = 5
	default:
		return nil, ErrObservedPCMSequence
	}
	write := op.Writes[0]
	if write.Order != op.IntentOffset || write.Kind != "ym" || write.Bank != channel>>2 || write.Register != 0xb4+(channel&3) || write.Value&8 != 0 || write.Pan != nil || write.DescriptorID != "" || write.Action != "" || write.SoundID != 0 {
		return nil, ErrObservedPCMSequence
	}
	value := score.Pan{Left: write.Value&128 != 0, Right: write.Value&64 != 0, AMS: write.Value >> 4 & 3, FMS: write.Value & 7}
	pan, err := BeginPanRouting(clock, PanRequest{Entry: panEntry, Channel: channel, Pan: stored, WritePan: &value}, bus, chip, limits)
	if err != nil {
		return nil, err
	}
	return &ObservedPCMStage{read: score.FeedbackRead{VoiceSlot: op.VoiceSlot, Position: op.Position, IntentOffset: op.IntentOffset}, pan: pan, intent: write, expected: true}, nil
}

func (t *ObservedPCMStage) Feedback() score.DACFeedback { return t.feedback }
func (t *ObservedPCMStage) Clock() Clock {
	if t.music != nil {
		return t.music.Clock()
	}
	return t.pan.Clock()
}
func (t *ObservedPCMStage) ExpectedIntent() (score.Intent, bool) {
	intent := t.intent
	if intent.Pan != nil {
		pan := *intent.Pan
		intent.Pan = &pan
	}
	return intent, t.expected
}
func (t *ObservedPCMStage) fail(err error) error { t.failure = err; return err }

func sameObservedIntent(a, b score.Intent) bool {
	if (a.Pan == nil) != (b.Pan == nil) || a.Pan != nil && *a.Pan != *b.Pan {
		return false
	}
	a.Pan, b.Pan = nil, nil
	return a == b
}

// Consume validates the canonical operation's owner, logical event and span
// before its single real effect. A copied ExpectedIntent is never itself an
// authorization to apply it at another voice, event or intent position.
func (t *ObservedPCMStage) Consume(op score.Operation) error {
	if t == nil {
		return ErrObservedPCMSequence
	}
	if t.done || t.failure != nil || t.consumed || !t.expected || op.Version != 1 || op.Kind != score.OpChipWrite || op.WriteCount != 1 || op.VoiceSlot != t.read.VoiceSlot || op.Position != t.read.Position || op.IntentOffset != t.read.IntentOffset || !sameObservedIntent(op.Writes[0], t.intent) {
		return t.fail(ErrObservedPCMSequence)
	}
	var err error
	if t.music != nil {
		input, _ := t.music.ExpectedInput()
		err = t.music.Apply(input)
	} else {
		effect, _ := t.pan.ExpectedEffect()
		err = t.pan.Apply(effect)
	}
	if err != nil {
		return t.fail(err)
	}
	t.consumed = true
	return nil
}

func (t *ObservedPCMStage) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrObservedPCMSequence
	}
	if t.done || t.failure != nil || t.expected && !t.consumed {
		return Clock{}, t.fail(ErrObservedPCMSequence)
	}
	var clock Clock
	var err error
	if t.music != nil {
		clock, err = t.music.Finish()
	} else {
		clock, err = t.pan.Finish()
	}
	if err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return clock, nil
}
