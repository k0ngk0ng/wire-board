package engine

import "testing"

func TestRiderActionDoesNotIgnoreAcceptedUnrestoredRequests(t *testing.T) {
	// Original E208 request masks clear bit 0x04 in byte 7 for these requests
	// while the current action is 3D. They dispatch another action, so this
	// bounded rider handler must not silently keep standing on the parent.
	for _, request := range []Action{4, 7, 9, 0x14, 0x17, 0x19, 0x24, 0x27, 0x29, 0x34, 0x37, 0x39, 0x3e, 0x3f, 0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x5d, 0x60, 0x64, 0xff} {
		s := RiderPlayerState{Player: Player{Action: ActionRiding, Entered: true, Airborne: true, Request: ActionRequest{Action: request, Pending: true}}}
		ctx := RiderPlayerContext{Actor: 0, Attachment: 0xc1}
		got, err := StepRiderPlayer(s, ctx, KunioProfile(), NewYorkGroundCourt())
		if err == nil || got.State != s || got.Attachment != ctx.Attachment || got.ChangeSupportLink || got.Ball.Change {
			t.Fatalf("request %x silently ignored or partially applied: %+v %v", request, got, err)
		}
	}
}

func TestRiderActionRetainsRejectedRequest(t *testing.T) {
	s := RiderPlayerState{Player: Player{Action: ActionRiding, Entered: true, Airborne: true, Request: ActionRequest{Action: ActionIdle, Pending: true}}}
	got, err := StepRiderPlayer(s, RiderPlayerContext{Actor: 0, Attachment: 0xc1}, KunioProfile(), NewYorkGroundCourt())
	if err != nil || got.State.Player.Request != s.Player.Request || got.State.Player.Action != ActionRiding {
		t.Fatal("rejected request consumed", got, err)
	}
}
