package engine

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func TestOriginalFullResultPresentation(t *testing.T) {
	data, err := os.ReadFile("testdata/match-result.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame  int
		Ticks  uint8
		Sounds []uint8
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("testdata/match-result.ram.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	for _, row := range rows {
		var raw [4096]byte
		if _, err = io.ReadFull(z, raw[:]); err != nil {
			t.Fatal(err)
		}
		s, want := openingRAM(raw[:2048]), openingRAM(raw[2048:])
		got, err := StepOriginalResultPresentation(s, raw[4])
		if err != nil {
			t.Fatal(row.Frame, err)
		}
		got.State.Transition.Match.Clock.VideoRemainder += row.Ticks
		if !reflect.DeepEqual(got.State, want) {
			a, b := reflect.ValueOf(got.State), reflect.ValueOf(want)
			for i := 0; i < a.NumField(); i++ {
				if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
					t.Errorf("frame %d %s got %+v want %+v", row.Frame, a.Type().Field(i).Name, a.Field(i).Interface(), b.Field(i).Interface())
				}
			}
			t.FailNow()
		}
		if len(got.Transition.SoundIDs) != len(row.Sounds) || (len(row.Sounds) > 0 && !reflect.DeepEqual(got.Transition.SoundIDs, row.Sounds)) {
			t.Fatal(row.Frame, got.Transition.SoundIDs, row.Sounds)
		}
	}
	if len(rows) != 145 {
		t.Fatal("result coverage", len(rows))
	}
	t.Logf("matched %d full result calls, including all 580 actor animation/motion/projection updates", len(rows))
}

func TestResultActorScriptsAndConfirmation(t *testing.T) {
	for _, mode := range []uint8{0, 128} {
		for _, winner := range []uint8{0, 1} {
			s, _ := NewOriginalMatch(DefaultOriginalMatchConfiguration())
			s.Transition.Scene = 1
			s.Transition.Mode = mode
			s.Transition.Match.Phase = 6
			s.Transition.Match.Period = 2
			s.Transition.Scores[winner] = 12
			s.Projection = resetOriginalWorldDrawList(s.Projection)
			got, err := StepOriginalResultPresentation(s, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got.Transition.WinningTeam != winner || got.Transition.State.Match.Phase != 6 {
				t.Fatal(got)
			}
			for frame := 0; frame < 144; frame++ {
				got.State.Projection = resetOriginalWorldDrawList(got.State.Projection)
				got, err = StepOriginalResultPresentation(got.State, 0)
				if err != nil {
					t.Fatal(mode, winner, frame, err)
				}
			}
			if !got.Transition.FadeOut || got.State.Transition.Match.Phase == 6 {
				t.Fatal("result timer did not finish")
			}
		}
	}
}

func TestRuntimeResultOutcomeRestoreAndBoundary(t *testing.T) {
	for _, mode := range []uint8{0, 128} {
		for _, winner := range []uint8{0, 1} {
			for _, confirm := range []Buttons{0, ButtonA, ButtonStart} {
				c := DefaultRuntimeConfig()
				c.Match.Mode = mode
				s, err := NewRuntime(c)
				if err != nil {
					t.Fatal(err)
				}
				o := *s.Opening
				o.Transition.Match.Phase = 6
				o.Transition.Match.Period = 2
				o.Transition.Scores[winner] = 12
				// Cached display deliberately differs from live scores.
				o.Transition.Match.DisplayedScores = [2][3]uint8{{0, 0, 9}, {0, 0, 8}}
				o.Projection = resetOriginalWorldDrawList(o.Projection)
				s.Opening = nil
				s.Transition = &RuntimeTransitionState{Scene: o}
				s.LogicSteps = 1
				r, err := StepLogic(s, [4]ControllerFrame{})
				if err != nil {
					t.Fatal(mode, winner, err)
				}
				s = r.State
				if s.Outcome == nil || s.Outcome.Confirmed || s.Outcome.WinningTeam != winner || s.Outcome.Scores != o.Transition.Scores {
					t.Fatal("initial result must not be confirmed", s.Outcome)
				}
				for n := 0; n < 500 && !s.Outcome.Confirmed; n++ {
					raw, _ := json.Marshal(s)
					var restored RuntimeState
					if err = json.Unmarshal(raw, &restored); err != nil {
						t.Fatal(err)
					}
					if err = ValidateRuntimeState(restored); err != nil {
						t.Fatal("restore", err)
					}
					input := [4]Buttons{}
					if n == 10 {
						input[0] = confirm
					}
					a, e1 := StepVideo(s, input)
					b, e2 := StepVideo(restored, input)
					if e1 != nil || e2 != nil || !reflect.DeepEqual(a, b) {
						t.Fatal("result restore divergence", n, e1, e2)
					}
					s = a.State
				}
				if !s.Outcome.Confirmed {
					t.Fatal("result never confirmed")
				}
				if s.AwaitingNextPeriod {
					t.Fatal("final result must not use intermission restart")
				}
				if s.Display.Visible.Result == nil || s.Display.Visible.Result.Digits != o.Transition.Match.DisplayedScores || s.Display.Visible.Result.WinningTeam != winner {
					t.Fatal("missing result presentation", s.Display.Visible.Result)
				}
				s.WaitVideoFrames = 0
				before, _ := json.Marshal(s)
				failed, err := StepLogic(s, [4]ControllerFrame{})
				if err == nil {
					t.Fatal("unrestored destination must be explicit")
				}
				after, _ := json.Marshal(failed.State)
				if string(before) != string(after) {
					t.Fatal("failed boundary changed state")
				}
			}
		}
	}
}
