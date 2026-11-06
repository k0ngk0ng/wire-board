package score_test

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func parserMetadata(t *testing.T, c *score.Catalog) (*score.ParserMetadata, []byte) {
	t.Helper()
	f, err := os.Open("../../data/audio-score-parser-work.json.gz")
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
	m, err := score.DecodeParserMetadata(c, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return m, raw
}

func TestParserMetadataRejectsUnknownAndUnboundData(t *testing.T) {
	c := catalog(t)
	m, raw := parserMetadata(t, c)
	for _, test := range []struct{ name, old, new string }{
		{"catalog-digest", "735d012d3d3140e76f8f756f0ca757bdc4b8deeb5a66750dd4610e41e06af830", strings.Repeat("0", 64)},
		{"unknown-kind", `"kind":"token-read"`, `"kind":"cpu-bytecode"`},
		{"unknown-role", `"role":"instrument-selection"`, `"role":"raw-source-pc"`},
		{"unknown-entry", `"continue"`, `"oracle-frame"`},
		{"unproved-multiply", `"additions":1`, `"additions":255`},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := bytes.Replace(raw, []byte(test.old), []byte(test.new), 1)
			if bytes.Equal(changed, raw) {
				t.Fatal("negative fixture did not change")
			}
			if _, err := score.DecodeParserMetadata(c, bytes.NewReader(changed)); err == nil {
				t.Fatal("invalid parser metadata accepted")
			}
		})
	}
	position := score.EventPosition{Valid: true, Owner: score.Owner{ScoreID: 0x88, VoiceID: 6}, Cycle: true, Index: 0, Entry: score.EntryContinue}
	if _, err := m.Lookup(position); err == nil {
		t.Fatal("unknown manual cycle0 history silently guessed")
	}
	position.Entry = score.EntryFromPrefix
	first, err := m.Lookup(position)
	if err != nil {
		t.Fatal(err)
	}
	position.Entry = score.EntryCycleWrap
	repeat, err := m.Lookup(position)
	if err != nil {
		t.Fatal(err)
	}
	countJump := func(work []score.ParserWork) int {
		n := 0
		for _, w := range work {
			if w.Kind == score.WorkJump {
				n++
			}
		}
		return n
	}
	if countJump(first) != 0 || countJump(repeat) != 1 {
		t.Fatal("first/repeated cycle entry collapsed")
	}
	first[0].Kind = "mutated"
	position.Entry = score.EntryFromPrefix
	again, _ := m.Lookup(position)
	if again[0].Kind == "mutated" {
		t.Fatal("metadata lookup aliases immutable storage")
	}
}

func TestObservedParserWorkMatchesOriginalReadCounts(t *testing.T) {
	c := catalog(t)
	metadata, _ := parserMetadata(t, c)
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
	if err = json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	counts := map[score.ParserWorkKind]int{}
	tokens, commits, controls, firstTaken, repeatTaken, repeatExit := 0, 0, 0, 0, 0, 0
	cases, frames := 0, 0
	for _, test := range index.Cases {
		if !strings.HasPrefix(test.ID, "isolated-") {
			continue
		}
		f, err := os.Open(filepath.Join("testdata", test.Path))
		if err != nil {
			t.Fatal(err)
		}
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		decoder := json.NewDecoder(z)
		var initial originalFrame
		if err = decoder.Decode(&initial); err != nil {
			t.Fatal(err)
		}
		d, _ := score.New(c)
		legacy, _ := score.New(c)
		steps := 0
		for {
			var frame originalFrame
			err = decoder.Decode(&frame)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			input := score.Input{SoundID: frame.Dispatch}
			if len(frame.Feedback) == 1 {
				input.DAC = score.DACFeedback{Blocked: frame.Feedback[0].Blocks, Priority: frame.Feedback[0].Priority}
			}
			if len(frame.PanFeedback) == 1 {
				input.DAC.PanRedirect = frame.PanFeedback[0].Redirect
			}
			out, err := d.AdvanceObserved(score.ObservedInput{SoundID: input.SoundID}, score.ObservedHooks{Parser: metadata, Operation: func(op score.Operation) error {
				if op.Kind == score.OpParserUnavailable {
					t.Fatal("complete bound parser work missing")
				}
				if op.Kind != score.OpParserWork {
					return nil
				}
				w := op.Work
				counts[w.Kind]++
				switch w.Kind {
				case score.WorkTokenRead, score.WorkDurationProbe:
					tokens++
				case score.WorkControlDispatch:
					controls++
				case score.WorkNoteCommit:
					commits++
					if w.Phase == "after-pitch" && op.WorkSite != score.SiteEarlyCommit || w.Phase == "final" && op.WorkSite != score.SiteFinalCommit {
						t.Fatal("commit site reordered")
					}
				case score.WorkLoop:
					if w.First && w.Taken {
						firstTaken++
					} else if !w.First && w.Taken {
						repeatTaken++
					} else if !w.First && !w.Taken {
						repeatExit++
					}
				}
				return nil
			}, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return input.DAC, nil }})
			if err != nil {
				t.Fatal(test.ID, steps, err)
			}
			before, err := legacy.Advance(input)
			if err != nil || !reflect.DeepEqual(out, before) || !reflect.DeepEqual(d.Snapshot(), legacy.Snapshot()) {
				t.Fatal("metadata changed score behavior", test.ID, steps, err)
			}
			steps++
			frames++
		}
		z.Close()
		f.Close()
		if steps != test.Steps {
			t.Fatal("partial original parser replay")
		}
		cases++
	}
	if cases != 23 || frames != 122810 || tokens != 199477 || controls != 26130 || commits != 127691 || counts[score.WorkLoop] != 3388 || counts[score.WorkJump] != 942 || counts[score.WorkCall] != 263 || counts[score.WorkReturn] != 246 || firstTaken != 619 || repeatTaken != 2163 || repeatExit != 606 {
		t.Fatalf("source parser counts differ: cases%d frames%d reads%d controls%d commits%d work%v loops%d/%d/%d", cases, frames, tokens, controls, commits, counts, firstTaken, repeatTaken, repeatExit)
	}
	t.Logf("original parser: %d frames, %d reads, %d controls, %d commit sites", frames, tokens, controls, commits)
}
