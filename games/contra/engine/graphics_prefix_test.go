package contra

// Before Guardian's finite queue-head effect: clearing first Kind stops drawing
// without changing allocation/palette budgets; cloned masks are independent;
// shortened native rows use their modified header; reads beyond owned graphics
// cannot be invented from adjacent CPU storage; reset removes old head masks.
import (
	"encoding/json"
	"testing"
)

func TestGraphicsPrefixTerminationPreservesBudget(t *testing.T) {
	var q GraphicsQueue
	if e := q.Append(GraphicsPacket{Kind: 1, Length: 65, Segments: []GraphicsSegment{{Address: 0x2000, Values: make([]uint8, 65)}}}); e != nil {
		t.Fatal(e)
	}
	before, _ := q.Encode()
	copy := q.Clone()
	if e := copy.ClearPrefix(0, 0xfc); e != nil {
		t.Fatal(e)
	}
	after, e := copy.Encode()
	if e != nil {
		t.Fatal(e)
	}
	if copy.Used != 70 || len(after) != len(before) || after[0] != 0 || len(copy.Writes()) != 0 {
		t.Fatal("native stop changed budget or still drew")
	}
	original, _ := q.Encode()
	if original[0] != 1 {
		t.Fatal("head effect leaked into source clone")
	}
	p := WorldPresentation{Queue: copy, Palette: PaletteOutput{RequestedColors: 16}}
	if e = p.Seal(); e != nil {
		t.Fatal(e)
	}
	if _, e = p.Commit(); e != nil {
		t.Fatal(e)
	}
	if p.Palette.RequestedColors != 16 {
		t.Fatal("head stop incorrectly bypassed palette gate")
	}
	copy.Reset()
	if e = copy.Append(GraphicsPacket{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: 0x2000, Values: []uint8{7}}}}); e != nil {
		t.Fatal(e)
	}
	after, _ = copy.Encode()
	if after[0] != 1 {
		t.Fatal("reset retained stale head effect")
	}
}

func TestGraphicsPrefixDecodeAndFailure(t *testing.T) {
	var q GraphicsQueue
	if e := q.Append(GraphicsPacket{Kind: 1, Length: 3, Segments: []GraphicsSegment{{Address: 0x2000, Values: []uint8{7, 0, 0}}}}); e != nil {
		t.Fatal(e)
	}
	if e := q.ClearPrefix(1, 0xfd); e != nil {
		t.Fatal(e)
	}
	writes, e := q.DecodedWrites()
	if e != nil || len(writes) != 1 || len(writes[0].Values) != 1 || writes[0].Values[0] != 7 {
		t.Fatal("modified native length ignored", writes, e)
	}
	before, _ := json.Marshal(q)
	if e = q.ClearPrefix(5, 0); e == nil {
		t.Fatal("unbounded prefix write accepted")
	}
	after, _ := json.Marshal(q)
	if string(before) != string(after) {
		t.Fatal("invalid mask changed queue")
	}
	if e = q.ClearPrefix(2, 0xfe); e != nil {
		t.Fatal(e)
	}
	p := WorldPresentation{Queue: q}
	before, _ = json.Marshal(p)
	if _, e = p.Commit(); e == nil {
		t.Fatal("graphics reader invented an out-of-buffer count")
	}
	after, _ = json.Marshal(p)
	if string(before) != string(after) {
		t.Fatal("invalid graphics partially submitted")
	}
}
