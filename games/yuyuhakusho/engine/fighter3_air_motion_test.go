package engine

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
)

type fighter3AirRaw struct {
	ObjectID uint16 `json:"object_id"`
	Family   string
	Pool     struct {
		Cursor uint16
		Bytes  string
	}
	Buffers      string
	Offsets      [4]uint16
	PendingSound uint8    `json:"pending_sound"`
	SoundQueue   [4]uint8 `json:"sound_queue"`
	RandomSeed   uint32   `json:"random_seed"`
	Parameters   JumpParameters
	Environment  ChuProjectileEnvironment
}

type fighter3AirCase struct {
	Kind, Name, Scenario string
	ObjectID             uint16 `json:"object_id"`
	Before, After        fighter3AirRaw
	Draw                 bool
	ClearedBytes         uint16                                `json:"cleared_bytes"`
	RecordKey            AnimationFrameKey                     `json:"record_key"`
	External             map[string]uint8                      `json:"external_input"`
	RNGCalls             int                                   `json:"rng_calls"`
	Clears               []struct{ PC, Address string }        `json:"external_queue_clears"`
	ExternalChanges      []struct{ Offset, Before, After int } `json:"external_changes"`
	Unexplained          []struct{ Offset, Before, After int } `json:"unexplained_changes"`
	Interrupts           []struct {
		Entry, Exit string
		Changes     []struct{ Offset, Before, After int }
	}
}

// This verification adapter has one canonical WorldObject per parent/child/FX.
// Unknown bytes are retained only for exact original-output comparison; the
// production rule API never reads or writes a RAM image.
type fighter3AirModel struct {
	State ChuWorldState
	Raw   fighter3AirRaw
}

func decodeFighter3AirModel(t *testing.T, raw fighter3AirRaw) fighter3AirModel {
	t.Helper()
	m := fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw}
	b := attackContactBytes(t, raw.Family, 1024)
	for i := 0; i < 4; i++ {
		id := raw.ObjectID + uint16(i)*256
		m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(b[i*256:(i+1)*256]))
	}
	if m.State.Objects[raw.ObjectID].Tag != 3 {
		t.Fatal("family is not original source3")
	}
	pool := attackContactBytes(t, raw.Pool.Bytes, 2048)
	for i := 0; i < 16; i++ {
		id := uint16(0x9400 + i*128)
		padded := append(append([]byte(nil), pool[i*128:(i+1)*128]...), make([]byte, 128)...)
		m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(padded))
	}
	buffers := attackContactBytes(t, raw.Buffers, 1024)
	for i := range m.State.Buffers {
		m.State.Buffers[i] = attackContactBuffer(t, raw.Offsets[i], hex.EncodeToString(buffers[i*256:(i+1)*256]))
	}
	m.State.PendingSound, m.State.SoundQueue, m.State.RandomSeed, m.State.EffectCursor = raw.PendingSound, raw.SoundQueue, raw.RandomSeed, raw.Pool.Cursor
	e := raw.Environment
	m.State.Clock.PauseFlags, m.State.Clock.SlowFlag, m.State.Clock.ScreenShake = e.PauseFlags, e.SlowFlag, uint16(e.Shake)
	m.State.Camera.Left, m.State.Camera.Right, m.State.Camera.X, m.State.Camera.Y = e.ArenaLeft, e.ArenaRight, e.CameraX, e.CameraY
	m.State.Match.AttributeMask = e.PaletteAttributes
	return m
}

