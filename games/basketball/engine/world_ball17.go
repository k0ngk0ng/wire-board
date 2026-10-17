package engine

func advanceWorldBall17(result *WorldActionStep, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	prepared, e := PrepareOriginalBall17(s.Ball, worldTeamSpecialContext(*s, c))
	if e != nil {
		return e
	}
	if e = commitWorldTeamSpecial(s, OriginalTeamSpecialBallStep{OriginalShotBallStep: prepared}); e != nil {
		return e
	}
	result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
	ball, e := CompleteOriginalBall17(prepared.Ball, court)
	if e != nil {
		return e
	}
	// Motion may write shared state; the tail changes only the ball.
	s.Ball = ball
	return nil
}
