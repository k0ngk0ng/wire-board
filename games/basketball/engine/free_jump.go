package engine

// FreePlayerContext carries ownership, shot-cycle memory and the input-stage
// jump phase into ordinary movement handlers. Their action IDs can remain
// active while holding the ball; this context does not select an AI policy.
type FreePlayerContext struct {
	Computer  bool
	HasBall   bool
	JumpPhase int8
	ShotCycle int8
}

func StepFreePlayerWithContext(original Player, ctx FreePlayerContext, profile CharacterProfile, court FlatCourt) (Player, error) {
	return stepFreePlayer(original, profile, court, ctx)
}

func launchFreeJump(p *Player, action Action, profile CharacterProfile, phase int8) {
	if action == ActionJump {
		v := profile.JumpTravel[p.Heading]
		p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
	}
	p.Body.Velocity.Height = profile.JumpVelocity
	if uint8(phase) == 0xe0 {
		p.Body.Velocity.Height = 3 * Pixel
	}
	p.Body.Gravity = profile.Gravity
	p.Airborne = true
}

// stepFreeFalling retains the free-player compatibility API. Its caller has
// already excluded ball/prop ownership and rider attachments.
func stepFreeFalling(p Player, ctx FreePlayerContext, court FlatCourt) (Player, error) {
	step, err := StepFallingPlayer(p, FallingContext{Computer: ctx.Computer}, court)
	return step.Player, err
}

// FreePlayerStep is one original actor action result. Revisit is a scheduler
// signal, not another action already executed by this function.
type FreePlayerStep struct {
	Player    Player
	Revisit   bool
	Ball      BallCommand
	ShotCycle int8
}

// Compatibility callers request the former complete world phase for a single
// ordinary actor. The original jump handler needs at most one extra visit.
func stepFreePlayer(original Player, profile CharacterProfile, court FlatCourt, ctx FreePlayerContext) (Player, error) {
	p := original
	for visit := 0; visit < 2; visit++ {
		step, err := StepFreePlayerAction(p, ctx, profile, court)
		if err != nil {
			return original, err
		}
		p = step.Player
		if !step.Revisit {
			return p, nil
		}
	}
	return p, nil
}

// $88C9 can put any held running action at cursor 3. For ordinary action 2,
// $AA82[2] points to $AB61, so that cursor reads the adjacent original bytes
// at $AB67 (02 0F 02 FF 07 01 12 FF FF), not the dribble-run charge frames.
func animateFreeRunCharge(p *Player) error {
	state := p.Animation
	state.Cursor -= 3
	spec := AnimationSpec{Frames: []AnimationFrame{
		{Pose: Pose{Frame: 2}, Ticks: 15}, {Pose: Pose{Frame: 2}, Ticks: 255},
		{Pose: Pose{Frame: 7}, Ticks: 1}, {Pose: Pose{Frame: 18}, Ticks: 255},
	}, End: RepeatForever}
	step, err := AdvanceAnimation(spec, state, p.FacingLeft)
	if err != nil {
		return err
	}
	if !(step.State.Cursor == 0 && step.State.Elapsed == 0) {
		step.State.Cursor += 3
	}
	p.Animation, p.Pose = step.State, step.Pose
	return nil
}
