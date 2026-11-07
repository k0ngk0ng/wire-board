package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
)

type objectPrepareNative struct {
	Name          string
	Before, After ObjectDrawState
	Clock         struct{ Cycle, Refresh uint64 }
	Steps         []struct {
		Cycle, Refresh uint64
		Before         ObjectDrawState
	}
}

func objectPrepareNatives(t *testing.T) []objectPrepareNative {
	t.Helper()
	packed, err := os.ReadFile("../verification/object-prepare-work/native-object-prepare-work.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(packed)
	if hex.EncodeToString(digest[:]) != "40b83b1aad4d76c35ff577dea58d08a13acaf802631f72a9cebbdaa625ee9261" {
		t.Fatal("original complete-call fixture changed")
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Version int
		Cases   []objectPrepareNative
	}
	if err := json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Version != 1 || len(fixture.Cases) != 320 {
		t.Fatal("complete original drawing preparation domain is missing")
	}
	return fixture.Cases
}

func TestObjectPrepareWorkOriginalCompleteCalls(t *testing.T) {
	steps, counts := 0, [81]int{}
	for _, row := range objectPrepareNatives(t) {
		cycle, refresh, index := row.Clock.Cycle, row.Clock.Refresh, 0
		after, err := PrepareObjectDrawWork(row.Before, func(op MainWorkOperation, live *ObjectDrawState) error {
			if index >= len(row.Steps) || *live != row.Steps[index].Before {
				t.Fatal("original drawing preparation publication differs", row.Name, index, op, *live)
			}
			if op.InternalRefresh || op.TailCycles != 0 {
				t.Fatal("drawing preparation invented an internal refresh", row.Name, index, op)
			}
			var err error
			cycle, refresh, err = op.Advance(cycle, refresh)
			if err != nil || cycle != row.Steps[index].Cycle || refresh != row.Steps[index].Refresh {
				t.Fatal("actual original work boundary differs", row.Name, index, op, cycle, refresh, err, row.Steps[index])
			}
			index++
			return nil
		})
		if err != nil || after != row.After || index != len(row.Steps) {
			t.Fatal("original whole drawing preparation differs", row.Name, after, row.After, index, len(row.Steps), err)
		}
		if plain, err := PrepareObjectDrawWork(row.Before, nil); err != nil || plain != after {
			t.Fatal("semantic-only preparation differs", row.Name, plain, err)
		}
		steps += index
		counts[row.Before.ReservedCount]++
	}
	for count := 1; count <= 80; count++ {
		if counts[count] != 4 {
			t.Fatal("missing H40 retained-count/refresh phase", count, counts[count])
		}
	}
	if steps != 117120 {
		t.Fatal("original work coverage changed", steps)
	}
	t.Logf("two-core complete original calls=320, work boundaries=%d; retained counts=1..80, refresh phases=4", steps)
}

func TestObjectPrepareWorkFailureAtEveryOperation(t *testing.T) {
	rows := objectPrepareNatives(t)
	sentinel := errors.New("discard complete drawing preparation")
	for _, row := range []objectPrepareNative{rows[0], rows[len(rows)-1]} {
		for failed := range row.Steps {
			calls := 0
			after, err := PrepareObjectDrawWork(row.Before, func(_ MainWorkOperation, live *ObjectDrawState) error {
				calls++
				if calls == failed+1 {
					live.ReservedCount = 2
					live.Count = 0xbeef
					return sentinel
				}
				return nil
			})
			if !errors.Is(err, sentinel) || after != row.Before || calls != failed+1 {
				t.Fatal("failed preparation leaked state or continued", row.Name, failed, after, calls, err)
			}
		}
	}
}

func TestObjectPrepareWorkLiveOwnerAndReadLatch(t *testing.T) {
	before := ObjectDrawState{ReservedCount: 2, Count: 71, WriteIndex: 8, FrameStartIndex: 4, Flags: 1}
	head, links, calls := 0, 0, 0
	after, err := PrepareObjectDrawWork(before, func(op MainWorkOperation, live *ObjectDrawState) error {
		calls++
		switch op.Phase {
		case "object-layer-head-reset":
			head++
			live.ReservedCount = 3
		case "object-retained-count-read":
			if head != 64 || live.ReservedCount != 3 || live.Count != before.Count || live.Flags != 0 {
				t.Fatal("drawing fields were reset before their actual publication", head, *live)
			}
			// The read was latched before this operation completed. A callback
			// change is kept in the owner but cannot rewrite its local loop bound.
			live.ReservedCount = 4
		case "object-retained-link-write":
			links++
			if live.Count != before.Count || live.WriteIndex != before.WriteIndex || live.FrameStartIndex != before.FrameStartIndex {
				t.Fatal("retained link work published a later cursor", *live)
			}
		case "object-count-publish":
			if live.Count != before.Count {
				t.Fatal("count was published before work completion", *live)
			}
		case "object-write-index-publish":
			if live.Count != 3 || live.WriteIndex != before.WriteIndex {
				t.Fatal("write cursor publication order differs", *live)
			}
		case "object-frame-start-publish":
			if live.WriteIndex != 3 || live.FrameStartIndex != before.FrameStartIndex {
				t.Fatal("frame start publication order differs", *live)
			}
		case "object-drawing-enable":
			if live.FrameStartIndex != 3 || live.Flags != 0 {
				t.Fatal("drawing enabled before both cursors were ready", *live)
			}
		}
		return nil
	})
	want := ObjectDrawState{ReservedCount: 4, Count: 3, WriteIndex: 3, FrameStartIndex: 3, Flags: 128}
	if err != nil || after != want || head != 64 || links != 3 || calls != 216 {
		t.Fatal("live owner or latched loop was replaced by an entry snapshot", after, want, head, links, calls, err)
	}
}

func TestObjectPrepareWorkInvalidDomain(t *testing.T) {
	for _, count := range []uint16{0, 81, 255, 65535} {
		before := ObjectDrawState{ReservedCount: count, Count: 17, WriteIndex: 18, FrameStartIndex: 19, Flags: 255}
		calls := 0
		after, err := PrepareObjectDrawWork(before, func(MainWorkOperation, *ObjectDrawState) error { calls++; return nil })
		if err == nil || after != before || calls != 0 {
			t.Fatal("invalid retained count performed work", count, after, calls, err)
		}
	}
	before := ObjectDrawState{ReservedCount: 2, Count: 17}
	links := 0
	after, err := PrepareObjectDrawWork(before, func(op MainWorkOperation, live *ObjectDrawState) error {
		if op.Phase == "object-layer-head-reset" {
			live.ReservedCount = 81
		}
		if op.Phase == "object-retained-link-write" {
			links++
		}
		return nil
	})
	if err == nil || after != before || links != 0 {
		t.Fatal("live unsupported count escaped validation", after, links, err)
	}
}

func TestObjectPrepareWorkIndependentCalls(t *testing.T) {
	for count := uint16(1); count <= 80; count++ {
		t.Run(strconv.Itoa(int(count)), func(t *testing.T) {
			t.Parallel()
			before := ObjectDrawState{ReservedCount: count, Count: 255, WriteIndex: 4, FrameStartIndex: 5, Flags: 1}
			operations := 0
			after, err := PrepareObjectDrawWork(before, func(MainWorkOperation, *ObjectDrawState) error { operations++; return nil })
			want := ObjectDrawState{ReservedCount: count, Count: count, WriteIndex: count, FrameStartIndex: count, Flags: 128}
			if err != nil || after != want || operations != 204+4*int(count) {
				t.Fatal("independent preparation shared state", after, operations, err)
			}
		})
	}
}
