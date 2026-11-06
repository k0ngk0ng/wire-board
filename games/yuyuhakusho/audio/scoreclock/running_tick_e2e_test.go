package scoreclock_test

import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/rootcontrol"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
	"reflect"
	"testing"
)

type runningNative struct {
	Input struct {
		ID              string
		Before          score.State
		Origin, Refresh uint64
	}
	Events []instrumentNativeEvent
	PSG    []struct {
		At    uint64
		Value uint8
	}
	End            scoreclock.Clock
	Priority       uint8
	Queue          [4]uint8
	TempoRemaining uint8 `json:"tempo_remaining"`
	TempoPeriod    uint8 `json:"tempo_period"`
	Equal          bool  `json:"native_observer_state_equal"`
}

func runningNatives(t *testing.T) []runningNative {
	t.Helper()
	var f struct {
		Version int
		Cases   []runningNative
	}
	if e := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-running-tick/native-running-tick-full.json.gz"), &f); e != nil {
		t.Fatal(e)
	}
	if f.Version != 1 || len(f.Cases) != 307 {
		t.Fatal("whole running-tick oracle missing")
	}
	return f.Cases
}
func TestRunningTickCompleteOriginalAndActualRoot(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	c := l.Catalog()
	for _, row := range runningNatives(t) {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("running observer changed native state")
			}
			d, err := score.Restore(c, row.Input.Before)
			if err != nil {
				t.Fatal(err)
			}
			legacy, err := score.Restore(c, row.Input.Before)
			if err != nil {
				t.Fatal(err)
			}
			root := rootcontrol.InitialState()
			root.Priority = 63
			queue := [4]uint8{}
			pause := uint8(0)
			e := newNoteVoiceIO(t, l, 0, 0)
			result, err := scoreclock.AdvanceRunningTick(scoreclock.Clock{Cycle: row.Input.Origin, Refresh: row.Input.Refresh}, &root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{}, c, l.Parser(), e, e, scoreclock.Limits{})
			if err != nil || result.Clock != row.End {
				t.Fatal("whole running driver clock differs", result.Clock, row.End, err)
			}
			wantTrace := noteAccesses(t, noteNative{Events: row.Events, PSG: row.PSG})
			if !reflect.DeepEqual(e.trace, wantTrace) {
				for i := 0; i < len(e.trace) && i < len(wantTrace); i++ {
					if e.trace[i] != wantTrace[i] {
						t.Fatalf("whole tick access%d got%+v want%+v", i, e.trace[i], wantTrace[i])
					}
				}
				t.Fatal("whole tick access count differs", len(e.trace), len(wantTrace))
			}
			var reads []instrumentNativeEvent
			for _, ev := range row.Events {
				if ev.Kind == 17 || ev.Kind == 19 {
					reads = append(reads, ev)
				}
			}
			next := 0
			feedback := func(read score.FeedbackRead) (score.DACFeedback, error) {
				if next >= len(reads) || reads[next].Kind != 17 {
					return score.DACFeedback{}, errors.New("native feedback marker missing")
				}
				marker := uint8(reads[next].Value)
				next++
				if read.Kind == score.ReadPanRedirect {
					return score.DACFeedback{PanRedirect: marker != 0}, nil
				}
				value := score.DACFeedback{Blocked: marker&128 != 0}
				if !value.Blocked {
					if next >= len(reads) || reads[next].Kind != 19 {
						return score.DACFeedback{}, errors.New("native feedback flags missing")
					}
					value.Priority = uint8(reads[next].Value) & 192
					next++
				}
				return value, nil
			}
			lr := rootcontrol.InitialState()
			lr.Priority = 63
			lq := [4]uint8{}
			lp := uint8(0)
			want, failure := rootcontrol.AdvanceRules(&lr, rootcontrol.RuleOwners{Score: legacy, Queue: &lq, AudioPause: &lp}, rootcontrol.RuleInput{}, rootcontrol.RuleHooks{Operation: func(rootcontrol.RuleOperation) error { return nil }, Score: score.ObservedHooks{Parser: l.Parser(), Operation: func(score.Operation) error { return nil }, ReadDAC: feedback}})
			if failure != nil || next != len(reads) || !reflect.DeepEqual(result.Output, want) || root != lr || queue != lq || d.Snapshot() != legacy.Snapshot() {
				t.Fatal("running clock changed canonical owners or feedback order", failure, next, len(reads))
			}
			if root.Priority != row.Priority || queue != row.Queue || d.Snapshot().TempoPeriod != row.TempoPeriod || d.Snapshot().TempoRemaining != row.TempoRemaining {
				t.Fatal("whole native Root/tempo result differs")
			}
		})
	}
}
func TestRunningTickRejectsUnpricedBoundariesAndRollsBack(t *testing.T) {
	l := observedEnvelopeLibrary(t)
	c := l.Catalog()
	row := runningNatives(t)[0]
	for _, mode := range []string{"queue", "selected-zero", "fade", "pause", "control"} {
		d, err := score.Restore(c, row.Input.Before)
		if err != nil {
			t.Fatal(err)
		}
		root := rootcontrol.InitialState()
		root.Priority = 63
		queue := [4]uint8{}
		pause := uint8(0)
		input := rootcontrol.RuleInput{}
		switch mode {
		case "queue":
			queue[0] = 6
		case "selected-zero":
			root.Selected = 0
		case "fade":
			root.Fade.Steps = 1
		case "pause":
			pause = 1
		case "control":
			input.Idle.ControlMode = 1
		}
		oldRoot, oldQueue, oldPause := root, queue, pause
		e := newNoteVoiceIO(t, l, 0, 0)
		_, err = scoreclock.AdvanceRunningTick(scoreclock.Clock{}, &root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, input, c, l.Parser(), e, e, scoreclock.Limits{})
		if err == nil || root != oldRoot || queue != oldQueue || pause != oldPause || d.Snapshot() != row.Input.Before || len(e.trace) != 0 {
			t.Fatal("unpriced full-tick boundary accepted or touched owners", mode, err)
		}
	}
	var witness runningNative
	for _, r := range runningNatives(t) {
		if r.Input.ID != "" && r.Input.Before.Voices[1].Active && r.Input.Before.Voices[0].Active {
			witness = r
			break
		}
	}
	if witness.Input.ID == "" {
		t.Fatal("whole-budget witness missing")
	}
	for _, failIO := range []bool{false, true} {
		d, err := score.Restore(c, witness.Input.Before)
		if err != nil {
			t.Fatal(err)
		}
		root := rootcontrol.InitialState()
		root.Priority = 63
		queue := [4]uint8{}
		pause := uint8(0)
		e := newNoteVoiceIO(t, l, 0, 0)
		e.failWrite = failIO
		limits := scoreclock.Limits{MaxOperations: 300}
		if failIO {
			limits = scoreclock.Limits{}
		}
		_, err = scoreclock.AdvanceRunningTick(scoreclock.Clock{}, &root, rootcontrol.RuleOwners{Score: d, Queue: &queue, AudioPause: &pause}, rootcontrol.RuleInput{}, c, l.Parser(), e, e, limits)
		if err == nil || !failIO && !errors.Is(err, scoreclock.ErrBudget) || d.Snapshot() != witness.Input.Before || len(e.trace) == 0 {
			t.Fatal("whole-tick failure committed score or reset nested budget", failIO, err)
		}
	}
}
