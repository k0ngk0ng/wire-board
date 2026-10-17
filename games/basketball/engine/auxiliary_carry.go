package engine

import "fmt"

// ResolveAuxiliaryRequest is the object-4/5 E18B acceptance path, before
// handler selection. Rejected requests retain their pending state.
func ResolveAuxiliaryRequest(original AuxiliaryState) (AuxiliaryState, error) {
	if !original.Request.Pending {
		return original, nil
	}
	if original.Action >= 16 || original.Request.Action >= 16 {
		return original, fmt.Errorf("auxiliary request outside original table")
	}
	if auxiliaryRequestRejections[original.Request.Action][original.Action/8]&(128>>(original.Action%8)) != 0 {
		return original, nil
	}
	result := original
	result.Action, result.Entered, result.Request.Pending = uint8(result.Request.Action), false, false
	return result, nil
}

func followAuxiliarySource(s *AuxiliaryState, owner *Player) error {
	if owner == nil || owner.Action >= 106 {
		return fmt.Errorf("auxiliary follow source is unavailable")
	}
	offset := auxiliaryFollowOffsets[auxiliaryOwnerOffsets[owner.Action]]
	dx := Fixed(offset.X) * Pixel
	if owner.Pose.Mirrored {
		dx = -dx
	}
	s.Body.Position.X = signed24((owner.Body.Position.X &^ 255) + dx + (s.Body.Position.X & 255))
	s.Body.Position.Height = signed24((owner.Body.Position.Height &^ 255) + Fixed(offset.Height)*Pixel + (s.Body.Position.Height & 255))
	s.Body.Position.Depth = Fixed(uint8(owner.Body.Position.Depth>>8)-offset.Depth)*Pixel + (s.Body.Position.Depth & 255)
	return nil
}

func advanceAuxiliaryCarryAnimation(s *AuxiliaryState, action uint8) error {
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{}, Ticks: 255}, {Pose: Pose{}, Ticks: 255}}, End: RepeatForever}
	if action == 6 {
		spec.Frames = []AnimationFrame{{Pose: Pose{Frame: 127}, Ticks: 255}}
	}
	if action == 8 {
		spec.Frames = []AnimationFrame{{Pose: Pose{Frame: 0}, Ticks: 1}, {Pose: Pose{Frame: 1}, Ticks: 3}, {Pose: Pose{Frame: 2}, Ticks: 3}, {Pose: Pose{Frame: 3}, Ticks: 3}, {Pose: Pose{Frame: 0}, Ticks: 2}}
	}
	if action == 14 || action == 15 {
		spec = AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{}, Ticks: 2}}, End: RepeatRemaining}
	}
	state := s.Animation
	lead := action == 3 && state.Cursor >= 3
	if lead {
		state.Cursor -= 3
	}
	r, err := AdvanceAnimation(spec, state, s.FacingLeft)
	if err != nil {
		return err
	}
	if lead && !(r.State.Cursor == 0 && r.State.Elapsed == 0) {
		r.State.Cursor += 3
	}
	s.Animation, s.Pose = r.State, r.Pose
	if r.ChangeAction {
		s.Action, s.Entered = uint8(r.Action), false
	}
	return nil
}

func advanceAuxiliaryCarryMotion(s *AuxiliaryState, ctx AuxiliaryContext, court FlatCourt) error {
	if s.Attachment&128 != 0 {
		support := ctx.Support
		if support == nil {
			if ctx.Owner == nil || s.Attachment&15 >= 4 || int8(s.Attachment&15) != s.Owner || ctx.OwnerKind >= 16 {
				return fmt.Errorf("attached auxiliary support unavailable")
			}
			support = &RiderSupport{Body: ctx.Owner.Body, Pose: ctx.Owner.Pose, Kind: ctx.OwnerKind}
		}
		if support.Kind >= 16 {
			return fmt.Errorf("attached auxiliary support kind unavailable")
		}
		p := Player{Body: s.Body}
		attachRiderPosition(&p, *support)
		s.Body = p.Body
		return nil
	}
	if court.originalMotion != nil {
		return court.originalMotion.auxiliary(s)
	}
	// The common airborne motion does not apply the player's action-14
	// falling transition to objects 4/5. Handle that object-neutral branch
	// before entering the older player-shaped New York surface adapter.
	next := IntegrateMotion(s.Body)
	clampPlayerTravel(&next, court)
	if s.NewYorkPassage == 0 && next.Position.Height > courtSurface(court, next.Position.X, next.Position.Depth) {
		court.recordMotionSurface(s.Body.Position)
		s.Body, s.Airborne = next, true
		return nil
	}
	p := Player{Body: s.Body, Action: Action(s.Action), Entered: s.Entered, Pose: s.Pose, Animation: s.Animation, Airborne: s.Airborne, FacingLeft: s.FacingLeft, Request: s.Request, NewYorkPassage: s.NewYorkPassage, Heading: HeadingNone}
	if err := advanceAuxiliaryOnCourt(&p, s.Owner < 0, court); err != nil {
		return err
	}
	s.Body, s.Action, s.Entered, s.Pose, s.Animation = p.Body, uint8(p.Action), p.Entered, p.Pose, p.Animation
	s.Airborne, s.FacingLeft, s.Request, s.NewYorkPassage = p.Airborne, p.FacingLeft, p.Request, p.NewYorkPassage
	return nil
}

