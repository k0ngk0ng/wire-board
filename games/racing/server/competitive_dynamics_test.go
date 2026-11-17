package racing

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestClassicWallScrapeLosesTangentialSpeed(t *testing.T) {
	track := Track{Geometry: "wall-scrape-test", Barriers: []RoadBarrier{{A: Point{-10000, 0}, B: Point{10000, 0}, Bottom: -10, Top: 100}}}
	classicRules.byGeometry[track.Geometry] = buildClassicTrackInfo(track)
	t.Cleanup(func() { delete(classicRules.byGeometry, track.Geometry) })
	initial := ClassicMotionState{X: 0, Z: .8, VX: 70, VZ: -8, Angle: -.1}
	next := CollideClassicRoad(track, initial, "p911", 0)
	if next.VX > initial.VX*.92 || math.Abs(next.VZ) >= 8 {
		t.Fatalf("wall scrape preserved racing speed: %+v", next)
	}
	if math.Abs(next.YawRate) < .02 {
		t.Fatal("offset wall strike produced no yaw impulse")
	}
}

func TestClassicOffsetCarImpactDisruptsBothCars(t *testing.T) {
	track, _ := classicTrackByID("france")
	p := track.Points[20]
	cars := []Car{
		{Seat: 0, Vehicle: "mcf1", X: p.X, Y: p.Y, VX: 800, Angle: 0, RouteIndex: 20},
		{Seat: 1, Vehicle: "mcf1", X: p.X + 40, Y: p.Y + 6, VX: 120, Angle: 0, RouteIndex: 20},
	}
	for i := range cars {
		classicPose(&cars[i], track)
		classicMotionToCar(&cars[i], classicMotionFromCar(cars[i]))
	}
	s := State{Track: track, Cars: cars}
	collideClassicCars(&s, 1000)
	if s.Cars[0].Speed > 600 || s.Cars[1].Speed < 250 {
		t.Fatalf("impact did not exchange speed: %+v", s.Cars)
	}
	if math.Abs(s.Cars[0].Motion.YawRate)+math.Abs(s.Cars[1].Motion.YawRate) < .15 {
		t.Fatal("high-speed offset impact left both cars pointing straight")
	}
	if len(s.Events) != 2 || s.Events[0].Seat == s.Events[1].Seat {
		t.Fatal("both drivers must receive one collision event")
	}
	count := len(s.Events)
	collideClassicCars(&s, 1020)
	if len(s.Events) != count {
		t.Fatal("separated cars repeated their impact")
	}
}

func TestClassicFastCrossingCarsCannotTunnel(t *testing.T) {
	track, _ := classicTrackByID("france")
	p := track.Points[20]
	before := []Car{
		{Seat: 0, Vehicle: "mcf1", X: p.X, Y: p.Y - 15, VY: 1500, Angle: 0, RouteIndex: 20},
		{Seat: 1, Vehicle: "mcf1", X: p.X, Y: p.Y + 15, VY: -1500, Angle: 0, RouteIndex: 20},
	}
	for i := range before {
		classicPose(&before[i], track)
	}
	after := append([]Car{}, before...)
	after[0].Y += 30
	after[1].Y -= 30
	s := State{Track: track, Cars: after}
	collideClassicCars(&s, 1000, before)
	if s.Cars[0].VY > 500 || s.Cars[1].VY < -500 || s.Cars[0].Y >= s.Cars[1].Y {
		t.Fatal("fast crossing cars passed through each other", s.Cars)
	}
	for i := range before {
		before[i].Height += float64(i) * 100
	}
	s.Cars = append([]Car{}, before...)
	s.Cars[0].Y += 30
	s.Cars[1].Y -= 30
	collideClassicCars(&s, 1020, before)
	if s.Cars[0].VY != 1500 || s.Cars[1].VY != -1500 {
		t.Fatal("vertically separated cars collided")
	}
}

func TestClassicFastWallStrikeCannotTunnel(t *testing.T) {
	track := Track{Geometry: "wall-sweep-test", Barriers: []RoadBarrier{{A: Point{-10000, 0}, B: Point{10000, 0}, Bottom: -10, Top: 100}}}
	classicRules.byGeometry[track.Geometry] = buildClassicTrackInfo(track)
	t.Cleanup(func() { delete(classicRules.byGeometry, track.Geometry) })
	before := ClassicMotionState{X: 0, Z: 2, VZ: -150, Angle: 0}
	after := before
	after.Z = -1
	next := SweepClassicRoad(track, before, after, "p911", 0, .02)
	if next.Z < .94 || next.VZ < 0 {
		t.Fatalf("fast strike crossed thin wall: %+v", next)
	}
}

func TestClassicAirborneRouteWindingKeepsLap(t *testing.T) {
	track, _ := classicTrackByID("france")
	info := classicTrackInfoFor(track)
	last := len(track.Points) - 1
	p := track.Points[last]
	c := Car{Vehicle: "mcf1", X: p.X, Y: p.Y, Height: track.Road[last].Height + 50, Airborne: true, RouteIndex: last, Gate: last, Checkpoint: 0}
	c.LapTrack = &classicLapTracker{Version: 1, Arc: info.arcs[last], X: c.X, Y: c.Y, Turns: 0}
	before := Point{c.X, c.Y}
	c.X, c.Y, c.RouteIndex = track.Points[0].X, track.Points[0].Y, 0
	s := State{Track: track, Options: Options{Laps: 3}, Cars: []Car{c}}
	updateClassicProgress(&s, &s.Cars[0], before, 1000)
	if s.Cars[0].Lap != 1 {
		t.Fatal("a continuous airborne crossing erased the circuit", s.Cars[0].LapTrack)
	}
}

