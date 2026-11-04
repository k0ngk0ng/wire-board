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

type attachedFXSnapshot struct {
	ObjectID    uint16 `json:"object_id"`
	ActorBytes  string `json:"actor_bytes"`
	Pool        string
	ParentID    uint16 `json:"parent_id"`
	ParentBytes string `json:"parent_bytes"`
	Cursor      uint16
	World       AttachedEffectWorld
}

func attachedFXBytes(t *testing.T, encoded string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b) != size {
		t.Fatalf("invalid original attached FX bytes: %v, %d/%d", err, len(b), size)
	}
	return b
}

func attachedFXKind(t *testing.T, name string) AttachedEffectKind {
	t.Helper()
	if name == "combo" {
		return AttachedComboNumber
	}
	if name == "status" {
		return AttachedStunEffect
	}
	t.Fatal("unknown original effect kind", name)
	return 0
}

func decodeAttachedFX(t *testing.T, name string, raw attachedFXSnapshot) (AttachedEffectState, AttachedEffectParent) {
	t.Helper()
	a := attachedFXBytes(t, raw.ActorBytes, 128)
	w := func(i int) uint16 { return binary.BigEndian.Uint16(a[i:]) }
	l := func(i int) uint32 { return binary.BigEndian.Uint32(a[i:]) }
	s := AttachedEffectState{Kind: attachedFXKind(t, name), ObjectID: raw.ObjectID,
		Slot:    EffectSlot{Behavior: l(0), Size: w(4), Stage: w(6), VelocityX: int32(l(0x10)), Attributes: w(0x24), Layer: a[0x26], Depth: w(0x3e), X: int32(l(0x40)), Y: int32(l(0x44)), Type: a[0x58]},
		ScreenX: int16(w(8)), ScreenY: int16(w(12)), GraphicsSet: l(0x1c), TilesBase: l(0x20), SkipTiles: a[0x27],
		GraphicOffsetX: int16(w(0x28)), GraphicOffsetY: int16(w(0x2a)), Parameter: w(0x3c), Aux48: w(0x48), Sequence: l(0x50),
		OffsetX: int16(w(0x54)), OffsetY: int16(w(0x56)), Index: a[0x59], Delay: a[0x5a], Ticks: a[0x5b], Flags: a[0x5c]}
	for i := range s.GraphicCache {
		s.GraphicCache[i] = l(0x2c + i*4)
	}
	p := attachedFXBytes(t, raw.ParentBytes, 256)
	parent := AttachedEffectParent{ObjectID: raw.ParentID, CharacterID: p[0x58], FrameIndex: p[0x59], Ticks: p[0x5b], ActionKind: p[0x5d],
		StunTicks: binary.BigEndian.Uint16(p[0x84:]), X: int32(binary.BigEndian.Uint32(p[0x40:])), Y: int32(binary.BigEndian.Uint32(p[0x44:]))}
	return s, parent
}

