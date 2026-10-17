package engine

import (
	"reflect"
	"testing"
)

func TestOriginalKind14PickupThrowContactChain(t *testing.T) {
	var rows []struct {
		Chain, Tick                int
		World                      WorldActionState
		Controllers                [4]ControllerFrame
		Kind, Action, Frame, Flags uint8
		Shape                      [8]uint8
	}
	readPropAttackFixture(t, "testdata/kind14-pickup-throw-contact.json.gz", &rows)
	if len(rows) != 22 {
		t.Fatal("missing original pickup/throw chain contacts", len(rows))
	}
	for _, r := range rows {
		positions := worldLoopPositions(r.World)
		ctx := ContactGeometryContext{PhysicalControllers: &r.Controllers, Positions: &positions}
		got, err := OriginalContactProfileWithContext(ContactProfileKey{Kind: r.Kind, Action: r.Action, Frame: r.Frame}, ctx)
		box := func(i int, attack bool) ContactBoxSpec {
			low := int16(r.Shape[i])
			if attack {
				low = int16(int8(r.Shape[i]))
			}
			return ContactBoxSpec{Enabled: r.Shape[i] != 128, MinX: int16(int8(r.Shape[i+3])), MaxX: int16(int8(r.Shape[i+2])), MinHeight: low, MaxHeight: int16(r.Shape[i+1])}
		}
		want := ContactProfile{Shape: ContactShape{Attack: box(0, true), Body: box(4, false)}, AttackFlags: r.Flags, ReceiveFlags: r.Flags}
		if err != nil || got != want {
			t.Fatal(r.Chain, r.Tick, r.Frame, got, want, err)
		}
		objects, err := preparationObjects(r.World, WorldPreparationContext{Kinds: [7]uint8{0, 0, 1, 1, 13, 14, 8}, PhysicalControllers: &r.Controllers})
		if err != nil || objects[5].Profile != want {
			t.Fatal("preparation failed to use phase context", r.Tick, objects[5], err)
		}
	}
}

func TestDynamicContactGeometryRequiresOnlyItsTrueSource(t *testing.T) {
	var positions [13]Position
	var controllers [4]ControllerFrame
	for frame := uint8(0); frame < 4; frame++ {
		key := ContactProfileKey{Kind: 14, Action: 3, Frame: frame}
		if _, err := OriginalContactProfileWithContext(key, ContactGeometryContext{}); err == nil {
			t.Fatal("unknown source defaulted to zero", frame)
		}
		ctx := ContactGeometryContext{Positions: &positions}
		if frame&1 != 0 {
			ctx = ContactGeometryContext{PhysicalControllers: &controllers}
		}
		if _, err := OriginalContactProfileWithContext(key, ctx); err != nil {
			t.Fatal("known source needs unrelated context", frame, err)
		}
	}
	if _, err := OriginalContactProfileWithContext(ContactProfileKey{Kind: 14, Action: 3, Frame: 4}, ContactGeometryContext{Positions: &positions, PhysicalControllers: &controllers}); err == nil {
		t.Fatal("unknown dynamic pointer admitted")
	}
}

func TestRuntimeContactPhysicalControllersSurviveSeatMapping(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.ControllerAssignments = [4]int8{2, 0, -1, 2}
	frames := [4]ControllerFrame{{Held: ButtonA, Pressed: ButtonA}, {Held: ButtonLeft}, {Held: ButtonB, Pressed: ButtonB}, {Held: ButtonRight | ButtonB}}
	got := runtimeWorldInput(c, frames)
	if got.PhysicalControllers == nil || *got.PhysicalControllers != frames {
		t.Fatal("unassigned or reordered physical ports lost", got)
	}
	if got.Frames != ([4]ControllerFrame{frames[2], frames[0], {}, frames[2]}) {
		t.Fatal("seat mapping changed", got.Frames)
	}
	frames[3] = ControllerFrame{}
	if got.PhysicalControllers[3].Held != ButtonRight|ButtonB {
		t.Fatal("caller mutated the sample")
	}
}

func TestRecordedVideoRetainsUnassignedPhysicalControllerSamples(t *testing.T) {
	c := DefaultRuntimeConfig()
	c.Match.Seed = 0x12345678
	s, err := NewRuntime(c)
	if err != nil {
		t.Fatal(err)
	}
	for tick := 0; tick < 180; tick++ {
		held := [4]Buttons{}
		if tick%6 < 3 {
			held[3] = ButtonB | ButtonRight
		}
		record := RecordedVideoInput{Frame: s.VideoFrames + 1, Held: held, PriorHeld: s.Held, PendingPressed: s.PendingPressed, Automated: [4]bool{false, true, true, true}}
		direct, err := StepVideo(s, held)
		if err != nil {
			t.Fatal(err)
		}
		replayed, err := StepRecordedVideo(s, record)
		if err != nil || !reflect.DeepEqual(direct, replayed) {
			t.Fatal("recorded physical input changed replay", tick, err)
		}
		if replayed.State.Held[3] != held[3] {
			t.Fatal("unassigned controller sample discarded", tick)
		}
		s = direct.State
	}
}
