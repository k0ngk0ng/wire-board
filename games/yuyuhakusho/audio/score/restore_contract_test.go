package score_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
)

func restoreContractRaw(t *testing.T) []byte {
	t.Helper()
	f, err := os.Open("../../data/audio-score.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func restoreContractCatalog(t *testing.T) *score.Catalog {
	t.Helper()
	c, err := score.DecodeCatalog(bytes.NewReader(restoreContractRaw(t)))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func restoreContractStarted(t *testing.T, c *score.Catalog, id uint8) (*score.Driver, score.State) {
	t.Helper()
	d, err := score.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Advance(score.Input{SoundID: id}); err != nil {
		t.Fatal(err)
	}
	return d, d.Snapshot()
}

func TestRestoreContractRejectsInvalidState(t *testing.T) {
	c := restoreContractCatalog(t)
	_, initial := restoreContractStarted(t, c, 0x82)
	_, finite := restoreContractStarted(t, c, 0x8f)
	cases := []struct {
		name   string
		base   score.State
		change func(*score.State)
	}{
		{"unknown-channel", initial, func(s *score.State) { s.Voices[1].Channel = "not-a-channel" }},
		{"wrong-known-channel", initial, func(s *score.State) { s.Voices[1].Channel = "fm5" }},
		{"active-without-owner", initial, func(s *score.State) { s.Voices[1].Owned = false }},
		{"unowned-negative-event-owner", initial, func(s *score.State) {
			s.Voices[1].Owned = false
			s.Voices[1].Owner.VoiceID = -1
			s.Voices[1].NoteWait = 1
		}},
		{"finite-track-in-cycle", finite, func(s *score.State) { s.Voices[0].Cycle = true; s.Voices[0].EventIndex = 0 }},
		{"unknown-dac-descriptor", initial, func(s *score.State) { s.Voices[0].DACDescriptor = "missing-descriptor" }},
		{"volume-cursor-out-of-range", initial, func(s *score.State) {
			s.Voices[7].VolumeEnvelope = 1
			s.Voices[7].VolumeCursor = param.EnvelopeCursor{Index: 65535}
		}},
		{"pitch-cursor-out-of-range", initial, func(s *score.State) {
			s.Voices[1].PitchControl.EnvelopeID = 1
			s.Voices[1].PitchCursor = param.EnvelopeCursor{Index: 65535}
		}},
		{"pan-step-out-of-range", initial, func(s *score.State) {
			s.Voices[1].PanDefinition = param.PanDefinition{Mode: param.PanAdvanceOnNote, Pattern: "right-left-right-center", StepLimit: 3, Period: 1}
			s.Voices[1].PanState = param.PanState{Step: 255}
		}},
		{"modulation-without-definition", initial, func(s *score.State) {
			s.Voices[1].HasModulation = false
			s.Voices[1].PitchControl.ModulationEnabled = true
		}},
		{"modulation-delay-exceeds-definition", initial, func(s *score.State) {
			v := &s.Voices[1]
			v.HasModulation = true
			v.Modulation = score.Modulation{Delay: 1, Rate: 2, Delta: 3, Steps: 4}
			v.ModDelay = 2
			v.ModRate = 2
			v.ModDelta = 3
			v.ModSteps = 2
		}},
		{"modulation-steps-exceed-definition", initial, func(s *score.State) {
			v := &s.Voices[1]
			v.HasModulation = true
			v.Modulation = score.Modulation{Delay: 1, Rate: 2, Delta: 3, Steps: 4}
			v.ModDelay = 1
			v.ModRate = 2
			v.ModDelta = 3
			v.ModSteps = 5
		}},
		{"modulation-direction-invalid", initial, func(s *score.State) {
			v := &s.Voices[1]
			v.HasModulation = true
			v.Modulation = score.Modulation{Delay: 1, Rate: 2, Delta: 3, Steps: 4}
			v.ModDelay = 1
			v.ModRate = 2
			v.ModDelta = 99
			v.ModSteps = 2
		}},
		{"tempo-remaining-exceeds-period", initial, func(s *score.State) { s.TempoPeriod = 1; s.TempoRemaining = 2 }},
		{"tempo-zero-remaining", initial, func(s *score.State) { s.TempoPeriod = 1; s.TempoRemaining = 0 }},
		{"disabled-tempo-nonzero-remaining", initial, func(s *score.State) { s.TempoPeriod = 0; s.TempoRemaining = 1 }},
		{"conflicting-music-instance", initial, func(s *score.State) { s.Voices[1].Owner.ScoreID = 0x83 }},
		{"invalid-noise-latch", initial, func(s *score.State) { s.Voices[9].Channel = "psg-noise"; s.Voices[9].NoiseControl = 0xff }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.base
			tc.change(&s)
			before := s
			if _, err := score.Restore(c, s); err == nil {
				t.Fatal("invalid checkpoint accepted")
			}
			if !reflect.DeepEqual(s, before) {
				t.Fatal("rejection changed caller checkpoint")
			}
		})
	}
}

func TestRestoreContractKeepsIndependentOwners(t *testing.T) {
	c := restoreContractCatalog(t)
	d, saved := restoreContractStarted(t, c, 0x82)
	before := d.Snapshot()
	// Values in an external snapshot must not alias the live driver.
	saved.Voices[1].BaseLevel++
	if !reflect.DeepEqual(d.Snapshot(), before) {
		t.Fatal("snapshot aliases live state")
	}
	restored, err := score.Restore(c, before)
	if err != nil {
		t.Fatal(err)
	}
	before.Voices[1].BaseLevel++
	if !reflect.DeepEqual(restored.Snapshot(), d.Snapshot()) {
		t.Fatal("Restore aliases caller input")
	}
	state := d.Snapshot()
	output, err := d.Advance(score.Input{SoundID: 0xac})
	if err == nil || len(output.Intents) != 0 || output.ClearPrimaryPriority || !reflect.DeepEqual(state, d.Snapshot()) {
		t.Fatal("invalid Advance partially committed")
	}
}

func TestDecodeCatalogContractRejectsInvalidProfile(t *testing.T) {
	raw := restoreContractRaw(t)
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"unknown-channel", func(c map[string]any) {
			c["scores"].([]any)[0].(map[string]any)["voices"].([]any)[0].(map[string]any)["channel"] = "unknown-channel"
		}},
		{"missing-required-pitch-envelope", func(c map[string]any) { c["pitch_envelopes"].([]any)[0].(map[string]any)["id"] = float64(9) }},
		{"missing-required-volume-envelope", func(c map[string]any) {
			for _, entry := range c["psg_volume_envelopes"].([]any) {
				row := entry.(map[string]any)
				if row["id"].(float64) == 6 {
					row["id"] = float64(12)
					return
				}
			}
			t.Fatal("missing source envelope 6")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var catalog map[string]any
			if err := json.Unmarshal(raw, &catalog); err != nil {
				t.Fatal(err)
			}
			tc.change(catalog)
			changed, err := json.Marshal(catalog)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = score.DecodeCatalog(bytes.NewReader(changed)); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}

func TestDecodeCatalogContractRejectsHiddenTrailingDocument(t *testing.T) {
	raw := restoreContractRaw(t)
	if len(raw) >= 16<<20 {
		t.Fatal("catalog unexpectedly exceeds profile")
	}
	withHiddenTail := append(append(append([]byte{}, raw...), bytes.Repeat([]byte{' '}, (16<<20)-len(raw)+1)...), []byte("{}")...)
	if _, err := score.DecodeCatalog(bytes.NewReader(withHiddenTail)); err == nil {
		t.Fatal("trailing JSON beyond LimitReader boundary accepted")
	}
}
