package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestAIForcedShotOriginalNaturalCalls(t *testing.T) {
	f, err := os.Open("testdata/ai-forced-shot.json.gz")
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
	if len(rows) != 10 {
		t.Fatal("incomplete original A49E calls", len(rows))
	}
	for _, r := range rows {
		state := r.Before
		if err := advanceWorldComputerInput(&state, r.Actor, r.context(t)); err != nil {
			t.Fatal(r.Frame, r.Actor, err)
		}
		if state != r.After {
			t.Fatal(r.Frame, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(r.After), "state"))
		}
	}
	t.Log("matched all 10 original A49E calls with complete world input state")
}

func TestAIForcedShotIsolatedOriginalMatrix(t *testing.T) {
	f, err := os.Open("testdata/ai-forced-shot-function.json.gz")
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
		Mode, Parameter, Actor, Action, Zone, Grade uint8
		Entered, Human                              bool
		Seed                                        RandomState
		After                                       struct {
			Mode, Plan, Counter, Parameter, Destination, Target, Action, Heading, Cycle, Phase, NextAction uint8
			Random                                                                                         RandomState
		}
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1536 {
		t.Fatal("incomplete isolated A49E matrix", len(rows))
	}
	for i, r := range rows {
		control := AIControlState{Mode: AIMode(r.Mode), Entered: r.Entered, Plan: 8, Counter: 9, PlanParameter: r.Parameter, Destination: 32, TrackedObject: 2}
		speed, _ := JumpVelocityForGrade(r.Grade)
		zones := [4]CourtZone{}
		zones[r.Actor] = CourtZone(r.Zone)
		v := AIControlView{Actor: r.Actor, Action: Action(r.Action), ActionEntered: true, Heading: HeadingUpRight, HasBall: true, Airborne: true, Random: r.Seed, TeamHasHuman: r.Human, CachedZones: &zones, ShotContext: AIShotContext{JumpVelocity: speed, Character: 1, ShotSkill: 3, VerticalSpeed: 1}}
		got, err := StepOriginalAIControl(control, v)
		if err != nil {
			t.Fatal(i, err)
		}
		applied := ApplyOriginalAIActorDecision(ComputerActorState{Carrier: HoopCarrier{ShotCycle: 8}}, got)
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
		next := uint8(255)
		if applied.Carrier.Player.Animation.HasNextAction {
			next = uint8(applied.Carrier.Player.Animation.NextAction)
		}
		a := r.After
		if action != a.Action || mode != a.Mode || uint8(got.Control.Plan) != a.Plan || uint8(got.Control.Counter) != a.Counter || got.Control.PlanParameter != a.Parameter || uint8(got.Control.Destination) != a.Destination || uint8(got.Control.TrackedObject) != a.Target || heading != a.Heading || uint8(applied.Carrier.ShotCycle) != a.Cycle || uint8(applied.JumpPhase) != a.Phase || next != a.NextAction || got.Random != a.Random {
			t.Fatalf("isolated A49E case%d %+v: got %+v applied %+v", i, r, got, applied)
		}
	}
	t.Log("matched all 1536 original isolated A49E/B383 forced-shot entry and persisted-plan executions")
}
