package engine

// Captured entry writes are visible to the original court motion and to the
// subsequent container/ball visits in the same action scheduler iteration.
func advanceWorldCapturedPlayer(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	ctx := CapturedPlayerContext{Actor: actor, Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), Attachment: s.PlayerLinks[actor], CaptureCounter: s.CaptureCounter}
	prepared, e := PrepareCapturedPlayer(CapturedPlayerState{Player: a.Player, ShotCycle: a.ShotCycle}, ctx, c.Profiles[actor])
	if e != nil {
		return e
	}
	a.Player, a.ShotCycle = prepared.State.Player, prepared.State.ShotCycle
	s.PlayerLinks[actor] = prepared.Attachment
	if prepared.ChangeSpecialHolder {
		s.SpecialHolder = prepared.SpecialHolder
	}
	if prepared.ReleaseHolder {
		s.Holder = 6
	}
	s.Ball = ApplyBallCommand(s.Ball, prepared.Ball)
	if prepared.ChangeBallVelocityX {
		s.Ball.Body.Velocity.X = prepared.BallVelocityX
	}
	result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
	ctx.HasBall = s.Holder == int8(actor)
	ctx.Attachment = prepared.Attachment
	completed, e := CompleteCapturedPlayer(prepared.State, ctx, c.Profiles[actor], court)
	if e != nil {
		return e
	}
	a.Player, a.ShotCycle = completed.State.Player, completed.State.ShotCycle
	s.PlayerLinks[actor] = completed.Attachment
	s.Ball = ApplyBallCommand(s.Ball, completed.Ball)
	if completed.ChangeCaptureCounter {
		s.CaptureCounter = completed.CaptureCounter
	}
	if completed.ReleaseContainer {
		s.Auxiliary[0].Action, s.Auxiliary[0].Entered = 7, false
	}
	return nil
}
