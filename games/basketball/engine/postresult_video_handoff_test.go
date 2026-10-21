package engine

import (
	"reflect"
	"testing"
)

func effectivePostResultEffects(original []PostResultVideoEffect) []PostResultVideoEffect {
	out := append([]PostResultVideoEffect(nil), original...)
	for i := range out {
		if out[i].Kind == "upload-commit" || out[i].Kind == "upload-queued" {
			for j := int(out[i].Upload.Length); j < 32; j++ {
				out[i].Upload.Data[j] = 0
			}
		}
	}
	return out
}
func TestOriginalPostResultVideoCanonicalHandoff(t *testing.T) {
	for _, chain := range postResultVideoRows(t) {
		source := postResultVideoRAM(chain.Initial, chain.Progress)
		black := uint8(0)
		display := DisplayState{BallCHR: chain.Initial[0x54], PaletteID: chain.Initial[0x304], PaletteLevel: &black}
		beforeAward := chain.Progress
		// The real runtime retains pre-result campaign period/wins until settlement.
		beforeAward.Period -= 2
		index := ((source.Scene.Transition.Match.Period >> 1) + 1) & 1
		beforeAward.WinBits[index] &^= 1 << (source.Scene.Transition.Teams[1] - 1)
		got, e := NewOriginalPostResultVideoAfterResult(source.Scene, display, beforeAward, source.LogicByte)
		if e != nil {
			t.Fatal(chain.Kind, e)
		}
		reference := WaitAfterOriginalPostResultDispatch(source)
		if got.Scene != source.Scene || got.Progress != source.Progress || got.Scene.Transition.Random != source.Scene.Transition.Random {
			t.Fatal("handoff rewrote live source")
		}
		if got.Context.Ending.Actor.TextRequestFlags != 0 || got.Context.Ending.TextActive != 0 || got.Context.Ending.Upload.Address&0x8000 == 0 || got.Context.SecondUpload.Address&0x8000 == 0 {
			t.Fatal("canonical state issued old text")
		}
		for _, row := range chain.Rows {
			input := [4]ControllerFrame{{Held: row.Held, Pressed: row.Pressed}}
			a, e := StepOriginalPostResultVideo(got, input)
			if e != nil {
				t.Fatal(chain.Kind, row.Frame, e)
			}
			b, e := StepOriginalPostResultVideo(reference, input)
			if e != nil {
				t.Fatal(e)
			}
			if a.State.Scene != b.State.Scene || a.State.Progress != b.State.Progress || a.State.Clock != b.State.Clock || a.State.Resource != b.State.Resource || a.State.FieldCHR != b.State.FieldCHR || a.State.Control != b.State.Control || a.State.Mask != b.State.Mask || a.State.PaletteBuildLevel != b.State.PaletteBuildLevel || a.LiveRaster != b.LiveRaster || a.DispatchStarted != b.DispatchStarted || a.DispatchCompleted != b.DispatchCompleted || a.Reset != b.Reset || a.Handoff != b.Handoff {
				t.Fatal("canonical handoff changed behavior", chain.Kind, row.Frame)
			}
			if !reflect.DeepEqual(effectivePostResultEffects(a.Effects), effectivePostResultEffects(b.Effects)) {
				t.Fatal("canonical inactive bytes changed effectivewrites", chain.Kind, row.Frame)
			}
			got, reference = a.State, b.State
		}
	}
}
func TestOriginalPostResultVideoHandoffRejectsWrongBoundary(t *testing.T) {
	c := postResultVideoRows(t)[0]
	source := postResultVideoRAM(c.Initial, c.Progress)
	black := uint8(0)
	d := DisplayState{BallCHR: 13, PaletteID: source.Context.Ending.Actor.DisplayMask, PaletteLevel: &black}
	bad := d
	bad.PaletteFade = &DisplayPaletteFade{Out: true}
	if _, e := NewOriginalPostResultVideoAfterResult(source.Scene, bad, c.Progress, source.LogicByte); e == nil {
		t.Fatal("activefade accepted")
	}
	white := uint8(4)
	bad = d
	bad.PaletteLevel = &white
	if _, e := NewOriginalPostResultVideoAfterResult(source.Scene, bad, c.Progress, source.LogicByte); e == nil {
		t.Fatal("unfaded result accepted")
	}
	bad = d
	bad.BallCHR = 32
	if _, e := NewOriginalPostResultVideoAfterResult(source.Scene, bad, c.Progress, source.LogicByte); e == nil {
		t.Fatal("missing BCC3 CHR accepted")
	}
	scene := source.Scene
	scene.Transition.Match.Entered = true
	if _, e := NewOriginalPostResultVideoAfterResult(scene, d, c.Progress, source.LogicByte); e == nil {
		t.Fatal("running presentation accepted")
	}
	// Valid runtime palette variants must not collapse back to the court default.
	for _, x := range []struct{ court, palette uint8 }{{4, 9}, {4, 10}, {6, 11}, {6, 15}} {
		scene := source.Scene
		scene.Transition.Court = x.court
		display := d
		display.PaletteID = x.palette
		got, e := NewOriginalPostResultVideoAfterResult(scene, display, c.Progress, source.LogicByte)
		if e != nil || got.Context.Ending.Actor.DisplayMask != x.palette {
			t.Fatal("court palette lost", x, e)
		}
	}
}
