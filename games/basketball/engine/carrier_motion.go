package engine

func setRunningDepth(p *Player) {
	p.Body.Velocity.Depth = 0
	switch p.Heading {
	case HeadingUp, HeadingUpRight, HeadingUpLeft:
		p.Body.Velocity.Depth = 2 * Pixel
	case HeadingDown, HeadingDownRight, HeadingDownLeft:
		p.Body.Velocity.Depth = -2 * Pixel
	}
}

func brakePlayer(p *Player, profile CharacterProfile) {
	if p.Body.Velocity.X > 0 {
		p.Body.Velocity.X = max(0, p.Body.Velocity.X-profile.BrakeRight)
	}
	if p.Body.Velocity.X < 0 {
		p.Body.Velocity.X = min(0, p.Body.Velocity.X+profile.BrakeLeft)
	}
	if p.Body.Velocity.Depth > 0 {
		p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth - profile.BrakeDepth)
	} else if p.Body.Velocity.Depth < 0 {
		p.Body.Velocity.Depth = signed16(p.Body.Velocity.Depth + profile.BrakeDepth)
	}
}

func launchCarrierJump(result *CarrierStep, ctx CarrierContext, profile CharacterProfile) {
	p := &result.Carrier.Player
	if p.Action&15 == 5 {
		setRunningDepth(p)
	} else {
		v := profile.JumpTravel[p.Heading]
		p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
	}
	p.Body.Velocity.Height, p.Body.Gravity = profile.JumpVelocity, profile.Gravity
	p.Airborne = true
	result.Revisit = true
	if ctx.HasBall {
		result.Ball.Change, result.Ball.Action = true, BallHeld
	}
}

func animateCarrierRun(p *Player, profile CharacterProfile) error {
	if p.Animation.Cursor < 3 {
		return animatePlayer(p, profile, ActionDribbleRun)
	}
	s := p.Animation
	s.Cursor -= 3
	step, err := AdvanceAnimation(profile.RunCharge, s, p.FacingLeft)
	if err != nil {
		return err
	}
	if !(step.State.Cursor == 0 && step.State.Elapsed == 0) {
		step.State.Cursor += 3
	}
	p.Animation, p.Pose = step.State, step.Pose
	return nil
}
