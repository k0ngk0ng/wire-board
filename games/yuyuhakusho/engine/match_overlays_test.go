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

type matchOverlaySnapshot struct {
	Objects      string
	ShadowBytes  string `json:"shadow_bytes"`
	Owners       map[uint16]string
	Registration string
	Used         uint16
	Queue        [4]uint8
	Pause, Slow  uint8
	Attributes   uint16
	CameraX      int16 `json:"camera_x"`
	CameraY      int16 `json:"camera_y"`
	Shake        int16
	Winner       int16
	Characters   [4]uint8
	TeamTitles   [2]uint16 `json:"team_titles"`
	Mode         uint8
	Round        uint16
	Blink        uint8
	Held         [4]uint8
	Extended     [4]uint8
}

type matchOverlayCall struct {
	Kind             string
	Name             string
	ObjectID         uint16 `json:"object_id"`
	Before, After    matchOverlaySnapshot
	Draw             bool
	Cleared          uint16  `json:"cleared_bytes"`
	ExternalUsed     *uint16 `json:"external_registration_used"`
	ExternalMode     *uint8  `json:"external_mode"`
	ExternalBlink    *uint8  `json:"external_blink"`
	ExternalTraining *struct {
		Pause          uint8
		Held, Extended [4]uint8
	} `json:"external_training"`
	ReferenceOnly bool `json:"reference_only"`
}

func overlayBytes(t *testing.T, text string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(text)
	if err != nil || len(b) != size {
		t.Fatalf("invalid original overlay bytes: %v size %d/%d", err, len(b), size)
	}
	return b
}

func overlayObject(b []byte) MatchOverlayObject {
	w := func(o int) uint16 { return binary.BigEndian.Uint16(b[o:]) }
	l := func(o int) uint32 { return binary.BigEndian.Uint32(b[o:]) }
	return MatchOverlayObject{Header: MatchObjectHeader{Behavior: l(0), Size: w(4), Stage: w(6)}, ScreenX: int16(w(8)), ScreenY: int16(w(12)),
		Counter: w(0x10), Timer: w(0x18), Data: w(0x1a), GraphicsSet: l(0x1c), TilesBase: l(0x20), Attributes: w(0x24), Layer: b[0x26], SkipGraphics: b[0x27],
		GraphicOffsetX: int16(w(0x28)), GraphicOffsetY: int16(w(0x2a)), GraphicCache: [4]uint32{l(0x2c), l(0x30), l(0x34), l(0x38)}, Parameter: w(0x3c), Depth: w(0x3e)}
}

func encodeOverlayObject(s MatchOverlayObject, b []byte) {
	w := func(o int, v uint16) { binary.BigEndian.PutUint16(b[o:], v) }
	l := func(o int, v uint32) { binary.BigEndian.PutUint32(b[o:], v) }
	l(0, s.Header.Behavior)
	w(4, s.Header.Size)
	w(6, s.Header.Stage)
	w(8, uint16(s.ScreenX))
	w(12, uint16(s.ScreenY))
	w(0x10, s.Counter)
	w(0x18, s.Timer)
	w(0x1a, s.Data)
	l(0x1c, s.GraphicsSet)
	l(0x20, s.TilesBase)
	w(0x24, s.Attributes)
	b[0x26], b[0x27] = s.Layer, s.SkipGraphics
	w(0x28, uint16(s.GraphicOffsetX))
	w(0x2a, uint16(s.GraphicOffsetY))
	for i, value := range s.GraphicCache {
		l(0x2c+i*4, value)
	}
	w(0x3c, s.Parameter)
	w(0x3e, s.Depth)
}

func decodeMatchOverlay(t *testing.T, raw matchOverlaySnapshot, id uint16, kind string) MatchOverlayState {
	t.Helper()
	b := overlayBytes(t, raw.Objects, 768)
	off := int(id) - 0x8100
	s := MatchOverlayState{ObjectID: id, Object: overlayObject(b[off:]), SoundQueue: raw.Queue, ModeFlag: raw.Mode, SlowFlag: raw.Slow}
	if kind == "intro" {
		s.Children[0], s.Children[1] = overlayObject(b[off+64:]), overlayObject(b[off+128:])
	}
	return s
}

func decodeMatchShadow(t *testing.T, raw matchOverlaySnapshot) (MatchShadowState, map[uint16]MatchShadowOwner) {
	t.Helper()
	b := overlayBytes(t, raw.ShadowBytes, 128)
	s := MatchShadowState{Object: overlayObject(b), X: int32(binary.BigEndian.Uint32(b[0x40:])), Y: int32(binary.BigEndian.Uint32(b[0x44:])),
		Cursor: binary.BigEndian.Uint16(b[0x50:]), ScreenOffsetX: int16(binary.BigEndian.Uint16(b[0x54:])), ScreenOffsetY: int16(binary.BigEndian.Uint16(b[0x56:])),
		Registration: FighterRegistrationState{Used: raw.Used, Owners: make([]uint16, 64)}}
	registration := overlayBytes(t, raw.Registration, 128)
	for i := range s.Registration.Owners {
		s.Registration.Owners[i] = binary.BigEndian.Uint16(registration[i*2:])
	}
	owners := make(map[uint16]MatchShadowOwner, len(raw.Owners))
	for id, text := range raw.Owners {
		p := overlayBytes(t, text, 256)
		owners[id] = MatchShadowOwner{Depth: binary.BigEndian.Uint16(p[0x3e:]), X: int32(binary.BigEndian.Uint32(p[0x40:])), Ground: int32(binary.BigEndian.Uint32(p[0x9c:]))}
	}
	return s, owners
}

