package engine

import (
	"reflect"
	"slices"
	"testing"
)

type hoopSharedOracle struct {
	worldActionFixture
	Held, Pressed [4]Buttons
	ResetBefore   WorldActionState
	Actor         uint8
	Sounds        []uint8
}
type hoopSharedFixture struct {
	Calls, Phases []hoopSharedOracle
	Chains        []struct {
		Action  Action
		Index   int
		Left    bool
		Records []hoopSharedOracle
	}
}

func (r hoopSharedOracle) context(t *testing.T) WorldActionContext {
	c := r.worldActionFixture.context(t)
	c.ActorHeldButtons = r.Held
	c.ActorPressedButtons = &r.Pressed
	return c
}

func hoopSharedRows(t *testing.T) hoopSharedFixture {
	t.Helper()
	var f hoopSharedFixture
	readPropAttackFixture(t, "testdata/hoop-shared-original.json.gz", &f)
	if len(f.Calls) != 1632 || len(f.Phases) != 440 || len(f.Chains) != 24 {
		t.Fatal("missing original coverage", len(f.Calls), len(f.Phases), len(f.Chains))
	}
	return f
}
func TestHoopSharedOriginalCalls(t *testing.T) {
	for i, r := range hoopSharedRows(t).Calls {
		result := WorldActionStep{State: r.Before}
		court, err := originalWorldMotionCourt(&result, r.context(t), r.Actor)
		if err != nil {
			t.Fatal(i, err)
		}
		if r.Actor < 4 {
			err = advanceWorldPlayer(&result, r.Actor, r.context(t), court)
		} else {
			err = advanceWorldAuxiliary(&result, r.Actor, r.context(t), court)
		}
		if err != nil {
			t.Fatal(i, err)
		}
		if result.State != r.After {
			t.Fatal(i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
		}
		if !slices.Equal(result.SoundIDs, r.Sounds) {
			t.Fatal(i, "sounds", result.SoundIDs, r.Sounds)
		}
	}
}
func assertHoopSharedPhase(t *testing.T, r hoopSharedOracle) WorldActionState {
	t.Helper()
	result, err := StepWorldActions(r.Before, r.context(t))
	if err != nil {
		t.Fatal(err)
	}
	if result.State != r.After {
		t.Fatal(firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"))
	}
	if !slices.Equal(result.Calls, r.Calls) || !slices.Equal(result.SoundIDs, r.Sounds) {
		t.Fatal("scheduler/sounds", result.Calls, r.Calls, result.SoundIDs, r.Sounds)
	}
	return result.State
}
func TestHoopLobOriginalPhasesAndChains(t *testing.T) {
	f := hoopSharedRows(t)
	for _, r := range f.Phases {
		assertHoopSharedPhase(t, r)
	}
	for _, c := range f.Chains {
		state := c.Records[0].ResetBefore
		for _, r := range c.Records {
			if r.ResetBefore != state {
				t.Fatal("discontinuous original chain")
			}
			links := BeginOriginalContacts(ContactPhaseState{Links: state.actionLinks()}, state.Ball.Action).Links
			state.setActionLinks(links)
			if state != r.Before {
				t.Fatal("original reset changed other world state")
			}
			state = assertHoopSharedPhase(t, r)
		}
		if state.Hoops[c.Index].Action != HoopInactive {
			t.Fatal("did not land")
		}
	}
}
