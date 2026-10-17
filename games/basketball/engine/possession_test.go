package engine

// These original-game oracles precede implementation. Failure cases are listed
// in VERIFICATION.md: release timing, facing, jump revisit, landing possession,
// shot cycle, final animation pose, and integer/fractional held-ball offsets.
import (
	"encoding/json"
	"os"
	"testing"
)

type possessionObservation struct {
	Holder, Heading, SpecialShot int32
	Held, Pressed                uint8
	InputMemory                  observedInput
	Player, Teammate, Ball       observedPlayer
}

func TestCarrierInputAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range possessionRows(t) {
		a := r.Before.Player.Action & 127
		if r.Kind != "input" || a < 16 || a > 28 {
			continue
		}
		p := observedCarrier(r.Before).Player
		ctx := CarrierInputContext{Player: FreeInputContext{Action: p.Action, Airborne: p.Airborne, PoseFacingLeft: p.Pose.Mirrored, Request: p.Request}, HasBall: r.Before.Holder == 0}
		got, err := ResolveCarrierInput(ctx, inputMemory(r.Before.InputMemory), ControllerFrame{Held: Buttons(r.Before.Held), Pressed: Buttons(r.Before.Pressed)})
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		heading := HeadingNone
		if r.After.Heading != 255 {
			heading = Heading(r.After.Heading / 32)
		}
		if got.Memory != inputMemory(r.After.InputMemory) || got.Heading != heading || got.Request != observedRequest(uint8(r.After.Player.Request)) {
			t.Fatalf("%s frame %d: got %+v want %+v", r.Scenario, r.Frame, got, r.After)
		}
		count++
	}
	if count < 150 {
		t.Fatalf("too few original carrier inputs %d", count)
	}
	t.Logf("matched %d original carrier input calls", count)
}

type possessionRow struct {
	Scenario, Kind string
	Frame          int
	Before, After  possessionObservation
}

func possessionRows(t *testing.T) []possessionRow {
	t.Helper()
	b, err := os.ReadFile("testdata/possession.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []possessionRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}
func observedCarrier(r possessionObservation) Carrier {
	r.Player.Heading = r.Heading
	return Carrier{Player: playerFromObservation(r.Player), ShotCycle: int8(r.SpecialShot)}
}

func TestCarrierActionsAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range possessionRows(t) {
		a := r.Before.Player.Action & 127
		if r.Kind != "player" || a < 16 || a > 28 {
			continue
		}
		if r.Before.Player.Attachment&128 != 0 || r.Before.Player.Boost != 0 {
			t.Fatal("fixture leaves unattached normal-jump context")
		}
		original := observedCarrier(r.Before)
		ctx := CarrierContext{HasBall: r.Before.Holder == 0, Teammate: playerFromObservation(r.Before.Teammate).Body.Position, BallAction: BallAction(r.Before.Ball.Action & 127)}
		got, err := StepCarrierAction(original, ctx, KunioProfile(), NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		if got.Carrier != observedCarrier(r.After) {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, original, got.Carrier, observedCarrier(r.After))
		}
		ballAction := r.Before.Ball.Action
		if got.Ball.Change {
			ballAction = int32(got.Ball.Action)
		}
		if ballAction != r.After.Ball.Action || got.Revisit != (r.After.Player.Attachment&64 == 0) {
			t.Fatalf("%s frame %d: effects %+v; ball %d => %d; attachment %d", r.Scenario, r.Frame, got, r.Before.Ball.Action, r.After.Ball.Action, r.After.Player.Attachment)
		}
		count++
	}
	if count < 150 {
		t.Fatalf("too few carrier calls: %d", count)
	}
	t.Logf("matched %d original carrier action calls", count)
}

func TestHeldBallAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range possessionRows(t) {
		a := r.Before.Ball.Action & 127
		pa := r.Before.Player.Action & 127
		if r.Kind != "ball" || (a != 1 && a != 2) || r.Before.Holder != 0 || pa < 16 || pa > 28 {
			continue
		}
		p := playerFromObservation(r.Before.Ball)
		b := Ball{Body: p.Body, Action: BallAction(a), Entered: p.Entered, Pose: p.Pose, Animation: p.Animation, Airborne: p.Airborne}
		got, err := StepHeldBall(b, observedCarrier(r.Before).Player)
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		p = playerFromObservation(r.After.Ball)
		want := Ball{Body: p.Body, Action: BallAction(p.Action), Entered: p.Entered, Pose: p.Pose, Animation: p.Animation, Airborne: p.Airborne}
		if got != want {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, b, got, want)
		}
		count++
	}
	if count < 60 {
		t.Fatalf("too few held-ball calls: %d", count)
	}
	t.Logf("matched %d original held-ball calls", count)
}

