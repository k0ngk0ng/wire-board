package racing

import "math"

// ClassicMotionState is an independent SI motion state: metres, seconds and radians.
// Angle zero points along +X; positive steering rotates toward +Z.
type ClassicMotionState struct {
	X            float64 `json:"x"`
	Z            float64 `json:"z"`
	VX           float64 `json:"vx"`
	VZ           float64 `json:"vz"`
	Angle        float64 `json:"angle"`
	YawRate      float64 `json:"yawRate"`
	Steering     float64 `json:"steering"`
	LongAccel    float64 `json:"longAccel"`
	LateralAccel float64 `json:"lateralAccel"`
	RPM          float64 `json:"rpm"`
	Gear         int     `json:"gear"`
	ShiftTimer   float64 `json:"shiftTimer"`
	Throttle     float64 `json:"throttle"`
	Brake        float64 `json:"brake"`
	Handbrake    float64 `json:"handbrake"`
	ReverseTimer float64 `json:"reverseTimer"`
}

type ClassicMotionInput struct {
	Steer       float64 `json:"steer"`
	Throttle    float64 `json:"throttle"`
	BrakeAmount float64 `json:"brakeAmount"`
	Handbrake   bool    `json:"handbrake"`
	// Only intentional, current player/bot input may request reversing.
	AllowReverse bool `json:"allowReverse"`
}

type ClassicRoadSurface struct {
	Resistance float64 `json:"resistance"`
	Grip       float64 `json:"grip"`
	Grade      float64 `json:"grade"`
	NormalY    float64 `json:"normalY"`
}

type ClassicVehicleParams struct {
	GearCount                                      int
	ReverseVelocityToRPM, ReverseEfficiency        float64
	ID                                             string
	Mass, Wheelbase, TireRadius, MaxSpeed          float64
	IdleRPM, RedlineRPM, FinalDrive, MaxBraking    float64
	FrontGripBias, FrontBrakeBias, FrontDriveRatio float64
	TurnInRate, TurnOutRate, TurningRadius         float64
	LateralGrip, GasOffFactor, ShiftDelay          float64
	GearRatios, GearEfficiency, VelocityToRPM      [6]float64
	TorqueCurve                                    [21]float64
}

