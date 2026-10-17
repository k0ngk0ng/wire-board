package engine

import "testing"

// Integration failures: rejecting a valid attached 3E entry before dispatch;
// loading old attachment/RNG during motion; missing target/pose-source writes;
// remapping 04E0+actor through controller ID again; consulting a throw selector
// on an entered continuation that never uses E003.
func TestWorldLaunchedPlayerCommitsBeforeMotion(t *testing.T) {
	s := courtTwoPlayingForTest(t)
	r := WorldActionStep{State: s.Playing.World.Preparation.Actions}
	c := s.Playing.Context.Actions
	c.Controls[0] = 3
	c.ActorHeldButtons[0] = ButtonUp
	c.ActorHeldButtons[3] = 0
	c.EnhancementEnabled = false
	p := &r.State.Actors[0].Player
	p.Action, p.Entered, p.Request = ActionLaunched, false, ActionRequest{}
	p.Body = Body{Position: Position{X: 280 * Pixel, Depth: 72 * Pixel, Height: 32 * Pixel}}
	r.State.Actors[0].SpecialShot = 3
	r.State.Actors[0].ShotCycle = 0
	r.State.PlayerLinks[0] = 128 | 4
	r.State.Auxiliary[0].Body.Position.Height = 60 * Pixel
	r.State.Auxiliary[0].FacingLeft = true
	r.State.Holder = 6
	r.State.SelectedHoop = 7
	r.State.MountFlags[0] = 0
	random := r.State.Random
	court, e := originalWorldMotionCourt(&r, c, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = advanceWorldPlayer(&r, 0, c, court); e != nil {
		t.Fatal(e)
	}
	if r.State.PlayerLinks[0] != 64 || r.State.PoseSource != 4 || r.State.Random != random {
		t.Fatal("launch preparation was not committed", r.State.PlayerLinks[0], r.State.PoseSource, r.State.Random, random)
	}
	if r.State.LaunchTarget != (Position{X: 68 * Pixel, Depth: 47 * Pixel, Height: 240 * Pixel}) || r.State.LaunchArcHeight != 24*Pixel || r.State.Actors[0].SpecialShot != 0 {
		t.Fatal("raw actor buttons or global launch metadata differ", r.State.LaunchTarget, r.State.LaunchArcHeight, r.State.Actors[0].SpecialShot)
	}
	if p.Body.Position.X < 200*Pixel || p.Body.Position.Height <= 32*Pixel {
		t.Fatal("motion followed old attachment or skipped launch integration", p.Body)
	}
	if len(r.SoundIDs) == 0 || r.SoundIDs[0] != 6 {
		t.Fatal("launch sound not before physics", r.SoundIDs)
	}
}
func TestWorldLaunchedContinuationSkipsUnusedSelector(t *testing.T) {
	s := courtTwoPlayingForTest(t)
	r := WorldActionStep{State: s.Playing.World.Preparation.Actions}
	c := s.Playing.Context.Actions
	p := &r.State.Actors[0].Player
	p.Action, p.Entered, p.Request = ActionLaunched, true, ActionRequest{}
	p.Body = Body{Position: Position{X: 280 * Pixel, Depth: 40 * Pixel, Height: 80 * Pixel}, Velocity: Velocity{X: Pixel, Height: Pixel}, Gravity: 64}
	p.Airborne = true
	p.Animation = AnimationState{}
	r.State.PlayerLinks[0] = 64
	r.State.SelectedHoop = -1
	r.State.Holder = 6
	random, target, arc := r.State.Random, r.State.LaunchTarget, r.State.LaunchArcHeight
	court, e := originalWorldMotionCourt(&r, c, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = advanceWorldPlayer(&r, 0, c, court); e != nil {
		t.Fatal(e)
	}
	if r.State.Random != random || r.State.LaunchTarget != target || r.State.LaunchArcHeight != arc || len(r.SoundIDs) != 0 {
		t.Fatal("continued launch reran preparation")
	}
	if p.Body.Position.X != 281*Pixel || p.Body.Position.Height != 80*Pixel+192 {
		t.Fatal("continued launch physics", p.Body)
	}
}
