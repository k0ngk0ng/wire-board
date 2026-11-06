package scoreclock

import (
	"errors"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

var ErrObservedEnvelopeSequence = errors.New("invalid observed envelope phase sequence")

func observedVoiceMatches(op score.Operation, v score.VoiceState) bool {
	return op.Version == 1 && op.VoiceSlot >= 0 && op.VoiceSlot < 18 && op.IntentOffset >= 0 && op.Channel == v.Channel && op.BaseLevel == v.BaseLevel && op.BasePitch == v.BasePitch && op.GateTicks == v.GateTicks && op.Flags.NoteOff == v.NoteOff && op.Flags.Claimed == v.Claimed && op.Flags.Tie == v.Tie && (!op.Position.Valid || op.Position.Owner == v.Owner)
}

// PriceObservedPitchCalculation starts at the already selected calculator
// resource phase, after its caller guards. It does not time later key/frequency
// writes, advance the cursor, or claim the enclosing voice/tick was priced.
func PriceObservedPitchCalculation(clock Clock, op score.Operation, v score.VoiceState, c *score.Catalog, limits Limits) (PitchCalculationResult, error) {
	if c == nil || op.Kind != score.OpResourceUnavailable || op.Routine != score.RoutinePitch || v.PitchControl.EnvelopeID == 0 || v.NoteOff || v.Claimed || !observedVoiceMatches(op, v) {
		return PitchCalculationResult{}, ErrObservedEnvelopeSequence
	}
	tick, work, err := c.PitchEnvelopeStep(v.PitchControl.EnvelopeID, v.PitchCursor)
	if err != nil {
		return PitchCalculationResult{}, err
	}
	family := "fm"
	switch v.Channel {
	case "psg1", "psg2", "psg3", "psg-noise":
		family = "psg"
	}
	return PricePitchCalculation(clock, PitchCalculationRequest{Family: family, HasEnvelope: true, Work: work, Tick: tick, Scale: v.PitchScale, ModulationEnabled: v.PitchControl.ModulationEnabled}, limits)
}

type ObservedEnvelopeVolume struct {
	op                       score.Operation
	volume                   *VolumeTransfer
	intent                   score.Intent
	expected, consumed, done bool
	failure                  error
}

// BeginObservedEnvelopeVolume consumes current working voice state, not the
// authority driver's pre-transaction snapshot. The supplied clock is the
// volume body entry; held-note caller checks remain the outer timer's work.
func BeginObservedEnvelopeVolume(clock Clock, op score.Operation, v score.VoiceState, c *score.Catalog, chip PSGChip, limits Limits) (*ObservedEnvelopeVolume, error) {
	if c == nil || op.Kind != score.OpEnter || op.Routine != score.RoutineVolume || !observedVoiceMatches(op, v) {
		return nil, ErrObservedEnvelopeSequence
	}
	var channel uint8
	switch v.Channel {
	case "psg1":
		channel = 128
	case "psg2":
		channel = 160
	case "psg3":
		channel = 192
	case "psg-noise":
		channel = 224
	default:
		return nil, ErrObservedEnvelopeSequence
	}
	request := EnvelopeVolumeRequest{HasEnvelope: v.VolumeEnvelope != 0, BaseLevel: v.BaseLevel, Channel: channel, NoteOff: v.NoteOff, Claimed: v.Claimed, Tied: v.Tie, GateTicks: v.GateTicks, GateRemaining: v.GateRemaining}
	if request.HasEnvelope {
		var err error
		request.Tick, request.Work, err = c.VolumeEnvelopeStep(v.VolumeEnvelope, v.VolumeCursor)
		if err != nil {
			return nil, err
		}
	}
	volume, err := BeginEnvelopeVolume(clock, request, chip, limits)
	if err != nil {
		return nil, err
	}
	t := &ObservedEnvelopeVolume{op: op, volume: volume}
	if value, ok := volume.ExpectedWrite(); ok {
		t.expected = true
		t.intent = score.Intent{Order: op.IntentOffset, Kind: "psg", Value: value}
	}
	return t, nil
}

func (t *ObservedEnvelopeVolume) ExpectedIntent() (score.Intent, bool) { return t.intent, t.expected }
func (t *ObservedEnvelopeVolume) Clock() Clock                         { return t.volume.Clock() }
func (t *ObservedEnvelopeVolume) fail(err error) error                 { t.failure = err; return err }
func (t *ObservedEnvelopeVolume) Consume(op score.Operation) error {
	if t == nil {
		return ErrObservedEnvelopeSequence
	}
	if t.done || t.failure != nil || t.consumed || !t.expected || op.Version != 1 || op.Kind != score.OpChipWrite || op.WriteCount != 1 || op.VoiceSlot != t.op.VoiceSlot || op.Position != t.op.Position || op.IntentOffset != t.op.IntentOffset || !sameObservedIntent(op.Writes[0], t.intent) {
		return t.fail(ErrObservedEnvelopeSequence)
	}
	if err := t.volume.Write(op.Writes[0].Value); err != nil {
		return t.fail(err)
	}
	t.consumed = true
	return nil
}
func (t *ObservedEnvelopeVolume) Finish() (Clock, error) {
	if t == nil {
		return Clock{}, ErrObservedEnvelopeSequence
	}
	if t.done || t.failure != nil || t.expected && !t.consumed {
		return Clock{}, t.fail(ErrObservedEnvelopeSequence)
	}
	clock, err := t.volume.Finish()
	if err != nil {
		return Clock{}, t.fail(err)
	}
	t.done = true
	return clock, nil
}
