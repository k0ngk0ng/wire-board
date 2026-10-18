package engine

import (
	"reflect"
	"testing"
)

type evasionDirectCase struct {
	Actor, Parameter, Counter, Zone, Action, Heading, JumpPhase, VerticalSpeed uint8
	Random                                                                     RandomState
	After                                                                      struct {
		Parameter, Counter, Action, Heading, JumpPhase, Mode, Plan uint8
		Random                                                     RandomState
	}
}

func evasionDirectInput(r evasionDirectCase) (AIControlDecision, AIControlView) {
	zones := [4]CourtZone{}
	zones[r.Actor] = CourtZone(r.Zone)
	heading := Heading(r.Heading / 32)
	if r.Heading == 255 {
		heading = HeadingNone
	}
	v := AIControlView{Actor: r.Actor, Action: Action(r.Action & 127), ActionEntered: r.Action&128 != 0, Heading: heading, Random: r.Random, CachedZones: &zones, ShotContext: AIShotContext{JumpPhase: int8(r.JumpPhase), VerticalSpeed: int8(r.VerticalSpeed)}}
	result := AIControlDecision{Control: AIControlState{Mode: 26, Entered: true, Plan: 8, Counter: int8(r.Counter), PlanParameter: r.Parameter}, Action: v.Action, ActionEntered: v.ActionEntered, Heading: heading, Random: r.Random, BindHoop: -1}
	return result, v
}
func TestOriginalEvasionDirectPlans(t *testing.T) {
	var f struct {
		Calls              []evasionDirectCase
		UnsupportedHeading []evasionDirectCase
	}
	readPropAttackFixture(t, "testdata/ai-evasion-direct.json.gz", &f)
	if len(f.Calls) != 10752 || len(f.UnsupportedHeading) != 4 {
		t.Fatal("missing direct source calls", len(f.Calls), len(f.UnsupportedHeading))
	}
	for i, r := range f.Calls {
		result, view := evasionDirectInput(r)
		if e := advanceOriginalDribbleEvasion(&result, view, AIEvasionContext{Court: 1}); e != nil {
			t.Fatal(i, r.Parameter, e)
		}
		got := r.After
		got.Parameter = result.Control.PlanParameter
		got.Counter = uint8(result.Control.Counter)
		got.Action = uint8(result.Action)
		if result.ActionEntered {
			got.Action |= 128
		}
		got.Heading = 255
		if result.Heading != HeadingNone {
			got.Heading = uint8(result.Heading) * 32
		}
		got.JumpPhase = r.JumpPhase
		if result.ShotPlan.ChangeJumpPhase {
			got.JumpPhase = uint8(result.ShotPlan.JumpPhase)
		}
		got.Mode = uint8(result.Control.Mode)
		if result.Control.Entered {
			got.Mode |= 128
		}
		got.Plan = uint8(result.Control.Plan)
		got.Random = result.Random
		if got != r.After {
			t.Fatalf("call%d plan%d counter%d: got%+v want%+v", i, r.Parameter, r.Counter, got, r.After)
		}
	}
	for _, r := range f.UnsupportedHeading {
		result, v := evasionDirectInput(r)
		initial := result
		if r.After.Heading != 127 {
			t.Fatal("source does not prove raw heading alias")
		}
		if e := advanceOriginalDribbleEvasion(&result, v, AIEvasionContext{Court: 1}); e == nil || !reflect.DeepEqual(initial, result) {
			t.Fatal("unsupported heading normalized or partially committed", e)
		}
	}
}

// Fixed external context, evolving source AI state only: no injection of next
// actions, jump phase, RNG or counters. Plan6 intentionally keeps waiting while
// its dodge animation has not run; these are not whole-match chains.
func TestOriginalEvasionDirectContinuousCalls(t *testing.T) {
	var f struct {
		Chains []struct {
			Actor, Parameter uint8
			Records          []evasionDirectCase
		}
	}
	readPropAttackFixture(t, "testdata/ai-evasion-direct.json.gz", &f)
	if len(f.Chains) != 28 {
		t.Fatal("missing direct continuous calls", len(f.Chains))
	}
	for ci, chain := range f.Chains {
		state, v := evasionDirectInput(chain.Records[0])
		for tick, r := range chain.Records {
			state.ShotPlan = AIShotCommand{}
			if e := advanceOriginalDribbleEvasion(&state, v, AIEvasionContext{Court: 1}); e != nil {
				t.Fatal(ci, tick, e)
			}
			action := uint8(state.Action)
			if state.ActionEntered {
				action |= 128
			}
			heading := uint8(255)
			if state.Heading != HeadingNone {
				heading = uint8(state.Heading) * 32
			}
			phase := uint8(v.ShotContext.JumpPhase)
			if state.ShotPlan.ChangeJumpPhase {
				phase = uint8(state.ShotPlan.JumpPhase)
			}
			mode := uint8(state.Control.Mode)
			if state.Control.Entered {
				mode |= 128
			}
			if action != r.After.Action || heading != r.After.Heading || phase != r.After.JumpPhase || mode != r.After.Mode || state.Control.PlanParameter != r.After.Parameter || uint8(state.Control.Counter) != r.After.Counter || uint8(state.Control.Plan) != r.After.Plan || state.Random != r.After.Random {
				t.Fatalf("chain%d tick%d state%+v expected%+v", ci, tick, state, r.After)
			}
			v.Action, v.ActionEntered, v.Heading, v.Random = state.Action, state.ActionEntered, state.Heading, state.Random
			v.ShotContext.JumpPhase = int8(phase)
		}
	}
}
