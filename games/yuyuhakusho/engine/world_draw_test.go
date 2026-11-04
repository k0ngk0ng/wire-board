package engine

import (
	"reflect"
	"testing"
)

func TestWorldDrawFieldsAndValueOwnership(t *testing.T) {
	object := WorldObject{ObjectID: 0x8400, GraphicsSet: 0x1a234, TilesBase: 0x200000,
		ScreenX: -320, ScreenY: 2047, GraphicOffsetX: -7, GraphicOffsetY: 13,
		ScreenOffsetX: 99, ScreenOffsetY: -99, GraphicParameter: 0x32, SkipGraphics: 1,
		GraphicCache: [4]uint32{1, 2, 3, 4}, Body: FighterBodyState{Attributes: 0xa123, Depth: 384, Layer: 0xf4}}
	want := WorldDraw{ObjectID: 0x8400, GraphicsSet: 0x1a234, TileBank: 0x200000,
		ScreenX: uint16(0xfec0), ScreenY: 2047, OffsetX: -7, OffsetY: 13,
		Parameter: 0x32, SkipTiles: 1, Cache: [4]uint32{1, 2, 3, 4}, Attributes: 0xa123, Depth: 384, Layer: 0xf4}
	got := object.DrawRequest()
	if got != want {
		t.Fatalf("draw uses wrong source fields: got %+v want %+v", got, want)
	}
	object.GraphicCache[0], object.ScreenX = 0, 100
	if got != want {
		t.Fatal("draw request retained mutable object storage")
	}
}

func TestWorldDrawCaptureOnlyActualRequests(t *testing.T) {
	state := ChuWorldState{Objects: WorldObjects{0x8300: {ObjectID: 0x8300, Header: MatchObjectHeader{Behavior: 0xfe78, Size: 64}}}}
	for _, paused := range []bool{false, true} {
		state.Clock.PauseFlags = 0
		if paused {
			state.Clock.PauseFlags = 0x80
		}
		next, events, err := StepChuWorldObjects(state, ChuWorldView{}, ChuWorldLibrary{})
		if err != nil {
			t.Fatal(err)
		}
		var draws []*WorldDraw
		for _, event := range events {
			if event.Kind == "draw" {
				if event.Draw == nil || event.Draw.ObjectID != event.ObjectID {
					t.Fatal("actual draw lost callback-time payload")
				}
				draws = append(draws, event.Draw)
			} else if event.Draw != nil {
				t.Fatal("non-draw event has a draw payload")
			}
		}
		if paused != (len(draws) == 1) {
			t.Fatalf("unrequested object draw: paused=%v draws=%d", paused, len(draws))
		}
		if paused {
			want := next.Objects[0x8300].DrawRequest()
			if !reflect.DeepEqual(*draws[0], want) {
				t.Fatal("draw payload does not follow completed callback")
			}
			next.Objects[0x8300] = WorldObject{ObjectID: 0x8300}
			if *draws[0] != want {
				t.Fatal("later clear/reuse changed a prior draw payload")
			}
		}
	}
}
