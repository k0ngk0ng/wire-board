package scoreclock_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/chips/psg"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/mixed"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/resourcebridge"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

type envelopeNative struct {
	Input struct {
		ID  string
		Key struct {
			Family, Section string
			ID              uint16 `json:"envelope_id"`
			Index           uint16 `json:"tick_index"`
		}
		Work            score.EnvelopePathWork
		Tick            json.RawMessage
		Scale           int8
		BasePitch       uint16 `json:"base_pitch"`
		Detune          int8
		Modulation      bool
		ModOffset       int16 `json:"mod_offset"`
		BaseLevel       uint8 `json:"base_level"`
		NoteOff         bool  `json:"note_off"`
		Claimed, Tie    bool
		GateTicks       uint8 `json:"gate_ticks"`
		GateRemaining   uint8 `json:"gate_remaining"`
		Channel         uint8
		Origin, Refresh uint64
	}
	End    scoreclock.Clock
	Next   string
	Word   uint16
	Scale  uint8
	Writes []struct {
		At    uint64
		Value uint8
	}
	Equal bool `json:"native_observer_state_equal"`
}

func envelopeNatives(t *testing.T) []envelopeNative {
	t.Helper()
	var result []envelopeNative
	for _, name := range []string{"native-envelope-full.json.gz", "native-envelope-none.json.gz"} {
		var f struct {
			Version int
			Cases   []envelopeNative
		}
		if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-envelope-source/"+name), &f); err != nil {
			t.Fatal(err)
		}
		if f.Version != 1 {
			t.Fatal("unknown native envelope version")
		}
		result = append(result, f.Cases...)
	}
	if len(result) != 1030 {
		t.Fatal("native envelope scope incomplete", len(result))
	}
	return result
}

func envelopeWorkRaw(t *testing.T) []byte {
	return parserClockUnpack(t, "../../verification/score-envelope-source/audio-score-envelope-work.json.gz")
}

func TestEnvelopeWorkAllLogicalTicksAndAliases(t *testing.T) {
	raw := parserClockUnpack(t, "../../data/audio-score.json.gz")
	base, err := score.DecodeCatalog(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	bound, err := score.BindEnvelopeWork(base, bytes.NewReader(envelopeWorkRaw(t)))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Records []struct {
			Key struct {
				Family, Section string
				ID              uint16 `json:"envelope_id"`
				Index           uint16 `json:"tick_index"`
			}
			Work score.EnvelopePathWork
		}
	}
	if err = json.Unmarshal(envelopeWorkRaw(t), &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Records) != 580 {
		t.Fatal("work resource logical count changed")
	}
	for _, r := range f.Records {
		cursor := param.EnvelopeCursor{Cycle: r.Key.Section == "cycle", Index: r.Key.Index}
		got, err := bound.EnvelopeWork(r.Key.Family, r.Key.ID, cursor)
		if err != nil || got != r.Work {
			t.Fatal("bound logical work differs", r.Key, got, err)
		}
		if _, err = base.EnvelopeWork(r.Key.Family, r.Key.ID, cursor); err == nil {
			t.Fatal("unbound catalog guessed a work path")
		}
	}
	for _, pair := range []struct {
		family     string
		id, length uint16
	}{{"pitch", 2, 1}, {"pitch", 4, 1}, {"volume", 1, 21}, {"volume", 7, 0}} {
		a, err := bound.EnvelopeWork(pair.family, pair.id, param.EnvelopeCursor{Index: pair.length})
		if err != nil {
			t.Fatal(err)
		}
		b, err := bound.EnvelopeWork(pair.family, pair.id, param.EnvelopeCursor{Cycle: true})
		if err != nil || a != b {
			t.Fatal("prefix-end alias does not resolve canonical cycle work", pair, err)
		}
	}
	for _, bad := range []struct {
		family string
		id     uint16
		cursor param.EnvelopeCursor
	}{{"unknown", 1, param.EnvelopeCursor{}}, {"pitch", 0, param.EnvelopeCursor{}}, {"pitch", 1, param.EnvelopeCursor{Index: 22}}, {"volume", 1, param.EnvelopeCursor{Cycle: true, Index: 2}}} {
		if _, err := bound.EnvelopeWork(bad.family, bad.id, bad.cursor); err == nil {
			t.Fatal("unknown envelope position accepted", bad)
		}
	}
	if _, err = score.BindEnvelopeWork(bound, bytes.NewReader(envelopeWorkRaw(t))); err == nil {
		t.Fatal("catalog rebound with second work owner")
	}
	for _, mutate := range []func(map[string]any){
		func(m map[string]any) { m["catalog_json_sha256"] = "wrong" },
		func(m map[string]any) { m["records"] = m["records"].([]any)[1:] },
		func(m map[string]any) { r := m["records"].([]any); m["records"] = append(r, r[0]) },
		func(m map[string]any) {
			m["records"].([]any)[0].(map[string]any)["work"].(map[string]any)["restarts"] = 2
		},
		func(m map[string]any) {
			delete(m["records"].([]any)[0].(map[string]any)["work"].(map[string]any), "restarts")
		},
		func(m map[string]any) { m["records"].([]any)[0].(map[string]any)["work"].(map[string]any)["pc"] = 123 },
	} {
		var document map[string]any
		if err = json.Unmarshal(envelopeWorkRaw(t), &document); err != nil {
			t.Fatal(err)
		}
		mutate(document)
		bad, _ := json.Marshal(document)
		if result, err := score.BindEnvelopeWork(base, bytes.NewReader(bad)); err == nil || result != nil {
			t.Fatal("malformed or unbound work resource accepted")
		}
	}
}

