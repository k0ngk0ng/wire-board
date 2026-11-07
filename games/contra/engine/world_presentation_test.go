package contra

// Failures considered before implementation: clearing the graphics byte count
// before the palette gate; showing next-tick sprites early; output slices alias
// mutable queues; invalid packet leaves partially submitted state; restored
// presentation differs from uninterrupted presentation; deferred colors lost.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWorldPresentationTerminatorCountsForPaletteGate(t *testing.T) {
	var p WorldPresentation
	p.Palette.RequestedColors = 16
	if err := p.Queue.Append(GraphicsPacket{Kind: 1, Length: 65, Segments: []GraphicsSegment{{Address: 0x2000, Values: make([]uint8, 65)}}}); err != nil {
		t.Fatal(err)
	}
	if err := p.Seal(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Commit(); err != nil {
		t.Fatal(err)
	}
	if p.Palette.RequestedColors != 16 || p.Terminated {
		t.Fatal("terminator lost from palette admission or next queue")
	}
}

func TestWorldPresentationBoundaryAndRestore(t *testing.T) {
	var p WorldPresentation
	p.Sprites.OAM[0] = 42
	p.Patterns.Desired[5] = 69
	p.Palette.RequestedColors = 32
	for i := range p.Palette.Desired {
		p.Palette.Desired[i] = uint8(i)
	}
	packet := GraphicsPacket{Kind: 1, Length: 66, Segments: []GraphicsSegment{{Address: 0x2000, Values: make([]uint8, 66)}}}
	packet.Segments[0].Values[0] = 17
	if err := p.Queue.Append(packet); err != nil {
		t.Fatal(err)
	}
	// 71 bytes is exactly the original palette deferral boundary.
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var restored WorldPresentation
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	first, err := p.Commit()
	if err != nil {
		t.Fatal(err)
	}
	other, err := restored.Commit()
	if err != nil || !reflect.DeepEqual(first, other) || !reflect.DeepEqual(p, restored) {
		t.Fatal("restore changed boundary", err)
	}
	if first.OAM[0] != 42 || first.Patterns[5] != 69 || first.Palette[1] != 0 || p.Palette.RequestedColors != 32 || p.Queue.Used != 0 {
		t.Fatal("wrong deferred submission", first, p)
	}
	p.Sprites.OAM[0] = 55
	p.Patterns.Desired[5] = 32
	if first.OAM[0] != 42 || first.Patterns[5] != 69 {
		t.Fatal("next frame leaked into submitted frame")
	}
	second, err := p.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if second.OAM[0] != 55 || second.Patterns[5] != 32 || second.Palette[0] != 16 || second.Palette[1] != 1 || p.Palette.RequestedColors != 0 || len(second.Writes) != 0 {
		t.Fatal("deferred palette was lost", second)
	}
	if first.Writes[0].Values[0] != 17 {
		t.Fatal("submitted graphics mutated")
	}
}

func TestWorldPresentationFailureAtomic(t *testing.T) {
	for _, broken := range []WorldPresentation{
		{Palette: PaletteOutput{RequestedColors: 7}},
		{Queue: GraphicsQueue{Used: 1}},
		{Queue: GraphicsQueue{Used: 6, Packets: []GraphicsPacket{{Kind: 3, Length: 1, Segments: []GraphicsSegment{{Address: 0x2000, Values: []uint8{1}}}}}}},
	} {
		before, _ := json.Marshal(broken)
		if _, err := broken.Commit(); err == nil {
			t.Fatal("invalid presentation accepted")
		}
		after, _ := json.Marshal(broken)
		if string(after) != string(before) {
			t.Fatal("failed boundary changed state")
		}
	}
}
