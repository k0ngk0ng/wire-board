package engine

import "fmt"

func originalHoopSharedAction(action Action) bool {
	return action == 0x31 || action == 0x33 || action == 0x37 || action == 0x38
}

// These are original shared handler aliases, not conversions to unarmed IDs.
// Preserve the original action during animation and court-dependent motion.
func advanceWorldHoopShared(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	p := &s.Actors[actor].Player
	action := p.Action
	// Court1 normally uses a flat fast path; an attached carrier must run the
	// real EEFF dependency motion just as on every other court.
	if s.PlayerLinks[actor]&128 != 0 && court.originalMotion == nil {
		var err error
		court, err = originalWorldMotionCourt(result, c, actor)
		if err != nil {
			return err
		}
	}
	if !originalHoopSharedAction(action) {
		return fmt.Errorf("unsupported shared hoop action %x", action)
	}
	if !p.Entered {
		initializeAction(p)
		if action == 0x37 {
			p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
			if s.PlayerLinks[actor]&128 != 0 {
				p.Animation.NextAction = ActionRiding
			}
		}
	}
	if action == 0x31 {
		faceHeading(p)
	}
	var spec AnimationSpec
	switch action {
	case 0x31:
		spec = c.Profiles[actor].Animations[ActionWalk]
	case 0x33:
		spec = c.Profiles[actor].Animations[ActionBrake]
	case 0x37:
		spec = AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 43}, Ticks: 4}, {Pose: Pose{Frame: 20}, Ticks: 1}, {Pose: Pose{Frame: 20}, Ticks: 6}}, End: RepeatRemaining}
	case 0x38:
		spec = AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 43}, Ticks: 4}, {Pose: Pose{Frame: 20}, Ticks: 1}, {Pose: Pose{Frame: 20}, Ticks: 255}, {Pose: Pose{Frame: 20}, Ticks: 255}}, End: RepeatForever}
	}
	anim, err := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if err != nil {
		return err
	}
	p.Animation, p.Pose = anim.State, anim.Pose
	if anim.ChangeAction {
		p.Action, p.Entered = anim.Action, false
	}
	if (action == 0x37 || action == 0x38) && p.Animation.Cursor == 1 {
		index := 0
		for index < 5 && s.Hoops[index].Owner != actor {
			index++
		}
		h := &s.Hoops[index]
		h.Action, h.Entered, h.FacingLeft = HoopLobbed, false, p.FacingLeft
		s.HoopHeadings[index] = HeadingRight
		if p.FacingLeft {
			s.HoopHeadings[index] = HeadingLeft
		}
	}
	switch action {
	case 0x31:
		v := c.Profiles[actor].Walk[p.Heading]
		p.Body.Velocity.X, p.Body.Velocity.Depth = v.X, v.Depth
	case 0x33:
		brakePlayer(p, c.Profiles[actor])
	case 0x37:
		p.Body.Velocity.X = approachZero(p.Body.Velocity.X, 128)
		if p.Body.Velocity.Depth > 0 {
			p.Body.Velocity.Depth -= 16
		} else if p.Body.Velocity.Depth < 0 {
			p.Body.Velocity.Depth += 16
		}
	case 0x38:
		if s.CarriedPlayer == int8(actor) {
			return advanceWorldCarriedAirAction(result, actor, c)
		}
	}
	ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor)}, court)
	if err != nil {
		return err
	}
	s.Ball = ApplyBallCommand(s.Ball, ball)
	switch action {
	case 0x31:
		if p.Airborne {
			p.Action, p.Entered = ActionFalling, false
		}
	case 0x33:
		if p.Body.Velocity.X == 0 {
			p.Action, p.Entered = p.Action&0x70, false
			if s.PlayerLinks[actor]&128 != 0 {
				p.Action = ActionRiding
			}
		}
	case 0x38:
		if !p.Airborne {
			p.Action, p.Entered = ActionLand, false
		}
	}
	return nil
}

