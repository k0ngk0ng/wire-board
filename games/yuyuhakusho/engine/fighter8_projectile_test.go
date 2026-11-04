package engine

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
)

func decodeFighter8ProjectileModel(t *testing.T, raw fighter3AirRaw) fighter3AirModel {
	t.Helper()
	m := fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw}
	family := attackContactBytes(t, raw.Family, 1024)
	for i := 0; i < 4; i++ {
		id := raw.ObjectID + uint16(i)*256
		m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(family[i*256:(i+1)*256]))
	}
	if m.State.Objects[raw.ObjectID].Tag != 8 {
		t.Fatal("not an original source8 family")
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

func stepFighter8ProjectileModel(t *testing.T, m *fighter3AirModel, row fighter3AirCase, library AnimationLibrary) (bool, uint16) {
	t.Helper()
	if row.Kind == "clock" || row.Kind == "record" {
		return m.step(t, row, ChuWorldLibrary{Animations: library})
	}
	id := row.ObjectID
	object := m.State.Objects[id]
	if row.Kind == "creator" {
		pool, err := BuildWorldEffectPool(m.State.Objects, m.State.EffectCursor)
		if err != nil {
			t.Fatal(err)
		}
		view := ChuSpecialMotionState{ObjectID: id, Body: object.Body, Controls: object.Controls, Energy: object.Energy, Power: object.Power,
			AlternateVelocity: object.Aux48, ContactFlags: object.ContactFlags, TargetBit: object.TargetBit, Pool: pool}
		for i := range view.Children {
			view.Children[i] = m.State.Objects[id+uint16(i+1)*256].AsChuChild()
		}
		view, err = StepFighter8Motion(view, m.Raw.Parameters)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		object.Body, object.Controls, object.Energy, object.Power = view.Body, view.Controls, view.Energy, view.Power
		object.Aux48, object.ContactFlags, object.TargetBit = view.AlternateVelocity, view.ContactFlags, view.TargetBit
		m.State.Objects[id] = object
		for i, child := range view.Children {
			childID := id + uint16(i+1)*256
			m.State.Objects[childID] = m.State.Objects[childID].MergeChuChild(child)
		}
		m.State.Objects, err = MergeWorldEffectPool(m.State.Objects, view.Pool)
		if err != nil {
			t.Fatal(err)
		}
		m.State.EffectCursor = view.Pool.Cursor
		return false, 0
	}
	if row.Kind != "child" {
		t.Fatal("unknown source8 callback", row.Kind)
	}
	view := Fighter8ProjectileState{Header: object.Header, Animation: object.AsAnimation(), GraphicsSet: object.GraphicsSet, TilesBase: object.TilesBase,
		GraphicCache: object.GraphicCache, Health: object.Body.Health, Shake: object.Shake, ScreenX: object.ScreenX, ScreenY: object.ScreenY,
		ScreenOffsetX: object.ScreenOffsetX, ScreenOffsetY: object.ScreenOffsetY, Buffers: m.State.Buffers, PendingSound: m.State.PendingSound, SoundQueue: m.State.SoundQueue}
	e := m.Raw.Environment
	next, out, err := StepFighter8Projectile(view, ChuAttachmentWorld{PauseFlags: e.PauseFlags, SlowFlag: e.SlowFlag, AttributeMask: e.PaletteAttributes, CameraX: e.CameraX, CameraY: e.CameraY, Shake: e.Shake}, library)
	if err != nil {
		t.Fatal(row.Name, err)
	}
	object, err = ApplyObjectClearPrefix(object, out.ClearedBytes)
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
	m.State.Objects[id] = object
	m.State.Buffers, m.State.PendingSound, m.State.SoundQueue = next.Buffers, next.PendingSound, next.SoundQueue
	return out.Draw, out.ClearedBytes
}

func verifyFighter8ProjectileCases(t *testing.T, name string) (int, map[string]int) {
	t.Helper()
	seen := map[string]int{}
	count, external := walkChuActor(t, name, func(d *json.Decoder, library AnimationLibrary) {
		var row fighter3AirCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if row.RNGCalls != 0 || len(row.Unexplained) != 0 || len(row.Clears) != 0 {
			t.Fatal("unexplained source8 callback mutation")
		}
		for _, change := range row.ExternalChanges {
			value, found := change.Before, false
			for _, irq := range row.Interrupts {
				if irq.Entry != "0xfff000" || (irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a") {
					t.Fatal("unidentified source8 IRQ")
				}
				for _, delta := range irq.Changes {
					if delta.Offset == change.Offset {
						if delta.Before != value {
							t.Fatal("IRQ chain disagrees")
						}
						value, found = delta.After, true
					}
				}
			}
			if !found || value != change.After {
				t.Fatal("outside write not attributable", change)
			}
		}
		model := decodeFighter8ProjectileModel(t, row.Before)
		draw, clear := stepFighter8ProjectileModel(t, &model, row, library)
		if draw != row.Draw || clear != row.ClearedBytes {
			t.Fatal("source8 output boundary differs", row.Name)
		}
		compareFighter3AirRaw(t, model.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
		seen[row.Kind]++
	})
	if external != 0 {
		t.Fatal("source8 independent queue changes require explicit evidence")
	}
	return count, seen
}

// Original data and failure scenarios precede the new production rule.
func TestFighter8ProjectileOriginalBoundaries(t *testing.T) {
	count, seen := verifyFighter8ProjectileCases(t, "fighter8-projectile-boundaries")
	if count != 3279 || seen["child"] != 3135 || seen["creator"] != 144 {
		t.Fatal("missing source8 original contexts", count, seen)
	}
}

func TestFighter8ProjectileNaturalCallbacks(t *testing.T) {
	count, seen := verifyFighter8ProjectileCases(t, "fighter8-projectile-special-oracle")
	if count == 0 || seen["child"] != count {
		t.Fatal("no actual source8 natural callback evidence", count, seen)
	}
	t.Logf("source8 full natural147ae calls=%d", count)
}

func TestFighter8ProjectileCreatorLifecycles(t *testing.T) {
	steps := 0
	parentsMoved := 0
	count, external := walkChuActor(t, "fighter8-projectile-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name    string
			Initial fighter3AirRaw
			Calls   []fighter3AirCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("missing source8 flow", err)
		}
		model := decodeFighter8ProjectileModel(t, stream.Initial)
		for _, row := range stream.Calls {
			for key, value := range row.External {
				if key != "pause_flags" {
					t.Fatal("undeclared source8 external input")
				}
				model.State.Clock.PauseFlags = value
			}
			compareFighter3AirRaw(t, model.encode(t, nil), row.Before)
			parentBefore := model.State.Objects[stream.Initial.ObjectID]
			childBefore := model.State.Objects[stream.Initial.ObjectID+256]
			draw, clear := stepFighter8ProjectileModel(t, &model, row, library)
			if draw != row.Draw || clear != row.ClearedBytes || row.RNGCalls != 0 || len(row.Clears) != 0 || len(row.ExternalChanges) != 0 {
				t.Fatal("source8 flow outcome differs", stream.Name, row.Name)
			}
			compareFighter3AirRaw(t, model.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
			parentAfter := model.State.Objects[stream.Initial.ObjectID]
			childAfter := model.State.Objects[stream.Initial.ObjectID+256]
			if row.Kind == "creator" && parentBefore.Body.X != parentAfter.Body.X {
				if childBefore.Body.X != childAfter.Body.X || childBefore.Body.Facing != childAfter.Body.Facing {
					t.Fatal("source8 child followed parent")
				}
				parentsMoved++
			}
			steps++
		}
		if model.State.Objects[stream.Initial.ObjectID].Body.ActionKind != 0 || model.State.Objects[stream.Initial.ObjectID+256].Header.Size != 0 {
			t.Fatal("source8 flow did not recover/delete")
		}
	})
	if count != 24 || steps != 7788 || parentsMoved != 24 || external != 0 {
		t.Fatal("missing source8 clock/creator/lifetime coverage", count, steps, parentsMoved)
	}
}

func TestFighter8ProjectileExistingRecords(t *testing.T) {
	existing := chuWorldLibrariesTest(t).Animations
	called := false
	walkChuActor(t, "fighter8-projectile-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			t.Fatal(err)
		}
		if called {
			return
		}
		called = true
		child := 0
		for key, data := range library {
			if old, ok := existing[key]; !ok || !reflect.DeepEqual(data, old) {
				t.Fatal("source8 CPU semantic data differs from existing catalog", key)
			}
			if key.AnimationSet == 0xd1e5a && key.ShapeSet == 0xd2c6a && key.Pose == 0xa6 {
				child++
			}
		}
		if child != 4 {
			t.Fatal("source8 childa6 library incomplete", child)
		}
	})
	if !called {
		t.Fatal("no source8 record evidence")
	}
}

