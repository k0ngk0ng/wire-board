package score_test

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func TestObservedOriginalTrajectories(t *testing.T) {
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
	if err = json.Unmarshal(raw, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Cases) != 42 {
		t.Fatal("original scope changed")
	}
	for _, test := range index.Cases {
		t.Run(test.ID, func(t *testing.T) {
			f, err := os.Open(filepath.Join("testdata", test.Path))
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
			var first originalFrame
			if err := decoder.Decode(&first); err != nil {
				t.Fatal(err)
			}
			d, err := score.New(c)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := score.New(c)
			if err != nil {
				t.Fatal(err)
			}
			frames, intents, operations := 0, 0, 0
			for {
				var frame originalFrame
				err := decoder.Decode(&frame)
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
				var sequence uint32
				offset := 0
				trace := []score.Intent{}
				reads := []score.FeedbackKind{}
				hooks := score.ObservedHooks{Operation: func(op score.Operation) error {
					if op.Sequence != sequence || op.IntentOffset != offset {
						return fmt.Errorf("noncanonical operation order/span: %+v expected sequence%d offset%d", op, sequence, offset)
					}
					sequence++
					if op.Kind == score.OpParserUnavailable && (!op.Position.Valid || op.Position.Index < 0) {
						return errors.New("unbound parser gap")
					}
					if op.Kind == score.OpPSGPitchWrite && (op.WriteCount != 2 || op.Writes[0].Kind != "psg" || op.Writes[1].Kind != "psg") {
						return errors.New("invalid PSG pitch group")
					}
					for n := 0; n < int(op.WriteCount); n++ {
						if op.Writes[n].Order != offset {
							return errors.New("incorrect write order")
						}
						trace = append(trace, op.Writes[n])
						offset++
					}
					return nil
				}, ReadDAC: func(read score.FeedbackRead) (score.DACFeedback, error) {
					reads = append(reads, read.Kind)
					return input.DAC, nil
				}}
				got, err := d.AdvanceObserved(score.ObservedInput{SoundID: input.SoundID}, hooks)
				if err != nil {
					t.Fatalf("frame%d: %v", frames, err)
				}
				want, err := legacy.Advance(input)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(d.Snapshot(), legacy.Snapshot()) {
					t.Fatalf("frame%d legacy behavior differs", frames)
				}
				if !reflect.DeepEqual(trace, got.Intents) && !(len(trace) == 0 && len(got.Intents) == 0) {
					t.Fatalf("frame%d operation effects do not flatten to output", frames)
				}
				if len(reads) > 2 {
					t.Fatalf("frame%d unexpected repeated feedback", frames)
				}
				equalJSON(t, fmt.Sprintf("observed frame%d", frames), d.View(), frame.State)
				if got.ClearPrimaryPriority != frame.ClearPriority {
					t.Fatal("priority summary changed")
				}
				if sequence == 0 {
					t.Fatal("no operations")
				}
				operations += int(sequence)
				intents += len(got.Intents)
				frames++
			}
			if frames != test.Steps {
				t.Fatal("incomplete original replay")
			}
			t.Logf("observed %d frames, %d ordered intents, %d temporary operations", frames, intents, operations)
		})
	}
}

