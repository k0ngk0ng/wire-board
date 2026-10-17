package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestReceiveWhileRidingOriginalCalls(t *testing.T) {
	var rows []struct {
		Frame                   int
		Actor                   uint8
		WorldBefore, WorldAfter WorldActionState
	}
	data, err := os.ReadFile("testdata/rider-receive-natural.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatal("missing natural attached receive calls", len(rows))
	}
	for _, row := range rows {
		b := row.WorldBefore
		parent := b.PlayerLinks[row.Actor] & 15
		p := b.Actors[parent].Player
		support := RiderSupport{Body: p.Body, Pose: p.Pose, Kind: 0}
		got, err := StepCarrierAction(Carrier{Player: b.Actors[row.Actor].Player, ShotCycle: b.Actors[row.Actor].ShotCycle}, CarrierContext{HasBall: true, AttachedSupport: &support}, KunioProfile(), NewYorkGroundCourt())
		want := Carrier{Player: row.WorldAfter.Actors[row.Actor].Player, ShotCycle: row.WorldAfter.Actors[row.Actor].ShotCycle}
		if err != nil || got.Carrier != want || got.Revisit {
			t.Fatalf("frame %d got %+v want %+v err %v", row.Frame, got, want, err)
		}
		result := WorldActionStep{State: b}
		ctx := WorldActionContext{CourtID: 1}
		for i := range ctx.Profiles {
			ctx.Profiles[i] = KunioProfile()
		}
		err = advanceWorldPlayer(&result, row.Actor, ctx, NewYorkGroundCourt())
		if err != nil || result.State != row.WorldAfter {
			t.Fatalf("frame %d complete attached receive world mismatch: %v", row.Frame, err)
		}
	}
}

func TestAttachedCarrierRejectsUnrestoredActionOrSupport(t *testing.T) {
	for _, tc := range []struct {
		action Action
		kind   uint8
	}{{ActionAimShot, 0}, {ActionReceiveBall, 255}} {
		s := Carrier{Player: Player{Action: tc.action, Airborne: true}, ShotCycle: 7}
		got, err := StepCarrierAction(s, CarrierContext{HasBall: true, AttachedSupport: &RiderSupport{Kind: tc.kind}}, KunioProfile(), NewYorkGroundCourt())
		if err == nil || got.Carrier != s || got.Ball.Change || got.Revisit || got.ChangeAimWindow {
			t.Fatal("unrestored attached carrier mutated", got, err)
		}
	}
}
