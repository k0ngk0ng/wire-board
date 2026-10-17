package engine

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestLowJumpShotOriginalCallsAndChains(t *testing.T) {
	var rows struct {
		Calls  []launchedPlayerFixture
		Chains [][]launchedPlayerFixture
	}
	readPropAttackFixture(t, "testdata/low-jump-shot.json.gz", &rows)
	if len(rows.Calls) != 336 || len(rows.Chains) != 8 {
		t.Fatal("incomplete original low jump fixture")
	}
	run := func(row launchedPlayerFixture, s WorldActionState) WorldActionState {
		t.Helper()
		r := WorldActionStep{State: s}
		c := row.context(t)
		c.ActorHeldButtons = row.Held
		court, e := originalWorldMotionCourt(&r, c, row.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = advanceWorldPlayer(&r, row.Actor, c, court); e != nil {
			t.Fatal(e)
		}
		if r.State != row.After {
			t.Fatal(firstWorldLoopDifference(reflect.ValueOf(r.State), reflect.ValueOf(row.After), "world"))
		}
		if !slices.Equal(r.SoundIDs, row.Sounds) {
			t.Fatal("sounds differ", r.SoundIDs, row.Sounds)
		}
		return r.State
	}
	for _, r := range rows.Calls {
		run(r, r.Before)
	}
	for i, chain := range rows.Chains {
		s := chain[0].Before
		for j, r := range chain {
			if s != r.Before {
				t.Fatal("continuous original low jump mismatch", i, j)
			}
			data, e := json.Marshal(s)
			if e != nil {
				t.Fatal(e)
			}
			var restored WorldActionState
			if e = json.Unmarshal(data, &restored); e != nil {
				t.Fatal(e)
			}
			next := run(r, s)
			if run(r, restored) != next {
				t.Fatal("JSON replay mismatch")
			}
			s = next
		}
	}
	t.Log("336 original calls and 8 continuous chains / 368 calls matched")
}

func TestLowJumpShotRealTeamFourContinuation(t *testing.T) {
	var s RuntimeState
	readPropAttackFixture(t, "testdata/low-jump-shot-runtime-before.json.gz", &s)
	if s.VideoFrames != 3611 {
		t.Fatal("wrong first failure fixture")
	}
	for tick := 0; tick < 120; tick++ {
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
			t.Fatalf("video %d: %v", s.VideoFrames, e)
		}
		replay, e := StepVideo(restored, [4]Buttons{})
		if e != nil || !reflect.DeepEqual(next, replay) {
			t.Fatal("restore drift", e)
		}
		s = next.State
	}
}
