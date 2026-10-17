package engine

import "testing"

func coldOpeningForTest(t *testing.T) OpeningState {
	t.Helper()
	cfg := DefaultOriginalMatchConfiguration()
	cfg.Seed = 0x12345678
	s, e := NewOriginalMatch(cfg)
	if e != nil {
		t.Fatal(e)
	}
	court, e := InitializeOriginalCourt(s)
	if e != nil {
		t.Fatal(e)
	}
	start, e := StartOriginalCourt(court.State)
	if e != nil {
		t.Fatal(e)
	}
	s = start.State
	for n := 0; n < 33; n++ {
		s.Transition.CameraScroll[0], s.Transition.CameraScroll[1] = s.Transition.CameraScroll[2], s.Transition.CameraScroll[3]
		s.Projection = resetOriginalWorldDrawList(s.Projection)
		s.Transition.Random, _ = NextRandom(s.Transition.Random)
		next, e := StepOriginalRefereeOpening(s)
		if e != nil {
			t.Fatal(e)
		}
		s = next.State
	}
	period, e := InitializeOpeningPeriod(s)
	if e != nil {
		t.Fatal(e)
	}
	return period.State
}
func TestConfiguredOpeningEntersRealWorld(t *testing.T) {
	s := coldOpeningForTest(t)
	world, ctx, e := WorldFromOriginalOpening(s, OpeningWorldOptions{LogicFrame: 35, HumanTactics: [2]uint8{0, 0}})
	if e != nil {
		t.Fatal(e)
	}
	if world.Started || world.Updates != 0 || world.Preparation.Actions.Ball.Body.Position.Height != 73*Pixel || ctx.Actions.Controls != ([4]uint8{0, 128, 128, 128}) || ctx.AttackGrades != ([4]uint8{3, 5, 3, 3}) {
		t.Fatal(world, ctx)
	}
	if ctx.PursuitAlias74 == nil || *ctx.PursuitAlias74 != (PursuitAlias74{}) {
		t.Fatal("cold-cleared pursuit alias must be explicit", ctx.PursuitAlias74)
	}
	for n := 0; n < 120; n++ {
		next, e := StepWorldLoop(world, WorldLoopInput{}, ctx)
		if e != nil {
			t.Fatalf("cold configured world update %d: %v", n, e)
		}
		world = next.State
	}
	if world.Updates != 120 {
		t.Fatal(world.Updates)
	}
	t.Logf("configuration + seed + original opening advanced %d real world updates; no fixture input", world.Updates)
}
func TestOpeningWorldRejectsPrematureHandoff(t *testing.T) {
	s, e := NewOriginalMatch(DefaultOriginalMatchConfiguration())
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = WorldFromOriginalOpening(s, OpeningWorldOptions{}); e == nil {
		t.Fatal("unopened court accepted as playable world")
	}
}
