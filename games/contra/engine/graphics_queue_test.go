package contra

// Failure scenarios fixed before implementation: checking projected capacity
// instead of the original current-byte cursor; wrong row/column packet sizes;
// partial palette quadrant masks; 240-pixel wrap or horizontal page carry;
// palette-suppression bit lost; changing base supertiles/collision early;
// rejected requests mutating the queue; or caller-owned packet data aliasing.

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type graphicsOracleState struct {
	Used, Mode, ScrollX, ScrollY, Control, Level, SceneType uint8
	AttributeLookup                                         uint8
	Address                                                 uint16
	Queue, Supertiles, Collision                            string
}

func TestOriginalGraphicsCallerAttributeLookup(t *testing.T) {
	// Red-turret code consumes the request's attribute lookup byte as a later
	// target-array offset. Preserve this observed ABI effect with named data;
	// rejected requests must not overwrite the caller's previous selection.
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	rows := graphicsRecords(t, "testdata/original-graphics-requests-controlled-abi.json.gz")
	if len(rows) != 420 {
		t.Fatal("attribute ABI coverage changed", len(rows))
	}
	for _, r := range rows {
		data, _ := hex.DecodeString(r.Before.Supertiles)
		maps := ([128]uint8)(data)
		q := GraphicsQueue{Used: r.Before.Used}
		result, err := q.RequestSupertile(r.X, r.Y, r.Code, GraphicsContext{Scroll: CollisionScroll{X: r.Before.ScrollX, Y: r.Before.ScrollY, NameTable: r.Before.Control}, Supertiles: &maps, Tileset: tileset})
		if err != nil {
			t.Fatal(err)
		}
		callerValue := r.Before.AttributeLookup
		if result.AttributeLookupWritten {
			callerValue = result.AttributeLookup
		}
		if callerValue != r.After.AttributeLookup || result.AttributeLookupWritten != (r.Carry == 0) {
			t.Fatalf("original request %d changed lookup %+v before%d want%d", r.Index, result, r.Before.AttributeLookup, r.After.AttributeLookup)
		}
	}
}

type graphicsOracleRecord struct {
	Index, Frame      int
	X, Y, Code, Carry uint8
	Before, After     graphicsOracleState
}

func graphicsRecords(t *testing.T, path string) []graphicsOracleRecord {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Original bool `json:"source_is_original_execution"`
		Records  []graphicsOracleRecord
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if !oracle.Original {
		t.Fatal("fixture is not original execution")
	}
	return oracle.Records
}

func TestOriginalGraphicsRequests(t *testing.T) {
	tileset, err := JungleGraphicsTileset()
	if err != nil {
		t.Fatal(err)
	}
	accepted, rejected := 0, 0
	for _, scope := range []string{"natural", "controlled"} {
		rows := graphicsRecords(t, "testdata/original-graphics-requests-"+scope+".json.gz")
		wantCount := 3
		if scope == "controlled" {
			wantCount = 420
		}
		if len(rows) != wantCount {
			t.Fatal("oracle count changed", scope, len(rows))
		}
		for _, r := range rows {
			if r.Before.Level != 0 || r.Before.SceneType != 0 {
				t.Fatal("this fixture requires another tileset")
			}
			rawMap, err := hex.DecodeString(r.Before.Supertiles)
			if err != nil || len(rawMap) != 128 {
				t.Fatal(err)
			}
			maps := ([128]uint8)(rawMap)
			beforeMap := maps
			queue := GraphicsQueue{Used: r.Before.Used}
			result, err := queue.RequestSupertile(r.X, r.Y, r.Code, GraphicsContext{Scroll: CollisionScroll{X: r.Before.ScrollX, Y: r.Before.ScrollY, NameTable: r.Before.Control}, Supertiles: &maps, Tileset: tileset})
			if err != nil {
				t.Fatalf("%s request %d: %v", scope, r.Index, err)
			}
			if result.Accepted != (r.Carry == 0) || queue.Used != r.After.Used {
				t.Fatalf("%s request %d result %+v used%d want carry%d used%d", scope, r.Index, result, queue.Used, r.Carry, r.After.Used)
			}
			if maps != beforeMap || r.Before.Supertiles != r.After.Supertiles || r.Before.Collision != r.After.Collision || r.Before.Mode != r.After.Mode {
				t.Fatal("graphics request changed map/collision/mode")
			}
			beforeBytes, _ := hex.DecodeString(r.Before.Queue)
			afterBytes, _ := hex.DecodeString(r.After.Queue)
			var encoded []uint8
			for _, packet := range queue.Packets {
				data, err := packet.Encode()
				if err != nil {
					t.Fatal(err)
				}
				encoded = append(encoded, data...)
			}
			if result.Accepted {
				accepted++
				if result.Address != r.After.Address {
					t.Fatalf("%s request %d address %#x want %#x", scope, r.Index, result.Address, r.After.Address)
				}
				if !bytes.Equal(encoded, afterBytes[int(r.Before.Used):int(r.After.Used)]) {
					t.Fatalf("%s request %d encoded %x want %x", scope, r.Index, encoded, afterBytes[int(r.Before.Used):int(r.After.Used)])
				}
				copy(beforeBytes[int(r.Before.Used):], encoded)
			} else {
				rejected++
				if len(encoded) != 0 {
					t.Fatal("rejected request emitted packets")
				}
			}
			if !bytes.Equal(beforeBytes, afterBytes) {
				t.Fatalf("%s request %d modified bytes outside appended packets", scope, r.Index)
			}
		}
	}
	if accepted != 255 || rejected != 168 {
		t.Fatal("original capacity coverage changed", accepted, rejected)
	}
	t.Logf("%d accepted and %d rejected original requests match", accepted, rejected)
}