// Fixed DF39: follow the previously owned hoop, apply E9B6's independent
// direction, launch vertically, release ownership, then animate and move.
// Heading is persisted separately: original entered flight must not infer it
// from current facing, and the owner can change its facing after dispatch.
func advanceWorldHoopLobbed(result *WorldActionStep, object uint8, c WorldActionContext, court FlatCourt) error {
	if object < 7 || object >= 13 {
		return fmt.Errorf("invalid hoop object %d", object)
	}
	s := &result.State
	index := object - 7
	h := &s.Hoops[index]
	if h.Request.Pending && h.Request.Action != 0 {
		return fmt.Errorf("hoop lob request %d not reconstructed", h.Request.Action)
	}
	if !h.Entered {
		if h.Owner >= 4 {
			return fmt.Errorf("hoop lob owner %d outside player state", h.Owner)
		}
		heading := s.HoopHeadings[index]
		if heading > HeadingNone {
			return fmt.Errorf("invalid hoop lob heading %d", heading)
		}
		h.Entered = true
		h.Animation.Cursor, h.Animation.Elapsed, h.Animation.Loops = 0, 0, 0
		s.PoseSource = h.Owner
		component := h.component()
		if err := positionCarriedHoop(&component, s.Actors[h.Owner].Player); err != nil {
			return err
		}
		h.Position = component.Position
		travel := [9]Velocity{{Depth: 1600}, {X: 1664, Depth: 1280}, {X: 2048}, {X: 1664, Depth: -1536}, {Depth: -1600}, {X: -1664, Depth: -1536}, {X: -2048}, {X: -1664, Depth: 1280}, {}}
		h.Velocity = travel[heading]
		h.Velocity.Height = 6 * Pixel
		h.Gravity = Pixel
		h.Owner = 15
	}
	anim, err := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 1, Mirrored: true}, Ticks: 255}}, End: RepeatForever}, h.Animation, h.FacingLeft)
	if err != nil {
		return err
	}
	h.Animation, h.Pose = anim.State, anim.Pose
	if anim.ChangeAction {
		h.Action, h.Entered = HoopAction(anim.Action), false
	}
	component := h.component()
	if err = advanceLooseHoop(&component, court); err != nil {
		return err
	}
	h.applyComponent(component, index)
	if !h.Airborne {
		h.Action, h.Entered = HoopInactive, false
	}
	return nil
}

// Bank6 B117 is the auxiliary braking state selected by carried-air landing.
// It uses EA5C's row7 (64 in each horizontal direction), then EC4A clears both
// horizontal axes when X stops. The remaining original court motion still runs.
func advanceWorldAuxiliaryBrake(result *WorldActionStep, object uint8, c WorldActionContext, court FlatCourt) error {
	if object < 4 || object > 5 {
		return fmt.Errorf("invalid braking object %d", object)
	}
	a := &result.State.Auxiliary[object-4]
	if !a.Entered {
		a.Entered = true
		a.Animation.Cursor, a.Animation.Elapsed, a.Animation.Loops = 0, 0, 0
	}
	anim, err := AdvanceAnimation(AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{}, Ticks: 255}, {Pose: Pose{}, Ticks: 255}}, End: RepeatForever}, a.Animation, a.FacingLeft)
	if err != nil {
		return err
	}
	a.Animation, a.Pose = anim.State, anim.Pose
	if anim.ChangeAction {
		a.Action, a.Entered = uint8(anim.Action), false
	}
	a.Body.Velocity.X = approachZero(a.Body.Velocity.X, 64)
	if a.Body.Velocity.Depth > 0 {
		a.Body.Velocity.Depth = signed16(a.Body.Velocity.Depth - 64)
	} else if a.Body.Velocity.Depth < 0 {
		a.Body.Velocity.Depth = signed16(a.Body.Velocity.Depth + 64)
	}
	if a.Body.Velocity.X == 0 {
		a.Action, a.Entered = 0, false
		a.Body.Velocity.Depth = 0
	}
	if court.originalMotion == nil {
		court, err = originalWorldMotionCourt(result, c, object)
		if err != nil {
			return err
		}
	}
	return court.originalMotion.auxiliary(a)
}
