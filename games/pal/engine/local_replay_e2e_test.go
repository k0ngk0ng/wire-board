package engine_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
)

func TestOriginalLocalReplayCheckpointAndNativeOperations(t *testing.T) {
	runtime := nativeRuntime(t)
	initial := originalInn(t, runtime)
	saved, err := runtime.Encode(initial)
	if err != nil {
		t.Fatal(err)
	}
	native, err := runtime.Continue(saved)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := native.ClientCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	input, err := gzip.NewReader(bytes.NewReader(checkpoint))
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(input)
	input.Close()
	if err != nil || len(payload) < 16 || string(payload[:8]) != "PALNET01" {
		t.Fatal("invalid original browser checkpoint", err)
	}
	size := int(binary.LittleEndian.Uint32(payload[8:12]))
	memory := int(binary.LittleEndian.Uint32(payload[12:16]))
	var metadata struct {
		Snapshot engine.Snapshot `json:"snapshot"`
		Words    []string        `json:"words"`
		Messages []string        `json:"messages"`
		Roles    []string        `json:"roles"`
	}
	if err := json.Unmarshal(payload[16:16+size], &metadata); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload[16+size:], initial.Memory) || memory != len(initial.Memory) || metadata.Snapshot.Memory != nil ||
		metadata.Snapshot.BuildID != initial.BuildID || len(metadata.Words) < 500 || len(metadata.Messages) < 1000 || len(metadata.Roles) != 6 {
		t.Fatal("browser checkpoint lost original memory, version or text")
	}
	remaining := uint32(initial.Event.Value)
	type result struct {
		Operations []engine.ReplayOperation `json:"operations"`
		Checkpoint []byte                   `json:"checkpoint"`
	}
	results := []result{}
	appendResult := func() {
		t.Helper()
		operations := native.TakeReplay()
		if len(operations) == 0 {
			t.Fatal("original mutation did not produce a replayable operation")
		}
		if len(native.TakeReplay()) != 0 {
			t.Fatal("replay observations repeated a committed operation")
		}
		next, err := native.ClientCheckpoint()
		if err != nil {
			t.Fatal(err)
		}
		results = append(results, result{operations, next})
	}
	for _, keys := range []uint32{16, 4, 1} {
		advanced, err := native.AdvanceScheduled(t.Context(), remaining, 100, func(uint32) engine.Input {
			return engine.Input{Direction: 4, Keys: keys}
		})
		if err != nil {
			t.Fatal(err)
		}
		remaining = advanced.RemainingMS
		appendResult()
	}
	for i := 0; i < 32 && native.View().Menu.Kind != 1; i++ {
		advanced, err := native.AdvanceScheduled(t.Context(), remaining, 100, func(uint32) engine.Input { return engine.Input{Direction: 4} })
		if err != nil {
			t.Fatal(err)
		}
		remaining = advanced.RemainingMS
		appendResult()
	}
	if native.View().Menu.Kind != 1 {
		t.Fatal("local replay fixture did not open the original menu")
	}
	if err := native.ReleaseInputs(t.Context()); err != nil {
		t.Fatal(err)
	}
	appendResult()
	for _, keys := range []uint32{2, 1, 1, 4, 4, 2, 1, 16, 4, 32, 8} {
		advanced, err := native.AdvanceScheduled(t.Context(), remaining, 100, func(uint32) engine.Input { return engine.Input{Direction: 4, Keys: keys} })
		if err != nil {
			t.Fatal(err)
		}
		remaining = advanced.RemainingMS
		if advanced.Steps > 0 {
			appendResult()
		}
	}
	before, err := native.Encode()
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := native.Step(cancelled, 1); err == nil || len(native.TakeReplay()) != 0 {
		t.Fatal("failed native step leaked into the replay journal")
	}
	after, err := native.Encode()
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed replay mutated the native save", err)
	}
	copyRuntime := nativeRuntime(t)
	copyState, err := copyRuntime.Decode(after)
	if err != nil || !reflect.DeepEqual(copyState.View, native.View().StateView) {
		t.Fatal("network replay changed the existing original save format", err)
	}
	output := filepath.Join("..", "..", "..", ".local/pal/verification/local-first")
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{"passed": true, "initial": checkpoint, "results": results, "core": initial.BuildID})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "native-replay-fixture.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
