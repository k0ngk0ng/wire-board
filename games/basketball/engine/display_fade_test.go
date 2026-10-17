package engine

// Failure scenarios, before implementation: full colors flash before opening;
// fade levels commit early or are lost without sprite submissions; stale staged
// snapshots reset committed colors; zero disappears from JSON; restore skips a
// level; pending queue replacement changes the original input; invalid levels or
// overlapping fades pass validation; result enters while still black.
import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func fadeLevel(v *uint8) uint8 {
	if v == nil {
		return 4
	}
	return *v
}
func TestDisplayFadeOriginalVideoBoundaries(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Name                  string
			Out                   bool
			Palette, InitialLevel uint8
			Levels                []uint8
			QueueNMIs, CommitNMIs []uint64
		}
	}
	raw, e := os.ReadFile("testdata/palette-fade-video.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(raw, &fixture); e != nil {
		t.Fatal(e)
	}
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			initial := c.InitialLevel
			s := DisplayState{CourtEpoch: 1, PaletteID: c.Palette, PaletteLevel: &initial, HasVisible: true, Visible: BasketballRenderSnapshot{Court: 4, PaletteID: c.Palette, PaletteLevel: &initial}}
			before, _ := json.Marshal(s)
			queued, e := QueueDisplayFade(s, c.Out)
			if e != nil {
				t.Fatal(e)
			}
			after, _ := json.Marshal(s)
			if string(before) != string(after) || fadeLevel(queued.Visible.PaletteLevel) != initial {
				t.Fatal("queue changed visible/input")
			}
			if _, e = QueueDisplayFade(queued, c.Out); e == nil {
				t.Fatal("overlapping fade accepted")
			}
			s = queued
			for n, want := range c.Levels {
				serialized, _ := json.Marshal(s)
				var restored DisplayState
				if e = json.Unmarshal(serialized, &restored); e != nil {
					t.Fatal(e)
				}
				next, e := CommitDisplayVideo(s, 0, 0)
				if e != nil {
					t.Fatal(e)
				}
				replay, e := CommitDisplayVideo(restored, 0, 0)
				if e != nil || !reflect.DeepEqual(next, replay) {
					t.Fatal("restored fade diverged", n, e)
				}
				if fadeLevel(next.PaletteLevel) != want || fadeLevel(next.Visible.PaletteLevel) != want || next.Visible.PaletteID != c.Palette {
					t.Fatalf("video %d want level%d got %+v", n+1, want, next)
				}
				if n%2 == 0 && c.CommitNMIs[n/2]-c.QueueNMIs[0] != uint64(n+1) {
					t.Fatal("source NMI spacing changed")
				}
				s = next
			}
			if s.PaletteFade != nil || s.PendingPaletteLevel != nil {
				t.Fatal("finished fade retains pending state")
			}
			stable, e := CommitDisplayVideo(s, 0, 0)
			if e != nil || !reflect.DeepEqual(s, stable) {
				t.Fatal("completed colors changed")
			}
			if c.Out {
				raw, _ = json.Marshal(s.Visible)
				var v map[string]any
				_ = json.Unmarshal(raw, &v)
				if v["paletteLevel"] != float64(0) {
					t.Fatal("black omitted from JSON")
				}
			}
			// A newly staged old image must use the committed level at transfer.
			s.HasSubmission = true
			s.Submitted = BasketballRenderSnapshot{Court: 4}
			next, e := CommitDisplayVideo(s, 0, 0)
			if e != nil || fadeLevel(next.Visible.PaletteLevel) != c.Levels[7] {
				t.Fatal("staged snapshot overwrote fade")
			}
		})
	}
}
func TestDisplayFadeInvalidStateIsAtomic(t *testing.T) {
	s := DisplayState{CourtEpoch: 1}
	for _, level := range []uint8{5, 255} {
		got, e := QueueDisplayPaletteLevel(s, 1, level)
		if e == nil || !reflect.DeepEqual(s, got) {
			t.Fatal("invalid level queue not atomic")
		}
		for i := 0; i < 4; i++ {
			bad := s
			v := level
			switch i {
			case 0:
				bad.PaletteLevel = &v
			case 1:
				bad.PendingPaletteLevel = &v
			case 2:
				bad.Visible.PaletteLevel = &v
			case 3:
				bad.Submitted.PaletteLevel = &v
			}
			if validateRuntimeDisplay(bad) == nil {
				t.Fatal("invalid saved fade accepted", i)
			}
		}
	}
	bad := s
	bad.PaletteFade = &DisplayPaletteFade{Frame: 8}
	if validateRuntimeDisplay(bad) == nil {
		t.Fatal("invalid fade cursor accepted")
	}
	for _, frame := range []uint8{0, 2, 4, 6} {
		bad = s
		bad.PaletteFade = &DisplayPaletteFade{Frame: frame}
		if validateRuntimeDisplay(bad) == nil {
			t.Fatal("missing queued fade level accepted", frame)
		}
	}
	bad = s
	bad.PaletteFade = &DisplayPaletteFade{Frame: 1}
	bad.PaletteLevel = displayPaletteLevel(4)
	if validateRuntimeDisplay(bad) == nil {
		t.Fatal("inconsistent committed fade level accepted")
	}
}
func TestRuntimeOpeningUsesOriginalFadeLevels(t *testing.T) {
	for _, court := range []uint8{0, 1, 2, 3, 4} {
		c := DefaultRuntimeConfig()
		c.Match.Court = court
		s, e := NewRuntime(c)
		if e != nil {
			t.Fatal(e)
		}
		var started bool
		for frame := 0; frame < 100; frame++ {
			r, e := StepVideo(s, [4]Buttons{})
			if e != nil {
				t.Fatal(e)
			}
			s = r.State
			if r.FadeIn {
				started = true
				break
			}
			if fadeLevel(s.Display.PaletteLevel) != 0 {
				t.Fatalf("court%d flashes before opening", court)
			}
		}
		if !started {
			t.Fatal("no opening fade")
		}
		steps := s.LogicSteps
		for n, want := range []uint8{1, 1, 2, 2, 3, 3, 4, 4} {
			r, e := StepVideo(s, [4]Buttons{})
			if e != nil {
				t.Fatal(e)
			}
			s = r.State
			if fadeLevel(s.Display.Visible.PaletteLevel) != want || s.LogicSteps != steps {
				t.Fatal("opening wait/fade mismatch", court, n, want)
			}
		}
	}
}
func TestRuntimePeriodAndResultFadeBoundaries(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 4
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	o := *s.Opening
	o.Transition.Match.Phase = 4
	o.Transition.Match.Period = 1
	o.Transition.Scores = [2]uint16{12, 24}
	s.Opening = nil
	s.Transition = &RuntimeTransitionState{Scene: o}
	s.Display.PaletteID = 9
	s.Display.PaletteLevel = nil
	s.Display.HasVisible = true
	s.Display.Visible = BasketballRenderSnapshot{Court: 4, PaletteID: 9}
	r, e := StepLogic(s, [4]ControllerFrame{})
	if e != nil {
		t.Fatal(e)
	}
	s = r.State
	for _, want := range []uint8{3, 3, 2, 2, 1, 1, 0, 0} {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
		if fadeLevel(s.Display.Visible.PaletteLevel) != want {
			t.Fatal("period fade", want)
		}
	}
	for n := 0; n < 10 && s.Outcome == nil; n++ {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
	}
	if s.Outcome == nil || s.Display.PaletteFade == nil {
		t.Fatal("result did not schedule its original fade-in")
	}
	for _, want := range []uint8{1, 1, 2, 2, 3, 3, 4, 4} {
		r, e = StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(e)
		}
		s = r.State
		if fadeLevel(s.Display.Visible.PaletteLevel) != want || s.Display.Visible.PaletteID != 9 || s.Display.Visible.Result == nil {
			t.Fatal("result fade lost result art/retained palette", want)
		}
	}
}
