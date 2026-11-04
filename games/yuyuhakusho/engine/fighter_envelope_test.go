package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

type envelopeSnapshot struct {
	Actor      string
	StreamBase uint32 `json:"stream_base"`
	Stream     string
	Input      ControllerFrame
	Disabled   uint8
	Stopped    uint8
	Markers    [4]uint8
	CameraX    int16 `json:"camera_x"`
	CameraY    int16 `json:"camera_y"`
	Shake      int16
	Cells      string
	Queue      string
	Cursor     uint16
	Blink      uint8
	Owners     string
	Used       uint16
}

type envelopeCall struct {
	Kind, Name, Route string
	ObjectID          uint16 `json:"object_id"`
	Before, After     envelopeSnapshot
	External          *ControllerFrame `json:"external_input"`
}

func envelopeBytes(t *testing.T, value string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(value)
	if err != nil || len(b) != size {
		t.Fatalf("invalid original envelope bytes: %v, %d/%d", err, len(b), size)
	}
	return b
}

func envelopeWords(t *testing.T, value string, size int) []uint16 {
	b := envelopeBytes(t, value, size)
	words := make([]uint16, size/2)
	for i := range words {
		words[i] = binary.BigEndian.Uint16(b[i*2:])
	}
	return words
}

func encodeEnvelopeWords(words []uint16) string {
	b := make([]byte, len(words)*2)
	for i, word := range words {
		binary.BigEndian.PutUint16(b[i*2:], word)
	}
	return hex.EncodeToString(b)
}

func decodeEnvelopeInput(t *testing.T, raw envelopeSnapshot) FighterInputEnvelopeState {
	a := envelopeBytes(t, raw.Actor, 256)
	return FighterInputEnvelopeState{Mode: int8(a[0xc9]), Channel: a[0x76], Disabled: raw.Disabled, Stopped: raw.Stopped,
		Position: binary.BigEndian.Uint32(a[0xca:]), StreamBase: raw.StreamBase, Stream: envelopeBytes(t, raw.Stream, 1024), Input: raw.Input, EndMarkers: raw.Markers,
		Controls: FighterControls{Held: Buttons(a[0x71]) | Buttons(a[0xc0])<<8, Pressed: Buttons(a[0x72]) | Buttons(a[0xc1])<<8,
			Changed: Buttons(a[0x73]) | Buttons(a[0xc2])<<8, RawHeld: a[0x74], RetainTicks: a[0x75]}}
}

func encodeEnvelopeInput(t *testing.T, state FighterInputEnvelopeState, previous envelopeSnapshot) envelopeSnapshot {
	raw := previous
	a := envelopeBytes(t, raw.Actor, 256)
	a[0xc9], a[0x76] = uint8(state.Mode), state.Channel
	a[0x71], a[0xc0] = uint8(state.Controls.Held), uint8(state.Controls.Held>>8)
	a[0x72], a[0xc1] = uint8(state.Controls.Pressed), uint8(state.Controls.Pressed>>8)
	a[0x73], a[0xc2] = uint8(state.Controls.Changed), uint8(state.Controls.Changed>>8)
	a[0x74], a[0x75] = state.Controls.RawHeld, state.Controls.RetainTicks
	binary.BigEndian.PutUint32(a[0xca:], state.Position)
	raw.Actor, raw.StreamBase, raw.Stream = hex.EncodeToString(a), state.StreamBase, hex.EncodeToString(state.Stream)
	raw.Input, raw.Disabled, raw.Stopped, raw.Markers = state.Input, state.Disabled, state.Stopped, state.EndMarkers
	return raw
}

func decodeEnvelopeHUD(t *testing.T, raw envelopeSnapshot) FighterHUDState {
	a := envelopeBytes(t, raw.Actor, 256)
	w := func(off int) uint16 { return binary.BigEndian.Uint16(a[off:]) }
	return FighterHUDState{Health: int16(w(0x78)), DisplayedHealth: int16(w(0x7a)), Energy: int16(w(0xb8)), DisplayedEnergy: int16(w(0xbc)),
		Offset: w(0x8a), TargetBit: a[0x70], Blink: raw.Blink, Cells: envelopeWords(t, raw.Cells, 1024), TransferCursor: raw.Cursor}
}