func TestCarrierRejectsUnreconstructedLinkedActionWithoutChangingState(t *testing.T) {
	original := Carrier{Player: Player{Action: Action(0x3d)}}
	got, err := StepCarrierAction(original, CarrierContext{HasBall: true}, KunioProfile(), NewYorkGroundCourt())
	if err == nil || got.Carrier != original || got.Ball.Change || got.Revisit {
		t.Fatal("unreconstructed linked action must fail atomically", got, err)
	}
}

func TestPassedBallFlightAgainstOriginal(t *testing.T) {
	count := 0
	for _, r := range possessionRows(t) {
		if r.Kind != "ball" || r.Before.Ball.Action&127 != 3 || r.After.Ball.Action&127 != 3 {
			continue
		}
		p := playerFromObservation(r.Before.Ball)
		b := Ball{Body: p.Body, Action: BallPass, Entered: p.Entered, Pose: p.Pose, Animation: p.Animation, Airborne: p.Airborne}
		target := playerFromObservation(r.Before.Teammate).Body.Position
		got, err := StepPassedBall(b, target)
		if err != nil {
			t.Fatalf("%s frame %d: %v", r.Scenario, r.Frame, err)
		}
		p = playerFromObservation(r.After.Ball)
		want := Ball{Body: p.Body, Action: BallPass, Entered: p.Entered, Pose: p.Pose, Animation: p.Animation, Airborne: p.Airborne}
		if got != want {
			t.Fatalf("%s frame %d\nbefore %+v\ngot %+v\nwant %+v", r.Scenario, r.Frame, b, got, want)
		}
		count++
	}
	if count < 20 {
		t.Fatalf("too few original pass flight calls: %d", count)
	}
	t.Logf("matched %d original pass flight calls", count)
}

func TestContinuousPossessionFromButtons(t *testing.T) {
	// Fixed prefixes end before original combat, another player catches the
	// ball, or basket contact begins. No mismatch-driven skipping is permitted.
	// The quick shot contacts a rim before update 34: the separate original
	// world-contact phase quarters velocity while retaining shot action 4.
	// Action identity alone is therefore not a valid contact boundary.
	limits := map[string]int{"idle": 34, "walk": 27, "pass": 17, "shoot": 34, "quick-shot": 34, "jump": 34, "jump-pass": 29, "jump-shot": 33}
	rows := possessionRows(t)
	inputs, worlds, balls := map[string][]possessionRow{}, map[string][]possessionRow{}, map[string][]possessionRow{}
	for _, r := range rows {
		switch r.Kind {
		case "input":
			inputs[r.Scenario] = append(inputs[r.Scenario], r)
		case "world":
			worlds[r.Scenario] = append(worlds[r.Scenario], r)
		case "ball":
			balls[r.Scenario] = append(balls[r.Scenario], r)
		}
	}
	count := 0
	for name, limit := range limits {
		start := inputs[name][0].Before
		b := playerFromObservation(worlds[name][0].Before.Ball)
		state := PossessionState{Carrier: observedCarrier(start), Input: inputMemory(start.InputMemory), HasBall: start.Holder == 0, Ball: Ball{Body: b.Body, Action: BallAction(b.Action), Entered: b.Entered, Pose: b.Pose, Animation: b.Animation, Airborne: b.Airborne, Request: BallRequest{Action: BallAction(b.Request.Action), Pending: b.Request.Pending}}}
		for i := 0; i < limit; i++ {
			in, world, ball := inputs[name][i], worlds[name][i], balls[name][i]
			// The teammate is still the original CPU player's external state.
			// Neither the controlled player nor the ball is reinitialized.
			teammate := playerFromObservation(ball.Before.Teammate).Body.Position
			var err error
			state, err = StepControlledPossession(state, ControllerFrame{Held: Buttons(in.Before.Held), Pressed: Buttons(in.Before.Pressed)}, teammate, KunioProfile(), NewYorkGroundCourt())
			if err != nil {
				t.Fatalf("%s update %d: %v", name, i, err)
			}
			b = playerFromObservation(world.After.Ball)
			wantBall := Ball{Body: b.Body, Action: BallAction(b.Action), Entered: b.Entered, Pose: b.Pose, Animation: b.Animation, Airborne: b.Airborne, Request: BallRequest{Action: BallAction(b.Request.Action), Pending: b.Request.Pending}}
			if state.Carrier != observedCarrier(world.After) || state.Ball != wantBall || state.HasBall != (world.After.Holder == 0) || state.Input != inputMemory(in.After.InputMemory) {
				t.Fatalf("%s update %d\ngot %+v\nwant carrier %+v ball %+v holder %d input %+v", name, i, state, observedCarrier(world.After), wantBall, world.After.Holder, inputMemory(in.After.InputMemory))
			}
			count++
		}
	}
	t.Logf("matched %d continuous updates across %d scenarios from raw logical buttons", count, len(limits))
}

