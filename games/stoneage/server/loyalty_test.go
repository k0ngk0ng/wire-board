package stoneage

import "testing"

// Boundary expectations are from battle.c::BATTLE_PetLoyalCheck, RAND(1,100).
// Keep literal boundary cases independent of the implementation's tier loop.
func TestClassicLoyaltyBoundaries(t *testing.T) {
	cases := []struct {
		loyalty, roll int
		want          string
	}{
		{100, 1, "normal"}, {80, 100, "normal"},
		{79, 9, "target"}, {70, 10, "normal"},
		{69, 19, "target"}, {60, 20, "normal"},
		{59, 34, "target"}, {50, 35, "normal"},
		{49, 49, "target"}, {40, 50, "normal"},
		{39, 69, "random"}, {30, 70, "normal"}, {20, 69, "random"},
		{19, 79, "owner"}, {10, 80, "enemy"},
		{9, 59, "owner"}, {0, 60, "flee"}, {0, 100, "flee"},
	}
	for _, c := range cases {
		if got := loyaltyMode(c.loyalty, c.roll); got != c.want {
			t.Fatalf("loyalty=%d roll=%d: %s want %s", c.loyalty, c.roll, got, c.want)
		}
	}
}
