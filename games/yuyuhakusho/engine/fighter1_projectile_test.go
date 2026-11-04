package engine

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func decodeFighter1ProjectileModel(t *testing.T, raw fighter3AirRaw) fighter3AirModel {
	t.Helper()
	m := fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw}
	family := attackContactBytes(t, raw.Family, 1024)
	for i := 0; i < 4; i++ {
		id := raw.ObjectID + uint16(i)*256
		m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(family[i*256:(i+1)*256]))
	}
	if m.State.Objects[raw.ObjectID].Tag != 1 {
		t.Fatal("not a real source1 family")
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

func fighter1WaveTable(t *testing.T) []int8 {
	t.Helper()
	raw, err := os.ReadFile("../data/arena-runtime-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Library struct {
			WaveX []int8 `json:"wave_x"`
		}
	}
	if err := json.Unmarshal(raw, &d); err != nil || len(d.Library.WaveX) != 1024 {
		t.Fatal("missing already recovered wave table", err)
	}
	return d.Library.WaveX
}

func fighter1VisualView(object WorldObject, state ChuWorldState) Fighter8ProjectileState {
	return Fighter8ProjectileState{Header: object.Header, Animation: object.AsAnimation(), GraphicsSet: object.GraphicsSet, TilesBase: object.TilesBase, GraphicCache: object.GraphicCache,
		Health: object.Body.Health, Shake: object.Shake, ScreenX: object.ScreenX, ScreenY: object.ScreenY, ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY,
		Buffers: state.Buffers, PendingSound: state.PendingSound, SoundQueue: state.SoundQueue}
}

func mergeFighter1Visual(t *testing.T, object WorldObject, next Fighter8ProjectileState, clear uint16) WorldObject {
	t.Helper()
	var err error
	object, err = ApplyObjectClearPrefix(object, clear)
	if err != nil {
		t.Fatal(err)
	}
	object, err = object.MergeAnimation(next.Animation)
	if err != nil {
		t.Fatal(err)
	}
	object.Header, object.GraphicsSet, object.TilesBase, object.GraphicCache = next.Header, next.GraphicsSet, next.TilesBase, next.GraphicCache
	object.Body.Health, object.Shake = next.Health, next.Shake
	object.ScreenX, object.ScreenY, object.ScreenOffsetX, object.ScreenOffsetY = next.ScreenX, next.ScreenY, next.ScreenOffsetX, next.ScreenOffsetY
	return object
}

func stepFighter1ProjectileModel(t *testing.T, m *fighter3AirModel, row fighter3AirCase, library AnimationLibrary, wave []int8) (bool, uint16) {
	t.Helper()
	if row.Kind == "clock" || row.Kind == "record" {
		return m.step(t, row, ChuWorldLibrary{Animations: library})
	}
	if row.Kind == "beam" {
		return stepFighter2ProjectileModel(t, m, row, library)
	}
	id := row.ObjectID
	object := m.State.Objects[id]
	if row.Kind == "creator_beam" || row.Kind == "creator_linear" || row.Kind == "creator_wave" {
		pool, err := BuildWorldEffectPool(m.State.Objects, m.State.EffectCursor)
		if err != nil {
			t.Fatal(err)
		}
		s := Fighter1MotionState{ChuSpecialMotionState: ChuSpecialMotionState{ObjectID: id, Body: object.Body, Controls: object.Controls, Energy: object.Energy, Power: object.Power, AlternateVelocity: object.Aux48, ContactFlags: object.ContactFlags, TargetBit: object.TargetBit, Pool: pool}}
		for i := range s.Children {
			s.Children[i] = m.State.Objects[id+uint16(i+1)*256].AsChuChild()
		}
		for i := range s.EffectExtras {
			s.EffectExtras[i] = m.State.Objects[0x9400+uint16(i)*128].AsReactionExtra()
		}
		s, err = StepFighter1Motion(s, m.Raw.Parameters)
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
		for i, extra := range s.EffectExtras {
			fxID := 0x9400 + uint16(i)*128
			m.State.Objects[fxID] = m.State.Objects[fxID].MergeReactionExtra(extra)
		}
		m.State.EffectCursor = s.Pool.Cursor
		return false, 0
	}
	visual := fighter1VisualView(object, m.State)
	e := m.Raw.Environment
	var next Fighter8ProjectileState
	var outcome ChuProjectileOutcome
	if row.Kind == "linear" {
		s, out, err := StepFighter1LinearProjectile(Fighter1LinearProjectileState{Visual: visual, VelocityX: object.Body.VelocityX, VelocityY: object.Body.VerticalVelocity, Ground: object.Body.Ground, ContactFlags: object.ContactFlags}, e, library)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		next, outcome = s.Visual, out
		object = mergeFighter1Visual(t, object, next, out.ClearedBytes)
		object.Body.VelocityX, object.Body.VerticalVelocity, object.Body.Ground, object.ContactFlags = s.VelocityX, s.VelocityY, s.Ground, s.ContactFlags
	} else if row.Kind == "wave" {
		s, out, err := StepFighter1WaveProjectile(Fighter1WaveProjectileState{Visual: visual, OriginX: object.Body.VelocityX, VelocityY: object.Body.VerticalVelocity, Phase: uint16(uint32(object.Aux48) >> 16), Remaining: uint16(uint32(object.Body.Gravity) >> 16), ContactFlags: object.ContactFlags}, ChuAttachmentWorld{PauseFlags: e.PauseFlags, SlowFlag: e.SlowFlag, AttributeMask: e.PaletteAttributes, CameraX: e.CameraX, CameraY: e.CameraY, Shake: e.Shake}, library, wave)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		next, outcome = s.Visual, out
		object = mergeFighter1Visual(t, object, next, out.ClearedBytes)
		object.Body.VelocityX, object.Body.VerticalVelocity, object.ContactFlags = s.OriginX, s.VelocityY, s.ContactFlags
		object.Aux48 = worldObjectHighWord(object.Aux48, s.Phase)
		object.Body.Gravity = worldObjectHighWord(object.Body.Gravity, s.Remaining)
	} else {
		t.Fatal("unknown source1 projectile stage", row.Kind)
	}
	m.State.Objects[id] = object
	m.State.Buffers, m.State.PendingSound, m.State.SoundQueue = next.Buffers, next.PendingSound, next.SoundQueue
	return outcome.Draw, outcome.ClearedBytes
}

