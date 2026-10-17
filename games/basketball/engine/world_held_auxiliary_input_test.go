package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type heldAuxiliaryInputCase struct {
	worldInputFixture
	Actor                                       uint8
	Name                                        string
	Tick                                        int
	PickupBefore, PickupAfter, JumpEnhancements [4]uint8
	AfterActions                                *WorldInputState
}

func heldAuxiliaryInputCases(t *testing.T) []heldAuxiliaryInputCase {
	t.Helper()
	f, err := os.Open("testdata/held-auxiliary-human-input.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var rows []heldAuxiliaryInputCase
	if err := json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1714 {
		t.Fatal("missing original controller calls", len(rows))
	}
	return rows
}

func TestOriginalHeldAuxiliaryHumanInputCalls(t *testing.T) {
	for _, r := range heldAuxiliaryInputCases(t) {
		s := r.Before
		ctx := r.context(t)
		ctx.PickupFlags, ctx.JumpEnhancements = r.PickupBefore, r.JumpEnhancements
		effects, err := advanceWorldHumanInputWithEffects(&s, r.Actor, ctx)
		flags := r.PickupBefore
		if effects.ChangePickupFlags {
			flags[r.Actor] = effects.PickupFlags
		}
		if err != nil || s != r.After || flags != r.PickupAfter {
			t.Fatalf("%s tick%d: %s flags%v/%v err=%v", r.Name, r.Tick, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "state"), flags, r.PickupAfter, err)
		}
	}
}

func TestHeldAuxiliaryInputUsesPoseAndCurrentHeading(t *testing.T) {
	for _, button := range []Buttons{ButtonA, ButtonB | ButtonRight} {
		s := humanInputTestState()
		s.World.Auxiliary[1].Owner = 0
		s.World.Auxiliary[1].Heading = HeadingUpRight
		p := &s.World.Actors[0].Player
		p.Action, p.Entered, p.Heading, p.Pose.Mirrored, p.FacingLeft = 0x20, true, HeadingLeft, true, false
		ctx := WorldInputContext{}
		for tick := 0; tick < 2; tick++ {
			ctx.Frames[0] = ControllerFrame{Held: button}
			if tick == 0 {
				ctx.Frames[0].Pressed = button
			}
			if _, err := advanceWorldHumanInputWithEffects(&s, 0, ctx); err != nil {
				t.Fatal(err)
			}
		}
		if button == ButtonA {
			if s.World.Auxiliary[1].Heading != HeadingLeft || p.Request.Action != 0x27 {
				t.Fatal("A must use pose mirror", s)
			}
		} else {
			if p.Request.Action != 0x29 || s.World.Auxiliary[1].FacingLeft || s.World.Auxiliary[1].Heading != HeadingUpRight {
				t.Fatal("B must use fresh physical heading and current facing", s)
			}
		}
		if s.World.Auxiliary[1].Owner != 0 {
			t.Fatal("input released owner before action", s)
		}
	}
}

func TestOriginalHeldAuxiliaryControllerActionChains(t *testing.T) {
	var s WorldInputState
	var flags [4]uint8
	name := ""
	chains := 0
	for _, r := range heldAuxiliaryInputCases(t) {
		if !strings.HasPrefix(r.Name, "chain-") {
			continue
		}
		if r.Name != name {
			name = r.Name
			s = r.Before
			flags = r.PickupBefore
			chains++
		}
		if s != r.Before || flags != r.PickupBefore {
			t.Fatalf("%s tick%d chain before: %s", name, r.Tick, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.Before), "state"))
		}
		ctx := r.context(t)
		ctx.PickupFlags, ctx.JumpEnhancements = flags, r.JumpEnhancements
		effects, err := advanceWorldHumanInputWithEffects(&s, r.Actor, ctx)
		if err != nil {
			t.Fatal(name, r.Tick, err)
		}
		if effects.ChangePickupFlags {
			flags[r.Actor] = effects.PickupFlags
		}
		if s != r.After {
			t.Fatalf("%s tick%d input: %s", name, r.Tick, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(r.After), "state"))
		}
		step := WorldActionStep{State: s.World}
		c := ctx.World
		c.PickupFlags = &flags
		c.JumpEnhancements = &ctx.JumpEnhancements
		court, err := originalWorldMotionCourt(&step, c, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err = advanceWorldPlayer(&step, 0, c, court); err != nil {
			t.Fatal(name, r.Tick, err)
		}
		court, err = originalWorldMotionCourt(&step, c, 5)
		if err != nil {
			t.Fatal(err)
		}
		if err = advanceWorldAuxiliary(&step, 5, c, court); err != nil {
			t.Fatal(name, r.Tick, err)
		}
		s.World = step.State
		if r.AfterActions == nil {
			t.Fatal("missing original action result", name, r.Tick)
		}
		if s != *r.AfterActions {
			t.Fatalf("%s tick%d actions: %s", name, r.Tick, firstWorldLoopDifference(reflect.ValueOf(s), reflect.ValueOf(*r.AfterActions), "state"))
		}
	}
	if chains != 5 {
		t.Fatal("missing controller/action chains", chains)
	}
}
