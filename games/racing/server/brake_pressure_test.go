package racing

// Before implementation: partial braking collapses to full, old saves lose
// braking, timeout retains a weak pressure, or a new input overwrites replay.
import "testing"

func TestBrakePressureAndLegacyBehavior(t *testing.T) {
	track := Track{Width: 1000, Height: 1000, RoadWidth: 180, Points: []Point{{100, 100}, {900, 100}, {900, 900}, {100, 900}}}
	base := Car{X: 400, Y: 100, VX: 220, Speed: 220, LastInputAt: 1000}
	coast, light, full, legacy := base, base, base, base
	quarter, one := .25, 1.
	light.Brake = true
	light.BrakeAmount = &quarter
	full.Brake = true
	full.BrakeAmount = &one
	legacy.Brake = true
	for at := int64(1020); at <= 1500; at += 20 {
		stepCar(&coast, track, 20, at)
		stepCar(&light, track, 20, at)
		stepCar(&full, track, 20, at)
		stepCar(&legacy, track, 20, at)
	}
	if !(coast.Speed > light.Speed && light.Speed > full.Speed) || full.Speed != legacy.Speed {
		t.Fatalf("pressure/legacy brake mismatch: %v %v %v %v", coast.Speed, light.Speed, full.Speed, legacy.Speed)
	}
	light, legacy = base, base
	light.Brake = true
	light.BrakeAmount = &quarter
	stepCar(&light, track, 20, 1000+lostInputMS+1)
	stepCar(&legacy, track, 20, 1000+lostInputMS+1)
	if light.Speed != legacy.Speed {
		t.Fatal("lost-input braking retained weak pressure")
	}
}
