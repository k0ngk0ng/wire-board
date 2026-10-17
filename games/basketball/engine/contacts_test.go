package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type observedContactObject struct {
	Action, Kind, Variant, Cursor, Pose, Disabled, Owner, AttackFlags, ReceiveFlags uint8
	Shape                                                                           [8]uint8
	X, Depth, Height                                                                int32
}

func observedContactProfile(o observedContactObject) ContactProfile {
	return ContactProfile{AttackFlags: o.AttackFlags, ReceiveFlags: o.ReceiveFlags, Shape: ContactShape{
		Attack: ContactBoxSpec{Enabled: o.Shape[0] != 128, MinHeight: int16(int8(o.Shape[0])), MaxHeight: int16(o.Shape[1]), MinX: int16(int8(o.Shape[3])), MaxX: int16(int8(o.Shape[2]))},
		Body:   ContactBoxSpec{Enabled: o.Shape[4] != 128, MinHeight: int16(o.Shape[4]), MaxHeight: int16(o.Shape[5]), MinX: int16(int8(o.Shape[7])), MaxX: int16(int8(o.Shape[6]))},
	}}
}

func contactObject(t *testing.T, o observedContactObject) ContactObject {
	t.Helper()
	profile, err := OriginalContactProfile(ContactProfileKey{Kind: o.Kind, Action: o.Action & 127, Variant: o.Variant, Frame: o.Cursor})
	if err != nil || profile != observedContactProfile(o) {
		t.Fatal("contact profile differs", o, profile, err)
	}
	return ContactObject{Action: o.Action & 127, Entered: o.Action&128 != 0, Mirrored: o.Pose&128 != 0, Suppressed: o.Disabled != 0, Owner: int8(o.Owner), Position: Position{X: Fixed(o.X << 8 >> 8), Depth: Fixed(o.Depth), Height: Fixed(o.Height << 8 >> 8)}, Profile: profile}
}

func TestOriginalContactCandidates(t *testing.T) {
	data, err := os.ReadFile("testdata/contacts.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Scenario string
		Frame    int
		Before   struct {
			Objects  [7]observedContactObject
			Rotation uint8
		}
		After struct {
			Candidates [7]uint8
			Rotation   uint8
		}
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2392 {
		t.Fatal("unexpected contact count", len(rows))
	}
	for _, r := range rows {
		var objects [7]ContactObject
		for i, o := range r.Before.Objects {
			objects[i] = contactObject(t, o)
		}
		got := FindObjectContacts(objects, r.Before.Rotation)
		var want [7]int8
		for i, v := range r.After.Candidates {
			want[i] = -1
			if v < 128 {
				want[i] = int8(v)
			}
		}
		if got.Candidates != want || got.Rotation != r.After.Rotation {
			t.Fatalf("%s frame %d: got %+v want %v rotation %d", r.Scenario, r.Frame, got, want, r.After.Rotation)
		}
	}
	t.Log("matched 2392 original contact-search phases with original frame geometry")
}

func TestContactRotationAndActionEntry(t *testing.T) {
	var objects [7]ContactObject
	for i := 0; i < 2; i++ {
		objects[i] = ContactObject{Entered: true, Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}, Profile: ContactProfile{ReceiveFlags: 16, Shape: ContactShape{Body: ContactBoxSpec{Enabled: true, MinX: -8, MaxX: 8, MaxHeight: 32}}}}
	}
	objects[6] = ContactObject{Action: 3, Entered: true, Owner: 6, Position: Position{X: 100 * Pixel, Depth: 40 * Pixel, Height: 8 * Pixel}, Profile: ContactProfile{AttackFlags: 32, Shape: ContactShape{Attack: ContactBoxSpec{Enabled: true, MinX: -8, MaxX: 8, MaxHeight: 16}}}}
	for priority := uint8(0); priority < 2; priority++ {
		got := FindObjectContacts(objects, priority)
		if got.Candidates[6] != int8(priority) || got.Rotation != priority+1 {
			t.Fatal(got)
		}
	}
	objects[0].Entered = false
	if got := FindObjectContacts(objects, 0); got.Candidates[6] != 1 {
		t.Fatal("unentered actor received contact", got)
	}
}

func TestContactMirroringAndExactEdges(t *testing.T) {
	var objects [7]ContactObject
	objects[0] = ContactObject{Action: 96, Entered: true, Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}, Profile: ContactProfile{AttackFlags: 32, Shape: ContactShape{Attack: ContactBoxSpec{Enabled: true, MinX: 0, MaxX: 16, MaxHeight: 32}}}}
	objects[2] = ContactObject{Entered: true, Position: Position{X: 120 * Pixel, Depth: 33 * Pixel}, Profile: ContactProfile{ReceiveFlags: 8, Shape: ContactShape{Body: ContactBoxSpec{Enabled: true, MinX: -4, MaxX: 4, MaxHeight: 32}}}}
	if got := FindObjectContacts(objects, 0); got.Candidates[0] != 2 {
		t.Fatal("inclusive X/depth lower edge missed", got)
	}
	objects[2].Position.Depth = 48 * Pixel
	if got := FindObjectContacts(objects, 0); got.Candidates[0] != -1 {
		t.Fatal("exclusive depth upper edge accepted", got)
	}
	objects[2].Position.Depth = 40 * Pixel
	objects[0].Mirrored = true
	if got := FindObjectContacts(objects, 0); got.Candidates[0] != -1 {
		t.Fatal("mirrored strike kept right-facing box", got)
	}
	objects[2].Position.X = 80 * Pixel
	if got := FindObjectContacts(objects, 0); got.Candidates[0] != 2 {
		t.Fatal("mirrored left edge missed", got)
	}
}

func TestNegativeStrikeHeightRetainsTraversal(t *testing.T) {
	var objects [7]ContactObject
	objects[6] = ContactObject{Action: 3, Entered: true, Owner: 6, Position: Position{X: 100 * Pixel, Depth: 40 * Pixel}, Profile: ContactProfile{AttackFlags: 32, Shape: ContactShape{Attack: ContactBoxSpec{Enabled: true, MinHeight: -1, MaxHeight: 8, MinX: -8, MaxX: 8}}}}
	got := FindObjectContacts(objects, 7)
	if got.Candidates[6] != -1 || got.Rotation != 3 {
		t.Fatal("negative-height overlap or incorrect traversal", got)
	}
}