func (m *fighter3AirModel) encode(t *testing.T, cleared map[uint16]uint16) fighter3AirRaw {
	t.Helper()
	raw := m.Raw
	family := attackContactBytes(t, raw.Family, 1024)
	for i := 0; i < 4; i++ {
		id := raw.ObjectID + uint16(i)*256
		previous := hex.EncodeToString(family[i*256 : (i+1)*256])
		encoded := encodeWorldObjectTest(t, m.State.Objects[id], previous, cleared[id])
		copy(family[i*256:], attackContactBytes(t, encoded, 256))
	}
	raw.Family = hex.EncodeToString(family)
	pool := attackContactBytes(t, raw.Pool.Bytes, 2048)
	for i := 0; i < 16; i++ {
		id := uint16(0x9400 + i*128)
		padded := append(append([]byte(nil), pool[i*128:(i+1)*128]...), make([]byte, 128)...)
		encoded := encodeWorldObjectTest(t, m.State.Objects[id], hex.EncodeToString(padded), 0)
		copy(pool[i*128:], attackContactBytes(t, encoded, 256)[:128])
	}
	raw.Pool.Bytes, raw.Pool.Cursor = hex.EncodeToString(pool), m.State.EffectCursor
	buffers := make([]byte, 1024)
	for group, buffer := range m.State.Buffers {
		raw.Offsets[group] = buffer.Offset
		for i, record := range buffer.Records {
			b := buffers[group*256+i*16:]
			binary.BigEndian.PutUint16(b, uint16(record.X))
			binary.BigEndian.PutUint16(b[4:], uint16(record.Y))
			b[8], b[9] = record.Width, record.Height
			binary.BigEndian.PutUint16(b[12:], record.Owner)
			for j, off := range []int{2, 6, 10, 14} {
				binary.BigEndian.PutUint16(b[off:], record.Preserved[j])
			}
		}
	}
	raw.Buffers, raw.PendingSound, raw.SoundQueue, raw.RandomSeed = hex.EncodeToString(buffers), m.State.PendingSound, m.State.SoundQueue, m.State.RandomSeed
	raw.Environment = ChuProjectileEnvironment{PauseFlags: m.State.Clock.PauseFlags, SlowFlag: m.State.Clock.SlowFlag, PaletteAttributes: m.State.Match.AttributeMask,
		ArenaLeft: m.State.Camera.Left, ArenaRight: m.State.Camera.Right, CameraX: m.State.Camera.X, CameraY: m.State.Camera.Y, Shake: int16(m.State.Clock.ScreenShake)}
	m.Raw = raw
	return raw
}

func (m *fighter3AirModel) step(t *testing.T, row fighter3AirCase, library ChuWorldLibrary) (bool, uint16) {
	t.Helper()
	id := row.ObjectID
	object := m.State.Objects[id]
	if row.Kind == "clock" {
		object.Body.Clock = StepAnimationClock(object.Body.Clock)
		m.State.Objects[id] = object
		return false, 0
	}
	if row.Kind == "record" {
		key := AnimationFrameKey{AnimationSet: object.AnimationSet, ShapeSet: object.ShapeSet, Pose: object.Body.Pose, Index: object.Body.Clock.FrameIndex}
		if key != row.RecordKey {
			t.Fatal("independent source3 timing key differs", key, row.RecordKey)
		}
		data, exists := library.Animations[key]
		if !exists {
			t.Fatal("source3 timing data absent", key)
		}
		object.Body.Clock.Duration, object.Body.Clock.Flags = data.Duration, data.Flags
		m.State.Objects[id] = object
		return false, 0
	}
	if row.Kind == "air" || row.Kind == "charge_motion" {
		pool, err := BuildWorldEffectPool(m.State.Objects, m.State.EffectCursor)
		if err != nil {
			t.Fatal(err)
		}
		s := ChuSpecialMotionState{ObjectID: id, Body: object.Body, Controls: object.Controls, Energy: object.Energy, Power: object.Power,
			AlternateVelocity: object.Aux48, ContactFlags: object.ContactFlags, TargetBit: object.TargetBit, Pool: pool}
		for i := range s.Children {
			s.Children[i] = m.State.Objects[id+uint16(i+1)*256].AsChuChild()
		}
		if row.Kind == "air" {
			s, err = StepFighter3AirMotion(s, m.Raw.Parameters)
		} else {
			view := Fighter0MotionState{ChuSpecialMotionState: s, PendingSound: m.State.PendingSound}
			for i := range view.EffectExtras {
				view.EffectExtras[i] = m.State.Objects[0x9400+uint16(i)*128].AsReactionExtra()
			}
			view, err = StepFighter0Motion(view, m.Raw.Parameters)
			s, m.State.PendingSound = view.ChuSpecialMotionState, view.PendingSound
			for i, extra := range view.EffectExtras {
				fxID := 0x9400 + uint16(i)*128
				m.State.Objects[fxID] = m.State.Objects[fxID].MergeReactionExtra(extra)
			}
		}
		if err != nil {
			t.Fatal(row.Name, err)
		}
		object.Body, object.Controls, object.Energy, object.Power = s.Body, s.Controls, s.Energy, s.Power
		object.Aux48, object.ContactFlags, object.TargetBit = s.AlternateVelocity, s.ContactFlags, s.TargetBit
		m.State.Objects[id] = object
		for i, child := range s.Children {
			childID := id + uint16(i+1)*256
			m.State.Objects[childID] = m.State.Objects[childID].MergeChuChild(child)
		}
		m.State.Objects, err = MergeWorldEffectPool(m.State.Objects, s.Pool)
		if err != nil {
			t.Fatal(err)
		}
		m.State.EffectCursor = s.Pool.Cursor
		return false, 0
	}
	var events []ChuWorldEvent
	var err error
	switch row.Kind {
	case "main":
		events, err = stepChuWorldProjectile(&m.State, object, library.Animations)
	case "charge":
		events, err = stepChuWorldAttachment(&m.State, object, library.Attachments)
	case "release":
		events, err = stepFighter0WorldProjectile(&m.State, object, library.Fighter0Projectiles)
	default:
		t.Fatal("unknown source3 family callback", row.Kind)
	}
	if err != nil {
		t.Fatal(row.Kind, row.Name, err)
	}
	var draw bool
	var clear uint16
	for _, event := range events {
		if event.Kind == "draw" {
			draw = true
		} else if event.Kind == "clear" {
			clear = event.ClearedBytes
		} else {
			t.Fatal("unexpected family stage event", event.Kind)
		}
	}
	return draw, clear
}

