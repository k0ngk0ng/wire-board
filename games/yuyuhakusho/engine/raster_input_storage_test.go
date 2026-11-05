package engine

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

// Ownership and alias failure cases were frozen in
// RASTER_INPUT_STORAGE_FAILURES.md before this test or production changes.
func rasterJSONRowsTest(t *testing.T, state ChuWorldState) [2][224]int16 {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct{ PanA, PanB [224]int16 }
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	return [2][224]int16{wire.PanA, wire.PanB}
}

func rasterWorldTest() (ChuWorldState, ChuWorldLibrary) {
	s := ChuWorldState{
		Objects: WorldObjects{0x8200: {ObjectID: 0x8200, Header: MatchObjectHeader{Behavior: 0x20918, Size: 128, Stage: 2}}},
		Stream:  make([]byte, 1024), Camera: DefaultArenaCamera{X: 744}, Clock: WorldClockState{PanOffset: 0xfff9},
	}
	for i := range s.Stream {
		s.Stream[i] = byte(i*29 + 17)
	}
	for i := range s.Cells {
		s.Cells[i] = uint16(i - 500)
	}
	library := ChuWorldLibrary{Arena: ArenaRuntimeLibrary{WaveX: make([]int8, 1024), Tails: map[int16][]int16{744: make([]int16, 56)}}}
	return s, library
}

func TestRasterInputStorageSingleOwner(t *testing.T) {
	typ := reflect.TypeFor[ChuWorldState]()
	for _, name := range []string{"PanA", "PanB"} {
		if _, exists := typ.FieldByName(name); exists {
			t.Errorf("world still owns mutable %s independently of Stream", name)
		}
	}
	s, _ := rasterWorldTest()
	view, ok := any(s).(interface{ PanRows(int) [224]int16 })
	if !ok {
		t.Fatal("world must expose its canonical bytes through a copied PanRows view")
	}
	for plane := 0; plane < 2; plane++ {
		rows := view.PanRows(plane)
		for i, row := range rows {
			if row != int16(binary.BigEndian.Uint16(s.Stream[plane*512+i*2:])) {
				t.Fatal("raster view uses a different owner, endian order or row offset", plane, i)
			}
		}
		before := append([]byte(nil), s.Stream...)
		rows[0]++
		if !bytes.Equal(s.Stream, before) {
			t.Fatal("mutating returned raster view changed canonical bytes")
		}
	}
	if view.PanRows(-1) != [224]int16{} || view.PanRows(2) != [224]int16{} {
		t.Fatal("unavailable raster planes must not alias another plane")
	}
}

