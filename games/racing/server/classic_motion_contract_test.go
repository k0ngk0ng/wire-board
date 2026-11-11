package racing

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

type classicFixtureSegment struct {
	Seconds float64            `json:"seconds"`
	Input   ClassicMotionInput `json:"input"`
	Surface ClassicRoadSurface `json:"surface"`
}
type classicFixture struct {
	Name     string                  `json:"name"`
	Vehicle  string                  `json:"vehicle"`
	Initial  ClassicMotionState      `json:"initial"`
	Segments []classicFixtureSegment `json:"segments"`
}
type classicTrajectory struct {
	Name   string               `json:"name"`
	States []ClassicMotionState `json:"states"`
}

func TestClassicMotionContract(t *testing.T) {
	for _, id := range []string{"", "missing", "__proto__", "constructor"} {
		if ClassicVehicle(id).ID != "p911" {
			t.Errorf("unknown vehicle did not default safely: %s", id)
		}
	}
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
	fixturePath := filepath.Join(root, "games/racing/tests/classic-motion-scenarios.json")
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []classicFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	byName := map[string][]ClassicMotionState{}
	trajectories := []classicTrajectory{}
	for _, fixture := range fixtures {
		state := fixture.Initial
		states := []ClassicMotionState{state}
		vehicle := ClassicVehicle(fixture.Vehicle)
		for _, segment := range fixture.Segments {
			for i := 0; i < int(math.Round(segment.Seconds/.02)); i++ {
				state = StepClassicMotion(state, segment.Input, vehicle, segment.Surface, .02)
				if !classicStateFinite(state) {
					t.Fatalf("%s step %d is not finite: %+v", fixture.Name, len(states), state)
				}
				states = append(states, state)
			}
		}
		byName[fixture.Name] = states
		trajectories = append(trajectories, classicTrajectory{Name: fixture.Name, States: states})
	}
	last := func(name string) ClassicMotionState { a := byName[name]; return a[len(a)-1] }
	speed := func(s ClassicMotionState) float64 { return math.Hypot(s.VX, s.VZ) }
	metrics := map[string]float64{}
	for _, id := range []string{"p911", "fm50"} {
		v := speed(last(id + "-straight"))
		if v < 60 || v > ClassicVehicle(id).MaxSpeed*1.025 {
			t.Errorf("%s straight speed %.3f m/s outside frozen limits", id, v)
		}
		metrics[id+"StraightSpeed"] = v
		a := byName[id+"-brake"]
		if speed(a[len(a)-1]) > .05 {
			t.Errorf("%s did not stop: %+v", id, a[len(a)-1])
		}
		if a[len(a)-1].X < 35 || a[len(a)-1].X > 90 {
			t.Errorf("%s braking distance %.3f m outside 35..90", id, a[len(a)-1].X)
		}
		for i := 1; i < len(a); i++ {
			if speed(a[i-1])-speed(a[i]) > .35 {
				t.Fatalf("%s abrupt braking step %d", id, i)
			}
			if a[i].VX < -.000001 {
				t.Fatalf("%s protection brake reversed at %d", id, i)
			}
		}
		metrics[id+"BrakingDistance"] = a[len(a)-1].X
		shifted := false
		straight := byName[id+"-straight"]
		for i := 1; i < len(straight); i++ {
			if straight[i].Gear > straight[i-1].Gear && straight[i-1].Gear > 0 {
				shifted = true
				j := i + 5
				if j >= len(straight) {
					j = len(straight) - 1
				}
				if straight[j].RPM >= straight[i-1].RPM-50 {
					t.Errorf("%s upshift failed to lower RPM", id)
				}
			}
		}
		if !shifted {
			t.Errorf("%s never changed forward gears", id)
		}
	}
	if speed(last("fm50-straight")) <= speed(last("p911-straight"))+2 {
		t.Error("two original cars have no meaningful high-speed difference")
	}
	for _, id := range []string{"ldia", "mcf1", "ccor"} {
		p := ClassicVehicle(id)
		if p.ID != id || p.ReverseVelocityToRPM <= 0 || p.ReverseEfficiency <= 0 {
			t.Errorf("missing original parameter set for %s", id)
		}
		for _, s := range byName[id+"-straight"] {
			if s.Gear > p.GearCount {
				t.Errorf("%s selected unavailable automatic gear %d", id, s.Gear)
				break
			}
		}
		v := speed(last(id + "-straight"))
		if v < 60 || v > p.MaxSpeed*1.025 {
			t.Errorf("%s straight speed %.3f outside CARP limits", id, v)
		}
		metrics[id+"StraightSpeed"] = v
	}
	if ClassicVehicle("ccor").GearCount != 4 || ClassicVehicle("ldia").GearCount != 5 || ClassicVehicle("mcf1").GearCount != 6 {
		t.Error("original automatic gear counts were discarded")
	}
	if ClassicVehicle("ldia").FrontDriveRatio != .5 {
		t.Error("Diablo original all-wheel-drive data was discarded")
	}
	if speed(last("ldia-straight")) <= 90 {
		t.Error("new vehicle incorrectly retains the old 900 decimetres/s cap")
	}
	if math.Abs(byName["p911-steering"][5].Steering-.4) > .000001 || math.Abs(byName["fm50-steering"][5].Steering-.3) > .000001 {
		t.Error("CARP turn-in rates are not preserved")
	}
	if math.Abs(last("p911-steering").Steering) > .000001 || math.Abs(last("fm50-steering").Steering) > .000001 {
		t.Error("released steering did not return to center")
	}
	low := last("p911-low-turn")
	if math.Abs(low.Z) < 1 || math.Abs(low.Angle) < .3 {
		t.Errorf("low-speed turn did not turn: %+v", low)
	}
	for _, s := range byName["p911-high-turn"] {
		if math.Abs(s.YawRate) > 1.5 {
			t.Errorf("unbounded high-speed yaw: %g", s.YawRate)
			break
		}
	}
	maxSlip := func(a []ClassicMotionState) float64 {
		result := 0.
		for _, s := range a {
			u := s.VX*math.Cos(s.Angle) + s.VZ*math.Sin(s.Angle)
			v := -s.VX*math.Sin(s.Angle) + s.VZ*math.Cos(s.Angle)
			if math.Abs(u) > 2 {
				result = math.Max(result, math.Abs(math.Atan2(v, math.Abs(u))))
			}
		}
		return result
	}
	if maxSlip(byName["p911-handbrake"]) <= maxSlip(byName["p911-corner"])+.015 {
		t.Error("handbrake did not produce additional slip")
	}
	hb := last("p911-handbrake")
	u := hb.VX*math.Cos(hb.Angle) + hb.VZ*math.Sin(hb.Angle)
	v := -hb.VX*math.Sin(hb.Angle) + hb.VZ*math.Cos(hb.Angle)
	if math.Abs(math.Atan2(v, math.Max(math.Abs(u), .5))) > .25 || math.Abs(hb.YawRate) > .4 {
		t.Errorf("handbrake did not recover: %+v", hb)
	}
	shoulder := byName["p911-shoulder"]
	if speed(shoulder[50])-speed(shoulder[51]) > .5 {
		t.Error("road shoulder clipped speed")
	}
	resistance := byName["p911-shoulder-resistance"]
	if speed(resistance[50])-speed(resistance[51]) > .5 {
		t.Error("continuous shoulder resistance clipped speed")
	}
	if speed(last("p911-shoulder"))-speed(last("p911-shoulder-resistance")) < 3 {
		t.Error("off-road resistance did not continuously slow the vehicle")
	}
	if s := last("p911-grass-start"); s.X < 2 || speed(s) < 1 {
		t.Errorf("low-speed grass resistance prevents ordinary driving: %+v", s)
	}
	if s := last("p911-grass-reverse"); s.Gear != -1 || s.X > -1 || speed(s) < 1 {
		t.Errorf("low-speed grass resistance prevents ordinary reverse recovery: %+v", s)
	}
	if r := last("p911-reverse"); r.Gear != -1 || r.X >= -1 || r.VX >= 0 {
		t.Errorf("held low-speed brake did not reverse: %+v", r)
	}
	if r := last("p911-protection"); r.Gear == -1 || math.Abs(r.X) > .01 {
		t.Errorf("protection brake drove the car: %+v", r)
	}
	if r := last("p911-reverse-forward"); r.Gear < 1 || r.VX <= 1 {
		t.Errorf("reverse could not recover forward: %+v", r)
	}
	if !(speed(last("p911-uphill")) < speed(last("p911-flat")) && speed(last("p911-flat")) < speed(last("p911-downhill"))) {
		t.Error("grade does not affect speed in the correct direction")
	}
	for _, dt := range []float64{.02, 0, -1, math.NaN(), math.Inf(1)} {
		s := ClassicMotionState{X: math.NaN(), VX: math.Inf(1), YawRate: math.NaN(), Steering: math.Inf(-1), RPM: math.NaN(), Gear: 999}
		s = StepClassicMotion(s, ClassicMotionInput{Steer: math.NaN(), Throttle: math.Inf(1), BrakeAmount: math.NaN()}, ClassicVehicleParams{}, ClassicRoadSurface{Grip: math.NaN(), Grade: math.Inf(1)}, dt)
		if !classicStateFinite(s) {
			t.Fatalf("invalid input propagated for dt %v: %+v", dt, s)
		}
	}
	badParams := []ClassicVehicleParams{ClassicVehicle("p911"), ClassicVehicle("p911"), ClassicVehicle("p911"), ClassicVehicle("p911")}
	badParams[0].Mass = math.NaN()
	badParams[1].LateralGrip = math.NaN()
	badParams[2].RedlineRPM = math.Inf(1)
	badParams[3].GearRatios[0] = math.NaN()
	for _, p := range badParams {
		if s := StepClassicMotion(ClassicMotionState{}, ClassicMotionInput{Throttle: 1}, p, ClassicRoadSurface{}, .02); !classicStateFinite(s) {
			t.Errorf("invalid vehicle data propagated: %+v", s)
		}
	}
	artifactDir := filepath.Join(root, ".local/racing-nfs-motion")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(trajectories)
	goPath := filepath.Join(artifactDir, "go-trajectories.json")
	if err := os.WriteFile(goPath, b, 0644); err != nil {
		t.Fatal(err)
	}
	metricsBytes, _ := json.MarshalIndent(metrics, "", "  ")
	if err := os.WriteFile(filepath.Join(artifactDir, "motion-metrics.json"), metricsBytes, 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "--experimental-strip-types", filepath.Join(root, "games/racing/tests/classic-motion-parity.mjs"), fixturePath, goPath, filepath.Join(artifactDir, "parity-report.json"))
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Go/TS trajectory verification failed: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}

func classicStateFinite(s ClassicMotionState) bool {
	values := []float64{s.X, s.Z, s.VX, s.VZ, s.Angle, s.YawRate, s.Steering, s.LongAccel, s.LateralAccel, s.RPM, s.ShiftTimer, s.Throttle, s.Brake, s.Handbrake, s.ReverseTimer}
	for _, n := range values {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return false
		}
	}
	return true
}