func TestClassicFastCrossingOverlapCannotSwapSides(t *testing.T) {
	track, _ := classicTrackByID("france")
	p := track.Points[20]
	before := []Car{
		{Seat: 0, Vehicle: "mcf1", X: p.X, Y: p.Y - 12, VY: 1000, RouteIndex: 20},
		{Seat: 1, Vehicle: "mcf1", X: p.X, Y: p.Y + 12, VY: -1000, RouteIndex: 20},
	}
	for i := range before {
		classicPose(&before[i], track)
	}
	s := State{Track: track, Cars: append([]Car{}, before...)}
	s.Cars[0].Y += 20
	s.Cars[1].Y -= 20
	collideClassicCars(&s, 1000, before)
	if s.Cars[0].Y >= s.Cars[1].Y || s.Cars[0].VY > 100 || s.Cars[1].VY < -100 {
		t.Fatal("overlapping fast cars swapped sides without an impulse", s.Cars)
	}
}

func TestClassicTrafficLaneKeepsParallelCarsApart(t *testing.T) {
	track := Track{Geometry: "traffic-lane-test"}
	for i := 0; i < 64; i++ {
		a := float64(i) * math.Pi / 32
		track.Points = append(track.Points, Point{X: math.Cos(a) * 10000, Y: math.Sin(a) * 10000})
		track.Road = append(track.Road, RoadSample{Left: 70, Right: 70, NY: 1, RX: math.Cos(a), RZ: math.Sin(a)})
	}
	classicRules.byGeometry[track.Geometry] = buildClassicTrackInfo(track)
	t.Cleanup(func() { delete(classicRules.byGeometry, track.Geometry) })
	left := Car{Seat: 0, Vehicle: "mcf1", X: 9978, Y: 0, Angle: math.Pi / 2, Speed: 300, VY: 300}
	right := left
	right.Seat, right.X = 1, 10022
	cars := []Car{left, right}
	lane0 := classicTrafficLane(left, track, ClassicRoadAt(track, left.X, left.Y, 0, 0), 0, nil, cars)
	lane1 := classicTrafficLane(right, track, ClassicRoadAt(track, right.X, right.Y, 0, 0), 0, nil, cars)
	if lane0 > -22 || lane1 < 22 {
		t.Fatal("parallel drivers aimed at the same centre line", lane0, lane1)
	}
	if lane := classicTrafficLane(left, track, ClassicRoadAt(track, left.X, left.Y, 0, 0), 0, nil, []Car{left}); lane != 0 {
		t.Fatal("an empty road did not return to the racing line", lane)
	}
}

func TestClassicBlockedPartialThrottleRecordsRecovery(t *testing.T) {
	track, _ := classicTrackByID("france")
	p := track.Points[20]
	c := Car{Vehicle: "mcf1", X: p.X, Y: p.Y, Height: track.Road[20].Height, RouteIndex: 20, Throttle: .18, LastInputAt: 1000}
	stepClassicCar(&c, track, 20, 1000, nil)
	if c.Driver == nil || c.Driver.BlockedAt == 0 {
		t.Fatal("blocked partial-throttle traffic never becomes eligible for recovery")
	}
}

func TestClassicReverseRecoveryTurnsOutOfWallWedge(t *testing.T) {
	track, _ := classicTrackByID("france")
	p, r := track.Points[206], track.Road[206]
	tv := tangent(track, 206)
	c := Car{Vehicle: "mcf1", X: p.X + r.RX*70, Y: p.Y + r.RZ*70, Height: r.Height, Angle: math.Atan2(tv.Y, tv.X) + .8, Gear: -1, RouteIndex: 206, Checkpoint: 207, Driver: &classicDriverState{BlockedAt: 1, ReverseAt: 2900}}
	s := State{Track: track, Cars: []Car{c}, ServerAt: 3000}
	data, err := classicBotAction(gameapi.Context{Actor: 0}, s)
	if err != nil {
		t.Fatal(err)
	}
	var action Action
	if err := json.Unmarshal(data, &action); err != nil {
		t.Fatal(err)
	}
	if !action.Brake || math.Abs(action.Steer) < .2 {
		t.Fatal("reverse recovery backed straight into the outside wall", string(data))
	}
}

func TestClassicRecoveryClearsAfterSlowForwardProgress(t *testing.T) {
	track, _ := classicTrackByID("france")
	p := track.Points[20]
	c := Car{Vehicle: "mcf1", X: p.X, Y: p.Y, Height: track.Road[20].Height, RouteIndex: 20, Throttle: .18, LastInputAt: 1000, VX: 20, Driver: &classicDriverState{BlockedAt: 100, OriginX: p.X - 30, OriginY: p.Y}}
	stepClassicCar(&c, track, 20, 1000, nil)
	if c.Driver.BlockedAt != 0 {
		t.Fatal("moving traffic retained a stale blocked state", c.Driver)
	}
}