// ClassicVehicle preserves the base CARP data shipped in the NFS4 public demo.
// The physical interpretation below is independent; these are not measured runs.
func ClassicVehicle(id string) ClassicVehicleParams {
	switch id {
	case "fm50":
		return ClassicVehicleParams{
			ID:                   "fm50",
			Mass:                 1690.0,
			Wheelbase:            2.5,
			TireRadius:           0.33185,
			MaxSpeed:             88.962402,
			IdleRPM:              1155,
			RedlineRPM:           7700,
			FinalDrive:           3.86,
			MaxBraking:           10.2,
			FrontGripBias:        0.481,
			FrontBrakeBias:       0.5312,
			FrontDriveRatio:      0.0,
			TurnInRate:           3.0,
			TurnOutRate:          6.0,
			TurningRadius:        5.75,
			LateralGrip:          3.43,
			GasOffFactor:         0.374991,
			ShiftDelay:           0.25,
			GearCount:            6,
			ReverseVelocityToRPM: 360.994202,
			ReverseEfficiency:    0.8,
			GearRatios:           [6]float64{3.153846, 2.17647, 1.565217, 1.185185, 0.935483, 0.757575},
			GearEfficiency:       [6]float64{0.67, 0.67, 0.67, 0.7, 0.91, 1.28},
			VelocityToRPM:        [6]float64{350.313904, 241.75177, 173.856735, 131.644608, 103.908997, 84.147835},
			TorqueCurve:          [21]float64{0.0, 350.0, 390.0, 425.0, 430.0, 430.0, 450.0, 475.0, 530.0, 550.0, 568.5, 550.0, 525.0, 510.0, 490.0, 450.0, 400.0, 350.0, 0.0, 0.0, 0.0},
		}
	case "ldia":
		return ClassicVehicleParams{
			ID:                   "ldia",
			Mass:                 1625.0,
			Wheelbase:            2.65,
			TireRadius:           0.34585,
			MaxSpeed:             93.055557,
			IdleRPM:              1030,
			RedlineRPM:           7300,
			FinalDrive:           3.8319,
			MaxBraking:           10.25,
			FrontGripBias:        0.47,
			FrontBrakeBias:       0.55624,
			FrontDriveRatio:      0.5,
			TurnInRate:           4.5,
			TurnOutRate:          8.0,
			TurningRadius:        6.27,
			LateralGrip:          3.64,
			GasOffFactor:         0.285,
			ShiftDelay:           0.1875,
			GearCount:            5,
			ReverseVelocityToRPM: 224.831268,
			ReverseEfficiency:    0.8,
			GearRatios:           [6]float64{2.31, 1.52, 1.12, 0.83, 0.68, 0.0},
			GearEfficiency:       [6]float64{0.86, 0.87, 0.87, 0.89, 1.055, 0.0},
			VelocityToRPM:        [6]float64{244.4048, 160.82048, 118.499298, 87.816444, 71.946007, 0.0},
			TorqueCurve:          [21]float64{0.0, 300.0, 350.0, 401.0, 425.0, 450.0, 450.0, 460.0, 515.0, 550.0, 575.0, 600.0, 585.0, 550.0, 500.0, 450.0, 0.0, 0.0, 0.0, 0.0, 0.0},
		}
	case "mcf1":
		return ClassicVehicleParams{
			ID:                   "mcf1",
			Mass:                 915.0,
			Wheelbase:            2.718,
			TireRadius:           0.4736,
			MaxSpeed:             88.068306,
			IdleRPM:              1095,
			RedlineRPM:           7300,
			FinalDrive:           3.45,
			MaxBraking:           11.8,
			FrontGripBias:        0.49,
			FrontBrakeBias:       0.5328,
			FrontDriveRatio:      0.0,
			TurnInRate:           4.0,
			TurnOutRate:          8.0,
			TurningRadius:        6.1,
			LateralGrip:          3.9,
			GasOffFactor:         0.425,
			ShiftDelay:           0.125,
			GearCount:            6,
			ReverseVelocityToRPM: 197.10582,
			ReverseEfficiency:    0.799988,
			GearRatios:           [6]float64{2.857143, 2.176471, 1.789474, 1.473684, 1.272727, 1.15},
			GearEfficiency:       [6]float64{0.75, 0.95, 0.95, 0.899994, 0.95, 1.06},
			VelocityToRPM:        [6]float64{193.707443, 147.489532, 121.661873, 99.912262, 86.318756, 74.764282},
			TorqueCurve:          [21]float64{0.0, 474.529999, 528.762024, 576.215027, 582.994019, 582.994019, 610.109985, 644.005005, 677.900024, 714.506592, 711.794983, 691.458008, 664.34198, 610.109985, 542.320007, 474.25473, 0.0, 0.0, 0.0, 0.0, 0.0},
		}
	case "ccor":
		return ClassicVehicleParams{
			ID:                   "ccor",
			Mass:                 1472.0,
			Wheelbase:            2.6543,
			TireRadius:           0.3386,
			MaxSpeed:             76.445084,
			IdleRPM:              900,
			RedlineRPM:           6000,
			FinalDrive:           3.15,
			MaxBraking:           10.5,
			FrontGripBias:        0.49,
			FrontBrakeBias:       0.5328,
			FrontDriveRatio:      0.0,
			TurnInRate:           4.0,
			TurnOutRate:          8.0,
			TurningRadius:        6.1,
			LateralGrip:          3.23,
			GasOffFactor:         0.405,
			ShiftDelay:           0.1875,
			GearCount:            4,
			ReverseVelocityToRPM: 179.70903,
			ReverseEfficiency:    0.799988,
			GearRatios:           [6]float64{3.73, 2.12, 1.43, 1.04, 0.0, 0.0},
			GearEfficiency:       [6]float64{0.83, 0.82, 0.85, 1.18, 0.0, 0.0},
			VelocityToRPM:        [6]float64{292.713837, 166.368179, 112.220047, 75.232437, 0.0, 0.0},
			TorqueCurve:          [21]float64{0.0, 302.367798, 380.979797, 393.182007, 412.163208, 420.298004, 440.63501, 454.192993, 474.529999, 474.529999, 440.63501, 406.73999, 372.845001, 338.950012, 305.054993, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
		}
	default:
		return ClassicVehicleParams{
			ID:                   "p911",
			Mass:                 1500.0,
			Wheelbase:            2.272,
			TireRadius:           0.3141,
			MaxSpeed:             80.555557,
			IdleRPM:              975,
			RedlineRPM:           6500,
			FinalDrive:           3.444444,
			MaxBraking:           11.8,
			FrontGripBias:        0.45,
			FrontBrakeBias:       0.6345,
			FrontDriveRatio:      0.0,
			TurnInRate:           4.0,
			TurnOutRate:          8.0,
			TurningRadius:        11.0,
			LateralGrip:          3.7,
			GasOffFactor:         0.3424,
			ShiftDelay:           0.1875,
			GearCount:            6,
			ReverseVelocityToRPM: 299.19516,
			ReverseEfficiency:    0.8,
			GearRatios:           [6]float64{3.818182, 2.15, 1.45, 1.11, 0.92, 0.7},
			GearEfficiency:       [6]float64{0.68, 0.7, 0.82, 0.91, 0.95, 1.28},
			VelocityToRPM:        [6]float64{399.833557, 225.144363, 151.841553, 116.237328, 96.340843, 73.302818},
			TorqueCurve:          [21]float64{0.0, 205.0, 240.0, 275.0, 345.0, 455.0, 500.0, 525.0, 535.0, 540.0, 515.0, 500.0, 455.0, 410.0, 360.0, 310.0, 0.0, 0.0, 0.0, 0.0, 0.0},
		}
	}
}

func classicFinite(n, fallback float64) float64 {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return fallback
	}
	return n
}
func classicClamp(n, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, n)) }
func classicToward(n, target, step float64) float64 {
	if n < target {
		return math.Min(target, n+step)
	}
	return math.Max(target, n-step)
}
func classicSign(n float64) float64 {
	if n > 0 {
		return 1
	}
	if n < 0 {
		return -1
	}
	return 0
}
func classicTorque(p ClassicVehicleParams, rpm float64) float64 {
	x := classicClamp(rpm/500, 0, 20)
	i := int(math.Min(19, math.Floor(x)))
	return p.TorqueCurve[i] + (p.TorqueCurve[i+1]-p.TorqueCurve[i])*(x-float64(i))
}

