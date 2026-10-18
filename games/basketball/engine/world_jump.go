package engine

import "fmt"

func originalWorldJumpAction(action Action) bool {
	switch action {
	case 4, 5, 13, 0x14, 0x15, 0x24, 0x25, 0x34, 0x35, 0x44, 0x45:
		return true
	}
	return false
}

// advanceWorldJump preserves all five EC31 callers. In particular, ordinary,
// ball and hoop jumps launch after their windup motion; carried-body jumps
// move the support and rider together. Equipment only changes takeoff gravity.
func advanceWorldJump(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	action := p.Action
	if !originalWorldJumpAction(action) {
		return fmt.Errorf("jump action %02x is outside original handlers", action)
	}
	if action == 0x24 || action == 0x25 {
		return advanceWorldAuxiliaryCarrier(result, actor, c, court)
	}
	paired := action == 0x44 || action == 0x45
	carrier := action == 0x14 || action == 0x15
	hoop := action == 0x34 || action == 0x35
	entered := !p.Entered
	fast := entered && a.JumpPhase < 0
	if entered {
		initializeAction(p)
		if !carrier && !paired {
			if err := detachWorldJumpSupport(s, actor, c); err != nil {
				return err
			}
			s.PlayerLinks[actor] = 0
		}
		if paired && s.Holder == int8(actor) {
			s.Ball.Action, s.Ball.Entered = BallHeld, false
		}
		if fast {
			p.Animation.Cursor++
		}
	}
	if !fast {
		faceHeading(p)
		frame := uint8(4)
		if paired {
			frame = 56
		}
		frames := []AnimationFrame{{Pose: Pose{Frame: 5}, Ticks: 2}, {Pose: Pose{Frame: frame}, Ticks: 255}, {Pose: Pose{Frame: frame}, Ticks: 255}}
		if action == 13 {
			frames = frames[1:]
		}
		animation, err := AdvanceAnimation(AnimationSpec{Frames: frames, End: RepeatForever}, p.Animation, p.FacingLeft)
		if err != nil {
			return err
		}
		p.Animation, p.Pose = animation.State, animation.Pose
		if animation.ChangeAction {
			p.Action, p.Entered = animation.Action, false
		}
		if !paired {
			ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor)}, court)
			if err != nil {
				return err
			}
			s.Ball = ApplyBallCommand(s.Ball, ball)
		}
		if p.Animation.Cursor == 0 {
			return nil
		}
	}
	if fast || p.Animation.Elapsed == 0 {
		if paired {
			support := s.PlayerLinks[actor] & 15
			if support >= 4 {
				return fmt.Errorf("paired jump support %d is not a player", support)
			}
			s.Actors[support].Player.Action, s.Actors[support].Player.Entered = 0x13, false
			s.Actors[support].Player.FacingLeft = p.FacingLeft
		}
		result.SoundIDs = append(result.SoundIDs, 23)
		if !paired {
			p.Airborne = true
		}
		if carrier {
			stopVertical(p)
			if err := detachWorldJumpSupport(s, actor, c); err != nil {
				return err
			}
			s.PlayerLinks[actor] = 0
		}
		if action&15 != 5 {
			v := c.Profiles[actor].JumpTravel[p.Heading]
			p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
		} else if carrier {
			setRunningDepth(p)
		}
		stopVertical(p)
		p.Body.Velocity.Height = c.Profiles[actor].JumpVelocity
		if !carrier && !hoop && !paired && uint8(a.JumpPhase) == 0xe0 {
			p.Body.Velocity.Height = 3 * Pixel
		}
		p.Body.Gravity = Pixel
		if c.EnhancementEnabled && c.Teams[actor/2] == 0 && c.JumpEnhancements != nil && c.JumpEnhancements[actor] == 23 {
			p.Body.Gravity = Pixel / 2
		}
		if carrier && s.Holder == int8(actor) {
			s.Ball.Action, s.Ball.Entered = BallHeld, false
		}
	}
	if paired {
		return advanceWorldPairedJumpMotion(result, actor, c)
	}
	if !p.Airborne {
		p.Action, p.Entered = ActionLand, false
		if hoop {
			p.Action = 0x36
		} else if carrier && s.Holder == int8(actor) {
			p.Action = ActionCarrierLand
		}
	}
	return nil
}

// Original9C4E: copy rider motion, move support then rider, copy resulting
// support motion back, and stop support. The attachment is intentionally kept.
func advanceWorldPairedJumpMotion(result *WorldActionStep, actor uint8, c WorldActionContext) error {
	s := &result.State
	support := s.PlayerLinks[actor] & 15
	if support >= 4 || support == actor {
		return fmt.Errorf("paired jump support %d outside other players", support)
	}
	f := worldFloorState(*s, c)
	f.Objects[support].Body.Velocity = f.Objects[actor].Body.Velocity
	f.Objects[support].Body.Gravity = f.Objects[actor].Body.Gravity
	for _, object := range []uint8{support, actor} {
		binding, err := originalWorldMotionCourt(result, c, object)
		if err != nil {
			return err
		}
		moved, err := binding.originalMotion.run(f)
		if err != nil {
			return err
		}
		f = moved.State
		storeWorldFloorState(s, f, 13)
	}
	f.Objects[actor].Body.Velocity = f.Objects[support].Body.Velocity
	f.Objects[actor].Body.Gravity = f.Objects[support].Body.Gravity
	f.Objects[support].Body.Velocity = Velocity{}
	f.Objects[support].Body.Gravity = 0
	storeWorldFloorState(s, f, 13)
	if !s.Actors[support].Player.Airborne {
		s.Actors[actor].Player.Action, s.Actors[actor].Player.Entered = 0x40, false
		s.Actors[support].Player.Action, s.Actors[support].Player.Entered = 4, false
	}
	return nil
}
