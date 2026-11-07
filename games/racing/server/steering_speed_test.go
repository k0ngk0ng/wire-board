package racing

// Failure cases BEFORE implementation: high-speed yaw grows instead of falling;
// a stopped car pivots; left/right or analogue input lose symmetry; drift
// restores the excessive high-speed yaw; prediction disagrees with authority.
import (
	"math"
	"testing"
)

func TestSteeringAuthorityFallsAtRoadSpeeds(t *testing.T) {
	track := Track{Width: 5000, Height: 5000, RoadWidth: 10000, Points: []Point{{100, 100}, {4900, 100}, {4900, 4900}, {100, 4900}}}
	step := func(speed, steer float64, drift bool) float64 {
		c := Car{X: 1000, Y: 1000, VX: speed, Speed: speed, Steer: steer, Drift: drift, LastInputAt: 1000}
		stepCar(&c, track, 20, 1000)
		return c.Angle
	}
	previous := math.Inf(1)
	for _, speed := range []float64{65, 130, 260, 520, 900} {
		yaw := step(speed, 1, false)
		if !(yaw > 0 && yaw < previous) {
			t.Fatalf("speed %v: yaw %v must fall below %v", speed, yaw, previous)
		}
		if math.Abs(step(speed, -1, false)+yaw) > 1e-12 {
			t.Fatal("asymmetric steering")
		}
		if math.Abs(step(speed, .25, false)-yaw*.25) > 1e-12 {
			t.Fatal("analogue magnitude lost")
		}
		previous = yaw
	}
	if step(0, 1, false) != 0 {
		t.Fatal("stationary pivot")
	}
	if step(900, 1, true) >= step(130, 1, false)*.3 {
		t.Fatal("high-speed handbrake remains unrealistically sharp")
	}
}