func verifyFighter1ProjectileCases(t *testing.T, name string) (int, map[string]int) {
	t.Helper()
	wave := fighter1WaveTable(t)
	seen := map[string]int{}
	count, external := walkChuActor(t, name, func(d *json.Decoder, library AnimationLibrary) {
		var row fighter3AirCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if row.RNGCalls != 0 || len(row.Unexplained) != 0 || len(row.Clears) != 0 {
			t.Fatal("unexplained source1 callback mutation")
		}
		for _, change := range row.ExternalChanges {
			value, found := change.Before, false
			for _, irq := range row.Interrupts {
				if irq.Entry != "0xfff000" || (irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a") {
					t.Fatal("unidentified IRQ")
				}
				for _, delta := range irq.Changes {
					if delta.Offset == change.Offset {
						if delta.Before != value {
							t.Fatal("IRQ byte chain differs")
						}
						value, found = delta.After, true
					}
				}
			}
			if !found || value != change.After {
				t.Fatal("unattributed outside byte")
			}
		}
		m := decodeFighter1ProjectileModel(t, row.Before)
		draw, clear := stepFighter1ProjectileModel(t, &m, row, library, wave)
		if draw != row.Draw || clear != row.ClearedBytes {
			t.Fatal("source1 outcome differs", row.Name)
		}
		compareFighter3AirRaw(t, m.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
		seen[row.Kind]++
	})
	if external != 0 {
		t.Fatal("source1 independent audio consumer not covered")
	}
	return count, seen
}

// All callback fixtures, failure scenarios and tests precede the new Go file.
func TestFighter1ProjectileOriginalBoundaries(t *testing.T) {
	count, seen := verifyFighter1ProjectileCases(t, "fighter1-projectile-boundaries")
	if count != 4230 || seen["linear"] != 1535 || seen["wave"] != 2695 {
		t.Fatal("missing source1 original boundary coverage", count, seen)
	}
}

func TestFighter1ProjectileNaturalCallbacks(t *testing.T) {
	count, seen := verifyFighter1ProjectileCases(t, "fighter1-projectile-special-oracle")
	if count == 0 || seen["linear"] == 0 || seen["wave"] == 0 || seen["beam"] == 0 {
		t.Fatal("missing actual source1 natural child kinds", count, seen)
	}
	t.Logf("source1 full natural callbacks=%d kinds=%v", count, seen)
}

func TestFighter1ProjectileCreatorLifecycles(t *testing.T) {
	wave := fighter1WaveTable(t)
	steps := 0
	count, external := walkChuActor(t, "fighter1-projectile-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name    string
			Initial fighter3AirRaw
			Calls   []fighter3AirCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("missing source1 creator/life flow", err)
		}
		m := decodeFighter1ProjectileModel(t, stream.Initial)
		for _, row := range stream.Calls {
			if len(row.External) != 0 || row.RNGCalls != 0 || len(row.Clears) != 0 || len(row.ExternalChanges) != 0 {
				t.Fatal("unexpected source1 flow injection")
			}
			compareFighter3AirRaw(t, m.encode(t, nil), row.Before)
			draw, clear := stepFighter1ProjectileModel(t, &m, row, library, wave)
			if draw != row.Draw || clear != row.ClearedBytes {
				t.Fatal("source1 flow outcome differs", stream.Name, row.Name)
			}
			compareFighter3AirRaw(t, m.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
			steps++
		}
		if m.State.Objects[stream.Initial.ObjectID].Body.ActionKind != 0 {
			t.Fatal("source1 parent did not recover")
		}
		for i := 1; i <= 3; i++ {
			if m.State.Objects[stream.Initial.ObjectID+uint16(i)*256].Header.Size != 0 {
				t.Fatal("source1 child still active", stream.Name, i)
			}
		}
	})
	if count != 16 || steps != 4588 || external != 0 {
		t.Fatal("incomplete source1 family stages", count, steps, external)
	}
}