func stepMatchOverlayReference(t *testing.T, input matchOverlaySnapshot, row matchOverlayCall) (matchOverlaySnapshot, MatchOverlayOutcome) {
	t.Helper()
	actual := input
	b := overlayBytes(t, input.Objects, 768)
	if row.Kind == "training" {
		var state MatchTrainingState
		ids := [4]uint16{0x8400, 0x8800, 0x8c00, 0x9000}
		for i, id := range ids {
			owner := overlayBytes(t, input.Owners[id], 256)
			state.Targets[i] = MatchHealthTarget{Behavior: binary.BigEndian.Uint32(owner), Health: int16(binary.BigEndian.Uint16(owner[0x78:]))}
		}
		got := StepMatchTraining(state, input.Pause, input.Held, input.Extended)
		actual.Owners = make(map[uint16]string, len(input.Owners))
		for id, text := range input.Owners {
			actual.Owners[id] = text
		}
		for i, id := range ids {
			owner := overlayBytes(t, input.Owners[id], 256)
			binary.BigEndian.PutUint32(owner, got.Targets[i].Behavior)
			binary.BigEndian.PutUint16(owner[0x78:], uint16(got.Targets[i].Health))
			actual.Owners[id] = hex.EncodeToString(owner)
		}
		return actual, MatchOverlayOutcome{}
	}
	if row.Kind == "shadow" {
		b = overlayBytes(t, input.ShadowBytes, 128)
		state, owners := decodeMatchShadow(t, input)
		copy, ownerCopy := decodeMatchShadow(t, input)
		got, outcome, err := StepMatchShadow(state, MatchShadowEnvironment{AttributeMask: input.Attributes, CameraX: input.CameraX, CameraY: input.CameraY, Shake: input.Shake}, owners)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		encodeOverlayObject(got.Object, b)
		binary.BigEndian.PutUint32(b[0x40:], uint32(got.X))
		binary.BigEndian.PutUint32(b[0x44:], uint32(got.Y))
		binary.BigEndian.PutUint16(b[0x50:], got.Cursor)
		binary.BigEndian.PutUint16(b[0x54:], uint16(got.ScreenOffsetX))
		binary.BigEndian.PutUint16(b[0x56:], uint16(got.ScreenOffsetY))
		actual.Used, actual.ShadowBytes = got.Registration.Used, hex.EncodeToString(b)
		if !reflect.DeepEqual(state, copy) || !reflect.DeepEqual(owners, ownerCopy) {
			t.Fatal("shadow modified caller registration/owners")
		}
		want, _ := decodeMatchShadow(t, row.After)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s semantic shadow differs: object=%+v/%+v XY=%x,%x/%x,%x cursor=%x/%x offsets=%d,%d/%d,%d used=%d/%d", row.Name,
				got.Object, want.Object, got.X, got.Y, want.X, want.Y, got.Cursor, want.Cursor, got.ScreenOffsetX, got.ScreenOffsetY, want.ScreenOffsetX, want.ScreenOffsetY, got.Registration.Used, want.Registration.Used)
		}
		return actual, outcome
	}
	state := decodeMatchOverlay(t, input, row.ObjectID, row.Kind)
	copy := state
	got, outcome, err := StepMatchOverlay(state, MatchOverlayEnvironment{PauseFlags: input.Pause, WinnerCode: input.Winner, Characters: input.Characters, TeamTitles: input.TeamTitles, RoundIndex: input.Round, Blink: input.Blink})
	if row.ReferenceOnly {
		if err == nil || got != state || outcome != (MatchOverlayOutcome{}) {
			t.Fatal("unsupported zero-graphic round was not atomically rejected")
		}
		return actual, outcome
	}
	if err != nil {
		t.Fatal(row.Name, err)
	}
	off := int(row.ObjectID) - 0x8100
	if outcome.ClearedBytes > 0 {
		clear(b[off : off+int(outcome.ClearedBytes)])
	}
	encodeOverlayObject(got.Object, b[off:])
	if row.Kind == "intro" {
		encodeOverlayObject(got.Children[0], b[off+64:])
		encodeOverlayObject(got.Children[1], b[off+128:])
	}
	actual.Objects, actual.Queue, actual.Mode, actual.Slow = hex.EncodeToString(b), got.SoundQueue, got.ModeFlag, got.SlowFlag
	if state != copy || got != decodeMatchOverlay(t, row.After, row.ObjectID, row.Kind) {
		t.Fatalf("%s semantic overlay differs: got=%+v want=%+v", row.Name, got, decodeMatchOverlay(t, row.After, row.ObjectID, row.Kind))
	}
	return actual, outcome
}

