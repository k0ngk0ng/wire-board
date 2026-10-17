package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalFreeJumpWorldTrace(t *testing.T) {
	var rows []struct {
		Scenario      string
		Frame         int
		Actor         uint8
		Before, After struct {
			observedPlayer
			Control, RunGrade int32
		}
	}
	raw, err := os.ReadFile("testdata/free-jump.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("missing original free jump phases")
	}
	counts := map[Action]int{}
	boosted := 0
	for _, r := range rows {
		b, a := r.Before, r.After
		input, want := playerFromObservation(b.observedPlayer), playerFromObservation(a.observedPlayer)
		profile := KunioProfile()
		profile.JumpVelocity = (7 * Pixel) + Fixed(b.JumpGrade)*Pixel/2
		profile.RunSpeed = 4*Pixel + Fixed(b.RunGrade)*Pixel/4
		got, err := StepFreePlayerWithContext(input, FreePlayerContext{Computer: b.Control&128 != 0, JumpPhase: int8(b.Boost)}, profile, NewYorkGroundCourt())
		if err != nil || got != want {
			t.Fatalf("frame %d actor %d before %+v\ngot %+v\nwant %+v\nerr %v", r.Frame, r.Actor, b, got, want, err)
		}
		counts[input.Action]++
		if b.Boost >= 128 {
			boosted++
		}
	}
	t.Logf("matched %d original free jump world phases; actions=%v negative jump phases=%d", len(rows), counts, boosted)
}

func TestFreeJumpWindupAndSpecialBoost(t *testing.T) {
	for _, tc := range []struct {
		name            string
		phase           int8
		height          Fixed
		cursor, elapsed uint8
	}{{"ordinary", 0, 0, 1, 0}, {"fast", -1, 15 * Pixel / 2, 1, 1}, {"special shallow", -32, 2 * Pixel, 1, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			p := Player{Action: ActionJump, Heading: HeadingRight, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}, Pose: Pose{Frame: 9}}
			got, err := StepFreePlayerWithContext(p, FreePlayerContext{Computer: true, JumpPhase: tc.phase}, KunioProfile(), NewYorkGroundCourt())
			if err != nil || got.Body.Position.Height != tc.height || got.Animation.Cursor != tc.cursor || got.Animation.Elapsed != tc.elapsed || !got.Airborne {
				t.Fatal(got, err)
			}
		})
	}
}
func TestFreeFallingEntryAndLanding(t *testing.T) {
	p := Player{Action: ActionFalling, Heading: HeadingNone, FacingLeft: true, Airborne: true, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 20 * Pixel}, Velocity: Velocity{X: Pixel, Height: -2 * Pixel}}}
	got, err := StepFreePlayerWithContext(p, FreePlayerContext{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.Body.Velocity.Height != 3*Pixel/4 || got.Body.Position.Height != 20*Pixel+3*Pixel/4 || got.Pose != (Pose{Frame: 4, Mirrored: true}) || got.Animation.Elapsed != 0 {
		t.Fatal(got, err)
	}
	got, err = StepFreePlayerWithContext(got, FreePlayerContext{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.Body.Velocity.Height != -Pixel/4 || got.Animation.Elapsed != 0 {
		t.Fatal(got, err)
	}
	p.Entered = true
	p.Body.Position.Height = Pixel
	p.Body.Velocity.Height = -2 * Pixel
	p.Body.Gravity = Pixel
	got, err = StepFreePlayerWithContext(p, FreePlayerContext{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.Action != ActionLand || got.Entered || got.Airborne || got.Body.Position.Height != 0 {
		t.Fatal(got, err)
	}
}

func TestComputerActorRetainsFastFreeJumpPhase(t *testing.T) {
	actor := ComputerActorState{Carrier: HoopCarrier{Player: Player{Action: ActionIdle, Entered: true, Heading: HeadingNone, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}}, Control: AIControlState{Mode: 7, Entered: true, PlanParameter: 3}}
	got, err := AdvanceComputerActor(actor, ComputerActorContext{Seat: 0, BallHolder: 6}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.JumpPhase != -1 || got.State.Carrier.Player.Body.Position.Height != 15*Pixel/2 || got.State.Carrier.Player.Action != ActionJump {
		t.Fatal(got, err)
	}
}

func TestFallingPhysicsDoesNotEnableUnrestoredHumanInput(t *testing.T) {
	_, err := ResolveFreeInput(FreeInputContext{Action: ActionFalling, Airborne: true}, InputMemory{}, ControllerFrame{})
	if err == nil {
		t.Fatal("restoring action physics silently enabled human falling input")
	}
}
