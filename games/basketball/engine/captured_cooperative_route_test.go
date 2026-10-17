package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Acceptance risks identified before the continuation: old Kind14 support
// rejection remains; fixing it loses attached jump ownership; AI contact is
// incorrectly suppressed to force five escapes; native input stops after the
// box breaks; restore restarts clock, input delay, or original AI decisions.
func TestCapturedCooperativeRecordedKind14Failure(t *testing.T) {
	f, err := os.Open("testdata/captured-cooperative-runtime-before.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var saved struct {
		State RuntimeState
		Held  [4]Buttons
		Error string
	}
	if err = json.NewDecoder(z).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if saved.State.VideoFrames != 845 || saved.Error != "actions: object 1: rider support object 4 requires its original kind" {
		t.Fatal("wrong recorded regression")
	}
	s := saved.State
	for s.VideoFrames < 1029 {
		held := saved.Held
		if s.VideoFrames >= 921 && s.VideoFrames < 939 {
			held[1] = ButtonRight
		}
		if s.VideoFrames >= 939 {
			held[1] = ButtonLeft
		}
		r, e := StepVideo(s, held)
		if e != nil {
			t.Fatalf("video%d: %v", s.VideoFrames, e)
		}
		s = r.State
	}
	t.Logf("recorded Kind14 rider failure continues naturally from845 to%d; no world changes", s.VideoFrames)
}

func TestRuntimeCourtFourCooperativeCaptureControllerRoute(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, 1, -1, -1}
	s, err := NewRuntime(c)
	if err != nil {
		t.Fatal(err)
	}
	var trace []struct {
		Video         uint64
		Before, After int8
		Count         uint8
		Action        Action
	}
	advance := func(frames int, a, b Buttons) {
		t.Helper()
		for i := 0; i < frames; i++ {
			var before int8 = -1
			if s.Playing != nil {
				before = s.Playing.World.Preparation.Actions.SpecialHolder
			}
			raw, _ := json.Marshal(s)
			var restored RuntimeState
			if e := json.Unmarshal(raw, &restored); e != nil {
				t.Fatal(e)
			}
			held := [4]Buttons{a, b}
			r, e := StepVideo(s, held)
			if e != nil {
				t.Fatalf("video%d: %v", s.VideoFrames, e)
			}
			replay, e := StepVideo(restored, held)
			if e != nil || !reflect.DeepEqual(r.State, replay.State) {
				t.Fatalf("restore replay video%d: %v", s.VideoFrames, e)
			}
			s = r.State
			if s.Playing != nil {
				w := s.Playing.World.Preparation.Actions
				if before != w.SpecialHolder {
					trace = append(trace, struct {
						Video         uint64
						Before, After int8
						Count         uint8
						Action        Action
					}{s.VideoFrames, before, w.SpecialHolder, w.CaptureCounter, w.Actors[1].Player.Action})
				}
			}
		}
	}
	advance(114, 0, 0)
	advance(21, ButtonUp, ButtonUp)
	advance(54, ButtonLeft, ButtonUp)
	advance(15, ButtonLeft, 0)
	advance(6, ButtonA, 0)
	advance(15, 0, 0)
	advance(6, ButtonRight|ButtonA, 0)
	advance(60, 0, 0)
	if s.Playing.World.Preparation.Actions.SpecialHolder != 1 {
		t.Fatal("original thrown box did not capture teammate")
	}
	for i := 0; i < 5; i++ {
		advance(6, 0, ButtonA|ButtonB)
		advance(120, 0, 0)
	}
	advance(18, 0, ButtonRight)
	if s.VideoFrames != 939 {
		t.Fatal("route changed", s.VideoFrames)
	}
	before := s.Playing.World.Preparation.Actions.Actors[1].Player.Body.Position
	advance(90, 0, ButtonLeft)
	after := s.Playing.World.Preparation.Actions.Actors[1].Player.Body.Position
	if before == after {
		t.Fatal("post-contact human input did not move actor")
	}
	if len(trace) < 2 || trace[0].After != 1 || trace[1].After != -1 || trace[1].Count == 255 {
		t.Fatal("expected original AI to interrupt box before fifth escape", trace)
	}
	t.Logf("original AI capture transitions%+v; normal human Left input video939..1029 moves%+v to%+v; every video JSON replay identical", trace, before, after)
}
