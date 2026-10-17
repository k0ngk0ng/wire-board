package engine

import "fmt"

// advanceWorldAuxiliaryRunningPlayer is $9FA7's action68 role dispatch. Object
// commands precede EEFF, so court devices observe the actual changed links.
func advanceWorldAuxiliaryRunningPlayer(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	if c.Teams[actor/2] >= 8 || c.Characters[actor] >= 4 {
		return fmt.Errorf("running auxiliary role outside original table")
	}
	role := int(c.Teams[actor/2])*4 + int(c.Characters[actor])
	selected := originalAttackProfiles[8][role]
	if selected.Handler == 8 && c.ActorPressedButtons == nil {
		return fmt.Errorf("running auxiliary attack requires original actor pressed buttons")
	}
	if !p.Entered {
		a.AttackVariant = selected.Variant
	}
	if selected.Handler&128 != 0 {
		p.Action, p.Entered = 0x23, false
		return advanceWorldAuxiliaryCarrier(result, actor, c, court)
	}
	if selected.Handler != 8 && selected.Handler != 9 {
		return fmt.Errorf("unexpected running auxiliary handler %d", selected.Handler)
	}
	sounds := []uint8{}
	soundStart := len(result.SoundIDs)
	if !p.Entered {
		initializeAction(p)
		p.Animation.NextAction, p.Animation.HasNextAction = ActionIdle, true
		object := uint8(4)
		if s.Auxiliary[0].Owner != int8(actor) {
			object = 5
		}
		aux := &s.Auxiliary[object-4]
		if selected.Handler == 9 {
			aux.Action, aux.Entered, aux.Attachment = 10, false, 0
			p.Animation.Loops = 3
		} else {
			sounds = append(sounds, originalAttackSounds[8][role])
			if s.PlayerLinks[actor]&128 != 0 {
				p.Animation.NextAction = ActionRiding
			}
			aux.Action, aux.Entered = 9, false
			s.Auxiliary[0].ObjectLink, s.Auxiliary[1].ObjectLink = int8(actor), int8(object)
		}
	}
	if selected.Handler == 9 {
		sounds = append(sounds, originalAttackSounds[8][role])
	}
	if err := advanceRunningAttackAnimation(p, a.AttackVariant); err != nil {
		return err
	}
	if selected.Handler == 8 {
		if p.Animation.Cursor < 3 && c.ActorPressedButtons[actor]&ButtonA != 0 {
			p.Animation.Cursor, p.Animation.Elapsed = 3, 0
			target := s.Auxiliary[1].ObjectLink
			if target >= 0 {
				if err := setWorldRunningObjectAction(s, uint8(target), 11); err != nil {
					return err
				}
			}
			sounds = append(sounds, 48)
		}
		p.Body.Velocity.X = approachZero(p.Body.Velocity.X, 64)
		p.Body.Velocity.Depth = approachZero(p.Body.Velocity.Depth, 64)
	}
	ctx := AuxiliaryCarrierContext{Actor: actor, Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor), Attachment: s.PlayerLinks[actor]}
	if ctx.Attachment&128 != 0 {
		support, err := worldAuxiliarySupport(*s, c, ctx.Attachment&15)
		if err != nil {
			return err
		}
		ctx.Support = support
	}
	link, ball, err := advanceAuxiliaryCarrierPlayer(p, ctx, false, court)
	if err != nil {
		return err
	}
	s.PlayerLinks[actor] = link
	s.Ball = ApplyBallCommand(s.Ball, ball)
	appendPlayerActionSounds(result, soundStart, sounds)
	return nil
}

