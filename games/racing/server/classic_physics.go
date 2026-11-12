package racing

import "math"

// Driving telemetry is persisted for the explicit computer/assisted driver.
// Collecting it never takes over a human seat or writes race progress.
type classicDriverState struct {
	ForwardUntil int64   `json:"forwardUntil,omitempty"`
	BlockedAt    int64   `json:"blockedAt,omitempty"`
	ReverseAt    int64   `json:"reverseAt,omitempty"`
	OriginX      float64 `json:"originX,omitempty"`
	OriginY      float64 `json:"originY,omitempty"`
}

func classicMotionFromCar(c Car) ClassicMotionState {
	var motion ClassicMotionState
	if c.Motion != nil {
		motion = *c.Motion
	} else {
		motion = ClassicMotionState{Gear: 1, RPM: ClassicVehicle(c.Vehicle).IdleRPM}
	}
	motion.X = c.X / 10
	motion.Z = c.Y / 10
	motion.VX = c.VX / 10
	motion.VZ = c.VY / 10
	motion.Angle = c.Angle
	return motion
}
func classicMotionToCar(c *Car, motion ClassicMotionState) {
	c.Motion = &motion
	c.X = motion.X * 10
	c.Y = motion.Z * 10
	c.VX = motion.VX * 10
	c.VY = motion.VZ * 10
	c.Angle = motion.Angle
	c.Speed = math.Hypot(c.VX, c.VY)
	c.RPM = motion.RPM
	c.Gear = motion.Gear
}

func stepClassicCar(c *Car, t Track, ms, at int64, props *classicPropCollision) float64 {
	road := classicPose(c, t)
	motion := classicMotionFromCar(*c)
	protect := c.Applied.Protect || at-c.LastInputAt > lostInputMS || at < c.ResetUntil
	input := ClassicMotionInput{Steer: c.Steer, Throttle: c.Throttle, BrakeAmount: brakeLevel(c.Brake, c.BrakeAmount), Handbrake: c.Drift, AllowReverse: !protect}
	if protect {
		input = ClassicMotionInput{BrakeAmount: 1}
		motion.Throttle = 0
		motion.Brake = 1
		motion.ReverseTimer = 0
	}
	oldGear := motion.Gear
	next := StepClassicMotion(motion, input, ClassicVehicle(c.Vehicle), ClassicSurfaceFor(road, c.Angle), float64(ms)/1000)
	classicMotionToCar(c, next)
	classicPose(c, t)
	before := c.Speed
	next = collideClassicRoadWithProps(t, next, c.Vehicle, c.Height, props)
	classicMotionToCar(c, next)
	classicPose(c, t)
	if c.Driver == nil {
		c.Driver = &classicDriverState{}
	}
	driver := c.Driver
	forward := c.VX*math.Cos(c.Angle) + c.VY*math.Sin(c.Angle)
	if c.Gear > 0 && forward > 45 {
		c.Driver = &classicDriverState{}
	} else if !protect && c.Speed < 15 && input.Throttle > .7 && !input.Handbrake && c.Gear > 0 && driver.BlockedAt == 0 {
		driver.BlockedAt = at
		driver.OriginX = c.X
		driver.OriginY = c.Y
	}
	if c.Gear < 0 && (driver.ReverseAt == 0 || oldGear >= 0) {
		driver.ReverseAt = at
	}
	if c.Gear > 0 && oldGear < 0 && driver.ForwardUntil > at {
		driver.ForwardUntil = mathMaxInt64(driver.ForwardUntil, at+1500)
	}
	if c.Gear < 0 && c.Speed < 10 && driver.ReverseAt > 0 && at-driver.ReverseAt > 1800 && at >= driver.ForwardUntil {
		driver.ForwardUntil = at + 2500
	}
	if c.Gear > 0 && c.Speed < 15 && driver.ReverseAt > 0 && at-driver.ReverseAt > 4500 {
		driver.BlockedAt = at - 1300
		driver.ReverseAt = 0
		driver.OriginX = c.X
		driver.OriginY = c.Y
	}
	return math.Max(0, before-c.Speed)
}

func mathMaxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func collideClassicCars(s *State, at int64) {
	for pass := 0; pass < 3; pass++ {
		for i := range s.Cars {
			a := &s.Cars[i]
			if a.FinishAt > 0 {
				continue
			}
			for j := i + 1; j < len(s.Cars); j++ {
				b := &s.Cars[j]
				if b.FinishAt > 0 || math.Hypot(a.X-b.X, a.Y-b.Y) > 55 {
					continue
				}
				abody, bbody := ClassicVehicleBody(a.Vehicle), ClassicVehicleBody(b.Vehicle)
				if a.Height+abody.Height*10 < b.Height || b.Height+bbody.Height*10 < a.Height {
					continue
				}
				aa, az, ar := classicCapsule(a.X, a.Y, a.Angle, abody)
				ba, bz, br := classicCapsule(b.X, b.Y, b.Angle, bbody)
				p, q := classicSegmentClosest(aa, az, ba, bz)
				dx, dy := q.X-p.X, q.Y-p.Y
				distance := math.Hypot(dx, dy)
				depth := ar + br - distance
				if depth <= 0 {
					continue
				}
				nx, ny := 0., 0.
				if distance > 1e-7 {
					nx = dx / distance
					ny = dy / distance
				} else {
					nx = b.X - a.X
					ny = b.Y - a.Y
					l := math.Hypot(nx, ny)
					if l > 1e-7 {
						nx /= l
						ny /= l
					} else {
						nx = -math.Sin(a.Angle)
						ny = math.Cos(a.Angle)
					}
				}
				ia, ib := 1/ClassicVehicle(a.Vehicle).Mass, 1/ClassicVehicle(b.Vehicle).Mass
				a.X -= nx * (depth + .001) * ia / (ia + ib)
				a.Y -= ny * (depth + .001) * ia / (ia + ib)
				b.X += nx * (depth + .001) * ib / (ia + ib)
				b.Y += ny * (depth + .001) * ib / (ia + ib)
				relative := (a.VX-b.VX)*nx + (a.VY-b.VY)*ny
				if relative > 0 {
					impulse := relative * 1.15 / (ia + ib)
					a.VX -= impulse * ia * nx
					a.VY -= impulse * ia * ny
					b.VX += impulse * ib * nx
					b.VY += impulse * ib * ny
					if relative > 45 {
						s.emit("collision", i, at, clamp(relative/200, 0, 1))
					}
				}
				classicMotionToCar(a, classicMotionFromCar(*a))
				classicMotionToCar(b, classicMotionFromCar(*b))
				classicPose(a, s.Track)
				classicPose(b, s.Track)
			}
		}
	}
}

func compactClassicCar(c *Car) {
	q := func(n, scale float64) float64 { return math.Round(n*scale) / scale }
	c.X = q(c.X, 1000)
	c.Y = q(c.Y, 1000)
	c.VX = q(c.VX, 1000)
	c.VY = q(c.VY, 1000)
	c.Speed = q(c.Speed, 1000)
	c.Height = q(c.Height, 1000)
	c.Angle = q(c.Angle, 1e6)
	c.Pitch = q(c.Pitch, 1e6)
	c.Roll = q(c.Roll, 1e6)
	c.RPM = q(c.RPM, 10)
	c.Progress = q(c.Progress, 1000)
	if c.Motion != nil {
		m := *c.Motion
		m.X = q(m.X, 1e5)
		m.Z = q(m.Z, 1e5)
		m.VX = q(m.VX, 1e5)
		m.VZ = q(m.VZ, 1e5)
		m.Angle = q(m.Angle, 1e6)
		m.YawRate = q(m.YawRate, 1e6)
		m.Steering = q(m.Steering, 1e6)
		m.LongAccel = q(m.LongAccel, 1e5)
		m.LateralAccel = q(m.LateralAccel, 1e5)
		m.RPM = q(m.RPM, 10)
		m.ShiftTimer = q(m.ShiftTimer, 1e6)
		m.Throttle = q(m.Throttle, 1e6)
		m.Brake = q(m.Brake, 1e6)
		m.Handbrake = q(m.Handbrake, 1e6)
		m.ReverseTimer = q(m.ReverseTimer, 1e6)
		c.Motion = &m
	}
}
