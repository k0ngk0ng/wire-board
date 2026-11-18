package engine_test

// Test first. A transaction must validate untrusted bytes before ownership,
// preserve the original C continuation, keep all native memory private, detach
// caller input/presentation, and publish no native mutation after a failed or
// canceled rule call. Separate transactions must not share Wasm instances.
import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestOriginalOwnedContinuation(t *testing.T) {
	r := nativeRuntime(t)
	inn := originalInn(t, r)
	menu := menuUntil(t, r, menuStep(t, r, inn, 1), 1)
	for _, state := range []engine.Snapshot{inn, menu} {
		encoded, e := r.Encode(state)
		if e != nil {
			t.Fatal(e)
		}
		ownedBytes := bytes.Clone(encoded)
		transaction, e := r.Continue(ownedBytes)
		if e != nil {
			t.Fatal(e)
		}
		clear(ownedBytes)
		want, e := r.View(state)
		if e != nil || !reflect.DeepEqual(transaction.View(), want) || transaction.Event() != state.Event {
			t.Fatal("owned continuation changed original public view", e)
		}
		v := transaction.View()
		v.Party[0] = 5
		if len(v.Menu.Items) > 0 {
			v.Menu.Items[0].Label = "caller mutation"
		}
		if !reflect.DeepEqual(transaction.View(), want) {
			t.Fatal("caller presentation aliases private continuation")
		}
		before, e := transaction.Encode()
		if e != nil {
			t.Fatal(e)
		}
		canceled, cancel := context.WithCancel(context.Background())
		cancel()
		if e = transaction.Step(canceled, 0); e == nil {
			t.Fatal("canceled private continuation advanced")
		}
		if _, e = transaction.AdvanceScheduled(context.Background(), uint32(state.Event.Value), 250, func(uint32) engine.Input { return engine.Input{Direction: 99} }); e == nil {
			t.Fatal("invalid private continuation input advanced")
		}
		after, e := transaction.Encode()
		if e != nil || !bytes.Equal(before, after) {
			t.Fatal("failed native transaction published a partial continuation", e)
		}
		expected, e := r.Step(context.Background(), state, 0)
		if e != nil {
			t.Fatal(e)
		}
		if e = transaction.Step(context.Background(), 0); e != nil {
			t.Fatal(e)
		}
		saved, e := transaction.Encode()
		if e != nil {
			t.Fatal(e)
		}
		got, e := r.Decode(saved)
		if e != nil || !reflect.DeepEqual(got, expected) {
			t.Fatal("private transaction differs from original independent rule step", e)
		}
		expected, e = r.ReleaseInputs(context.Background(), expected)
		if e != nil {
			t.Fatal(e)
		}
		if e = transaction.ReleaseInputs(context.Background()); e != nil {
			t.Fatal(e)
		}
		saved, e = transaction.Encode()
		if e != nil {
			t.Fatal(e)
		}
		got, e = r.Decode(saved)
		if e != nil || !reflect.DeepEqual(got, expected) {
			t.Fatal("private transaction changed original input release", e)
		}
		if _, e = r.Continue(encoded[:len(encoded)-1]); e == nil {
			t.Fatal("private transaction accepted truncated native memory")
		}
	}
	encoded, e := r.Encode(inn)
	if e != nil {
		t.Fatal(e)
	}
	schedule := func(uint32) engine.Input { return engine.Input{Direction: 0} }
	want, e := r.AdvanceScheduled(context.Background(), inn, uint32(inn.Event.Value), 250, schedule)
	if e != nil {
		t.Fatal(e)
	}
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Go(func() {
			transaction, e := r.Continue(encoded)
			if e != nil {
				t.Error(e)
				return
			}
			advanced, e := transaction.AdvanceScheduled(context.Background(), uint32(inn.Event.Value), 250, schedule)
			if e != nil || advanced.Steps != want.Steps || advanced.RemainingMS != want.RemainingMS || advanced.InputReset != want.InputReset {
				t.Error("owned timed execution changed original schedule", e)
				return
			}
			saved, e := transaction.Encode()
			if e != nil {
				t.Error(e)
				return
			}
			got, e := r.Decode(saved)
			if e != nil || !reflect.DeepEqual(got, want.Snapshot) {
				t.Error("concurrent transactions mixed native memory", e)
			}
		})
	}
	group.Wait()
	if !t.Failed() {
		report, _ := json.MarshalIndent(map[string]any{"passed": true, "test": t.Name(), "scope": "actual original inn and nested menu; owned native transactions, detached bytes and views, failed/canceled rollback, exact original step/release/scheduled continuation and four concurrent independent instances; not full performance acceptance"}, "", "  ")
		if e = os.WriteFile(filepath.Join("..", "..", "..", ".local/pal/verification/owned-continuation.json"), append(report, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
}