func validClassicVehicle(p ClassicVehicleParams) bool {
	if p.GearCount < 1 || p.GearCount > 6 {
		return false
	}
	positive := []float64{p.Mass, p.Wheelbase, p.TireRadius, p.MaxSpeed, p.IdleRPM, p.RedlineRPM, p.FinalDrive, p.MaxBraking, p.TurnInRate, p.TurnOutRate, p.TurningRadius, p.LateralGrip, p.ShiftDelay, p.ReverseVelocityToRPM, p.ReverseEfficiency}
	for _, n := range positive {
		if n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	for _, n := range []float64{p.FrontGripBias, p.FrontBrakeBias, p.FrontDriveRatio, p.GasOffFactor} {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	for i := 0; i < p.GearCount; i++ {
		for _, n := range []float64{p.GearRatios[i], p.GearEfficiency[i], p.VelocityToRPM[i]} {
			if n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
				return false
			}
		}
	}
	for _, n := range p.TorqueCurve {
		if n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	return true
}

func cleanClassicState(s ClassicMotionState, p ClassicVehicleParams) ClassicMotionState {
	s.X = classicFinite(s.X, 0)
	s.Z = classicFinite(s.Z, 0)
	s.VX = classicClamp(classicFinite(s.VX, 0), -200, 200)
	s.VZ = classicClamp(classicFinite(s.VZ, 0), -200, 200)
	s.Angle = classicFinite(s.Angle, 0)
	s.YawRate = classicClamp(classicFinite(s.YawRate, 0), -3, 3)
	s.Steering = classicClamp(classicFinite(s.Steering, 0), -1, 1)
	s.LongAccel = classicClamp(classicFinite(s.LongAccel, 0), -30, 30)
	s.LateralAccel = classicClamp(classicFinite(s.LateralAccel, 0), -40, 40)
	s.RPM = classicClamp(classicFinite(s.RPM, p.IdleRPM), p.IdleRPM, p.RedlineRPM*1.05)
	if s.Gear < -1 || s.Gear > p.GearCount || s.Gear == 0 {
		s.Gear = 1
	}
	s.ShiftTimer = classicClamp(classicFinite(s.ShiftTimer, 0), 0, 2)
	s.Throttle = classicClamp(classicFinite(s.Throttle, 0), 0, 1)
	s.Brake = classicClamp(classicFinite(s.Brake, 0), 0, 1)
	s.Handbrake = classicClamp(classicFinite(s.Handbrake, 0), 0, 1)
	s.ReverseTimer = classicClamp(classicFinite(s.ReverseTimer, 0), 0, 1)
	return s
}

// StepClassicMotion advances at most 100 ms in fixed steps no longer than 20 ms.
// Tyre stiffness, CG height, low-speed blend and drag fit are explicit design
// choices, rather than claims about unrecovered original executable formulas.
func StepClassicMotion(s ClassicMotionState, in ClassicMotionInput, p ClassicVehicleParams, road ClassicRoadSurface, dt float64) ClassicMotionState {
	if !validClassicVehicle(p) {
		p = ClassicVehicle(p.ID)
	}
	s = cleanClassicState(s, p)
	in.Steer = classicClamp(classicFinite(in.Steer, 0), -1, 1)
	in.Throttle = classicClamp(classicFinite(in.Throttle, 0), 0, 1)
	in.BrakeAmount = classicClamp(classicFinite(in.BrakeAmount, 0), 0, 1)
	road.Grip = classicFinite(road.Grip, 1)
	if road.Grip <= 0 {
		road.Grip = 1
	}
	road.Grip = classicClamp(road.Grip, .1, 1.5)
	road.NormalY = classicFinite(road.NormalY, 1)
	if road.NormalY <= 0 {
		road.NormalY = 1
	}
	road.NormalY = classicClamp(road.NormalY, .25, 1)
	road.Grade = classicClamp(classicFinite(road.Grade, 0), -.65, .65)
	road.Resistance = classicClamp(classicFinite(road.Resistance, 0), 0, 20)
	dt = classicClamp(classicFinite(dt, 0), 0, .1)
	for left := dt; left > 1e-9; left -= .02 {
		s = classicMotionStep(s, in, p, road, math.Min(.02, left))
	}
	return s
}

func classicMotionStep(s ClassicMotionState, in ClassicMotionInput, p ClassicVehicleParams, road ClassicRoadSurface, dt float64) ClassicMotionState {
	cs, sn := math.Cos(s.Angle), math.Sin(s.Angle)
	u, v := s.VX*cs+s.VZ*sn, -s.VX*sn+s.VZ*cs
	steerRate := p.TurnInRate
	if in.Steer*s.Steering < 0 || math.Abs(in.Steer) < math.Abs(s.Steering) {
		steerRate = p.TurnOutRate
	}
	steerTarget := in.Steer
	if in.Steer*s.Steering < 0 {
		steerTarget = 0
	}
	s.Steering = classicToward(s.Steering, steerTarget, steerRate*dt)
	handTarget := 0.
	if in.Handbrake {
		handTarget = 1
	}
	s.Handbrake = classicToward(s.Handbrake, handTarget, 10*dt)
	if in.AllowReverse && math.Abs(u) < .4 && ((s.Gear > 0 && in.BrakeAmount > .5 && in.Throttle < .1) || (s.Gear < 0 && in.Throttle > .5 && in.BrakeAmount < .1)) {
		s.ReverseTimer += dt
		if s.ReverseTimer >= .3 {
			if s.Gear < 0 {
				s.Gear = 1
			} else {
				s.Gear = -1
			}
			s.ShiftTimer = p.ShiftDelay
			s.Throttle = 0
			s.Brake = 0
			s.ReverseTimer = 0
		}
	} else {
		s.ReverseTimer = 0
	}
	throttleTarget, brakeTarget := in.Throttle, in.BrakeAmount
	if s.Gear < 0 {
		if in.AllowReverse {
			throttleTarget, brakeTarget = in.BrakeAmount, in.Throttle
		} else {
			throttleTarget = 0
			brakeTarget = math.Max(in.Throttle, in.BrakeAmount)
		}
	}
	throttleRate := 16.0 / 255 * 32
	if throttleTarget < s.Throttle {
		throttleRate = 32.0 / 255 * 32
	}
	s.Throttle = classicToward(s.Throttle, throttleTarget, throttleRate*dt)
	brakeCurve := [8]float64{64, 32, 16, 8, 4, 2, 1, 1}
	idx := int(math.Round(s.Brake*255)) >> 5
	if idx > 7 {
		idx = 7
	}
	brakeRate := brakeCurve[idx] / 255 * 32
	if brakeTarget < s.Brake {
		brakeRate = 32.0 / 255 * 32
	}
	s.Brake = classicToward(s.Brake, brakeTarget, brakeRate*dt)
	s.ShiftTimer = math.Max(0, s.ShiftTimer-dt)
	gearIndex := s.Gear - 1
	if gearIndex < 0 {
		gearIndex = 0
	}
	velocityRatio := p.VelocityToRPM[gearIndex]
	if s.Gear < 0 {
		velocityRatio = p.ReverseVelocityToRPM
	}
	wheelRPM := math.Abs(u) * velocityRatio
	if s.ShiftTimer == 0 && s.Gear > 0 {
		if s.Gear < p.GearCount && wheelRPM > p.RedlineRPM*.94 {
			s.Gear++
			s.ShiftTimer = p.ShiftDelay
		} else if s.Gear > 1 && wheelRPM < p.RedlineRPM*.36 && math.Abs(u)*p.VelocityToRPM[s.Gear-2] < p.RedlineRPM*.78 {
			s.Gear--
			s.ShiftTimer = p.ShiftDelay
		}
		gearIndex = s.Gear - 1
		velocityRatio = p.VelocityToRPM[gearIndex]
		wheelRPM = math.Abs(u) * velocityRatio
	}
	rpmTarget := math.Max(p.IdleRPM, wheelRPM)
	if math.Abs(u) < 5 && s.Throttle > 0 {
		rpmTarget = math.Max(rpmTarget, p.IdleRPM+s.Throttle*1200)
	}
	rpmTarget = classicClamp(rpmTarget, p.IdleRPM, p.RedlineRPM*1.025)
	s.RPM += (rpmTarget - s.RPM) * (1 - math.Exp(-12*dt))
	efficiency := p.GearEfficiency[gearIndex]
	if s.Gear < 0 {
		efficiency = p.ReverseEfficiency
	}
	engineForce := classicTorque(p, s.RPM) * velocityRatio * (2 * math.Pi / 60) * efficiency * s.Throttle
	if s.ShiftTimer > 0 {
		engineForce = 0
	}
	if s.Gear < 0 {
		engineForce = -engineForce
		if u < -12 {
			engineForce = 0
		}
	}
	if wheelRPM > p.RedlineRPM*1.025 {
		engineForce = 0
	}
	brakeForce := p.Mass * p.MaxBraking * s.Brake * classicSign(u)
	frontFraction := classicClamp(p.FrontGripBias, .3, .65)
	a, b := p.Wheelbase*(1-frontFraction), p.Wheelbase*frontFraction
	// Use the original bias as a static axle-load proxy; 0.48 m CG height is chosen.
	weight := p.Mass * 9.81 * road.NormalY
	frontLoad := classicClamp(weight*frontFraction-p.Mass*s.LongAccel*.48/p.Wheelbase, weight*.15, weight*.85)
	rearLoad := weight - frontLoad
	mu := p.LateralGrip * .5 * road.Grip
	frontLimit, rearLimit := mu*frontLoad, mu*rearLoad
	frontX := engineForce*p.FrontDriveRatio - brakeForce*p.FrontBrakeBias
	rearX := engineForce*(1-p.FrontDriveRatio) - brakeForce*(1-p.FrontBrakeBias) - classicSign(u)*p.Mass*5*s.Handbrake
	frontX = classicClamp(frontX, -frontLimit, frontLimit)
	rearX = classicClamp(rearX, -rearLimit, rearLimit)
	frontYLimit := math.Sqrt(math.Max(0, frontLimit*frontLimit-frontX*frontX))
	rearYLimit := math.Sqrt(math.Max(0, rearLimit*rearLimit-rearX*rearX)) * (1 - .82*s.Handbrake)
	maxSteer := math.Atan(p.Wheelbase / math.Max(p.TurningRadius, 1))
	delta := s.Steering * maxSteer / (1 + .000055*u*u)
	denom := math.Max(math.Abs(u), 1)
	alphaFront := math.Atan2(v+a*s.YawRate, denom) - delta*classicSign(u)
	alphaRear := math.Atan2(v-b*s.YawRate, denom)
	// Ten times axle load is an independently chosen cornering stiffness.
	frontY := 0.
	if frontYLimit > 1e-6 {
		frontY = -frontYLimit * math.Tanh(frontLoad*10*alphaFront/frontYLimit)
	}
	rearY := 0.
	if rearYLimit > 1e-6 {
		rearY = -rearYLimit * math.Tanh(rearLoad*10*alphaRear/rearYLimit)
	}
	topIndex := p.GearCount - 1
	topRPM := p.MaxSpeed * p.VelocityToRPM[topIndex]
	topAcceleration := classicTorque(p, topRPM) * p.VelocityToRPM[topIndex] * (2 * math.Pi / 60) * p.GearEfficiency[topIndex] / p.Mass
	dragCoefficient := math.Max(.00001, (topAcceleration-.13)/(p.MaxSpeed*p.MaxSpeed))
	rolling := .13 * (1 + math.Max(0, 1-road.Grip)*3)
	// Rough-ground resistance builds with speed instead of exceeding available
	// traction at a standstill; its full configured value remains above 6 m/s.
	dragAcceleration := (rolling + road.Resistance*classicClamp(math.Abs(u)/6, 0, 1) + dragCoefficient*u*u) * classicSign(u)
	if s.Throttle < .05 {
		dragAcceleration += p.GasOffFactor * (1 + p.GearRatios[gearIndex]*.35) * classicSign(u)
	}
	longAccel := (frontX+rearX-frontY*math.Sin(delta))/p.Mass - dragAcceleration - 9.81*math.Sin(road.Grade)
	latAccel := (frontY*math.Cos(delta) + rearY) / p.Mass
	nextU := u + (longAccel+v*s.YawRate)*dt
	nextV := v + (latAccel-u*s.YawRate)*dt
	inertia := p.Mass * p.Wheelbase * p.Wheelbase * .35
	nextYaw := s.YawRate + (a*frontY*math.Cos(delta)-b*rearY)/inertia*dt
	nextYaw *= math.Exp(-(.18 + .18*(1-s.Handbrake)) * dt)
	// Blend to the kinematic bicycle near walking speed to avoid tyre-force chatter.
	beta := math.Atan(b / p.Wheelbase * math.Tan(delta))
	kinYaw := nextU * math.Tan(delta) / p.Wheelbase
	kinV := nextU * math.Tan(beta)
	blend := classicClamp((math.Abs(u)-2)/5, 0, 1)
	nextV = kinV*(1-blend) + nextV*blend
	nextYaw = kinYaw*(1-blend) + nextYaw*blend
	maxYaw := math.Min(2.4, math.Max(.35, mu*9.81/(math.Abs(u)+4))*1.3)
	nextYaw = classicClamp(nextYaw, -maxYaw, maxYaw)
	if s.Brake > .01 && throttleTarget < .01 && ((u*nextU < 0) || math.Abs(u) < p.MaxBraking*s.Brake*dt+.02) {
		nextU = 0
	}
	if math.Abs(nextU) < .015 && s.Throttle < .01 {
		nextU = 0
		nextV = 0
		nextYaw = 0
	}
	oldYaw := s.YawRate
	s.YawRate = nextYaw
	s.Angle = math.Atan2(math.Sin(s.Angle+(oldYaw+nextYaw)*.5*dt), math.Cos(s.Angle+(oldYaw+nextYaw)*.5*dt))
	cs, sn = math.Cos(s.Angle), math.Sin(s.Angle)
	s.VX = nextU*cs - nextV*sn
	s.VZ = nextU*sn + nextV*cs
	if speed := math.Hypot(s.VX, s.VZ); speed > p.MaxSpeed*1.4 {
		s.VX *= p.MaxSpeed * 1.4 / speed
		s.VZ *= p.MaxSpeed * 1.4 / speed
	}
	s.X += s.VX * dt
	s.Z += s.VZ * dt
	s.LongAccel = longAccel
	s.LateralAccel = latAccel
	return s
}
