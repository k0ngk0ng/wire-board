package engine

import "testing"

func TestBallDeflectionAndDispatchAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range ballRows(t) {
		before, after := r.Before.Action&127, r.After.Action&127
		if after != 8 && !(before == 8 && after == 6) {
			continue
		}
		b, err := ResolveBallRequest(ballFromObservation(r.Before))
		if err != nil {
			t.Fatal(err)
		}
		switch b.Action {
		case BallDeflected:
			b, err = StepDeflectedBall(b, NewYorkBallCourt())
		case BallBounce:
			b, err = StepBouncingBall(b, NewYorkBallCourt())
		default:
			t.Fatalf("%s frame %d: dispatch left action %d", r.Scenario, r.Frame, b.Action)
		}
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		if want := ballFromObservation(r.After); b != want {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, ballFromObservation(r.Before), b, want)
		}
		count++
	}
	if count != 68 {
		t.Fatal("unexpected original deflection count", count)
	}
	t.Logf("matched %d original collision requests and deflection/bounce calls", count)
}

func TestBallRequestRetainsIllegalAndProcessedRequests(t *testing.T) {
	for _, b := range []Ball{{Action: BallShot, Entered: true, Request: BallRequest{Action: BallRest, Pending: true}}, {Action: BallBounce, Entered: true, Request: BallRequest{Action: BallDeflected, Pending: false}}} {
		got, err := ResolveBallRequest(b)
		if err != nil || got != b {
			t.Fatal(b, got, err)
		}
	}
}
