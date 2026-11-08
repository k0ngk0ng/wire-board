package stoneage

import (
	"reflect"
	"testing"
)

// Failure cases are recorded in ../tests/FAILURE_SCENARIOS.md before the
// distribution change. Sample the real pool, not a duplicate random function.
func TestArenaLoyaltyPoolDistribution(t *testing.T) {
	catalog()
	ordinary, strong, low := 0, 0, 0
	ordinaryValues, highValues, lowValues := map[int]bool{}, map[int]bool{}, map[int]bool{}
	for seed := uint64(1); seed <= 4000; seed++ {
		s := State{Random: seed}
		s.makePool()
		poolLow := 0
		for _, pet := range s.Pool.Pets {
			original, ok := petByID(sourceCatalog.Pets, pet.ID)
			if !ok || pet.Weights != original.Weights || pet.Elements != original.Elements {
				t.Fatal("loyalty changed species stats", pet)
			}
			weight := 0
			for _, n := range original.Weights {
				weight += n
			}
			if weight < 100 {
				ordinary++
				ordinaryValues[pet.Loyalty] = true
				if pet.Loyalty < 80 || pet.Loyalty > 100 {
					t.Fatalf("ordinary pet %s has loyalty %d, want 80..100", pet.Name, pet.Loyalty)
				}
				continue
			}
			strong++
			if pet.Loyalty < 80 {
				low++
				poolLow++
				lowValues[pet.Loyalty] = true
				if pet.Loyalty < 20 || pet.Loyalty > 30 {
					t.Fatal("NONO loyalty must be 20..30", pet)
				}
			} else {
				highValues[pet.Loyalty] = true
				if pet.Loyalty > 100 {
					t.Fatal("loyalty above 100", pet)
				}
			}
		}
		if poolLow > 3 {
			t.Fatal("fewer than 17 reliable candidates", seed, poolLow)
		}
		for _, ride := range s.Pool.Rides {
			if ride.Loyalty != 100 {
				t.Fatal("ride loyalty changed", ride)
			}
		}
		if seed <= 10 {
			again := State{Random: seed}
			again.makePool()
			if !reflect.DeepEqual(s.Pool, again.Pool) || s.Random != again.Random {
				t.Fatal("same seed does not reproduce candidates")
			}
		}
	}
	rate := float64(low) / float64(strong)
	if ordinary == 0 || strong == 0 || rate < .15 || rate > .23 {
		t.Fatal("strong low-loyalty rate must remain near 20%, subject to the pool cap", ordinary, strong, low, rate)
	}
	if len(ordinaryValues) != 21 || len(highValues) != 21 || len(lowValues) != 11 {
		t.Fatal("missing loyalty values or wrong bounds", ordinaryValues, highValues, lowValues)
	}
	t.Logf("4000 pools: ordinary=%d, strong=%d, low=%d (%.2f%% of strong candidates)", ordinary, strong, low, rate*100)
}
