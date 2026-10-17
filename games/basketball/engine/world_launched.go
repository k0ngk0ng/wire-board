package engine

import "fmt"

func worldLaunchedSource(s WorldActionState, c WorldActionContext, object uint8) (Player, uint8, error) {
	switch {
	case object < 4:
		return s.Actors[object].Player, c.Teams[object/2], nil
	case object < 6:
		a := s.Auxiliary[object-4]
		return Player{Body: a.Body, Action: Action(a.Action), Pose: a.Pose, FacingLeft: a.FacingLeft}, worldAuxiliaryKind(s, c.CourtID, int(object-4)), nil
	case object == 6:
		a := s.Ball
		return Player{Body: a.Body, Action: Action(a.Action), Pose: a.Pose, FacingLeft: a.FacingLeft}, 8, nil
	case object < 13:
		a := s.Hoops[object-7]
		return Player{Body: Body{Position: a.Position, Velocity: a.Velocity, Gravity: a.Gravity}, Action: Action(a.Action), Pose: a.Pose, FacingLeft: a.FacingLeft}, 9, nil
	default:
		return Player{}, 0, fmt.Errorf("launched source object %d outside original world", object)
	}
}
func advanceWorldLaunchedPlayer(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	ctx := LaunchedPlayerContext{BallPosition: s.Ball.Body.Position, Actor: actor, Attachment: s.PlayerLinks[actor], Random: s.Random, Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), Team: c.Teams[actor/2], Character: c.Characters[actor], EnhancementEnabled: c.EnhancementEnabled, SpecialShot: a.SpecialShot, ShotCycle: a.ShotCycle, HeldButtons: c.ActorHeldButtons[actor], AuxiliaryOwners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}, Holder: s.Holder}
	if ctx.Attachment&128 != 0 {
		var e error
		ctx.Source, ctx.SourceKind, e = worldLaunchedSource(*s, c, ctx.Attachment&15)
		if e != nil {
			return e
		}
	}
	if !a.Player.Entered && a.Player.Action == ActionLaunched {
		if c.EnhancementEnabled && ctx.Team == 0 {
			holder, _, e := worldLaunchedSource(*s, c, uint8(s.Holder))
			if e != nil {
				return e
			}
			ctx.HolderReleasing = holder.Action == ActionShoot || holder.Action == ActionAirShoot
			ctx.Equipment = c.Enhancements[actor]
			if ctx.HolderReleasing {
				ctx.Equipment = c.ShotEquipment[actor]
			}
			if s.Holder >= 0 && s.Holder < 4 {
				ctx.HolderSpecialShot = s.Actors[s.Holder].SpecialShot
			}
		}
		if s.SelectedHoop >= 7 && s.SelectedHoop < 13 {
			ctx.HighArc = s.MountFlags[s.SelectedHoop-7]&2 != 0
		} else if s.SelectedHoop >= 0 && s.SelectedHoop < 7 {
			ctx.HighArc = c.OtherObjectArcFlags[s.SelectedHoop]&2 != 0
		} else {
			return fmt.Errorf("launched arc selector %d not reconstructed", s.SelectedHoop)
		}
	}
	prepared, e := PrepareLaunchedPlayer(a.Player, ctx, PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
	if e != nil {
		return e
	}
	a.Player = prepared.Player
	s.PlayerLinks[actor] = prepared.Attachment
	ctx.Attachment = prepared.Attachment
	if prepared.ChangeRandom {
		s.Random = prepared.Random
	}
	if prepared.ChangePoseSource {
		s.PoseSource = prepared.PoseSource
	}
	if prepared.ChangeSpecialShot {
		a.SpecialShot = prepared.SpecialShot
	}
	if prepared.ClearHolderSpecialShot {
		if s.Holder < 0 || s.Holder >= 4 {
			return fmt.Errorf("launched special-shot holder %d outside actors", s.Holder)
		}
		s.Actors[s.Holder].SpecialShot = 0
	}
	if prepared.LaunchPlan != nil {
		s.LaunchTarget, s.LaunchArcHeight = prepared.LaunchPlan.Target, prepared.LaunchPlan.ArcHeight
	}
	result.SoundIDs = append(result.SoundIDs, prepared.SoundIDs...)
	completed, e := CompleteLaunchedPlayer(a.Player, ctx, court)
	if e != nil {
		return e
	}
	a.Player = completed.Player
	s.Ball = ApplyBallCommand(s.Ball, completed.Ball)
	return nil
}
