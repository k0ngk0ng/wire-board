package engine

import (
	"reflect"
	"testing"
)

func TestWorldOriginalAuxiliarySetDownBeforeDeviceMotion(t *testing.T) {
	var rows []struct {
		Actor, Court  uint8
		Before, After WorldActionState
	}
	decodeAuxiliaryWorldFixture(t, "auxiliary-setdown-probes.json.gz", &rows)
	for i, r := range rows {
		result := WorldActionStep{State: r.Before}
		ctx := WorldActionContext{CourtID: r.Court, Teams: [2]uint8{0, 1}}
		for j := range ctx.Profiles {
			ctx.Profiles[j] = KunioProfile()
		}
		court, e := originalWorldMotionCourt(&result, ctx, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		e = advanceWorldPlayer(&result, r.Actor, ctx, court)
		if e != nil || result.State != r.After {
			t.Fatalf("row%d: %s err%v", i, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"), e)
		}
	}
}

func TestWorldOriginalInputToNondirectionalBSetDown(t *testing.T) {
	var rows []struct {
		worldInputFixture
		Tick                                        int
		AfterActions                                WorldInputState
		PickupBefore, PickupAfter, JumpEnhancements [4]uint8
	}
	decodeAuxiliaryWorldFixture(t, "auxiliary-setdown-controller.json.gz", &rows)
	if len(rows) != 17 {
		t.Fatal("missing original controller cycles", len(rows))
	}
	state := rows[0].Before
	flags := rows[0].PickupBefore
	setDown := false
	for _, r := range rows {
		ctx := r.context(t)
		ctx.PickupFlags, ctx.JumpEnhancements = flags, r.JumpEnhancements
		effects, e := advanceWorldHumanInputWithEffects(&state, 0, ctx)
		if e != nil {
			t.Fatal(r.Tick, e)
		}
		if effects.ChangePickupFlags {
			flags[0] = effects.PickupFlags
		}
		if state != r.After || flags != r.PickupAfter {
			t.Fatalf("input tick%d: %s", r.Tick, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(r.After), "state"))
		}
		result := WorldActionStep{State: state.World}
		actionContext := ctx.World
		actionContext.PickupFlags = &flags
		actionContext.JumpEnhancements = &ctx.JumpEnhancements
		actionContext.ActorHeldButtons = state.Held
		for _, object := range []uint8{0, 5} {
			court, e := originalWorldMotionCourt(&result, actionContext, object)
			if e != nil {
				t.Fatal(e)
			}
			if object == 0 {
				e = advanceWorldPlayer(&result, object, actionContext, court)
			} else {
				e = advanceWorldAuxiliary(&result, object, actionContext, court)
			}
			if e != nil {
				t.Fatal(r.Tick, object, e)
			}
		}
		state.World = result.State
		if state != r.AfterActions {
			t.Fatalf("action tick%d: %s", r.Tick, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(r.AfterActions), "state"))
		}
		setDown = setDown || state.World.Actors[0].Player.Action == 0x4f
	}
	if !setDown || state.World.Auxiliary[1].Owner != -1 || state.World.Auxiliary[1].Action != 0 || state.World.Auxiliary[1].Airborne {
		t.Fatal("no-direction B did not put the prop down")
	}
	t.Log("17 original input/action cycles retained continuously; A pickup then unmodified nondirectional B enters4F and reaches idle prop")
}
