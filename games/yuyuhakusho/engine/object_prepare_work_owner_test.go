package engine

import (
	"errors"
	"testing"
)

func TestObjectPrepareOwnerOriginalCompleteCalls(t *testing.T) {
	steps := 0
	for _, row := range objectPrepareNatives(t) {
		owner := row.Before
		cycle, refresh, index := row.Clock.Cycle, row.Clock.Refresh, 0
		err := PrepareObjectDrawOnOwner(&owner, func(op MainWorkOperation, live *ObjectDrawState) error {
			if live != &owner {
				t.Fatal("work hook borrowed a copied drawing owner", row.Name, index)
			}
			if index >= len(row.Steps) || owner != row.Steps[index].Before {
				t.Fatal("original drawing owner publication differs", row.Name, index, op, owner)
			}
			var err error
			cycle, refresh, err = op.Advance(cycle, refresh)
			if err != nil || cycle != row.Steps[index].Cycle || refresh != row.Steps[index].Refresh {
				t.Fatal("original direct-owner work boundary differs", row.Name, index, op, cycle, refresh, err)
			}
			index++
			return nil
		})
		if err != nil || owner != row.After || index != len(row.Steps) {
			t.Fatal("original complete direct-owner preparation differs", row.Name, owner, row.After, index, err)
		}
		steps += index
	}
	if steps != 117120 {
		t.Fatal("direct-owner original coverage changed", steps)
	}
	t.Logf("same caller-owned pointer, complete original calls=320, work boundaries=%d", steps)
}

func TestObjectPrepareOwnerLivePublication(t *testing.T) {
	before := ObjectDrawState{ReservedCount: 2, Count: 71, WriteIndex: 8, FrameStartIndex: 4, Flags: 1}
	owner := before
	head, links, calls := 0, 0, 0
	err := PrepareObjectDrawOnOwner(&owner, func(op MainWorkOperation, live *ObjectDrawState) error {
		calls++
		if live != &owner {
			t.Fatal("drawing owner pointer was replaced", op)
		}
		switch op.Phase {
		case "object-drawing-disable":
			if owner.Flags != before.Flags {
				t.Fatal("drawing flag cleared before operation completion", owner)
			}
		case "object-layer-head-reset":
			head++
			if owner.Flags != 0 {
				t.Fatal("completed flag clear was hidden from the owner", owner)
			}
			owner.ReservedCount = 3
			if live.ReservedCount != 3 {
				t.Fatal("live hook cannot observe an outer owner update", *live)
			}
		case "object-retained-count-read":
			if head != 64 || owner.ReservedCount != 3 {
				t.Fatal("retained count read missed an earlier live write", head, owner)
			}
			owner.ReservedCount = 4
		case "object-retained-link-write":
			links++
			if owner.Count != before.Count || owner.WriteIndex != before.WriteIndex || owner.FrameStartIndex != before.FrameStartIndex {
				t.Fatal("a later field was published during retained link work", owner)
			}
		case "object-count-publish":
			if owner.Count != before.Count {
				t.Fatal("count published before current work completed", owner)
			}
			owner.Flags, owner.WriteIndex, owner.FrameStartIndex = 33, 11, 12
		case "object-write-index-publish":
			if owner.Count != 3 || owner.WriteIndex != 11 || owner.FrameStartIndex != 12 || owner.Flags != 33 {
				t.Fatal("count publication overwrote independent live fields", owner)
			}
			owner.Count = 9
		case "object-frame-start-publish":
			if owner.Count != 9 || owner.WriteIndex != 3 || owner.FrameStartIndex != 12 || owner.Flags != 33 {
				t.Fatal("write-index publication overwrote an earlier count update", owner)
			}
			owner.ReservedCount = 5
		case "object-drawing-enable":
			if owner.Count != 9 || owner.FrameStartIndex != 3 || owner.Flags != 33 || owner.ReservedCount != 5 {
				t.Fatal("frame-start publication overwrote independent fields", owner)
			}
		case "object-prepare-return":
			if owner.Flags != 128 {
				t.Fatal("completed enable was hidden from outer owner", owner)
			}
			owner.Count, owner.WriteIndex, owner.FrameStartIndex, owner.Flags = 11, 6, 7, 129
		}
		return nil
	})
	want := ObjectDrawState{ReservedCount: 5, Count: 11, WriteIndex: 6, FrameStartIndex: 7, Flags: 129}
	if err != nil || owner != want || head != 64 || links != 3 || calls != 216 {
		t.Fatal("direct owner updates or read latch were replaced", owner, want, head, links, calls, err)
	}
}

func TestObjectPrepareOwnerErrorKeepsCompletedChanges(t *testing.T) {
	rows := objectPrepareNatives(t)
	sentinel := errors.New("discard caller-owned drawing transaction")
	for _, row := range []objectPrepareNative{rows[0], rows[len(rows)-1]} {
		for failed := range row.Steps {
			owner, atFailure := row.Before, ObjectDrawState{}
			calls := 0
			err := PrepareObjectDrawOnOwner(&owner, func(_ MainWorkOperation, live *ObjectDrawState) error {
				if live != &owner {
					t.Fatal("failure hook borrowed copied owner", row.Name, failed)
				}
				calls++
				if calls == failed+1 {
					if owner != row.Steps[failed].Before {
						t.Fatal("partial owner did not reach actual failure boundary", row.Name, failed, owner)
					}
					owner.ReservedCount, owner.Count, owner.Flags = 2, 0xbeef, 77
					atFailure = owner
					return sentinel
				}
				return nil
			})
			if !errors.Is(err, sentinel) || calls != failed+1 || owner != atFailure {
				t.Fatal("direct-owner failure rolled back or executed more work", row.Name, failed, owner, atFailure, calls, err)
			}
		}
	}
}

func TestObjectPrepareOwnerInvalidDomain(t *testing.T) {
	calls := 0
	if err := PrepareObjectDrawOnOwner(nil, func(MainWorkOperation, *ObjectDrawState) error { calls++; return nil }); err == nil || calls != 0 {
		t.Fatal("missing owner performed work", calls, err)
	}
	for _, count := range []uint16{0, 81, 255, 65535} {
		before := ObjectDrawState{ReservedCount: count, Count: 17, WriteIndex: 18, FrameStartIndex: 19, Flags: 255}
		owner := before
		err := PrepareObjectDrawOnOwner(&owner, func(MainWorkOperation, *ObjectDrawState) error { calls++; return nil })
		if err == nil || owner != before || calls != 0 {
			t.Fatal("invalid owner performed work", count, owner, calls, err)
		}
	}
	before := ObjectDrawState{ReservedCount: 2, Count: 17, WriteIndex: 18, FrameStartIndex: 19, Flags: 255}
	owner, links := before, 0
	err := PrepareObjectDrawOnOwner(&owner, func(op MainWorkOperation, live *ObjectDrawState) error {
		if live != &owner {
			t.Fatal("validation hook borrowed copied owner", op)
		}
		if op.Phase == "object-layer-head-reset" {
			owner.ReservedCount = 81
		}
		if op.Phase == "object-retained-link-write" {
			links++
		}
		return nil
	})
	want := before
	want.ReservedCount, want.Flags = 81, 0
	if err == nil || owner != want || links != 0 {
		t.Fatal("live invalid owner lost completed work or reached links", owner, want, links, err)
	}
}
