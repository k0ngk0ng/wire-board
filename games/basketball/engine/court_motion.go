package engine

import "fmt"

// FloorPoint is pending horizontal/depth travel, before the original floor
// resolver commits it. Height has already been integrated in Body.Position.
type FloorPoint struct{ X, Depth Fixed }
type CourtFloorObject struct {
	Body                                Body
	Travel                              FloorPoint
	PreviousHeight                      Fixed
	Action                              uint8
	Entered                             bool
	Request                             ActionRequest
	Pose                                Pose
	FacingLeft                          bool
	Flags, Kind, Attachment, Controller uint8
	Owner                               int8
}
type CourtFloorState struct {
	Objects        [13]CourtFloorObject
	CourtFlags     [13]uint8 // 06D7..06E3: court devices and New York passages share these slots.
	LinkedActors   [4]int8
	JumpPhases     [4]int8
	BallDeflection DeflectionAxes
	LastShooter    uint8
	Random         RandomState
}
type CourtFloorStep struct {
	// Explicit writes survive action-local return values even when the old value was already zero.
	ResetJumpPhase [4]bool
	Ball           BallCommand
	State          CourtFloorState
	SoundIDs       []uint8
	FloorCode      CourtSurface
	FloorHeight    uint8
}
type courtFloorResolver struct {
	step          CourtFloorStep
	object, court uint8
	samples       [4]CourtSurface
}

func (r *courtFloorResolver) ballDeflectX()          { r.step.State.BallDeflection = DeflectX }
func (r *courtFloorResolver) obj() *CourtFloorObject { return &r.step.State.Objects[r.object] }
func floorSetAction(o *CourtFloorObject, a uint8)    { o.Action = a; o.Entered = false }
func floorSetRequest(o *CourtFloorObject, a uint8) {
	o.Request = ActionRequest{Action: Action(a), Pending: true}
}
func floorLow(v Fixed) uint8                { return uint8(v >> 8) }
func floorHigh(v Fixed) int8                { return int8(v >> 16) }
func floorSetDepth(v Fixed, hi uint8) Fixed { return Fixed(hi)*Pixel + (v & 255) }

