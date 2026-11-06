package scoreclock_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score/param"
	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/scoreclock"
)

func frequencyPanAccesses(t *testing.T, events []instrumentNativeEvent, ports []struct {
	At    uint64
	Value uint8
}) []access {
	t.Helper()
	out := instrumentAccesses(t, events)
	registers := [2]uint8{}
	for i := range out {
		a := &out[i]
		if a.Kind == "write" {
			if a.Port%2 == 0 {
				registers[a.Port/2] = a.Value
			} else if r := registers[a.Port/2]; r >= 0xa4 && r <= 0xae {
				a.Value &= 63
			}
		}
	}
	for _, w := range ports {
		out = append(out, access{Kind: "psg", At: w.At, Value: w.Value})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}

func TestFrequencyPanOriginalCallersAndActualOwners(t *testing.T) {
	var f struct {
		Version int
		Cases   []struct {
			Input struct {
				ID, Kind        string
				Channel         uint8
				Word            uint16
				Mode            uint8
				OnNote          bool `json:"on_note"`
				NoteOff         bool `json:"note_off"`
				Claimed         bool
				Definition      param.PanDefinition
				State           param.PanState
				After           param.PanState
				Output          *param.Stereo
				Origin, Refresh uint64
			}
			Events []instrumentNativeEvent
			PSG    []struct {
				At    uint64
				Value uint8
			}
			End           scoreclock.Clock
			Step, Elapsed uint8
			Equal         bool `json:"native_observer_state_equal"`
		}
	}
	if err := json.Unmarshal(parserClockUnpack(t, "../../verification/score-clock-frequency-pan/native-frequency-pan-full.json.gz"), &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != 1 || len(f.Cases) != 128 {
		t.Fatal("native frequency/pan scope incomplete")
	}
	library := observedEnvelopeLibrary(t)
	for _, row := range f.Cases {
		t.Run(row.Input.ID, func(t *testing.T) {
			if !row.Equal {
				t.Fatal("native observer changed source state")
			}
			owner, _, _ := newPauseMixed(t)
			if _, err := owner.r.RequestBus(0, true); err != nil {
				t.Fatal(err)
			}
			e := &keyIO{&instrumentIO{r: owner.r}}
			in := row.Input
			clock := scoreclock.Clock{Cycle: in.Origin, Refresh: in.Refresh}
			var end scoreclock.Clock
			var err error
			if in.Kind == "frequency" {
				tx, failure := scoreclock.BeginFrequencyTransfer(clock, scoreclock.FrequencyRequest{Channel: in.Channel, Word: in.Word}, e, e, scoreclock.Limits{})
				if failure != nil {
					t.Fatal(failure)
				}
				if len(e.trace) != 0 {
					t.Fatal("frequency preparation emitted early hardware")
				}
				for _, effect := range tx.ExpectedEffects() {
					if err = tx.WriteNext(effect); err != nil {
						t.Fatal(err)
					}
				}
				end, err = tx.Finish()
				if _, failure = tx.Finish(); !errors.Is(failure, scoreclock.ErrFrequencySequence) {
					t.Fatal("frequency transfer reused")
				}
			} else {
				step, failure := library.Catalog().PanAutomationStep(in.Definition, in.State, in.OnNote, in.NoteOff)
				if failure != nil || step.State != in.After || !reflect.DeepEqual(step.Pan, in.Output) {
					t.Fatal("canonical pan step differs from original input projection", step, in.After, failure)
				}
				if step.State.Step != row.Step || step.State.Elapsed != row.Elapsed {
					t.Fatal("canonical pan counters differ from source result")
				}
				tx, failure := scoreclock.BeginPanAutomation(clock, scoreclock.PanAutomationRequest{Channel: in.Channel, Definition: in.Definition, State: in.State, OnNote: in.OnNote, NoteOff: in.NoteOff, Claimed: in.Claimed, Pan: panValue(181)}, library.Catalog(), e, e, scoreclock.Limits{})
				if failure != nil {
					t.Fatal(failure)
				}
				if len(e.trace) != 0 {
					t.Fatal("automation preparation emitted early hardware")
				}
				if effect, ok := tx.ExpectedEffect(); ok {
					if err = tx.Apply(effect); err != nil {
						t.Fatal(err)
					}
				}
				end, err = tx.Finish()
				if _, failure = tx.Finish(); !errors.Is(failure, scoreclock.ErrPanAutomationSequence) {
					t.Fatal("pan automation reused")
				}
			}
			if err != nil || end != row.End {
				t.Fatal("original frequency/pan final clock differs", end, row.End, err)
			}
			want := frequencyPanAccesses(t, row.Events, row.PSG)
			if !reflect.DeepEqual(e.trace, want) {
				t.Fatal("independent mixed frequency/pan IO differs", e.trace, want)
			}
		})
	}
}

func TestFrequencyPanActualScoreWritesAndStateOwnership(t *testing.T) {
	library := observedEnvelopeLibrary(t)
	c := library.Catalog()
	before := observedEnvelopeState(t, c, 1)
	before.Voices[1].Pan = panValue(181)
	before.Voices[1].PanDefinition = param.PanDefinition{Mode: param.PanRestartEveryDriverTick, Pattern: "right-left-right-center", StepLimit: 3, Period: 1}
	before.Voices[1].PanState = param.InitialPanState(before.Voices[1].PanDefinition)
	d, err := score.Restore(c, before)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := score.Restore(c, before)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, _ := newPauseMixed(t)
	e := &keyIO{&instrumentIO{r: owner.r}}
	var tx *score.TickTransaction
	var pan *scoreclock.PanAutomationTransfer
	var clock scoreclock.Clock
	writes := 0
	tx, err = d.BeginTickObserved(score.ObservedHooks{Operation: func(op score.Operation) error {
		if op.Kind == score.OpEnter && op.Routine == score.RoutinePan {
			state, err := tx.Snapshot()
			if err != nil {
				return err
			}
			v := state.Voices[1]
			pan, err = scoreclock.BeginPanAutomation(clock, scoreclock.PanAutomationRequest{Channel: 0, Definition: v.PanDefinition, State: v.PanState, OnNote: false, NoteOff: v.NoteOff, Claimed: v.Claimed, Pan: v.Pan}, c, e, e, scoreclock.Limits{})
			return err
		}
		if op.Kind == score.OpChipWrite && pan != nil {
			expected, ok := pan.ExpectedEffect()
			if !ok || expected.Kind != "ym" || op.Writes[0].Value != expected.Write.Value {
				return errors.New("canonical automatic pan intent differs")
			}
			writes++
			return pan.Apply(expected)
		}
		if op.Kind == score.OpLeave && op.Routine == score.RoutinePan {
			var err error
			clock, err = pan.Finish()
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
	want, fail := legacy.Advance(score.Input{})
	if err != nil || fail != nil || writes != 1 || !reflect.DeepEqual(out, want) || d.Snapshot() != legacy.Snapshot() || d.Snapshot().Voices[1].Pan != panValue(181) {
		t.Fatal("automation consumer changed/duplicated persistent state", writes, err, fail)
	}
}

func TestFrequencyPanValidationBudgetAndPoisoning(t *testing.T) {
	owner, _, _ := newPauseMixed(t)
	e := &keyIO{&instrumentIO{r: owner.r}}
	library := observedEnvelopeLibrary(t)
	if _, err := scoreclock.BeginFrequencyTransfer(scoreclock.Clock{}, scoreclock.FrequencyRequest{Channel: 3}, e, e, scoreclock.Limits{}); err == nil {
		t.Fatal("unknown frequency channel")
	}
	if _, err := scoreclock.BeginFrequencyTransfer(scoreclock.Clock{}, scoreclock.FrequencyRequest{FM3Mode: true}, e, e, scoreclock.Limits{}); err == nil {
		t.Fatal("unpriced FM3 mode accepted")
	}
	def := param.PanDefinition{Mode: param.PanRestartEveryDriverTick, Pattern: "right-left-right-center", Period: 1, StepLimit: 3}
	tx, err := scoreclock.BeginPanAutomation(scoreclock.Clock{}, scoreclock.PanAutomationRequest{Channel: 0, Definition: def, State: param.InitialPanState(def), Pan: panValue(181)}, library.Catalog(), e, e, scoreclock.Limits{MaxOperations: 40})
	if err != nil {
		t.Fatal(err)
	}
	if effect, ok := tx.ExpectedEffect(); !ok {
		t.Fatal("counterexample pan effect absent")
	} else if err = tx.Apply(effect); !errors.Is(err, scoreclock.ErrBudget) {
		t.Fatal("nested pan/YM reset automation budget", err)
	}
	if _, err = tx.Finish(); !errors.Is(err, scoreclock.ErrPanAutomationSequence) {
		t.Fatal("failed automation continued")
	}
	freq, err := scoreclock.BeginFrequencyTransfer(scoreclock.Clock{}, scoreclock.FrequencyRequest{}, e, e, scoreclock.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = freq.Finish(); !errors.Is(err, scoreclock.ErrFrequencySequence) {
		t.Fatal("premature frequency finish")
	}
	if err = freq.WriteNext(freq.ExpectedEffects()[0]); !errors.Is(err, scoreclock.ErrFrequencySequence) {
		t.Fatal("poisoned frequency continued")
	}
}
