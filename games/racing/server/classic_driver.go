package racing

import "math"

func classicTrafficLane(c Car, t Track, road ClassicRoadContact, base float64, props *ClassicPropState, cars []Car) float64 {
	body := ClassicVehicleBody(c.Vehicle)
	if road.Side < -road.Left+body.Width*5 || road.Side > road.Right-body.Width*5 {
		return base
	}
	tv := tangent(t, road.Index)
	for _, other := range cars {
		if other.Seat == c.Seat || other.FinishAt > 0 || math.Abs(other.Height-c.Height) > 20 {
			continue
		}
		dx, dy := other.X-c.X, other.Y-c.Y
		along := dx*tv.X + dy*tv.Y
		otherRoad := ClassicRoadAt(t, other.X, other.Y, other.Height, road.Index)
		beside := math.Abs(along) < 90 && math.Abs(road.Side-otherRoad.Side) < 60
		passing := along > 60 && along < math.Max(220, c.Speed*1.2) && (other.Speed < c.Speed-20 || math.Abs(road.Side-base) > 18) && math.Abs(road.Side-otherRoad.Side) < 50
		if !beside && !passing {
			continue
		}
		preferred := -24.
		if road.Side > otherRoad.Side || road.Side == otherRoad.Side && c.Seat%2 == 1 {
			preferred = 24
		}
		for _, lane := range []float64{preferred, -preferred} {
			if classicLaneCost(t, road.Index, lane, body, props) > 0 {
				continue
			}
			clear := true
			for _, neighbor := range cars {
				if neighbor.Seat == c.Seat || neighbor.FinishAt > 0 || math.Abs(neighbor.Height-c.Height) > 20 {
					continue
				}
				nx, ny := neighbor.X-c.X, neighbor.Y-c.Y
				nAlong := nx*tv.X + ny*tv.Y
				nRoad := ClassicRoadAt(t, neighbor.X, neighbor.Y, neighbor.Height, road.Index)
				if nAlong > -90 && nAlong < math.Max(120, c.Speed*.6) && math.Abs(nRoad.Side-lane) < 23 {
					clear = false
					break
				}
			}
			if clear {
				return lane
			}
		}
	}
	return base
}

func classicPlanSteering(c Car, t Track, lane, pursuit, throttle, brake float64) float64 {
	if c.Speed < 100 || c.Gear < 0 || c.Airborne {
		return pursuit
	}
	p := ClassicVehicle(c.Vehicle)
	body := ClassicVehicleBody(c.Vehicle)
	initial := classicMotionFromCar(c)
	road := ClassicRoadAt(t, c.X, c.Y, c.Height, c.RouteIndex)
	surface := ClassicTerrainSurface(t, &c, ClassicSurfaceFor(road, c.Angle))
	best, bestCost := pursuit, math.Inf(1)
	for k := -4; k <= 4; k++ {
		steer := clamp(pursuit+float64(k)*.07, -1, 1)
		motion := initial
		contact := road
		cost := math.Pow(steer-initial.Steering, 2) * .2
		for step := 0; step < 8; step++ {
			motion = StepClassicMotion(motion, ClassicMotionInput{Steer: steer, Throttle: throttle, BrakeAmount: brake}, p, surface, .1)
			contact = ClassicRoadAt(t, motion.X*10, motion.Z*10, contact.Height, contact.Index)
			left, right := math.Max(10, contact.Left-body.Width*5-5), math.Max(10, contact.Right-body.Width*5-5)
			desired := clamp(lane, -left, right)
			error := (contact.Side - desired) / 10
			tangent := tangent(t, contact.Index)
			heading := angle(math.Atan2(motion.VZ, motion.VX) - math.Atan2(tangent.Y, tangent.X))
			cost += error*error + heading*heading*30
			if contact.Side < -left || contact.Side > right {
				cost += 100 + error*error*4
			}
		}
		if cost < bestCost {
			best, bestCost = steer, cost
		}
	}
	return best
}
