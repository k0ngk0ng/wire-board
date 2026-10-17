package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type rimBodyObservation struct {
	observedBall
	Link, Cooldown int32
}
type rimObservation struct {
	Ball                                                                                                                        observedBall
	Hoops                                                                                                                       [6]rimBodyObservation
	RNG                                                                                                                         [4]byte
	Scores                                                                                                                      [2]uint16
	Holder, Bounces, Deflection, PreviousAction, SelectedHoop, Shooter, ShooterAction, Skill, ScoreAbility, OriginX, SoundDelay int32
}

func rimFromObservation(r rimObservation) RimState {
	r.Ball.Bounces, r.Ball.Deflection = r.Bounces, r.Deflection
	s := RimState{Ball: ballFromObservation(r.Ball), Random: RandomState(binary.BigEndian.Uint32(r.RNG[:])), Scores: r.Scores, ScoreSoundDelay: uint8(r.SoundDelay)}
	for i, h := range r.Hoops {
		s.Hoops[i] = hoopFromObservation(observedHoop{observedBall: h.observedBall, Link: h.Link, Cooldown: h.Cooldown})
	}
	return s
}

func TestRimContactAgainstOriginal(t *testing.T) {
	data, err := os.ReadFile("testdata/rim.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario      string
		Frame         int
		Before, After rimObservation
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1195 {
		t.Fatal("unexpected rim call count", len(rows))
	}
	goals, misses := 0, 0
	for _, r := range rows {
		selected := int(r.Before.SelectedHoop) - 7
		if selected < 0 || selected >= 6 {
			selected = -1
		}
		ctx := RimContext{PreviousBallAction: BallAction(r.Before.PreviousAction), SelectedHoop: selected, ShotOriginX: Fixed(r.Before.OriginX) * Pixel, ShooterAction: Action(r.Before.ShooterAction & 127), ShotSkill: uint8(r.Before.Skill), ScoringBonus: r.Before.Shooter < 2 && r.Before.ScoreAbility == 27}
		got, err := ResolveRimContact(rimFromObservation(r.Before), ctx)
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		if want := rimFromObservation(r.After); got.State != want {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, rimFromObservation(r.Before), got.State, want)
		}
		if r.Before.Scores != r.After.Scores {
			goals++
			if got.Outcome != RimGoal {
				t.Fatal("score update without goal outcome", got)
			}
		} else if r.Before.RNG != r.After.RNG {
			misses++
			if got.Outcome != RimMiss && got.Outcome != RimStuck {
				t.Fatal("miss without matching outcome", got)
			}
		}
	}
	if goals != 11 || misses != 2 {
		t.Fatal(goals, misses)
	}
	t.Logf("matched %d original rim calls including %d scores and %d misses", len(rows), goals, misses)
}

func rimExample() (RimState, RimContext) {
	s := RimState{Ball: Ball{Action: BallShot, Entered: true, Body: Body{Position: Position{X: 65*Pixel + 77, Depth: 46*Pixel + 99, Height: 84 * Pixel}, Velocity: Velocity{Height: -Pixel}, Gravity: 128}, Airborne: true}}
	s.Hoops[0] = Hoop{Position: Position{X: 65 * Pixel, Depth: 43 * Pixel, Height: 80 * Pixel}, Action: HoopReady, Entered: true, CanReceive: true}
	return s, RimContext{SelectedHoop: 0, ShotOriginX: 200 * Pixel, ShotSkill: 7}
}

func TestRimUndersideAndScoreLock(t *testing.T) {
	s, ctx := rimExample()
	s.Ball.Body.Velocity.Height = Pixel
	got, err := ResolveRimContact(s, ctx)
	if err != nil || got.Outcome != RimUnderside || got.State.Scores != s.Scores || got.State.Random != s.Random || got.State.Ball.Request != (BallRequest{Action: BallDeflected, Pending: true}) || got.State.Ball.Deflection != DeflectHeight {
		t.Fatal(got, err)
	}
	s, ctx = rimExample()
	got, err = ResolveRimContact(s, ctx)
	if err != nil || got.Outcome != RimGoal || got.Points != 3 || got.ScoringSide != 1 || got.State.Scores != ([2]uint16{0, 3}) || got.State.Hoops[0].ScoreLock != 16 || got.State.Ball.Body.Position.X != 65*Pixel+77 || got.State.Ball.Body.Position.Depth != 45*Pixel+99 {
		t.Fatal(got, err)
	}
	again, err := ResolveRimContact(got.State, ctx)
	if err != nil || again.Outcome != RimNoContact || again.State != got.State {
		t.Fatal("cooldown repeated goal", again, err)
	}
}

func TestRimEdgesAndScoreBonus(t *testing.T) {
	for _, p := range []Position{{X: 57 * Pixel, Depth: 46 * Pixel, Height: 84 * Pixel}, {X: 65 * Pixel, Depth: 44 * Pixel, Height: 84 * Pixel}, {X: 65 * Pixel, Depth: 46 * Pixel, Height: 80 * Pixel}} {
		s, ctx := rimExample()
		s.Ball.Body.Position = p
		got, err := ResolveRimContact(s, ctx)
		if err != nil || got.Outcome != RimNoContact || got.State != s {
			t.Fatal("strict edge accepted", p, got, err)
		}
	}
	s, ctx := rimExample()
	ctx.ScoringBonus = true
	s.Ball.Body.Position.X = 73 * Pixel
	got, err := ResolveRimContact(s, ctx)
	if err != nil || got.Points != 5 || got.State.Scores[1] != 5 {
		t.Fatal(got, err)
	}
	s, ctx = rimExample()
	s.Hoops[0].Position.Height = 272 * Pixel
	s.Ball.Body.Position.Height = 276 * Pixel
	got, err = ResolveRimContact(s, ctx)
	if err != nil || got.Points != 4 {
		t.Fatal(got, err)
	}
}

func TestRimMissAndHangingBall(t *testing.T) {
	s, ctx := rimExample()
	ctx.ShotSkill = 0
	got, err := ResolveRimContact(s, ctx)
	if err != nil || got.Outcome != RimStuck || got.State.Ball.Action != BallStuck || got.State.Ball.Entered || got.State.Scores != s.Scores {
		t.Fatal(got, err)
	}
	ctx.SelectedHoop = -1
	s.Ball.Body.Velocity.Height = -384
	got, err = ResolveRimContact(s, ctx)
	if err != nil || got.Outcome != RimMiss || got.State.Ball.Body.Velocity.Height != 640 || got.State.Ball.Body.Velocity.X != 2*Pixel {
		t.Fatal(got, err)
	}
}

func TestRimShortestDistanceStillRejectsRandom127(t *testing.T) {
	s, ctx := rimExample()
	s.Random = RandomState(0xfcfe0000)
	ctx.ShotOriginX = 65 * Pixel
	got, err := ResolveRimContact(s, ctx)
	if err != nil || got.Outcome != RimMiss || got.State.Scores != s.Scores {
		t.Fatal("original closest-distance threshold is 127, not 128", got, err)
	}
}
