package contra

// Before browser split rendering: scene raster effects last only this frame;
// NMI restores normal shadow state. Keep active flags separate from zero-valued
// effect parameters and never overwrite gameplay scroll/control with a split.
import "testing"

func TestSceneRasterEffectsAreFrameScoped(t *testing.T) {
	w := World{}
	w.Active.Combat.World.Scroll = CollisionScroll{X: 12, Y: 224, NameTable: 0xb0}
	w.Active.Presentation.Display.Mask = 0x1e
	out := CampaignSceneResult{Controls: []CampaignSceneControl{{Kind: "split-scroll", Value: 0}, {Kind: "raster-mask", Value: 0}, {Kind: "raster-control", Value: 0xb1}}}
	if e := w.applyScene(out); e != nil {
		t.Fatal(e)
	}
	d := w.Active.Presentation.Display
	if !d.SplitActive || !d.RasterActive || d.SplitScroll != 0 || d.RasterMask != 0 || d.RasterControl != 0xb1 {
		t.Fatal("zero-valued active effect lost", d)
	}
	if w.Active.Combat.World.Scroll != (CollisionScroll{X: 12, Y: 224, NameTable: 0xb0}) || d.Mask != 0x1e {
		t.Fatal("raster overwrote authoritative shadow")
	}
	if _, e := w.Active.Presentation.CommitWithScroll(w.Active.Combat.World.Scroll); e != nil {
		t.Fatal(e)
	}
	d = w.Active.Presentation.Display
	if d.SplitActive || d.RasterActive || d.AppliedMask != 0x1e {
		t.Fatal("old split leaked into next frame", d)
	}
}