func compareFighter3AirRaw(t *testing.T, got, want fighter3AirRaw) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	for name, values := range map[string][2]string{"family": {got.Family, want.Family}, "pool": {got.Pool.Bytes, want.Pool.Bytes}, "buffers": {got.Buffers, want.Buffers}} {
		a, _ := hex.DecodeString(values[0])
		b, _ := hex.DecodeString(values[1])
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("source3 %s byte%x got%x want%x", name, i, a[i], b[i])
			}
		}
	}
	t.Fatalf("source3 shared metadata differs got=%+v want=%+v", got.Environment, want.Environment)
}

func fighter3AirLibrary(t *testing.T, frames AnimationLibrary) ChuWorldLibrary {
	t.Helper()
	library := chuWorldLibrariesTest(t)
	for key, data := range frames {
		if reference, exists := library.Animations[key]; !exists || !reflect.DeepEqual(data, reference) {
			t.Fatal("source3 captured immutable record differs from existing semantic data", key)
		}
	}
	library.Animations, library.Attachments.Animations, library.Fighter0Projectiles.Animations = frames, frames, frames
	return library
}

// Original CPU family data and these assertions precede the production file.
func TestFighter3AirMotionOriginalContexts(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen := map[string]int{}
		irqCalls := 0
		var library ChuWorldLibrary
		count, external := walkChuActor(t, "fighter3-air-motion-"+suffix, func(d *json.Decoder, frames AnimationLibrary) {
			if library.Animations == nil {
				library = fighter3AirLibrary(t, frames)
			}
			var row fighter3AirCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if row.RNGCalls != 0 || len(row.Clears) != 0 || len(row.Unexplained) != 0 {
				t.Fatal("unexplained source3 rule/audio mutation")
			}
			for _, change := range row.ExternalChanges {
				found := false
				value := change.Before
				for _, irq := range row.Interrupts {
					if irq.Entry != "0xfff000" || (irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a") {
						t.Fatal("unrecognized RAM IRQ boundary")
					}
					for _, observed := range irq.Changes {
						if observed.Offset == change.Offset {
							if observed.Before != value {
								t.Fatal("observed IRQ write chain is inconsistent", change, observed)
							}
							value, found = observed.After, true
						}
					}
				}
				if !found || value != change.After {
					t.Fatal("external byte is not attributed to observed IRQ", change)
				}
			}
			irqCalls += len(row.Interrupts)
			model := decodeFighter3AirModel(t, row.Before)
			draw, clear := model.step(t, row, library)
			if draw != row.Draw || clear != row.ClearedBytes {
				t.Fatal("source3 draw/clear outcome differs", row.Kind, row.Name)
			}
			compareFighter3AirRaw(t, model.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
			seen[row.Kind]++
		})
		if external != 0 || (suffix == "oracle" && (count != 175 || seen["air"] != 19 || seen["main"] != 77 || seen["charge"] != 48 || seen["release"] != 31 || irqCalls == 0)) ||
			(suffix == "boundaries" && (count != 4908 || seen["air"] != 4764 || irqCalls != 0)) {
			t.Fatal("missing source3 complete context coverage", suffix, count, seen, irqCalls)
		}
		t.Logf("source3 complete %s calls=%d kinds=%v observedIRQs=%d", suffix, count, seen, irqCalls)
	}
}

