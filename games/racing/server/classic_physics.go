package racing

import "math"

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
	dt := float64(ms) / 1000
	flight := classicFlightFromCar(*c)
	if !flight.Airborne {
		flight.VerticalSpeed = classicFlightGroundAt(*c, t).VerticalSpeed
	}
	next := motion
	if flight.Airborne {
		next = StepClassicAirMotion(motion, dt)
	} else {
		next = StepClassicMotion(motion, input, ClassicVehicle(c.Vehicle), ClassicTerrainSurface(t, c, ClassicSurfaceFor(road, c.Angle)), dt)
	}
	classicMotionToCar(c, next)
	ground := classicFlightGroundAt(*c, t)
	flight = StepClassicFlight(flight, ground, c.Speed, dt)
	classicFlightToCar(c, flight)
	next = classicMotionFromCar(*c)
	before := c.Speed
	if flight.Impact > 30 {
		before += flight.Impact * .5
	}
	next = sweepClassicRoadWithProps(t, motion, next, c.Vehicle, c.Height, dt, props)
	classicMotionToCar(c, next)
	classicPose(c, t)
	if c.Driver == nil {
		c.Driver = &classicDriverState{}
	}
	driver := c.Driver
	forward := c.VX*math.Cos(c.Angle) + c.VY*math.Sin(c.Angle)
	if c.Gear > 0 && (forward > 45 || driver.BlockedAt > 0 && math.Hypot(c.X-driver.OriginX, c.Y-driver.OriginY) > 20) {
		c.Driver = &classicDriverState{}
	} else if !protect && c.Speed < 40 && input.Throttle > .1 && !input.Handbrake && c.Gear > 0 && driver.BlockedAt == 0 {
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
	if c.Gear < 0 && c.Speed < 10 && driver.ReverseAt > 0 && at-driver.ReverseAt > 6500 && at >= driver.ForwardUntil {
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

func compactClassicCar(c *Car) {
	q := func(n, scale float64) float64 { return math.Round(n*scale) / scale }
	c.X = q(c.X, 1000)
	c.Y = q(c.Y, 1000)
	c.VX = q(c.VX, 1000)
	c.VY = q(c.VY, 1000)
	c.Speed = q(c.Speed, 1000)
	c.Height = q(c.Height, 1e6)
	c.Angle = q(c.Angle, 1e8)
	c.Pitch = q(c.Pitch, 1e6)
	c.Roll = q(c.Roll, 1e6)
	c.RPM = q(c.RPM, 10)
	c.Progress = q(c.Progress, 1000)
	if c.Motion != nil {
		m := *c.Motion
		m.X = q(m.X, 1e7)
		m.Z = q(m.Z, 1e7)
		m.VX = q(m.VX, 1e7)
		m.VZ = q(m.VZ, 1e7)
		m.Angle = q(m.Angle, 1e8)
		m.YawRate = q(m.YawRate, 1e8)
		m.Steering = q(m.Steering, 1e8)
		m.LongAccel = q(m.LongAccel, 1e5)
		m.LateralAccel = q(m.LateralAccel, 1e5)
		m.RPM = q(m.RPM, 1e4)
		m.ShiftTimer = q(m.ShiftTimer, 1e6)
		m.Throttle = q(m.Throttle, 1e6)
		m.Brake = q(m.Brake, 1e6)
		m.Handbrake = q(m.Handbrake, 1e6)
		m.ReverseTimer = q(m.ReverseTimer, 1e6)
		c.Motion = &m
	}
	c.VerticalSpeed = q(c.VerticalSpeed, 1e6)
}
