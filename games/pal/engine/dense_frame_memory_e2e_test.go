package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestOriginalDenseFrameMemoryAndIsolation(t *testing.T) {
	scope := os.Getenv("PAL_DENSE_MEMORY_E2E")
	if scope == "" {
		t.Skip("explicit original battle memory measurement")
	}
	if !regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`).MatchString(scope) {
		t.Fatal("invalid dense frame evidence scope", scope)
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	wasm, err := os.ReadFile(filepath.Join(root, ".local/pal/dense-battle-candidate/battle/rules.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	r, err := NewRuntime(ctx, wasm, os.DirFS(filepath.Join(root, ".local/pal/original-98")))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close(ctx)
	original, err := r.StartCampaignForPlayers(ctx, 12345, 1)
	if err != nil {
		t.Fatal(err)
	}
	step := func(s Snapshot, keys uint32) Snapshot {
		t.Helper()
		next, err := r.StepWithInput(ctx, s, Input{Direction: 4, Keys: keys})
		if err != nil {
			t.Fatal(err)
		}
		return next
	}
	for i := 0; i < 32 && original.View.Battle.State != 1; i++ {
		original = step(original, 0)
	}
	if original.View.Battle.State != 1 || original.Event != (Event{Frame, 40}) {
		t.Fatal("original team did not reach the 40 ms command selector")
	}
	original = step(step(original, 32), 2)
	for i := 0; i < 8; i++ {
		original = step(original, 0)
	}
	if original.View.Battle.Phase != 1 || !slices.Contains(original.View.BattleAssets, [2]int{8, 50}) {
		t.Fatal("fixture did not enter the original cooperative action")
	}
	originalBytes, err := r.Encode(original)
	if err != nil {
		t.Fatal(err)
	}
	neutral := func(uint32) Input { return Input{Direction: 4} }
	advance := func(encoded []byte) ([]byte, error) {
		txn, err := r.Continue(encoded)
		if err != nil {
			return nil, err
		}
		result, err := txn.AdvanceScheduled(ctx, 40, 40, neutral)
		if err != nil {
			return nil, err
		}
		if result.Steps != 1 || result.RemainingMS != 40 {
			return nil, invalidDenseFrame{result.Steps, result.RemainingMS}
		}
		return txn.Encode()
	}
	want, err := r.AdvanceScheduled(ctx, original, 40, 40, neutral)
	if err != nil {
		t.Fatal(err)
	}
	expectedBytes, err := r.Encode(want.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var parallel sync.WaitGroup
	for range 4 {
		parallel.Go(func() {
			got, e := advance(originalBytes)
			if e != nil || !bytes.Equal(got, expectedBytes) {
				t.Errorf("independent continuations do not reproduce original snapshot: %v", e)
			}
		})
	}
	parallel.Wait()
	if known, err := r.Encode(original); err != nil || !bytes.Equal(known, originalBytes) {
		t.Fatal("public input snapshot was mutated", err)
	}
	txn, err := r.Continue(originalBytes)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	if _, err := txn.AdvanceScheduled(ctx, 40, 80, func(offset uint32) Input {
		calls++
		if offset > 40 {
			return Input{Direction: 99}
		}
		return neutral(offset)
	}); err == nil || calls != 2 {
		t.Fatal("second original step should fail after a valid step", calls, err)
	}
	if known, err := txn.Encode(); err != nil || !bytes.Equal(known, originalBytes) {
		t.Fatal("failed second step changed the committed continuation", err)
	}
	handle := *txn
	if _, err := handle.AdvanceScheduled(ctx, 40, 40, neutral); err != nil {
		t.Fatal(err)
	}
	if throughOwner, err := txn.Encode(); err != nil || !bytes.Equal(throughOwner, expectedBytes) {
		t.Fatal("a copy of the same transaction handle detached metadata from native memory", err)
	}
	const frames = 58
	encoded := originalBytes
	timings := make([]float64, 0, frames)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range frames {
		began := time.Now()
		encoded, err = advance(encoded)
		if err != nil {
			t.Fatal(err)
		}
		timings = append(timings, float64(time.Since(began))/float64(time.Millisecond))
	}
	runtime.ReadMemStats(&after)
	got, err := r.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	end := original
	for range frames {
		end = step(end, 0)
	}
	if !reflect.DeepEqual(got, end) || got.View.Clock-original.View.Clock != 40*frames {
		t.Fatal("owned capture changed the original motion, damage, RNG, or timing")
	}
	slices.Sort(timings)
	report := map[string]any{
		"passed": true, "scope": "actual original dense 40 ms battle frames, allocation and wall time, public/owned byte parity, four concurrent restores and rollback after the first real step",
		"buildID": got.BuildID, "memoryBytes": len(got.Memory), "frames": frames, "originalElapsedMS": got.View.Clock - original.View.Clock,
		"allocatedBytesPerFrame": (after.TotalAlloc - before.TotalAlloc) / frames, "frameMSP95": timings[(len(timings)-1)*95/100],
		"failedAfterValidStepPreserved": true, "parallelIndependent": true, "exactEncodedPublicParity": true, "firstMemoryID": original.MemoryID, "lastMemoryID": got.MemoryID,
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(root, ".local/pal/dense-frame-memory-"+scope+".json")
	file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Write(append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	t.Log(string(raw))
}

type invalidDenseFrame struct {
	steps     int
	remaining uint32
}

func (e invalidDenseFrame) Error() string {
	raw, _ := json.Marshal([]int{e.steps, int(e.remaining)})
	return "original dense frame did not retain its cadence: " + string(raw)
}
