package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"
)

// Failure inventory in verify_rider_launch.py precedes implementation. This
// compares full actor3F transactions and uninterrupted two-actor support/rider
// calls, including target position, arc, attachment, sounds and JSON restores.
func TestOriginalRiderLaunchAndCoupledLiftChain(t *testing.T) {
	f, e := os.Open("testdata/rider-launch.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	var rows struct {
		Calls  []capturedOracleCall
		Chains [][]capturedOracleCall
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows.Calls) != 288 || len(rows.Chains) != 4 {
		t.Fatal("incomplete original lift fixture")
	}
	run := func(row capturedOracleCall, s WorldActionState) WorldActionState {
		t.Helper()
		r := WorldActionStep{State: s}
		c := row.context(t)
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
			t.Fatal("lift sounds differ", r.SoundIDs, row.Sounds)
		}
		return r.State
	}
	for _, r := range rows.Calls {
		run(r, r.Before)
	}
	for _, chain := range rows.Chains {
		s := chain[0].Before
		for _, r := range chain {
			if s != r.Before {
				t.Fatal("original lift continuous input diverged")
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
				t.Fatal("lift restore diverged")
			}
			s = next
		}
	}
	t.Log("matched 288 original actor3F calls and 4 coupled support07/rider3F chains (384 calls), including JSON replay")
}
