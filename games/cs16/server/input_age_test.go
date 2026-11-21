package cs16

import (
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func TestTransportAgeAndExecutionQueueAreSeparate(t *testing.T) {
	ctx, match := cursorFixture(t)
	base := ctx.Now
	ctx.Now = base.Add(600 * time.Millisecond)
	var err error
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	action := inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 1, Life: state.World.Players[0].Life, ViewFrame: 0, MoveY: 1000}}
	ctx.InputReceivedAt = base.Add(500 * time.Millisecond)
	if _, err = validatedInput(ctx, state, actionBytes(t, action)); err != nil {
		t.Fatal("500ms transport and 100ms queue boundary rejected", err)
	}
	ctx.InputReceivedAt = base.Add(501 * time.Millisecond)
	if _, err = validatedInput(ctx, state, actionBytes(t, action)); err == nil {
		t.Fatal("501ms old transport accepted as fresh input")
	}
	ctx.InputReceivedAt = base.Add(399 * time.Millisecond)
	if _, err = validatedInput(ctx, state, actionBytes(t, action)); err == nil {
		t.Fatal("201ms server execution queue accepted")
	}
	ctx.InputReceivedAt = base.Add(400 * time.Millisecond)
	if _, err = validatedInput(ctx, state, actionBytes(t, action)); err != nil {
		t.Fatal("200ms queue boundary rejected", err)
	}
	ctx.InputReceivedAt = base.Add(500 * time.Millisecond)
	action.ViewFrame = 60
	if _, err = validatedInput(ctx, state, actionBytes(t, action)); err == nil {
		t.Fatal("reference newer than receipt anchor accepted")
	}
	action.ViewFrame = 0
	action.Cancel = true
	action.MoveY = 0
	ctx.InputReceivedAt = base.Add(501 * time.Millisecond)
	if _, err = validatedInput(ctx, state, actionBytes(t, action)); err != nil {
		t.Fatal("cleanup could not release controls from an old display", err)
	}
}