func TestObservedFeedbackReadsAreStaged(t *testing.T) {
	c := catalog(t)
	d, err := score.New(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Advance(score.Input{SoundID: 0x88}); err != nil {
		t.Fatal(err)
	}
	before := d.Snapshot()
	snapshot, err := score.Restore(c, before)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := snapshot.Advance(score.Input{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasIntent(stale.Intents, "dac") {
		t.Fatal("counterexample must start with an accepted stale DAC request")
	}
	// These values stand for detached bus/PCM owners. The authoritative copy is
	// untouched while callbacks advance the working copy through the same tick.
	var authorityMailbox uint8
	workingMailbox := authorityMailbox
	reads := []score.FeedbackKind{}
	got, err := d.AdvanceObserved(score.ObservedInput{}, score.ObservedHooks{
		Operation: func(op score.Operation) error {
			for n := 0; n < int(op.WriteCount); n++ {
				write := op.Writes[n]
				if write.Kind == "ym" && write.Bank == 1 && write.Register == 0xb6 {
					workingMailbox = 0x80
				}
			}
			return nil
		}, ReadDAC: func(read score.FeedbackRead) (score.DACFeedback, error) {
			reads = append(reads, read.Kind)
			return score.DACFeedback{Blocked: workingMailbox&0x80 != 0, PanRedirect: workingMailbox != 0}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reads, []score.FeedbackKind{score.ReadPanRedirect, score.ReadBlockedPriority}) {
		t.Fatalf("read stages %v", reads)
	}
	if hasIntent(got.Intents, "dac") || !hasIntent(got.Intents, "ym") {
		t.Fatal("later arbitration reused stale entry feedback")
	}
	if authorityMailbox != 0 || workingMailbox != 0x80 {
		t.Fatal("detached transaction example invalid")
	}
	authorityMailbox = workingMailbox
	if authorityMailbox != 0x80 {
		t.Fatal("successful transaction not committed")
	}
}

func hasIntent(intents []score.Intent, kind string) bool {
	for _, intent := range intents {
		if intent.Kind == kind {
			return true
		}
	}
	return false
}

func TestObservedPriorityWritesAreAtOriginalPositions(t *testing.T) {
	c := catalog(t)
	for _, test := range []struct {
		name    string
		sound   uint8
		step    int
		offsets []int
	}{
		{"fm-keyoff-before-restore", 0xa7, 80, []int{12, 39}},
		{"psg-keyoff-before-noise", 0xa1, 94, []int{7}},
	} {
		t.Run(test.name, func(t *testing.T) {
			d, _ := score.New(c)
			var clears []int
			for step := 0; step <= test.step; step++ {
				id := uint8(0)
				if step == 0 {
					id = 0x82
				}
				if step == 40 {
					id = test.sound
				}
				_, err := d.AdvanceObserved(score.ObservedInput{SoundID: id}, score.ObservedHooks{Operation: func(op score.Operation) error {
					if step == test.step && op.Kind == score.OpPriorityClear && op.Branch == score.BranchPrimaryTrackStop {
						clears = append(clears, op.IntentOffset)
					}
					return nil
				}, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }})
				if err != nil {
					t.Fatal(err)
				}
			}
			if !reflect.DeepEqual(clears, test.offsets) {
				t.Fatalf("clear at %v expected%v", clears, test.offsets)
			}
		})
	}
	// Save and restore are distinct external-owner effects, even when an
	// interleaved actor changes priority between StopPrimary and music reset.
	d, _ := score.New(c)
	priority := uint8(0x22)
	latch := uint8(0)
	saved, restored := false, false
	_, err := d.AdvanceObserved(score.ObservedInput{SoundID: 0x82}, score.ObservedHooks{Operation: func(op score.Operation) error {
		switch op.Kind {
		case score.OpPriorityClear:
			priority = 0
		case score.OpPrioritySave:
			latch = priority
			saved = true
		case score.OpPriorityRestore:
			priority = latch
			restored = true
		}
		if op.WriteCount == 1 && op.Writes[0].Kind == "ym" && op.Writes[0].Register == 0x27 && op.IntentOffset == 1 {
			priority = 0x55
		}
		return nil
	}, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }})
	if err != nil || !saved || !restored || priority != 0x55 {
		t.Fatalf("priority latch lost: %x %v %v %v", priority, saved, restored, err)
	}
}

func TestObservedErrorsDoNotCommitScoreOrRepeatCallbacks(t *testing.T) {
	c := catalog(t)
	sentinel := errors.New("detached service failed")
	for _, stage := range []string{"callback", "feedback", "invalid-feedback", "budget", "parser-missing"} {
		t.Run(stage, func(t *testing.T) {
			d, _ := score.New(c)
			if _, err := d.Advance(score.Input{SoundID: 0x88}); err != nil {
				t.Fatal(err)
			}
			before := d.Snapshot()
			calls := 0
			failed := false
			detachedChanges := 0
			hooks := score.ObservedHooks{Operation: func(op score.Operation) error {
				if failed {
					t.Fatal("callback repeated after failure")
				}
				calls++
				detachedChanges++
				if stage == "callback" && calls == 3 {
					failed = true
					return sentinel
				}
				if stage == "parser-missing" && op.Kind == score.OpParserUnavailable {
					failed = true
					return sentinel
				}
				return nil
			}, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) {
				if failed {
					t.Fatal("read repeated after failure")
				}
				if stage == "feedback" {
					failed = true
					return score.DACFeedback{}, sentinel
				}
				if stage == "invalid-feedback" {
					return score.DACFeedback{Priority: 1}, nil
				}
				return score.DACFeedback{}, nil
			}}
			if stage == "budget" {
				hooks.MaxOperations = 2
			}
			output, err := d.AdvanceObserved(score.ObservedInput{}, hooks)
			if err == nil || len(output.Intents) != 0 || output.ClearPrimaryPriority || !reflect.DeepEqual(d.Snapshot(), before) {
				t.Fatal("failed transaction partially committed score")
			}
			if detachedChanges == 0 {
				t.Fatal("test must demonstrate external effects were not automatically rolled back")
			}
		})
	}
}

func TestObservedRecordsHaveNoMutableOutputAlias(t *testing.T) {
	c := catalog(t)
	d, _ := score.New(c)
	d.Advance(score.Input{SoundID: 0x82})
	output, err := d.AdvanceObserved(score.ObservedInput{}, score.ObservedHooks{Operation: func(op score.Operation) error {
		for n := 0; n < int(op.WriteCount); n++ {
			if op.Writes[n].Pan != nil {
				op.Writes[n].Pan.Left = false
				op.Writes[n].Pan.Right = false
			}
		}
		return nil
	}, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	for _, intent := range output.Intents {
		if intent.Kind == "dac" && (intent.Pan == nil || !intent.Pan.Left || !intent.Pan.Right) {
			t.Fatal("hook mutated score output")
		}
	}
}