func encodeAttachedFX(t *testing.T, state AttachedEffectState, parent AttachedEffectParent, world AttachedEffectWorld, out AttachedEffectOutcome, previous attachedFXSnapshot) attachedFXSnapshot {
	t.Helper()
	raw := previous
	a := attachedFXBytes(t, previous.ActorBytes, 128)
	if out.ClearedBytes > 128 {
		t.Fatal("effect clear escaped one slot")
	}
	clear(a[:out.ClearedBytes])
	w := func(i int, value uint16) { binary.BigEndian.PutUint16(a[i:], value) }
	l := func(i int, value uint32) { binary.BigEndian.PutUint32(a[i:], value) }
	v := state.Slot
	l(0, v.Behavior)
	w(4, v.Size)
	w(6, v.Stage)
	l(0x10, uint32(v.VelocityX))
	w(0x24, v.Attributes)
	a[0x26] = v.Layer
	w(0x3e, v.Depth)
	l(0x40, uint32(v.X))
	l(0x44, uint32(v.Y))
	a[0x58] = v.Type
	w(8, uint16(state.ScreenX))
	w(12, uint16(state.ScreenY))
	l(0x1c, state.GraphicsSet)
	l(0x20, state.TilesBase)
	a[0x27] = state.SkipTiles
	w(0x28, uint16(state.GraphicOffsetX))
	w(0x2a, uint16(state.GraphicOffsetY))
	for i, value := range state.GraphicCache {
		l(0x2c+i*4, value)
	}
	w(0x3c, state.Parameter)
	w(0x48, state.Aux48)
	l(0x50, state.Sequence)
	w(0x54, uint16(state.OffsetX))
	w(0x56, uint16(state.OffsetY))
	a[0x59], a[0x5a], a[0x5b], a[0x5c] = state.Index, state.Delay, state.Ticks, state.Flags
	raw.ActorBytes = hex.EncodeToString(a)
	pool := attachedFXBytes(t, previous.Pool, 2048)
	index := int(state.ObjectID) - 0x9400
	if index < 0 || index+128 > len(pool) || index%128 != 0 {
		t.Fatal("invalid original pool identity")
	}
	copy(pool[index:index+128], a)
	raw.Pool = hex.EncodeToString(pool)
	p := attachedFXBytes(t, previous.ParentBytes, 256)
	p[0x58], p[0x59], p[0x5b], p[0x5d] = parent.CharacterID, parent.FrameIndex, parent.Ticks, parent.ActionKind
	binary.BigEndian.PutUint16(p[0x84:], parent.StunTicks)
	binary.BigEndian.PutUint32(p[0x40:], uint32(parent.X))
	binary.BigEndian.PutUint32(p[0x44:], uint32(parent.Y))
	raw.ParentID, raw.ParentBytes, raw.World = parent.ObjectID, hex.EncodeToString(p), world
	return raw
}

type attachedFXFixture struct {
	Library struct {
		Sequences []struct {
			ID     uint32
			Frames []AttachedEffectFrame
		}
		OrbitX  [1024]int8 `json:"orbit_x"`
		OrbitY  [1024]int8 `json:"orbit_y"`
		OffsetY [11]int16  `json:"offset_y"`
	}
	Calls json.RawMessage
}

func readAttachedFX(t *testing.T, name string) (attachedFXFixture, AttachedEffectLibrary, int) {
	t.Helper()
	zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
	if err != nil {
		t.Fatal(err)
	}
	z, err := gzip.NewReader(bytes.NewReader(zipped))
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
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original attached FX fixture")
	}
	var f attachedFXFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	lib := AttachedEffectLibrary{Sequences: make(map[uint32][]AttachedEffectFrame), OrbitX: f.Library.OrbitX, OrbitY: f.Library.OrbitY, OffsetY: f.Library.OffsetY}
	for _, sequence := range f.Library.Sequences {
		lib.Sequences[sequence.ID] = sequence.Frames
	}
	return f, lib, m.Records
}

type attachedFXCall struct {
	Kind, Name    string
	Before, After attachedFXSnapshot
	Draw          bool
	ClearedBytes  uint16 `json:"cleared_bytes"`
}

// Original complete-rule CPU fixtures and these tests precede attached_effects.go.
func TestAttachedEffectsOriginalRules(t *testing.T) {
	for _, name := range []string{"attached-effects-oracle", "attached-effects-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, lib, count := readAttachedFX(t, name)
			var rows []attachedFXCall
			if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count {
				t.Fatal("missing original FX calls", err)
			}
			immutable, _ := json.Marshal(f.Library)
			for i, row := range rows {
				input, parent := decodeAttachedFX(t, row.Kind, row.Before)
				want, _ := decodeAttachedFX(t, row.Kind, row.After)
				got, out, err := StepAttachedEffect(input, parent, row.Before.World, lib)
				if err != nil || out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeAttachedFX(t, got, parent, row.Before.World, out, row.Before), row.After) {
					t.Fatalf("FX %d %s/%s differs: %v; outcome=%+v want draw=%v clear=%d", i, row.Kind, row.Name, err, out, row.Draw, row.ClearedBytes)
				}
			}
			for i := range f.Library.Sequences {
				f.Library.Sequences[i].Frames = lib.Sequences[f.Library.Sequences[i].ID]
			}
			after, _ := json.Marshal(f.Library)
			if !bytes.Equal(immutable, after) {
				t.Fatal("effect mutated immutable rule data")
			}
			t.Logf("original full effect rules=%d", count)
		})
	}
}

