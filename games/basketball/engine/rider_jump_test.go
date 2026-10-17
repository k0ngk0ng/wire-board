package engine

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

// Failure inventory is also retained in verify_rider_jump.py: accepted jump
// refused, stale attachment during motion, wrong support mutation, lost fractional
// positions, negative jump-phase dispatch, and continuous landing/restore drift.
func TestOriginalRiderJumpDetachesBeforeMotion(t *testing.T) {
	f, e := os.Open("testdata/rider-jump.json.gz")
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	z, e := gzip.NewReader(f)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	type call struct {
		worldActionFixture
		Actor             uint8
		Sounds            []uint8
		MotionAttachments []uint8
	}
	var rows struct {
		Calls  []call
		Riders []call
		Runs   []call
		Chains [][]call
	}
	if e = json.NewDecoder(z).Decode(&rows); e != nil {
		t.Fatal(e)
	}
	if len(rows.Calls) != 288 || len(rows.Riders) != 72 || len(rows.Runs) != 48 || len(rows.Chains) != 8 {
		t.Fatal("incomplete rider jump oracle")
	}
	run := func(t *testing.T, row call, s WorldActionState) WorldActionState {
		t.Helper()
		r := WorldActionStep{State: s}
		c := row.context(t)
		court, e := originalWorldMotionCourt(&r, c, row.Actor)
		if e != nil {
			t.Fatal(e)
		}
		var attachments []uint8
		load := court.originalMotion.load
		court.originalMotion.load = func() CourtFloorState {
			attachments = append(attachments, r.State.PlayerLinks[row.Actor])
			return load()
		}
		if e = advanceWorldPlayer(&r, row.Actor, c, court); e != nil {
			t.Fatal(e)
		}
		if (row.Before.Actors[row.Actor].Player.Action != ActionRiding || row.Before.Actors[row.Actor].Player.Request.Pending) && !slices.Equal(attachments, row.MotionAttachments) {
			t.Fatalf("motion observed attachment %v, original %v", attachments, row.MotionAttachments)
		}
		if r.State != row.After {
			t.Fatal(firstWorldLoopDifference(reflect.ValueOf(r.State), reflect.ValueOf(row.After), "world"))
		}
		if !slices.Equal(r.SoundIDs, row.Sounds) {
			t.Fatal("rider jump sounds differ", r.SoundIDs, row.Sounds)
		}
		return r.State
	}
	for i, row := range rows.Calls {
		t.Run(fmt.Sprint(i), func(t *testing.T) { run(t, row, row.Before) })
	}
	for i, row := range rows.Riders {
		t.Run(fmt.Sprintf("support%d", i), func(t *testing.T) { run(t, row, row.Before) })
	}
	for i, row := range rows.Runs {
		t.Run(fmt.Sprintf("run%d", i), func(t *testing.T) { run(t, row, row.Before) })
	}
	for i, chain := range rows.Chains {
		s := chain[0].Before
		for j, row := range chain {
			if s != row.Before {
				t.Fatal("continuous original rider jump diverged", i, j)
			}
			encoded, e := json.Marshal(s)
			if e != nil {
				t.Fatal(e)
			}
			var restored WorldActionState
			if e = json.Unmarshal(encoded, &restored); e != nil {
				t.Fatal(e)
			}
			next := run(t, row, s)
			if run(t, row, restored) != next {
				t.Fatal("restored rider jump diverged")
			}
			s = next
		}
	}
	t.Log("matched 288 original rider jump calls, 72 original support calls, 48 attached run/fall calls and 8 uninterrupted jump-to-landing chains (136 calls), including JSON replay")
}