func TestEnvelopeWorkFiveResourceIdentityAndScoreCompatibility(t *testing.T) {
	s := parserClockUnpack(t, "../../data/audio-score.json.gz")
	p := parserClockUnpack(t, "../../data/audio-pcm-payloads.json.gz")
	m := parserClockUnpack(t, "../../data/audio-score-parser-work.json.gz")
	i := parserClockUnpack(t, "../../data/audio-score-instrument-work.json.gz")
	w := envelopeWorkRaw(t)
	old, err := resourcebridge.DecodeWithInstrumentWork(s, p, m, i)
	if err != nil {
		t.Fatal(err)
	}
	current, err := resourcebridge.DecodeWithEnvelopeWork(s, p, m, i, w)
	if err != nil {
		t.Fatal(err)
	}
	if old.Identity() == current.Identity() {
		t.Fatal("fifth resource missing from audio identity")
	}
	variant, err := resourcebridge.DecodeWithEnvelopeWork(s, p, m, i, append(append([]byte{}, w...), byte(' ')))
	if err != nil || variant.Identity() == current.Identity() {
		t.Fatal("raw work resource identity not bound", err)
	}
	if _, err = resourcebridge.DecodeWithEnvelopeWork(s, p, m, i, nil); err == nil {
		t.Fatal("missing fifth resource accepted")
	}
	left, err := score.New(old.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	right, err := score.New(current.Catalog())
	if err != nil {
		t.Fatal(err)
	}
	for tick := 0; tick < 128; tick++ {
		input := score.Input{}
		if tick == 0 {
			input.SoundID = 0x82
		}
		a, err := left.Advance(input)
		if err != nil {
			t.Fatal(err)
		}
		b, err := right.Advance(input)
		if err != nil || !reflect.DeepEqual(a, b) || left.Snapshot() != right.Snapshot() {
			t.Fatal("work binding changed existing score semantics", tick, err)
		}
	}
	oldCfg, _ := old.PCMConfig(0, 64)
	newCfg, _ := current.PCMConfig(0, 64)
	oldMixed := mixed.Config{PCM: oldCfg, Chips: chips.Config{Profile: chips.ReferenceMixedHQ44100}}
	newMixed := mixed.Config{PCM: newCfg, Chips: oldMixed.Chips}
	runtime, err := mixed.New(oldMixed, chips.State{YM: chips.ResetYMState(), PSG: psg.ResetState()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = mixed.Restore(newMixed, runtime.Snapshot()); err == nil {
		t.Fatal("four-resource checkpoint restored under five-resource identity")
	}
}

func TestEnvelopeClockOriginalPitchAndActualPSGVolume(t *testing.T) {
	for index, row := range envelopeNatives(t) {
		t.Run(fmt.Sprintf("%d-%s", index, row.Input.Key.Family), func(t *testing.T) {
			if !row.Equal {
				t.Fatal("native recorder changed envelope state")
			}
			in := row.Input
			clock := scoreclock.Clock{Cycle: in.Origin, Refresh: in.Refresh}
			has := in.Key.ID != 0
			if in.Key.Family == "pitch" {
				var tick param.PitchTick
				if has {
					if err := json.Unmarshal(in.Tick, &tick); err != nil {
						t.Fatal(err)
					}
				}
				family := "fm"
				if in.Channel >= 128 {
					family = "psg"
				}
				r, err := scoreclock.PricePitchCalculation(clock, scoreclock.PitchCalculationRequest{Family: family, HasEnvelope: has, Work: in.Work, Tick: tick, Scale: in.Scale, ModulationEnabled: in.Modulation}, scoreclock.Limits{})
				if err != nil || r.Clock != row.End || r.Next != row.Next {
					t.Fatal("original pitch calculator clock differs", r, row.End, row.Next, err)
				}
				if has {
					mod := uint16(0)
					if in.Modulation {
						mod = uint16(in.ModOffset)
					}
					word, scale, err := param.PitchWord(tick, in.Scale, in.BasePitch, in.Detune, mod)
					if err != nil || word != row.Word || uint8(scale) != row.Scale {
						t.Fatal("canonical pitch semantics differ from original calculator", word, row.Word, scale, row.Scale, err)
					}
				}
			} else {
				var tick param.VolumeTick
				if has {
					if err := json.Unmarshal(in.Tick, &tick); err != nil {
						t.Fatal(err)
					}
				}
				owner, _, _ := newPauseMixed(t)
				chip := &controlPSG{r: owner.r}
				req := scoreclock.EnvelopeVolumeRequest{HasEnvelope: has, Work: in.Work, Tick: tick, BaseLevel: in.BaseLevel, Channel: in.Channel, NoteOff: in.NoteOff, Claimed: in.Claimed, Tied: in.Tie, GateTicks: in.GateTicks, GateRemaining: in.GateRemaining}
				tx, err := scoreclock.BeginEnvelopeVolume(clock, req, chip, scoreclock.Limits{})
				if err != nil {
					t.Fatal(err)
				}
				if len(chip.writes) != 0 {
					t.Fatal("volume lookup/feedback emitted early PSG output")
				}
				value, write := tx.ExpectedWrite()
				if write != (len(row.Writes) == 1) {
					t.Fatal("original volume suppression/keyoff differs")
				}
				if write {
					if value != row.Writes[0].Value {
						t.Fatal("original volume port byte differs", value, row.Writes[0].Value)
					}
					if err = tx.Write(value); err != nil {
						t.Fatal(err)
					}
				}
				end, err := tx.Finish()
				if err != nil || end != row.End || len(chip.writes) != len(row.Writes) {
					t.Fatal("original volume final clock/count differs", end, row.End, err)
				}
				for n, w := range row.Writes {
					if chip.writes[n].At != w.At || chip.writes[n].Value != w.Value {
						t.Fatal("actual independent PSG timing/value differs")
					}
				}
			}
		})
	}
}

func TestEnvelopeClockBudgetValidationAndPoisonedVolume(t *testing.T) {
	level := uint8(3)
	work := scoreclock.EnvelopeVolumeRequest{HasEnvelope: true, Tick: param.VolumeTick{Level: &level, WritesLevel: true}, Channel: 128}
	owner, _, _ := newPauseMixed(t)
	chip := &controlPSG{r: owner.r}
	tx, err := scoreclock.BeginEnvelopeVolume(scoreclock.Clock{}, work, chip, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrVolumeSequence) {
		t.Fatal("early volume finish accepted")
	}
	value, _ := tx.ExpectedWrite()
	if err = tx.Write(value); !errors.Is(err, scoreclock.ErrVolumeSequence) {
		t.Fatal("poisoned volume transaction continued")
	}
	if _, err = scoreclock.PricePitchCalculation(scoreclock.Clock{Cycle: math.MaxUint64}, scoreclock.PitchCalculationRequest{Family: "fm"}, scoreclock.Limits{}); err == nil {
		t.Fatal("overflow accepted")
	}
	if _, err = scoreclock.BeginEnvelopeVolume(scoreclock.Clock{}, work, nil, scoreclock.Limits{}); err == nil {
		t.Fatal("missing PSG owner accepted")
	}
	if _, err = scoreclock.ServeEnvelopeVolume(scoreclock.Clock{}, work, chip, scoreclock.Limits{MaxOperations: 8}); !errors.Is(err, scoreclock.ErrBudget) {
		t.Fatal("volume lookup/write stages reset budget", err)
	}
	if _, err = scoreclock.PricePitchCalculation(scoreclock.Clock{}, scoreclock.PitchCalculationRequest{Family: "fm", HasEnvelope: true, Tick: param.PitchTick{}, Work: score.EnvelopePathWork{Restarts: 2}}, scoreclock.Limits{}); err == nil {
		t.Fatal("unknown envelope path accepted")
	}
}
