package racing

// Isolated failure cases declared before implementation: different skins alter
// acceleration; drift fails to retain sideways momentum; grass does not slow;
// collision creates NaN or overlap; out-of-order/reverse gates add a lap;
// reset advances progress; latest input affects already elapsed physics.
import (
	"math"
	"testing"
)

func TestVehicleSurfacesDriftAndSkins(t *testing.T) {
	track := Tracks()[0]
	p := track.Points[3]
	next := track.Points[4]
	a := math.Atan2(next.Y-p.Y, next.X-p.X)
	base := Car{X: p.X, Y: p.Y, Angle: a, Throttle: 1, LastInputAt: 1000}
	cars := []Car{base, base}
	cars[1].Skin = 4
	for i := 0; i < 25; i++ {
		for j := range cars {
			stepCar(&cars[j], track, 20, 1000+int64(i)*20)
		}
	}
	if cars[0].X != cars[1].X || cars[0].Speed != cars[1].Speed {
		t.Fatal("skin changed physics")
	}
	road := base
	road.VX = math.Cos(a) * 200
	road.VY = math.Sin(a) * 200
	grass := road
	grass.X = 50
	grass.Y = 50
	for i := 0; i < 15; i++ {
		stepCar(&road, track, 20, 1000+int64(i)*20)
		stepCar(&grass, track, 20, 1000+int64(i)*20)
	}
	if grass.Speed >= road.Speed*.8 {
		t.Fatalf("grass not slower %v %v", grass.Speed, road.Speed)
	}
	normal := base
	normal.VX = math.Cos(a)*160 - math.Sin(a)*80
	normal.VY = math.Sin(a)*160 + math.Cos(a)*80
	drift := normal
	drift.Drift = true
	normal.Steer = .4
	drift.Steer = .4
	for i := 0; i < 10; i++ {
		stepCar(&normal, track, 20, 1000+int64(i)*20)
		stepCar(&drift, track, 20, 1000+int64(i)*20)
	}
	side := func(c Car) float64 { return math.Abs(-math.Sin(c.Angle)*c.VX + math.Cos(c.Angle)*c.VY) }
	if side(drift) <= side(normal) {
		t.Fatal("drift lacks lateral slip")
	}
}
func TestOrderedGateProtection(t *testing.T) {
	track := Tracks()[0]
	s := State{Options: Options{Track: track.ID, Laps: 1}, Track: track, Phase: "racing", StartAt: 1000, MaxRaceAt: 481000}
	c := Car{Seat: 0, Checkpoint: 1, Gate: 0, Lap: 0}
	p := track.Points[0]
	c.X = p.X
	c.Y = p.Y
	s.Cars = []Car{c}
	last := track.Points[len(track.Points)-1]
	updateProgress(&s, &s.Cars[0], last, 1001)
	if s.Cars[0].Lap != 0 || s.Cars[0].FinishAt != 0 {
		t.Fatal("finish shortcut counted")
	}
	s.Cars[0].Checkpoint = 0
	s.Cars[0].Gate = len(track.Points) - 1
	s.Cars[0].X = last.X
	s.Cars[0].Y = last.Y
	updateProgress(&s, &s.Cars[0], p, 1002)
	if s.Cars[0].Lap != 0 {
		t.Fatal("reverse line counted")
	}
}
func TestCollisionRemainsFiniteAndSeparated(t *testing.T) {
	cars := []Car{{Seat: 0, X: 100, Y: 100, VX: 100}, {Seat: 1, X: 100, Y: 100, VX: -100}}
	s := State{Cars: cars, Events: []Event{}}
	collide(&s, 1000)
	a, b := s.Cars[0], s.Cars[1]
	if !finite(a.X) || !finite(b.X) || math.Hypot(a.X-b.X, a.Y-b.Y) < 25.9 {
		t.Fatal("invalid collision separation", s.Cars)
	}
}

