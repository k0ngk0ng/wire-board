package engine

func worldHurtContext(s WorldActionState, actor uint8, c WorldActionContext) HurtContext {
	return HurtContext{Actor: actor, Computer: c.Controls[actor]&128 != 0, RecoveryGrade: c.RecoveryGrades[actor], Random: s.Random, BallHolder: s.Holder, BallVelocityX: s.Ball.Body.Velocity.X, AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}, HoopOwners: s.hoopOwners(), SpecialHolder: s.SpecialHolder, AssistedActor: s.Auxiliary[1].ObjectLink, LinkedActor: s.Actors[actor].LinkedActor, Attachments: s.PlayerLinks}
}

// advanceWorldHurtPlayer commits original injury entry effects before EEFF.
// In action49, AA2B precedes the support operation8B67, which precedes clearing
// the actor attachment. Keep these as ordered commands, not a late link reset.
func advanceWorldHurtPlayer(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	attached := s.PlayerLinks[actor]&128 != 0
	prepared, e := PrepareHurtPlayer(HurtState{Player: a.Player, Endurance: a.Endurance}, worldHurtContext(*s, actor, c))
	if e != nil {
		return e
	}
	a.Player, a.Endurance, s.Random = prepared.State.Player, prepared.State.Endurance, prepared.Random
	for _, command := range prepared.Objects {
		if command.DetachSupport {
			if e = detachWorldJumpSupport(s, command.Object, c); e != nil {
				return e
			}
		} else if e = s.applyHurtObject(command); e != nil {
			return e
		}
	}
	s.Ball = ApplyBallCommand(s.Ball, prepared.Ball)
	if prepared.ReleaseBall {
		s.Holder = 6
	}
	if prepared.ChangeBallVelocity {
		s.Ball.Body.Velocity.X = prepared.BallVelocityX
	}
	if prepared.ClearSpecialHolder {
		s.SpecialHolder = -1
	}
	if attached && court.originalMotion == nil {
		court, e = originalWorldMotionCourt(result, c, actor)
		if e != nil {
			return e
		}
	}
	completed, e := CompleteHurtPlayer(prepared, worldHurtContext(*s, actor, c), court)
	if e != nil {
		return e
	}
	a.Player, a.Endurance = completed.State.Player, completed.State.Endurance
	s.Ball = ApplyBallCommand(s.Ball, completed.Ball)
	return nil
}
