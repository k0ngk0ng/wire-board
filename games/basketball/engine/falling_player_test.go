package engine

// Failure cases: held entry loses its BallHeld command; already-entered falling
// repeats its impulse; landing chooses free rather than held action; terrain
// requests disappear; unsupported riders/held props silently become ordinary;
// invalid terrain partially mutates the returned actor or ball.
import (
	"encoding/json"
	"os"
	"testing"
)

func TestFallingPlayersAgainstOriginalSingleCalls(t *testing.T) {
	var rows []struct {
		Scenario      string
		Frame, Actor  int
		Before, After struct {
			observedHoopPlayer
			BallAction, BallRequest uint8
		}
	}
	data, err := os.ReadFile("testdata/ball-player.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	held, total := 0, 0
	for _, r := range rows {
		if r.Before.Action&127 != int32(ActionFalling) {
			continue
		}
		other := false
		for _, owner := range r.Before.Owners {
			other = other || int(owner) == r.Actor
		}
		ctx := FallingContext{Computer: r.Before.Control&128 != 0, HasBall: r.Before.Holder == int32(r.Actor), HasOtherHeldObject: other, Attached: r.Before.Attachment&128 != 0}
		original := playerFromObservation(r.Before.observedPlayer)
		got, err := StepFallingPlayer(original, ctx, NewYorkGroundCourt())
		want := playerFromObservation(r.After.observedPlayer)
		if err != nil || got.Player != want {
			t.Fatalf("%s frame %d actor %d\nbefore %+v\ngot %+v\nwant %+v\nerr %v", r.Scenario, r.Frame, r.Actor, original, got, want, err)
		}
		ball := Ball{Action: BallAction(r.Before.BallAction & 127), Entered: r.Before.BallAction&128 != 0, Request: BallRequest{Action: BallAction(r.Before.BallRequest & 127), Pending: r.Before.BallRequest&128 == 0}}
		ball = ApplyBallCommand(ball, got.Ball)
		action := uint8(ball.Action)
		if ball.Entered {
			action |= 128
		}
		request := uint8(ball.Request.Action)
		if !ball.Request.Pending {
			request |= 128
		}
		if action != r.After.BallAction || request != r.After.BallRequest {
			t.Fatalf("frame %d ball command %+v action/request %d/%d want %d/%d", r.Frame, got.Ball, action, request, r.After.BallAction, r.After.BallRequest)
		}
		total++
		if ctx.HasBall {
			held++
		}
	}
	if total != 33 || held != 19 {
		t.Fatalf("unexpected original coverage %d held %d", total, held)
	}
	t.Logf("matched %d original single falling calls, including %d held-ball calls", total, held)
}

func TestFallingHeldLandingAndTerrainCommands(t *testing.T) {
	p := Player{Action: ActionFalling, Entered: true, Airborne: true, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: Pixel}, Velocity: Velocity{Height: -2 * Pixel}, Gravity: Pixel}}
	got, err := StepFallingPlayer(p, FallingContext{HasBall: true}, NewYorkGroundCourt())
	if err != nil || got.Player.Action != ActionCarrierLand || got.Player.Entered || got.Player.Airborne || got.Ball != (BallCommand{}) {
		t.Fatal(got, err)
	}
	p = Player{Action: ActionFalling, Entered: true, NewYorkPassage: 1, Body: Body{Position: Position{X: 375 * Pixel, Depth: 80 * Pixel, Height: -256 * Pixel}}}
	got, err = StepFallingPlayer(p, FallingContext{HasBall: true}, NewYorkGroundCourt())
	if err != nil || !got.Ball.Change || got.Ball.Action != BallHeld || !got.Ball.ChangeRequest || got.Ball.Request != (BallRequest{Action: BallHeld, Pending: true}) {
		t.Fatal(got, err)
	}
}

func TestFallingRejectsUnrestoredContextAtomically(t *testing.T) {
	p := Player{Action: ActionFalling}
	for _, ctx := range []FallingContext{{Attached: true}, {HasOtherHeldObject: true}} {
		got, err := StepFallingPlayer(p, ctx, NewYorkGroundCourt())
		if err == nil || got != (FallingStep{Player: p}) {
			t.Fatal(got, err)
		}
	}
	p.NewYorkPassage = 1
	got, err := StepFallingPlayer(p, FallingContext{HasBall: true}, FlatCourt{})
	if err == nil || got != (FallingStep{Player: p}) {
		t.Fatal(got, err)
	}
	p = Player{Action: ActionIdle}
	got, err = StepFallingPlayer(p, FallingContext{}, NewYorkGroundCourt())
	if err == nil || got != (FallingStep{Player: p}) {
		t.Fatal(got, err)
	}
}
