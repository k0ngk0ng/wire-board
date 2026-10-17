package engine

import "fmt"

type CourtMotionStep struct {
	CourtFloorStep
	HasSurface    bool
	SurfaceOrigin Position
}

// StepOriginalCourtMotion restores EEFF through EFF1: attached follow, or
// integration -> travel bounds -> passage/floor resolution -> XY commit.
func StepOriginalCourtMotion(original CourtFloorState, object, court uint8) (CourtMotionStep, error) {
	initial := CourtMotionStep{CourtFloorStep: CourtFloorStep{State: original}}
	if object >= 13 {
		return initial, fmt.Errorf("invalid motion object %d", object)
	}
	if _, e := OriginalCourtProfile(court); e != nil {
		return initial, e
	}
	r := initial
	o := &r.State.Objects[object]
	if o.Attachment&128 != 0 {
		parent := o.Attachment & 15
		if parent >= 13 {
			return initial, fmt.Errorf("motion attachment %d outside original objects", parent)
		}
		src := r.State.Objects[parent]
		if src.Kind >= 16 {
			return initial, fmt.Errorf("unrestored attached motion kind %d", src.Kind)
		}
		heights := [16]uint8{32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 16, 40, 8, 8, 13, 16}
		height := heights[src.Kind]
		if src.Kind < 8 {
			pose := src.Pose.Frame
			if pose == 5 || (pose >= 71 && pose < 75) {
				height = 24
			} else if pose == 25 || pose >= 75 {
				height = 16
			}
		}
		o.Body.Position.X = (src.Body.Position.X &^ 255) + (o.Body.Position.X & 255)
		o.Body.Position.Depth = (src.Body.Position.Depth &^ 255) + (o.Body.Position.Depth & 255)
		o.Body.Position.Height = signed24((src.Body.Position.Height &^ 255) + Fixed(height)*Pixel + (o.Body.Position.Height & 255))
		return r, nil
	}
	previous := o.Body.Position
	integrated := IntegrateMotion(o.Body)
	o.PreviousHeight = previous.Height &^ 255
	o.Travel = FloorPoint{X: integrated.Position.X, Depth: integrated.Position.Depth}
	o.Body.Position.Height = integrated.Position.Height
	o.Body.Velocity = integrated.Velocity
	bounded, e := ClampOriginalCourtTravel(CourtBoundaryState{Body: integrated, Action: o.Action, Entered: o.Entered, Control: o.Controller}, court, object)
	if e != nil {
		return initial, e
	}
	o.Travel = FloorPoint{X: bounded.Body.Position.X, Depth: bounded.Body.Position.Depth}
	o.Body.Velocity = bounded.Body.Velocity
	o.Action, o.Entered = bounded.Action, bounded.Entered
	if court == 1 && r.State.CourtFlags[object] != 0 {
		if e = advanceOriginalCourtPassage(&r.State, object); e != nil {
			return initial, e
		}
	} else {
		resolved, e := ResolveOriginalCourtSurface(r.State, object, court)
		if e != nil {
			return initial, e
		}
		r.CourtFloorStep = resolved
		r.HasSurface = true
		r.SurfaceOrigin = previous
	}
	o = &r.State.Objects[object]
	o.Body.Position.X, o.Body.Position.Depth = o.Travel.X, o.Travel.Depth
	return r, nil
}

func advanceOriginalCourtPassage(s *CourtFloorState, object uint8) error {
	o := &s.Objects[object]
	action := uint8(0)
	if int8(object) == s.Objects[6].Owner {
		action = 16
	}
	floorSetAction(o, action)
	control := o.Controller
	if object >= 4 {
		control = uint8(o.Owner)
	}
	if control&128 == 0 {
		floorSetRequest(o, action)
	}
	for i := 0; i < 9; i++ {
		p := &s.Objects[i]
		if p.Attachment&128 != 0 && p.Attachment&15 == object {
			floorSetAction(p, 0)
			floorSetRequest(p, 0)
			break
		}
	}
	o.Body.Velocity = Velocity{}
	o.Body.Gravity = 0
	o.Travel.Depth = o.Body.Position.Depth
	delta := 10 * Pixel
	exitPage := uint8(1)
	exitBand := uint8(128)
	if object >= 7 {
		exitBand = 96
	}
	if s.CourtFlags[object] == 2 {
		delta = -delta
		exitPage = 0
		if object >= 7 {
			exitBand = 160
		}
	}
	o.Body.Position.X = signed24(o.Body.Position.X + delta)
	o.Travel.X = (o.Body.Position.X &^ 255) + (o.Travel.X & 255)
	page := uint8(o.Body.Position.X >> 16)
	if page != exitPage || floorLow(o.Body.Position.X)&240 != exitBand {
		return nil
	}
	s.CourtFlags[object] = 0
	o.Body.Position.Height = 0
	o.Body.Velocity.X = 0
	if object >= 6 {
		floorSetAction(o, 0)
		return nil
	}
	speed := 6 * Pixel
	if page != 0 {
		speed = -speed
	}
	if object >= 4 {
		if o.Kind != 12 {
			floorSetAction(o, 0)
			return nil
		}
		o.Body.Velocity.X = speed
		floorSetAction(o, 4)
		for i := 0; i < 6; i++ {
			p := &s.Objects[i]
			if p.Attachment&128 != 0 && p.Attachment&15 == object {
				floorSetAction(p, 61)
				break
			}
		}
		return nil
	}
	o.Body.Velocity.X = speed
	o.Pose = Pose{Frame: 4, Mirrored: page != 0}
	o.Body.Velocity.Height = 8 * Pixel
	o.Body.Gravity = Pixel
	action = 4
	if int8(object) == s.Objects[6].Owner {
		action = 20
	} else if int8(object) == s.Objects[4].Owner || int8(object) == s.Objects[5].Owner {
		action = 36
	}
	o.Action, o.Entered = action, true
	if int8(object) == s.Objects[6].Owner {
		floorSetAction(&s.Objects[6], 2)
		floorSetRequest(&s.Objects[6], 2)
	}
	return nil
}