func TestAttachedEffectsSingleInitialLifetimes(t *testing.T) {
	f, lib, count := readAttachedFX(t, "attached-effects-sequences")
	var rows []struct {
		Kind    string
		Initial attachedFXSnapshot
		Calls   []struct {
			attachedFXCall
			External struct {
				X, Y        int32
				Stun        uint16
				Action      uint8
				ParentIndex uint8 `json:"parent_index"`
				ParentTicks uint8 `json:"parent_ticks"`
				Pause, Slow uint8
			}
		}
	}
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count || count != 12 {
		t.Fatal("missing original effect lifetimes", err)
	}
	steps, deleted := 0, 0
	for _, stream := range rows {
		state, parent := decodeAttachedFX(t, stream.Kind, stream.Initial)
		encoded, world := stream.Initial, stream.Initial.World
		for i, call := range stream.Calls {
			parent.X, parent.Y, parent.StunTicks, parent.ActionKind, parent.FrameIndex, parent.Ticks = call.External.X, call.External.Y, call.External.Stun, call.External.Action, call.External.ParentIndex, call.External.ParentTicks
			world.PauseFlags, world.SlowFlag = call.External.Pause, call.External.Slow
			var out AttachedEffectOutcome
			var err error
			state, out, err = StepAttachedEffect(state, parent, world, lib)
			encoded = encodeAttachedFX(t, state, parent, world, out, encoded)
			want, _ := decodeAttachedFX(t, stream.Kind, call.After)
			if err != nil || out.Draw != call.Draw || out.ClearedBytes != call.ClearedBytes || !reflect.DeepEqual(state, want) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("FX lifetime %s step %d differs: %v", stream.Kind, i, err)
			}
			steps++
			if out.ClearedBytes != 0 {
				deleted++
			}
		}
	}
	if steps != 538 || deleted != 12 {
		t.Fatal("missing true pool recycle lifetimes", steps, deleted)
	}
}

func TestAttachedEffectsErrorsAreAtomic(t *testing.T) {
	f, lib, _ := readAttachedFX(t, "attached-effects-oracle")
	var rows []attachedFXCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	base, parent := decodeAttachedFX(t, rows[0].Kind, rows[0].Before)
	world := AttachedEffectWorld{}
	base.Kind, base.Slot.Stage, base.Aux48 = AttachedComboNumber, 2, 31
	for _, size := range []uint16{1, 15, 144, 65535} {
		input := base
		input.Slot.Size = size
		got, out, err := StepAttachedEffect(input, parent, world, lib)
		if err == nil || out != (AttachedEffectOutcome{}) || !reflect.DeepEqual(got, input) {
			t.Fatal("unsupported effect clear leaked mutations")
		}
	}
	base.Kind = AttachedEffectKind(255)
	got, out, err := StepAttachedEffect(base, parent, world, lib)
	if err == nil || out != (AttachedEffectOutcome{}) || !reflect.DeepEqual(got, base) {
		t.Fatal("unknown effect kind changed state")
	}
	base.Kind, base.Slot.Stage, base.Slot.Type = AttachedStunEffect, 0, 255
	got, out, err = StepAttachedEffect(base, parent, world, lib)
	if err == nil || out != (AttachedEffectOutcome{}) || !reflect.DeepEqual(got, base) {
		t.Fatal("unknown initialized status style changed state")
	}
	base.Slot.Stage, base.Slot.Type, base.Aux48, base.Delay, base.Index, base.Ticks = 2, 0, parent.ObjectID, 0, 255, 1
	base.Sequence = 0x15082
	parent.ActionKind, parent.StunTicks = 0, 1
	got, out, err = StepAttachedEffect(base, parent, world, AttachedEffectLibrary{})
	if err == nil || out != (AttachedEffectOutcome{}) || !reflect.DeepEqual(got, base) {
		t.Fatal("missing countdown program leaked changes")
	}
	parent.CharacterID = 255
	got, out, err = StepAttachedEffect(base, parent, world, lib)
	if err == nil || out != (AttachedEffectOutcome{}) || !reflect.DeepEqual(got, base) {
		t.Fatal("missing owner character offset leaked changes")
	}
}
