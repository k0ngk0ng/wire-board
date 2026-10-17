package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalDroppedAndKnockedBackBallExistingTrace(t *testing.T) {
	counts := map[BallAction]int{}
	for _, r := range ballRows(t) {
		b := ballFromObservation(r.Before)
		if b.Action != BallAction(11) && b.Action != BallKnockback {
			continue
		}
		var got LooseBallStep
		var err error
		if b.Action == BallKnockback {
			got, err = StepKnockedBackBall(b, NewYorkBallCourt())
		} else {
			got, err = StepDroppedBall(b, DroppedBallContext{CourtID: 1}, NewYorkBallCourt())
		}
		if err != nil || got.Ball != ballFromObservation(r.After) || got.ClearExcludedCatcher == b.Entered {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v\nerror %v", r.Scenario, r.Frame, b, got, ballFromObservation(r.After), err)
		}
		if r.Before.Holder != r.After.Holder {
			t.Fatal("unexpected owner change", r.Frame)
		}
		counts[b.Action]++
	}
	if counts[BallAction(11)] != 40 || counts[BallKnockback] != 25 {
		t.Fatal("missing original ball phases", counts)
	}
	t.Log("matched all existing 40 dropped-ball and 25 knockback-ball phases")
}

func TestOriginalLooseBallLongTrace(t *testing.T) {
	data, err := os.ReadFile("testdata/loose-ball.json")
	if err != nil {
		t.Fatal(err)
	}
	type state struct {
		observedBall
		ExcludedCatcher, Court, Random0, Random1, Random2, Random3 uint8
	}
	var rows []struct {
		Frame         int
		Before, After state
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 84 {
		t.Fatal("missing all action 7/11 phases", len(rows))
	}
	counts := map[BallAction]int{}
	for _, r := range rows {
		b := ballFromObservation(r.Before.observedBall)
		var got LooseBallStep
		if b.Action == BallKnockback {
			got, err = StepKnockedBackBall(b, NewYorkBallCourt())
		} else {
			got, err = StepDroppedBall(b, DroppedBallContext{CourtID: r.Before.Court}, NewYorkBallCourt())
		}
		excluded := r.Before.ExcludedCatcher
		if got.ClearExcludedCatcher {
			excluded = 255
		}
		if err != nil || got.Ball != ballFromObservation(r.After.observedBall) || excluded != r.After.ExcludedCatcher {
			t.Fatalf("frame %d\nbefore %+v\ngot %+v\nwant %+v\nerror %v", r.Frame, r.Before, got, r.After, err)
		}
		if r.Before.Holder != r.After.Holder || r.Before.Random0 != r.After.Random0 || r.Before.Random1 != r.After.Random1 || r.Before.Random2 != r.After.Random2 || r.Before.Random3 != r.After.Random3 {
			t.Fatal("unexpected original ownership/RNG mutation", r.Frame)
		}
		counts[b.Action]++
	}
	if counts[BallKnockback] != 25 || counts[BallAction(11)] != 59 {
		t.Fatal(counts)
	}
	t.Log("matched all 84 loose-ball phases from the 6000-frame original reference")
}

func TestDroppedBallEntryAndRetainedMotion(t *testing.T) {
	b := Ball{Action: BallAction(11), Body: Body{Position: Position{X: 200*Pixel + 17, Depth: 40*Pixel + 33, Height: 20*Pixel + 99}, Velocity: Velocity{X: -389, Depth: 51, Height: -1777}, Gravity: 777}, Animation: AnimationState{Cursor: 2, Elapsed: 1, Loops: 3}}
	got, err := StepDroppedBall(b, DroppedBallContext{CourtID: 1}, NewYorkBallCourt())
	if err != nil || !got.ClearExcludedCatcher || !got.Ball.Entered || got.Ball.Body.Velocity != (Velocity{X: -389, Depth: 51, Height: 4 * Pixel}) || got.Ball.Body.Gravity != Pixel || got.Ball.BouncesRemaining != 48 || got.Ball.Animation.Cursor != 5 || got.Ball.Animation.Elapsed != 1 || got.Ball.Pose.Frame != 2 || got.Ball.Body.Position.X != b.Body.Position.X-389 || got.Ball.Body.Position.Height != b.Body.Position.Height+4*Pixel {
		t.Fatal(got, err)
	}
	next, err := StepDroppedBall(got.Ball, DroppedBallContext{CourtID: 1}, NewYorkBallCourt())
	if err != nil || next.ClearExcludedCatcher || next.Ball.Body.Velocity.Height != 3*Pixel || next.Ball.Animation.Elapsed != 2 {
		t.Fatal(next, err)
	}
	flat := NewYorkBallCourt()
	flat.RaisedDepth = 0
	b.Body.Position.Depth = 72*Pixel + 33
	got, err = StepDroppedBall(b, DroppedBallContext{CourtID: 2}, flat)
	if err != nil || got.Ball.Body.Velocity.Depth != -Pixel || got.Ball.Body.Position.Depth != 71*Pixel+33 {
		t.Fatal(got, err)
	}
}

func TestKnockbackBallEntryAndLandingCounter(t *testing.T) {
	b := Ball{Action: BallKnockback, Body: Body{Position: Position{X: 200*Pixel + 17, Depth: 40*Pixel + 33, Height: 20*Pixel + 99}, Velocity: Velocity{X: 389, Depth: 51, Height: 1700}, Gravity: 777}}
	got, err := StepKnockedBackBall(b, NewYorkBallCourt())
	if err != nil || !got.ClearExcludedCatcher || got.Ball.Body.Velocity != (Velocity{X: -389, Height: 1444}) || got.Ball.Body.Gravity != Pixel || got.Ball.Animation.Cursor != 5 || got.Ball.Pose.Frame != 3 {
		t.Fatal(got, err)
	}
	got.Ball.Body.Position.Height = 0
	got.Ball.Body.Velocity.Height = -Pixel
	landed, err := StepKnockedBackBall(got.Ball, NewYorkBallCourt())
	if err != nil || landed.ClearExcludedCatcher || landed.Ball.Action != BallBounce || landed.Ball.Entered || landed.Ball.BouncesRemaining != 132 {
		t.Fatal(landed, err)
	}
}

func TestLooseBallInvalidActionIsAtomic(t *testing.T) {
	b := Ball{Action: BallHeld}
	got, err := StepDroppedBall(b, DroppedBallContext{}, NewYorkBallCourt())
	if err == nil || got.Ball != b || got.ClearExcludedCatcher {
		t.Fatal(got, err)
	}
	got, err = StepKnockedBackBall(b, NewYorkBallCourt())
	if err == nil || got.Ball != b || got.ClearExcludedCatcher {
		t.Fatal(got, err)
	}
}
