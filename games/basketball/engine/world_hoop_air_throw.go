package engine

import "fmt"

const (
	ActionHoopAirAim   Action = 0x3a
	ActionHoopAirThrow Action = 0x3c
)

// Bank6 9949/9417 write the owned hoop before actor animation and EEFF.
// Commit the command to the world before motion so later court writes win.
func advanceWorldHoopAirAction(result *WorldActionStep, actor uint8, c WorldActionContext, court FlatCourt) error {
	s := &result.State
	a := &s.Actors[actor]
	p := &a.Player
	action := p.Action
	if action != ActionHoopAirAim && action != ActionHoopAirThrow {
		return fmt.Errorf("not an original airborne hoop action: %x", action)
	}
	// Court1 normally uses the flat free-player fast path. An attached actor
	// still needs EEFF's support-relative motion (including 3A's extra call).
	if s.PlayerLinks[actor]&128 != 0 && court.originalMotion == nil {
		var err error
		court, err = originalWorldMotionCourt(result, c, actor)
		if err != nil {
			return err
		}
	}
	if !p.Entered {
		initializeAction(p)
		// Both original loops test owners 0..4, falling through to slot5.
		index := 0
		for index < 5 && s.Hoops[index].Owner != actor {
			index++
		}
		h := &s.Hoops[index]
		h.Action, h.Entered, h.FacingLeft = HoopCarried, false, p.FacingLeft
		if action == ActionHoopAirAim {
			a.AimWindow = 32
		} else {
			h.Action = HoopThrown
			a.SpecialShot = 0
			if c.ActorHeldButtons[actor]&ButtonUp != 0 {
				a.SpecialShot = 128
				// 9448 writes X (0..3) to056D. Its canonical board-contact
				// consumer reads the high2 bits, hence this source write is0.
				result.ChangeBackboardStrength, result.BackboardStrength = true, actor>>6
			}
		}
	}
	// AA82: aim3A aliases ABC1; air throw3C aliases ABCE, as does1C.
	spec := AnimationSpec{Frames: []AnimationFrame{{Pose: Pose{Frame: 11}, Ticks: 255}}, End: RepeatForever}
	if action == ActionHoopAirThrow {
		faceHeading(p)
		spec.Frames = []AnimationFrame{{Pose: Pose{Frame: 12}, Ticks: 5}, {Pose: Pose{Frame: 4}, Ticks: 255}}
	}
	anim, err := AdvanceAnimation(spec, p.Animation, p.FacingLeft)
	if err != nil {
		return err
	}
	p.Animation, p.Pose = anim.State, anim.Pose
	if anim.ChangeAction {
		p.Action, p.Entered = anim.Action, false
	}
	if s.CarriedPlayer == int8(actor) {
		if err := advanceWorldCarriedAirAction(result, actor, c); err != nil {
			return err
		}
		// 3C returns after8C33; 3A additionally executes EEFF for the actor.
		if action == ActionHoopAirThrow {
			return nil
		}
	} else if action == ActionHoopAirAim && !p.Airborne {
		p.Action, p.Entered = 0x36, false
	}
	ball, err := advancePlayerOnCourt(p, playerTerrainContext{Computer: c.Controls[actor]&128 != 0, HasBall: s.Holder == int8(actor)}, court)
	if err != nil {
		return err
	}
	s.Ball = ApplyBallCommand(s.Ball, ball)
	if action == ActionHoopAirThrow && !p.Airborne {
		p.Action, p.Entered = ActionIdle, false
	}
	return nil
}

// Bank6 8C33 ->9C4E moves the supporting body first, then attaches the rider,
// copies velocity back and clears support motion. 8C33 then checks support air
// before selecting actor40 and auxiliary4 action4. Callers own any later EEFF.
func advanceWorldCarriedAirAction(result *WorldActionStep, actor uint8, c WorldActionContext) error {
	s := &result.State
	support := s.PlayerLinks[actor] & 15
	if support >= 13 {
		return fmt.Errorf("carried air action support %d outside world", support)
	}
	f := worldFloorState(*s, c)
	f.Objects[support].Body.Velocity = f.Objects[actor].Body.Velocity
	f.Objects[support].Body.Gravity = f.Objects[actor].Body.Gravity
	for _, object := range []uint8{support, actor} {
		binding, e := originalWorldMotionCourt(result, c, object)
		if e != nil {
			return e
		}
		moved, e := binding.originalMotion.run(f)
		if e != nil {
			return e
		}
		f = moved.State
		storeWorldFloorState(s, f, 13)
	}
	f.Objects[actor].Body.Velocity = f.Objects[support].Body.Velocity
	f.Objects[actor].Body.Gravity = f.Objects[support].Body.Gravity
	f.Objects[support].Body.Velocity = Velocity{}
	f.Objects[support].Body.Gravity = 0
	storeWorldFloorState(s, f, 13)
	if f.Objects[support].Flags&128 == 0 {
		s.Actors[actor].Player.Action, s.Actors[actor].Player.Entered = 0x40, false
		s.Auxiliary[0].Action, s.Auxiliary[0].Entered = 4, false
	}
	return nil
}
