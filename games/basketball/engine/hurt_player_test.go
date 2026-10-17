package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type observedHurt struct {
	observedPlayer
	Health, RecoveryGrade, LinkedActor, Control uint8
	SpecialHolder, AssistedActor                uint8
	Attachments                                 [4]uint8
	Actions, Facings, Owners                    [13]uint8
	BallVX                                      uint16
	Random                                      [4]uint8
}

func TestOriginalHurtPlayerTrace(t *testing.T) {
	var rows []struct {
		Frame         int
		Actor         uint8
		Before, After observedHurt
	}
	raw, err := os.ReadFile("testdata/hurt-player.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 805 {
		t.Fatal("missing original hurt calls", len(rows))
	}
	counts := map[Action]int{}
	for _, r := range rows {
		b, a := r.Before, r.After
		s := HurtState{Player: playerFromObservation(b.observedPlayer), Endurance: b.Health}
		ctx := HurtContext{Actor: r.Actor, Computer: b.Control&128 != 0, RecoveryGrade: b.RecoveryGrade, Random: RandomState(binary.BigEndian.Uint32(b.Random[:])), BallHolder: int8(b.Holder), BallVelocityX: Fixed(int16(b.BallVX)), SpecialHolder: int8(b.SpecialHolder), AssistedActor: int8(b.AssistedActor), Attachments: b.Attachments, LinkedActor: int8(b.LinkedActor)}
		for i := range ctx.AuxiliaryOwners {
			ctx.AuxiliaryOwners[i] = int8(b.Owners[4+i])
		}
		for i := range ctx.HoopOwners {
			ctx.HoopOwners[i] = int8(b.Owners[7+i])
		}
		got, err := StepHurtPlayer(s, ctx, NewYorkGroundCourt())
		want := HurtState{Player: playerFromObservation(a.observedPlayer), Endurance: a.Health}
		if err != nil || got.State != want || got.Random != RandomState(binary.BigEndian.Uint32(a.Random[:])) {
			t.Fatalf("frame %d actor %d before %+v\ngot %+v\nwant %+v err=%v", r.Frame, r.Actor, b, got, want, err)
		}
		actions, facings, owners, attachments := b.Actions, b.Facings, b.Owners, b.Attachments
		actions[r.Actor] = uint8(got.State.Player.Action)
		if got.State.Player.Entered {
			actions[r.Actor] |= 128
		}
		facings[r.Actor] = 0
		if got.State.Player.FacingLeft {
			facings[r.Actor] = 128
		}
		for _, c := range got.Objects {
			if c.SetAction {
				actions[c.Object] = c.Action
			}
			if c.SetOwner {
				owners[c.Object] = uint8(c.Owner)
			}
			if c.SetFacing {
				facings[c.Object] = 0
				if c.FacingLeft {
					facings[c.Object] = 128
				}
			}
			if c.ClearAttachment {
				attachments[c.Object] = 0
			}
		}
		holder, special, ballVX := uint8(b.Holder), b.SpecialHolder, b.BallVX
		if got.ReleaseBall {
			holder = 6
			owners[6] = 6
		}
		if got.Ball.Change {
			actions[6] = uint8(got.Ball.Action)
		}
		if got.ChangeBallVelocity {
			ballVX = uint16(got.BallVelocityX)
		}
		if got.ClearSpecialHolder {
			special = 255
		}
		if actions != a.Actions || facings != a.Facings || owners != a.Owners || attachments != a.Attachments || holder != uint8(a.Holder) || special != a.SpecialHolder || ballVX != a.BallVX {
			t.Fatalf("frame %d actor %d world effect mismatch: %+v\nactions %v/%v owners %v/%v attachments %v/%v ballVX %d/%d", r.Frame, r.Actor, got, actions, a.Actions, owners, a.Owners, attachments, a.Attachments, ballVX, a.BallVX)
		}
		counts[s.Player.Action]++
	}
	t.Logf("matched all %d original hurt calls: %v", len(rows), counts)
}
func TestHurtDropBallVelocityAndRecovery(t *testing.T) {
	for _, tc := range []struct {
		action Action
		vx     Fixed
		left   bool
		ballVX Fixed
		want   Fixed
	}{{0x48, 0, false, 123, 3 * Pixel}, {0x48, 0, true, 123, -3 * Pixel}, {0x49, -Pixel, false, 123, 4*Pixel + 123}, {0x49, Pixel, false, 123, -2*Pixel + 123}} {
		s := HurtState{Player: Player{Action: tc.action, Heading: HeadingNone, FacingLeft: tc.left, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}, Velocity: Velocity{X: tc.vx}}}, Endurance: 1}
		ctx := emptyHurtContext()
		ctx.BallHolder = 0
		ctx.BallVelocityX = tc.ballVX
		got, err := StepHurtPlayer(s, ctx, NewYorkGroundCourt())
		if err != nil || !got.ReleaseBall || !got.ChangeBallVelocity || got.BallVelocityX != tc.want {
			t.Fatal(got, err)
		}
	}
	s := HurtState{Player: Player{Action: 0x4a, Heading: HeadingNone, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}, Endurance: 20}
	ctx := emptyHurtContext()
	ctx.Random = 12345
	got, err := StepHurtPlayer(s, ctx, NewYorkGroundCourt())
	if err != nil || got.Random != ctx.Random || got.State.Player.Animation.NextAction != ActionLand {
		t.Fatal(got, err)
	}
	ctx.Attachments[0] = 128
	s.Player.Action = 0x49
	if _, err := StepHurtPlayer(s, ctx, NewYorkGroundCourt()); err == nil {
		t.Fatal("unrepresented rider link accepted")
	}
}
func emptyHurtContext() HurtContext {
	return HurtContext{BallHolder: 6, SpecialHolder: -1, AssistedActor: -1, LinkedActor: -1, AuxiliaryOwners: [2]int8{-1, -1}, HoopOwners: [6]int8{7, 8, 9, 10, 11, 12}}
}