// ResolveOriginalCourtSurface restores bank-4 8249 and its floor/side/device
// calls. It neither integrates motion nor commits pending XY. This explicit
// boundary preserves original old-point sampling and candidate travel order.
func ResolveOriginalCourtSurface(original CourtFloorState, object, court uint8) (CourtFloorStep, error) {
	initial := CourtFloorStep{State: original}
	if object >= 13 {
		return initial, fmt.Errorf("invalid floor object %d", object)
	}
	r := courtFloorResolver{step: initial, object: object, court: court}
	o := r.obj()
	p := CourtTerrainProbe{Position: o.Body.Position, Candidate: Position{X: o.Travel.X, Depth: o.Travel.Depth, Height: o.Body.Position.Height}, Flags: o.Flags, Action: o.Action, VelocityDepth: o.Body.Velocity.Depth}
	probe, err := ProbeOriginalCourtTerrain(p, court)
	if err != nil {
		return initial, err
	}
	r.samples = probe.Samples
	r.step.FloorCode = probe.Samples[0]
	r.step.FloorHeight = uint8(probe.Samples[0]) & 248
	o.Body.Position.Depth = probe.State.Position.Depth
	o.Travel.Depth = probe.State.Candidate.Depth
	o.Flags = probe.State.Flags
	o.Body.Velocity.Depth = probe.State.VelocityDepth
	if err = r.resolve(); err != nil {
		return initial, err
	}
	return r.step, nil
}
func (r *courtFloorResolver) resolve() error {
	o := r.obj()
	floor := uint8(0)
	if floorHigh(o.Body.Position.Height) == 0 {
		floor = uint8(r.samples[1]) & 248
		if floorLow(o.Body.Position.Height) == floor {
			if int8(o.Body.Velocity.Height>>8) <= 0 {
				o.Body.Position.Height = Fixed(floor) * Pixel
				if e := r.device(); e != nil {
					return e
				}
				if o.Body.Velocity.Height == 0 && o.Body.Gravity == 0 {
					o.Flags &^= 128
				}
			}
			if o.Flags&128 == 0 {
				if r.object == 6 {
					if o.Action == 1 || o.Action == 2 {
						return nil
					}
					if o.Action == 0 && floorLow(o.Body.Position.Height) != 0 {
						floorSetAction(o, 6)
						o.Body.Velocity.Depth = -128
					}
				}
				r.travelDepth(r.samples[1])
			}
			return nil
		}
	}
	if floorHigh(o.Body.Position.Height) > 0 || (floorHigh(o.Body.Position.Height) == 0 && floorLow(o.Body.Position.Height) > floor) {
		o.Flags |= 128
		if o.Flags&64 == 0 && r.object < 4 && o.Action == 14 {
			floorSetAction(o, 15)
		}
		if o.Attachment&128 != 0 {
			o.Flags &^= 128
		} else {
			r.surfaceAhead()
		}
		return nil
	}
	edge := o.Flags&64 != 0 && floorLow(o.PreviousHeight) != 0 && o.Action == 14
	if !edge && o.Body.Velocity.Height < 0 && floorHigh(o.PreviousHeight) >= 0 {
		if floorHigh(o.PreviousHeight) != 0 || floorLow(o.PreviousHeight) >= floor {
			o.Body.Position.Height = Fixed(floor) * Pixel
			o.Body.Velocity.Height = 0
			o.Flags &^= 128
			if r.object != 6 {
				if e := r.side(); e != nil {
					return e
				}
			}
			o.Body.Gravity = 0
			r.surfaceAhead()
			return r.device()
		}
		if r.object < 6 && (floorHigh(o.Body.Position.Height) < 0 || floorLow(o.Body.Position.Height) < r.step.FloorHeight) {
			r.restoreTravel()
			o.Body.Position.Height = Fixed(r.step.FloorHeight) * Pixel
			o.Body.Velocity.Height = 0
			o.Body.Gravity = 0
			o.Flags &^= 128
			r.surfaceAhead()
			return r.device()
		}
	}
	o.Flags &^= 128
	if edge || o.Body.Velocity.Height != 0 || o.Body.Gravity != 0 {
		o.Flags |= 128
	}
	if e := r.side(); e != nil {
		return e
	}
	return r.device()
}
func (r *courtFloorResolver) travelDepth(cell CourtSurface) {
	k := cell.Travel()
	if k == 0 {
		return
	}
	o := r.obj()
	masks := [5]uint8{255, 240, 240, 248, 248}
	subtract := [5]uint8{0, 0, 8, 0, 1}
	// Original restored tables contain only attributes 0..4.
	if k >= 5 {
		return
	}
	o.Travel.Depth = floorSetDepth(o.Travel.Depth, (floorLow(o.Travel.Depth)&masks[k])-subtract[k])
}
func (r *courtFloorResolver) surfaceAhead() {
	o := r.obj()
	if o.Flags&128 == 0 {
		if r.object == 6 && (o.Action == 1 || o.Action == 2) {
			return
		}
		if r.samples[0].Travel() == 0 {
			return
		}
		r.travelDepth(r.samples[0])
	}
	if r.object == 6 && o.Action == 6 && r.samples[0].Travel() != 0 && o.Body.Velocity.Depth >= 0 {
		o.Body.Velocity.Depth = 0
	}
}
func (r *courtFloorResolver) restoreTravel() {
	o := r.obj()
	o.Travel = FloorPoint{X: o.Body.Position.X, Depth: o.Body.Position.Depth}
}
func (r *courtFloorResolver) side() error {
	o := r.obj()
	blocked := false
	if uint8(r.samples[2])&248 > floorLow(o.Body.Position.Height) {
		if r.object == 6 || o.Body.Velocity.Height < 0 {
			o.Body.Velocity.X = 0
		}
		if uint16(o.Body.Position.X>>8) > uint16(o.Travel.X>>8) {
			o.Travel.X = (o.Body.Position.X &^ 2047) + (o.Body.Position.X & 255)
		} else if uint16(o.Body.Position.X>>8) < uint16(o.Travel.X>>8) {
			o.Travel.X = (o.Body.Position.X &^ 2047) + 7*Pixel + (o.Body.Position.X & 255)
		}
		r.surfaceAhead()
		if r.object == 6 {
			if int8(o.Body.Velocity.X>>8) == 0 && int8(o.Body.Velocity.Depth>>8) == 0 {
				o.Body.Position.Depth = floorSetDepth(o.Body.Position.Depth, floorLow(o.Body.Position.Depth)-2)
			}
			floorSetRequest(o, 8)
			r.ballDeflectX()
		} else if r.court == 1 {
			if e := r.newYorkSide(); e != nil {
				return e
			}
		} else {
			if e := r.bounceSide(); e != nil {
				return e
			}
		}
		blocked = true
	} else if o.Body.Velocity.Height == 0 && o.Body.Gravity == 0 && !(o.Flags&64 != 0 && o.Action == 14) {
		o.Body.Position.Height = boardReplaceLow(o.Body.Position.Height, uint8(r.samples[2])&248)
	}
	cell := r.samples[3]
	if cell == 4 || uint8(cell)&248 > floorLow(o.Body.Position.Height) {
		if r.object == 6 || o.Body.Velocity.Height < 0 {
			o.Body.Velocity.Depth = 0
		}
		o.Travel.Depth = floorSetDepth(o.Body.Position.Depth, floorLow(o.Body.Position.Depth)|7)
		r.surfaceAhead()
		blocked = true
	} else if !(r.object == 6 && o.Flags&128 != 0) && o.Body.Velocity.Height == 0 && o.Body.Gravity == 0 {
		o.Body.Position.Height = boardReplaceLow(o.Body.Position.Height, uint8(cell)&248)
	}
	if !blocked {
		r.restoreTravel()
	}
	return nil
}
func (r *courtFloorResolver) kick(target uint8) error {
	if target >= 13 {
		return fmt.Errorf("terrain linked object %d is outside restored objects", target)
	}
	src := r.obj()
	dst := &r.step.State.Objects[target]
	floorSetAction(dst, 73)
	r.step.SoundIDs = append(r.step.SoundIDs, 19)
	dst.Body.Gravity = Pixel
	dst.Body.Velocity.X = signed16(-src.Body.Velocity.X)
	dst.FacingLeft = uint8(src.Body.Position.X>>16) != 0
	return nil
}
func (r *courtFloorResolver) bounceSide() error {
	o := r.obj()
	if r.object < 4 {
		switch o.Action {
		case 2, 18, 82, 34, 50:
			o.Body.Velocity.Height = 8*Pixel + Fixed(uint8(o.Body.Velocity.Height))
			if e := r.kick(r.object); e != nil {
				return e
			}
			o.Body.Velocity.X = signed16(o.Body.Velocity.X) >> 1
			if r.court >= 7 {
				o.Body.Velocity.Depth = signed16(o.Body.Velocity.Depth - Pixel)
			}
		}
	} else if r.object != 6 && o.Action == 5 {
		for i := 0; i < 6; i++ {
			dst := &r.step.State.Objects[i]
			if dst.Attachment&128 != 0 && dst.Attachment&15 == r.object && dst.Action == 66 {
				floorSetAction(dst, 73)
			}
		}
	}
	return nil
}
func (r *courtFloorResolver) newYorkSide() error {
	o := r.obj()
	page, low := uint8(o.Travel.X>>16), floorLow(o.Travel.X)
	if floorLow(o.Body.Position.Depth) < 73 || (page == 0 && low >= 137) || (page != 0 && low < 135) {
		return r.bounceSide()
	}
	if page == 0 {
		r.step.State.CourtFlags[r.object] = 1
	} else {
		r.step.State.CourtFlags[r.object] = 2
	}
	if r.object >= 7 && o.Action == 5 {
		if o.Owner < 0 || o.Owner >= 13 {
			return fmt.Errorf("carried hoop has invalid terrain owner")
		}
		dst := &r.step.State.Objects[o.Owner]
		floorSetAction(dst, 1)
		floorSetRequest(dst, 1)
	}
	if r.object == 4 || r.object == 5 {
		reverse := o.Action == 3 || o.Action == 13 || o.Action == 7
		if o.Action == 10 || o.Action == 1 || reverse {
			if reverse {
				o.FacingLeft = !o.FacingLeft
			}
			floorSetAction(o, 7)
			r.step.State.CourtFlags[r.object] = 0
			return r.bounceSide()
		}
		if o.Kind == 12 {
			for i := uint8(0); i < 4; i++ {
				p := &r.step.State.Objects[i]
				if p.Attachment&128 == 0 || p.Attachment&15 != r.object {
					continue
				}
				if linked := r.step.State.LinkedActors[i]; linked >= 0 {
					if e := r.kick(uint8(linked)); e != nil {
						return e
					}
				} else if q := &r.step.State.Objects[5]; q.Attachment&128 != 0 && q.Attachment&15 == i {
					if e := r.kick(5); e != nil {
						return e
					}
					floorSetAction(q, 7)
					q.Attachment = 0
				}
				break
			}
		}
	}
	if r.object < 4 && r.step.State.LinkedActors[r.object] >= 0 {
		if e := r.kick(uint8(r.step.State.LinkedActors[r.object])); e != nil {
			return e
		}
	} else {
		for a := uint8(4); a < 6; a++ {
			aux := &r.step.State.Objects[a]
			if aux.Attachment&128 == 0 || aux.Attachment&15 != r.object {
				continue
			}
			for i := uint8(0); i < 4; i++ {
				p := &r.step.State.Objects[i]
				if p.Attachment&128 != 0 && p.Attachment&15 == a {
					if e := r.kick(i); e != nil {
						return e
					}
					break
				}
			}
			break
		}
	}
	parent := o.Attachment & 15
	if parent >= 13 {
		return fmt.Errorf("terrain attachment target %d outside restored objects", parent)
	}
	if p := &r.step.State.Objects[parent]; p.Action == 66 {
		floorSetAction(p, 0)
		floorSetRequest(p, 0)
	}
	if int8(r.object) == r.step.State.Objects[6].Owner {
		floorSetAction(o, 29)
		floorSetRequest(o, 29)
		ball := &r.step.State.Objects[6]
		floorSetAction(ball, 2)
		floorSetRequest(ball, 2)
	} else {
		floorSetAction(o, 0)
		floorSetRequest(o, 0)
	}
	o.Body.Gravity = 0
	o.Body.Velocity.Height = 0
	o.Flags &^= 128
	o.Body.Position.Height = -Pixel + (o.Body.Position.Height & 255)
	return nil
}
func (r *courtFloorResolver) releaseForDevice() {
	s := &r.step.State
	if int8(r.object) == s.Objects[6].Owner {
		if r.object < 4 || r.court == 5 {
			floorSetAction(&s.Objects[6], 2)
			r.step.Ball = BallCommand{Change: true, Action: BallHeld}
		}
		return
	}
	for i := 4; i < 6; i++ {
		if int8(r.object) == s.Objects[i].Owner {
			if r.object < 4 {
				floorSetAction(&s.Objects[i], 1)
			}
			return
		}
	}
}

