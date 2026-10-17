package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedShootingAim struct {
	observedDunk
	ShotCycle, AimWindow uint8
	MountFlags           [6]uint8
}

func TestOriginalShootingAimTrace(t *testing.T) {
	var rows []struct {
		Frame         int
		Actor         uint8
		Before, After observedShootingAim
	}
	raw, err := os.ReadFile("testdata/shooting-aim-player.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 125 {
		t.Fatal("missing original aim calls", len(rows))
	}
	dunks := 0
	for _, r := range rows {
		b, a := r.Before, r.After
		s := Carrier{Player: playerFromObservation(b.observedPlayer), ShotCycle: int8(b.ShotCycle)}
		search := DunkSearchContext{BallPosition: ballFromObservation(b.Ball).Body.Position, MountFlags: b.MountFlags}
		copy(search.HoopOwners[:], b.Owners[7:])
		ctx := CarrierContext{HasBall: b.Holder == int32(r.Actor), BallAction: BallAction(b.Ball.Action & 127), Computer: b.Control&128 != 0, DunkSearch: search, CarryingPlayer: b.CarriedPlayer == r.Actor}
		got, err := StepCarrierAction(s, ctx, KunioProfile(), NewYorkGroundCourt())
		want := Carrier{Player: playerFromObservation(a.observedPlayer), ShotCycle: int8(a.ShotCycle)}
		aim, target := b.AimWindow, b.TargetObject
		if got.ChangeAimWindow {
			aim = got.AimWindow
		}
		if got.ChangeDunkTarget {
			target = got.DunkTargetObject
		}
		ball := ApplyBallCommand(ballFromObservation(b.Ball), got.Ball)
		if err != nil || got.Carrier != want || aim != a.AimWindow || target != a.TargetObject || ball != ballFromObservation(a.Ball) {
			t.Fatalf("frame %d before %+v\ngot %+v\nwant %+v err=%v", r.Frame, b, got, a, err)
		}
		if got.Carrier.Player.Action == 0x58 {
			dunks++
		}
	}
	if dunks != 8 {
		t.Fatal("missing natural dunk selections", dunks)
	}
	t.Log("matched all 125 original aim calls including 8 dunk selections")
}
func TestShootingAimAutomaticShotAndByteCycle(t *testing.T) {
	for _, tc := range []struct {
		computer  bool
		cycle     int8
		automatic bool
	}{{true, 3, true}, {false, 0, true}, {false, 3, false}} {
		s := Carrier{Player: Player{Action: ActionAimShot, Heading: HeadingNone, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}, ShotCycle: tc.cycle}
		got, err := StepCarrierAction(s, CarrierContext{HasBall: true, Computer: tc.computer}, KunioProfile(), NewYorkGroundCourt())
		if err != nil || !got.ChangeAimWindow || got.AimWindow != 32 || got.Carrier.Player.Animation.HasNextAction != tc.automatic || tc.automatic && got.Carrier.Player.Animation.NextAction != ActionShoot {
			t.Fatal(got, err)
		}
	}
	s := Carrier{Player: Player{Action: ActionAimAirShot, Entered: true, Heading: HeadingNone, Airborne: true, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 30 * Pixel}}}, ShotCycle: 127}
	got, err := StepCarrierAction(s, CarrierContext{}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.Carrier.ShotCycle != -1 || !got.ChangeDunkTarget || got.DunkTargetObject != 0 {
		t.Fatal(got, err)
	}
}
func TestOriginalDunkSearchWindowAndDirection(t *testing.T) {
	for _, tc := range []struct {
		name             string
		x, depth, height int
		heading          Heading
		owner            uint8
		flags            uint8
		want             Action
		target           uint8
	}{
		{"base left", 56, 42, 80, HeadingNone, 7, 1, 0x58, 7},
		{"base upper bound", 87, 52, 91, HeadingNone, 7, 1, 0x58, 7},
		{"outside x", 88, 42, 80, HeadingNone, 7, 1, ActionAimAirShot, 0},
		{"outside depth", 56, 53, 80, HeadingNone, 7, 1, ActionAimAirShot, 0},
		{"outside height", 56, 42, 92, HeadingNone, 7, 1, ActionAimAirShot, 0},
		{"reverse", 56, 42, 80, HeadingRight, 7, 1, 0x59, 7},
		{"upward", 56, 42, 80, HeadingUp, 7, 1, 0x5b, 7},
		{"raised", 56, 42, 112, HeadingNone, 135, 2, 0x58, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := CarrierStep{Carrier: Carrier{Player: Player{Action: ActionAimAirShot, Entered: true, Airborne: true, Heading: tc.heading, Body: Body{Position: Position{X: Fixed(tc.x) * Pixel, Depth: Fixed(tc.depth) * Pixel}}}}}
			ctx := DunkSearchContext{BallPosition: Position{Height: Fixed(tc.height) * Pixel}}
			ctx.HoopOwners[0] = tc.owner
			ctx.MountFlags[0] = tc.flags
			selectOriginalDunkTarget(&s, ctx)
			if !s.ChangeDunkTarget || s.DunkTargetObject != tc.target || s.Carrier.Player.Action != tc.want {
				t.Fatal(s)
			}
		})
	}
}
