package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestAIMode31IsolatedOriginalMatrix(t *testing.T) {
	f, err := os.Open("testdata/ai-mode31-function.json.gz")
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
		Parameter, Actor, Action, Difficulty uint8
		Horizontal, Vertical, Phase          int8
		Airborne                             bool
		Entered, Human                       bool
		Seed                                 RandomState
		After                                struct {
			Mode, Plan, Counter, Parameter, Destination, Target, Action, Heading, Cycle, Phase, NextAction uint8
			Random                                                                                         RandomState
		}
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 16896 {
		t.Fatal("incomplete isolated AB87 matrix", len(rows))
	}
	for i, r := range rows {
		control := AIControlState{Mode: AIMode(31), Entered: r.Entered, Plan: 8, Counter: 9, PlanParameter: r.Parameter, Destination: 32, TrackedObject: 2}
		speed, _ := JumpVelocityForGrade(3)
		zones := [4]CourtZone{}
		zones[r.Actor] = CourtZone(0x52)
		v := AIControlView{Actor: r.Actor, Action: Action(r.Action), ActionEntered: true, Heading: HeadingUpRight, HasBall: true, Airborne: r.Airborne, Random: r.Seed, Difficulty: r.Difficulty, TeamHasHuman: r.Human, CachedZones: &zones, ShotContext: AIShotContext{JumpVelocity: speed, Character: 1, ShotSkill: 3, VerticalSpeed: r.Vertical, HorizontalSpeed: r.Horizontal, JumpPhase: r.Phase}}
		got, err := StepOriginalAIControl(control, v)
		if err != nil {
			t.Fatal(i, err)
		}
		applied := ApplyOriginalAIActorDecision(ComputerActorState{Carrier: HoopCarrier{ShotCycle: 8}, JumpPhase: r.Phase}, got)
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
			t.Fatalf("isolated AB87 case%d %+v: got %+v applied %+v", i, r, got, applied)
		}
	}
	t.Log("matched all 16896 original isolated AB87/B383 mode31 entry and persisted-plan executions")
}
