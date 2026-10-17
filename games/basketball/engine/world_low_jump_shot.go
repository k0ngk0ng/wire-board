package engine

import "fmt"

// advanceWorldLowJumpShot restores bank 6 $9040, the two character-specific
// follow-through actions selected by the original low-jump shot plan.
func advanceWorldLowJumpShot(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	action := p.Action
	if action != 0x5a && action != 0x5c {
		return fmt.Errorf("low-jump shot action %02x outside original handler", action)
	}
	if !p.Entered {
		ctx := LaunchedPlayerContext{Actor: actor, Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), Team: c.Teams[actor/2], Character: c.Characters[actor], EnhancementEnabled: c.EnhancementEnabled, SpecialShot: a.SpecialShot, ShotCycle: a.ShotCycle, HeldButtons: c.ActorHeldButtons[actor], Holder: s.Holder}
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
			return fmt.Errorf("low-jump shot arc selector %d not reconstructed", s.SelectedHoop)
		}
		initial := LaunchedPlayerPreparation{Player: *p}
		prepared := initial
		initializeAction(&prepared.Player)
		source := prepared.Player
		source.FacingLeft = actor >= 2
		prepared, e := prepareOriginalPlayerTrajectory(initial, prepared, source, ctx, PassLaunchContext{SecondPlayerX: s.Actors[1].Player.Body.Position.X})
		if e != nil {
			return e
		}
		*p = prepared.Player
		s.Ball.Action, s.Ball.Entered = BallHeld, false
		if action == 0x5a {
			s.Ball.Pose = Pose{Frame: 127}
		}
		if prepared.ChangeSpecialShot {
			a.SpecialShot = prepared.SpecialShot
		}
		if prepared.ClearHolderSpecialShot {
			s.Actors[s.Holder].SpecialShot = 0
		}
		if prepared.LaunchPlan != nil {
			s.LaunchTarget, s.LaunchArcHeight = prepared.LaunchPlan.Target, prepared.LaunchPlan.ArcHeight
		}
	}
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 59}, Ticks: 1}, {Pose: Pose{Frame: 60}, Ticks: 1}, {Pose: Pose{Frame: 61}, Ticks: 1}, {Pose: Pose{Frame: 62}, Ticks: 1}}, End: RepeatUntilCount, RepeatLimit: 255}
	if action == 0x5a {
		spec.Frames = []AnimationFrame{{Pose: Pose{Frame: 4}, Ticks: 1}, {Pose: Pose{Frame: 4}, Ticks: 1}, {Pose: Pose{Frame: 50}, Ticks: 1}, {Pose: Pose{Frame: 50}, Ticks: 1}, {Pose: Pose{Frame: 4, Mirrored: true}, Ticks: 1}, {Pose: Pose{Frame: 4, Mirrored: true}, Ticks: 1}}
	}
	animation, e := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if e != nil {
		return e
	}
	p.Animation, p.Pose = animation.State, animation.Pose
	if animation.ChangeAction {
		p.Action, p.Entered = animation.Action, false
	}
	dunk := CarrierStep{Carrier: Carrier{Player: *p, ShotCycle: a.ShotCycle}}
	search := DunkSearchContext{BallPosition: s.Ball.Body.Position, MountFlags: s.MountFlags}
	for i, h := range s.Hoops {
		search.HoopOwners[i] = h.Owner
	}
	selectOriginalDunkTarget(&dunk, search)
	*p = dunk.Carrier.Player
	if dunk.ChangeDunkTarget {
		s.DunkTargetObject = dunk.DunkTargetObject
	}
	ball, e := advancePlayerOnCourt(p, playerTerrainContext{HasBall: s.Holder == int8(actor), Computer: c.Controls[actor]&128 != 0}, court)
	if e != nil {
		return e
	}
	s.Ball = ApplyBallCommand(s.Ball, ball)
	if !p.Airborne {
		p.Action, p.Entered = ActionCarrierLand, false
	}
	return nil
}
