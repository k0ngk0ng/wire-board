package engine

// Before opening the last court: reject a NewYork fallback; preserve original
//13/14 props and camera; do not invent animation/RNG/score updates for A32C;
// save/replay must agree and out-of-range courts remain rejected. Full gates
// retain the original seed, clock, inputs and two independent repeats.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRuntimeCourtSevenColdBindingAndNoAnimation(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 7
	c.Match.Seed = 0x12345678
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; s.Playing == nil && n < 500; n++ {
		next, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		s = next.State
	}
	if s.Playing == nil || s.Playing.Context.Actions.CourtID != 7 || s.Playing.Context.ObjectKinds[4] != 13 || s.Playing.Context.ObjectKinds[5] != 14 {
		t.Fatal("original Court7 opening/props missing")
	}
	before, _ := json.Marshal(s)
	sounds, e := observeRuntimeCourtAnimation(&s, &s.Playing.World, s.Playing.Context, s.Playing.Match)
	after, _ := json.Marshal(s)
	if e != nil || len(sounds) != 0 || string(before) != string(after) {
		t.Fatal("Court7 A32C must be no-op", e)
	}
	var restored RuntimeState
	_ = json.Unmarshal(before, &restored)
	a, e := StepVideo(s, [4]Buttons{ButtonLeft})
	b, f := StepVideo(restored, [4]Buttons{ButtonLeft})
	if e != nil || f != nil || !reflect.DeepEqual(a, b) {
		t.Fatal("cold Court7 replay differs", e, f)
	}
	if !a.State.Display.HasVisible || a.State.Display.Visible.Court != 7 || a.State.Display.Visible.PaletteID != 8 {
		t.Fatal("court selector/base palette lost")
	}
	c.Match.Court = 8
	if _, e = NewRuntime(c); e == nil {
		t.Fatal("out-of-range court accepted")
	}
}
func TestRuntimeCourtSevenColdCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 7
	c.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtSevenAllComputerCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 7
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtSevenCooperativeCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 7
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, 1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtSevenVersusCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 7
	c.Match.Mode = 128
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{0, -1, 2, -1}
	verifyColdVideoRuntime(t, c, true)
}
