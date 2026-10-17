package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalShotPlanThreeNaturalCall(t *testing.T) {
	f, err := os.Open("testdata/ai-shot-plan3.json.gz")
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
	if len(rows) != 1 {
		t.Fatal("missing original B471 call", len(rows))
	}
	for _, row := range rows {
		state := row.Before
		if err := advanceWorldComputerInput(&state, row.Actor, row.context(t)); err != nil {
			t.Fatal(err)
		}
		if state != row.After {
			t.Fatal(firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(row.After), "state"))
		}
	}
}

func TestShotPlanThreeIsolatedOriginalMatrix(t *testing.T) {
	f, err := os.Open("testdata/ai-shot-plan3-function.json.gz")
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
		Actor, Grade, Zone, Action uint8
		Entered                    bool
		Mounts                     [6]uint8
		After                      struct{ Action, Heading, Mode, Plan, Parameter, Counter uint8 }
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5120 {
		t.Fatal("incomplete isolated B471 matrix", len(rows))
	}
	for i, r := range rows {
		zones := [4]CourtZone{}
		zones[r.Actor] = CourtZone(r.Zone)
		v := AIControlView{Actor: r.Actor, CachedZones: &zones, Action: Action(r.Action), ActionEntered: r.Entered}
		initial := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, Plan: 8, PlanParameter: 3, Counter: 7}, Action: v.Action, ActionEntered: r.Entered, Heading: HeadingUpRight, Random: 0x12345678}
		got := initial
		speed, _ := JumpVelocityForGrade(r.Grade)
		if err := advanceOriginalShotPlan(&got, v, AIShotContext{JumpVelocity: speed, MountFlags: &r.Mounts}); err != nil {
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
		a := r.After
		if action != a.Action || mode != a.Mode || heading != a.Heading || uint8(got.Control.Plan) != a.Plan || got.Control.PlanParameter != a.Parameter || uint8(got.Control.Counter) != a.Counter || got.Random != initial.Random {
			t.Fatalf("isolated B471 case%d %+v: %+v", i, r, got)
		}
	}
	t.Log("matched 5120 isolated original B471 cases with actor-specific mount, grade, zone and action-entry boundaries")
}

func TestShotPlanThreeOnlyReadsNeededContext(t *testing.T) {
	initial := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 3}}
	got := initial
	if err := advanceOriginalShotPlan(&got, AIControlView{}, AIShotContext{}); err == nil || got != initial {
		t.Fatal("unknown mounts accepted or error mutated state", got, err)
	}
	mounts := [6]uint8{}
	if err := advanceOriginalShotPlan(&got, AIControlView{}, AIShotContext{MountFlags: &mounts}); err != nil || got.Control.PlanParameter != 2 {
		t.Fatal("absent mount requested unused jump profile", got, err)
	}
	got = initial
	mounts[1] = 1
	if err := advanceOriginalShotPlan(&got, AIControlView{}, AIShotContext{MountFlags: &mounts}); err == nil || got != initial {
		t.Fatal("needed jump profile defaulted", got, err)
	}
}
