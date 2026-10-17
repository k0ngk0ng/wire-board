package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestEvasionChoicesOriginalNaturalCalls(t *testing.T) {
	file, err := os.Open("testdata/ai-evasion-choices.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
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
	if len(rows) != 2 {
		t.Fatal("incomplete natural evasion calls", len(rows))
	}
	counts := map[uint8]int{}
	for i, row := range rows {
		state := row.Before
		if err := advanceWorldComputerInput(&state, row.Actor, row.context(t)); err != nil {
			t.Fatal(i, err)
		}
		if state != row.After {
			t.Fatalf("call%d frame%d: %s", i, row.Frame, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(row.After), "state"))
		}
		counts[row.Before.Team.Players[row.Actor].PlanParameter]++
	}
	t.Logf("matched complete original natural evasion branch calls: %v", counts)
}

func TestEvasionChoicesIsolatedOriginalFunctionMatrix(t *testing.T) {
	file, err := os.Open("testdata/ai-evasion-choice-functions.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []struct {
		Choice, Actor, Action uint8
		X, Other              uint16
		After                 struct{ Action, Heading, Mode, Plan, Parameter, JumpPhase uint8 }
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1056 {
		t.Fatal("incomplete isolated evasion matrix", len(rows))
	}
	for i, r := range rows {
		initial := AIControlDecision{Control: AIControlState{Mode: 26, Entered: true, Plan: 8, PlanParameter: r.Choice}, Action: Action(r.Action), ActionEntered: true, Heading: HeadingUpRight, Random: 0x12345678}
		got := initial
		v := AIControlView{Actor: r.Actor, Action: Action(r.Action), Position: Position{X: Fixed(r.X) * Pixel}}
		v.Players[r.Actor^1] = Position{X: Fixed(r.Other) * Pixel}
		zones := [4]CourtZone{0x20, 0x20, 0x20, 0x20}
		v.CachedZones = &zones
		if err := advanceOriginalDribbleEvasion(&got, v, AIEvasionContext{Court: 1}); err != nil {
			t.Fatal(i, err)
		}
		action := uint8(got.Action)
		if got.ActionEntered {
			action |= 128
		}
		mode := uint8(got.Control.Mode)
		if got.Control.Entered {
			mode |= 128
		}
		heading := uint8(255)
		if got.Heading != HeadingNone {
			heading = uint8(got.Heading) * 32
		}
		phase := uint8(3)
		if got.ShotPlan.ChangeJumpPhase {
			phase = uint8(got.ShotPlan.JumpPhase)
		}
		a := r.After
		if action != a.Action || mode != a.Mode || heading != a.Heading || uint8(got.Control.Plan) != a.Plan || got.Control.PlanParameter != a.Parameter || phase != a.JumpPhase || got.Random != initial.Random {
			t.Fatalf("isolated case%d %+v: %+v", i, r, got)
		}
	}
	t.Log("matched all 1056 isolated original B840/B87F/B885 executions, including 16-bit wrap and strict 96 boundary")
}
