package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalFreePlayerSingleActionTrace(t *testing.T) {
	var rows []struct {
		Frame         int
		Actor         uint8
		Before, After struct {
			observedPlayer
			Control, RunGrade int32
		}
	}
	raw, err := os.ReadFile("testdata/free-action.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 420 {
		t.Fatal("missing single-call original trace")
	}
	revisits := 0
	for _, r := range rows {
		b, a := r.Before, r.After
		input, want := playerFromObservation(b.observedPlayer), playerFromObservation(a.observedPlayer)
		profile := KunioProfile()
		profile.JumpVelocity = 7*Pixel + Fixed(b.JumpGrade)*Pixel/2
		profile.RunSpeed = 4*Pixel + Fixed(b.RunGrade)*Pixel/4
		got, err := StepFreePlayerAction(input, FreePlayerContext{Computer: b.Control&128 != 0, JumpPhase: int8(b.Boost)}, profile, NewYorkGroundCourt())
		revisit := a.Attachment&64 == 0
		if err != nil || got.Player != want || got.Revisit != revisit {
			t.Fatalf("frame %d actor %d before %+v\ngot %+v\nwant %+v revisit=%v err=%v", r.Frame, r.Actor, b, got, want, revisit, err)
		}
		if got.Revisit {
			revisits++
		}
	}
	t.Logf("matched %d original individual free-player calls with %d processed-flag clears", len(rows), revisits)
}
func TestFreePlayerSingleActionQuickJump(t *testing.T) {
	p := Player{Action: ActionJump, Heading: HeadingRight, Pose: Pose{Frame: 9}, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}
	got, err := StepFreePlayerAction(p, FreePlayerContext{Computer: true, JumpPhase: -1}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || !got.Revisit || got.Player.Body.Position != p.Body.Position || got.Player.Pose != p.Pose || got.Player.Animation.Cursor != 1 || got.Player.Animation.Elapsed != 0 || !got.Player.Airborne {
		t.Fatal(got, err)
	}
	second, err := StepFreePlayerAction(got.Player, FreePlayerContext{Computer: true, JumpPhase: -1}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || second.Revisit || second.Player.Body.Position.Height != 15*Pixel/2 || second.Player.Animation.Elapsed != 1 {
		t.Fatal(second, err)
	}
	p.Action = 0x3e
	failed, err := StepFreePlayerAction(p, FreePlayerContext{}, KunioProfile(), NewYorkGroundCourt())
	if err == nil || failed.Player != p || failed.Revisit {
		t.Fatal("unsupported action was not atomic", failed, err)
	}
}
