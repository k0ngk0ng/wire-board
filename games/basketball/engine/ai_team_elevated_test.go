package engine

import "testing"

// The complete preparation fixture first reaches this previously rejected
// branch at original frame 7614. Its holder is grounded at height zero, so the
// original $9D54/$9D59 bypasses elevated role selection and decrements normally.
func TestOriginalTeamMode11GroundedHolder(t *testing.T) {
	s := TeamAIState{Teams: [2]TeamStrategyState{{Mode: 0}, {Mode: 11, Entered: true, Counter: 48}}}
	c := TeamAIContext{Holder: 0, PreviousHolder: 0, CourtID: 1}
	got, err := StepOriginalTeamAI(s, c)
	want := s
	want.Teams[1].Counter--
	if err != nil || got.State != want {
		t.Fatal(got, want, err)
	}
}

func TestOriginalTeamMode11ElevatedGateAndRoles(t *testing.T) {
	s := TeamAIState{Teams: [2]TeamStrategyState{{Mode: 0}, {Mode: 11, Entered: true, Counter: 48}}}
	c := TeamAIContext{Holder: 0, PreviousHolder: 0, CourtID: 1}
	c.Objects[0] = Position{X: 100 * Pixel, Height: 64 * Pixel}
	c.Objects[2].X, c.Objects[3].X = 90*Pixel, 300*Pixel
	for i := range c.Hoops {
		c.Hoops[i].Action = HoopReady
		c.HoopEntered[i] = true
	}
	got, err := StepOriginalTeamAI(s, c)
	want := s
	want.Players[2].Mode = 28
	want.Players[3].Mode = 29
	if err != nil || got.State != want {
		t.Fatal("elevated exact-ready roles", got, want, err)
	}
	got, err = StepOriginalTeamAI(want, c)
	if err != nil || got.State != want {
		t.Fatal("existing complementary roles must preserve counter", got, err)
	}
	for _, alter := range []func(*TeamAIContext){
		func(c *TeamAIContext) { c.HoopEntered[0] = false },
		func(c *TeamAIContext) { c.Hoops[0].Action = HoopScored },
		func(c *TeamAIContext) { c.Objects[0].Height = -Pixel },
		func(c *TeamAIContext) { c.Objects[0].Height = 0 },
		func(c *TeamAIContext) { c.CourtID = 0 },
	} {
		v := c
		alter(&v)
		got, err = StepOriginalTeamAI(s, v)
		want = s
		want.Teams[1].Counter--
		if err != nil || got.State != want {
			t.Fatal("elevated gate bypass", got, want, err)
		}
	}
}
