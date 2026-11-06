package scoreclock_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type heldEnvelopeKey struct {
	ID      uint16 `json:"envelope_id"`
	Section string
	Index   uint16 `json:"tick_index"`
}

type heldNative struct {
	Input struct {
		ID              string
		Channel         uint8
		NoteWait        uint8 `json:"note_wait"`
		GateRemaining   uint8 `json:"gate_remaining"`
		GateTicks       uint8 `json:"gate_ticks"`
		NoteOff         bool  `json:"note_off"`
		Claimed, Tie    bool
		BasePitch       uint16 `json:"base_pitch"`
		BaseLevel       uint8  `json:"base_level"`
		Detune, Scale   int8
		ModEnabled      bool  `json:"mod_enabled"`
		ModDelay        uint8 `json:"mod_delay"`
		ModRate         uint8 `json:"mod_rate"`
		ModSteps        uint8 `json:"mod_steps"`
		ModDelta        int8  `json:"mod_delta"`
		ModOffset       int16 `json:"mod_offset"`
		Modulation      score.Modulation
		PanDefinition   param.PanDefinition `json:"pan_definition"`
		PanState        param.PanState      `json:"pan_state"`
		PitchKey        *heldEnvelopeKey    `json:"pitch_key"`
		VolumeKey       *heldEnvelopeKey    `json:"volume_key"`
		Origin, Refresh uint64
	}
	Events []instrumentNativeEvent
	PSG    []struct {
		At    uint64
		Value uint8
	}
	End   scoreclock.Clock
	State struct {
		NoteWait      uint8 `json:"note_wait"`
		GateRemaining uint8 `json:"gate_remaining"`
		NoteOff       bool  `json:"note_off"`
		Scale         int8
		ModDelay      uint8          `json:"mod_delay"`
		ModRate       uint8          `json:"mod_rate"`
		ModSteps      uint8          `json:"mod_steps"`
		ModDelta      int8           `json:"mod_delta"`
		ModOffset     int16          `json:"mod_offset"`
		PanState      param.PanState `json:"pan_state"`
		StoredPan     uint8          `json:"stored_pan"`
	}
	Equal bool `json:"native_observer_state_equal"`
}

