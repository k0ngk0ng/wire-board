package engine

import "testing"

// Court 2 binding failures: NY geometry/boundaries substituted; shared switch
// flags duplicated; device-cleared JumpPhase overwritten by a local actor
// return; a pre-motion dribble command overwrites the later device-held-ball
// write; sound requests reorder around the physics call.
func courtTwoPlayingForTest(t *testing.T) RuntimeState {
	t.Helper()
	c := DefaultRuntimeConfig()
	c.Match.Court = 2
	c.Match.Seed = 0x12345678
	s, e := NewRuntime(c)
	if e != nil {
		t.Fatal(e)
	}
	for n := 0; s.Playing == nil && n < 400; n++ {
		step, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(n, e)
		}
		s = step.State
	}
	if s.Playing == nil {
		t.Fatal("court two did not open")
	}
	return s
}
func TestCourtTwoDeviceBindingRetainsLaterBallAndJumpWrites(t *testing.T) {
	s := courtTwoPlayingForTest(t)
	r := WorldActionStep{State: s.Playing.World.Preparation.Actions}
	c := s.Playing.Context.Actions
	a := &r.State.Actors[0]
	p := &a.Player
	p.Body = Body{Position: Position{X: 280 * Pixel, Depth: 72 * Pixel, Height: 32 * Pixel}}
	p.Action, p.Entered, p.Request, p.Airborne = ActionDribbleIdle, true, ActionRequest{}, false
	p.Animation = AnimationState{}
	p.Heading = HeadingNone
	a.JumpPhase = 7
	r.State.Holder = 0
	r.State.Ball.Action = BallHeld
	r.State.Ball.Entered = true
	court, e := originalWorldMotionCourt(&r, c, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = advanceWorldPlayer(&r, 0, c, court); e != nil {
		t.Fatal(e)
	}
	if r.State.Actors[0].Player.Action != 62 || r.State.Actors[0].Player.Entered || r.State.Actors[0].JumpPhase != 0 {
		t.Fatal("device actor state lost", r.State.Actors[0])
	}
	if r.State.Actors[1].Player.NewYorkPassage != 128 {
		t.Fatal("device switch did not use canonical 06D8")
	}
	if r.State.Ball.Action != BallHeld || r.State.Ball.Entered {
		t.Fatal("pre-motion dribble overwrote device ball action", r.State.Ball)
	}
	if len(r.SoundIDs) != 1 || r.SoundIDs[0] != 21 {
		t.Fatal("device sound", r.SoundIDs)
	}
}
func TestCourtTwoDeviceJumpResetSurvivesAttackResult(t *testing.T) {
	s := courtTwoPlayingForTest(t)
	r := WorldActionStep{State: s.Playing.World.Preparation.Actions}
	c := s.Playing.Context.Actions
	found := false
	for kind, profiles := range originalAttackProfiles {
		for role, profile := range profiles {
			if profile.Handler == 3 && !found {
				c.Teams[0], c.Characters[0] = uint8(role/4), uint8(role%4)
				r.State.Actors[0].Player.Action = Action(96 + kind)
				found = true
			}
		}
	}
	if !found {
		t.Fatal("original attack handler3 missing")
	}
	p := &r.State.Actors[0].Player
	p.Entered = false
	p.Request = ActionRequest{}
	p.Airborne = false
	p.Heading = HeadingRight
	p.Body = Body{Position: Position{X: 280 * Pixel, Depth: 72 * Pixel, Height: 32 * Pixel}, Velocity: Velocity{Height: -0x960}, Gravity: -0x160}
	r.State.Actors[0].JumpPhase = 0
	r.State.Holder = 6
	court, e := originalWorldMotionCourt(&r, c, 0)
	if e != nil {
		t.Fatal(e)
	}
	if e = advanceWorldPlayer(&r, 0, c, court); e != nil {
		t.Fatal(e)
	}
	if r.State.Actors[0].JumpPhase != 0 || r.State.Actors[0].Player.Action != 62 {
		t.Fatal("actor return overwrote device write", r.State.Actors[0])
	}
	if len(r.SoundIDs) != 2 || r.SoundIDs[1] != 21 {
		t.Fatal("device sound must follow attack entry sound", r.SoundIDs)
	}
}
