package basketball_test

// Before implementing reliable browser batches: delayed responses must not
// erase short/turbo edges, retry duplicates must not replay inputs or keep a
// stale hold alive, cancellation must overtake old requests, and a stale page
// or different authenticated seat must not accept an old client's batch.
import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func inputEvent(seq int64, buttons engine.Buttons, offset int, release bool) map[string]any {
	return map[string]any{"sequence": seq, "buttons": buttons, "offsetMS": offset, "release": release}
}
func inputBatch(t *testing.T, m gameapi.Match, seq int64, events ...map[string]any) json.RawMessage {
	t.Helper()
	s := hostState(t, m)
	raw, err := json.Marshal(map[string]any{"type": "inputs", "epoch": s.Epoch, "seat": 0, "basedOn": s.ServerAt, "sequence": seq, "events": events})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestHostReliableBatchKeepsEdgesAndIgnoresOldRetries(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	ctx.Actor = 0
	old := m
	expected := hostState(t, m).Runtime
	events := []map[string]any{inputEvent(1, engine.ButtonB, 0, false), inputEvent(2, 0, 20, false), inputEvent(3, engine.ButtonB, 70, false), inputEvent(4, 0, 80, false)}
	var err error
	m, err = module.Apply(ctx, m, inputBatch(t, m, 4, events...))
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := module.Apply(ctx, m, inputBatch(t, old, 2, events[:2]...))
	if err != nil || !reflect.DeepEqual(duplicate, m) {
		t.Fatal("old retry replayed an input", err)
	}
	for _, held := range []engine.Buttons{engine.ButtonB, 0, 0, 0, engine.ButtonB, 0} {
		r, e := engine.StepVideo(expected, [4]engine.Buttons{held})
		if e != nil {
			t.Fatal(e)
		}
		expected = r.State
	}
	m = hostTick(t, module, &ctx, m)
	if !reflect.DeepEqual(hostState(t, m).Runtime, expected) {
		t.Fatal("batch did not preserve independent original video input sequence")
	}
}
func TestHostBatchCancellationOvertakesDelayedRequests(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	ctx.Actor = 0
	original := m
	down := inputBatch(t, m, 1, inputEvent(1, engine.ButtonB, 0, false))
	var err error
	m, err = module.Apply(ctx, m, down)
	if err != nil {
		t.Fatal(err)
	}
	release := inputBatch(t, original, 2, inputEvent(2, 0, 0, true))
	// A newer packet carries the still-unacknowledged cancellation as well
	// as the fresh gesture, so the release request may arrive out of order.
	fresh := inputBatch(t, original, 3, inputEvent(2, 0, 0, true), inputEvent(3, engine.ButtonRight, 10, false))
	m, err = module.Apply(ctx, m, fresh)
	if err != nil {
		t.Fatal(err)
	}
	for _, late := range []json.RawMessage{release, down} {
		next, e := module.Apply(ctx, m, late)
		if e != nil || !reflect.DeepEqual(next, m) {
			t.Fatal("late packet changed newer intent", e)
		}
	}
	s := hostState(t, m)
	if s.Inputs[0].Pending&engine.ButtonB != 0 || s.Runtime.PendingPressed[0]&engine.ButtonB != 0 || s.Inputs[0].Buttons != engine.ButtonRight {
		t.Fatal("cancelled old B survived the newer direction")
	}
}
func TestHostBatchRejectsStaleSeatAndInvalidTimelineAtomically(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	ctx.Actor = 0
	valid := inputBatch(t, m, 1, inputEvent(1, engine.ButtonRight, 0, false))
	for _, change := range []func(map[string]any){
		func(a map[string]any) { a["seat"] = 1 },
		func(a map[string]any) { delete(a, "seat") },
		func(a map[string]any) { delete(a, "basedOn") },
		func(a map[string]any) { a["basedOn"] = ctx.Now.Add(time.Millisecond).UnixMilli() },
		func(a map[string]any) { a["basedOn"] = ctx.Now.Add(-2 * time.Second).UnixMilli() },
		func(a map[string]any) { a["events"] = []any{} },
		func(a map[string]any) { a["events"] = []any{inputEvent(1, 1, -1, false)} },
		func(a map[string]any) { a["events"] = []any{inputEvent(1, 1, 501, false)} },
		func(a map[string]any) { a["events"] = []any{inputEvent(1, 1, 0, false), inputEvent(1, 0, 1, false)} },
		func(a map[string]any) {
			a["sequence"] = 2
			a["events"] = []any{inputEvent(1, 1, 20, false), inputEvent(2, 0, 10, false)}
		},
		func(a map[string]any) { a["sequence"] = 1025; a["events"] = []any{inputEvent(1025, 1, 0, false)} },
		func(a map[string]any) { a["sequence"] = 2 },
		func(a map[string]any) { a["events"] = []any{inputEvent(1, 1, 0, true)} },
		func(a map[string]any) { a["buttons"] = 1 },
	} {
		var a map[string]any
		_ = json.Unmarshal(valid, &a)
		change(a)
		raw, _ := json.Marshal(a)
		next, err := module.Apply(ctx, m, raw)
		if err == nil || !reflect.DeepEqual(next, m) || module.AcceptStale(ctx, m, raw) {
			t.Fatal("invalid batch was accepted or mutated state", string(raw), err)
		}
	}
}

func TestHostBatchQueueLimitAndCancellationRecovery(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	ctx.Actor = 0
	events := make([]map[string]any, 64)
	for n := range events {
		buttons := engine.ButtonB
		if n%2 == 1 {
			buttons = 0
		}
		events[n] = inputEvent(int64(n+1), buttons, n*7, false)
	}
	var err error
	m, err = module.Apply(ctx, m, inputBatch(t, m, 64, events...))
	if err != nil {
		t.Fatal(err)
	}
	// Validate through both entry points. Neither may mutate the old queue.
	overflow := inputBatch(t, m, 65, inputEvent(65, engine.ButtonA, 0, false))
	next, err := module.Apply(ctx, m, overflow)
	if err == nil || !reflect.DeepEqual(next, m) || module.AcceptStale(ctx, m, overflow) {
		t.Fatal("queue overflow was not rejected atomically", err)
	}
	recovery := inputBatch(t, m, 66, inputEvent(65, 0, 0, true), inputEvent(66, engine.ButtonRight, 25, false))
	if !module.AcceptStale(ctx, m, recovery) {
		t.Fatal("cancellation cannot recover a full queue")
	}
	next, err = module.Apply(ctx, m, recovery)
	if err != nil {
		t.Fatal(err)
	}
	s := hostState(t, next)
	if len(s.Inputs[0].Changes) != 1 || s.Inputs[0].Changes[0].At != ctx.Now.UnixMilli()+25 {
		t.Fatal("cancel did not reset the old future timeline", s.Inputs[0])
	}
}

func TestHostBatchRetryDoesNotExtendLostTransport(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	ctx.Actor = 0
	packet := inputBatch(t, m, 1, inputEvent(1, engine.ButtonRight, 0, false))
	var err error
	m, err = module.Apply(ctx, m, packet)
	if err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 8; n++ {
		m = hostTick(t, module, &ctx, m)
		ctx.Actor = 0
		before := m
		m, err = module.Apply(ctx, m, packet)
		if err != nil || !reflect.DeepEqual(m, before) {
			t.Fatal("retry refreshed old hold", err)
		}
	}
	if hostState(t, m).Inputs[0].Buttons != 0 {
		t.Fatal("retries kept disconnected input held")
	}
}

// Cancelling a touch button while a keyboard direction remains held must not
// introduce a second direction edge or erase another source's queued tap.
func TestHostBatchPartialCancellationPreservesOtherInputTimeline(t *testing.T) {
	module, ctx, m := hostPlaying(t)
	ctx.Actor = 0
	expected := hostState(t, m).Runtime
	cancel := inputEvent(5, 0, 80, true)
	cancel["cancelMask"] = engine.ButtonB
	packet := inputBatch(t, m, 5,
		inputEvent(1, engine.ButtonRight, 0, false),
		inputEvent(2, engine.ButtonRight|engine.ButtonB, 20, false),
		inputEvent(3, engine.ButtonB, 40, false),
		inputEvent(4, engine.ButtonRight|engine.ButtonB, 60, false), cancel)
	var err error
	m, err = module.Apply(ctx, m, packet)
	if err != nil {
		t.Fatal(err)
	}
	for _, held := range []engine.Buttons{engine.ButtonRight, engine.ButtonRight, 0, engine.ButtonRight, engine.ButtonRight, engine.ButtonRight} {
		r, e := engine.StepVideo(expected, [4]engine.Buttons{held})
		if e != nil {
			t.Fatal(e)
		}
		expected = r.State
	}
	m = hostTick(t, module, &ctx, m)
	if !reflect.DeepEqual(hostState(t, m).Runtime, expected) {
		t.Fatal("partial cancel changed the uncancelled direction timeline")
	}
}
