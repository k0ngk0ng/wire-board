package contra

// Failure cases before tightening the queue domain: a legal exactly-full
// graphics buffer is rejected; one more byte aliases palette state; a rejected
// append changes existing packets or the byte cursor; admission is mistakenly
// changed from the independent original 64-byte request threshold.

import (
	"reflect"
	"testing"
)

func TestGraphicsQueuePhysicalCapacity(t *testing.T) {
	q := GraphicsQueue{Used: 183}
	packet := GraphicsPacket{Kind: 1, Length: 4, Segments: []GraphicsSegment{{Address: 0x23c2, Values: []uint8{80, 83, 92, 95}}}}
	if err := q.Append(packet); err != nil || q.Used != 192 {
		t.Fatal("183+9 must fit exactly", q.Used, err)
	}
	before := GraphicsQueue{Used: q.Used, Packets: append([]GraphicsPacket(nil), q.Packets...)}
	if err := q.Append(GraphicsPacket{Kind: 1, Length: 1, Segments: []GraphicsSegment{{Address: 0x2000, Values: []uint8{1}}}}); err == nil {
		t.Fatal("append crossed the palette boundary")
	}
	if !reflect.DeepEqual(q, before) {
		t.Fatal("capacity failure changed queue")
	}
	other := GraphicsQueue{Used: 184}
	if err := other.Append(packet); err == nil || other.Used != 184 || len(other.Packets) != 0 {
		t.Fatal("184+9 must not enter palette storage", other, err)
	}
}
