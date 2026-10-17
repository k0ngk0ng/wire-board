package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalActionSchedule(t *testing.T) {
	data, err := os.ReadFile("testdata/action-schedule.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Frame         int
		Before, After [7]uint8
		Calls         []struct {
			Actor         uint8
			Before, After [7]uint8
		}
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1998 {
		t.Fatal("missing original world schedules", len(rows))
	}
	calls, revisits, reordered := 0, 0, 0
	for _, r := range rows {
		var scheduler ActionScheduler
		flags := r.Before
		seen := [13]bool{}
		previous := uint8(0)
		for index, call := range r.Calls {
			actor, before, ok := scheduler.Next(flags)
			if !ok || actor != call.Actor || before != call.Before {
				t.Fatalf("frame %d call %d got actor=%d flags=%v ok=%v want actor=%d flags=%v", r.Frame, index, actor, before, ok, call.Actor, call.Before)
			}
			if seen[actor] {
				revisits++
			}
			seen[actor] = true
			if index > 0 && actor < previous {
				reordered++
			}
			previous = actor
			// Only the action's link/processed feedback crosses this scheduler boundary.
			// The test does not claim to execute its positions, attacks or ball physics.
			flags = call.After
			calls++
		}
		_, final, ok := scheduler.Next(flags)
		if ok || final != r.After {
			t.Fatalf("frame %d final flags %v want %v ok=%v", r.Frame, final, r.After, ok)
		}
	}
	if calls != 26093 || revisits != 119 || reordered != 0 {
		t.Fatal(calls, revisits, reordered)
	}
	t.Logf("matched 1998 original schedules / %d actions, %d repeated objects, %d descending-order transitions", calls, revisits, reordered)
}

func TestActionSchedulerRevisitsAndLinkedPriority(t *testing.T) {
	var scheduler ActionScheduler
	links := [7]uint8{0x81, 0, 0, 0, 0, 0, 0}
	expected := []uint8{1, 0, 0, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	for index, want := range expected {
		actor, updated, ok := scheduler.Next(links)
		if !ok || actor != want {
			t.Fatal(index, actor, ok, want)
		}
		links = updated
		if index == 1 {
			links[0] = 0
		} // The action requests its original same-step revisit.
		if actor == 6 {
			links[1] = 0
		} // Tail processing must not restart the actor search.
	}
	_, _, ok := scheduler.Next(links)
	if ok {
		t.Fatal("scheduler repeated completed tail")
	}
}
