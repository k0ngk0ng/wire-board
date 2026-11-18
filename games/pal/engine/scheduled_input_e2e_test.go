package engine_test

// Test first against original GameMain. Failures: a schedule is sampled at the
// start/end of a batch instead of original continuation due times; a partial
// wait consumes input; new-menu input is consumed by the previous controller;
// an invalid/canceled schedule mutates its source snapshot. This is native
// integration, not a replacement game or a mock of its frame loop.
import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestNativeScheduledInputDueTimesAndMenuBoundary(t *testing.T) {
	ctx := context.Background()
	r := nativeRuntime(t)
	inn := originalInn(t, r)
	wait := uint32(inn.Event.Value)
	if wait != 100 {
		t.Fatal("original exploration frame changed", wait)
	}
	var calls []uint32
	whole, e := r.AdvanceScheduled(ctx, inn, wait, 250, func(at uint32) engine.Input {
		calls = append(calls, at)
		keys := uint32(0)
		if at == 100 {
			keys = 4
		}
		return engine.Input{Direction: 4, Keys: keys}
	})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(calls, []uint32{100, 200}) || whole.Snapshot.View.X != inn.View.X-16 || whole.Snapshot.View.Y != inn.View.Y+8 || whole.RemainingMS != 50 {
		t.Fatal("schedule did not follow the actual original continuation clock", calls, whole.Snapshot.View.X, whole.Snapshot.View.Y, whole.RemainingMS)
	}
	first, e := r.AdvanceTimed(ctx, inn, engine.Input{Direction: 4, Keys: 4}, wait, 100)
	if e != nil {
		t.Fatal(e)
	}
	if first.Snapshot.View.X != inn.View.X-16 || first.Snapshot.View.Y != inn.View.Y+8 {
		t.Fatal("input arriving during the original wait was deferred to a later frame")
	}
	second, e := r.AdvanceTimed(ctx, first.Snapshot, engine.Input{Direction: 4}, first.RemainingMS, 150)
	if e != nil || !reflect.DeepEqual(whole.Snapshot, second.Snapshot) || whole.RemainingMS != second.RemainingMS {
		t.Fatal("scheduled batch differs from sequential original execution", e)
	}
	partialCalls := 0
	partial, e := r.AdvanceScheduled(ctx, inn, wait, 99, func(uint32) engine.Input {
		partialCalls++
		return engine.Input{Direction: 0, Keys: 4}
	})
	if e != nil || partialCalls != 0 || partial.Steps != 0 || partial.RemainingMS != 1 || !reflect.DeepEqual(partial.Snapshot, inn) {
		t.Fatal("partial original wait consumed a scheduled input", e, partialCalls)
	}
	menuCalls := 0
	menu, e := r.AdvanceScheduled(ctx, inn, wait, 250, func(uint32) engine.Input {
		menuCalls++
		if menuCalls == 1 {
			return engine.Input{Direction: 4, Keys: 1}
		}
		return engine.Input{Direction: 4, Keys: 2}
	})
	if e != nil || menuCalls != 1 || !menu.InputReset || menu.Snapshot.View.Menu.Kind != 1 {
		t.Fatal("a scheduled confirm crossed the original main-menu boundary", e, menuCalls, menu.Snapshot.View.Menu)
	}
	source := bytes.Clone(inn.Memory)
	invalidCalls := 0
	if _, e = r.AdvanceScheduled(ctx, inn, wait, 250, func(uint32) engine.Input {
		invalidCalls++
		if invalidCalls == 1 {
			return engine.Input{Direction: 4, Keys: 4}
		}
		return engine.Input{Direction: 5}
	}); e == nil || !bytes.Equal(source, inn.Memory) {
		t.Fatal("invalid later input mutated its source snapshot", e)
	}
	if _, e = r.AdvanceScheduled(ctx, inn, wait, 250, nil); e == nil {
		t.Fatal("nil input schedule accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = r.AdvanceScheduled(canceled, inn, wait, 250, func(uint32) engine.Input { t.Fatal("canceled schedule was called"); return engine.Input{} }); e == nil {
		t.Fatal("canceled scheduled execution accepted")
	}
	data, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "actual original C frame loop, scheduled continuation offsets, exact native snapshot equality, partial waits and menu-boundary suppression", "dueTimes": calls, "menuProviderCalls": menuCalls, "remainingMS": whole.RemainingMS}, "", "  ")
	if e = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/scheduled-input-native.json"), append(data, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
}
