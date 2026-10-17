package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type landingSpecialOracle struct {
	worldActionFixture
	Actor                               uint8
	Action                              Action
	Entered                             bool
	BeforeFlag, PreparedFlag, AfterFlag uint8
}

func landingSpecialRows(t *testing.T) []landingSpecialOracle {
	t.Helper()
	var rows []landingSpecialOracle
	readPropAttackFixture(t, "testdata/landing-special-clear.json.gz", &rows)
	if len(rows) != 160 {
		t.Fatal("incomplete landing entry oracle", len(rows))
	}
	return rows
}

// Failure inventory: only clear on next frame; clear already-entered landing;
// clear another player's flag; clear nonlanding actions; clear after court
// motion and overwrite later writes; change all actors/ball to repair the flag.
func TestOriginalLandingSpecialEntry(t *testing.T) {
	for _, r := range landingSpecialRows(t) {
		got := r.Before.Actors[r.Actor]
		want := got
		want.SpecialShot = r.PreparedFlag
		clearOriginalLandingSpecialShot(&got)
		if got != want {
			t.Fatal(r.Actor, r.Action, r.Entered, r.BeforeFlag, got.SpecialShot, want.SpecialShot)
		}
	}
	for action := Action(0); action < 106; action++ {
		if action == 6 || action == 22 || action == 38 || action == 54 || action == 70 {
			continue
		}
		before := WorldActionActor{Player: Player{Action: action}, SpecialShot: 128}
		got := before
		clearOriginalLandingSpecialShot(&got)
		if got != before {
			t.Fatal("nonlanding flag changed", action)
		}
	}
}
func TestOriginalLandingSpecialFullCalls(t *testing.T) {
	for i, r := range landingSpecialRows(t) {
		// The two additional source aliases are entry-tested above; their complete
		// hoop/person carrying action handlers are not restored by this change.
		if r.Action == 54 || r.Action == 70 {
			continue
		}
		result := WorldActionStep{State: r.Before}
		if e := advanceWorldPlayer(&result, r.Actor, r.context(t), NewYorkGroundCourt()); e != nil {
			t.Fatal(i, e)
		}
		if result.State != r.After {
			t.Fatalf("call %d: %s", i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
	}
}
func TestOriginalLandingSpecialCommittedBeforeMotion(t *testing.T) {
	for _, r := range landingSpecialRows(t) {
		if r.Entered || r.BeforeFlag != 128 || r.Action == 54 || r.Action == 70 {
			continue
		}
		result := WorldActionStep{State: r.Before}
		ctx := r.context(t)
		motionRead := false
		court := FlatCourt{originalMotion: &courtMotionBinding{object: r.Actor, court: 1,
			load: func() CourtFloorState {
				motionRead = true
				if result.State.Actors[r.Actor].SpecialShot != 0 {
					t.Fatal("landing clear late", r.Action)
				}
				return worldFloorState(result.State, ctx)
			},
			store: func(s CourtFloorState, selected uint8) {
				storeWorldFloorState(&result.State, s, selected)
				result.State.Actors[r.Actor].SpecialShot = 77
			},
		}}
		if e := advanceWorldPlayer(&result, r.Actor, ctx, court); e != nil {
			t.Fatal(e)
		}
		if !motionRead || result.State.Actors[r.Actor].SpecialShot != 77 {
			t.Fatal("entry command replayed over later world write", r.Action)
		}
	}
}
func TestOriginalLandingSpecialRealContinuation(t *testing.T) {
	var s RuntimeState
	var expected []WorldActionState
	readPropAttackFixture(t, "testdata/landing-special-runtime-before.json.gz", &s)
	readPropAttackFixture(t, "testdata/landing-special-runtime-worlds.json.gz", &expected)
	if s.VideoFrames != 3152 || len(expected) != 60 {
		t.Fatal("wrong original comparison boundary")
	}
	next := 0
	for i := 0; i < 180; i++ {
		data, _ := json.Marshal(s)
		var restored RuntimeState
		if e := json.Unmarshal(data, &restored); e != nil {
			t.Fatal(e)
		}
		a, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(i, e)
		}
		b, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("restore differs", i, e)
		}
		if a.LogicAdvanced {
			got := a.State.Playing.World.Preparation.Actions
			if got != expected[next] {
				t.Fatalf("source world %d: %s", 1013+next, firstWorldLoopDifference(reflect.ValueOf(got), reflect.ValueOf(expected[next]), "world"))
			}
			next++
		}
		s = a.State
	}
	if next != 60 {
		t.Fatal("missing full source worlds", next)
	}
}
