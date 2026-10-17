package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Investigative recorder, not a gameplay success gate. It identifies the first
// original-seed interval with a free, motionless airborne rest ball. No state
// repair, clock shortening or alternative AI is permitted in this recorder.
func TestBallRestStallDiagnostic(t *testing.T) {
	output := os.Getenv("BASKETBALL_REST_STALL_OUTPUT")
	if output == "" {
		t.Skip("opt-in original team5 stalled-ball recorder")
	}
	config := DefaultRuntimeConfig()
	config.Match.Court = 1
	config.Match.Teams = [2]uint8{0, 5}
	config.Match.Characters = [4]uint8{0, 1, 0, 1}
	config.Match.Seed = 0x12345678
	config.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	if source := os.Getenv("BASKETBALL_REST_STALL_SOURCE"); source != "" {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		var retained RuntimeState
		if err = json.Unmarshal(data, &retained); err != nil {
			t.Fatal(err)
		}
		config = retained.Config
	}
	s, e := NewRuntime(config)
	if e != nil {
		t.Fatal(e)
	}
	type sample struct {
		Video  uint64
		Before json.RawMessage
		After  json.RawMessage
	}
	var history []sample
	var first json.RawMessage
	stable := 0
	var previous Ball
	for frame := 0; frame < 48000; frame++ {
		before, _ := json.Marshal(s)
		var held [4]Buttons
		if s.AwaitingNextPeriod && s.WaitVideoFrames == 0 {
			held[0] = ButtonStart
		}
		step, e := StepVideo(s, held)
		if e != nil {
			t.Fatal(frame, e)
		}
		s = step.State
		if !step.LogicAdvanced || s.Playing == nil {
			continue
		}
		after, _ := json.Marshal(s)
		history = append(history, sample{Video: s.VideoFrames, Before: before, After: after})
		if len(history) > 50 {
			history = history[1:]
		}
		w := s.Playing.World.Preparation.Actions
		b := w.Ball
		if w.Holder == 6 && b.Action == BallRest && b.Airborne && b.Body.Position.Height > 0 && b.Body.Gravity == 0 && b.Body.Velocity == (Velocity{}) {
			if stable == 0 || b.Body != previous.Body {
				first = before
				stable = 0
			}
			stable++
			previous = b
			if stable >= 12 {
				if e = os.MkdirAll(output, 0755); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(output, "first-before.json"), first, 0644); e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(output, "stalled.json"), after, 0644); e != nil {
					t.Fatal(e)
				}
				data, _ := json.Marshal(history)
				if e = os.WriteFile(filepath.Join(output, "history.json"), data, 0644); e != nil {
					t.Fatal(e)
				}
				t.Logf("recorded first persistent airborne-rest interval video=%d logic=%d ball=%+v", s.VideoFrames, s.LogicSteps, b)
				return
			}
		} else {
			stable = 0
		}
	}
	t.Fatal("recorded configuration no longer reproduces suspended rest ball; inspect changes before replacing evidence")
}