func heldNatives(t *testing.T) []heldNative {
	t.Helper()
	var f struct {
		Version int
		Cases   []heldNative
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-held-voice/native-held-voice-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 721 {
		t.Fatal("complete held-note oracle is missing")
	}
	return f.Cases
}

func heldState(t *testing.T, c *score.Catalog, row heldNative) (score.State, int) {
	t.Helper()
	in := row.Input
	slot := map[uint8]int{0: 1, 1: 2, 2: 3, 4: 4, 5: 5, 128: 7, 160: 8, 192: 9, 224: 9}[in.Channel]
	s := observedEnvelopeState(t, c, slot)
	v := &s.Voices[slot]
	if in.Channel == 224 {
		v.Channel, v.NoiseControl = "psg-noise", 0xe3
	}
	v.NoteWait, v.GateRemaining, v.GateTicks = in.NoteWait, in.GateRemaining, in.GateTicks
	v.NoteOff, v.Claimed, v.Tie = in.NoteOff, in.Claimed, in.Tie
	v.BasePitch, v.BaseLevel, v.Detune, v.PitchScale = in.BasePitch, in.BaseLevel, in.Detune, in.Scale
	v.HasModulation, v.Modulation = true, in.Modulation
	v.ModDelay, v.ModRate, v.ModSteps = in.ModDelay, in.ModRate, in.ModSteps
	v.ModDelta, v.ModOffset = in.ModDelta, in.ModOffset
	v.Pan, v.PanDefinition, v.PanState = panValue(181), in.PanDefinition, in.PanState
	v.PitchControl, v.VolumeEnvelope = score.PitchControl{ModulationEnabled: in.ModEnabled}, 0
	v.PitchCursor, v.VolumeCursor = param.EnvelopeCursor{}, param.EnvelopeCursor{}
	if key := in.PitchKey; key != nil {
		v.PitchControl.EnvelopeID = key.ID
		v.PitchCursor = param.EnvelopeCursor{Cycle: key.Section == "cycle", Index: key.Index}
	}
	if key := in.VolumeKey; key != nil {
		v.VolumeEnvelope = key.ID
		v.VolumeCursor = param.EnvelopeCursor{Cycle: key.Section == "cycle", Index: key.Index}
	}
	return s, slot
}

func TestHeldVoiceCompleteOriginalCallersAndActualScore(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	c := library.Catalog()
	for _, row := range heldNatives(t) {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("observer changed native voice state")
			}
			before, slot := heldState(t, c, row)
			d, err := score.Restore(c, before)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := score.Restore(c, before)
			if err != nil {
				t.Fatal(err)
			}
			owner, _, _ := newPauseMixed(t)
			if _, err = owner.r.RequestBus(0, true); err != nil {
				t.Fatal(err)
			}
			e := &keyIO{&instrumentIO{r: owner.r}}
			var tx *score.TickTransaction
			var voice *scoreclock.HeldVoiceTransfer
			var end scoreclock.Clock
			closed := false
			tx, err = d.BeginTickObserved(score.ObservedHooks{Operation: func(op score.Operation) error {
				if op.VoiceSlot != int8(slot) || closed {
					return nil
				}
				if voice == nil && !(op.Kind == score.OpEnter && op.Routine == score.RoutineVoice) {
					return nil
				}
				state, failure := tx.Snapshot()
				if failure != nil {
					return failure
				}
				if voice == nil {
					voice, failure = scoreclock.BeginHeldVoice(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, op, state.Voices[slot], c, e, e, scoreclock.Limits{})
					if len(e.trace) != 0 {
						return errors.New("held-note preparation wrote hardware")
					}
					return failure
				}
				if failure = voice.Consume(op, state.Voices[slot]); failure != nil {
					return failure
				}
				if op.Kind == score.OpLeave && op.Routine == score.RoutineVoice {
					end, failure = voice.Finish()
					closed = true
				}
				return failure
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Abort()
			if err = tx.AdvanceTempo(); err != nil {
				t.Fatal(err)
			}
			if err = tx.Dispatch(0); err != nil {
				t.Fatal(err)
			}
			if err = tx.AdvanceVoices(); err != nil {
				t.Fatal(err)
			}
			out, err := tx.Finish()
			want, failure := legacy.Advance(score.Input{})
			if err != nil || failure != nil || !closed || voice == nil || end != row.End || !reflect.DeepEqual(out, want) || d.Snapshot() != legacy.Snapshot() {
				t.Fatal("complete held voice changed score or final clock", end, row.End, err, failure)
			}
			if _, err = voice.Finish(); !errors.Is(err, scoreclock.ErrHeldVoiceSequence) {
				t.Fatal("closed complete voice reused")
			}
			if want := frequencyPanAccesses(t, row.Events, row.PSG); !reflect.DeepEqual(e.trace, want) {
				t.Fatal("complete voice IO order/value/timing differs", e.trace, want)
			}
			v, n := d.Snapshot().Voices[slot], row.State
			if v.NoteWait != n.NoteWait || v.GateRemaining != n.GateRemaining || v.NoteOff != n.NoteOff || v.PitchScale != n.Scale || v.ModDelay != n.ModDelay || v.ModRate != n.ModRate || v.ModSteps != n.ModSteps || v.ModDelta != n.ModDelta || v.ModOffset != n.ModOffset || v.PanState != n.PanState || v.Pan != panValue(n.StoredPan) {
				t.Fatal("actual score counters differ from source whole-voice result", v, n)
			}
		})
	}
}

type heldFailedChip struct {
	*keyIO
	fail bool
}

func (e *heldFailedChip) WritePort(at uint64, port, value uint8) error {
	if e.fail {
		return errors.New("detached held voice chip failed")
	}
	return e.keyIO.WritePort(at, port, value)
}

func TestHeldVoiceSharedBudgetExternalFailureAndScoreRollback(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	c := library.Catalog()
	var row heldNative
	for _, candidate := range heldNatives(t) {
		if candidate.Input.ID == "643-modulation" {
			row = candidate
		}
	}
	if row.Input.ID == "" {
		t.Fatal("shared-budget witness missing")
	}
	for _, failIO := range []bool{false, true} {
		before, slot := heldState(t, c, row)
		d, err := score.Restore(c, before)
		if err != nil {
			t.Fatal(err)
		}
		owner, _, _ := newPauseMixed(t)
		if _, err = owner.r.RequestBus(0, true); err != nil {
			t.Fatal(err)
		}
		e := &heldFailedChip{keyIO: &keyIO{&instrumentIO{r: owner.r}}, fail: failIO}
		limits := scoreclock.Limits{MaxOperations: 80}
		if failIO {
			limits = scoreclock.Limits{}
		}
		var tx *score.TickTransaction
		var voice *scoreclock.HeldVoiceTransfer
		tx, err = d.BeginTickObserved(score.ObservedHooks{Operation: func(op score.Operation) error {
			if op.VoiceSlot != int8(slot) {
				return nil
			}
			if voice == nil && !(op.Kind == score.OpEnter && op.Routine == score.RoutineVoice) {
				return nil
			}
			s, failure := tx.Snapshot()
			if failure != nil {
				return failure
			}
			if voice == nil {
				voice, failure = scoreclock.BeginHeldVoice(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, op, s.Voices[slot], c, e, e, limits)
				return failure
			}
			return voice.Consume(op, s.Voices[slot])
		}})
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.AdvanceTempo(); err != nil {
			t.Fatal(err)
		}
		if err = tx.Dispatch(0); err != nil {
			t.Fatal(err)
		}
		err = tx.AdvanceVoices()
		if err == nil || !failIO && !errors.Is(err, scoreclock.ErrBudget) || d.Snapshot() != before || len(e.trace) == 0 {
			t.Fatal("whole-voice failure committed score or reset nested budget", failIO, err)
		}
		if _, err = voice.Finish(); !errors.Is(err, scoreclock.ErrHeldVoiceSequence) {
			t.Fatal("failed whole voice continued")
		}
		if _, err = tx.Snapshot(); err == nil {
			t.Fatal("failed score transaction remained open")
		}
	}
}

func TestHeldVoiceInvalidEntrySequenceAndEarlyFinish(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	c := library.Catalog()
	before, slot := heldState(t, c, heldNatives(t)[643])
	var first, next score.Operation
	var firstVoice, nextVoice score.VoiceState
	d, err := score.Restore(c, before)
	if err != nil {
		t.Fatal(err)
	}
	var tx *score.TickTransaction
	tx, err = d.BeginTickObserved(score.ObservedHooks{Operation: func(op score.Operation) error {
		if op.VoiceSlot != int8(slot) {
			return nil
		}
		if op.Kind == score.OpEnter && op.Routine == score.RoutineVoice {
			s, failure := tx.Snapshot()
			first, firstVoice = op, s.Voices[slot]
			return failure
		}
		if first.Version != 0 && next.Version == 0 {
			s, failure := tx.Snapshot()
			next, nextVoice = op, s.Voices[slot]
			return failure
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Abort()
	if err = tx.AdvanceTempo(); err != nil {
		t.Fatal(err)
	}
	if err = tx.Dispatch(0); err != nil {
		t.Fatal(err)
	}
	if err = tx.AdvanceVoices(); err != nil {
		t.Fatal(err)
	}
	owner, _, _ := newPauseMixed(t)
	e := &keyIO{&instrumentIO{r: owner.r}}
	for _, change := range []func(*score.Operation, *score.VoiceState){
		func(op *score.Operation, v *score.VoiceState) { op.Version = 0 },
		func(op *score.Operation, v *score.VoiceState) { op.Channel = "psg2" },
		func(op *score.Operation, v *score.VoiceState) { v.NoteWait = 1 },
		func(op *score.Operation, v *score.VoiceState) { op.Channel, v.Channel = "dac", "dac" },
		func(op *score.Operation, v *score.VoiceState) { op.Channel, v.Channel = "fm6", "fm6" },
	} {
		op, v := first, firstVoice
		change(&op, &v)
		if _, err := scoreclock.BeginHeldVoice(scoreclock.Clock{}, op, v, c, e, e, scoreclock.Limits{}); err == nil {
			t.Fatal("unsupported/mismatched complete voice accepted")
		}
	}
	for _, bad := range []bool{false, true} {
		voice, err := scoreclock.BeginHeldVoice(scoreclock.Clock{}, first, firstVoice, c, e, e, scoreclock.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		if bad {
			op := next
			op.Sequence++
			if err = voice.Consume(op, nextVoice); !errors.Is(err, scoreclock.ErrHeldVoiceSequence) {
				t.Fatal("skipped canonical operation accepted", err)
			}
		} else if _, err = voice.Finish(); !errors.Is(err, scoreclock.ErrHeldVoiceSequence) {
			t.Fatal("early complete voice finish accepted")
		}
		if err = voice.Consume(next, nextVoice); !errors.Is(err, scoreclock.ErrHeldVoiceSequence) {
			t.Fatal("poisoned complete voice continued")
		}
	}
}