func TestFighter8ProjectileErrorsAndPausedInit(t *testing.T) {
	base := Fighter8ProjectileState{Header: MatchObjectHeader{Behavior: 0x147ae, Size: 256}, Animation: AnimationActorState{ObjectID: 0x8500, Pose: 0xa6, WorldX: 100 << 16, WorldY: 260 << 16}, Health: -7, Shake: 5, ScreenOffsetX: 7}
	env := ChuAttachmentWorld{}
	got, out, err := StepFighter8Projectile(base, env, nil)
	if err == nil || !reflect.DeepEqual(got, base) || out != (ChuProjectileOutcome{}) {
		t.Fatal("missing AN data leaked initialization")
	}
	base.Header.Stage, base.Header.Size, base.Animation.Clock.Completed = 2, 1, 1
	got, out, err = StepFighter8Projectile(base, env, nil)
	if err == nil || !reflect.DeepEqual(got, base) || out != (ChuProjectileOutcome{}) {
		t.Fatal("late clear error leaked clock/health/shake")
	}
	base.Header.Stage, base.Header.Size = 0, 256
	base.Animation.Clock.FrameIndex, base.Animation.Clock.Flags = 2, 0xaa
	env.PauseFlags = 128
	got, out, err = StepFighter8Projectile(base, env, nil)
	if err != nil || out != (ChuProjectileOutcome{Draw: true}) || got.Header.Stage != 2 || got.Health != 255 || got.Shake != 5 || got.Animation.Clock.FrameIndex != 2 || got.Animation.Clock.Flags != 0xaa || got.Animation.Clock.Completed != 1 || got.ScreenOffsetX != 7 || got.GraphicsSet != 0xd104a || got.TilesBase != 0x1e0000 || got.Animation.AnimationSet != 0xd1e5a || got.Animation.ShapeSet != 0xd2c6a {
		t.Fatal("source8 paused initializer lost retained/owned fields", err, out)
	}
}
