package engine

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Full CE09/F002 failure inventory precedes implementation in
// POSTRESULT_SURFACE_VERIFICATION.md. Captured and controlled cases are labeled.
func TestOriginalPostResultOAMFullSource(t *testing.T) {
	var cases []struct {
		Label                 string
		Before, OAM, Reversed []byte
		Pieces                uint8
	}
	readPropAttackFixture(t, "testdata/postresult-oam.json.gz", &cases)
	if len(cases) != 3533 {
		t.Fatalf("source OAM cases%d", len(cases))
	}
	frames, full := 0, 0
	for _, c := range cases {
		state := openingRAM(c.Before)
		var previous [256]uint8
		copy(previous[:], c.Before[0x200:0x300])
		var context OriginalPostResultSpriteContext
		copy(context.SurfaceCodes[:], c.Before[0x5ea:0x5f7])
		before := state
		got, err := EmitOriginalPostResultOAM(previous, state, context)
		if err != nil {
			t.Fatalf("%s: %v", c.Label, err)
		}
		if !reflect.DeepEqual(got.OAM[:], c.OAM) || got.Pieces != c.Pieces {
			t.Fatalf("%s pieces%d/%d OAM%v/%v", c.Label, got.Pieces, c.Pieces, got.OAM, c.OAM)
		}
		if state != before {
			t.Fatal("OAM emission mutated actors")
		}
		reversed := ReverseOriginalPostResultOAM(got.OAM)
		if !reflect.DeepEqual(reversed[:], c.Reversed) {
			t.Fatal("F002", c.Label)
		}
		input, err := json.Marshal(struct {
			Previous [256]uint8
			Actors   OpeningState
			Context  OriginalPostResultSpriteContext
		}{previous, state, context})
		if err != nil {
			t.Fatal(err)
		}
		var restored struct {
			Previous [256]uint8
			Actors   OpeningState
			Context  OriginalPostResultSpriteContext
		}
		if err = json.Unmarshal(input, &restored); err != nil {
			t.Fatal(err)
		}
		again, err := EmitOriginalPostResultOAM(restored.Previous, restored.Actors, restored.Context)
		if err != nil || again != got {
			t.Fatalf("JSON restore %s %v", c.Label, err)
		}
		if len(c.Label) > 9 && c.Label[:9] == "original-" {
			frames++
		}
		if got.Pieces == 64 {
			full++
		}
	}
	if frames != 16 || full != 1 {
		t.Fatalf("source scope frames%d capacity%d", frames, full)
	}
}

func TestOriginalPostResultOAMInvalidIsAtomic(t *testing.T) {
	var initial [256]uint8
	for i := range initial {
		initial[i] = uint8(i)
	}
	base := OpeningState{}
	base.Transition.Scene = 7
	base.Transition.Match.Phase = 1
	base.Transition.Kinds[0] = 17
	for i := range base.Projection.DrawOrder {
		base.Projection.DrawOrder[i] = 255
	}
	base.Projection.DrawOrder[0] = 0
	invalid := []OpeningState{base, base, base, base}
	invalid[0].Transition.Scene = 0
	invalid[1].Projection.DrawOrder[0] = 13
	invalid[2].Transition.Kinds[0] = 15
	invalid[3].Transition.Objects[0].Pose.Frame = 127
	for i, s := range invalid {
		got, err := EmitOriginalPostResultOAM(initial, s, OriginalPostResultSpriteContext{})
		if err == nil || got.OAM != initial {
			t.Fatalf("invalid input%d changed priorOAM or accepted", i)
		}
	}
}
