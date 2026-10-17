package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalActiveWorldHumanInput(t *testing.T) {
	var rows []struct {
		worldInputFixture
		Actor                     uint8
		Scenario                  string
		PickupBefore, PickupAfter [4]uint8
		JumpEnhancements          [4]uint8
	}
	f, err := os.Open("testdata/world-human-input.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	if err = json.NewDecoder(z).Decode(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1117 {
		t.Fatal("missing original active human calls", len(rows))
	}
	actions := map[Action]int{}
	for _, r := range rows {
		s := r.Before
		ctx := r.context(t)
		ctx.PickupFlags = r.PickupBefore
		ctx.JumpEnhancements = r.JumpEnhancements
		effects, err := advanceWorldHumanInputWithEffects(&s, r.Actor, ctx)
		flags := r.PickupBefore
		if effects.ChangePickupFlags {
			flags[r.Actor] = effects.PickupFlags
		}
		if err != nil || s != r.After || flags != r.PickupAfter {
			t.Logf("scenario %s frame %d actor %d action %d frame %+v memory %+v", r.Scenario, r.Frame, r.Actor, r.Before.World.Actors[r.Actor].Player.Action, ctx.Frames[r.Actor], r.Before.Memory[r.Actor])
			a, b := reflect.ValueOf(s), reflect.ValueOf(r.After)
			for i := 0; i < a.NumField(); i++ {
				if !reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
					t.Logf("different %s: got %+v want %+v", a.Type().Field(i).Name, a.Field(i).Interface(), b.Field(i).Interface())
				}
			}
			t.Fatalf("human mismatch effects %+v flags %v/%v error %v", effects, flags, r.PickupAfter, err)
		}
		actions[r.Before.World.Actors[r.Actor].Player.Action]++
	}
	t.Logf("matched all %d active original human calls across actions %v", len(rows), actions)
}
func humanInputTestState() WorldInputState {
	s := WorldInputState{}
	s.World.Holder = 6
	s.World.SpecialHolder = -1
	s.World.CarriedPlayer = -1
	for i := range s.World.Actors {
		s.World.Actors[i].LinkedActor = -1
		s.World.Actors[i].Player.Body.Position = Position{X: 200 * Pixel, Depth: 40 * Pixel}
	}
	for i := range s.World.Auxiliary {
		s.World.Auxiliary[i].Owner = -1
	}
	for i := range s.World.Hoops {
		s.World.Hoops[i].Owner = uint8(7 + i)
		s.World.Hoops[i].Action = HoopReady
	}
	return s
}
func TestWorldHumanAirborneInputs(t *testing.T) {
	for _, action := range []Action{ActionJump, ActionFalling, 0x61, ActionAimAirShot, ActionLand} {
		s := humanInputTestState()
		s.World.Actors[0].Player.Action = action
		s.World.Actors[0].Player.Airborne = true
		s.Memory[0] = InputMemory{PendingButtons: ButtonA, ButtonDelay: 1}
		ctx := WorldInputContext{}
		ctx.Frames[0] = ControllerFrame{Held: ButtonA}
		_, err := advanceWorldHumanInputWithEffects(&s, 0, ctx)
		if err != nil || s.World.Actors[0].Player.Request != (ActionRequest{Action: 0x61, Pending: true}) {
			t.Fatal(action, s.World.Actors[0], err)
		}
	}
	s := humanInputTestState()
	s.World.Actors[0].Player.Action = ActionJump
	s.World.Actors[0].Player.Airborne = true
	s.Memory[0] = InputMemory{PendingButtons: ButtonA | ButtonB, ButtonDelay: 1}
	ctx := WorldInputContext{}
	ctx.Frames[0] = ControllerFrame{Held: ButtonA | ButtonB}
	_, err := advanceWorldHumanInputWithEffects(&s, 0, ctx)
	if err != nil || s.World.Actors[0].JumpPhase != -128 || s.World.Actors[0].Player.Request.Action != ActionJump {
		t.Fatal(s, err)
	}
	s.Memory[0] = InputMemory{PendingButtons: ButtonA | ButtonB, ButtonDelay: 1}
	_, err = advanceWorldHumanInputWithEffects(&s, 0, ctx)
	if err != nil || s.World.Actors[0].Player.Request.Action != ActionIdle {
		t.Fatal("repeated airborne jump ignored original gate", s, err)
	}
}
func TestWorldHumanPickupFlagCommandAndVerticalTap(t *testing.T) {
	s := humanInputTestState()
	s.World.Ball = Ball{Action: BallRest, Body: Body{Position: Position{X: 200 * Pixel, Depth: 40 * Pixel}}}
	s.Memory[0] = InputMemory{PendingButtons: ButtonA, ButtonDelay: 1}
	ctx := WorldInputContext{}
	ctx.Frames[0] = ControllerFrame{Held: ButtonA}
	effect, err := advanceWorldHumanInputWithEffects(&s, 0, ctx)
	if err != nil || !effect.ChangePickupFlags || effect.PickupFlags != 1 || s.World.Actors[0].Player.Request.Action != 0x57 {
		t.Fatal(s, effect, err)
	}
	s = humanInputTestState()
	s.World.Actors[0].Player.Action = ActionRun
	s.Memory[0] = InputMemory{TapWindow: 5, LastTap: ButtonUp}
	ctx.Frames[0] = ControllerFrame{Held: ButtonUp, Pressed: ButtonUp}
	_, err = advanceWorldHumanInputWithEffects(&s, 0, ctx)
	if err != nil || s.World.Actors[0].Player.Request.Action != 0x2d {
		t.Fatal(s, err)
	}
}
