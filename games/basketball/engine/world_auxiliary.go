package engine

import "fmt"

func worldAuxiliarySupport(s WorldActionState, c WorldActionContext, object uint8) (*RiderSupport, error) {
	if object >= 13 {
		return nil, fmt.Errorf("auxiliary support object %d outside world", object)
	}
	o := worldFloorState(s, c).Objects[object]
	return &RiderSupport{Body: o.Body, Pose: o.Pose, Kind: o.Kind}, nil
}

func worldAuxiliaryPlayerAction(s WorldActionState, actor uint8) bool {
	action := s.Actors[actor].Player.Action
	if action == 0x4e || action >= 0x20 && action <= 0x2c {
		return true
	}
	return action == ActionFalling && (s.Auxiliary[0].Owner == int8(actor) || s.Auxiliary[1].Owner == int8(actor))
}

func advanceWorldAuxiliaryCarrier(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	ctx := AuxiliaryCarrierContext{Actor: actor, HasBall: s.Holder == int8(actor), Computer: c.Controls[actor]&128 != 0, Owners: [2]int8{s.Auxiliary[0].Owner, s.Auxiliary[1].Owner}, Attachment: s.PlayerLinks[actor], CarriedPlayer: &s.CarriedPlayer, EnhancementEnabled: c.EnhancementEnabled, Team: c.Teams[actor/2]}
	if c.PickupFlags != nil {
		ctx.PickupFlags, ctx.PickupFlagsKnown = c.PickupFlags[actor], true
	}
	if c.JumpEnhancements != nil {
		ctx.JumpEnhancement = &c.JumpEnhancements[actor]
	}
	if ctx.Attachment&128 != 0 {
		support, err := worldAuxiliarySupport(*s, c, ctx.Attachment&15)
		if err != nil {
			return err
		}
		ctx.Support = support
	}
	soundStart := len(result.SoundIDs)
	step, err := stepResolvedAuxiliaryCarrier(a.Player, ctx, c.Profiles[actor], court)
	if err != nil {
		return err
	}
	a.Player = step.Player
	s.Ball = ApplyBallCommand(s.Ball, step.Ball)
	if step.ChangeShotCycle {
		a.ShotCycle = step.ShotCycle
	}
	// The shared landing entry committed ClearSpecialShot before motion.
	// Do not replay it over world writes produced by that motion.
	if step.ChangeAimWindow {
		a.AimWindow = step.AimWindow
	}
	if step.ChangeAttachment {
		s.PlayerLinks[actor] = step.Attachment
	} else if step.Revisit {
		s.PlayerLinks[actor] = 0
	}
	if step.ChangeAuxiliary {
		object := &s.Auxiliary[step.Object-4]
		object.Action, object.Entered = step.Action, false
		if step.BindOwner {
			object.Owner, object.Attachment = int8(actor), actor|128
		}
	}
	appendPlayerActionSounds(result, soundStart, step.SoundIDs)
	return nil
}

func advanceWorldAuxiliary(result *WorldActionStep, object uint8, c WorldActionContext, court FlatCourt) error {
	if object < 4 || object > 5 {
		return fmt.Errorf("auxiliary object %d outside world", object)
	}
	s := &result.State
	resolved, err := ResolveAuxiliaryRequest(s.Auxiliary[object-4])
	if err != nil {
		return err
	}
	if resolved.Action >= 9 && resolved.Action <= 11 {
		s.Auxiliary[object-4] = resolved
		return advanceWorldAuxiliaryRunningObject(result, object, c, court)
	}
	if resolved.Action == 4 {
		s.Auxiliary[object-4] = resolved
		return advanceWorldAuxiliaryBrake(result, object, c, court)
	}
	ctx := AuxiliaryContext{Object: object, Court: c.CourtID, SpecialHolder: s.SpecialHolder}
	if resolved.Action == 8 && !resolved.Entered {
		source, e := worldAuxiliaryRunningSource(*s, resolved.Owner)
		if e != nil {
			return e
		}
		ctx.OwnerFacing = &source.FacingLeft
	}
	if resolved.Owner >= 0 && resolved.Owner < 4 {
		ctx.Owner = &s.Actors[resolved.Owner].Player
		ctx.OwnerKind = c.Teams[resolved.Owner/2]
	}
	if resolved.Attachment&128 != 0 {
		ctx.Support, err = worldAuxiliarySupport(*s, c, resolved.Attachment&15)
		if err != nil {
			return err
		}
	}
	if s.SpecialHolder >= 0 && s.SpecialHolder < 4 {
		ctx.SpecialOwner = &s.Actors[s.SpecialHolder].Player
	}
	if resolved.Action == 2 && !resolved.Entered {
		var flags uint8
		if s.SelectedHoop >= 7 && s.SelectedHoop < 13 {
			flags = s.MountFlags[s.SelectedHoop-7]
		} else if s.SelectedHoop >= 0 && s.SelectedHoop < 7 {
			flags = c.OtherObjectArcFlags[s.SelectedHoop]
		} else {
			return fmt.Errorf("auxiliary throw arc selector %d is unavailable", s.SelectedHoop)
		}
		ctx.Launch = &AuxiliaryLaunchContext{ArcFlags: flags}
		if resolved.Owner < 0 || resolved.Owner >= 4 {
			owner := uint8(resolved.Owner) & 127
			if owner == 127 {
				if c.BackgroundLaunchHeight == nil {
					return fmt.Errorf("unowned auxiliary launch requires the last generated background column")
				}
				ctx.Launch.OwnerHeight = c.BackgroundLaunchHeight
			} else if owner < 13 {
				source, _, e := worldLaunchedSource(*s, c, owner)
				if e != nil {
					return e
				}
				height := Fixed(uint16(source.Body.Position.Height>>8)) * Pixel
				ctx.Launch.OwnerHeight = &height
			} else {
				return fmt.Errorf("auxiliary launch owner alias %d is not restored", owner)
			}
		}
	}
	var step AuxiliaryStep
	switch resolved.Action {
	case 1, 2, 3, 6, 8, 14, 15:
		step, err = StepAuxiliaryCarryThrow(resolved, ctx, court)
	default:
		step, err = stepAuxiliaryRestOnCourt(resolved, ctx, court)
	}
	if err != nil {
		return err
	}
	s.Auxiliary[object-4] = step.State
	if step.ChangePoseSource {
		s.PoseSource = step.PoseSource
	}
	if step.ClearSpecialHolder {
		s.SpecialHolder = -1
	}
	if step.ChangeLinkedPose {
		if err = s.setObjectPose(step.LinkedObject, step.LinkedPose); err != nil {
			return err
		}
	}
	if step.LaunchPlan != nil {
		s.LaunchTarget, s.LaunchArcHeight = step.LaunchPlan.Target, step.LaunchPlan.ArcHeight
	}
	return nil
}
