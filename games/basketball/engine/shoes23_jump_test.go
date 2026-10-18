package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"
)

// Failure inventory precedes implementation in SHOES23_JUMP_VERIFICATION.md.
func TestShoes23CompleteOriginalJumpHandlers(t *testing.T) {
	type row struct {
		worldActionFixture
		Label                            string
		Actor                            uint8
		Shoes                            [4]uint8
		Sounds, ShoeCalls, MotionObjects []uint8
	}
	var data struct {
		Calls  []row
		Chains [][]row
	}
	readPropAttackFixture(t, "testdata/shoes23-jump.json.gz", &data)
	if len(data.Calls) == 0 || len(data.Chains) != 132 {
		t.Fatal("missing shoe23 original calls")
	}
	run := func(t *testing.T, r row, before WorldActionState) WorldActionState {
		t.Helper()
		c := r.context(t)
		c.JumpEnhancements = &r.Shoes
		got := WorldActionStep{State: before}
		court, e := originalWorldMotionCourt(&got, c, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = advanceWorldPlayer(&got, r.Actor, c, court); e != nil {
			t.Fatalf("%s: %v", r.Label, e)
		}
		if got.State != r.After {
			t.Fatalf("%s: %s", r.Label, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(got.SoundIDs, r.Sounds) {
			t.Fatalf("%s sounds got%v want%v", r.Label, got.SoundIDs, r.Sounds)
		}
		return got.State
	}
	for _, r := range data.Calls {
		t.Run(r.Label, func(t *testing.T) { run(t, r, r.Before) })
	}
	for i, chain := range data.Chains {
		t.Run(fmt.Sprintf("chain%d", i), func(t *testing.T) {
			state := chain[0].Before
			for _, r := range chain {
				var restored WorldActionState
				b, e := json.Marshal(state)
				if e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(b, &restored); e != nil {
					t.Fatal(e)
				}
				state = run(t, r, state)
				if replay := run(t, r, restored); replay != state {
					t.Fatal("restore drift")
				}
			}
		})
	}
}
