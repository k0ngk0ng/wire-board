package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"testing"
)

func TestShotPlanSevenRunZoneAndGradeGate(t *testing.T) {
	for _, grade := range []uint8{3, 7} {
		for _, zone := range []CourtZone{0x50, 0x60} {
			zones := [4]CourtZone{0, 0, 0, zone}
			v := AIControlView{Actor: 3, Action: ActionDribbleRun, HasBall: true, CachedZones: &zones}
			got := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 7, Plan: 17}, Action: ActionDribbleRun, ActionEntered: true, Random: 0x12345678}
			speed, _ := JumpVelocityForGrade(grade)
			if err := advanceOriginalShotPlan(&got, v, AIShotContext{JumpVelocity: speed}); err != nil {
				t.Fatal(err)
			}
			if got.Action != ActionDribbleRun || !got.ActionEntered || got.Control.Destination != 0x60 || got.Random != 0x12345678 {
				t.Fatal(got)
			}
			if zone == 0x60 {
				if got.Heading != HeadingNone || got.Control.Mode != 18 || got.Control.Entered || got.Control.Plan != 22 {
					t.Fatal("arrival", got)
				}
			} else if got.Heading != HeadingLeft || got.Control.Mode != 6 || got.Control.Plan != 17 {
				t.Fatal("approach", got)
			}
		}
	}
	initial := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 7}}
	got := initial
	if err := advanceOriginalShotPlan(&got, AIControlView{}, AIShotContext{}); err == nil || got != initial {
		t.Fatal("unknown jump profile silently defaulted", got, err)
	}
}

func TestShotPlanSevenLowJumpRouteAndLandCommand(t *testing.T) {
	speed, _ := JumpVelocityForGrade(2)
	for _, zone := range []CourtZone{0x52, 0x62} {
		zones := [4]CourtZone{zone}
		v := AIControlView{Actor: 0, Action: ActionDribbleRun, CachedZones: &zones}
		got := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 7}, Action: ActionDribbleRun, ActionEntered: true}
		if err := advanceOriginalShotPlan(&got, v, AIShotContext{JumpVelocity: speed}); err != nil {
			t.Fatal(err)
		}
		if got.Control.Plan != 22 {
			t.Fatal("B4F8 must overwrite next plan on every low route", got)
		}
		if zone == 0x62 {
			if got.Action != ActionCarrierJump || got.ActionEntered || got.Heading != HeadingNone {
				t.Fatal(got)
			}
		} else if got.Action != ActionDribbleRun || got.Control.Destination != 0x62 || got.Heading != HeadingRight {
			t.Fatal(got)
		}
	}
	for _, character := range []uint8{0, 1} {
		for _, human := range []bool{false, true} {
			v := AIControlView{Actor: 0, Action: ActionCarrierLand, ActionEntered: true, TeamHasHuman: human}
			got := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 7}, Action: ActionCarrierLand, ActionEntered: true, Random: 0x12345678}
			if err := advanceOriginalShotPlan(&got, v, AIShotContext{JumpVelocity: speed, Character: character}); err != nil {
				t.Fatal(err)
			}
			next := Action(0x5a)
			if character == 1 {
				next = 0x5c
			}
			wantCycle := int8(0)
			wantRandom, _ := NextRandom(0x12345678)
			if human {
				wantCycle = 0
				wantRandom = 0x12345678
			}
			if !got.ShotPlan.ChangeNextAction || got.ShotPlan.NextAction != next || !got.ShotPlan.ChangeShotCycle || got.ShotPlan.ShotCycle != wantCycle || got.Random != wantRandom || got.Control.Mode != 1 || got.Control.Entered || got.Control.Plan != 22 {
				t.Fatal(character, human, got)
			}
			applied := ApplyOriginalAIActorDecision(ComputerActorState{}, got)
			if !applied.Carrier.Player.Animation.HasNextAction || applied.Carrier.Player.Animation.NextAction != next || applied.Carrier.ShotCycle != wantCycle {
				t.Fatal("low shot commands not applied", applied)
			}
		}
	}
}

func TestShotPlanSevenIsolatedOriginalFunctionMatrix(t *testing.T) {
	f, err := os.Open("testdata/ai-shot-plan7-function.json.gz")
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
		Actor, Grade, Zone, Action, Character uint8
		Entered, Human                        bool
		Seed                                  RandomState
		After                                 struct {
			Action, Heading, Mode, NextMode, Counter, Parameter, Destination, Target, NextAction, ShotCycle, JumpPhase uint8
			Random                                                                                                     RandomState
		}
	}
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8192 {
		t.Fatal("incomplete isolated function matrix", len(rows))
	}
	for i, r := range rows {
		zones := [4]CourtZone{}
		zones[r.Actor] = CourtZone(r.Zone)
		v := AIControlView{Actor: r.Actor, Action: Action(r.Action), ActionEntered: r.Entered, TeamHasHuman: r.Human, CachedZones: &zones}
		got := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, Plan: 17, Counter: 9, PlanParameter: 7, Destination: 32, TrackedObject: 2}, Action: v.Action, ActionEntered: r.Entered, Heading: HeadingRight, Random: r.Seed}
		speed, _ := JumpVelocityForGrade(r.Grade)
		if err := advanceOriginalShotPlan(&got, v, AIShotContext{JumpVelocity: speed, Character: r.Character, ShotSkill: 3}); err != nil {
			t.Fatal(i, err)
		}
		applied := ApplyOriginalAIActorDecision(ComputerActorState{Carrier: HoopCarrier{ShotCycle: 8}}, got)
		a := r.After
		rawAction := uint8(got.Action)
		if got.ActionEntered {
			rawAction |= 128
		}
		rawMode := uint8(got.Control.Mode)
		if got.Control.Entered {
			rawMode |= 128
		}
		heading := uint8(255)
		if got.Heading != HeadingNone {
			heading = uint8(got.Heading) * 32
		}
		next := uint8(255)
		if applied.Carrier.Player.Animation.HasNextAction {
			next = uint8(applied.Carrier.Player.Animation.NextAction)
		}
		if rawAction != a.Action || rawMode != a.Mode || uint8(got.Control.Plan) != a.NextMode || uint8(got.Control.Counter) != a.Counter || got.Control.PlanParameter != a.Parameter || uint8(got.Control.Destination) != a.Destination || uint8(got.Control.TrackedObject) != a.Target || heading != a.Heading || next != a.NextAction || uint8(applied.Carrier.ShotCycle) != a.ShotCycle || uint8(applied.JumpPhase) != a.JumpPhase || got.Random != a.Random {
			t.Fatalf("isolated original case %d %+v: got %+v applied %+v", i, r, got, applied)
		}
	}
	t.Log("matched 8192 isolated original B4EE executions; not natural/full-match evidence")
}
