package racing

import (
	"encoding/json"
	"math"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func classicAIAcceleration(p ClassicVehicleParams, speed float64) float64 {
	gear := 0
	for gear+1 < p.GearCount && speed*p.VelocityToRPM[gear] > p.RedlineRPM*.9 {
		gear++
	}
	rpm := math.Max(p.IdleRPM, speed*p.VelocityToRPM[gear])
	if speed < 5 {
		rpm = math.Max(rpm, p.IdleRPM+1200)
	}
	return math.Max(.2, classicTorque(p, math.Min(rpm, p.RedlineRPM))*p.VelocityToRPM[gear]*(2*math.Pi/60)*p.GearEfficiency[gear]/p.Mass)
}

// Basic computers use one public conservative pace envelope. Stronger cars
// use less pedal, while their underlying human-controlled performance remains
// unchanged. This also avoids unsafe closing speeds in mixed vehicle traffic.
func classicFleetPace(_ State, speed float64) (float64, float64, float64, float64) {
	cruise, lateral, braking, acceleration := 65., math.Inf(1), math.Inf(1), math.Inf(1)
	// Use the public base catalogue as a stable basic-driver safety reference,
	// rather than changing skill when a different opponent joins the room.
	for _, id := range []string{"p911", "fm50", "ldia", "mcf1", "ccor"} {
		p := ClassicVehicle(id)
		cruise = math.Min(cruise, p.MaxSpeed*.82)
		lateral = math.Min(lateral, p.LateralGrip*.5*9.81*.62)
		braking = math.Min(braking, p.MaxBraking)
		acceleration = math.Min(acceleration, classicAIAcceleration(p, speed))
	}
	return cruise, lateral, braking, acceleration
}

func classicAhead(t Track, index int, fraction, distance float64) (Point, int) {
	info := classicTrackInfoFor(t)
	n := len(t.Points)
	for k := 0; k < n; k++ {
		i := (index + k) % n
		j := (i + 1) % n
		length := info.length - info.arcs[i]
		if j > i {
			length = info.arcs[j] - info.arcs[i]
		}
		start := 0.
		if k == 0 {
			start = fraction
		}
		available := length * (1 - start)
		if distance <= available {
			f := start + distance/math.Max(length, 1e-6)
			return Point{t.Points[i].X + (t.Points[j].X-t.Points[i].X)*f, t.Points[i].Y + (t.Points[j].Y-t.Points[i].Y)*f}, i
		}
		distance -= available
	}
	return t.Points[index], index
}

func classicLaneCost(t Track, start int, offset float64, body ClassicBody, props *ClassicPropState) float64 {
	info := classicTrackInfoFor(t)
	n := len(t.Points)
	cost := 0.
	for k := 0; k < 15; k++ {
		i := (start + k) % n
		p, r := t.Points[i], t.Road[i]
		if offset < -r.Left+body.Width*5+3 || offset > r.Right-body.Width*5-3 {
			return 1e9
		}
		l := math.Hypot(r.RX, r.RZ)
		if l < 1e-7 {
			continue
		}
		x, y := p.X+r.RX/l*offset, p.Y+r.RZ/l*offset
		h := r.Height - (r.NX*(x-p.X)+r.NZ*(y-p.Y))/math.Max(.1, r.NY)
		tv := tangent(t, i)
		a, b, radius := classicCapsule(x, y, math.Atan2(tv.Y, tv.X), body)
		radius += 3
		for _, idx := range info.grid[[2]int{int(math.Floor(x / 100)), int(math.Floor(y / 100))}] {
			if classicBrokenBarrier(props, idx) {
				continue
			}
			wall := t.Barriers[idx]
			if h+body.Height*10 < wall.Bottom+.05 || h > wall.Top-.05 {
				continue
			}
			u, v := classicSegmentClosest(a, b, wall.A, wall.B)
			depth := radius - dist(u, v)
			if depth > 0 {
				cost += 1e5 + depth
			}
		}
	}
	return cost
}

func classicDrivingLane(t Track, road ClassicRoadContact, body ClassicBody, props *ClassicPropState) (float64, bool) {
	base := classicLaneCost(t, road.Index, 0, body, props)
	if base == 0 {
		return 0, false
	}
	best, bestCost := 0., base+math.Abs(road.Side)*.05
	for offset := -80.; offset <= 100; offset += 10 {
		if offset == 0 {
			continue
		}
		cost := classicLaneCost(t, road.Index, offset, body, props) + math.Abs(offset) + math.Abs(offset-road.Side)*.05
		if cost < bestCost {
			best, bestCost = offset, cost
		}
	}
	return best, bestCost >= 1e5
}

// Pure-pursuit steering, brake-distance preview and ordinary reversing use the
// same public route and drive action as humans. No position/progress is written.
func classicBotAction(ctx gameapi.Context, s State) (json.RawMessage, error) {
	c := s.Cars[ctx.Actor]
	t := s.Track
	n := len(t.Points)
	vehicle := ClassicVehicle(c.Vehicle)
	road := ClassicRoadAt(t, c.X, c.Y, c.Height, c.RouteIndex)
	speed := c.Speed / 10
	cruise, lateralBudget, brakingBudget, accelerationBudget := classicFleetPace(s, speed)
	tires := ClassicTerrainSurface(t, &c, ClassicSurfaceFor(road, c.Angle))
	lateralBudget *= math.Min(tires.FrontGrip, tires.RearGrip)
	look := clamp(4+speed*.35, 6, 24)
	target, targetIndex := classicAhead(t, road.Index, road.Fraction, look*10)
	// The native centreline is a conservative racing line. Computers merge by
	// yielding to the car ahead instead of carrying a fixed offset into walls.
	lane, blockedLane := classicDrivingLane(t, road, ClassicVehicleBody(c.Vehicle), s.Props)
	body := ClassicVehicleBody(c.Vehicle)
	lane = clamp(lane, -math.Max(0, t.Road[targetIndex].Left-body.Width*5-5), math.Max(0, t.Road[targetIndex].Right-body.Width*5-5))
	r := t.Road[targetIndex]
	rl := math.Hypot(r.RX, r.RZ)
	if rl > 1e-7 {
		target.X += r.RX / rl * lane
		target.Y += r.RZ / rl * lane
	}
	if s.ServerAt < s.StartAt+int64(ctx.Actor)*250 {
		pressure := 0.
		return json.Marshal(Action{Type: "drive", Epoch: s.Epoch, Seq: c.Seq + 1, Steer: 0, Throttle: 0, BrakeAmount: &pressure, Drift: true})
	}
	gate := t.Points[c.Checkpoint]
	gateTangent := tangent(t, c.Checkpoint)
	behindGate := (c.X-gate.X)*gateTangent.X + (c.Y-gate.Y)*gateTangent.Y
	gap := (c.Checkpoint - road.Index + n) % n
	missed := gap > n/2 && behindGate > 12
	recovering := c.Driver != nil && c.Driver.BlockedAt > 0 && s.ServerAt-c.Driver.BlockedAt > 1200
	reverse := missed
	if recovering && !missed {
		reverse = c.Driver.ReverseAt == 0 || s.ServerAt-c.Driver.ReverseAt < 1200
	}
	if c.Driver != nil && s.ServerAt < c.Driver.ForwardUntil {
		reverse = false
	}
	if reverse {
		forwardTarget := target
		if missed {
			target = Point{gate.X - gateTangent.X*55, gate.Y - gateTangent.Y*55}
		} else {
			target = Point{c.X - math.Cos(c.Angle)*60, c.Y - math.Sin(c.Angle)*60}
		}
		heading := c.Angle + math.Pi
		diff := angle(math.Atan2(target.Y-c.Y, target.X-c.X) - heading)
		delta := math.Atan(2 * vehicle.Wheelbase * math.Sin(diff) / math.Max(4, dist(Point{c.X, c.Y}, target)/10))
		maxSteer := math.Atan(vehicle.Wheelbase / vehicle.TurningRadius)
		steer := -clamp(delta/maxSteer, -1, 1)
		// Until reverse is actually engaged the steering still acts forwards.
		// Applying the reverse correction during braking steers away from road.
		if c.Gear >= 0 {
			fd := angle(math.Atan2(forwardTarget.Y-c.Y, forwardTarget.X-c.X) - c.Angle)
			distance := math.Max(3, dist(Point{c.X, c.Y}, forwardTarget)/10)
			steer = clamp(math.Atan(2*vehicle.Wheelbase*math.Sin(fd)/distance)/maxSteer, -1, 1)
		}
		pressure := 1.
		return json.Marshal(Action{Type: "drive", Epoch: s.Epoch, Seq: c.Seq + 1, Steer: steer, Throttle: 0, Brake: true, BrakeAmount: &pressure})
	}
	diff := angle(math.Atan2(target.Y-c.Y, target.X-c.X) - c.Angle)
	targetDistance := math.Max(2, dist(Point{c.X, c.Y}, target)/10)
	desiredDelta := math.Atan(2 * vehicle.Wheelbase * math.Sin(diff) / targetDistance)
	maxSteer := math.Atan(vehicle.Wheelbase / vehicle.TurningRadius)
	steer := clamp(desiredDelta/maxSteer*(1+.000055*speed*speed), -1, 1)
	if c.Motion != nil {
		desiredYaw := speed * 2 * math.Sin(diff) / targetDistance
		steer = clamp(steer+(desiredYaw-c.Motion.YawRate)*.13/maxSteer, -1, 1)
	}
	targetSpeed := cruise
	if blockedLane {
		targetSpeed = math.Min(targetSpeed, 6)
	}
	// Look far enough to establish the gradual brake before a sharp bend.
	preview := math.Max(160, speed*speed/(2*brakingBudget*.4)+speed*.95+50)
	travelled := 0.
	for k := 0; k < 160 && travelled < preview; k++ {
		i := (road.Index + k) % n
		j := (i + 1) % n
		dx, dy := t.Points[j].X-t.Points[i].X, t.Points[j].Y-t.Points[i].Y
		travelled += math.Hypot(dx, dy) / 10
		pa, pb := t.Points[(i-1+n)%n], t.Points[i]
		pc, pd := t.Points[i], t.Points[(i+1)%n]
		bend := math.Abs(angle(math.Atan2(pd.Y-pc.Y, pd.X-pc.X) - math.Atan2(pb.Y-pa.Y, pb.X-pa.X)))
		span := (dist(pa, pb) + dist(pc, pd)) / 20
		curvature := bend / math.Max(1, span)
		if curvature > 1e-5 {
			surface := ClassicSurfaceFor(ClassicRoadContact{NY: t.Road[i].NY, Left: t.Road[i].Left, Right: t.Road[i].Right, Surface: t.Road[i].Surface}, c.Angle)
			lateral := lateralBudget * surface.Grip
			curveSpeed := math.Sqrt(lateral / curvature)
			brakeDistance := math.Max(0, travelled-speed*.95-3)
			allowed := math.Sqrt(curveSpeed*curveSpeed + 2*brakingBudget*.4*brakeDistance)
			targetSpeed = math.Min(targetSpeed, allowed)
		}
	}
	if math.Abs(diff) > .7 {
		targetSpeed = math.Min(targetSpeed, 7)
	}
	if math.Abs(diff) > 1.2 {
		targetSpeed = math.Min(targetSpeed, 3.5)
	}
	for _, other := range s.Cars {
		if other.Seat == c.Seat || other.FinishAt > 0 || math.Abs(other.Height-c.Height) > 20 {
			continue
		}
		dx, dy := (other.X-c.X)/10, (other.Y-c.Y)/10
		along := dx*math.Cos(c.Angle) + dy*math.Sin(c.Angle)
		side := -dx*math.Sin(c.Angle) + dy*math.Cos(c.Angle)
		if along > 0 && along < math.Max(18, speed*1.5) && math.Abs(side) < 2.1 {
			gap := along - ClassicVehicleBody(c.Vehicle).Length
			safe := 1 + speed*.42
			if gap < safe {
				targetSpeed = math.Min(targetSpeed, math.Max(0, other.Speed/10+(gap-safe)*.65))
			}
		}
	}
	throttle := clamp((targetSpeed-speed)/4+.18, 0, 1)
	if c.Gear > 0 {
		own := classicAIAcceleration(vehicle, speed)
		if c.Motion != nil && c.Gear <= vehicle.GearCount {
			i := c.Gear - 1
			own = math.Max(.2, classicTorque(vehicle, c.Motion.RPM)*vehicle.VelocityToRPM[i]*(2*math.Pi/60)*vehicle.GearEfficiency[i]/vehicle.Mass)
		}
		throttle = math.Min(throttle, clamp(accelerationBudget/own, 0, 1))
	}
	pressure := clamp((speed-targetSpeed-.7)/4, 0, 1)
	if pressure > 0 {
		throttle = 0
	}
	if c.Gear < 0 {
		throttle = 1
		pressure = 0
		steer = 0
	}
	return json.Marshal(Action{Type: "drive", Epoch: s.Epoch, Seq: c.Seq + 1, Steer: steer, Throttle: throttle, Brake: pressure > 0, BrakeAmount: &pressure})
}
