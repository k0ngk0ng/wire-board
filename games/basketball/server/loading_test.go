package basketball_test

// Failure scenarios specified before implementation: timers/inputs advance an
// unloaded match; the first human starts for everyone; observers, wrong epochs
// or automated seats bypass the barrier; duplicates restart the clock; restore
// loses readiness; all-bot starts hang; takeover changes a running match.
import (
	"encoding/json"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"reflect"
	"testing"
	"time"
)

func loadingStart(t *testing.T, flags []bool) (gameapi.Module, gameapi.Context, gameapi.Match) {
	t.Helper()
	mod := game.Module()
	ctx := gameapi.Context{Players: 2, Actor: 0, Now: time.UnixMilli(1000000), Automated: flags}
	m, e := mod.Start(ctx, json.RawMessage(`{"mode":"versus"}`))
	if e != nil {
		t.Fatal(e)
	}
	return mod, ctx, m
}
func loadingAction(t *testing.T, mod gameapi.Module, ctx gameapi.Context, m gameapi.Match, seat int) gameapi.Match {
	t.Helper()
	ctx.Actor = seat
	b, _ := json.Marshal(map[string]any{"type": "loaded", "epoch": hostState(t, m).Epoch})
	next, e := mod.Apply(ctx, m, b)
	if e != nil {
		t.Fatal(e)
	}
	return next
}
func loadingView(t *testing.T, mod gameapi.Module, m gameapi.Match) game.View {
	t.Helper()
	v, e := mod.View(m, 0)
	if e != nil {
		t.Fatal(e)
	}
	return v.(game.View)
}
func TestBasketballLoadingBarrier(t *testing.T) {
	mod, ctx, m := loadingStart(t, []bool{false, false})
	initial := hostState(t, m).Runtime
	if v := loadingView(t, mod, m); v.Phase != "loading" || v.CanInput {
		t.Fatalf("started before ready: %s input=%v", v.Phase, v.CanInput)
	}
	ctx.Now = ctx.Now.Add(time.Minute)
	next, e := mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	m = next
	if !reflect.DeepEqual(hostState(t, m).Runtime, initial) {
		t.Fatal("loading advanced original simulation")
	}
	beforeKey := m.CheckpointKey
	m = loadingAction(t, mod, ctx, m, 0)
	if loadingView(t, mod, m).Phase != "loading" || m.CheckpointKey == beforeKey {
		t.Fatal("partial readiness must persist without starting")
	}
	m, e = mod.Restore(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	ctx.Now = ctx.Now.Add(time.Minute)
	m = loadingAction(t, mod, ctx, m, 1)
	if v := loadingView(t, mod, m); v.Phase != "opening" || !v.CanInput {
		t.Fatal("ready players did not start")
	}
	if !reflect.DeepEqual(hostState(t, m).Runtime, initial) || hostState(t, m).ServerAt != ctx.Now.UnixMilli() {
		t.Fatal("load time consumed match time")
	}
	ctx.Now = ctx.Now.Add(time.Second)
	duplicate := loadingAction(t, mod, ctx, m, 0)
	if string(duplicate.Data) != string(m.Data) {
		t.Fatal("duplicate/reconnect reset live match")
	}
	next, e = mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if hostState(t, next).Runtime.VideoFrames == initial.VideoFrames {
		t.Fatal("ready match cannot advance")
	}
}
func TestBasketballLoadingRejectsInvalidAndEarlyInput(t *testing.T) {
	mod, ctx, m := loadingStart(t, []bool{false, true})
	epoch := hostState(t, m).Epoch
	for _, tc := range []struct {
		seat   int
		action map[string]any
	}{
		{-1, map[string]any{"type": "loaded", "epoch": epoch}}, {1, map[string]any{"type": "loaded", "epoch": epoch}},
		{0, map[string]any{"type": "loaded", "epoch": "old"}}, {0, map[string]any{"type": "loaded", "epoch": epoch, "buttons": 1}},
		{0, map[string]any{"type": "buttons", "epoch": epoch, "sequence": 1, "buttons": 1}},
	} {
		ctx.Actor = tc.seat
		raw, _ := json.Marshal(tc.action)
		if _, e := mod.Apply(ctx, m, raw); e == nil {
			t.Fatalf("accepted invalid loading action: %v", tc)
		}
		if mod.AcceptStale(ctx, m, raw) {
			t.Fatal("stale route bypassed validation")
		}
	}
	ctx.Actor = 0
	raw, _ := json.Marshal(map[string]any{"type": "loaded", "epoch": epoch})
	if !mod.AcceptStale(ctx, m, raw) {
		t.Fatal("ready ack must survive concurrent seat version")
	}
	m = loadingAction(t, mod, ctx, m, 0)
	if loadingView(t, mod, m).Phase == "loading" {
		t.Fatal("computer blocked human")
	}
}
func TestBasketballLoadingAutomationAndLegacy(t *testing.T) {
	mod, ctx, m := loadingStart(t, []bool{true, true})
	if loadingView(t, mod, m).Phase == "loading" {
		t.Fatal("all-bot match blocked")
	}
	_, ctx, m = loadingStart(t, []bool{false, false})
	m = loadingAction(t, mod, ctx, m, 0)
	ctx.Automated[1] = true
	ctx.Now = ctx.Now.Add(time.Second)
	next, e := mod.OnTimer(ctx, m)
	if e != nil {
		t.Fatal(e)
	}
	if loadingView(t, mod, next).Phase == "loading" {
		t.Fatal("explicit autoplay did not unblock")
	}
	// Old schema-1 snapshots have no loading field and must never acquire a gate.
	var fields map[string]json.RawMessage
	json.Unmarshal(next.Data, &fields)
	delete(fields, "loading")
	next.Data, _ = json.Marshal(fields)
	restored, e := mod.Restore(ctx, next)
	if e != nil {
		t.Fatal(e)
	}
	if loadingView(t, mod, restored).Phase == "loading" {
		t.Fatal("legacy restore paused match")
	}
}
