package engine

import "testing"

// Runtime must use Court 0's original 32-pixel rear ledge and exclusive depth
// 73, not New York's 64-pixel side platforms/depth88. Both controller modes
// must complete naturally, with the existing restoration/integration gate.
func TestRuntimeCourtZeroColdCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 0
	c.Match.Seed = 0x12345678
	verifyColdVideoRuntime(t, c, true)
}
func TestRuntimeCourtZeroAllComputerCompleteMatchAndRestore(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 0
	c.Match.Seed = 0x12345678
	c.Match.ControllerAssignments = [4]int8{-1, -1, -1, -1}
	verifyColdVideoRuntime(t, c, true)
}
func TestWorldCourtZeroUsesOriginalRearLedge(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Court = 0
	c.Match.Seed = 0x12345678
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; s.Playing == nil && n < 400; n++ {
		r, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		s = r.State
	}
	if s.Playing == nil {
		t.Fatal("court zero opening failed")
	}
	state := s.Playing.World.Preparation.Actions
	p := &state.Actors[0].Player
	p.Body = Body{Position: Position{X: 264 * Pixel, Depth: 72 * Pixel, Height: 33 * Pixel}, Velocity: Velocity{Height: -2 * Pixel}}
	p.Action, p.Entered, p.Request, p.Airborne = ActionIdle, true, ActionRequest{}, true
	state.PlayerLinks[0] = 0
	state.Holder = 6
	ctx := s.Playing.Context.Actions
	got, e := StepWorldActions(state, ctx)
	if e != nil {
		t.Fatal(e)
	}
	if got.State.Actors[0].Player.Body.Position.Height != 32*Pixel || got.State.Actors[0].Player.Airborne {
		t.Fatal("court zero ledge replaced by New York geometry", got.State.Actors[0].Player)
	}
	if len(got.MotionSurfaceWrites) == 0 {
		t.Fatal("real court surface event missing")
	}
}
