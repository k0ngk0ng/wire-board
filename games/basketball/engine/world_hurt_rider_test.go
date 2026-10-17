package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

type hurtRiderOracle struct {
	worldActionFixture
	Actor                  uint8
	Label                  string
	Prepared, BeforeMotion WorldActionState
}

func hurtRiderRows(t *testing.T) []hurtRiderOracle {
	t.Helper()
	var rows []hurtRiderOracle
	readPropAttackFixture(t, "testdata/hurt-rider.json.gz", &rows)
	if len(rows) != 160 {
		t.Fatal("missing original hurt rider cases", len(rows))
	}
	return rows
}
func TestOriginalHurtRiderFullCalls(t *testing.T) {
	for _, r := range hurtRiderRows(t) {
		t.Run(r.Label, func(t *testing.T) {
			result := WorldActionStep{State: r.Before}
			if e := advanceWorldHurtPlayer(&result, r.Actor, r.context(t), NewYorkGroundCourt()); e != nil {
				t.Fatal(e)
			}
			if result.State != r.After {
				t.Fatal(firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
			}
		})
	}
}
func TestOriginalHurtRiderPreparationBeforeMotion(t *testing.T) {
	for _, r := range hurtRiderRows(t) {
		result := WorldActionStep{State: r.Before}
		c := r.context(t)
		seen := false
		court := FlatCourt{originalMotion: &courtMotionBinding{object: r.Actor, court: 1,
			load: func() CourtFloorState {
				seen = true
				if result.State != r.Prepared {
					t.Fatal(r.Label, "pre-motion commands", firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.Prepared), "world"))
				}
				return worldFloorState(result.State, c)
			}, store: func(s CourtFloorState, selected uint8) { storeWorldFloorState(&result.State, s, selected) },
		}}
		if e := advanceWorldHurtPlayer(&result, r.Actor, c, court); e != nil {
			t.Fatal(r.Label, e)
		}
		if !seen || result.State != r.After {
			t.Fatal(r.Label, "final", seen, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
	}
}
func TestOriginalHurtRiderIntegratedDispatcher(t *testing.T) {
	for _, r := range hurtRiderRows(t) {
		result := WorldActionStep{State: r.Before}
		if e := advanceWorldPlayer(&result, r.Actor, r.context(t), NewYorkGroundCourt()); e != nil {
			t.Fatal(r.Label, e)
		}
		if result.State != r.After {
			t.Fatal(r.Label, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
	}
}
func TestRuntimeHurtRiderFailureContinues(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/hurt-rider-runtime-before.json.gz", &s)
	if s.VideoFrames != 11546 {
		t.Fatal("wrong failure origin", s.VideoFrames)
	}
	for i := 0; i < 180; i++ {
		data, e := json.Marshal(s)
		if e != nil {
			t.Fatal(e)
		}
		var restored RuntimeState
		if e = json.Unmarshal(data, &restored); e != nil {
			t.Fatal(e)
		}
		next, e := StepVideo(s, [4]Buttons{})
		if e != nil {
			t.Fatal(i, e)
		}
		replay, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(next, replay) {
			t.Fatal(i, "restore", e)
		}
		s = next.State
	}
}
