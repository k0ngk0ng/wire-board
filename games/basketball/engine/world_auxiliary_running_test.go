package engine

import (
	"reflect"
	"testing"
)

type auxiliaryRunningFixture struct {
	worldActionFixture
	Actor   uint8
	Pressed [4]Buttons
	Sounds  []uint8
}

func TestWorldOriginalAuxiliaryRunningBRoleProbes(t *testing.T) {
	var rows []auxiliaryRunningFixture
	decodeAuxiliaryWorldFixture(t, "auxiliary-running-players.json.gz", &rows)
	roles := map[[2]uint8]bool{}
	for i, r := range rows {
		ctx := r.context(t)
		ctx.ActorPressedButtons = &r.Pressed
		result := WorldActionStep{State: r.Before}
		court, e := originalWorldMotionCourt(&result, ctx, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		e = advanceWorldPlayer(&result, r.Actor, ctx, court)
		if e != nil || result.State != r.After {
			t.Fatalf("row%d role%d/%d: %s err%v", i, ctx.Teams[r.Actor/2], ctx.Characters[r.Actor], firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"), e)
		}
		if !reflect.DeepEqual(append([]uint8{}, result.SoundIDs...), append([]uint8{}, r.Sounds...)) {
			t.Fatalf("row%d sound %v != %v", i, result.SoundIDs, r.Sounds)
		}
		roles[[2]uint8{ctx.Teams[r.Actor/2], ctx.Characters[r.Actor]}] = true
	}
	if len(roles) != 32 {
		t.Fatal("role coverage", len(roles))
	}
	t.Log("original player probes", len(rows), "roles", len(roles))
}

func TestWorldOriginalAuxiliaryRunningObjectProbes(t *testing.T) {
	var rows []auxiliaryRunningFixture
	decodeAuxiliaryWorldFixture(t, "auxiliary-running-objects.json.gz", &rows)
	for i, r := range rows {
		ctx := r.context(t)
		result := WorldActionStep{State: r.Before}
		court, e := originalWorldMotionCourt(&result, ctx, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		e = advanceWorldAuxiliary(&result, r.Actor, ctx, court)
		if e != nil || result.State != r.After {
			t.Fatalf("row%d action%d: %s err%v", i, r.Before.Auxiliary[r.Actor-4].Action, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(r.After), "world"), e)
		}
	}
	t.Log("original auxiliary probes", len(rows))
}

func TestWorldOriginalAuxiliaryRunningControllerChains(t *testing.T) {
	var chains [][]struct {
		worldInputFixture
		Tick                      int
		Actor, Object             uint8
		AfterActions              WorldInputState
		PickupBefore, PickupAfter [4]uint8
		Boundaries                []auxiliaryRunningFixture
	}
	decodeAuxiliaryWorldFixture(t, "auxiliary-running-controller.json.gz", &chains)
	if len(chains) != 12 {
		t.Fatal("missing role/actor chains")
	}
	for ci, chain := range chains {
		state := chain[0].Before
		flags := chain[0].PickupBefore
		saw68 := false
		sawPickup, sawRun, sawObjectAttack, sawRelease := false, false, false, false
		for _, r := range chain {
			ctx := r.context(t)
			ctx.PickupFlags = flags
			effects, e := advanceWorldHumanInputWithEffects(&state, r.Actor, ctx)
			if e != nil {
				t.Fatal(ci, r.Tick, e)
			}
			if effects.ChangePickupFlags {
				flags[r.Actor] = effects.PickupFlags
			}
			if state != r.After || flags != r.PickupAfter {
				t.Fatalf("chain%d tick%d input %s", ci, r.Tick, firstWorldLoopDifference(reflect.ValueOf(state), reflect.ValueOf(r.After), "state"))
			}
			saw68 = saw68 || state.World.Actors[r.Actor].Player.Request == (ActionRequest{Action: 0x68, Pending: true})
			result := WorldActionStep{State: state.World}
			ac := ctx.World
			ac.ActorPressedButtons = &state.Pressed
			ac.ActorHeldButtons = state.Held
			ac.PickupFlags = &flags
			for _, boundary := range r.Boundaries {
				court, e := originalWorldMotionCourt(&result, ac, boundary.Actor)
				if e != nil {
					t.Fatal(e)
				}
				if boundary.Actor < 4 {
					e = advanceWorldPlayer(&result, boundary.Actor, ac, court)
				} else {
					e = advanceWorldAuxiliary(&result, boundary.Actor, ac, court)
				}
				if e != nil || result.State != boundary.After {
					t.Fatalf("chain%d tick%d object%d %s err%v", ci, r.Tick, boundary.Actor, firstWorldLoopDifference(reflect.ValueOf(result.State), reflect.ValueOf(boundary.After), "world"), e)
				}
			}
			state.World = result.State
			p := state.World.Actors[r.Actor].Player
			sawPickup = sawPickup || p.Action == 0x4e
			sawRun = sawRun || p.Action == 0x22
			auxAction := state.World.Auxiliary[r.Object-4].Action
			sawObjectAttack = sawObjectAttack || auxAction == 9 || auxAction == 10
			sawRelease = sawRelease || auxAction == 11
			if state != r.AfterActions {
				t.Fatalf("chain%d tick%d final state differs", ci, r.Tick)
			}
		}
		if !saw68 {
			t.Fatalf("chain%d never requested original68", ci)
		}
		team := chain[0].Context.World.Teams[chain[0].Actor/2]
		if !sawRun || len(chain) == 70 && !sawPickup || team != 2 && !sawObjectAttack || team == 7 && !sawRelease {
			t.Fatalf("chain%d missing actual pickup/run/object/launch phase: %v/%v/%v/%v", ci, sawPickup, sawRun, sawObjectAttack, sawRelease)
		}
	}
	t.Log("12 continuous original input/action chains, including pickup and double-tap run, no oracle reseeding")
}

func TestWorldAuxiliaryRunningRequiresPressedContext(t *testing.T) {
	var rows []auxiliaryRunningFixture
	decodeAuxiliaryWorldFixture(t, "auxiliary-running-players.json.gz", &rows)
	for _, r := range rows {
		if r.Context.Teams[r.Actor/2] != 7 || r.Context.Characters[r.Actor] != 0 {
			continue
		}
		ctx := r.context(t)
		result := WorldActionStep{State: r.Before}
		court, e := originalWorldMotionCourt(&result, ctx, r.Actor)
		if e != nil {
			t.Fatal(e)
		}
		if e = advanceWorldPlayer(&result, r.Actor, ctx, court); e == nil {
			t.Fatal("missing pressed context silently used zero")
		}
		return
	}
	t.Fatal("no handler8 fixture")
}
