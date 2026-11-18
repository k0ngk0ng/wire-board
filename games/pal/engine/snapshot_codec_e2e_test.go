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
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	"github.com/klauspost/compress/zstd"
)

func gzipSnapshotBody(t *testing.T, body []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	writer, e := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = writer.Write(body); e != nil {
		t.Fatal(e)
	}
	if e = writer.Close(); e != nil {
		t.Fatal(e)
	}
	return compressed.Bytes()
}
func legacySnapshot(t *testing.T, snapshot engine.Snapshot) []byte {
	t.Helper()
	body, e := json.Marshal(snapshot)
	if e != nil {
		t.Fatal(e)
	}
	return gzipSnapshotBody(t, body)
}
func uncompressedSnapshot(t *testing.T, encoded []byte) []byte {
	t.Helper()
	if bytes.HasPrefix(encoded, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		reader, e := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1), zstd.WithDecoderMaxMemory(136*1024*1024))
		if e != nil {
			t.Fatal(e)
		}
		defer reader.Close()
		body, e := reader.DecodeAll(encoded, nil)
		if e != nil {
			t.Fatal(e)
		}
		return body
	}
	reader, e := gzip.NewReader(bytes.NewReader(encoded))
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	body, e := io.ReadAll(reader)
	if e != nil {
		t.Fatal(e)
	}
	return body
}

func TestNativeSnapshotBinaryEnvelopeAndLegacyContinuation(t *testing.T) {
	r := nativeRuntime(t)
	inn := originalInn(t, r)
	menu := menuUntil(t, r, menuStep(t, r, inn, 1), 1)
	for _, state := range []engine.Snapshot{inn, menu} {
		encoded, e := r.Encode(state)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.HasPrefix(encoded, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
			t.Fatal("native snapshots still use the slower gzip writer")
		}
		body := uncompressedSnapshot(t, encoded)
		if len(body) < 16 || string(body[:8]) != "PALMEM01" {
			t.Fatal("native memory still uses the legacy JSON/base64 save envelope")
		}
		metadataSize, memorySize := int(binary.LittleEndian.Uint32(body[8:])), int(binary.LittleEndian.Uint32(body[12:]))
		if metadataSize <= 0 || memorySize != len(state.Memory) || 16+metadataSize+memorySize != len(body) {
			t.Fatal("binary snapshot lengths changed native memory")
		}
		for _, saved := range [][]byte{encoded, gzipSnapshotBody(t, body), legacySnapshot(t, state)} {
			restored, e := r.Decode(saved)
			if e != nil || !reflect.DeepEqual(restored, state) {
				t.Fatal("current/legacy codec changed original continuation", e)
			}
			fresh := nativeRuntime(t)
			continued, e := fresh.Decode(saved)
			if e != nil || !reflect.DeepEqual(continued, state) {
				t.Fatal("fresh runtime lost original snapshot", e)
			}
			next, e := fresh.Step(context.Background(), continued, 0)
			if e != nil {
				t.Fatal(e)
			}
			want, e := r.Step(context.Background(), state, 0)
			if e != nil || !reflect.DeepEqual(next, want) {
				t.Fatal("codec changed fresh original execution", e)
			}
		}
		decoded, e := r.Decode(encoded)
		if e != nil {
			t.Fatal(e)
		}
		decoded.Memory[len(decoded.Memory)-1] ^= 1
		if len(decoded.View.Menu.Items) > 0 {
			decoded.View.Menu.Items[0].Label = "mutated caller metadata"
		}
		again, e := r.Decode(encoded)
		if e != nil || !reflect.DeepEqual(again, state) {
			t.Fatal("decoded native buffers alias persisted input", e)
		}
		for _, mutate := range []func([]byte) []byte{
			func(b []byte) []byte { return b[:len(b)-1] },
			func(b []byte) []byte { return append(b, 0) },
			func(b []byte) []byte { binary.LittleEndian.PutUint32(b[8:], ^uint32(0)); return b },
			func(b []byte) []byte { binary.LittleEndian.PutUint32(b[12:], ^uint32(0)); return b },
			func(b []byte) []byte { b[len(b)-1] ^= 1; return b },
			func(b []byte) []byte {
				metadata := bytes.Replace(b[16:16+metadataSize], []byte(`"version":2`), []byte(`"unknown":1,"version":2`), 1)
				changed := append(bytes.Clone(b[:16]), metadata...)
				changed = append(changed, b[16+metadataSize:]...)
				binary.LittleEndian.PutUint32(changed[8:], uint32(len(metadata)))
				return changed
			},
		} {
			if _, e = r.Decode(gzipSnapshotBody(t, mutate(bytes.Clone(body)))); e == nil {
				t.Fatal("corrupt binary native snapshot accepted")
			}
		}
		for _, changed := range [][]byte{encoded[:len(encoded)-1], append(bytes.Clone(encoded), 0), append(bytes.Clone(encoded), encoded...)} {
			if _, e = r.Decode(changed); e == nil {
				t.Fatal("truncated/trailing/multistream compressed snapshot accepted")
			}
		}
	}
	var group sync.WaitGroup
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			state := []engine.Snapshot{inn, menu}[index%2]
			for n := 0; n < 2; n++ {
				encoded, e := r.Encode(state)
				if e != nil {
					t.Error(e)
					return
				}
				decoded, e := r.Decode(encoded)
				if e != nil || !reflect.DeepEqual(decoded, state) {
					t.Error("concurrent original codec mixed snapshots", e)
					return
				}
			}
		}(i)
	}
	group.Wait()
}

