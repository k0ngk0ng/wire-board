package contra

// Failures before implementation: using just static CHR presets; updating all
// pages on a one-slot transition; reloading timer before decrement/high-bit gate;
// phase zero not wrapping to three; disabled stage/Boss still animating; lair
// pace uses incremented A instead of old A; special row eight reloads timer;
// accidentally changing the currently displayed pages instead of desired pages.

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type chrObservedState struct {
	Stage, SceneType, Transition, Timer, Phase, Pace, Alternate, Random, Screen, Counter uint8
	Desired                                                                              [8]uint8
}
type chrObservedCall struct {
	Frame         int
	Before, After chrObservedState
}

func checkCHRCall(t *testing.T, c chrObservedCall) {
	t.Helper()
	b := c.Before
	s := CHRAnimation{Timer: b.Timer, Phase: b.Phase, Pace: b.Pace}
	transition := b.Transition
	desired := b.Desired
	context := CHRAnimationContext{Level: b.Stage, SceneType: b.SceneType, Alternate: b.Alternate, Random: b.Random, Screen: b.Screen, Counter: b.Counter}
	if err := s.Step(context, &transition, &desired); err != nil {
		t.Fatal(c.Frame, err)
	}
	got := b
	got.Timer, got.Phase, got.Pace = s.Timer, s.Phase, s.Pace
	got.Transition = transition
	got.Desired = desired
	if got != c.After {
		t.Fatalf("frame %d original CHR differs\ngot %+v\nwant %+v\nbefore %+v", c.Frame, got, c.After, c.Before)
	}
}
func readCHROracle(t *testing.T, name string, value any) {
	t.Helper()
	f, err := os.Open("../../../docs/research/nes-contra/evidence/" + name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err = json.NewDecoder(z).Decode(value); err != nil {
		t.Fatal(err)
	}
}
func TestOriginalCHRAnimation(t *testing.T) {
	var d struct {
		Stages []struct {
			Stage          int
			AnimationCalls []chrObservedCall
		}
	}
	readCHROracle(t, "levels-chr-original.json.gz", &d)
	if len(d.Stages) != 8 {
		t.Fatal("missing original stage animation")
	}
	checked := 0
	for _, stage := range d.Stages {
		if len(stage.AnimationCalls) != 256 {
			t.Fatal("incomplete stage trace", stage.Stage)
		}
		for _, call := range stage.AnimationCalls {
			checkCHRCall(t, call)
			checked++
		}
	}
	if checked != 2048 {
		t.Fatal(checked)
	}
}
func TestOriginalCHRAnimationBoundaries(t *testing.T) {
	var d struct {
		Observations []struct {
			Name  string
			Calls []chrObservedCall
		}
	}
	readCHROracle(t, "levels-chr-cases-original.json.gz", &d)
	if len(d.Observations) != 14 {
		t.Fatal("missing staged branch probes")
	}
	checked := 0
	for _, group := range d.Observations {
		for _, call := range group.Calls {
			checkCHRCall(t, call)
			checked++
		}
	}
	if checked != 28 {
		t.Fatal(checked)
	}
}
func TestCHRDisplayBoundaryAndAtomicFailure(t *testing.T) {
	display := PatternPages{Desired: [8]uint8{1, 4, 6, 7, 68, 69, 70, 71}}
	display.Commit()
	before := display.Applied
	state := CHRAnimation{Phase: 3}
	transition := uint8(0)
	if err := state.Step(CHRAnimationContext{Level: 0, Random: 1}, &transition, &display.Desired); err != nil {
		t.Fatal(err)
	}
	if display.Applied != before || display.Desired == before {
		t.Fatal("desired CHR leaked into current display")
	}
	display.Commit()
	if display.Applied != display.Desired {
		t.Fatal("next display did not commit desired CHR")
	}
	wantState, wantDisplay := state, display
	if err := state.Step(CHRAnimationContext{Level: 8}, &transition, &display.Desired); err == nil {
		t.Fatal("unmapped game stage accepted")
	}
	if state != wantState || !reflect.DeepEqual(display, wantDisplay) {
		t.Fatal("failed CHR step changed scene")
	}
}
