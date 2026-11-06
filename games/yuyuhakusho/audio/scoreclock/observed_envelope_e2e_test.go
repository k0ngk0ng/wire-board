package scoreclock_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

func observedEnvelopeLibrary(t *testing.T) *resourcebridge.Library {
	t.Helper()
	l, err := resourcebridge.DecodeWithEnvelopeWork(parserClockUnpack(t, "../../data/audio-score.json.gz"), parserClockUnpack(t, "../../data/audio-pcm-payloads.json.gz"), parserClockUnpack(t, "../../data/audio-score-parser-work.json.gz"), parserClockUnpack(t, "../../data/audio-score-instrument-work.json.gz"), parserClockUnpack(t, "../../data/audio-score-envelope-work.json.gz"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func observedEnvelopeState(t *testing.T, c *score.Catalog, slot int) score.State {
	t.Helper()
	d, err := score.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Advance(score.Input{SoundID: 0x82}); err != nil {
		t.Fatal(err)
	}
	s := d.Snapshot()
	for i := range s.Voices {
		s.Voices[i].Active = false
	}
	s.TempoPeriod, s.TempoRemaining = 0, 0
	v := &s.Voices[slot]
	v.Active, v.NoteOff, v.Claimed, v.NoteWait = true, false, false, 2
	v.GateTicks, v.GateRemaining = 0, 0
	v.PanDefinition = param.PanDefinition{}
	v.PanState = param.PanState{}
	v.PitchControl = score.PitchControl{}
	return s
}

func TestObservedEnvelopeReadonlyTicksAllPositions(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	c := l.Catalog()
	var doc struct {
		Records []struct {
			Key struct {
				Family, Section string
				ID              uint16 `json:"envelope_id"`
				Index           uint16 `json:"tick_index"`
			}
			Work score.EnvelopePathWork
		}
	}
	if err := json.Unmarshal(envelopeWorkRaw(t), &doc); err != nil {
		t.Fatal(err)
	}
	for _, row := range doc.Records {
		cursor := param.EnvelopeCursor{Cycle: row.Key.Section == "cycle", Index: row.Key.Index}
		if row.Key.Family == "pitch" {
			first, work, err := c.PitchEnvelopeStep(row.Key.ID, cursor)
			if err != nil || work != row.Work {
				t.Fatal("pitch step/work lookup failed", err)
			}
			want := first
			if first.Offset != nil {
				saved := *first.Offset
				want.Offset = &saved
				*first.Offset++
			}
			if first.FixedPitchWord != nil {
				saved := *first.FixedPitchWord
				want.FixedPitchWord = &saved
				*first.FixedPitchWord++
			}
			again, _, err := c.PitchEnvelopeStep(row.Key.ID, cursor)
			if err != nil || !reflect.DeepEqual(again, want) {
				t.Fatal("pitch query leaked immutable pointer", row.Key, err)
			}
		} else {
			first, work, err := c.VolumeEnvelopeStep(row.Key.ID, cursor)
			if err != nil || work != row.Work {
				t.Fatal("volume step/work lookup failed", err)
			}
			want := first
			if first.Level != nil {
				saved := *first.Level
				want.Level = &saved
				*first.Level++
			}
			again, _, err := c.VolumeEnvelopeStep(row.Key.ID, cursor)
			if err != nil || !reflect.DeepEqual(again, want) {
				t.Fatal("volume query leaked immutable pointer", row.Key, err)
			}
		}
	}
}

func TestObservedEnvelopeActualVolumeAll374Steps(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	c := l.Catalog()
	count := 0
	for index, row := range envelopeNatives(t) {
		in := row.Input
		if index >= 580 || in.Key.Family != "volume" {
			continue
		}
		count++
		t.Run(fmt.Sprintf("env%d-%s-%d", in.Key.ID, in.Key.Section, in.Key.Index), func(t *testing.T) {
			before := observedEnvelopeState(t, c, 7)
			v := &before.Voices[7]
			v.VolumeEnvelope = in.Key.ID
			v.VolumeCursor = param.EnvelopeCursor{Cycle: in.Key.Section == "cycle", Index: in.Key.Index}
			v.BaseLevel = in.BaseLevel
			d, err := score.Restore(c, before)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := score.Restore(c, before)
			if err != nil {
				t.Fatal(err)
			}
			owner, _, _ := newPauseMixed(t)
			chip := &controlPSG{r: owner.r}
			var tx *score.TickTransaction
			var stage *scoreclock.ObservedEnvelopeVolume
			var end scoreclock.Clock
			tx, err = d.BeginTickObserved(score.ObservedHooks{Operation: func(op score.Operation) error {
				if op.Kind == score.OpEnter && op.Routine == score.RoutineVolume && op.VoiceSlot == 7 {
					state, err := tx.Snapshot()
					if err != nil {
						return err
					}
					stage, err = scoreclock.BeginObservedEnvelopeVolume(scoreclock.Clock{Cycle: in.Origin, Refresh: in.Refresh}, op, state.Voices[7], c, chip, scoreclock.Limits{})
					if len(chip.writes) != 0 {
						return errors.New("volume lookup emitted early output")
					}
					return err
				}
				if op.Kind == score.OpChipWrite && stage != nil {
					return stage.Consume(op)
				}
				if op.Kind == score.OpLeave && op.Routine == score.RoutineVolume && op.VoiceSlot == 7 {
					if stage == nil {
						return errors.New("volume returned without stage")
					}
					var err error
					end, err = stage.Finish()
					return err
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
			out, err := tx.Finish()
			if err != nil || end != row.End || len(chip.writes) != len(row.Writes) {
				t.Fatal("actual volume phase clock/output differs", end, row.End, err)
			}
			want, err := legacy.Advance(score.Input{})
			if err != nil || !reflect.DeepEqual(out, want) || d.Snapshot() != legacy.Snapshot() {
				t.Fatal("consumer advanced duplicate score state", err)
			}
			for i, w := range row.Writes {
				if chip.writes[i].At != w.At || chip.writes[i].Value != w.Value {
					t.Fatal("actual source PSG intent time/value differs")
				}
			}
		})
	}
	if count != 374 {
		t.Fatal("formal volume connection incomplete", count)
	}
}

func TestObservedEnvelopeActualPitchAll206Steps(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	c := l.Catalog()
	count := 0
	for index, row := range envelopeNatives(t) {
		in := row.Input
		if index >= 580 || in.Key.Family != "pitch" {
			continue
		}
		count++
		t.Run(fmt.Sprintf("env%d-%s-%d", in.Key.ID, in.Key.Section, in.Key.Index), func(t *testing.T) {
			before := observedEnvelopeState(t, c, 1)
			v := &before.Voices[1]
			v.PitchControl.EnvelopeID = in.Key.ID
			v.PitchCursor = param.EnvelopeCursor{Cycle: in.Key.Section == "cycle", Index: in.Key.Index}
			v.PitchScale = in.Scale
			v.BasePitch = in.BasePitch
			v.Detune = in.Detune
			d, err := score.Restore(c, before)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := score.Restore(c, before)
			if err != nil {
				t.Fatal(err)
			}
			var tx *score.TickTransaction
			calls := 0
			tx, err = d.BeginTickObserved(score.ObservedHooks{Operation: func(op score.Operation) error {
				if op.Kind == score.OpResourceUnavailable && op.Routine == score.RoutinePitch {
					calls++
					current, err := tx.Snapshot()
					if err != nil {
						return err
					}
					r, err := scoreclock.PriceObservedPitchCalculation(scoreclock.Clock{Cycle: in.Origin, Refresh: in.Refresh}, op, current.Voices[1], c, scoreclock.Limits{})
					if err != nil {
						return err
					}
					if r.Clock != row.End || r.Next != row.Next {
						return errors.New("actual pitch calculation boundary differs")
					}
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
			out, err := tx.Finish()
			want, fail := legacy.Advance(score.Input{})
			if err != nil || fail != nil || calls != 1 || !reflect.DeepEqual(out, want) || d.Snapshot() != legacy.Snapshot() {
				t.Fatal("actual pitch resource connection changed score", calls, err, fail)
			}
		})
	}
	if count != 206 {
		t.Fatal("formal pitch connection incomplete", count)
	}
}