func TestWorldDoesNotIgnorePendingBallContact(t *testing.T) {
	pos := Position{X: 200 * Pixel, Depth: 40 * Pixel}
	original := PossessionState{Carrier: Carrier{Player: Player{Action: ActionIdle, Body: Body{Position: pos}}}, Ball: Ball{Action: BallBounce, Entered: true, BouncesRemaining: 2, Body: Body{Position: pos}, Request: BallRequest{Action: BallKnockback, Pending: true}}}
	got, err := StepControlledPossession(original, ControllerFrame{}, pos, KunioProfile(), NewYorkGroundCourt())
	if err == nil || got != original {
		t.Fatal("unrestored contact must not silently continue flight", got, err)
	}
}

func TestWorldAppliesDeflectionRequestBeforeBallPhysics(t *testing.T) {
	pos := Position{X: 200 * Pixel, Depth: 40 * Pixel}
	ballPos := pos
	ballPos.Height = 10 * Pixel
	s := PossessionState{Carrier: Carrier{Player: Player{Action: ActionIdle, Body: Body{Position: pos}}}, Ball: Ball{Action: BallShot, Entered: true, Airborne: true, Body: Body{Position: ballPos, Velocity: Velocity{X: 2 * Pixel}, Gravity: 128}, Request: BallRequest{Action: BallDeflected, Pending: true}, Deflection: DeflectX}}
	got, err := StepControlledPossession(s, ControllerFrame{}, pos, KunioProfile(), NewYorkGroundCourt())
	if err != nil {
		t.Fatal(err)
	}
	if got.Ball.Action != BallDeflected || !got.Ball.Entered || got.Ball.Request.Pending || got.Ball.Body.Position != (Position{X: 198 * Pixel, Depth: 40 * Pixel, Height: 10*Pixel - 128}) || got.HasBall {
		t.Fatal(got)
	}
}

func TestContinuousShotThroughRimGoal(t *testing.T) {
	// This integration scene uses seed zero for a deterministic made basket.
	// Original random-state/call-order fidelity is verified independently in
	// rim_test.go; the other players' RNG-consuming AI is not executed here.
	var inputs, worlds []possessionRow
	for _, r := range possessionRows(t) {
		if r.Scenario == "quick-shot" {
			if r.Kind == "input" {
				inputs = append(inputs, r)
			}
			if r.Kind == "world" {
				worlds = append(worlds, r)
			}
		}
	}
	start := inputs[0].Before
	b := playerFromObservation(worlds[0].Before.Ball)
	s := PossessionState{Carrier: observedCarrier(start), Input: inputMemory(start.InputMemory), HasBall: true, Ball: Ball{Body: b.Body, Action: BallAction(b.Action), Entered: b.Entered, Pose: b.Pose, Animation: b.Animation, Airborne: b.Airborne, Request: BallRequest{Action: BallAction(b.Request.Action), Pending: b.Request.Pending}}, RimFlags: [6]uint8{1, 1, 1, 1, 1, 1}}
	s.Hoops = NewYorkHoops()
	for i := 0; i < 38; i++ {
		var err error
		s, err = StepControlledPossession(s, ControllerFrame{Held: Buttons(inputs[i].Before.Held), Pressed: Buttons(inputs[i].Before.Pressed)}, Position{}, KunioProfile(), NewYorkGroundCourt())
		if err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
		b = playerFromObservation(worlds[i].After.Ball)
		wantBall := Ball{Body: b.Body, Action: BallAction(b.Action), Entered: b.Entered, Pose: b.Pose, Animation: b.Animation, Airborne: b.Airborne, Request: BallRequest{Action: BallAction(b.Request.Action), Pending: b.Request.Pending}}
		if s.Ball != wantBall || s.Carrier != observedCarrier(worlds[i].After) {
			t.Fatalf("update %d\ngot %+v\nwant ball %+v carrier %+v", i, s, wantBall, observedCarrier(worlds[i].After))
		}
		if i < 34 && s.Scores != ([2]uint16{}) {
			t.Fatal("scored before rim contact", i, s.Scores)
		}
		if i >= 34 && s.Scores != ([2]uint16{3, 0}) {
			t.Fatal("goal missing or counted twice", i, s.Scores)
		}
		if i >= 34 && (s.Hoops[1].Durability != 7 || !s.Hoops[1].Entered || s.Hoops[1].Animation.Elapsed == 0 && i == 34) {
			t.Fatal("scoring hoop action was not advanced exactly once", i, s.Hoops[1])
		}
	}
	t.Log("38 continuous controlled updates through the original made-shot trajectory; deterministic test seed")
}
