package cs16

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/cs16/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func actionBytes(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func primedGrenade(t *testing.T) (gameapi.Context, gameapi.Match) {
	t.Helper()
	ctx, match := cursorFixture(t)
	state, err := decode(match)
	if err != nil {
		t.Fatal(err)
	}
	match, err = apply(ctx, match, actionBytes(t, map[string]any{"type": "buy", "epoch": state.Epoch, "life": state.World.Players[0].Life, "round": state.World.Round, "item": "hegrenade"}))
	if err != nil {
		t.Fatal(err)
	}
	ctx.Now = ctx.Now.Add(3 * time.Second)
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	state, err = decode(match)
	if err != nil {
		t.Fatal(err)
	}
	match, err = apply(ctx, match, actionBytes(t, inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 1, Life: state.World.Players[0].Life, ViewFrame: state.World.Frame, Select: "hegrenade"}}))
	if err != nil {
		t.Fatal(err)
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
	ctx.Now = time.UnixMilli(state.BaseAt + (state.World.Players[0].NextAttack+1)*1000/engine.FPS)
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	state, err = decode(match)
	if err != nil {
		t.Fatal(err)
	}
	match, err = apply(ctx, match, actionBytes(t, inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 2, Life: state.World.Players[0].Life, ViewFrame: state.World.Frame, Fire: true}}))
	if err != nil {
		t.Fatal(err)
	}
	ctx.Now = ctx.Now.Add(10 * time.Millisecond)
	match, err = onTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	state, err = decode(match)
	if err != nil || !strings.HasPrefix(state.World.Players[0].Action, "grenade:") {
		t.Fatal("grenade did not prime through ordinary purchase/select/fire", err, state.World.Players[0].Action)
	}
	return ctx, match
}

func grenadeAmmo(state *matchState) int {
	for _, item := range state.World.Players[0].Inventory {
		if item.ID == "hegrenade" {
			return item.Ammo
		}
	}
	return 0
}

func TestCancellationKeepsGrenadeAndNormalReleaseThrows(t *testing.T) {
	ctx, primed := primedGrenade(t)
	state, err := decode(primed)
	if err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"type": "input", "epoch": state.Epoch, "seq": 3, "life": state.World.Players[0].Life, "viewFrame": state.World.Frame, "moveX": 0, "moveY": 0, "yaw": 0, "pitch": 0, "fire": false, "cancel": true}
	for field, value := range map[string]any{"epoch": "old", "life": 999, "seq": 1, "fire": true, "moveY": 1} {
		bad := map[string]any{}
		for key, item := range base {
			bad[key] = item
		}
		bad[field] = value
		if _, err = apply(ctx, primed, actionBytes(t, bad)); err == nil {
			t.Fatal("invalid cancellation accepted", field)
		}
	}
	aged := ctx
	aged.InputReceivedAt = ctx.Now.Add(-201 * time.Millisecond)
	if _, err = apply(aged, primed, actionBytes(t, base)); err == nil {
		t.Fatal("aged cancellation accepted")
	}
	if !isInputRelease(ctx, primed, actionBytes(t, base)) {
		t.Fatal("valid cancellation not admitted as release")
	}
	normal := map[string]any{}
	for key, value := range base {
		if key != "cancel" {
			normal[key] = value
		}
	}
	if isInputRelease(ctx, primed, actionBytes(t, normal)) {
		t.Fatal("ordinary input admitted as autoplay cleanup")
	}
	normallyReleased, err := apply(ctx, primed, actionBytes(t, normal))
	if err != nil {
		t.Fatal(err)
	}
	ctx.Now = ctx.Now.Add(600 * time.Millisecond)
	normallyReleased, err = onTimer(ctx, normallyReleased)
	if err != nil {
		t.Fatal(err)
	}
	normalState, err := decode(normallyReleased)
	if err != nil || len(normalState.World.Grenades) != 1 || grenadeAmmo(normalState) != 0 {
		t.Fatal("ordinary mouseup did not throw", err, len(normalState.World.Grenades), grenadeAmmo(normalState))
	}
	ctx.Now = ctx.Now.Add(-600 * time.Millisecond)
	queued, err := apply(ctx, primed, actionBytes(t, inputAction{Type: "input", Epoch: state.Epoch, Input: engine.Input{Seq: 3, Life: state.World.Players[0].Life, ViewFrame: state.World.Frame, Fire: true}}))
	if err != nil {
		t.Fatal(err)
	}
	base["seq"] = 4
	ctx.Automated = []bool{true, false}
	cancelled, err := apply(ctx, queued, actionBytes(t, base))
	if err != nil {
		t.Fatal(err)
	}
	cancelState, err := decode(cancelled)
	if err != nil {
		t.Fatal(err)
	}
	if cancelState.World.EventID <= state.World.EventID {
		t.Fatal("same-frame cancellation did not invalidate a sent projection cursor")
	}
	player := cancelState.World.Players[0]
	if player.Action != "" || player.Input.Fire || len(player.Pending) != 1 || player.Pending[0].Seq != 4 || player.Pending[0].Fire || player.ReceivedSeq != 4 || player.AppliedSeq != 2 {
		t.Fatal("cancel did not clear held grenade and obsolete queue while separating receive/apply", player.Action, player.ReceivedSeq, player.AppliedSeq, len(player.Pending))
	}
	ctx.Automated = nil
	ctx.Now = ctx.Now.Add(600 * time.Millisecond)
	cancelled, err = onTimer(ctx, cancelled)
	if err != nil {
		t.Fatal(err)
	}
	cancelState, err = decode(cancelled)
	if err != nil || len(cancelState.World.Grenades) != 0 || grenadeAmmo(cancelState) != 1 || cancelState.World.Players[0].AppliedSeq != 4 {
		t.Fatal("cancellation threw or spent grenade or replayed obsolete input", err)
	}
}