func TestNativeSnapshotCodecBudget(t *testing.T) {
	if os.Getenv("PAL_CODEC_BENCH") != "1" {
		t.Skip("explicit isolated native codec performance measurement")
	}
	tag := os.Getenv("PAL_CODEC_TAG")
	if tag == "" {
		tag = "current"
	}
	if tag != "baseline" && tag != "binary" && tag != "current" && tag != "original-current" {
		t.Fatal("unsupported codec report tag")
	}
	open := func() *engine.Runtime {
		if tag != "original-current" {
			return nativeRuntime(t)
		}
		program, err := os.ReadFile("../../../.local/pal/runtime-candidates/audio/original/rules.wasm")
		if err != nil {
			t.Fatal(err)
		}
		archive, err := os.ReadFile("../../../.local/pal/authority-prepared/authority.zip")
		if err != nil {
			t.Fatal(err)
		}
		runtime, err := engine.NewAuthorityRuntime(context.Background(), program, archive)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { runtime.Close(context.Background()) })
		return runtime
	}
	r := open()
	state := originalInn(t, r)
	states := []engine.Snapshot{state}
	for len(states) < 12 {
		next, e := r.Step(context.Background(), states[len(states)-1], 0)
		if e != nil {
			t.Fatal(e)
		}
		states = append(states, next)
	}
	fresh := open()
	encoded, e := r.Encode(state)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.Decode(encoded); e != nil {
		t.Fatal(e)
	}
	var encodeMS, decodeMS []float64
	for i := 0; i < 12; i++ {
		state = states[i]
		began := time.Now()
		encoded, e = r.Encode(state)
		encodeMS = append(encodeMS, float64(time.Since(began))/float64(time.Millisecond))
		if e != nil {
			t.Fatal(e)
		}
		began = time.Now()
		restored, e := fresh.Decode(encoded)
		decodeMS = append(decodeMS, float64(time.Since(began))/float64(time.Millisecond))
		if e != nil || restored.MemoryID != state.MemoryID {
			t.Fatal("measured decode changed native state", e)
		}
	}
	quantile := func(values []float64, p float64) float64 {
		copy := append([]float64(nil), values...)
		sort.Float64s(copy)
		return copy[int(float64(len(copy)-1)*p+.5)]
	}
	passed := quantile(encodeMS, .95) <= 5 && quantile(decodeMS, .95) <= 5
	result := map[string]any{"passed": passed, "scope": "12 distinct consecutive original inn snapshots and a separate decoding runtime; isolated codec measurement, not platform latency acceptance", "runtimeIdentity": r.Identity(), "samples": 12, "memoryBytes": len(state.Memory), "encodedBytes": len(encoded), "legacyBytes": len(legacySnapshot(t, state)), "encodeMS": encodeMS, "decodeMS": decodeMS, "encodeP95MS": quantile(encodeMS, .95), "decodeP95MS": quantile(decodeMS, .95), "budgetP95MS": 5}
	data, _ := json.MarshalIndent(result, "", "  ")
	directory := filepath.Join("..", "..", "..", ".local/pal/performance")
	if e = os.MkdirAll(directory, 0755); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(directory, "codec-"+tag+".json"), append(data, '\n'), 0600); e != nil {
		t.Fatal(e)
	}
	t.Log(string(data))
	if !passed {
		t.Fatal("frozen native codec performance budgets are not met")
	}
}
