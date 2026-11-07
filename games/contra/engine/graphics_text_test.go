package contra

// Failure cases before implementation: mistaking CB37 strings for CBC0
// rectangles; omitting the FF terminator from byte cost; losing a literal FF
// whose following byte is >=6; permitting an ambiguous FF followed by a new
// command byte 0..5; accepting a PPU address outside nametables/palette space.
import (
	"reflect"
	"testing"
)

func TestGraphicsOriginalTextPacket(t *testing.T) {
	// Japanese F62B, template7, invoked by C9C9 during the natural intro.
	p := GraphicsTextPacket{Kind: 1, Address: 0x20a1, Values: []uint8{0x1c, 0x0f, 0x1d, 0x1e, 0, 0, 0}}
	want := []uint8{1, 0x20, 0xa1, 0x1c, 0x0f, 0x1d, 0x1e, 0, 0, 0, 255}
	got, e := p.Encode()
	if e != nil || !reflect.DeepEqual(got, want) || p.EncodedSize() != len(want) {
		t.Fatalf("%x %v", got, e)
	}
	p.Kind = 2
	p.Values = []uint8{255, 6, 255}
	want = []uint8{2, 0x20, 0xa1, 255, 6, 255, 255}
	got, e = p.Encode()
	if e != nil || !reflect.DeepEqual(got, want) || p.EncodedSize() != len(want) {
		t.Fatalf("literal FF differs: %x %v", got, e)
	}
	got[3] = 0
	if p.Values[0] != 255 {
		t.Fatal("encoder aliases owned values")
	}
}
func TestGraphicsTextRejectsAmbiguousEncoding(t *testing.T) {
	for _, p := range []GraphicsTextPacket{{Kind: 0, Address: 0x2000, Values: []uint8{1}}, {Kind: 3, Address: 0x2000, Values: []uint8{1}}, {Kind: 1, Address: 0x4000, Values: []uint8{1}}, {Kind: 1, Address: 0x2000, Values: []uint8{255, 5}}} {
		if _, e := p.Encode(); e == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
}
