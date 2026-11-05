package syncwire

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/yuyuhakusho/engine"
)

func frameTest(frame uint64, value engine.Buttons, mask uint8) InputFrame {
	return InputFrame{Frame: frame, Held: [4]engine.Buttons{value, value ^ 0x123, value ^ 0x456, value ^ 0x789}, Automated: mask}
}
func TestWindowExactMD6AndUint64Format(t *testing.T) {
	in := []InputFrame{{Frame: 9007199254740993, Held: [4]engine.Buttons{1, 4095, 256, 2048}, Automated: 10}}
	got, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.FirstFrame != in[0].Frame || got.FrameCount != 1 || got.Data != "AQoAAQ//AQAIAA==" {
		t.Fatal("known12bit big-endian record changed", got)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"firstFrame":"9007199254740993"`)) {
		t.Fatal("frame lost safe JSON string", string(raw))
	}
	out, err := Decode(got)
	if err != nil || !reflect.DeepEqual(out, in) {
		t.Fatal(out, err)
	}
	parsed, err := DecodeJSON(raw)
	if err != nil || !reflect.DeepEqual(parsed, in) {
		t.Fatal("strict JSON differs", err)
	}
	for seat := 0; seat < 4; seat++ {
		for buttons := 0; buttons <= 4095; buttons++ {
			row := InputFrame{Frame: 1, Automated: 15}
			row.Held[seat] = engine.Buttons(buttons)
			w, err := Encode([]InputFrame{row})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := Decode(w)
			if err != nil || decoded[0] != row {
				t.Fatal("MD input bits changed", seat, buttons, err)
			}
		}
	}
}
func TestWindowRLEAndEmptyCanonical(t *testing.T) {
	empty, err := Encode(nil)
	if err != nil || empty != (InputWindow{Version: 1}) {
		t.Fatal(empty, err)
	}
	if frames, err := Decode(empty); err != nil || len(frames) != 0 {
		t.Fatal(frames, err)
	}
	rows := make([]InputFrame, 120)
	for i := range rows {
		rows[i] = frameTest(uint64(i+1), 0xfff, 3)
	}
	w, err := Encode(rows)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.StdEncoding.DecodeString(w.Data)
	if len(raw) != 10 || raw[0] != 120 {
		t.Fatal("equal inputs were not one bounded run", raw)
	}
	decoded, err := Decode(w)
	if err != nil || !reflect.DeepEqual(decoded, rows) {
		t.Fatal(err)
	}
	for i := 60; i < 120; i++ {
		rows[i].Automated = 7
	}
	w, err = Encode(rows)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = base64.StdEncoding.DecodeString(w.Data)
	if len(raw) != 20 || raw[0] != 60 || raw[10] != 60 {
		t.Fatal("automation effective frame was lost", raw)
	}
	decoded, err = Decode(w)
	if err != nil || !reflect.DeepEqual(decoded, rows) {
		t.Fatal("automation masks did not roundtrip", err)
	}
	max := []InputFrame{{Frame: math.MaxUint64, Held: [4]engine.Buttons{0, 0, 0, 4095}, Automated: 8}}
	w, err = Encode(max)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err = Decode(w)
	if err != nil || !reflect.DeepEqual(decoded, max) {
		t.Fatal("last uint64 frame rejected", err)
	}
}
func TestWindowRejectsInvalidInputsAtomically(t *testing.T) {
	for _, rows := range [][]InputFrame{{{Frame: 0}}, {frameTest(1, 0, 0), frameTest(1, 0, 0)}, {frameTest(2, 0, 0), frameTest(1, 0, 0)}, {frameTest(1, 0, 0), frameTest(3, 0, 0)}, {frameTest(math.MaxUint64, 0, 0), frameTest(0, 0, 0)}, make([]InputFrame, 121)} {
		before := append([]InputFrame(nil), rows...)
		w, err := Encode(rows)
		if err == nil || w != (InputWindow{}) || !reflect.DeepEqual(before, rows) {
			t.Fatal("invalid input emitted partial window or changed caller", rows, err)
		}
	}
	for seat := 0; seat < 4; seat++ {
		for _, bad := range []engine.Buttons{0x1000, 0x8000, 0xffff} {
			row := InputFrame{Frame: 1}
			row.Held[seat] = bad
			if w, err := Encode([]InputFrame{row}); err == nil || w != (InputWindow{}) {
				t.Fatal("bad MD bit accepted", seat, bad, err)
			}
		}
	}
	for _, mask := range []uint8{16, 128, 255} {
		if _, err := Encode([]InputFrame{{Frame: 1, Automated: mask}}); err == nil {
			t.Fatal("invalid automation bit accepted", mask)
		}
	}
}
func TestWindowRejectsMalformedWireAtomically(t *testing.T) {
	valid, _ := Encode([]InputFrame{frameTest(1, 0, 3)})
	raw, _ := base64.StdEncoding.DecodeString(valid.Data)
	cases := []InputWindow{{}, {Version: 2}, {Version: 1, FirstFrame: 1}, {Version: 1, Data: valid.Data}, {Version: 1, FirstFrame: 0, FrameCount: 1, Data: valid.Data}, {Version: 1, FirstFrame: 1, FrameCount: 121, Data: valid.Data}, {Version: 1, FirstFrame: math.MaxUint64, FrameCount: 2, Data: valid.Data}}
	mutate := func(change func([]byte), count uint16) {
		data := append([]byte(nil), raw...)
		change(data)
		cases = append(cases, InputWindow{Version: 1, FirstFrame: 1, FrameCount: count, Data: base64.StdEncoding.EncodeToString(data)})
	}
	mutate(func(b []byte) { b[0] = 0 }, 1)
	mutate(func(b []byte) { b[0] = 121 }, 1)
	mutate(func(b []byte) { b[0] = 2 }, 1)
	mutate(func(b []byte) { b[1] = 16 }, 1)
	mutate(func(b []byte) { b[2] = 0x10 }, 1)
	for _, data := range []string{"!", valid.Data + "\n", valid.Data + "=", valid.Data[:len(valid.Data)-1], base64.StdEncoding.EncodeToString(raw[:9]), base64.StdEncoding.EncodeToString(append(append([]byte(nil), raw...), raw...)), "AQMAAAEjBFYHiR=="} {
		cases = append(cases, InputWindow{Version: 1, FirstFrame: 1, FrameCount: 1, Data: data})
	}
	// Same payload represented by two adjacent runs is noncanonical.
	split := append(append([]byte(nil), raw...), raw...)
	cases = append(cases, InputWindow{Version: 1, FirstFrame: 1, FrameCount: 2, Data: base64.StdEncoding.EncodeToString(split)})
	for _, w := range cases {
		before := w
		out, err := Decode(w)
		if err == nil || out != nil || w != before {
			t.Fatal("bad wire accepted or partially decoded", w, err)
		}
	}
}
func TestWindowStrictJSONBoundary(t *testing.T) {
	valid, _ := Encode([]InputFrame{frameTest(1, 0, 0)})
	raw, _ := json.Marshal(valid)
	cases := []string{"", `null`, `[]`, `{}`, `{"version":1,"firstFrame":"1","frameCount":1,"data":null}`, `{"version":1,"firstFrame":1,"frameCount":1,"data":""}`, `{"version":1,"firstFrame":"01","frameCount":1,"data":""}`, `{"version":1,"firstFrame":"+1","frameCount":1,"data":""}`, `{"version":1,"firstFrame":"18446744073709551616","frameCount":1,"data":""}`, string(raw) + ` {}`, string(bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)), string(bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"unknown":0`), 1)), string(bytes.Replace(raw, []byte(`"version":1`), []byte(`"Version":1`), 1))}
	for _, text := range cases {
		out, err := DecodeJSON([]byte(text))
		if err == nil || out != nil {
			t.Fatal("invalid JSON accepted", text, err)
		}
	}
}
func TestHistoryPushCopiesAndRetainsOnly120(t *testing.T) {
	var history []InputFrame
	for frame := uint64(100); frame < 350; frame++ {
		before := append([]InputFrame(nil), history...)
		next := frameTest(frame, engine.Buttons(frame&4095), uint8(frame&15))
		updated, err := Push(history, next)
		if err != nil || !reflect.DeepEqual(history, before) {
			t.Fatal("push mutated old history", frame, err)
		}
		history = updated
		if len(history) > 120 || cap(history) > 120 {
			t.Fatal("unbounded history allocation", len(history), cap(history))
		}
	}
	if len(history) != 120 || history[0].Frame != 230 || history[119].Frame != 349 {
		t.Fatal("wrong retained window")
	}
	before := append([]InputFrame(nil), history...)
	for _, row := range []InputFrame{frameTest(349, 0, 0), frameTest(351, 0, 0), {Frame: 350, Automated: 16}} {
		out, err := Push(history, row)
		if err == nil || out != nil || !reflect.DeepEqual(before, history) {
			t.Fatal("bad append changed history", err)
		}
	}
	history[1].Frame = history[0].Frame
	if out, err := Push(history, frameTest(350, 0, 0)); err == nil || out != nil {
		t.Fatal("invalid retained history accepted")
	}
}
