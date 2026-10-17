package engine

import "fmt"

// A binding exists only inside one transactional action/opening call. It gives
// the existing semantic action handlers the complete original motion operation
// without a second integration pass or a persistent duplicate world.
type courtMotionBinding struct {
	object, court  uint8
	resetJumpPhase bool
	attachment     *uint8
	load           func() CourtFloorState
	store          func(CourtFloorState, uint8)
	sounds         *[]uint8
	writes         *[]MotionSurfaceWrite
}

func floorObjectFromPlayer(p Player) CourtFloorObject {
	o := CourtFloorObject{Body: p.Body, Action: uint8(p.Action), Entered: p.Entered, Request: p.Request, Pose: p.Pose, FacingLeft: p.FacingLeft}
	if p.Airborne {
		o.Flags = 128
	}
	return o
}
func applyFloorPlayer(p *Player, o CourtFloorObject, flags uint8) {
	p.Body, p.Action, p.Entered, p.Request, p.Pose, p.FacingLeft = o.Body, Action(o.Action), o.Entered, o.Request, o.Pose, o.FacingLeft
	p.Airborne = o.Flags&128 != 0
	p.NewYorkPassage = flags
}
func (b *courtMotionBinding) run(s CourtFloorState) (CourtMotionStep, error) {
	if b.attachment != nil {
		s.Objects[b.object].Attachment = *b.attachment
	}
	r, e := StepOriginalCourtMotion(s, b.object, b.court)
	if e != nil {
		return CourtMotionStep{}, e
	}
	if b.object < 4 && r.ResetJumpPhase[b.object] {
		b.resetJumpPhase = true
	}
	if b.attachment != nil {
		*b.attachment = r.State.Objects[b.object].Attachment
	}
	b.store(r.State, b.object)
	if b.sounds != nil {
		*b.sounds = append(*b.sounds, r.SoundIDs...)
	}
	if b.writes != nil && r.HasSurface {
		*b.writes = append(*b.writes, MotionSurfaceWrite{Object: b.object, Origin: r.SurfaceOrigin})
	}
	return r, nil
}
func (b *courtMotionBinding) player(p *Player, ctx *playerTerrainContext) (BallCommand, error) {
	s := b.load()
	if ctx != nil {
		if ctx.HasBall {
			s.Objects[6].Owner = int8(b.object)
		} else if s.Objects[6].Owner == int8(b.object) {
			s.Objects[6].Owner = 6
		}
	}
	o := floorObjectFromPlayer(*p)
	old := s.Objects[b.object]
	o.Controller, o.Owner, o.Kind, o.Attachment = old.Controller, old.Owner, old.Kind, old.Attachment
	s.Objects[b.object] = o
	r, e := b.run(s)
	if e != nil {
		return BallCommand{}, e
	}
	applyFloorPlayer(p, r.State.Objects[b.object], r.State.CourtFlags[b.object])
	return r.Ball, nil
}
func (b *courtMotionBinding) ball(ball *Ball) error {
	s := b.load()
	o := &s.Objects[b.object]
	o.Body, o.Action, o.Entered, o.Request, o.Pose, o.FacingLeft = ball.Body, uint8(ball.Action), ball.Entered, ActionRequest{Action: Action(ball.Request.Action), Pending: ball.Request.Pending}, ball.Pose, ball.FacingLeft
	o.Flags = 0
	if ball.Airborne {
		o.Flags = 128
	}
	s.BallDeflection = ball.Deflection
	r, e := b.run(s)
	if e != nil {
		return e
	}
	o = &r.State.Objects[b.object]
	ball.Body, ball.Action, ball.Entered, ball.Request, ball.Pose, ball.FacingLeft = o.Body, BallAction(o.Action), o.Entered, BallRequest{Action: BallAction(o.Request.Action), Pending: o.Request.Pending}, o.Pose, o.FacingLeft
	ball.Airborne = o.Flags&128 != 0
	ball.Deflection = r.State.BallDeflection
	return nil
}
func (b *courtMotionBinding) auxiliary(a *AuxiliaryState) error {
	s := b.load()
	o := &s.Objects[b.object]
	o.Body, o.Action, o.Entered, o.Request, o.Pose, o.FacingLeft, o.Owner, o.Attachment = a.Body, a.Action, a.Entered, a.Request, a.Pose, a.FacingLeft, a.Owner, a.Attachment
	o.Flags = 0
	if a.Airborne {
		o.Flags = 128
	}
	r, e := b.run(s)
	if e != nil {
		return e
	}
	o = &r.State.Objects[b.object]
	a.Body, a.Action, a.Entered, a.Request, a.Pose, a.FacingLeft, a.Owner, a.Attachment = o.Body, o.Action, o.Entered, o.Request, o.Pose, o.FacingLeft, o.Owner, o.Attachment
	a.Airborne = o.Flags&128 != 0
	a.NewYorkPassage = r.State.CourtFlags[b.object]
	if b.court == 4 || a.Kind != 0 {
		a.Kind = o.Kind
	}
	return nil
}
func (b *courtMotionBinding) hoop(h *Hoop) error {
	s := b.load()
	o := &s.Objects[b.object]
	o.Body = Body{Position: h.Position, Velocity: h.Velocity, Gravity: h.Gravity}
	o.Action, o.Entered, o.Request, o.Pose, o.FacingLeft = uint8(h.Action), h.Entered, ActionRequest{Action: Action(h.Request.Action), Pending: h.Request.Pending}, h.Pose, h.FacingLeft
	o.Flags = 0
	if h.Airborne {
		o.Flags = 128
	}
	if h.Carried {
		o.Owner = int8(h.CarrierSeat)
	}
	r, e := b.run(s)
	if e != nil {
		return e
	}
	o = &r.State.Objects[b.object]
	h.Position, h.Velocity, h.Gravity = o.Body.Position, o.Body.Velocity, o.Body.Gravity
	h.Action, h.Entered, h.Request, h.Pose, h.FacingLeft = HoopAction(o.Action), o.Entered, HoopRequest{Action: HoopAction(o.Request.Action), Pending: o.Request.Pending}, o.Pose, o.FacingLeft
	h.Airborne = o.Flags&128 != 0
	return nil
}
func worldFloorState(s WorldActionState, c WorldActionContext) CourtFloorState {
	var f CourtFloorState
	for i, a := range s.Actors {
		o := floorObjectFromPlayer(a.Player)
		o.Controller, o.Kind, o.Attachment = c.Controls[i], c.Teams[i/2], s.PlayerLinks[i]
		f.Objects[i] = o
		f.CourtFlags[i] = a.Player.NewYorkPassage
		f.LinkedActors[i], f.JumpPhases[i] = a.LinkedActor, a.JumpPhase
	}
	for i, a := range s.Auxiliary {
		o := floorObjectFromPlayer(Player{Body: a.Body, Action: Action(a.Action), Entered: a.Entered, Request: a.Request, Pose: a.Pose, FacingLeft: a.FacingLeft, Airborne: a.Airborne})
		o.Owner, o.Attachment, o.Kind = a.Owner, a.Attachment, worldAuxiliaryKind(s, c.CourtID, i)
		f.Objects[i+4] = o
		f.CourtFlags[i+4] = a.NewYorkPassage
	}
	ball := s.Ball
	f.Objects[6] = floorObjectFromPlayer(Player{Body: ball.Body, Action: Action(ball.Action), Entered: ball.Entered, Request: ActionRequest{Action: Action(ball.Request.Action), Pending: ball.Request.Pending}, Pose: ball.Pose, FacingLeft: ball.FacingLeft, Airborne: ball.Airborne})
	f.Objects[6].Owner, f.Objects[6].Kind, f.Objects[6].Attachment = s.Holder, 8, s.BallLink
	for i, h := range s.Hoops {
		o := floorObjectFromPlayer(Player{Body: Body{Position: h.Position, Velocity: h.Velocity, Gravity: h.Gravity}, Action: Action(h.Action), Entered: h.Entered, Request: ActionRequest{Action: Action(h.Request.Action), Pending: h.Request.Pending}, Pose: h.Pose, FacingLeft: h.FacingLeft, Airborne: h.Airborne})
		o.Owner, o.Kind = int8(h.Owner), 9
		f.Objects[i+7] = o
	}
	f.Random, f.LastShooter, f.BallDeflection = s.Random, uint8(s.LastShooter), s.Ball.Deflection
	return f
}
func storeWorldFloorState(s *WorldActionState, f CourtFloorState, selected uint8) {
	for i := uint8(0); i < 13; i++ {
		if i == selected {
			continue
		}
		o := f.Objects[i]
		switch {
		case i < 4:
			applyFloorPlayer(&s.Actors[i].Player, o, f.CourtFlags[i])
			s.Actors[i].LinkedActor, s.Actors[i].JumpPhase = f.LinkedActors[i], f.JumpPhases[i]
			s.PlayerLinks[i] = o.Attachment
		case i < 6:
			a := &s.Auxiliary[i-4]
			a.Body, a.Action, a.Entered, a.Request, a.Pose, a.FacingLeft, a.Owner, a.Attachment = o.Body, o.Action, o.Entered, o.Request, o.Pose, o.FacingLeft, o.Owner, o.Attachment
			a.Airborne = o.Flags&128 != 0
			a.NewYorkPassage = f.CourtFlags[i]
			if a.Kind != 0 {
				a.Kind = o.Kind
			}
		case i == 6:
			a := &s.Ball
			a.Body, a.Action, a.Entered, a.Request, a.Pose, a.FacingLeft = o.Body, BallAction(o.Action), o.Entered, BallRequest{Action: BallAction(o.Request.Action), Pending: o.Request.Pending}, o.Pose, o.FacingLeft
			a.Airborne = o.Flags&128 != 0
			s.Holder, s.BallLink = o.Owner, o.Attachment
		default:
			a := &s.Hoops[i-7]
			a.Position, a.Velocity, a.Gravity = o.Body.Position, o.Body.Velocity, o.Body.Gravity
			a.Action, a.Entered, a.Request, a.Pose, a.FacingLeft = HoopAction(o.Action), o.Entered, HoopRequest{Action: HoopAction(o.Request.Action), Pending: o.Request.Pending}, o.Pose, o.FacingLeft
			a.Airborne = o.Flags&128 != 0
			a.Owner = uint8(o.Owner)
		}
	}
	s.Random, s.LastShooter, s.Ball.Deflection = f.Random, int8(f.LastShooter), f.BallDeflection
}
func originalWorldMotionCourt(result *WorldActionStep, c WorldActionContext, object uint8) (FlatCourt, error) {
	if c.CourtID > 5 {
		return FlatCourt{}, fmt.Errorf("world original motion binding court %d not enabled", c.CourtID)
	}
	b := &courtMotionBinding{object: object, court: c.CourtID, sounds: &result.SoundIDs, writes: &result.MotionSurfaceWrites}
	b.load = func() CourtFloorState { return worldFloorState(result.State, c) }
	b.store = func(s CourtFloorState, selected uint8) { storeWorldFloorState(&result.State, s, selected) }
	return FlatCourt{originalMotion: b}, nil
}
func openingFloorState(s OpeningState) CourtFloorState {
	var f CourtFloorState
	t := s.Transition
	for i, o := range t.Objects {
		p := floorObjectFromPlayer(Player{Body: o.Body, Action: Action(o.Action), Entered: o.Entered, Request: o.Request, Pose: o.Pose, FacingLeft: o.FacingLeft, Airborne: o.Airborne})
		p.Kind, p.Attachment = t.Kinds[i], s.Links[i]
		if i < 4 {
			p.Controller = t.PlayerControls[i]
		} else {
			p.Owner = int8(t.Owners[i-4])
		}
		f.Objects[i] = p
	}
	for i, v := range t.Passages {
		f.CourtFlags[i] = v
	}
	for i := range f.LinkedActors {
		f.LinkedActors[i] = -1
	}
	f.Random, f.LastShooter, f.BallDeflection = t.Random, t.LastShooter, s.BallDeflection
	return f
}
func originalOpeningMotionCourt(s *OpeningState, sounds *[]uint8, writes *[]MotionSurfaceWrite) FlatCourt {
	b := &courtMotionBinding{object: 6, court: s.Transition.Court, sounds: sounds, writes: writes}
	b.load = func() CourtFloorState { return openingFloorState(*s) }
	b.store = func(f CourtFloorState, selected uint8) {
		t := &s.Transition
		for i, o := range f.Objects {
			if uint8(i) == selected {
				continue
			}
			p := &t.Objects[i]
			p.Body, p.Action, p.Entered, p.Request, p.Pose, p.FacingLeft, p.Airborne = o.Body, o.Action, o.Entered, o.Request, o.Pose, o.FacingLeft, o.Flags&128 != 0
			s.Links[i] = o.Attachment
			if i >= 4 {
				t.Owners[i-4] = uint8(o.Owner)
			}
		}
		copy(t.Passages[:], f.CourtFlags[:])
		t.Random, t.LastShooter = f.Random, f.LastShooter
		s.BallDeflection = f.BallDeflection
	}
	return FlatCourt{originalMotion: b}
}
