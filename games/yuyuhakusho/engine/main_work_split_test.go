package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestMainWorkNativeSplitTemporarySave(t *testing.T) {
	packed, err := os.ReadFile("../verification/battle-sample-irq/native-battle-prefix.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Fields []string
		Cases  []struct{ Events [][]uint32 }
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	indices := map[string]int{}
	for i, n := range data.Fields {
		indices[n] = i
	}
	seen := 0
	for _, row := range data.Cases {
		for i, e := range row.Events {
			if e[indices["kind"]] != 1 || e[indices["pc"]] != 0xfff01e {
				continue
			}
			var next []uint32
			for _, candidate := range row.Events[i+1:] {
				if candidate[indices["kind"]] == 1 {
					next = candidate
					break
				}
			}
			if next == nil || next[indices["pc"]] != 0xfff022 || next[indices["line"]] != e[indices["line"]] {
				t.Fatal("native temporary-save context missing")
			}
			op := MainWorkOperation{Phase: "save-temporaries", CPUCycles: 112, InternalRefresh: true, TailCycles: 8}
			cycle, refresh, err := op.Advance(uint64(e[indices["cycle"]]), uint64(e[indices["refresh"]]))
			if err != nil || cycle != uint64(next[indices["cycle"]]) || refresh != uint64(next[indices["refresh"]]) {
				t.Fatal("internal refresh split differs", e, next, cycle, refresh, err)
			}
			seen++
		}
	}
	if seen != 23 {
		t.Fatal("native split coverage missing", seen)
	}
	for _, op := range []MainWorkOperation{{Phase: "bad-tail", CPUCycles: 8, TailCycles: 8}, {Phase: "bad-tail", CPUCycles: 112, InternalRefresh: true, TailCycles: 255}} {
		if _, _, err := op.Advance(0, 896); err == nil {
			t.Fatal("invalid split work accepted", op)
		}
	}
}
