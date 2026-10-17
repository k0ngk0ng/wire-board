package engine

// advanceWorldBall18 keeps preparation writes before motion and tail writes
// after it. Shared fields not changed by the tail retain the device's values.
func advanceWorldBall18(result *WorldActionStep, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	p, e := PrepareOriginalBall18(s.Ball, worldTeamSpecialContext(*s, c), PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
	if e != nil {
		return e
	}
	if e = commitWorldTeamSpecial(s, OriginalTeamSpecialBallStep{OriginalShotBallStep: p.OriginalShotBallStep}); e != nil {
		return e
	}
	if p.ChangeContactLatch {
		s.ContactLatch = p.ContactLatch
	}
	result.SoundIDs = append(result.SoundIDs, p.SoundIDs...)
	done, e := CompleteOriginalBall18(p, court, func() Ball18TailContext {
		return Ball18TailContext{ContactLatch: s.ContactLatch, Random: s.Random, SelectedHoop: s.SelectedHoop}
	})
	if e != nil {
		return e
	}
	s.Ball = done.Ball
	if done.ChangeContactLatch {
		s.ContactLatch = done.ContactLatch
	}
	if done.ChangeRandom {
		s.Random = done.Random
	}
	result.SoundIDs = append(result.SoundIDs, done.SoundIDs...)
	return nil
}
