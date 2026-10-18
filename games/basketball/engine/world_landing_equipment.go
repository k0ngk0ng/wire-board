package engine

func originalLandingAction(action Action) bool {
	switch action {
	case 0x06, 0x16, 0x26, 0x36, 0x46:
		return true
	}
	return false
}

// selectOriginalLandingAction is 8BE0's held-B gate followed by EE52..EEF3.
// The caller must already have run animation and 8B90: the action can have
// changed by then, so checking the current action here would reject valid calls.
// Equipment arrays are actor-indexed projections of the character's slots.
func selectOriginalLandingAction(s *WorldActionState, actor uint8, c WorldActionContext) {
	if c.ActorHeldButtons[actor]&ButtonB == 0 || c.Controls[actor]&128 != 0 || s.Holder != int8(actor) {
		return
	}
	if s.PlayerLinks[actor^1] == 128|actor {
		return
	}
	a := &s.Actors[actor]
	equipment := c.EnhancementEnabled && c.Teams[actor/2] == 0
	pants := c.Enhancements[actor]
	zone := uint8(c.CachedZones[actor]) & 0xf0
	depth := uint8(a.Player.Body.Position.Depth >> 8)
	choose := func(index uint8) {
		a.Player.Action = [4]Action{0x5a, 0x5c, 0x5a, 0x5c}[index]
		a.Player.Entered = false
	}
	// A matching equipment zone suppresses the ordinary fallback even when
	// depth fails. A nonmatching zone (or no equipment) takes EEC2 instead.
	pantsZone := equipment && (pants == 11 && zone == 0x40 || pants == 15 && zone == 0x50)
	if pantsZone {
		if depth >= 25 && depth <= 57 {
			choose(c.Characters[actor])
		}
	} else {
		if c.ActorHeldButtons[actor]&ButtonUp != 0 {
			a.SpecialShot = 128
		}
		// Source compares only $75,X, preserving its high-X-byte alias behavior.
		x := uint8(a.Player.Body.Position.X >> 8)
		lower, upper := uint8(143), uint8(207)
		if actor&2 != 0 {
			lower, upper = 64, 128
		}
		if x > lower && x <= upper {
			if depth >= 25 && depth <= 57 {
				choose(c.Characters[actor])
			} else {
				a.SpecialShot = 0
			}
		}
	}
	// EE70 follows both paths, including failed position/depth selection.
	if equipment && c.JumpEnhancements != nil && c.JumpEnhancements[actor] == 30 {
		var value byte
		s.Random, value = NextRandom(s.Random)
		choose(value & 3)
	}
}

// advanceWorldLandingPlayer preserves the shared bank6 8BBD order for every
// alias: entry -> animation -> attack-next-action -> low-jump choice -> EEFF.
// It commits choices before court motion so subsequent device writes win.
func advanceWorldLandingPlayer(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	action := p.Action
	if !p.Entered {
		clearOriginalLandingSpecialShot(a)
		initializeAction(p)
		stopHorizontal(p)
		stopVertical(p)
		p.Animation.NextAction = action & 0x70
		p.Animation.HasNextAction = true
	}
	// AA82 points all aliases to AB7A except prop-carry landing26 -> AB77.
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 5}, Ticks: 4}}, End: RepeatRemaining}
	if action == 0x26 {
		spec.Frames[0] = AnimationFrame{Pose: Pose{Frame: 16}, Ticks: 2}
	}
	animation, e := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if e != nil {
		return e
	}
	p.Animation, p.Pose = animation.State, animation.Pose
	if animation.ChangeAction {
		p.Action, p.Entered = animation.Action, false
	}
	// 8B90 uses the post-animation action and a low-four-bit rider comparison;
	// EE52's later rider comparison deliberately uses all seven low bits.
	link := s.PlayerLinks[actor^1]
	if p.Action&0x70 == 0 && !(link&128 != 0 && link&15 == actor) && c.ActorPressedButtons != nil {
		pressed := c.ActorPressedButtons[actor] & (ButtonA | ButtonB)
		if pressed != 0 {
			p.Animation.NextAction = 0x67
			if pressed == ButtonA {
				p.Animation.NextAction = 0x63
			}
			p.Animation.HasNextAction = true
		}
	}
	selectOriginalLandingAction(s, actor, c)
	ball, e := advancePlayerOnCourt(p, playerTerrainContext{Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor)}, court)
	if e != nil {
		return e
	}
	s.Ball = ApplyBallCommand(s.Ball, ball)
	return nil
}
