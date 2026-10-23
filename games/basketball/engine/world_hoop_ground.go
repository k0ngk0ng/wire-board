package engine

func originalHoopGroundAction(action Action) bool {
	return action == ActionHoopCarryIdle || action == ActionHoopCarryRun || action == ActionHoopAim || action == ActionHoopThrow
}

// Ground-named hoop handlers still run on raised courts and during falls.
// Preserve their ordinary EEFF transitions instead of rejecting airborne state.
func advanceWorldHoopGround(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	action := p.Action
	computer := c.Controls[actor]&128 != 0
	profile := c.Profiles[actor]
	if s.PlayerLinks[actor]&128 != 0 && court.originalMotion == nil {
		var err error
		court, err = originalWorldMotionCourt(result, c, actor)
		if err != nil {
			return err
		}
	}
	if !p.Entered {
		initializeAction(p)
		switch action {
		case ActionHoopCarryIdle:
			stopHorizontal(p)
			if !p.Airborne {
				stopVertical(p)
			}
		case ActionHoopCarryRun:
			a.ShotCycle = 8
			setRunHeading(p, profile.RunSpeed)
		case ActionHoopAim:
			p.Animation.NextAction, p.Animation.HasNextAction = ActionHoopAim, true
			if computer {
				p.Animation.NextAction = ActionHoopThrow
			}
			a.AimWindow = 32
		case ActionHoopThrow:
			p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
			if s.PlayerLinks[actor]&128 != 0 {
				p.Animation.NextAction = ActionRiding
			}
			faceHeading(p)
			p.Body.Velocity.Depth = 0
			index := 0
			for index < 5 && s.Hoops[index].Owner != actor {
				index++
			}
			h := &s.Hoops[index]
			h.Action, h.Entered, h.FacingLeft = HoopThrown, false, p.FacingLeft
		}
	}
	if action == ActionHoopCarryRun {
		p.Body.Velocity.Depth = 0
		if p.Heading != HeadingNone {
			if computer {
				setRunHeading(p, profile.RunSpeed)
			}
			switch p.Heading {
			case HeadingUp, HeadingUpRight, HeadingUpLeft:
				p.Body.Velocity.Depth = 2 * Pixel
			case HeadingDown, HeadingDownRight, HeadingDownLeft:
				p.Body.Velocity.Depth = -2 * Pixel
			}
		}
		faceHeading(p)
	}
	if err := animatePlayer(p, profile, action); err != nil {
		return err
	}
	if action == ActionHoopAim || action == ActionHoopThrow {
		p.Body.Velocity.X = approachZero(p.Body.Velocity.X, 128)
		if p.Body.Velocity.Depth > 0 {
			p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth - 16)
		} else if p.Body.Velocity.Depth < 0 {
			p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth + 16)
		}
	}
	ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: computer, HasBall: s.Holder == int8(actor)}, court)
	if err != nil {
		return err
	}
	s.Ball = ApplyBallCommand(s.Ball, ball)
	switch action {
	case ActionHoopCarryIdle:
		if p.Airborne {
			p.Action, p.Entered = ActionFalling, false
			if s.PlayerLinks[actor]&128 != 0 {
				p.Action = ActionRiding
			}
		}
	case ActionHoopCarryRun:
		if p.Airborne {
			p.Action, p.Entered = ActionFalling, false
		}
	case ActionHoopAim:
		if p.Airborne && p.Body.Gravity == 0 {
			p.Body.Velocity.Height = signed16(p.Body.Velocity.Height + 0x3c0)
			p.Body.Gravity = 0x1e0
		}
	}
	return nil
}
