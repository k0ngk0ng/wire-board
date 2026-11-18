package engine_test

// Before timed batching: tiny original delays run at the platform tick rate;
// batching drops intermediate rules or key edges, skips dialogue, mutates the
// candidate on failure, or loses partially elapsed waits across save/restore.
import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestNativeTimedBatchMatchesOriginalSteps(t *testing.T) {
	r := nativeRuntime(t)
	s, err := r.StartCampaign(context.Background(), 12345)
	if err != nil {
		t.Fatal(err)
	}
	initial := s.MemoryID
	wait := uint32(s.Event.Value)
	baseline := s
	remaining, steps := wait, 0
	budget := uint32(250)
	for baseline.Event.Kind == engine.Delay || baseline.Event.Kind == engine.Frame {
		if remaining > budget {
			remaining -= budget
			break
		}
		budget -= remaining
		baseline, err = r.Step(context.Background(), baseline, 0)
		if err != nil {
			t.Fatal(err)
		}
		steps++
		remaining = 0
		if baseline.Event.Kind == engine.Delay || baseline.Event.Kind == engine.Frame {
			remaining = uint32(baseline.Event.Value)
		}
	}
	batched, err := r.AdvanceTimed(context.Background(), s, engine.Input{Direction: 4}, wait, 250)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(batched.Snapshot, baseline) || batched.RemainingMS != remaining || batched.Steps != steps || s.MemoryID != initial {
		t.Fatal("batch differs from unchanged original steps", batched.Snapshot.View, baseline.View, batched.RemainingMS, remaining, batched.Steps, steps)
	}
	// Partial waits are independent of native memory and survive serialization.
	if wait > 1 {
		part, e := r.AdvanceTimed(context.Background(), s, engine.Input{Direction: 4}, wait, 1)
		if e != nil {
			t.Fatal(e)
		}
		if part.Steps != 0 || part.RemainingMS != wait-1 || !reflect.DeepEqual(part.Snapshot, s) {
			t.Fatal("partial wait advanced rules")
		}
		encoded, e := r.Encode(part.Snapshot)
		if e != nil {
			t.Fatal(e)
		}
		restored, e := r.Decode(encoded)
		if e != nil {
			t.Fatal(e)
		}
		a, e := r.AdvanceTimed(context.Background(), restored, engine.Input{Direction: 4}, part.RemainingMS, 249)
		if e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(a.Snapshot, batched.Snapshot) || a.RemainingMS != batched.RemainingMS {
			t.Fatal("split elapsed time changed rules")
		}
	}
	dialogue := nextMessage(t, r, s)
	frozen, err := r.AdvanceTimed(context.Background(), dialogue, engine.Input{Direction: 4}, 0, 250)
	if err != nil || frozen.Steps != 0 || !reflect.DeepEqual(frozen.Snapshot, dialogue) {
		t.Fatal("timer skipped original dialogue", err)
	}
	for _, invalid := range []struct{ wait, elapsed uint32 }{{wait + 1, 1}, {wait, 251}} {
		if _, err := r.AdvanceTimed(context.Background(), s, engine.Input{Direction: 4}, invalid.wait, invalid.elapsed); err == nil {
			t.Fatal("invalid time budget accepted")
		}
	}
	for _, input := range []engine.Input{{Direction: 5}, {Direction: 4, Keys: 1 << 18}} {
		if _, err := r.AdvanceTimed(context.Background(), s, input, wait, 0); err == nil {
			t.Fatal("invalid input accepted during a partial wait")
		}
	}
	out := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	data, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "nativeSteps": steps, "elapsedMS": 250, "scope": "real original campaign; batch equals sequential continuation, partial wait restore and dialogue barrier"}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "timed-batch.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