// StepAuxiliaryCarryThrow implements original actions 1, 2, 3, 6, 14 and 15. Owner
// motion has already run. Following writes PoseSource separately; the action
// never synthesizes contacts, picks a distant owner or reads raw controller RAM.
func StepAuxiliaryCarryThrow(original AuxiliaryState, ctx AuxiliaryContext, court FlatCourt) (AuxiliaryStep, error) {
	initial := AuxiliaryStep{State: original}
	result := initial
	s := &result.State
	if ctx.Object < 4 || ctx.Object > 5 {
		return initial, fmt.Errorf("auxiliary index must be 4 or 5")
	}
	resolved, err := ResolveAuxiliaryRequest(original)
	if err != nil {
		return initial, err
	}
	*s = resolved
	action := s.Action
	if action != 1 && action != 2 && action != 3 && action != 6 && action != 8 && action != 14 && action != 15 {
		return initial, fmt.Errorf("auxiliary carry action %d not reconstructed", action)
	}
	follow := func(owner *Player, source int8) error {
		if source < 0 || source >= 4 {
			return fmt.Errorf("auxiliary source %d not reconstructed", source)
		}
		if err := followAuxiliarySource(s, owner); err != nil {
			return err
		}
		result.ChangePoseSource, result.PoseSource = true, uint8(source)
		return nil
	}
	if !s.Entered {
		s.Entered = true
		s.Animation.Cursor, s.Animation.Elapsed, s.Animation.Loops = 0, 0, 0
		switch action {
		case 1:
			s.Body.Velocity.X, s.Body.Velocity.Depth = 0, 0
			s.Airborne = true
		case 2:
			if ctx.Launch == nil {
				return initial, fmt.Errorf("auxiliary lob entry requires original arc flags")
			}
			var ownerHeight Fixed
			if s.Owner >= 0 && s.Owner < 4 {
				if ctx.Owner == nil {
					return initial, fmt.Errorf("auxiliary lob player owner is unavailable")
				}
				s.FacingLeft, ownerHeight = ctx.Owner.FacingLeft, ctx.Owner.Body.Position.Height
			} else {
				if ctx.Launch.OwnerHeight == nil {
					return initial, fmt.Errorf("auxiliary lob requires explicit non-player owner height")
				}
				// B089 skips copying facing when raw Owner >= 4.
				ownerHeight = *ctx.Launch.OwnerHeight
			}
			plan, err := ChooseOrdinaryShot(s.Body.Position, ownerHeight, s.FacingLeft, [6]uint8{})
			if err != nil {
				return initial, err
			}
			if ctx.Launch.ArcFlags&2 != 0 {
				plan.ArcHeight = 48 * Pixel
			}
			body, err := court.launchPass(s.Body.Position, plan.Target, plan.ArcHeight)
			if err != nil {
				return initial, err
			}
			s.Body = body
			s.Owner, s.Attachment = -1, 0
			result.LaunchPlan = &AuxiliaryLaunchPlan{Target: plan.Target, ArcHeight: plan.ArcHeight}
		case 3, 8:
			s.Animation.Cursor = 3
			if action == 8 && ctx.OwnerFacing != nil {
				s.FacingLeft = *ctx.OwnerFacing
			} else {
				if s.Owner < 0 || s.Owner >= 4 || ctx.Owner == nil {
					return initial, fmt.Errorf("throw entry requires old owner facing")
				}
				s.FacingLeft = ctx.Owner.FacingLeft
			}
			s.Body.Velocity.X = 12 * Pixel
			if s.FacingLeft {
				s.Body.Velocity.X = -12 * Pixel
			}
			s.Body.Velocity.Height, s.Body.Gravity = -3*Pixel, 0
			s.Owner, s.Attachment = -1, 0
		case 6:
			s.Body.Velocity, s.Body.Gravity = Velocity{}, 0
			s.Owner, s.Attachment = -1, 0
		case 15:
			s.Body.Velocity, s.Body.Gravity = Velocity{}, 0
			s.Airborne = true
			s.Animation.NextAction, s.Animation.HasNextAction = 2, true
		case 14:
			s.Body.Velocity, s.Body.Gravity = Velocity{}, 0
			s.Airborne = true
			s.Animation.NextAction, s.Animation.HasNextAction = 3, true
			if err := follow(ctx.Owner, s.Owner); err != nil {
				return initial, err
			}
		}
	}
	if err := advanceAuxiliaryCarryAnimation(s, action); err != nil {
		return initial, err
	}
	if action == 1 || action == 15 {
		if err := follow(ctx.Owner, s.Owner); err != nil {
			return initial, err
		}
	}
	if action == 6 {
		if err := follow(ctx.SpecialOwner, ctx.SpecialHolder); err != nil {
			return initial, err
		}
	}
	if err := advanceAuxiliaryCarryMotion(s, ctx, court); err != nil {
		return initial, err
	}
	if action == 2 && !s.Airborne {
		s.Action, s.Entered = 0, false
	}
	if action == 3 || action == 8 {
		if !s.Airborne {
			s.Action, s.Entered = 0, false
		} else if s.Body.Velocity.X == 0 {
			s.Body.Gravity = Pixel
			s.Animation.Cursor, s.Animation.Elapsed = 0, 0
		}
	}
	return result, nil
}
