package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestAICommandResumeOriginalNaturalCalls(t *testing.T) {
	f, err := os.Open("testdata/ai-command-resume.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		worldInputFixture
		Actor uint8
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 43 {
		t.Fatal("incomplete original A8C1 calls", len(rows))
	}
	for _, r := range rows {
		state := r.Before
		if err := advanceWorldComputerInput(&state, r.Actor, r.context(t)); err != nil {
			t.Fatal(r.Frame, r.Actor, err)
		}
		if state != r.After {
			t.Fatalf("frame%d actor%d: %s", r.Frame, r.Actor, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(r.After), "state"))
		}
	}
	t.Log("matched 43 original complete A8C1 calls with canonical command-status writes")
}

func TestAICommandResumeIsolatedOriginalMatrix(t *testing.T) {
	f, err := os.Open("testdata/ai-command-resume-function.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Actor, Tactic, Holder, Mode uint8
		Entered                     bool
		Statuses, AfterStatuses     [4]uint8
		AfterMode                   uint8
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2016 {
		t.Fatal("incomplete isolated A8C1 matrix", len(rows))
	}
	for i, r := range rows {
		var state WorldInputState
		state.World.Holder = int8(r.Holder)
		state.World.Actors[r.Actor].Player.Action = ActionDribbleIdle
		state.World.Actors[r.Actor].Player.Entered = true
		state.Team.Players[r.Actor] = AIControlState{Mode: AIMode(r.Mode), Entered: r.Entered, Plan: 17, Counter: 7, PlanParameter: 6, Destination: 32, TrackedObject: 2}
		for team := range state.Human {
			state.Human[team].EvenStatus = int8(r.Statuses[team*2])
			state.Human[team].Status = int8(r.Statuses[team*2+1])
		}
		want := state
		want.Team.Players[r.Actor].Mode = AIMode(r.AfterMode & 127)
		want.Team.Players[r.Actor].Entered = r.AfterMode&128 != 0
		for team := range want.Human {
			want.Human[team].EvenStatus = int8(r.AfterStatuses[team*2])
			want.Human[team].Status = int8(r.AfterStatuses[team*2+1])
		}
		var ctx WorldInputContext
		ctx.World.Controls = [4]uint8{128, 128, 128, 128}
		ctx.HumanTactics[r.Actor/2] = r.Tactic
		for j := range ctx.World.Profiles {
			ctx.World.Profiles[j] = KunioProfile()
		}
		if err := advanceWorldComputerInput(&state, r.Actor, ctx); err != nil {
			t.Fatal(i, err)
		}
		if state != want {
			t.Fatalf("isolated A8C1 case%d %+v: %s", i, r, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(want), "state"))
		}
	}
	t.Log("matched 2016 isolated original mode21/22 executions; actor-index behavior is not replaced by team indexing")
}
