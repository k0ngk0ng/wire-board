package score_test

import (
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/audio/score"
)

func TestObservedTempoWorkComesFromCurrentState(t *testing.T) {
	c := catalog(t)
	d, _ := score.New(c)
	for step := 0; step <= 9; step++ {
		id := uint8(0)
		if step == 0 {
			id = 0x83
		}
		before := d.Snapshot()
		seen := false
		_, err := d.AdvanceObserved(score.ObservedInput{SoundID: id}, score.ObservedHooks{Operation: func(op score.Operation) error {
			if op.Kind != score.OpEnter || op.Routine != score.RoutineTempo {
				return nil
			}
			if seen {
				t.Fatal("tempo entry repeated")
			}
			seen = true
			enabled := before.TempoPeriod != 0
			reload := enabled && before.TempoRemaining == 1
			mask := uint16(0)
			if reload {
				for slot := 0; slot < 10; slot++ {
					if before.Voices[slot].Active {
						mask |= 1 << slot
					}
				}
			}
			if op.Tempo.Enabled != enabled || op.Tempo.Reload != reload || op.Tempo.ActiveMusicMask != mask || op.Tempo.Period != before.TempoPeriod || op.Tempo.Remaining != before.TempoRemaining {
				t.Fatalf("step%d tempo work %+v vs current state", step, op.Tempo)
			}
			if step == 9 && (!reload || mask != 0x3bf) {
				t.Fatal("source10-slot reload boundary not reached")
			}
			return nil
		}, ReadDAC: func(score.FeedbackRead) (score.DACFeedback, error) { return score.DACFeedback{}, nil }})
		if err != nil || !seen {
			t.Fatal(step, err, seen)
		}
	}
}