func TestFighter3AirMotionContinuousFamilies(t *testing.T) {
	steps := 0
	var library ChuWorldLibrary
	count, external := walkChuActor(t, "fighter3-air-motion-sequences", func(d *json.Decoder, frames AnimationLibrary) {
		if library.Animations == nil {
			library = fighter3AirLibrary(t, frames)
		}
		var stream struct {
			Name, Mode string
			Initial    fighter3AirRaw
			Calls      []fighter3AirCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("missing source3 continuous family", err)
		}
		model := decodeFighter3AirModel(t, stream.Initial)
		for _, row := range stream.Calls {
			for key, value := range row.External {
				if key != "held" {
					t.Fatal("undeclared family external input", key)
				}
				object := model.State.Objects[stream.Initial.ObjectID]
				object.Controls.Held = object.Controls.Held&0xff00 | Buttons(value)
				model.State.Objects[object.ObjectID] = object
			}
			compareFighter3AirRaw(t, model.encode(t, nil), row.Before)
			draw, clear := model.step(t, row, library)
			if draw != row.Draw || clear != row.ClearedBytes || row.RNGCalls != 0 || len(row.Clears) != 0 || len(row.ExternalChanges) != 0 {
				t.Fatal("unexpected continuous source3 outcome or foreign execution", stream.Name, row.Name)
			}
			compareFighter3AirRaw(t, model.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
			steps++
		}
		parent := model.State.Objects[stream.Initial.ObjectID]
		if stream.Mode == "air" && parent.Body.ActionKind != 2 || stream.Mode == "ground" && parent.Body.ActionKind != 0 {
			t.Fatal("source3 parent did not recover", stream.Name)
		}
		for i := 1; i <= 3; i++ {
			if model.State.Objects[stream.Initial.ObjectID+uint16(i)*256].Header.Size != 0 {
				t.Fatal("source3 child did not actually delete", stream.Name, i)
			}
		}
	})
	if count != 24 || steps != 6272 || external != 0 {
		t.Fatal("missing source3 parent/child continuous coverage", count, steps, external)
	}
}

func TestFighter3AirMotionRejectsOnlyUnsupportedStage(t *testing.T) {
	base := ChuSpecialMotionState{Power: -32768, AlternateVelocity: -0x80000000, Pool: EffectPool{Cursor: 1}}
	got, err := StepFighter3AirMotion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported source3 stage did not reject atomically")
	}
	base.Body.ActionKind, base.Body.Clock.FrameIndex = 0x22, 2
	base.Body.Clock.Ticks = 1
	got, err = StepFighter3AirMotion(base, JumpParameters{})
	if err != nil || got != base {
		t.Fatal("unread data or unrelated FX cursor rejected a valid no-op")
	}
}
