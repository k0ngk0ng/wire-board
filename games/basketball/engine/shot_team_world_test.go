package engine

import (
	"reflect"
	"slices"
	"testing"
)

// World failures: motion loads the old owner/exclusion; entry audio moves
// behind court audio; landing shake is dropped or replaced by old camera;
// each new world tick is reinitialized from oracle snapshots.
func TestWorldTeamSpecialPrepareBeforeBoundMotion(t *testing.T) {
	runtime := courtTwoPlayingForTest(t)
	r := WorldActionStep{State: runtime.Playing.World.Preparation.Actions}
	c := runtime.Playing.Context.Actions
	r.State.Holder = 0
	r.State.ExcludedCatcher = 2
	r.State.Actors[0].Player.Action = ActionShoot
	r.State.Ball = Ball{Action: 15, Body: Body{Position: Position{X: 280 * Pixel, Depth: 40 * Pixel, Height: 60 * Pixel}}}
	court, e := originalWorldMotionCourt(&r, c, 6)
	if e != nil {
		t.Fatal(e)
	}
	load := court.originalMotion.load
	called := false
	court.originalMotion.load = func() CourtFloorState {
		called = true
		if r.State.Holder != 6 || r.State.ExcludedCatcher != -1 {
			t.Fatal("court loaded pre-release ownership", r.State.Holder, r.State.ExcludedCatcher)
		}
		if !slices.Equal(r.SoundIDs, []uint8{30}) {
			t.Fatal("entry sound not queued before court motion", r.SoundIDs)
		}
		return load()
	}
	if e = advanceWorldTeamSpecialBall(&r, c, court); e != nil {
		t.Fatal(e)
	}
	if !called {
		t.Fatal("original motion binding was skipped")
	}
}

func TestWorldTeamSpecialLandingAudioAndShake(t *testing.T) {
	runtime := courtTwoPlayingForTest(t)
	r := WorldActionStep{State: runtime.Playing.World.Preparation.Actions}
	c := runtime.Playing.Context.Actions
	r.State.Holder = 6
	r.State.Ball = Ball{Action: 15, Entered: true, Deflection: 128, BouncesRemaining: 13, Body: Body{Position: Position{X: 280 * Pixel, Depth: 76 * Pixel}}}
	court, e := originalWorldMotionCourt(&r, c, 6)
	if e != nil {
		t.Fatal(e)
	}
	if e = advanceWorldTeamSpecialBall(&r, c, court); e != nil {
		t.Fatal(e)
	}
	if !slices.Equal(r.SoundIDs, []uint8{21, 10}) {
		t.Fatal("court/landing sounds reordered", r.SoundIDs)
	}
	if !r.ChangeCameraShake || r.CameraShake != 11 || r.State.Ball.Action != 12 {
		t.Fatal("landing did not publish shake and next action", r.ChangeCameraShake, r.CameraShake, r.State.Ball.Action)
	}
}

func TestWorldLoopTeamSpecialShakeSurvivesProjection(t *testing.T) {
	fixture := originalWorldLoopFixture(t)
	s := fixture.Initial
	c := fixture.context(t)
	c.Actions.Controls = [4]uint8{}
	s.Camera.Shake = 5
	s.Preparation.Actions.Holder = 6
	s.Preparation.Actions.Ball = Ball{Action: 15, Entered: true, BouncesRemaining: 13, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}
	got, e := StepWorldLoop(s, WorldLoopInput{}, c)
	if e != nil {
		t.Fatal(e)
	}
	if got.State.Camera.Shake != 11 {
		t.Fatal("new ball shake lost after action/projection", got.State.Camera.Shake)
	}
	if got.State.Preparation.Actions.Ball.Action != 12 {
		t.Fatal("world did not dispatch special landing")
	}
}

func TestWorldTeamSpecialContinuousOriginalChains(t *testing.T) {
	_, chains := teamSpecialOracles(t)
	for _, chain := range chains {
		s := chain.Records[0].Before
		shake := chain.Records[0].ShakeBefore
		c := chain.Records[0].context(t)
		for i, row := range chain.Records {
			r := WorldActionStep{State: s}
			if e := advanceWorldTeamSpecialBall(&r, c, NewYorkBallCourt()); e != nil {
				t.Fatalf("action%d left%v tick%d: %v", chain.Action, chain.Left, i, e)
			}
			s = r.State
			if r.ChangeCameraShake {
				shake = r.CameraShake
			}
			if s != row.After || shake != row.ShakeAfter {
				t.Fatalf("action%d left%v tick%d: %s", chain.Action, chain.Left, i, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(row.After), "world"))
			}
			if !slices.Equal(r.SoundIDs, row.Sounds) {
				t.Fatal("world chain sounds differ", r.SoundIDs, row.Sounds)
			}
		}
	}
}

func TestWorldTeamSpecialRejectsUnknownReplacedTurnSelector(t *testing.T) {
	// BDD2 indexes the CURRENT post-motion action. A court device can replace
	// action14 with4, making that original lookup read outside its special-shot
	// table. Until that context is recovered, never silently use action14's32.
	runtime := courtTwoPlayingForTest(t)
	s := runtime.Playing.World.Preparation.Actions
	c := runtime.Playing.Context.Actions
	s.Holder = 6
	s.Ball = Ball{Action: 14, Body: Body{Position: Position{X: 280 * Pixel, Depth: 76 * Pixel}, Velocity: Velocity{Height: -1664}, Gravity: -192}}
	got, e := StepWorldActions(s, c)
	if e == nil {
		t.Fatal("terrain-replaced special turn lookup was silently approximated")
	}
	if got.State != s {
		t.Fatal("unknown turn lookup committed partial world state")
	}
}
