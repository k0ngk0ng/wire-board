package engine_test

// Test-first native integration. Failure inventory: wrong original resource
// identity; wrong ABI/build; missing host calls silently succeeding; C locals,
// VFS cursors, clock or RNG lost on fresh-instance restore; shared C globals;
// corrupt/truncated snapshot; canceled execution escaping its request budget;
// host filesystem/network access during rule callbacks; invalid continuation.
// These tests run original 98 data through the actual compiled C rule core.

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

type startupOnlyFS struct {
	fs.FS
	locked atomic.Bool
}

func (f *startupOnlyFS) Open(name string) (fs.File, error) {
	if f.locked.Load() {
		return nil, fs.ErrPermission
	}
	return f.FS.Open(name)
}

func nativeRuntime(t *testing.T) *engine.Runtime {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	program, err := os.ReadFile(filepath.Join(root, ".local", "pal", "native", "rules.wasm"))
	if err != nil {
		t.Skip("build the native rule core with python3 -B games/pal/build/native.py")
	}
	files := &startupOnlyFS{FS: os.DirFS(filepath.Join(root, ".local", "pal", "original-98"))}
	runtime, err := engine.NewRuntime(context.Background(), program, files)
	if err != nil {
		t.Fatal(err)
	}
	files.locked.Store(true)
	t.Cleanup(func() {
		if err := runtime.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return runtime
}

func nextMessage(t *testing.T, runtime *engine.Runtime, snapshot engine.Snapshot) engine.Snapshot {
	t.Helper()
	wait := uint32(snapshot.Event.Value)
	for i := 0; i < 4096; i++ {
		if snapshot.Event.Kind == engine.Dialogue || snapshot.Event.Kind == engine.SceneComplete {
			return snapshot
		}
		advanced, err := runtime.AdvanceTimed(context.Background(), snapshot, engine.Input{Direction: 4}, wait, 250)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, wait = advanced.Snapshot, advanced.RemainingMS
	}
	t.Fatal("original script did not reach its next message")
	return snapshot
}

func TestNativeOriginalOpeningSaveRestore(t *testing.T) {
	runtime := nativeRuntime(t)
	snapshot, err := runtime.StartOpening(context.Background(), 12345)
	if err != nil {
		t.Fatal(err)
	}
	wait := uint32(snapshot.Event.Value)
	for i := 0; i < 4096 && snapshot.View.Messages < 3; i++ {
		if snapshot.Event.Kind == engine.Dialogue {
			t.Fatal("original timed opening gained a manual wait")
		}
		advanced, e := runtime.AdvanceTimed(context.Background(), snapshot, engine.Input{Direction: 4}, wait, 250)
		if e != nil {
			t.Fatal(e)
		}
		snapshot, wait = advanced.Snapshot, advanced.RemainingMS
	}
	if snapshot.View.Messages != 3 || snapshot.View.Scene != 1 || snapshot.View.X != 1024 || snapshot.View.Y != 1024 || snapshot.View.Sprite != 193 || !reflect.DeepEqual(snapshot.View.Party, []int{0}) {
		t.Fatal("original opening state differs", snapshot.View, snapshot.Event)
	}
	if len(snapshot.View.Overlay) == 0 {
		t.Fatal("mid-dialogue save lost original text draw calls")
	}
	encoded, err := runtime.Encode(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 2*1024*1024 {
		t.Fatal("compressed rule snapshot exceeds budget", len(encoded))
	}
	// A new Runtime has no surviving native module instance or external cursors.
	fresh := nativeRuntime(t)
	restored, err := fresh.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, restored) {
		t.Fatal("saved C state changed")
	}
	finish := func(runtime *engine.Runtime, state engine.Snapshot) engine.Snapshot {
		remaining := uint32(state.Event.Value)
		for i := 0; i < 4096; i++ {
			if state.Event.Kind == engine.SceneComplete {
				return state
			}
			advanced, err := runtime.AdvanceTimed(context.Background(), state, engine.Input{Direction: 4}, remaining, 250)
			if err != nil {
				t.Fatal(err)
			}
			state, remaining = advanced.Snapshot, advanced.RemainingMS
		}
		t.Fatal("original opening did not finish")
		return state
	}
	a := finish(runtime, snapshot)
	if restored.View.Scene != 1 {
		t.Fatal("one runtime mutated another saved state")
	}
	b := finish(fresh, restored)
	if !reflect.DeepEqual(a, b) || a.View.Scene != 2 || a.View.Messages != 5 {
		t.Fatal("fresh runtime continuation differs", a.View, b.View)
	}
	if snapshot.View.Scene != 1 || snapshot.View.Messages != 3 {
		t.Fatal("Step mutated input snapshot")
	}
	for name, bad := range map[string][]byte{"truncated": encoded[:len(encoded)/2], "malformed": []byte(`{"version":1}`)} {
		t.Run(name, func(t *testing.T) {
			if _, err := fresh.Decode(bad); err == nil {
				t.Fatal("corrupt save accepted")
			}
		})
	}
	wrong := snapshot
	wrong.BuildID = "different-build"
	if _, err := fresh.Step(context.Background(), wrong, 0); err == nil {
		t.Fatal("wrong native build accepted")
	}
	wrong = snapshot
	wrong.ResourceID = "different-original-data"
	if _, err := fresh.Step(context.Background(), wrong, 0); err == nil {
		t.Fatal("wrong original data accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fresh.Step(ctx, snapshot, 0); err == nil {
		t.Fatal("canceled rule step ran")
	}
	start := time.Now()
	if _, err := fresh.Step(context.Background(), snapshot, 0); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": !t.Failed(), "scope": "original 98 script on Go/Wasm, compressed save and fresh Runtime restore; not full game", "savedBytes": len(encoded), "stepMS": float64(time.Since(start).Microseconds()) / 1000, "savedView": snapshot.View, "finalView": b.View}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "native-go-opening.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
