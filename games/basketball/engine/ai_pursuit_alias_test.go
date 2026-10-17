package engine

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalPursuitAliasCompleteInputPhase(t *testing.T) {
	f, err := os.Open("testdata/pursuit-alias-input.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var r struct {
		worldInputFixture
		PickupFlags, JumpEnhancements [4]uint8
		Probe                         struct {
			Action   Action
			X, Depth Fixed
		}
	}
	if err = json.NewDecoder(z).Decode(&r); err != nil {
		t.Fatal(err)
	}
	ctx := r.context(t)
	ctx.PickupFlags = r.PickupFlags
	ctx.JumpEnhancements = r.JumpEnhancements
	got, err := StepWorldInputs(r.Before, ctx)
	if err != nil || got.State != r.After || !reflect.DeepEqual(got.Calls, r.Calls) {
		t.Fatalf("original frame %d pursuit alias phase differs: %s: %v", r.Frame, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "state"), err)
	}
	probe, err := originalPursuitProbe(AIControlView{Auxiliary: &r.Before.World.Auxiliary}, 31)
	if err != nil || probe.Action != r.Probe.Action || probe.Position.X != r.Probe.X || probe.Position.Depth != r.Probe.Depth {
		t.Fatal("typed alias does not match original byte reads", probe, r.Probe, err)
	}
}

func TestOriginalPursuitAliasRequiresActualContext(t *testing.T) {
	if _, err := originalPursuitProbe(AIControlView{}, 31); err == nil {
		t.Fatal("missing auxiliary state silently treated as an opponent")
	}
	aux := [2]AuxiliaryState{}
	probe, err := originalPursuitProbe(AIControlView{Auxiliary: &aux}, 31)
	if err != nil || probe.Action != 127 {
		t.Fatal("absent next action must preserve original FF byte", probe, err)
	}
	aux[1].Animation.HasNextAction = true
	aux[1].Animation.NextAction = ActionIdle
	probe, err = originalPursuitProbe(AIControlView{Auxiliary: &aux}, 31)
	if err != nil || probe.Action != ActionIdle {
		t.Fatal(probe, err)
	}
}

func TestOriginalPursuitAlias74CompleteInputPhase(t *testing.T) {
	f, err := os.Open("testdata/pursuit-alias74-input.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var r struct {
		worldInputFixture
		PickupFlags, JumpEnhancements [4]uint8
		Probe                         struct {
			Action   Action
			X, Depth Fixed
		}
	}
	if err = json.NewDecoder(z).Decode(&r); err != nil {
		t.Fatal(err)
	}
	ctx := r.context(t)
	ctx.PickupFlags = r.PickupFlags
	ctx.JumpEnhancements = r.JumpEnhancements
	got, err := StepWorldInputs(r.Before, ctx)
	if err != nil || got.State != r.After || !reflect.DeepEqual(got.Calls, r.Calls) {
		t.Fatalf("original frame %d pursuit alias phase differs: %s: %v", r.Frame, firstWorldLoopDifference(reflect.ValueOf(got.State), reflect.ValueOf(r.After), "state"), err)
	}
	probe, err := originalPursuitProbe(AIControlView{PursuitAlias74: r.Context.PursuitAlias74, Hoops: [6]AIHoopView{2: {Position: r.Before.World.Hoops[2].Position}}}, 74)
	if err != nil || probe.Action != r.Probe.Action || probe.Position.X != r.Probe.X || probe.Position.Depth != r.Probe.Depth {
		t.Fatal("typed alias does not match original byte reads", probe, r.Probe, err)
	}
}

func TestOriginalPursuitAlias74ContextAndByteSemantics(t *testing.T) {
	if _, err := originalPursuitProbe(AIControlView{}, 74); err == nil {
		t.Fatal("unknown alias bytes silently defaulted")
	}
	v := AIControlView{Actor: 3, PursuitAlias74: &PursuitAlias74{ActionByte: 0x80, DepthByte: 250}}
	v.PlayerActions[0] = Action(0x49)
	v.Hoops[2].Position.Height = Fixed(0xfff3)*Pixel + 127
	p, err := originalPursuitProbe(v, 74)
	if err != nil || p.Action != 0 || p.Position.X != Fixed(0xfff3)*Pixel || p.Position.Depth != 250*Pixel {
		t.Fatal(p, err)
	}
	v.BallPosition = Position{X: 3 * Pixel, Depth: 2 * Pixel}
	if target, err := opponentNearOriginalBall(v); err != nil || target != 74 {
		t.Fatal("unsigned depth and signed X wrap", target, err)
	}
	v.PursuitAlias74.DepthByte = 219
	if target, err := opponentNearOriginalBall(v); err != nil || target != -1 {
		t.Fatal("outside depth window", target, err)
	}
	v.PursuitAlias74.DepthByte = 2
	v.PursuitAlias74.ActionByte = 0x80 | 0x49
	if target, err := opponentNearOriginalBall(v); err != nil || target != -1 {
		t.Fatal("unsearchable aliased action", target, err)
	}
}
