package engine

import "testing"

func TestOriginalCompleteProjectionStages(t *testing.T) {
	var rows []struct {
		Frame         int
		Before, After ProjectionState
		Context       ProjectionContext
	}
	readCameraFixture(t, "projection", &rows)
	if len(rows) != 1998 {
		t.Fatal("missing thirteen-object stages", len(rows))
	}
	for _, r := range rows {
		got, err := StepOriginalProjection(r.Before, r.Context)
		if err != nil || got != r.After {
			t.Fatalf("frame %d got %+v want %+v error %v", r.Frame, got, r.After, err)
		}
	}
}

func TestOriginalProjectionVisibilityAndStableDepth(t *testing.T) {
	s := ProjectionState{}
	for i := range s.DrawOrder {
		s.DrawOrder[i] = 255
	}
	c := ProjectionContext{LayoutID: 1, CameraY: 0x129}
	for i := range c.Poses {
		c.Poses[i] = 127
	}
	for i := 0; i < 3; i++ {
		c.Poses[i] = 0
		c.Positions[i] = Position{X: 100 * Pixel, Depth: 40 * Pixel, Height: 10 * Pixel}
	}
	c.Positions[2].Depth = 30 * Pixel
	got, err := StepOriginalProjection(s, c)
	if err != nil || got.Projected[0] != (ScreenPosition{X: 100, Y: 133}) || got.DrawOrder[0] != 2 || got.DrawOrder[1] != 0 || got.DrawOrder[2] != 1 {
		t.Fatal(got, err)
	}
	for _, r := range []struct {
		x       int16
		updated bool
	}{{-49, false}, {-48, true}, {303, true}, {304, false}} {
		c.Positions[0].X = Fixed(r.x) * Pixel
		s.Projected[0] = ScreenPosition{X: 17, Y: 888}
		p, err := ProjectOriginalObject(s, 0, c)
		if err != nil || p.Projected[0].X != r.x || (p.Projected[0].Y != 888) != r.updated {
			t.Fatal(r, p, err)
		}
	}
	c.Poses[0] = 255
	p, err := ProjectOriginalObject(s, 0, c)
	if err != nil || p != s {
		t.Fatal("hidden pose must preserve caches", p, err)
	}
	c.Poses[0] = 0
	c.Positions[0].X = 0
	c.LayoutID = 2
	c.CameraY = 0
	c.Positions[0].Depth = 30 * Pixel
	c.Positions[0].Height = 0
	s.DrawCount = 1
	s.DrawOrder[0] = 7
	p, err = ProjectOriginalObject(s, 0, c)
	if err != nil || p.DrawCount != 2 || p.DrawOrder[0] != 7 || p.DrawOrder[1] != 0 {
		t.Fatal("court2 append", p, err)
	}
}
