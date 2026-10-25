package tabletennis

import "math"

// Ballistic segments use exact time-of-impact plane intersections. At each
// contact a new segment starts; speed cannot tunnel through the table or net.
func acceleration(v Vec, spin float64) Vec {
	return Vec{-.12 * v.X, -9.81 - 1.6*spin*math.Min(1.5, math.Abs(v.Z)/4), -.12 * v.Z}
}
func position(p, v, a Vec, t float64) Vec {
	return Vec{p.X + v.X*t + .5*a.X*t*t, p.Y + v.Y*t + .5*a.Y*t*t, p.Z + v.Z*t + .5*a.Z*t*t}
}
func velocity(v, a Vec, t float64) Vec { return Vec{v.X + a.X*t, v.Y + a.Y*t, v.Z + a.Z*t} }
func roots(p, v, a, target float64) []float64 {
	if math.Abs(a) < 1e-9 {
		if math.Abs(v) < 1e-9 {
			return nil
		}
		t := (target - p) / v
		if t > 1e-6 {
			return []float64{t}
		}
		return nil
	}
	d := v*v - 2*a*(p-target)
	if d < 0 {
		return nil
	}
	a1, a2 := (-v-math.Sqrt(d))/a, (-v+math.Sqrt(d))/a
	if a1 > a2 {
		a1, a2 = a2, a1
	}
	out := []float64{}
	if a1 > 1e-6 {
		out = append(out, a1)
	}
	if a2 > 1e-6 {
		out = append(out, a2)
	}
	return out
}
func onTable(p Vec) bool {
	dx := math.Max(0, math.Abs(p.X)-HalfWidth)
	dz := math.Max(0, math.Abs(p.Z)-HalfLength)
	return dx*dx+dz*dz <= BallRadius*BallRadius
}
func traceFlight(p, v Vec, spin float64, at int64, from, to int, serve, doubles bool) Flight {
	f := Flight{Segments: []Segment{}, Events: []FlightEvent{}, Spin: spin, StartAt: at, WinnerSide: to, Serve: serve}
	bounces := 0
	crossed := false
	for iteration := 0; iteration < 10; iteration++ {
		a := acceleration(v, spin)
		dt := 4.
		kind := "out"
		for _, t := range roots(p.Y, v.Y, a.Y, BallRadius) {
			if v.Y+a.Y*t < 0 && t < dt {
				dt = t
				kind = "floor"
			}
		}
		for _, t := range roots(p.Y, v.Y, a.Y, TableHeight+BallRadius) {
			hit := position(p, v, a, t)
			if v.Y+a.Y*t < 0 && onTable(hit) && t < dt {
				dt = t
				kind = "table"
			}
		}
		if !crossed {
			for _, t := range roots(p.Z, v.Z, a.Z, 0) {
				q := position(p, v, a, t)
				if t < dt && math.Abs(q.X) <= HalfWidth+.05 {
					dt = t
					kind = "cross"
					break
				}
			}
		}
		end := at + int64(math.Round(dt*1000))
		if end <= at {
			end = at + 1
		}
		f.Segments = append(f.Segments, Segment{At: at, Until: end, P: p, V: v, A: a})
		p = position(p, v, a, dt)
		v = velocity(v, a, dt)
		at = end
		if kind == "cross" {
			crossed = true
			if p.Y < TableHeight+NetHeight-BallRadius {
				f.Reason = "net"
				f.EndAt = at
				f.Events = append(f.Events, FlightEvent{"net", at, p, -1})
				break
			}
			if p.Y <= TableHeight+NetHeight+BallRadius {
				f.TouchedNet = true
				f.Events = append(f.Events, FlightEvent{"net", at, p, -1})
				v.Z *= .55
				v.Y = math.Max(.35, v.Y*.2)
				p.Z = -sign(from) * .0001
			}
			continue
		}
		if kind == "table" {
			land := side(p.Z)
			f.Events = append(f.Events, FlightEvent{"bounce", at, p, land})
			expected := to
			if serve && bounces == 0 {
				expected = from
			}
			if land != expected {
				f.Reason = "wrong_side"
				f.EndAt = at
				break
			}
			if serve && doubles && ((bounces == 0 && p.X*sign(from) < -BallRadius) || (bounces == 1 && p.X*sign(to) < -BallRadius)) {
				f.Reason = "wrong_service_half"
				f.EndAt = at
				break
			}
			bounces++
			if land == to {
				if f.ReceiveAt != 0 {
					f.Reason = "double_bounce"
					f.EndAt = at
					f.WinnerSide = from
					break
				}
				if serve && f.TouchedNet {
					f.Reason = "let"
					f.EndAt = at
					f.WinnerSide = -1
					break
				}
				f.ReceiveAt = at
				f.ContactStart = at + 1
				f.WinnerSide = from
			}
			v.Y = math.Abs(v.Y) * .84
			v.X *= .92
			v.Z *= .86 + .12*spin
			p.Y = TableHeight + BallRadius + .00001
			continue
		}
		f.EndAt = at
		if f.ReceiveAt != 0 {
			f.Reason = "miss"
			f.WinnerSide = from
		} else {
			f.Reason = "out"
		}
		break
	}
	if f.EndAt == 0 {
		f.EndAt = at
		f.Reason = "out"
	}
	if f.ReceiveAt > 0 {
		f.ContactEnd = f.EndAt - 25
		// No body controls: the legal strike area stays in front of the fixed
		// first-person observer, including the visible near-court corridor.
		for _, segment := range f.Segments {
			if segment.At < f.ReceiveAt {
				continue
			}
			for _, t := range roots(segment.P.Z, segment.V.Z, segment.A.Z, sign(to)*ReachDepth) {
				crossing := segment.At + int64(math.Round(t*1000))
				if crossing <= segment.Until && crossing < f.ContactEnd {
					f.ContactEnd = crossing
				}
			}
		}
		if f.ContactEnd < f.ContactStart {
			f.ContactEnd = f.ContactStart
		}
	}
	return f
}
func (f *Flight) at(at int64) (Vec, Vec) {
	if f == nil || len(f.Segments) == 0 {
		return Vec{}, Vec{}
	}
	seg := f.Segments[0]
	for _, candidate := range f.Segments {
		if candidate.At <= at {
			seg = candidate
		} else {
			break
		}
	}
	t := clamp(float64(at-seg.At)/1000, 0, float64(seg.Until-seg.At)/1000)
	return position(seg.P, seg.V, seg.A, t), velocity(seg.V, seg.A, t)
}
func spinFor(kind string) float64 {
	switch kind {
	case "chop":
		return -.95
	case "block":
		return .05
	case "smash":
		return .4
	}
	return .9
}
func (s *State) launch(a Action, seat int, at int64, serve bool) {
	from := s.side(s.Options.Teams[seat])
	to := 1 - from
	dir := sign(from)
	spin := spinFor(a.Kind)
	kind := a.Kind
	var p, v Vec
	if serve {
		p = Vec{dir * .34, 1.1, dir * 1.28}
		t := .24
		vy := (TableHeight + BallRadius - p.Y + .5*9.81*t*t) / t
		v = Vec{-dir * (.6 + .18*a.Aim), vy, -dir * (2.0 + 2.1*a.Power)}
	} else {
		p, _ = s.Flight.at(at)
		if a.Kind == "drive" && p.Y > 1.18 && a.Power > .86 {
			kind = "smash"
			spin = spinFor(kind)
		}
		t := .98 - .71*a.Power
		if kind == "chop" {
			t += .13
		}
		if kind == "block" {
			t += .05
		}
		if kind == "smash" {
			t -= .08
		}
		ideal := s.Flight.ContactStart + 130
		timing := clamp(float64(at-ideal)/260, -1, 1)
		targetX := dir * (a.Aim*HalfWidth*(.92+.17*a.Power) + timing*a.Power*.25)
		targetZ := -dir * (.95 + math.Max(0, timing)*.2)
		if a.Lift > .25 && p.Y < 1.1 && kind == "drive" {
			fraction := clamp(math.Abs(p.Z/(targetZ-p.Z)), .04, .96)
			baseline := p.Y + (TableHeight+BallRadius-p.Y)*fraction
			clearance := TableHeight + NetHeight + BallRadius + .04
			t = math.Max(t, math.Sqrt(2*math.Max(0, clearance-baseline)/(9.81*fraction*(1-fraction))))
		}
		// Solve the intended arc, then let incoming spin and mistiming alter it.
		vz := (targetZ - p.Z) / (t - .06*t*t)
		vx := (targetX - p.X) / (t - .06*t*t)
		acc := acceleration(Vec{vx, 0, vz}, spin)
		vy := (TableHeight + BallRadius - p.Y - .5*acc.Y*t*t) / t
		incoming := s.Flight.Spin
		if incoming < -.3 && kind == "drive" {
			vy -= (-incoming) * math.Abs(timing) * a.Power * .9 * (1 - a.Lift)
		}
		if kind == "block" {
			vy -= incoming * .18
		}
		v = Vec{vx, vy, vz}
	}
	if serve && a.RacketY == nil {
		a.RacketX = .34 / .9
	}
	if a.RacketY == nil {
		a.RacketY = &p.Y
	}
	a.RacketZ = p.Z * dir
	s.recordPaddle(a, seat, at, "hit")
	flight := traceFlight(p, v, spin, at, from, to, serve, len(s.Seq) == 4)
	s.Flight = &flight
	s.FlightCursor = 0
	s.Pending = nil
	s.Shot++
	s.LastHitter = seat
	s.Expected = s.nextSeat(seat)
	s.Receiver = s.Expected
	s.Phase = "rally"
	s.ServeDeadline = 0
	s.emit("hit", at, seat, p, kind)
}
