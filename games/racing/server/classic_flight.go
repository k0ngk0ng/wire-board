package racing

import "math"

type ClassicFlightState struct {
	Height        float64 `json:"height"`
	VerticalSpeed float64 `json:"verticalSpeed"`
	Pitch         float64 `json:"pitch"`
	Roll          float64 `json:"roll"`
	Airborne      bool    `json:"airborne"`
	Impact        float64 `json:"impact"`
}

type ClassicFlightGround struct {
	Height        float64 `json:"height"`
	VerticalSpeed float64 `json:"verticalSpeed"`
	Pitch         float64 `json:"pitch"`
	Roll          float64 `json:"roll"`
	Supported     bool    `json:"supported"`
}

func StepClassicFlight(s ClassicFlightState, ground ClassicFlightGround, speed, dt float64) ClassicFlightState {
	dt = clamp(classicFinite(dt, 0), 0, .1)
	s.Impact = 0
	if dt == 0 {
		return s
	}
	nextHeight := s.Height + s.VerticalSpeed*dt - 49.05*dt*dt
	nextSpeed := s.VerticalSpeed - 98.1*dt
	if s.Airborne || !ground.Supported || (speed > 80 && nextHeight > ground.Height+.08) {
		s.Height = nextHeight
		s.VerticalSpeed = nextSpeed
		s.Airborne = true
		s.Pitch += (math.Atan2(nextSpeed, math.Max(40, speed)) - s.Pitch) * (1 - math.Exp(-2*dt))
		s.Roll *= math.Exp(-dt)
		if !ground.Supported || s.Height > ground.Height {
			return s
		}
		s.Impact = math.Max(0, ground.VerticalSpeed-nextSpeed)
	}
	s.Height = ground.Height
	s.VerticalSpeed = ground.VerticalSpeed
	s.Pitch = ground.Pitch
	s.Roll = ground.Roll
	s.Airborne = false
	return s
}

func StepClassicAirMotion(s ClassicMotionState, dt float64) ClassicMotionState {
	dt = clamp(classicFinite(dt, 0), 0, .1)
	drag := math.Exp(-.000035 * math.Hypot(s.VX, s.VZ) * dt)
	s.VX *= drag
	s.VZ *= drag
	s.X += s.VX * dt
	s.Z += s.VZ * dt
	s.Angle = angle(s.Angle + s.YawRate*dt)
	s.YawRate *= math.Exp(-.15 * dt)
	s.LongAccel = 0
	s.LateralAccel = 0
	return s
}

func classicFlightFromCar(c Car) ClassicFlightState {
	return ClassicFlightState{Height: c.Height, VerticalSpeed: c.VerticalSpeed, Pitch: c.Pitch, Roll: c.Roll, Airborne: c.Airborne}
}

func classicFlightGroundAt(c Car, t Track) ClassicFlightGround {
	c.Airborne = false
	classicPose(&c, t)
	ground := ClassicTerrainAt(t, c.X, c.Y, c.Height)
	return ClassicFlightGround{Height: c.Height, VerticalSpeed: math.Tan(c.Pitch)*(c.VX*math.Cos(c.Angle)+c.VY*math.Sin(c.Angle)) + math.Tan(c.Roll)*(c.VX*math.Sin(c.Angle)-c.VY*math.Cos(c.Angle)), Pitch: c.Pitch, Roll: c.Roll, Supported: ground.Found || classicTerrain.byGeometry[t.Geometry] == nil}
}

func classicFlightToCar(c *Car, flight ClassicFlightState) {
	c.Height, c.VerticalSpeed, c.Pitch, c.Roll, c.Airborne = flight.Height, flight.VerticalSpeed, flight.Pitch, flight.Roll, flight.Airborne
	if flight.Impact > 30 {
		factor := math.Exp(-math.Min(10, flight.Impact/10-3) * .035)
		c.VX *= factor
		c.VY *= factor
		if c.Motion != nil {
			c.Motion.LongAccel = -math.Min(30, flight.Impact/5)
		}
	}
}