func advanceRunningAttackAnimation(p *Player, variant uint8) error {
	spec, ok := originalAttackAnimations[variant]
	if !ok {
		return fmt.Errorf("running auxiliary animation %d unavailable", variant)
	}
	state := p.Animation
	tail := variant == 63 && state.Cursor >= 3
	if tail {
		if state.Cursor > 4 {
			return fmt.Errorf("running auxiliary release cursor outside original tail")
		}
		// The original jumps onto the FF 00 terminator as a one-update pose127,
		// then plays pose87 for six ticks. This is a real branch, not a new spec.
		if state.Cursor == 3 {
			p.Pose = Pose{Frame: 127, Mirrored: !p.FacingLeft}
			p.Animation.Cursor, p.Animation.Elapsed = 4, 0
			return nil
		}
		spec = AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 87}, Ticks: 6}}, End: RepeatRemaining}
		state.Cursor = 0
	}
	next, err := AdvanceAnimation(spec, state, p.FacingLeft)
	if err != nil {
		return err
	}
	p.Animation, p.Pose = next.State, next.Pose
	if tail && !(next.State.Cursor == 0 && next.State.Elapsed == 0) {
		p.Animation.Cursor = 4
	}
	if next.ChangeAction {
		p.Action, p.Entered = next.Action, false
	}
	return nil
}

// AA2B uses the canonical 0570 reference. Preserve valid non-auxiliary aliases
// as object action writes; out-of-world references are not guessed.
func setWorldRunningObjectAction(s *WorldActionState, object uint8, action Action) error {
	return s.applyHurtObject(HurtObjectCommand{Object: object, SetAction: true, Action: uint8(action)})
}

type auxiliaryRunningSource struct {
	Body       Body
	Pose       Pose
	FacingLeft bool
	Animation  AnimationState
}

func worldAuxiliaryRunningSource(s WorldActionState, object int8) (auxiliaryRunningSource, error) {
	switch {
	case object < 0 || object >= 13:
		return auxiliaryRunningSource{}, fmt.Errorf("running auxiliary owner %d outside world", object)
	case object < 4:
		p := s.Actors[object].Player
		return auxiliaryRunningSource{p.Body, p.Pose, p.FacingLeft, p.Animation}, nil
	case object < 6:
		p := s.Auxiliary[object-4]
		return auxiliaryRunningSource{p.Body, p.Pose, p.FacingLeft, p.Animation}, nil
	case object == 6:
		p := s.Ball
		return auxiliaryRunningSource{p.Body, p.Pose, p.FacingLeft, p.Animation}, nil
	default:
		p := s.Hoops[object-7]
		return auxiliaryRunningSource{Body{Position: p.Position, Velocity: p.Velocity, Gravity: p.Gravity}, p.Pose, p.FacingLeft, p.Animation}, nil
	}
}

