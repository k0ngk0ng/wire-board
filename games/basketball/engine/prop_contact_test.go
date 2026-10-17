package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOriginalPlayerPropContacts(t *testing.T) {
	var rows []struct {
		Name           string
		Source, Target int8
		Before, After  CombatState
		Sounds         []uint8
	}
	raw, err := os.ReadFile("testdata/prop-contact-isolated.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 288 {
		t.Fatal("missing original-code isolated cases", len(rows))
	}
	for _, row := range rows {
		got, err := StepOriginalCombat(row.Before, oneCombatCandidate(row.Source, row.Target))
		if err != nil || got.State != row.After || len(got.Sounds) != 0 || len(row.Sounds) != 0 {
			t.Fatalf("%s: before=%+v\ngot=%+v\nwant=%+v\nerror=%v", row.Name, row.Before, got, row.After, err)
		}
	}
}

func TestOriginalPlayerPropInvalidHolderIsAtomic(t *testing.T) {
	s := CombatState{SpecialHolder: -1}
	s.Auxiliary[0].Action = 6
	got, err := StepOriginalCombat(s, oneCombatCandidate(0, 4))
	if err == nil || got.State != s || len(got.Sounds) != 0 {
		t.Fatal(got, err)
	}
}

func TestOriginalPropSlide(t *testing.T) {
	var rows []struct {
		Case, Tick    int
		Actor         uint8
		Before, After AuxiliaryState
	}
	raw, err := os.ReadFile("testdata/prop-slide-isolated.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 844 {
		t.Fatal("missing original slide calls", len(rows))
	}
	for _, row := range rows {
		got, err := StepAuxiliaryRest(row.Before, AuxiliaryContext{Object: row.Actor, Court: 1})
		if err != nil || got.State != row.After || got.ChangeLinkedPose {
			t.Fatalf("case %d tick %d: before=%+v\ngot=%+v\nwant=%+v\nerror=%v", row.Case, row.Tick, row.Before, got, row.After, err)
		}
	}
}

func TestOriginalPropReactionGeometry(t *testing.T) {
	// Original bank5: kind maps $8F61/$8F71, flags $93A3;
	// groups 6/7 resolve to raw boxes $9161/$9169 for both animation frames.
	for _, kind := range []uint8{11, 12} {
		box := ContactBoxSpec{Enabled: true, MinX: -8, MaxX: 8, MinHeight: 1, MaxHeight: 39}
		if kind == 12 {
			box.MinX, box.MaxX, box.MaxHeight = -10, 10, 6
		}
		for _, action := range []uint8{12, 13} {
			flags := uint8(0)
			if action == 13 {
				flags = 0x28
			}
			for frame := uint8(0); frame < 2; frame++ {
				got, err := OriginalContactProfile(ContactProfileKey{Kind: kind, Action: action, Frame: frame})
				want := ContactProfile{Shape: ContactShape{Attack: box, Body: box}, AttackFlags: flags, ReceiveFlags: flags}
				if err != nil || got != want {
					t.Fatal(kind, action, frame, got, want, err)
				}
			}
		}
	}
}
