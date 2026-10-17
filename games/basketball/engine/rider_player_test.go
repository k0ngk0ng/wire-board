package engine

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type observedRiderSupport struct {
	X, Depth, Height, VX, VY, VZ, Gravity int32
	Pose, Kind, LinkedActor               uint8
}
type observedRiderPlayer struct {
	observedPlayer
	Boost           uint8
	Holder          int8
	Owners, Actions [13]uint8
	Objects         [6]observedRiderSupport
}

func TestRiderPlayerAllNaturalCalls(t *testing.T) {
	var rows []struct {
		Frame         int
		Actor         uint8
		Before, After observedRiderPlayer
	}
	data, err := os.ReadFile("testdata/rider-actions-natural.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 {
		t.Fatal("missing natural rider calls", len(rows))
	}
	detached, held := 0, 0
	for _, row := range rows {
		b, a := row.Before, row.After
		parent := b.Attachment & 15
		o := b.Objects[parent]
		ctx := RiderPlayerContext{Actor: row.Actor, Attachment: uint8(b.Attachment), Support: RiderSupport{Body: Body{Position: Position{X: Fixed(o.X), Depth: Fixed(o.Depth), Height: Fixed(o.Height)}, Velocity: Velocity{X: Fixed(int16(o.VX)), Depth: Fixed(int16(o.VY)), Height: Fixed(int16(o.VZ))}, Gravity: Fixed(int16(o.Gravity))}, Kind: o.Kind, Pose: Pose{Frame: o.Pose & 127, Mirrored: o.Pose&128 != 0}}, HasBall: b.Holder == int8(row.Actor), AuxiliaryOwners: [2]int8{int8(b.Owners[4]), int8(b.Owners[5])}}
		original := RiderPlayerState{Player: playerFromObservation(b.observedPlayer), JumpPhase: int8(b.Boost)}
		got, err := StepRiderPlayer(original, ctx, KunioProfile(), NewYorkGroundCourt())
		want := RiderPlayerState{Player: playerFromObservation(a.observedPlayer), JumpPhase: int8(a.Boost)}
		if err != nil || got.State != want || got.Attachment != uint8(a.Attachment) {
			t.Fatalf("frame %d\ngot %+v\nwant %+v attachment %d err %v", row.Frame, got, want, a.Attachment, err)
		}
		links := [4]uint8{b.Objects[0].LinkedActor, b.Objects[1].LinkedActor, b.Objects[2].LinkedActor, b.Objects[3].LinkedActor}
		if got.ChangeSupportLink {
			links[got.SupportObject] = row.Actor
		}
		for i, v := range links {
			if v != a.Objects[i].LinkedActor {
				t.Fatalf("frame %d support %d link %d want %d", row.Frame, i, v, a.Objects[i].LinkedActor)
			}
		}
		ballAction := b.Actions[6]
		if got.Ball.Change {
			ballAction = uint8(got.Ball.Action)
		}
		if ballAction != a.Actions[6] {
			t.Fatalf("frame %d ball action %d want %d", row.Frame, ballAction, a.Actions[6])
		}
		if got.Attachment&128 == 0 {
			detached++
		}
		if ctx.HasBall {
			held++
		}
	}
	if detached != 1 || held == 0 {
		t.Fatal("natural detach/held coverage missing", detached, held)
	}
	t.Logf("matched all %d natural rider calls including %d detached and %d held calls", len(rows), detached, held)
}

func TestRiderPlayerRejectsUnsupportedContextAtomically(t *testing.T) {
	s := RiderPlayerState{Player: Player{Action: 0x3d, Airborne: true}, JumpPhase: -1}
	for _, c := range []RiderPlayerContext{{Actor: 4}, {Actor: 0, Attachment: 0x8f}, {Actor: 0, Attachment: 0x81, Support: RiderSupport{Kind: 255}}} {
		got, err := StepRiderPlayer(s, c, KunioProfile(), NewYorkGroundCourt())
		want := RiderPlayerStep{State: s, Attachment: c.Attachment}
		if err == nil || !reflect.DeepEqual(got, want) {
			t.Fatal("invalid rider changed state", got, err)
		}
	}
}

func TestRiderPlayerSupportPoseOffsetsPreserveFractions(t *testing.T) {
	for _, tc := range []struct {
		kind, pose uint8
		offset     Fixed
	}{{0, 0, 32 * Pixel}, {0, 5, 24 * Pixel}, {0, 0x19, 16 * Pixel}, {0, 0x47, 24 * Pixel}, {0, 0x4b, 16 * Pixel}, {12, 5, 8 * Pixel}} {
		s := RiderPlayerState{Player: Player{Action: 0x3d, Entered: true, Airborne: true, Body: Body{Position: Position{X: 17, Depth: 31, Height: 63}}}}
		c := RiderPlayerContext{Actor: 0, Attachment: 0xc1, Support: RiderSupport{Kind: tc.kind, Pose: Pose{Frame: tc.pose}, Body: Body{Position: Position{X: 100*Pixel + 128, Depth: 40*Pixel + 192, Height: 10*Pixel + 250}}}}
		got, err := StepRiderPlayer(s, c, KunioProfile(), NewYorkGroundCourt())
		want := Position{X: 100*Pixel + 17, Depth: 40*Pixel + 31, Height: 10*Pixel + tc.offset + 63}
		if err != nil || got.State.Player.Body.Position != want {
			t.Fatal(tc, got, err)
		}
	}
}

func TestWorldRiderActorAllNaturalCalls(t *testing.T) {
	var rows []struct {
		Frame                   int
		Actor                   uint8
		WorldBefore, WorldAfter WorldActionState
	}
	data, err := os.ReadFile("testdata/rider-actions-natural.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 11 {
		t.Fatal("missing original rider worlds")
	}
	for _, row := range rows {
		result := WorldActionStep{State: row.WorldBefore}
		ctx := WorldActionContext{CourtID: 1}
		for i := range ctx.Profiles {
			ctx.Profiles[i] = KunioProfile()
		}
		err := advanceWorldPlayer(&result, row.Actor, ctx, NewYorkGroundCourt())
		if err != nil || result.State != row.WorldAfter {
			t.Fatalf("frame %d rider world error %v\ngot %+v\nwant %+v", row.Frame, err, result.State, row.WorldAfter)
		}
	}
}
