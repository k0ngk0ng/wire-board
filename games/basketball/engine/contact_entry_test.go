package engine

import "testing"

func TestPreparationSkipsUnenteredPlayerContactGeometry(t *testing.T) {
	s := WorldActionState{}
	// Use a restored in-play ball profile; the zero-value BallRest geometry
	// belongs to the opening and is unrelated to this player-entry boundary.
	s.Ball.Action = BallKnockback
	s.Actors[0].Player = Player{Action: ActionCatchBall, Animation: AnimationState{Cursor: 255}}
	ctx := WorldPreparationContext{Kinds: [7]uint8{0, 0, 1, 1, 12, 11, 8}}
	objects, err := preparationObjects(s, ctx)
	if err != nil {
		t.Fatalf("unentered player old animation cursor must never be interpreted: %v", err)
	}
	if objects[0].Entered || objects[0].Action != uint8(ActionCatchBall) {
		t.Fatal("lost original entry metadata", objects[0])
	}
	if got := FindObjectContacts(objects, 0); got.Candidates != ([7]int8{-1, -1, -1, -1, -1, -1, -1}) {
		t.Fatal("unentered player participated in contacts", got)
	}
	s.Actors[0].Player.Entered = true
	if _, err = preparationObjects(s, ctx); err == nil {
		t.Fatal("entered player invalid geometry silently skipped")
	}
}

func TestPreparationStillReadsUnenteredAuxiliaryTargetGeometry(t *testing.T) {
	s := WorldActionState{}
	s.Auxiliary[0].Animation.Cursor = 255
	ctx := WorldPreparationContext{Kinds: [7]uint8{0, 0, 1, 1, 12, 11, 8}}
	if _, err := preparationObjects(s, ctx); err == nil {
		t.Fatal("auxiliary target incorrectly treated as an unentered player")
	}
}