func encodeEnvelopeHUD(t *testing.T, state FighterHUDState, transfers []FighterBarTransfer, previous envelopeSnapshot) envelopeSnapshot {
	raw := previous
	a := envelopeBytes(t, raw.Actor, 256)
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Health))
	binary.BigEndian.PutUint16(a[0x7a:], uint16(state.DisplayedHealth))
	binary.BigEndian.PutUint16(a[0xb8:], uint16(state.Energy))
	binary.BigEndian.PutUint16(a[0xbc:], uint16(state.DisplayedEnergy))
	binary.BigEndian.PutUint16(a[0x8a:], state.Offset)
	a[0x70] = state.TargetBit
	raw.Actor, raw.Cells, raw.Blink = hex.EncodeToString(a), encodeEnvelopeWords(state.Cells), state.Blink
	q := envelopeBytes(t, raw.Queue, 3072)
	cursor := previous.Cursor
	for _, transfer := range transfers {
		source := uint32(0x271a00) + uint32(transfer.Pattern)*32
		wordSource := source >> 1
		destination := uint16(transfer.TargetTile << 5)
		command := uint32(destination&0x3fff)<<16 | uint32(destination>>14) | 0x40000080
		words := [...]uint16{0x9310, 0x9400, 0x9500 | uint16(wordSource&255), 0x9600 | uint16(wordSource>>8&255), 0x9700 | uint16(wordSource>>16&127), 0x8f02, uint16(command >> 16), uint16(command)}
		index := int(cursor) - 0xd400
		if index < 0 || index+16 > len(q) {
			t.Fatal("test adapter transfer exceeds declared original queue")
		}
		for j, word := range words {
			binary.BigEndian.PutUint16(q[index+j*2:], word)
		}
		cursor += 16
	}
	if cursor != state.TransferCursor {
		t.Fatal("semantic HUD transfers do not explain cursor")
	}
	raw.Queue, raw.Cursor = hex.EncodeToString(q), state.TransferCursor
	return raw
}

func readEnvelopeFixture(t *testing.T, name string, target any) int {
	t.Helper()
	compressed, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Pass    bool
		Records int
		Match   bool   `json:"frames_match_original_core"`
		SHA     string `json:"fixture_sha256"`
		Gzip    string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !m.Pass || !m.Match || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original envelope fixture")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return m.Records
}

func applyEnvelopeCall(t *testing.T, row envelopeCall) envelopeSnapshot {
	t.Helper()
	before := row.Before
	switch row.Kind {
	case "input", "record":
		input, copy := decodeEnvelopeInput(t, before), decodeEnvelopeInput(t, before)
		var got FighterInputEnvelopeState
		var route FighterInputEnvelopeRoute
		var err error
		if row.Kind == "input" {
			got, route, err = PrepareFighterInputEnvelope(input)
		} else {
			got, err = RecordFighterInputEnvelope(input)
		}
		wantRoute := FighterInputComplete
		if row.Route == "ai" {
			wantRoute = FighterInputNeedsAI
		} else if row.Route == "ai-record" {
			wantRoute = FighterInputNeedsAIThenRecord
		} else if row.Route != "complete" {
			t.Fatal("unknown original envelope route")
		}
		if err != nil || route != wantRoute || !reflect.DeepEqual(got, decodeEnvelopeInput(t, row.After)) || !reflect.DeepEqual(input, copy) {
			t.Fatalf("input envelope %s/%s differs: %v route=%d/%d", row.Kind, row.Name, err, route, wantRoute)
		}
		return encodeEnvelopeInput(t, got, before)
	case "hud":
		input, copy := decodeEnvelopeHUD(t, before), decodeEnvelopeHUD(t, before)
		got, transfers, err := StepFighterHUD(input)
		if err != nil || !reflect.DeepEqual(got, decodeEnvelopeHUD(t, row.After)) || !reflect.DeepEqual(input, copy) {
			t.Fatalf("HUD envelope %s differs: %v", row.Name, err)
		}
		return encodeEnvelopeHUD(t, got, transfers, before)
	case "register":
		s := FighterRegistrationState{Used: before.Used, Owners: envelopeWords(t, before.Owners, 128)}
		original := append([]uint16(nil), s.Owners...)
		got, err := RegisterFighter(s, row.ObjectID)
		if err != nil || !reflect.DeepEqual(s.Owners, original) {
			t.Fatalf("registration changed caller input: %v", err)
		}
		raw := before
		raw.Owners, raw.Used = encodeEnvelopeWords(got.Owners), got.Used
		return raw
	case "project":
		a := envelopeBytes(t, before.Actor, 256)
		w := func(off int) int16 { return int16(binary.BigEndian.Uint16(a[off:])) }
		s := ProjectFighter(ProjectionState{WorldX: int32(binary.BigEndian.Uint32(a[0x40:])), WorldY: int32(binary.BigEndian.Uint32(a[0x44:])),
			ScreenX: w(8), ScreenY: w(12), OffsetX: w(0x54), OffsetY: w(0x56), CameraX: before.CameraX, CameraY: before.CameraY, Shake: before.Shake})
		binary.BigEndian.PutUint16(a[8:], uint16(s.ScreenX))
		binary.BigEndian.PutUint16(a[12:], uint16(s.ScreenY))
		binary.BigEndian.PutUint16(a[0x54:], uint16(s.OffsetX))
		binary.BigEndian.PutUint16(a[0x56:], uint16(s.OffsetY))
		raw := before
		raw.Actor = hex.EncodeToString(a)
		return raw
	default:
		t.Fatal("unknown original envelope kind", row.Kind)
		return before
	}
}