func TestGraphicsQueueMixedScrollAndEnemyBudget(t *testing.T) {
	var queue GraphicsQueue
	column := GraphicsPacket{Kind: 2, Length: 28, Segments: []GraphicsSegment{{Address: 0x2000, Values: make([]uint8, 28)}}}
	attributes := GraphicsPacket{Kind: 1, Length: 1}
	for i := 0; i < 7; i++ {
		attributes.Segments = append(attributes.Segments, GraphicsSegment{Address: uint16(0x23c0 + i*8), Values: []uint8{uint8(i)}})
	}
	if column.EncodedSize() != 33 || attributes.EncodedSize() != 24 {
		t.Fatal("native packet cost")
	}
	if err := queue.Append(column); err != nil {
		t.Fatal(err)
	}
	if err := queue.Append(attributes); err != nil {
		t.Fatal(err)
	}
	if queue.Used != 57 {
		t.Fatal(queue.Used)
	}
	tileset, _ := JungleGraphicsTileset()
	var maps [128]uint8
	result, err := queue.RequestSupertile(0, 16, 0, GraphicsContext{Scroll: CollisionScroll{Y: 224}, Supertiles: &maps, Tileset: tileset})
	if err != nil || !result.Accepted || queue.Used != 90 {
		t.Fatalf("original permits crossing64 from57: %+v %v used%d", result, err, queue.Used)
	}
	count := len(queue.Packets)
	result, err = queue.RequestSupertile(0, 16, 0, GraphicsContext{})
	if err != nil || result.Accepted || len(queue.Packets) != count || queue.Used != 90 {
		t.Fatal("full queue did not reject before tileset reads", result, err)
	}
	writes := queue.Writes()
	if writes[0].Step != 32 || writes[0].Address != 0x2000 || len(writes[0].Values) != 28 {
		t.Fatal("column packet projection", writes[0])
	}
	for i := 1; i <= 7; i++ {
		if writes[i].Step != 1 || writes[i].Address != uint16(0x23c0+(i-1)*8) || writes[i].Values[0] != uint8(i-1) {
			t.Fatal("attribute packet projection", writes[i])
		}
	}
}

func TestGraphicsQueueValidationAndOwnership(t *testing.T) {
	var q GraphicsQueue
	data := []uint8{1, 2}
	packet := GraphicsPacket{Kind: 1, Length: 2, Segments: []GraphicsSegment{{Address: 0x2000, Values: data}}}
	if err := q.Append(packet); err != nil {
		t.Fatal(err)
	}
	data[0] = 99
	if q.Packets[0].Segments[0].Values[0] != 1 {
		t.Fatal("queue aliases caller data")
	}
	writes := q.Writes()
	writes[0].Values[0] = 55
	if q.Packets[0].Segments[0].Values[0] != 1 {
		t.Fatal("writes alias queue data")
	}
	old := q
	for _, invalid := range []GraphicsPacket{{Kind: 0, Length: 1, Segments: packet.Segments}, {Kind: 1, Length: 0, Segments: packet.Segments}, {Kind: 2, Length: 1, Segments: packet.Segments}, {Kind: 1, Length: 1}} {
		if err := q.Append(invalid); err == nil {
			t.Fatal("invalid packet accepted", invalid)
		}
		if !reflect.DeepEqual(q, old) {
			t.Fatal("invalid append changed queue")
		}
	}
	q.Reset()
	if q.Used != 0 || len(q.Packets) != 0 {
		t.Fatal("reset retained pending data")
	}
}
