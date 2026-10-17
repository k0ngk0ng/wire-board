package engine

import "testing"

// Failures: Court3 uses NY floor/passage rules; gate loses its existing low
// flags or overwrites its separate timer; X151/152 face the same direction;
// entered launch action replays sound21 or clears jump phase a second time.
func courtThreePlayingForTest(t *testing.T) RuntimeState {
	t.Helper()
	c := DefaultRuntimeConfig()
	c.Match.Court = 3
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
		t.Fatal("court three did not open")
	}
	return s
}
func TestCourtThreeGateBindingUsesOriginalFlagsAndFacing(t *testing.T) {
	for _, x := range []int{144, 151, 152, 159} {
		for _, active := range []bool{false, true} {
			s := courtThreePlayingForTest(t)
			r := WorldActionStep{State: s.Playing.World.Preparation.Actions}
			c := s.Playing.Context.Actions
			p := &r.State.Actors[0].Player
			cell, e := OriginalCourtSurface(3, Fixed(x)*Pixel, 80*Pixel)
			if e != nil {
				t.Fatal(e)
			}
			p.Body = Body{Position: Position{X: Fixed(x) * Pixel, Depth: 80 * Pixel, Height: cell.Height()}}
			p.Action, p.Entered, p.Request, p.Airborne = ActionIdle, true, ActionRequest{}, false
			if active {
				p.Action = ActionLaunched
			}
			p.FacingLeft = x >= 152
			p.Pose = Pose{Frame: 0, Mirrored: x >= 152}
			p.NewYorkPassage = 3
			r.State.Actors[0].JumpPhase = 7
			r.State.Actors[1].Player.NewYorkPassage = 5
			r.State.Holder = 0
			court, e := originalWorldMotionCourt(&r, c, 0)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = court.originalMotion.player(p, &playerTerrainContext{HasBall: true}); e != nil {
				t.Fatal(e)
			}
			if p.NewYorkPassage != 131 || r.State.Actors[1].Player.NewYorkPassage != 5 {
				t.Fatal("gate canonical flags/timer lost", x, active, p.NewYorkPassage, r.State.Actors[1].Player.NewYorkPassage)
			}
			if p.FacingLeft != (x < 152) || p.Pose.Mirrored != (x < 152) || p.Body.Position.Height != cell.Height() {
				t.Fatal("original gate facing/terrain differs", x, active, *p)
			}
			if active {
				if !p.Entered || len(r.SoundIDs) != 0 || court.originalMotion.resetJumpPhase {
					t.Fatal("existing launch retriggered gate", x, r.SoundIDs)
				}
			} else if p.Action != ActionLaunched || p.Entered || len(r.SoundIDs) != 1 || r.SoundIDs[0] != 21 || !court.originalMotion.resetJumpPhase {
				t.Fatal("gate failed to launch", x, *p, r.SoundIDs)
			}
		}
	}
}
func TestCourtThreeGateBindingHonorsOriginalBoundary(t *testing.T) {
	for _, point := range [][2]int{{143, 80}, {160, 80}, {147, 79}} {
		s := courtThreePlayingForTest(t)
		r := WorldActionStep{State: s.Playing.World.Preparation.Actions}
		c := s.Playing.Context.Actions
		p := &r.State.Actors[0].Player
		cell, e := OriginalCourtSurface(3, Fixed(point[0])*Pixel, Fixed(point[1])*Pixel)
		if e != nil {
			t.Fatal(e)
		}
		p.Body = Body{Position: Position{X: Fixed(point[0]) * Pixel, Depth: Fixed(point[1]) * Pixel, Height: cell.Height()}}
		p.Action, p.Entered, p.Request, p.Airborne = ActionIdle, true, ActionRequest{}, false
		p.NewYorkPassage = 3
		court, e := originalWorldMotionCourt(&r, c, 0)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = court.originalMotion.player(p, &playerTerrainContext{}); e != nil {
			t.Fatal(e)
		}
		if p.NewYorkPassage != 3 || p.Action != ActionIdle || len(r.SoundIDs) != 0 || court.originalMotion.resetJumpPhase {
			t.Fatal("gate triggered beyond original region", point, *p, r.SoundIDs)
		}
	}
}
