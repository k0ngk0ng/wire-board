package racing

import (
	"math"
	"testing"
)

func TestClassicFlightCrestGravityLandingAndFlatRoad(t *testing.T) {
	flat := ClassicFlightGround{Height: 0, Supported: true}
	s := ClassicFlightState{Height: 0}
	for i := 0; i < 100; i++ {
		s = StepClassicFlight(s, flat, 1000, .02)
		if s.Airborne || s.Height != 0 {
			t.Fatal("speed alone launched a car on flat asphalt")
		}
	}
	s = ClassicFlightState{Height: 20, VerticalSpeed: 100, Pitch: .1}
	ground := ClassicFlightGround{Height: 20, Supported: true}
	s = StepClassicFlight(s, ground, 800, .02)
	if !s.Airborne || s.Height < 21 || s.VerticalSpeed >= 100 {
		t.Fatalf("high-speed crest did not launch ballistically: %+v", s)
	}
	peak := s.Height
	for i := 0; i < 200 && s.Airborne; i++ {
		s = StepClassicFlight(s, ground, 800, .02)
		peak = math.Max(peak, s.Height)
	}
	if s.Airborne || s.Height != 20 || s.Impact < 80 || peak < 60 {
		t.Fatalf("flight did not end with a supported impact: peak=%v state=%+v", peak, s)
	}
	for i := 0; i < 50; i++ {
		s = StepClassicFlight(s, ground, 800, .02)
		if s.Airborne || s.Height != 20 || s.Impact != 0 {
			t.Fatal("landing bounced or repeated its impact")
		}
	}
	s = StepClassicFlight(ClassicFlightState{}, ClassicFlightGround{Height: 1, Supported: true, VerticalSpeed: 5}, 20, .02)
	if s.Airborne || s.Height != 1 {
		t.Fatal("slow driving did not follow the road")
	}
}

func TestClassicFlightAirControlCannotSteerOrBrake(t *testing.T) {
	s := ClassicMotionState{X: 3, Z: 4, VX: 80, VZ: 5, Angle: .2, YawRate: .4, Gear: 4, RPM: 6000}
	next := StepClassicAirMotion(s, .02)
	if next.VX < 79.8 || next.VZ < 4.98 || next.X <= s.X || next.Z <= s.Z || next.Gear != 4 {
		t.Fatalf("airborne car lost its momentum: %+v", next)
	}
	if math.Atan2(next.VZ, next.VX) != math.Atan2(s.VZ, s.VX) {
		t.Fatal("airborne trajectory turned without tyre contact")
	}
}
