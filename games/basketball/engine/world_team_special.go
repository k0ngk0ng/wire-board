package engine

import "fmt"

func worldTeamSpecialContext(s WorldActionState, c WorldActionContext) OriginalShotBallContext {
	ctx := OriginalShotBallContext{Holder: s.Holder, UnownedSpecialShot: s.UnownedSpecialShot, EnhancementEnabled: c.EnhancementEnabled, Equipment: 255, BallInputButtons: c.BallShotInputButtons, MountFlags: s.MountFlags, SelectedHoop: s.SelectedHoop, Target: s.LaunchTarget, ArcHeight: s.LaunchArcHeight, Landing: s.BallLanding}
	if s.Holder >= 0 && s.Holder < 4 {
		a := s.Actors[s.Holder]
		ctx.ShooterHeight, ctx.ShooterAction = a.Player.Body.Position.Height, a.Player.Action
		ctx.Team, ctx.Character = c.Teams[s.Holder/2], c.Characters[s.Holder]
		ctx.Equipment, ctx.SpecialShot = c.ShotEquipment[s.Holder], a.SpecialShot
	}
	return ctx
}

func commitWorldTeamSpecial(s *WorldActionState, step OriginalTeamSpecialBallStep) error {
	holder := s.Holder
	if step.ChangeSpecialShot {
		if holder < 0 || holder >= 4 {
			return fmt.Errorf("team-special shooter %d outside actors", holder)
		}
		s.Actors[holder].SpecialShot = step.SpecialShot
	}
	if step.ClearUnownedSpecialShot {
		s.UnownedSpecialShot = 0
	}
	s.Ball, s.SelectedHoop, s.LaunchTarget, s.LaunchArcHeight, s.BallLanding = step.Ball, step.SelectedHoop, step.Target, step.ArcHeight, step.Landing
	if step.ReleaseHolder {
		s.Holder = 6
	}
	if step.ClearExcludedCatcher {
		s.ExcludedCatcher = -1
	}
	return nil
}

// This adapter commits original entry writes before the bound court loads the
// world. Sounds emitted by that motion stay between entry and landing sounds.
func advanceWorldTeamSpecialBall(result *WorldActionStep, c WorldActionContext, court FlatCourt) error {
	prepared, e := PrepareOriginalTeamSpecialBall(result.State.Ball, worldTeamSpecialContext(result.State, c), PassLaunchContext{SecondPlayerX: result.State.Actors[1].Player.Body.Position.X})
	if e != nil {
		return e
	}
	if e = commitWorldTeamSpecial(&result.State, prepared); e != nil {
		return e
	}
	result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
	completed, e := CompleteOriginalTeamSpecialBall(prepared, court)
	if e != nil {
		return e
	}
	if e = commitWorldTeamSpecial(&result.State, completed); e != nil {
		return e
	}
	result.SoundIDs = append(result.SoundIDs, completed.SoundIDs...)
	if completed.ChangeCameraShake {
		result.ChangeCameraShake, result.CameraShake = true, completed.CameraShake
	}
	return nil
}
