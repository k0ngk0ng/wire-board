package engine

// Preparation writes must precede bound court motion; completion writes only
// the ball. Court devices own any shared metadata they change during motion.
func advanceWorldBall16(result *WorldActionStep, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	prepared, e := PrepareOriginalBall16(s.Ball, worldTeamSpecialContext(*s, c), PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
	if e != nil {
		return e
	}
	if e = commitWorldTeamSpecial(s, OriginalTeamSpecialBallStep{OriginalShotBallStep: prepared}); e != nil {
		return e
	}
	result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
	completed, e := CompleteOriginalBall16(prepared, court, func() int8 { return s.SelectedHoop })
	if e != nil {
		return e
	}
	s.Ball = completed.Ball
	result.SoundIDs = append(result.SoundIDs, completed.SoundIDs...)
	return nil
}
func advanceWorldBall19(result *WorldActionStep, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	prepared, e := PrepareOriginalBall19(s.Ball, worldTeamSpecialContext(*s, c), PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
	if e != nil {
		return e
	}
	if e = commitWorldTeamSpecial(s, OriginalTeamSpecialBallStep{OriginalShotBallStep: prepared.OriginalShotBallStep}); e != nil {
		return e
	}
	if prepared.ChangeContactLatch {
		s.ContactLatch = prepared.ContactLatch
	}
	result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
	completed, e := CompleteOriginalBall19(prepared, court, func() uint8 { return s.ContactLatch })
	if e != nil {
		return e
	}
	s.Ball = completed.Ball
	if completed.ChangeContactLatch {
		s.ContactLatch = completed.ContactLatch
	}
	result.SoundIDs = append(result.SoundIDs, completed.SoundIDs...)
	return nil
}
func advanceWorldBall22(result *WorldActionStep, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	prepared, e := PrepareOriginalBall22(s.Ball, worldTeamSpecialContext(*s, c))
	if e != nil {
		return e
	}
	if e = commitWorldTeamSpecial(s, OriginalTeamSpecialBallStep{OriginalShotBallStep: prepared}); e != nil {
		return e
	}
	result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
	completed, e := CompleteOriginalBall22(prepared, court, func() int8 { return s.SelectedHoop })
	if e != nil {
		return e
	}
	s.Ball = completed.Ball
	result.SoundIDs = append(result.SoundIDs, completed.SoundIDs...)
	return nil
}
