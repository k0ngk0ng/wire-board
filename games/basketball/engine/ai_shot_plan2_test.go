package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalAIShotPlanTwoCompleteCalls(t *testing.T) {
	f, err := os.Open("testdata/ai-shot-plan2.json.gz")
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
	if len(rows) != 57 {
		t.Fatal("incomplete original plan-two calls", len(rows))
	}
	for i, row := range rows {
		state := row.Before
		if err := advanceWorldComputerInput(&state, row.Actor, row.context(t)); err != nil {
			t.Fatalf("call %d frame %d actor %d: %v", i, row.Frame, row.Actor, err)
		}
		if state != row.After {
			t.Fatalf("call %d frame %d: %s", i, row.Frame, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(row.After), "state"))
		}
	}
	t.Log("all 57 original plan-two calls match complete WorldInputState, including both jumps, aim, shot cycle, RNG and unchanged world")
}

func TestShotPlanTwoUnsignedSpeedAndPhaseBoundaries(t *testing.T) {
	for _, phase := range []int8{-1, 0} {
		for _, air := range []bool{false, true} {
			for _, speed := range []int8{-128, -1, 0, 1, 2, 3, 127} {
				initial := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 2, Plan: 5}, Action: ActionCarrierJump, ActionEntered: true, Heading: HeadingNone, Random: 0x12345678}
				got := initial
				v := AIControlView{Actor: 2, Action: ActionCarrierJump, HasBall: true, Airborne: air}
				if err := advanceOriginalShotPlan(&got, v, AIShotContext{JumpPhase: phase, VerticalSpeed: speed}); err != nil {
					t.Fatal(err)
				}
				if got.Random != initial.Random || got.Control != initial.Control {
					t.Fatal("jump stage changed RNG or plan", phase, air, speed, got)
				}
				if phase < 0 && uint8(speed) < 3 {
					if got.Action != ActionAimAirShot || got.ActionEntered || got.ShotPlan.ChangeJumpPhase {
						t.Fatal("second jump aim gate", phase, air, speed, got)
					}
				} else if phase >= 0 && air && uint8(speed) < 2 {
					if got.Action != ActionCarrierJump || got.ActionEntered || !got.ShotPlan.ChangeJumpPhase || got.ShotPlan.JumpPhase != -1 {
						t.Fatal("second jump command", phase, air, speed, got)
					}
				} else if got != initial {
					t.Fatal("unmet gate changed state", phase, air, speed, got)
				}
			}
		}
	}
}

func TestShotPlanTwoInitialJumpAndApexRelease(t *testing.T) {
	initial := AIControlDecision{Control: AIControlState{Mode: 6, Entered: true, PlanParameter: 2, Plan: 5}, Action: ActionDribbleRun, ActionEntered: true, Heading: HeadingRight, Random: 0x12345678}
	got := initial
	v := AIControlView{Actor: 2, Action: ActionDribbleRun, HasBall: true}
	if err := advanceOriginalShotPlan(&got, v, AIShotContext{JumpPhase: -1}); err != nil {
		t.Fatal(err)
	}
	if got.Action != ActionCarrierJump || got.ActionEntered || got.Heading != HeadingNone || !got.ShotPlan.ChangeJumpPhase || got.ShotPlan.JumpPhase != 0 || got.Random != initial.Random {
		t.Fatal(got)
	}
	initial.Action = ActionAimAirShot
	for _, speed := range []int8{-1, 0, 1, 2} {
		got = initial
		v.Action = ActionAimAirShot
		if err := advanceOriginalShotPlan(&got, v, AIShotContext{VerticalSpeed: speed}); err != nil {
			t.Fatal(err)
		}
		if speed != 0 {
			if got != initial {
				t.Fatal("nonzero speed released", speed, got)
			}
			continue
		}
		random, _ := NextRandom(initial.Random)
		if got.Random != random || got.Action != ActionAirShoot || got.ActionEntered || got.Control.Mode != AIPossessionWait || got.Control.Entered || got.Control.Plan != 5 || got.Heading != HeadingLeft || !got.ShotPlan.ChangeShotCycle {
			t.Fatal("apex release", got)
		}
	}
}