var originalDeviceActions = [13]uint8{62, 62, 62, 62, 2, 2, 4, 6, 6, 6, 6, 6, 6}

func (r *courtFloorResolver) fireDevice() {
	floorSetAction(r.obj(), originalDeviceActions[r.object])
	r.step.SoundIDs = append(r.step.SoundIDs, 21)
	if r.object == 6 {
		r.step.State.LastShooter = 3
	}
}
func (r *courtFloorResolver) device() error {
	o := r.obj()
	x, depth := uint16(o.Body.Position.X>>8), floorLow(o.Body.Position.Depth)
	switch r.court {
	case 0, 1, 6, 7:
		return nil
	case 2:
		if r.object == 6 && r.step.State.Objects[6].Owner != 6 {
			return nil
		}
		if depth < 72 || x < 264 || x >= 408 || (x >= 304 && x < 376) {
			return nil
		}
		if x < 304 {
			r.step.State.CourtFlags[1] = 128
		} else {
			r.step.State.CourtFlags[2] = 128
		}
		r.releaseForDevice()
		if r.object < 4 {
			r.step.State.JumpPhases[r.object] = 0
			r.step.ResetJumpPhase[r.object] = true
		}
		r.fireDevice()
	case 3:
		if r.object == 6 && r.step.State.Objects[6].Owner != 6 {
			return nil
		}
		if depth < 80 || x < 144 || x >= 160 {
			return nil
		}
		o.FacingLeft = x < 152
		o.Pose.Mirrored = o.FacingLeft
		r.step.State.CourtFlags[0] |= 128
		r.releaseForDevice()
		if o.Action != originalDeviceActions[r.object] {
			if r.object < 4 {
				r.step.State.JumpPhases[r.object] = 0
				r.step.ResetJumpPhase[r.object] = true
			}
			r.fireDevice()
		}
	case 4:
		if (r.object != 4 && r.object != 5) || depth < 56 || depth >= 72 {
			return nil
		}
		wrap := false
		if r.step.State.CourtFlags[1] == 0 {
			o.Travel.X = signed24(o.Travel.X + 2*Pixel)
			if uint8(o.Travel.X>>16) != 0 && floorLow(o.Travel.X) >= 216 {
				o.Travel.X = 55*Pixel + (o.Travel.X & 255)
				wrap = true
			}
		} else {
			o.Travel.X = signed24(o.Travel.X - 2*Pixel)
			if uint8(o.Travel.X>>16) == 0 && floorLow(o.Travel.X) < 56 {
				o.Travel.X = 472*Pixel + (o.Travel.X & 255)
				wrap = true
			}
		}
		if wrap {
			var random uint8
			r.step.State.Random, random = NextRandom(r.step.State.Random)
			o.Kind = [8]uint8{10, 12, 13, 14, 11, 13, 14, 14}[(r.object&1)*4+(random&3)]
			floorSetAction(o, 0)
			o.Attachment = 0
			for i := 0; i < 4; i++ {
				p := &r.step.State.Objects[i]
				if p.Attachment&128 != 0 && p.Attachment&15 == r.object {
					p.Body.Gravity = Pixel
					floorSetAction(p, 73)
					p.Body.Velocity.X = Fixed(uint8(p.Body.Velocity.X))
					o.Body.Velocity.X = Fixed(uint8(o.Body.Velocity.X))
					p.Travel.X = boardReplaceLow(p.Travel.X, floorLow(p.Body.Position.X))
					p.Attachment = 0
				}
			}
		}
	case 5:
		if r.object >= 7 && o.Action == 5 {
			return nil
		}
		if r.object == 6 && r.step.State.Objects[6].Owner != 6 {
			return nil
		}
		if depth < 72 || !((x >= 120 && x < 184) || (x >= 272 && x < 336)) {
			return nil
		}
		if x < 256 {
			r.step.State.CourtFlags[1] = 128
		} else {
			r.step.State.CourtFlags[2] = 128
		}
		r.releaseForDevice()
		if r.object < 4 {
			r.step.State.JumpPhases[r.object] = 0
			r.step.ResetJumpPhase[r.object] = true
		}
		r.fireDevice()
	default:
		return fmt.Errorf("unrestored floor device court %d", r.court)
	}
	return nil
}
