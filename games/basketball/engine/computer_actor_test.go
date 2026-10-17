package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type observedCollection struct {
	Player                                                           observedHoopPlayer
	Control                                                          observedAIControl
	Hoops                                                            [6]observedHoop
	Holder, ContactLatch, FirstClimber, SecondClimber, SelectedFlags uint8
	MountFlags                                                       [6]uint8
	Players                                                          [4]observedPlayer
	Ball                                                             observedBall
}

func TestOriginalAICollectionSequence(t *testing.T) {
	b, err := os.ReadFile("testdata/collection.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame                               int
		Before, After                       observedCollection
		ActorRandomBefore, ActorRandomAfter [4]byte
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 27 {
		t.Fatal("unexpected collection interval", len(rows))
	}
	first := rows[0].Before
	actor := ComputerActorState{Carrier: hoopCarrierFromObservation(first.Player), Control: aiControlFromObservation(first.Control)}
	var hoops [6]Hoop
	for i, h := range first.Hoops {
		hoops[i] = hoopFromObservation(h)
	}
	mounts, latch := first.MountFlags, first.ContactLatch
	profile := KunioProfile()
	profile.ID = "original-opponent"
	profile.RunSpeed, err = RunSpeedForGrade(uint8(first.Player.RunGrade))
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	for _, r := range rows {
		// Ball possession and climbing belong to the other actors. No actor,
		// AI memory or hoop position/animation is reloaded after initialization.
		hoops = ResetHoopScoreLocks(hoops, r.Before.Holder != 6)
		ctx := ComputerActorContext{Seat: 2, HasBall: r.Before.Holder == 2, BallHolder: int8(r.Before.Holder), Hoops: hoops, BallPosition: ballFromObservation(r.Before.Ball).Body.Position, Random: RandomState(binary.BigEndian.Uint32(r.ActorRandomBefore[:]))}
		for n, p := range r.Before.Players {
			ctx.Players[n] = playerFromObservation(p).Body.Position
		}
		ctx.Players[2] = actor.Carrier.Player.Body.Position
		step, err := AdvanceComputerActor(actor, ctx, profile, NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("frame %d: %v", r.Frame, err)
		}
		actor, hoops = step.State, step.Hoops
		if step.Random != RandomState(binary.BigEndian.Uint32(r.ActorRandomAfter[:])) {
			t.Fatal("actor random consumption differs", r.Frame)
		}
		detached := uint8(0)
		for _, h := range hoops {
			if h.Action == HoopInactive || h.Action >= HoopCarried {
				detached++
			}
		}
		for n, h := range hoops {
			ctx := HoopContext{Index: uint8(n), DetachedCount: detached, MountFlags: mounts, ContactLatch: latch, HasClimber: int(r.Before.FirstClimber) == n+7 || int(r.Before.SecondClimber) == n+7}
			if h.Carried {
				if h.CarrierSeat != 2 {
					t.Fatal("collection fixture added another hoop carrier")
				}
				ctx.Owner = &actor.Carrier.Player
				ctx.OwnerComputer = true
				ctx.HighThrowArc = first.SelectedFlags&2 != 0
			}
			next, err := StepHoopAction(h, ctx, NewYorkGroundCourt())
			if err != nil {
				t.Fatalf("frame %d hoop %d: %v", r.Frame, n, err)
			}
			hoops[n], mounts, latch = next.Hoop, next.MountFlags, next.ContactLatch
		}
		if actor.Carrier != hoopCarrierFromObservation(r.After.Player) || actor.Control != aiControlFromObservation(r.After.Control) {
			t.Fatalf("frame %d\ngot %+v\nwant carrier %+v control %+v", r.Frame, actor, hoopCarrierFromObservation(r.After.Player), aiControlFromObservation(r.After.Control))
		}
		for n, h := range hoops {
			if h != hoopFromObservation(r.After.Hoops[n]) {
				t.Fatalf("frame %d hoop %d\ngot %+v\nwant %+v", r.Frame, n, h, hoopFromObservation(r.After.Hoops[n]))
			}
		}
		if mounts != r.After.MountFlags || latch != r.After.ContactLatch {
			t.Fatal("world hoop metadata diverged", r.Frame, mounts, latch)
		}
		completed++
	}
	if completed != 27 {
		t.Fatal(completed)
	}
	t.Log("27 continuous original AI→actor→six-hoop updates, through pickup, carry, throw and defensive handoff")
}

func TestPickupDecisionActorAndBallPhases(t *testing.T) {
	// A fixed rule scenario for the rare ground-pickup branch. This is not
	// counted as a naturally recorded original-game sequence.
	position := Position{X: 100 * Pixel, Depth: 40 * Pixel}
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionIdle, Entered: true, Heading: HeadingNone, Body: Body{Position: position}}}, Control: AIControlState{Mode: 2, Entered: true, Counter: 1}}
	ball := Ball{Action: BallRest, Body: Body{Position: Position{X: 109 * Pixel, Depth: 48 * Pixel}}}
	view := AIControlView{Actor: 1, Action: ActionIdle, ActionEntered: true, Heading: HeadingNone, Position: position, BallHolder: 6, BallAction: BallRest, BallPosition: ball.Body.Position}
	decision, err := ReactOriginalBall(actor.Control, view)
	if err != nil {
		t.Fatal(err)
	}
	ctx := ComputerActorContext{Seat: 1, BallHolder: 6, BallAction: BallRest, BallPosition: ball.Body.Position}
	bindings := 0
	for i := 0; i < 9; i++ {
		var step ComputerActorStep
		if i == 0 {
			step, err = advanceComputerActorDecision(actor, ctx, decision, KunioProfile(), NewYorkGroundCourt())
		} else {
			step, err = AdvanceComputerActor(actor, ctx, KunioProfile(), NewYorkGroundCourt())
		}
		if err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
		actor = step.State
		if step.BindBall {
			bindings++
			ctx.HasBall = true
			ctx.BallHolder = 1
		}
		if step.Ball.Change {
			ball.Action, ball.Entered = step.Ball.Action, false
		}
		if !ctx.HasBall {
			t.Fatal("held-ball phase ran before ownership binding")
		}
		ball, err = StepHeldBall(ball, actor.Carrier.Player)
		if err != nil {
			t.Fatal(err)
		}
		ctx.BallAction, ctx.BallPosition = ball.Action, ball.Body.Position
		if actor.Carrier.Player.Body.Position != position {
			t.Fatal("pickup teleported player", i, actor)
		}
	}
	if bindings != 1 || ctx.BallHolder != 1 || actor.Carrier.Player.Action != ActionDribbleIdle || actor.Carrier.Player.Entered || ball.Body.Position != (Position{X: 96 * Pixel, Depth: 39 * Pixel, Height: 7 * Pixel}) {
		t.Fatal(bindings, actor, ball)
	}
}
