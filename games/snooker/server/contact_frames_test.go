package snooker

// Before collision keyframes: interpolating between two regular 30Hz samples
// can visually move a target before contact or bounce before reaching a cushion.
import (
	"math"
	"testing"
)

func TestEveryImpactHasAnExactTrajectoryKeyframe(t *testing.T) {
	m, e := Simulate([]Ball{testBall(0, .7, .7), testBall(1, .8, .7)}, Strike{Power: .8})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Events) < 2 {
		t.Fatal("missing collision and cushion fixture")
	}
	for _, event := range m.Events {
		found := false
		for _, frame := range m.Frames {
			if math.Abs(frame.Time-event.Time) < 1e-9 {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("impact between keyframes", event.Kind, event.Time)
		}
	}
	for i := 1; i < len(m.Frames); i++ {
		if m.Frames[i].Time <= m.Frames[i-1].Time {
			t.Fatal("duplicate or reversed frame time")
		}
	}
}