// Full original function/prefix fixtures and these tests precede the new rules.
func TestFighterEnvelopeOriginalCalls(t *testing.T) {
	for _, name := range []string{"fighter-envelope-oracle", "fighter-envelope-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []envelopeCall
			count := readEnvelopeFixture(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original envelope calls")
			}
			for i, row := range rows {
				got := applyEnvelopeCall(t, row)
				if !reflect.DeepEqual(got, row.After) {
					t.Fatalf("envelope raw output %d %s/%s differs", i, row.Kind, row.Name)
				}
			}
			t.Logf("complete function or explicit AI prefix records=%d", count)
		})
	}
}

func TestFighterEnvelopeSingleInitialState(t *testing.T) {
	var streams []struct {
		Kind    string
		Initial envelopeSnapshot
		Calls   []envelopeCall
	}
	count := readEnvelopeFixture(t, "fighter-envelope-sequences", &streams)
	if len(streams) != count || count != 6 {
		t.Fatal("missing envelope streams")
	}
	steps := 0
	for _, stream := range streams {
		encoded := stream.Initial
		for i, step := range stream.Calls {
			// Only the actual selected external physical input changes.
			if step.External != nil {
				encoded.Input = *step.External
			}
			step.Before = encoded
			encoded = applyEnvelopeCall(t, step)
			if !reflect.DeepEqual(encoded, step.After) {
				t.Fatalf("envelope stream %s step%d differs", stream.Kind, i)
			}
			steps++
		}
	}
	if steps != 72 {
		t.Fatal("missing isolated envelope steps")
	}
}

func TestFighterEnvelopeMalformedStorageIsAtomic(t *testing.T) {
	s := FighterInputEnvelopeState{Mode: 1, Position: 100, StreamBase: 100, Stream: []byte{1, 2, 3}, Controls: FighterControls{Held: 7}, Channel: 0}
	got, _, err := PrepareFighterInputEnvelope(s)
	if err == nil || !reflect.DeepEqual(got, s) {
		t.Fatal("invalid playback storage changed state")
	}
	s.Mode = -1
	got, err = RecordFighterInputEnvelope(s)
	if err == nil || !reflect.DeepEqual(got, s) {
		t.Fatal("invalid record storage changed state")
	}
	s.Mode, s.Channel, s.Disabled = 0, 8, 0
	got, _, err = PrepareFighterInputEnvelope(s)
	if err == nil || !reflect.DeepEqual(got, s) {
		t.Fatal("unsupported human channel changed state")
	}
	hud := FighterHUDState{Health: 100, DisplayedHealth: 1, Energy: 100, DisplayedEnergy: 2, Cells: make([]uint16, 10), TransferCursor: 0xd400}
	hcopy := append([]uint16(nil), hud.Cells...)
	hgot, transfers, err := StepFighterHUD(hud)
	if err == nil || len(transfers) != 0 || !reflect.DeepEqual(hgot, hud) || !reflect.DeepEqual(hud.Cells, hcopy) {
		t.Fatal("late HUD storage failure leaked state")
	}
	for _, used := range []uint16{1, 4, 0xffff} {
		registration := FighterRegistrationState{Used: used, Owners: []uint16{0x1111, 0x2222}}
		rgot, err := RegisterFighter(registration, 0x8400)
		if err == nil || !reflect.DeepEqual(rgot, registration) {
			t.Fatal("invalid registration storage changed state")
		}
	}
	// MOVEA.W sign-extends the byte cursor: an overprovisioned logical slice
	// must not turn a negative original offset into a positive array index.
	large := FighterRegistrationState{Used: 0x8000, Owners: make([]uint16, 32768)}
	rgot, err := RegisterFighter(large, 0x8400)
	if err == nil || !reflect.DeepEqual(rgot, large) {
		t.Fatal("negative registration offset was treated as positive storage")
	}
}
