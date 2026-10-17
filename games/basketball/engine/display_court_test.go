package engine

import (
	"encoding/json"
	"testing"
)

// Court selector must survive the source/submitted/visible/JSON pipeline and
// remain independent from Scene/layout. Wider display support does not unlock
// incomplete runtime courts.
func TestDisplayPreservesOriginalCourtSelector(t *testing.T) {
	for court := uint8(0); court < 8; court++ {
		o, e := NewOriginalMatch(DefaultOriginalMatchConfiguration())
		if e != nil {
			t.Fatal(e)
		}
		o.Transition.Court = court
		o.Transition.Match.Phase = MatchPlaying
		o.Transition.Match.Entered = true
		o.Transition.CameraScroll = [4]uint16{128, 297, 128, 297}
		o.Projection = resetOriginalWorldDrawList(o.Projection)
		for i := range o.Transition.Objects {
			o.Transition.Objects[i].Pose.Frame = 127
		}
		src := DisplaySourceFromOpening(o)
		if src.Court != court {
			t.Fatal("opening source lost court", court, src.Court)
		}
		world := DisplaySourceFromWorld(WorldLoopState{}, WorldLoopContext{LayoutID: 1, Actions: WorldActionContext{CourtID: court}}, MatchState{Phase: MatchPlaying})
		if world.Court != court {
			t.Fatal("world source substituted layout for court", court, world.Court)
		}
		d, e := NewOriginalDisplay(DefaultOriginalMatchConfiguration())
		if e != nil {
			t.Fatal(e)
		}
		d, e = StageDisplay(d, src)
		if e != nil {
			t.Fatal(e)
		}
		d, e = CommitDisplayVideo(d, 0, 0)
		if e != nil {
			t.Fatal(e)
		}
		raw, e := json.Marshal(d)
		if e != nil {
			t.Fatal(e)
		}
		var restored DisplayState
		if e = json.Unmarshal(raw, &restored); e != nil {
			t.Fatal(e)
		}
		if restored.Submitted.Court != court || restored.Visible.Court != court {
			t.Fatal("display restore lost court", court)
		}
	}
	for _, court := range []uint8{8, 255} {
		c := DefaultRuntimeConfig()
		c.Match.Court = court
		if _, e := NewRuntime(c); e == nil {
			t.Fatal("display metadata unlocked unknown runtime court", court)
		}
	}
}

func TestDisplayMotionUsesSelectedCourtSurface(t *testing.T) {
	d, e := NewOriginalDisplay(DefaultOriginalMatchConfiguration())
	if e != nil {
		t.Fatal(e)
	}
	writes := []MotionSurfaceWrite{{Object: 0, Origin: Position{X: 264 * Pixel, Depth: 72 * Pixel}}}
	zero, e := ObserveDisplayMotionForCourt(d, writes, 0)
	if e != nil {
		t.Fatal(e)
	}
	ny, e := ObserveDisplayMotionForCourt(d, writes, 1)
	if e != nil {
		t.Fatal(e)
	}
	if zero.SurfaceCodes[0] != 32 || ny.SurfaceCodes[0] != 0 {
		t.Fatal("display terrain selected wrong profile", zero.SurfaceCodes, ny.SurfaceCodes)
	}
}
