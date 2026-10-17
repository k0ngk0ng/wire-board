package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestPlayerRequestsAgainstOriginalInstructionExecution(t *testing.T) {
	b, err := os.ReadFile("testdata/player-request-acceptance.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]Action
	if err = json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 106 {
		t.Fatal(len(rows))
	}
	for requested, allowed := range rows {
		for current := Action(0); current < 106; current++ {
			accepted := false
			for _, a := range allowed {
				if a == current {
					accepted = true
				}
			}
			for _, entered := range []bool{false, true} {
				before := Player{Action: current, Entered: entered, Request: ActionRequest{Action: Action(requested), Pending: true}, Animation: AnimationState{Cursor: 3, Elapsed: 4}}
				want := before
				if accepted {
					want.Action, want.Entered, want.Request.Pending = Action(requested), false, false
				}
				got, err := ResolvePlayerRequest(before)
				if err != nil || got != want {
					t.Fatalf("%x -> %x: got %+v want %+v err %v", current, requested, got, want, err)
				}
				before.Request.Pending = false
				if got, err := ResolvePlayerRequest(before); err != nil || got != before {
					t.Fatal("processed request changed", got, err)
				}
			}
		}
	}
	for _, before := range []Player{{Action: 106, Request: ActionRequest{Pending: true}}, {Request: ActionRequest{Action: 106, Pending: true}}} {
		if got, err := ResolvePlayerRequest(before); err == nil || got != before {
			t.Fatal("invalid request domain not atomic", got, err)
		}
	}
}

func TestWorldActionsOriginalHumanBRequestDispatch(t *testing.T) {
	f, err := os.Open("testdata/world-request-control-b.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []worldActionFixture
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 58 {
		t.Fatal("missing complete action stages", len(rows))
	}
	attacks := 0
	for _, r := range rows {
		got, err := StepWorldActions(r.Before, r.context(t))
		if err != nil {
			t.Fatalf("frame %d: %v", r.Frame, err)
		}
		if got.State != r.After {
			t.Fatalf("frame %d: %s", r.Frame, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "world"))
		}
		if r.Before.Actors[0].Player.Request.Pending && r.Before.Actors[0].Player.Request.Action == ActionAttackB && r.After.Actors[0].Player.Action == ActionAttackB {
			attacks++
		}
	}
	if attacks == 0 {
		t.Fatal("no human B accepted into real attack handler")
	}
	t.Logf("all %d original human B world actions match, %d accepted B requests", len(rows), attacks)
}