func TestRasterInputStorageRecordingChangesRows(t *testing.T) {
	for _, offset := range []int{0, 444, 512, 956} {
		s, _ := rasterWorldTest()
		before := append([]byte(nil), s.Stream...)
		recorded, err := RecordFighterInputEnvelope(FighterInputEnvelopeState{
			Mode: -1, Position: 0xff6000 + uint32(offset), StreamBase: 0xff6000, Stream: s.Stream,
			Controls: FighterControls{Held: 0x81, Pressed: 0x23, Changed: 0x45, RawHeld: 0x67},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(s.Stream, before) {
			t.Fatal("recording changed caller storage")
		}
		s.Stream = recorded.Stream // The existing actor-to-world ownership merge.
		rows := rasterJSONRowsTest(t, s)
		plane, row := offset/512, (offset%512)/2
		if rows[plane][row] != int16(-32477) || rows[plane][row+1] != 0x4567 {
			t.Fatalf("recorded bytes are not immediately visible as raster words at offset %d: %x %x", offset, uint16(rows[plane][row]), uint16(rows[plane][row+1]))
		}
	}
}

func TestRasterInputStorageArenaChangesPlayback(t *testing.T) {
	s, library := rasterWorldTest()
	before := append([]byte(nil), s.Stream...)
	next, _, err := StepChuWorldObjects(s, ChuWorldView{StreamBase: 0xff6000}, library)
	if err != nil {
		t.Fatal(err)
	}
	rows := rasterJSONRowsTest(t, next)
	if rows[0][0] != -251 || rows[0][223] != -13 || rows[1][0] != 5 || rows[1][223] != -7 {
		t.Fatal("fixture lost signed arena feedback boundaries", rows[0][0], rows[0][223], rows[1][0], rows[1][223])
	}
	for plane := 0; plane < 2; plane++ {
		for i, row := range rows[plane] {
			if int16(binary.BigEndian.Uint16(next.Stream[plane*512+i*2:])) != row {
				t.Fatalf("arena raster word did not change canonical stream plane=%d row=%d", plane, i)
			}
		}
		if !bytes.Equal(next.Stream[plane*512+448:plane*512+512], before[plane*512+448:plane*512+512]) {
			t.Fatal("arena overwrote the 64-byte non-raster gap", plane)
		}
		for _, row := range []int{0, 222} {
			offset := plane*512 + row*2
			played, route, err := PrepareFighterInputEnvelope(FighterInputEnvelopeState{
				Mode: 1, Position: 0xff6000 + uint32(offset), StreamBase: 0xff6000, Stream: next.Stream,
			})
			want := [4]byte{byte(uint16(rows[plane][row]) >> 8), byte(rows[plane][row]), byte(uint16(rows[plane][row+1]) >> 8), byte(rows[plane][row+1])}
			got := [4]byte{byte(played.Controls.Held), byte(played.Controls.Pressed), byte(played.Controls.Changed), played.Controls.RawHeld}
			if err != nil || route != FighterInputComplete || got != want {
				t.Fatal("playback did not observe current raster bytes", plane, row, got, want, err)
			}
		}
	}
	if !bytes.Equal(s.Stream, before) {
		t.Fatal("arena changed caller storage")
	}
}

func TestRasterInputStorageArenaGatesAndAtomicity(t *testing.T) {
	for _, gate := range []string{"no-feedback", "pause", "slow"} {
		t.Run(gate, func(t *testing.T) {
			s, library := rasterWorldTest()
			switch gate {
			case "no-feedback":
				s.Clock.PanOffset = 0
			case "pause":
				s.Clock.PauseFlags = 128
			case "slow":
				s.Clock.SlowFlag = 1
			}
			before := append([]byte(nil), s.Stream...)
			next, _, err := StepChuWorldObjects(s, ChuWorldView{}, library)
			if err != nil || !bytes.Equal(next.Stream, before) || !bytes.Equal(s.Stream, before) {
				t.Fatal("inactive feedback altered stream", err)
			}
		})
	}
	for _, failure := range []string{"late-callback", "nil", "short"} {
		t.Run(failure, func(t *testing.T) {
			s, library := rasterWorldTest()
			switch failure {
			case "late-callback":
				s.Objects[0x8300] = WorldObject{ObjectID: 0x8300, Header: MatchObjectHeader{Behavior: 0xdeadbeef, Size: 64}}
			case "nil":
				s.Stream = nil
			case "short":
				s.Stream = s.Stream[:1023]
			}
			before, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			next, events, err := StepChuWorldObjects(s, ChuWorldView{}, library)
			after, marshalErr := json.Marshal(s)
			if err == nil || marshalErr != nil || len(events) != 0 || !reflect.DeepEqual(next, s) || !bytes.Equal(before, after) {
				t.Fatal("failed feedback pass was not rejected atomically", err, marshalErr)
			}
		})
	}
}

func TestRasterInputStorageLegacyJSON(t *testing.T) {
	f, err := os.Open("testdata/raster-input-legacy-worlds.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var cases []struct {
		Name  string
		State json.RawMessage
	}
	if err := json.NewDecoder(z).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 6 {
		t.Fatal("lost legacy JSON fixture coverage", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var state ChuWorldState
			if err := json.Unmarshal(tc.State, &state); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(state)
			if err != nil || !bytes.Equal(encoded, tc.State) {
				t.Fatal("legacy field order, omission or complete world JSON bytes changed", err)
			}
		})
	}
	if _, err := io.Copy(io.Discard, z); err != nil {
		t.Fatal(err)
	}
}

func TestRasterInputStorageIgnoresLegacyProjection(t *testing.T) {
	s, _ := rasterWorldTest()
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	wire["PanA"], wire["PanB"] = json.RawMessage(`[12345]`), json.RawMessage(`[-23456]`)
	conflict, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var restored ChuWorldState
	if err := json.Unmarshal(conflict, &restored); err != nil {
		t.Fatal(err)
	}
	rows := rasterJSONRowsTest(t, restored)
	for plane := 0; plane < 2; plane++ {
		for row, value := range rows[plane] {
			if value != int16(binary.BigEndian.Uint16(s.Stream[plane*512+row*2:])) {
				t.Fatal("legacy derived projection became an independent state owner", plane, row)
			}
		}
	}
	if !bytes.Equal(restored.Stream, s.Stream) {
		t.Fatal("legacy projection replaced canonical stream")
	}
}
