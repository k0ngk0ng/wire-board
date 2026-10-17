package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedSpecialBall struct {
	observedSpecialShot
	Teams                  [2]uint8
	Characters             [4]uint8
	PlayerPositions        [4]Position
	Held, Pressed          [4]uint8
	MountFlags             [6]uint8
	LaunchTarget           Position
	LaunchArc              Fixed
	LandingX, LandingDepth int32
	SelectedHoop           uint8
}

func TestOriginalSpecialBallAndHighTargetTrace(t *testing.T) {
	counts := map[BallAction]int{}
	for _, path := range []string{"testdata/special-shot.json", "testdata/special-ball-cpu.json"} {
		var rows []struct {
			Kind, Scenario string
			Frame          int
			Before, After  observedSpecialBall
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(raw, &rows); err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.Kind != "ball" {
				continue
			}
			b, a := r.Before, r.After
			original := ballFromObservation(b.Ball)
			ctx := OriginalShotBallContext{Holder: int8(b.Holder), EnhancementEnabled: b.EnhancementEnabled, Equipment: 255, BallInputButtons: Buttons(b.Pressed[2]), MountFlags: b.MountFlags, SelectedHoop: int8(b.SelectedHoop), Target: b.LaunchTarget, ArcHeight: b.LaunchArc, Landing: Position{X: Fixed(b.LandingX) * Pixel, Depth: Fixed(b.LandingDepth) * Pixel}}
			if b.Holder < 4 {
				h := int(b.Holder)
				ctx.ShooterHeight = b.PlayerPositions[h].Height
				ctx.ShooterAction = Action(b.Actions[h] & 127)
				ctx.Team = b.Teams[h/2]
				ctx.Character = b.Characters[h]
				ctx.Equipment = b.ShotEquipment[ctx.Character]
				ctx.SpecialShot = b.SpecialShots[h]
			}
			got, err := StepOriginalShotBall(original, ctx, NewYorkBallCourt())
			holder := b.Holder
			if got.ReleaseHolder {
				holder = 6
			}
			flags := b.SpecialShots
			if got.ChangeSpecialShot && b.Holder < 4 {
				flags[b.Holder] = got.SpecialShot
			}
			if err != nil || got.Ball != ballFromObservation(a.Ball) || holder != a.Holder || flags != a.SpecialShots || got.SelectedHoop != int8(a.SelectedHoop) || got.Target != a.LaunchTarget || got.ArcHeight != a.LaunchArc || got.Landing != (Position{X: Fixed(a.LandingX) * Pixel, Depth: Fixed(a.LandingDepth) * Pixel}) {
				t.Fatalf("%s frame %d action%d entered=%v\nbefore %+v\ngot %+v\nwant %+v err=%v", r.Scenario, r.Frame, original.Action, original.Entered, b, got, a, err)
			}
			counts[original.Action]++
		}
	}
	if counts[4] != 1031 || counts[23] != 182 || counts[21] != 1 {
		t.Fatal("missing original special/normal ball calls", counts)
	}
	t.Logf("matched original ball calls %v; action20 is covered by fixed source-rule cases, not this natural corpus", counts)
}
func TestOriginalSpecialBallRedirectRules(t *testing.T) {
	b := Ball{Action: 20, Entered: true, Airborne: true, Body: Body{Position: Position{X: 463 * Pixel, Depth: 40 * Pixel, Height: 100 * Pixel}, Velocity: Velocity{X: 2 * Pixel}}}
	ctx := OriginalShotBallContext{Holder: 6, SelectedHoop: 8}
	got, err := StepOriginalShotBall(b, ctx, NewYorkBallCourt())
	if err != nil || got.Ball.Animation.Cursor != 5 || got.Ball.Animation.Elapsed != 1 || got.Ball.Body.Position.X != 464*Pixel || got.Ball.Body.Position.Depth != 45*Pixel || got.Ball.Body.Velocity != (Velocity{}) || got.Ball.Body.Gravity != Pixel/4 {
		t.Fatal(got, err)
	}
	b = Ball{Action: 21, Entered: true, Airborne: true, Animation: AnimationState{NextAction: 21, HasNextAction: true}, Body: Body{Position: Position{X: 440 * Pixel, Depth: 40 * Pixel, Height: 60 * Pixel}, Velocity: Velocity{X: 8 * Pixel}, Gravity: -160}}
	got, err = StepOriginalShotBall(b, ctx, NewYorkBallCourt())
	if err != nil || got.Ball.Body.Position.X != 463*Pixel || got.Ball.Body.Position.Depth != 45*Pixel || got.Ball.Body.Velocity.Height != -11*Pixel/4 || got.Ball.Body.Velocity.X != 0 || got.Ball.Body.Gravity != Pixel || got.Ball.Action != 21 {
		t.Fatal(got, err)
	}
}
func TestOriginalShotFlagConsumptionByCharacter(t *testing.T) {
	for _, tc := range []struct {
		team, character uint8
		clear           bool
		height          Fixed
	}{{0, 0, true, 272 * Pixel}, {0, 1, false, 88 * Pixel}, {1, 0, true, 272 * Pixel}} {
		b := Ball{Action: 4, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel, Height: 30 * Pixel}}}
		ctx := OriginalShotBallContext{Holder: 0, ShooterAction: ActionShoot, Team: tc.team, Character: tc.character, EnhancementEnabled: true, Equipment: 255, SpecialShot: 128}
		got, err := StepOriginalShotBall(b, ctx, NewYorkBallCourt())
		if err != nil || got.Target.Height != tc.height || got.ChangeSpecialShot != tc.clear || !got.ReleaseHolder {
			t.Fatal(got, err)
		}
	}
}
