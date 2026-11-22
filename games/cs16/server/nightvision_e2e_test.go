package cs16

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
)

func TestNightVisionModuleInputAndPrivateProjectionE2E(t *testing.T) {
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	state.World.Players[0].Money = 1250
	match, err = envelope(state, false, ctx.Now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	match, err = apply(ctx, match, actionBytes(t, buyAction{Type: "buy", Epoch: state.Epoch, Item: "nvgs", Life: state.World.Players[0].Life, Round: state.World.Round}))
	if err != nil {
		t.Fatal("formal buy path rejected night vision", err)
	}
	action := inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 1, Life: state.World.Players[0].Life, NightVision: true}}
	if released(action.Input) {
		t.Fatal("night vision key bypassed active input validation")
	}
	action.Cancel = true
	if _, err = apply(ctx, match, actionBytes(t, action)); err == nil {
		t.Fatal("cancel action smuggled night vision press")
	}
	action.Cancel = false
	match, err = apply(ctx, match, actionBytes(t, action))
	if err != nil {
		t.Fatal("formal input path rejected night vision", err)
	}
	ctx.Now = ctx.Now.Add(10 * time.Millisecond)
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	state, err = decode(match)
	if err != nil {
		t.Fatal(err)
	}
	own := projection(state, 0)
	if !own.Players[0].NightVisionOwned || !own.Players[0].NightVision || own.Players[0].Money == nil || *own.Players[0].Money != 0 {
		t.Fatal("own night vision equipment/display projection is missing")
	}
	for _, seat := range []int{1, -1} {
		raw, err := json.Marshal(projection(state, seat))
		if err != nil || strings.Contains(string(raw), "nightVision") || strings.Contains(string(raw), `"nightvision"`) || strings.Contains(string(raw), `"nvgs"`) {
			t.Fatal("foreign seat or ordinary spectator learned private night vision state", seat, err)
		}
	}
	changes := 0
	for _, event := range own.Events {
		if event.Kind == "nightvision" {
			changes++
			if event.Value != 1 {
				t.Fatal("night vision on event has incorrect value")
			}
		}
	}
	if changes != 1 {
		t.Fatal("own night vision toggle event was lost or repeated", changes)
	}
	if state.World.Players[0].AppliedSeq != 1 {
		t.Fatal("night vision input did not advance through authoritative input archive")
	}
	action.Seq++
	action.Life--
	if _, err = apply(ctx, match, actionBytes(t, action)); err == nil {
		t.Fatal("old life input toggled night vision")
	}
}

func TestNightVisionDefaultProjectionAddsNoInactiveFields(t *testing.T) {
	state := observerFixture()
	raw, err := json.Marshal(projection(state, 0))
	if err != nil || strings.Contains(string(raw), "nightVision") {
		t.Fatal("inactive night vision enlarged ordinary public projection", err)
	}
}
