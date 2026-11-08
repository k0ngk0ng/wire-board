package stoneage

// Failure cases listed before implementation in ../tests/FAILURE_SCENARIOS.md:
// wrong classic stat formula, melee/throwing ride coefficients, reversed
// elements, incorrect native guard distribution, overflow and zero divisors.
import (
	"math"
	"testing"
)

func TestClassicStatAndRideCoefficients(t *testing.T) {
	u := unitFromBuild([4]int{100, 100, 100, 100})
	if u.MaxHP != 700 || u.Attack != 125 || u.Defense != 125 || u.Speed != 100 {
		t.Fatal(u)
	}
	body := Unit{Attack: 100, Defense: 80, Speed: 60}
	mount := Unit{HP: 500, Attack: 200, Defense: 120, Speed: 90}
	a, d, s := rideStats(body, mount, false)
	if a != 240 || d != 140 || s != 84 {
		t.Fatal(a, d, s)
	}
	a, d, s = rideStats(body, mount, true)
	if a != 180 || d != 140 || s != 66 {
		t.Fatal(a, d, s)
	}
	mount.HP = 0
	a, d, s = rideStats(body, mount, false)
	if a != 100 || d != 80 || s != 60 {
		t.Fatal("fallen ride still contributes", a, d, s)
	}
}
func TestClassicElementAndGuardDistributions(t *testing.T) {
	earth, water, fire, wind := [4]int{100, 0, 0, 0}, [4]int{0, 100, 0, 0}, [4]int{0, 0, 100, 0}, [4]int{0, 0, 0, 100}
	for _, pair := range [][2][4]int{{earth, water}, {water, fire}, {fire, wind}, {wind, earth}} {
		if math.Abs(elementMultiplier(pair[0], pair[1])-1.5) > .00001 {
			t.Fatal("advantage")
		}
		if math.Abs(elementMultiplier(pair[1], pair[0])-.6) > .00001 {
			t.Fatal("disadvantage")
		}
	}
	if elementMultiplier(earth, earth) != 1 {
		t.Fatal("same element")
	}
	counts := map[int]int{}
	for r := 1; r <= 100; r++ {
		counts[guardDamage(100, r)]++
	}
	for damage, want := range map[int]int{0: 25, 10: 25, 20: 20, 30: 15, 40: 10, 50: 5} {
		if counts[damage] != want {
			t.Fatal(counts)
		}
	}
}
func TestClassicDamageBounds(t *testing.T) {
	for _, tc := range []struct {
		attack, defense int
		lo, hi          int
	}{{100, 70, 53, 67}, {100, 100, 0, 7}, {100, 120, 0, 1}, {0, 0, 0, 0}} {
		for r := 0; r < 10000; r += 31 {
			d := physicalBase(float64(tc.attack), float64(tc.defense), r)
			if d < tc.lo || d > tc.hi {
				t.Fatal(tc, r, d)
			}
		}
	}
}