func TestFighter1ProjectileSemanticRecords(t *testing.T) {
	raw, err := os.ReadFile("../data/fighter1-projectile-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Animations []struct {
			Key  AnimationFrameKey
			Data AnimationRecordData
		}
	}
	if err := json.Unmarshal(raw, &data); err != nil || len(data.Animations) != 8 {
		t.Fatal("missing eight source1 public records", err)
	}
	missing := make(AnimationLibrary)
	counts := map[uint8]int{}
	for _, row := range data.Animations {
		if _, ok := missing[row.Key]; ok {
			t.Fatal("duplicate source1 public record")
		}
		missing[row.Key] = row.Data
		counts[row.Key.Pose]++
	}
	if counts[10] != 2 || counts[11] != 6 {
		t.Fatal("wrong source1 public pose coverage", counts)
	}
	existing := chuWorldLibrariesTest(t).Animations
	checked := false
	walkChuActor(t, "fighter1-projectile-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var row json.RawMessage
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if checked {
			return
		}
		checked = true
		for key, record := range library {
			if key.AnimationSet == 0xd92ca && (key.Pose == 10 || key.Pose == 11) {
				if !reflect.DeepEqual(record, missing[key]) {
					t.Fatal("new public record differs from CPU", key)
				}
			} else if !reflect.DeepEqual(record, existing[key]) {
				t.Fatal("existing record context differs", key)
			}
		}
	})
	if !checked {
		t.Fatal("no source1 CPU record evidence")
	}
}

func TestFighter1ProjectileErrorsAndPausedInit(t *testing.T) {
	visual := Fighter8ProjectileState{Header: MatchObjectHeader{Behavior: 0x12ee8, Size: 256}, Animation: AnimationActorState{ObjectID: 0x8500, Pose: 99, WorldX: 100 << 16, WorldY: 260 << 16}, Health: -7, Shake: 5, ScreenOffsetX: 7}
	linear := Fighter1LinearProjectileState{Visual: visual, VelocityX: 0x60000, VelocityY: -1, Ground: 320 << 16}
	got, out, err := StepFighter1LinearProjectile(linear, ChuProjectileEnvironment{ArenaRight: 600}, nil)
	if err == nil || !reflect.DeepEqual(got, linear) || out != (ChuProjectileOutcome{}) {
		t.Fatal("linear missing data leaked init/motion")
	}
	linear.Visual.Header.Stage, linear.Visual.Header.Size, linear.ContactFlags = 2, 1, 8
	got, out, err = StepFighter1LinearProjectile(linear, ChuProjectileEnvironment{ArenaRight: 600}, nil)
	if err == nil || !reflect.DeepEqual(got, linear) || out != (ChuProjectileOutcome{}) {
		t.Fatal("linear late deletion did not rollback")
	}
	wave := Fighter1WaveProjectileState{Visual: visual, OriginX: 100<<16 | 0xabcd, VelocityY: -0x28000, Phase: 65535, Remaining: 0}
	wgot, out, err := StepFighter1WaveProjectile(wave, ChuAttachmentWorld{}, nil, nil)
	if err == nil || !reflect.DeepEqual(wgot, wave) || out != (ChuProjectileOutcome{}) {
		t.Fatal("missing wave table leaked init")
	}
	wgot, out, err = StepFighter1WaveProjectile(wave, ChuAttachmentWorld{}, nil, fighter1WaveTable(t))
	if err == nil || !reflect.DeepEqual(wgot, wave) || out != (ChuProjectileOutcome{}) {
		t.Fatal("late wave AN lookup leaked phase/motion")
	}
	wave.Visual.Header.Stage, wave.Visual.Header.Size, wave.Remaining = 2, 1, 1
	wgot, out, err = StepFighter1WaveProjectile(wave, ChuAttachmentWorld{}, nil, fighter1WaveTable(t))
	if err == nil || !reflect.DeepEqual(wgot, wave) || out != (ChuProjectileOutcome{}) {
		t.Fatal("wave late clear leaked output")
	}
	wave.Visual.Header.Stage, wave.Visual.Header.Size, wave.Visual.Animation.Clock.FrameIndex, wave.Visual.Animation.Clock.Flags, wave.Visual.Animation.Clock.Completed = 0, 256, 2, 0xaa, 1
	wgot, out, err = StepFighter1WaveProjectile(wave, ChuAttachmentWorld{PauseFlags: 128}, nil, nil)
	if err != nil || out != (ChuProjectileOutcome{Draw: true}) || wgot.Visual.Header.Stage != 2 || wgot.Visual.Health != 255 || wgot.Visual.Animation.Pose != 11 || wgot.Phase != 0x2c0 || wgot.Remaining != 24 || wgot.Visual.Animation.Clock.FrameIndex != 2 || wgot.Visual.Animation.Clock.Flags != 0xaa || wgot.Visual.Animation.Clock.Completed != 1 || wgot.Visual.Shake != 5 || wgot.Visual.ScreenOffsetX != 7 {
		t.Fatal("paused wave init violated fields", err, out)
	}
}
