package pokepath_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestNativePlatformInputTimePreservesNextAppointment(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	base := ctx.Now
	ctx.Actor = -1
	ctx.Now = base.Add(100 * time.Millisecond)
	var err error
	match, err = module.OnTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := module.StreamView(match, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx.Actor = 0
	ctx.Now = base.Add(195 * time.Millisecond)
	match = nativeAction(t, module, ctx, match, `"type":"speed"`)
	observation := nativeObserve(t, module, match, 0)
	if observation.Clock != 195 || observation.ServerAt != ctx.Now.UnixMilli() || match.SystemDeadline != base.Add(200*time.Millisecond).UnixMilli() {
		t.Fatal("input was applied retroactively or postponed the reserved system tick")
	}
	update, err := module.StreamView(match, 0, initial.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(update.Data)
	var delta struct {
		Ops []struct {
			Type         string `json:"type"`
			Milliseconds int64  `json:"milliseconds"`
		} `json:"operations"`
	}
	_ = json.Unmarshal(encoded, &delta)
	if len(delta.Ops) != 2 || delta.Ops[0].Type != "tick" || delta.Ops[0].Milliseconds != 95 || delta.Ops[1].Type != "action" {
		t.Fatalf("input timing cannot be replayed in authority order: %s", encoded)
	}
	ctx.Actor = -1
	ctx.Now = base.Add(200 * time.Millisecond)
	match, err = module.OnTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	if nativeObserve(t, module, match, 0).Clock != 200 || match.SystemDeadline != base.Add(300*time.Millisecond).UnixMilli() {
		t.Fatal("the scheduled frame duplicated input-time advancement")
	}
}

func TestNativePlatformFrequentInputsDoNotStarveSystemClock(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	base := ctx.Now
	for millisecond := 10; millisecond < 100; millisecond += 10 {
		ctx.Now = base.Add(time.Duration(millisecond) * time.Millisecond)
		match = nativeAction(t, module, ctx, match, `"type":"speed"`)
		if match.SystemDeadline != base.Add(100*time.Millisecond).UnixMilli() {
			t.Fatalf("input at %dms postponed the shared bot/frame appointment", millisecond)
		}
	}
	ctx.Actor = -1
	ctx.Now = base.Add(100 * time.Millisecond)
	var err error
	match, err = module.OnTimer(ctx, match)
	if err != nil {
		t.Fatal(err)
	}
	if nativeObserve(t, module, match, 0).Clock != 100 {
		t.Fatal("frequent human input starved or duplicated the shared simulation")
	}
}

func TestNativePlatformInvalidInputRollsBackItsTimeCandidate(t *testing.T) {
	module, ctx, match := nativeStart(t, 2)
	before := append([]byte(nil), match.Data...)
	deadline := match.SystemDeadline
	ctx.Now = ctx.Now.Add(95 * time.Millisecond)
	view := nativeObserve(t, module, match, 0)
	_, err := module.Apply(ctx, match, json.RawMessage(fmt.Sprintf(`{"type":"upgrade","epoch":%d,"pokemonID":999999}`, view.Epoch)))
	if err == nil || !bytes.Equal(before, match.Data) || match.SystemDeadline != deadline {
		t.Fatal("invalid input committed its preliminary time advancement")
	}
	match = nativeAction(t, module, ctx, match, `"type":"speed"`)
	if nativeObserve(t, module, match, 0).Clock != 95 {
		t.Fatal("the next valid candidate did not start from the unmodified rule world")
	}
}

func TestNativePlatformSaveFreezesActualTimeAndRemainingFrameBudget(t *testing.T) {
	module, ctx, match := nativeStart(t, 1)
	base := ctx.Now
	ctx.Now = base.Add(95 * time.Millisecond)
	frozen, err := module.SaveControl(ctx, match, true)
	if err != nil {
		t.Fatal(err)
	}
	if nativeObserve(t, module, frozen, 0).Clock != 95 {
		t.Fatal("save confirmation froze an obsolete rule time")
	}
	ctx.Now = base.Add(48 * time.Hour)
	resumed, err := module.ResumeSaved(ctx, frozen)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.SystemDeadline != ctx.Now.Add(5*time.Millisecond).UnixMilli() || nativeObserve(t, module, resumed, 0).Clock != 95 {
		t.Fatal("save resume reset its remaining appointment or advanced offline rules")
	}
	ctx.Actor = -1
	ctx.Now = ctx.Now.Add(5 * time.Millisecond)
	continued, err := module.OnTimer(ctx, resumed)
	if err != nil {
		t.Fatal(err)
	}
	if nativeObserve(t, module, continued, 0).Clock != 100 {
		t.Fatal("resumed frame did not preserve the exact fixed-step remainder")
	}
}
