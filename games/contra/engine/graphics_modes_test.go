package contra

// Before adding the second native parser: text/rectangle opcode 1 is ambiguous
// without the original mode; mixed queues must fail; FF-aware encoded byte cost
// controls palette admission; reset must keep mode; transaction copies cannot
// alias either payload; malformed persisted mode must not partially submit.
import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestGraphicsTextQueueAndPresentation(t *testing.T) {
	p := WorldPresentation{Queue: GraphicsQueue{Mode: GraphicsText}}
	packet := GraphicsTextPacket{Kind: 1, Address: 0x2000, Values: make([]uint8, 66)}
	for i := range packet.Values {
		packet.Values[i] = 7
	}
	if err := p.Queue.AppendText(packet); err != nil {
		t.Fatal(err)
	}
	if p.Queue.Used != 70 {
		t.Fatal("text budget was normalized to rectangle cost")
	}
	packet.Values[0] = 99
	p.Palette.RequestedColors = 16
	if err := p.Seal(); err != nil {
		t.Fatal(err)
	}
	out, err := p.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if p.Palette.RequestedColors != 16 || p.Queue.Mode != GraphicsText || p.Queue.Used != 0 || len(p.Queue.TextPackets) != 0 {
		t.Fatal("mode/reset/palette gate differs")
	}
	if len(out.Writes) != 1 || out.Writes[0].Step != 1 || out.Writes[0].Address != 0x2000 || out.Writes[0].Values[0] != 7 {
		t.Fatal("text projection or ownership differs", out.Writes)
	}
	if err = p.Queue.AppendText(GraphicsTextPacket{Kind: 2, Address: 0x2100, Values: []uint8{8, 9}}); err != nil {
		t.Fatal(err)
	}
	if p.Queue.Writes()[0].Step != 32 {
		t.Fatal("vertical text lost stride")
	}
}

func TestGraphicsQueueModesRejectMixingAtomically(t *testing.T) {
	rect := GraphicsPacket{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: 0x2000, Values: []uint8{7}}}}
	text := GraphicsTextPacket{Kind: 1, Address: 0x2000, Values: []uint8{7}}
	for _, mode := range []GraphicsQueueMode{GraphicsRectangular, GraphicsText} {
		q := GraphicsQueue{Mode: mode}
		before, _ := json.Marshal(q)
		var err error
		if mode == GraphicsRectangular {
			err = q.AppendText(text)
		} else {
			err = q.Append(rect)
		}
		if err == nil {
			t.Fatal("mixed parser append accepted")
		}
		after, _ := json.Marshal(q)
		if string(before) != string(after) {
			t.Fatal("failed append changed queue")
		}
	}
	p := WorldPresentation{Queue: GraphicsQueue{Mode: 99}}
	before := p
	if _, err := p.Commit(); err == nil || !reflect.DeepEqual(p, before) {
		t.Fatal("unknown parser submitted or mutated state")
	}
}

func TestGraphicsQueueCloneOwnsBothPayloads(t *testing.T) {
	q := GraphicsQueue{Mode: GraphicsText, Used: 5, TextPackets: []GraphicsTextPacket{{Kind: 1, Address: 0x2000, Values: []uint8{7}}}}
	copy := q.Clone()
	copy.TextPackets[0].Values[0] = 8
	if q.TextPackets[0].Values[0] != 7 {
		t.Fatal("text clone aliases original")
	}
	q = GraphicsQueue{Packets: []GraphicsPacket{{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: 0x2000, Values: []uint8{7}}}}}}
	copy = q.Clone()
	copy.Packets[0].Segments[0].Values[0] = 8
	if q.Packets[0].Segments[0].Values[0] != 7 {
		t.Fatal("rectangle clone aliases original")
	}
}