func advanceWorldAuxiliaryRunningObject(result *WorldActionStep, object uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Auxiliary[object-4]
	action := a.Action
	if action < 9 || action > 11 {
		return fmt.Errorf("running auxiliary object action outside9..11")
	}
	if !a.Entered {
		a.Entered = true
		a.Animation.Cursor, a.Animation.Elapsed, a.Animation.Loops = 0, 0, 0
		switch action {
		case 9:
			a.Body.Velocity = Velocity{Height: 11 * Pixel}
			a.Body.Gravity = Pixel
			target := s.Auxiliary[1].ObjectLink
			if target < 0 || target >= 7 {
				return fmt.Errorf("auxiliary lift attachment target %d unavailable", target)
			}
			links := s.actionLinks()
			links[target] = 0
			s.setActionLinks(links)
		case 10:
			owner, err := worldAuxiliaryRunningSource(*s, a.Owner)
			if err != nil {
				return err
			}
			a.Animation.NextAction, a.Animation.HasNextAction = 1, true
			a.Animation.Loops = owner.Animation.Loops
			a.FacingLeft = owner.Pose.Mirrored
		case 11:
			a.Body.Velocity.Height, a.Body.Gravity = 0, 0
			target := uint8(0)
			if uint8(s.Auxiliary[0].ObjectLink) < 2 {
				target = 2
			}
			first, second := s.Actors[target].Player.Body.Position, s.Actors[target+1].Player.Body.Position
			distance := func(x Fixed) int32 { return absPass(int32(int16(uint16(x>>8) - uint16(a.Body.Position.X>>8)))) }
			if distance(second.X) < distance(first.X) {
				first = second
			}
			velocity := func(delta int32) Fixed { v := uint16(max(1, absPass(delta)/8)); return Fixed(int16(v<<8 | v>>8)) }
			dx := int32(int16(uint16(first.X>>8) - uint16(a.Body.Position.X>>8)))
			dy := int32(uint8(first.Depth>>8)) - int32(uint8(a.Body.Position.Depth>>8))
			dz := int32(int16(uint16(first.Height>>8) - uint16(a.Body.Position.Height>>8)))
			a.Body.Velocity.X = velocity(dx)
			if dx < 0 {
				a.Body.Velocity.X = signed16(-a.Body.Velocity.X)
			}
			a.Body.Velocity.Depth = velocity(dy)
			if dy < 0 {
				a.Body.Velocity.Depth = signed16(-a.Body.Velocity.Depth)
			}
			a.Body.Velocity.Height = velocity(dz)
			if a.Body.Velocity.Height >= 0 {
				a.Body.Velocity.Height = signed16(-a.Body.Velocity.Height)
			}
			s.Auxiliary[0].ObjectLink, s.Auxiliary[1].ObjectLink, a.Owner = -1, -1, -1
		}
	}
	if action == 10 {
		owner, err := worldAuxiliaryRunningSource(*s, a.Owner)
		if err != nil {
			return err
		}
		offsets := [5]int8{24, 0, -24, -8, 8}
		if a.Animation.Cursor >= 5 {
			return fmt.Errorf("auxiliary spinning cursor outside original table")
		}
		dx := offsets[a.Animation.Cursor]
		if owner.FacingLeft {
			dx = -dx
		}
		a.Body.Position.X = signed24((owner.Body.Position.X &^ 255) + Fixed(dx)*Pixel + (a.Body.Position.X & 255))
		// B2C8/B2D0 discard carry and only replace the low integer byte.
		a.Body.Position.Height = (a.Body.Position.Height &^ Fixed(0xff00)) | Fixed(uint8(owner.Body.Position.Height>>8)+16)<<8
		a.Body.Position.Depth = (a.Body.Position.Depth & 255) | Fixed(uint8(owner.Body.Position.Depth>>8)+1)<<8
	}
	spec := AnimationSpec{Frames: []AnimationFrame{{Ticks: 255}, {Ticks: 255}}, End: RepeatForever}
	if action == 10 {
		spec = AnimationSpec{Frames: []AnimationFrame{{Ticks: 1}, {Pose: Pose{Frame: 127}, Ticks: 1}, {Pose: Pose{Mirrored: true}, Ticks: 1}, {Pose: Pose{Mirrored: true}, Ticks: 1}, {Ticks: 1}}, End: RepeatUntilCount, RepeatLimit: 4}
	}
	next, err := AdvanceAnimation(spec, a.Animation, a.FacingLeft)
	if err != nil {
		return err
	}
	a.Animation, a.Pose = next.State, next.Pose
	if next.ChangeAction {
		a.Action, a.Entered = uint8(next.Action), false
	}
	if action == 9 {
		high := uint8(a.Body.Position.Height >> 8)
		reverse := high >= 64 && a.Body.Gravity < 0 || high < 64 && a.Body.Gravity >= 0 && a.Body.Velocity.Height < 0
		if reverse {
			a.Body.Gravity = signed16(-a.Body.Gravity)
		}
	}
	if action == 11 && !a.Airborne {
		a.Action, a.Entered = 0, false
	}
	ctx := AuxiliaryContext{Object: object, Court: c.CourtID}
	if a.Attachment&128 != 0 {
		ctx.Support, err = worldAuxiliarySupport(*s, c, a.Attachment&15)
		if err != nil {
			return err
		}
	}
	if err = advanceAuxiliaryCarryMotion(a, ctx, court); err != nil {
		return err
	}
	if action == 10 && !a.Animation.HasNextAction {
		a.Attachment = uint8(a.Owner) | 192
	}
	return nil
}