// Capsule contact failure scenarios (before replacing circular contact): a
// 46-unit long car must touch nose-to-nose before centers are 26 apart; a side
// hit must respect both the nose and side; parallel rubbing must not inject
// speed, rotate the car, or remain interpenetrating. All skins share geometry.
func TestCapsuleContactMatchesCarSilhouette(t *testing.T) {
	for _, tt := range []struct {
		name string
		cars []Car
	}{
		{"nose_to_nose", []Car{{Seat: 0, X: 100, Y: 100, Angle: 0, VX: 100}, {Seat: 1, X: 140, Y: 100, Angle: math.Pi, VX: -100}}},
		{"nose_to_side", []Car{{Seat: 0, X: 100, Y: 100, Angle: 0, VX: 100}, {Seat: 1, X: 132, Y: 100, Angle: math.Pi / 2}}},
		{"parallel_rubbing", []Car{{Seat: 0, X: 100, Y: 100, Angle: 0, VX: 100, VY: 15}, {Seat: 1, X: 104, Y: 124, Angle: 0, VX: 100, VY: -15}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := State{Cars: tt.cars, Events: []Event{}}
			before := append([]Car{}, s.Cars...)
			energy := func(cars []Car) float64 {
				v := 0.
				for _, c := range cars {
					v += c.VX*c.VX + c.VY*c.VY
				}
				return v
			}
			collide(&s, 1000)
			a, b := s.Cars[0], s.Cars[1]
			if a.X == before[0].X && a.Y == before[0].Y {
				t.Fatal("visible car body passed through opponent")
			}
			if energy(s.Cars) > energy(before)+1e-6 {
				t.Fatal("contact injected energy")
			}
			if a.Angle != before[0].Angle || b.Angle != before[1].Angle {
				t.Fatal("contact rotated vehicle unpredictably")
			}
			if tt.name == "nose_to_nose" && b.X-a.X < 45.99 {
				t.Fatal("nose-to-nose body overlap", a, b)
			}
			if tt.name == "nose_to_side" && b.X-a.X < 35.99 {
				t.Fatal("nose penetrated side", a, b)
			}
			if tt.name == "parallel_rubbing" && b.Y-a.Y < 25.99 {
				t.Fatal("parallel body overlap", a, b)
			}
		})
	}
}

// Failure scenario before finish-time interpolation: cars that cross at
// different fractions of one 20 ms step must not share first place merely
// because the server publishes them in the same frame.
func TestFinishRanksUseActualCrossingWithinStep(t *testing.T) {
	tr := Tracks()[0]
	p, v := tr.Points[0], tangent(tr, 0)
	s := State{Track: tr, Options: Options{Track: tr.ID, Laps: 1}, Phase: "racing", Cars: []Car{
		{Seat: 0, Checkpoint: 0, Gate: len(tr.Points) - 1, X: p.X + v.X*2, Y: p.Y + v.Y*2},
		{Seat: 1, Checkpoint: 0, Gate: len(tr.Points) - 1, X: p.X + v.X*5, Y: p.Y + v.Y*5},
	}}
	updateProgress(&s, &s.Cars[0], Point{p.X - v.X*4, p.Y - v.Y*4}, 1020)
	updateProgress(&s, &s.Cars[1], Point{p.X - v.X, p.Y - v.Y}, 1020)
	rank(&s)
	if s.Cars[1].FinishAt >= s.Cars[0].FinishAt || s.Cars[1].Rank != 1 || s.Cars[0].Rank != 2 {
		t.Fatal("same-tick finish order lost", s.Cars)
	}
}

// Failure scenario before explicit tied-result handling: equal classified
// finishes for every seat must be a platform Draw, never multiple winners.
func TestAllCarsTiedUsesDraw(t *testing.T) {
	s := State{Cars: []Car{{Seat: 0, Gate: 1, FinishAt: 1000, Progress: 500}, {Seat: 1, Gate: 1, FinishAt: 1000, Progress: 500}}, ServerAt: 1000, MaxRaceAt: 8000}
	s.finish()
	if !s.Result.Draw || len(s.Result.Winners) != 0 || s.Result.Ranks[0] != 1 || s.Result.Ranks[1] != 1 {
		t.Fatal("all-tied race was rated as shared victory", s.Result)
	}
}

// Handbrake is a braking control as well as an oversteer aid; holding Space
// on a straight must slow down, while the service brake remains stronger.
func TestHandbrakeSlowsStraightCar(t *testing.T) {
	track := Track{Width: 1000, Height: 1000, RoadWidth: 180, Points: []Point{{100, 100}, {900, 100}, {900, 900}, {100, 900}}}
	coast := Car{X: 400, Y: 100, VX: 220, Speed: 220, LastInputAt: 1000}
	hand, service := coast, coast
	hand.Drift = true
	service.Brake = true
	for at := int64(1020); at <= 1600; at += 20 {
		stepCar(&coast, track, 20, at)
		stepCar(&hand, track, 20, at)
		stepCar(&service, track, 20, at)
	}
	if hand.Speed >= coast.Speed*.7 || service.Speed >= hand.Speed {
		t.Fatalf("handbrake is not an intermediate braking force: coast %.2f hand %.2f service %.2f", coast.Speed, hand.Speed, service.Speed)
	}
}

// Automatic throttle must not keep pushing the car while the brake is held.
func TestServiceBrakeOverridesAutomaticThrottle(t *testing.T) {
	track := Track{Width: 1000, Height: 1000, RoadWidth: 180, Points: []Point{{100, 100}, {900, 100}, {900, 900}, {100, 900}}}
	car := Car{X: 400, Y: 100, Throttle: 1, Brake: true, LastInputAt: 1000}
	stepCar(&car, track, 20, 1020)
	if car.Speed != 0 || car.X != 400 {
		t.Fatalf("brake accelerates a stopped car: %+v", car)
	}
}
