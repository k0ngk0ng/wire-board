package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalAuxiliaryGeometryCompleteTables(t *testing.T) {
	// Complete frame arrays for the restored action0..15 domains. Kind12's
	// additional riding actions and kind14/action3's dynamic RAM geometry
	// remain outside the static profile scope.
	var rows []struct {
		Kind, Action, Frame, Flags uint8
		Shape                      [8]uint8
	}
	raw, err := os.ReadFile("testdata/auxiliary-geometry-isolated.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 418 {
		t.Fatal("missing complete auxiliary source frames", len(rows))
	}
	for _, r := range rows {
		box := func(i int, attack bool) ContactBoxSpec {
			low := int16(r.Shape[i])
			if attack {
				low = int16(int8(r.Shape[i]))
			}
			return ContactBoxSpec{Enabled: r.Shape[i] != 128, MinX: int16(int8(r.Shape[i+3])), MaxX: int16(int8(r.Shape[i+2])), MinHeight: low, MaxHeight: int16(r.Shape[i+1])}
		}
		want := ContactProfile{Shape: ContactShape{Attack: box(0, true), Body: box(4, false)}, AttackFlags: r.Flags, ReceiveFlags: r.Flags}
		got, err := OriginalContactProfile(ContactProfileKey{Kind: r.Kind, Action: r.Action | 128, Variant: 255, Frame: r.Frame})
		if err != nil || got != want {
			t.Fatal(r, got, want, err)
		}
	}
	for _, key := range []ContactProfileKey{{Kind: 10, Action: 16}, {Kind: 10, Action: 0, Frame: 255}, {Kind: 15, Action: 0}} {
		if _, err := OriginalContactProfile(key); err == nil {
			t.Fatal("unrecovered table accepted", key)
		}
	}
}

func TestOriginalAuxiliaryDynamicGeometryRemainsExplicit(t *testing.T) {
	var rows []struct {
		Kind, Action, Frame   uint8
		ShapeAddress          uint16
		Shape                 [8]uint8
		UnsupportedDynamicRAM bool
	}
	raw, err := os.ReadFile("testdata/auxiliary-geometry-dynamic-isolated.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatal("missing original RAM-dependent shape cases", len(rows))
	}
	for i, r := range rows {
		if !r.UnsupportedDynamicRAM || r.ShapeAddress != [4]uint16{128, 0, 128, 0}[r.Frame] {
			t.Fatal("unexpected original pointer", r)
		}
		if _, err := OriginalContactProfile(ContactProfileKey{Kind: r.Kind, Action: r.Action, Frame: r.Frame}); err == nil {
			t.Fatal("dynamic original RAM shape silently treated as static", r)
		}
		if i >= 4 && r.Shape == rows[i-4].Shape {
			t.Fatal("RAM variation evidence lost", r)
		}
	}
}
