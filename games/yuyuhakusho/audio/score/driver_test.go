package score_test

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func catalog(t *testing.T) *score.Catalog {
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
	c, err := score.DecodeCatalog(z)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type originalFrame struct {
	Kind          string          `json:"kind"`
	Step          int             `json:"step"`
	Dispatch      uint8           `json:"dispatch_sound_id"`
	ClearPriority bool            `json:"clear_primary_priority"`
	State         json.RawMessage `json:"state"`
	Intents       []score.Intent  `json:"intents"`
	Feedback      []struct {
		Blocks   bool  `json:"blocks_score_dac"`
		Priority uint8 `json:"mode_priority"`
	} `json:"dac_feedback"`
	PanFeedback []struct {
		Redirect bool `json:"pan_redirect"`
	} `json:"pan_feedback"`
}

func equalJSON(t *testing.T, context string, actual any, expected json.RawMessage) {
	t.Helper()
	raw, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	var a, b any
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &b); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		// Keep the exact first mismatch reviewable without dumping all 18 voices.
		am, bm := a.(map[string]any), b.(map[string]any)
		for k, want := range bm {
			if !reflect.DeepEqual(am[k], want) {
				if k == "voices" {
					av, bv := am[k].([]any), want.([]any)
					for i := range bv {
						if !reflect.DeepEqual(av[i], bv[i]) {
							t.Fatalf("%s voice %d: actual=%v expected=%v", context, i, av[i], bv[i])
						}
					}
				}
				t.Fatalf("%s %s: actual=%v expected=%v", context, k, am[k], want)
			}
		}
		t.Fatalf("%s unexpected fields: actual=%s expected=%s", context, raw, expected)
	}
}

func replayOriginal(t *testing.T, path string, c *score.Catalog) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	decoder := json.NewDecoder(z)
	d, err := score.New(c)
	if err != nil {
		t.Fatal(err)
	}
	var first originalFrame
	if err := decoder.Decode(&first); err != nil || first.Kind != "initial" {
		t.Fatal("invalid initial fixture", err)
	}
	equalJSON(t, "initial", d.View(), first.State)
	var resumed *score.Driver
	frames, intents := 0, 0
	for {
		var original originalFrame
		if err := decoder.Decode(&original); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if original.Kind != "frame" {
			t.Fatal("unexpected fixture row")
		}
		if original.Step != frames {
			t.Fatal("nonconsecutive original steps")
		}
		input := score.Input{SoundID: original.Dispatch}
		if len(original.Feedback) > 1 {
			t.Fatal("multiple guards require an explicit input contract")
		}
		if len(original.Feedback) == 1 {
			input.DAC = score.DACFeedback{Blocked: original.Feedback[0].Blocks, Priority: original.Feedback[0].Priority}
		}
		if len(original.PanFeedback) > 1 {
			t.Fatal("multiple pan feedback reads require an explicit input contract")
		}
		if len(original.PanFeedback) == 1 {
			input.DAC.PanRedirect = original.PanFeedback[0].Redirect
		}
		actual, err := d.Advance(input)
		if err != nil {
			t.Fatalf("step %d: %v", frames, err)
		}
		if actual.ClearPrimaryPriority != original.ClearPriority {
			t.Fatalf("step %d priority release: actual=%v expected=%v", frames, actual.ClearPrimaryPriority, original.ClearPriority)
		}
		// Ordinary PCM dispatch belongs to the root PCM owner. Its presence
		// remains in the original fixture, separate from score chip output.
		wanted := make([]score.Intent, 0, len(original.Intents))
		for _, intent := range original.Intents {
			if intent.Kind != "pcm-command" {
				intent.Order = len(wanted)
				wanted = append(wanted, intent)
			}
		}
		if !reflect.DeepEqual(actual.Intents, wanted) {
			t.Fatalf("step %d chip intents: actual=%+v expected=%+v", frames, actual.Intents, wanted)
		}
		equalJSON(t, fmt.Sprintf("step %d", frames), d.View(), original.State)
		if frames == 7 {
			resumed, err = score.Restore(c, d.Snapshot())
			if err != nil {
				t.Fatal(err)
			}
		} else if resumed != nil {
			after, err := resumed.Advance(input)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, actual) || !reflect.DeepEqual(resumed.Snapshot(), d.Snapshot()) {
				t.Fatalf("step %d resumed state diverged", frames)
			}
		}
		frames++
		intents += len(wanted)
	}
	return frames, intents
}

func TestOriginalContinuousDriver(t *testing.T) {
	c := catalog(t)
	var index struct {
		Cases []struct {
			ID, Path string
			Steps    int
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/original-index.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Cases) < 23 {
		t.Fatal("missing original definitions or mixed cases")
	}
	for _, test := range index.Cases {
		t.Run(test.ID, func(t *testing.T) {
			frames, intents := replayOriginal(t, filepath.Join("testdata", test.Path), c)
			if frames != test.Steps || intents == 0 {
				t.Fatalf("incomplete replay %d/%d, %d intents", frames, test.Steps, intents)
			}
			t.Logf("%d frames, %d ordered chip/queue intents", frames, intents)
		})
	}
}

func TestOwnedSnapshotsAndAtomicRejection(t *testing.T) {
	c := catalog(t)
	d, err := score.New(c)
	if err != nil {
		t.Fatal(err)
	}
	before := d.Snapshot()
	if _, err := d.Advance(score.Input{SoundID: 0xac}); err == nil {
		t.Fatal("unrestored group2 accepted")
	}
	if !reflect.DeepEqual(d.Snapshot(), before) {
		t.Fatal("invalid command partially committed")
	}
	if _, err := d.Advance(score.Input{DAC: score.DACFeedback{Priority: 1}}); err == nil {
		t.Fatal("invalid PCM feedback accepted")
	}
	if !reflect.DeepEqual(d.Snapshot(), before) {
		t.Fatal("invalid feedback partially committed")
	}
	if _, err := d.Advance(score.Input{SoundID: 0x82}); err != nil {
		t.Fatal(err)
	}
	saved := d.Snapshot()
	copy, err := score.Restore(c, saved)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 64; n++ {
		a, err := d.Advance(score.Input{})
		if err != nil {
			t.Fatal(err)
		}
		b, err := copy.Advance(score.Input{})
		if err != nil || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(d.Snapshot(), copy.Snapshot()) {
			t.Fatal("snapshot continuation", n, err)
		}
	}
}