func readMatchOverlayFixture(t *testing.T, name string) (json.RawMessage, int) {
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
		Pass     bool
		Records  int
		External int    `json:"external_queue_calls"`
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
	if !m.Pass || !m.Baseline || m.External != 0 || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original match overlay evidence")
	}
	var fixture struct{ Calls json.RawMessage }
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Calls, m.Records
}

// These original fixtures and tests precede match_overlays.go.
func TestMatchOverlaysOriginalCalls(t *testing.T) {
	for _, name := range []string{"match-overlays-oracle", "match-overlays-boundaries"} {
		t.Run(name, func(t *testing.T) {
			raw, count := readMatchOverlayFixture(t, name)
			dec := json.NewDecoder(bytes.NewReader(raw))
			if token, err := dec.Token(); err != nil || token != json.Delim('[') {
				t.Fatal("missing original rows", err)
			}
			seen, kinds := 0, map[string]int{}
			for dec.More() {
				var row matchOverlayCall
				if err := dec.Decode(&row); err != nil {
					t.Fatal(err)
				}
				actual, outcome := stepMatchOverlayReference(t, row.Before, row)
				if row.ReferenceOnly {
					seen++
					kinds[row.Kind]++
					continue
				}
				if !reflect.DeepEqual(actual, row.After) || outcome.Draw != row.Draw || outcome.ClearedBytes != row.Cleared {
					t.Fatalf("complete overlay %d %s/%s differs: draw %v/%v clear %d/%d", seen, row.Kind, row.Name, outcome.Draw, row.Draw, outcome.ClearedBytes, row.Cleared)
				}
				seen++
				kinds[row.Kind]++
			}
			if seen != count || name == "match-overlays-boundaries" && len(kinds) != 14 {
				t.Fatal("missing callback coverage", seen, count, kinds)
			}
			t.Logf("complete original overlays=%d kinds=%v", count, kinds)
		})
	}
}

func TestMatchOverlaysSingleInitialState(t *testing.T) {
	raw, count := readMatchOverlayFixture(t, "match-overlays-sequences")
	var streams []struct {
		Kind    string
		Initial matchOverlaySnapshot
		Calls   []matchOverlayCall
	}
	if err := json.Unmarshal(raw, &streams); err != nil || len(streams) != count || count != 14 {
		t.Fatal("missing original lifecycle streams", err)
	}
	steps := 0
	for _, stream := range streams {
		encoded := stream.Initial
		for i, row := range stream.Calls {
			if row.ExternalUsed != nil {
				encoded.Used = *row.ExternalUsed
			}
			if row.ExternalMode != nil {
				encoded.Mode = *row.ExternalMode
			}
			if row.ExternalBlink != nil {
				encoded.Blink = *row.ExternalBlink
			}
			if row.ExternalTraining != nil {
				encoded.Pause = row.ExternalTraining.Pause
				encoded.Held = row.ExternalTraining.Held
				encoded.Extended = row.ExternalTraining.Extended
			}
			actual, outcome := stepMatchOverlayReference(t, encoded, row)
			if !reflect.DeepEqual(actual, row.After) || outcome.Draw != row.Draw || outcome.ClearedBytes != row.Cleared {
				t.Fatalf("stream %s step %d differs", stream.Kind, i)
			}
			encoded = actual // Only independently computed state is reused.
			steps++
		}
	}
	if steps != 2372 {
		t.Fatal("missing original lifecycle steps", steps)
	}
}

func TestMatchOverlaysErrorsAreAtomic(t *testing.T) {
	base := MatchOverlayState{ObjectID: 0x8340, Object: MatchOverlayObject{Header: MatchObjectHeader{Behavior: 0x10522, Size: 64, Stage: 4}, Parameter: 7}}
	got, out, err := StepMatchOverlay(base, MatchOverlayEnvironment{WinnerCode: 1})
	if err == nil || got != base || out != (MatchOverlayOutcome{}) {
		t.Fatal("missing character title did not roll back resource writes")
	}
	base.Object.Header.Behavior, base.Object.Header.Stage = 0x10330, 12
	base.Object.Header.Size = 128
	got, out, err = StepMatchOverlay(base, MatchOverlayEnvironment{})
	if err == nil || got != base || out != (MatchOverlayOutcome{}) {
		t.Fatal("unsupported clear extent changed overlay")
	}
	shadow := MatchShadowState{Registration: FighterRegistrationState{Used: 2, Owners: []uint16{0x8400}}, Object: MatchOverlayObject{Header: MatchObjectHeader{Stage: 2}}}
	next, out, err := StepMatchShadow(shadow, MatchShadowEnvironment{}, nil)
	if err == nil || !reflect.DeepEqual(next, shadow) || out != (MatchOverlayOutcome{}) {
		t.Fatal("missing shadow owner changed cursor/count")
	}
}
