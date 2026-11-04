package engine

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Reuse the exact family-byte verification schema, without changing its
// source tag. Runtime state continues to have one canonical object per slot.
func decodeFighter2ProjectileModel(t *testing.T, raw fighter3AirRaw) fighter3AirModel {
	t.Helper()
	m := fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw}
	family := attackContactBytes(t, raw.Family, 1024)
	for i := 0; i < 4; i++ {
		id := raw.ObjectID + uint16(i)*256
		m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(family[i*256:(i+1)*256]))
	}
	if m.State.Objects[raw.ObjectID].Tag != 2 {
		t.Fatal("projectile creator is not original source2")
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

func stepFighter2ProjectileModel(t *testing.T, m *fighter3AirModel, row fighter3AirCase, library AnimationLibrary) (bool, uint16) {
	t.Helper()
	id := row.ObjectID
	object := m.State.Objects[id]
	pool, err := BuildWorldEffectPool(m.State.Objects, m.State.EffectCursor)
	if err != nil {
		t.Fatal(err)
	}
	if row.Kind == "creator_beam" || row.Kind == "creator_scatter" {
		s := ChuSpecialMotionState{ObjectID: id, Body: object.Body, Controls: object.Controls, Energy: object.Energy, Power: object.Power,
			AlternateVelocity: object.Aux48, ContactFlags: object.ContactFlags, TargetBit: object.TargetBit, Pool: pool}
		for i := range s.Children {
			s.Children[i] = m.State.Objects[id+uint16(i+1)*256].AsChuChild()
		}
		s, err = StepFighter2Motion(s, m.Raw.Parameters)
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
	s := object.AsChuProjectile()
	s.Pool, s.Buffers, s.PendingSound, s.SoundQueue = pool, m.State.Buffers, m.State.PendingSound, m.State.SoundQueue
	var next ChuProjectileState
	var outcome ChuProjectileOutcome
	hitstop := object.HitStopTicks
	if row.Kind == "beam" {
		var updated Fighter2ProjectileState
		updated, outcome, err = StepFighter2Projectile(Fighter2ProjectileState{Projectile: s, HitStopTicks: hitstop}, m.Raw.Environment, library)
		next, hitstop = updated.Projectile, updated.HitStopTicks
	} else if row.Kind == "scatter" {
		var updated Fighter0ProjectileState
		updated, outcome, err = StepFighter0Projectile(Fighter0ProjectileState{Kind: Fighter0ScatterProjectile, Projectile: s}, m.Raw.Environment, Fighter0ProjectileLibrary{Animations: library})
		next = updated.Projectile
	} else {
		t.Fatal("unsupported source2 projectile verification kind", row.Kind)
	}
	if err != nil {
		t.Fatal(row.Name, err)
	}
	m.State.Objects, err = MergeWorldEffectPool(m.State.Objects, next.Pool)
	if err != nil {
		t.Fatal(err)
	}
	object, err = ApplyObjectClearPrefix(object, outcome.ClearedBytes)
	if err != nil {
		t.Fatal(err)
	}
	object, err = object.MergeChuProjectile(next)
	if err != nil {
		t.Fatal(err)
	}
	if row.Kind == "beam" {
		object.HitStopTicks = hitstop
	}
	m.State.Objects[id] = object
	m.State.EffectCursor, m.State.Buffers, m.State.PendingSound, m.State.SoundQueue = next.Pool.Cursor, next.Buffers, next.PendingSound, next.SoundQueue
	return outcome.Draw, outcome.ClearedBytes
}

// Full original CPU material and these tests precede fighter2_projectile.go.
func TestFighter2ProjectileOriginalCallbacks(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen := map[string]int{}
		count, external := walkChuActor(t, "fighter2-projectile-"+suffix, func(d *json.Decoder, library AnimationLibrary) {
			var row fighter3AirCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			if row.RNGCalls != 0 || len(row.Clears) != 0 || len(row.Unexplained) != 0 {
				t.Fatal("unexplained source2 global mutation")
			}
			for _, change := range row.ExternalChanges {
				value, found := change.Before, false
				for _, irq := range row.Interrupts {
					if irq.Entry != "0xfff000" || (irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a") {
						t.Fatal("unrecognized source2 IRQ attribution")
					}
					for _, delta := range irq.Changes {
						if delta.Offset == change.Offset {
							if value != delta.Before {
								t.Fatal("IRQ write chain differs", change, delta)
							}
							value, found = delta.After, true
						}
					}
				}
				if !found || value != change.After {
					t.Fatal("unattributed external source2 byte", change)
				}
			}
			m := decodeFighter2ProjectileModel(t, row.Before)
			draw, clear := stepFighter2ProjectileModel(t, &m, row, library)
			if draw != row.Draw || clear != row.ClearedBytes {
				t.Fatal("source2 draw/clear boundary differs", row.Name)
			}
			compareFighter3AirRaw(t, m.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
			seen[row.Kind]++
		})
		if external != 0 || (suffix == "oracle" && (count != 124 || seen["beam"] != 73 || seen["scatter"] != 51)) || (suffix == "boundaries" && count != 1724) {
			t.Fatal("missing original source2 complete callbacks", suffix, count, seen, external)
		}
		t.Logf("source2 complete %s calls=%d kinds=%v", suffix, count, seen)
	}
}

func TestFighter2ProjectileCreatorLifecycles(t *testing.T) {
	steps, creators := 0, 0
	count, external := walkChuActor(t, "fighter2-projectile-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name    string
			Creator *fighter3AirCase
			Initial fighter3AirRaw
			Calls   []fighter3AirCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("incomplete original source2 lifecycle", err)
		}
		m := decodeFighter2ProjectileModel(t, stream.Initial)
		if stream.Creator != nil {
			m = decodeFighter2ProjectileModel(t, stream.Creator.Before)
			stepFighter2ProjectileModel(t, &m, *stream.Creator, library)
			compareFighter3AirRaw(t, m.encode(t, nil), stream.Initial)
			creators++
		}
		for _, row := range stream.Calls {
			for key, value := range row.External {
				object := m.State.Objects[row.ObjectID]
				switch key {
				case "hitstop":
					object.HitStopTicks = value
				case "shake":
					object.Shake = value
				case "pause_flags":
					m.State.Clock.PauseFlags = value
				default:
					t.Fatal("undeclared source2 lifecycle input", key)
				}
				m.State.Objects[object.ObjectID] = object
			}
			compareFighter3AirRaw(t, m.encode(t, nil), row.Before)
			draw, clear := stepFighter2ProjectileModel(t, &m, row, library)
			if draw != row.Draw || clear != row.ClearedBytes || row.RNGCalls != 0 || len(row.Clears) != 0 || len(row.ExternalChanges) != 0 {
				t.Fatal("source2 continuous outcome differs", stream.Name, row.Name)
			}
			compareFighter3AirRaw(t, m.encode(t, map[uint16]uint16{row.ObjectID: clear}), row.After)
			steps++
		}
		last := stream.Calls[len(stream.Calls)-1]
		if last.ClearedBytes == 0 || m.State.Objects[last.ObjectID].Header.Size != 0 {
			t.Fatal("source2 child lifecycle did not reach deletion")
		}
	})
	if count != 24 || creators != 16 || steps < 500 || external != 0 {
		t.Fatal("missing source2 creator/lifetime coverage", count, creators, steps, external)
	}
	t.Logf("source2 continuous lifecycles=%d creators=%d callbackSteps=%d", count, creators, steps)
}

func TestFighter2ProjectileSemanticRecords(t *testing.T) {
	data, err := os.ReadFile("../data/fighter2-projectile-rules.json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Animations []struct {
			Key  AnimationFrameKey
			Data AnimationRecordData
		}
	}
	if err := json.Unmarshal(data, &decoded); err != nil || len(decoded.Animations) != 23 {
		t.Fatal("missing source2 semantic records", err)
	}
	semantic := make(AnimationLibrary)
	counts := make(map[uint8]int)
	for _, frame := range decoded.Animations {
		if _, exists := semantic[frame.Key]; exists {
			t.Fatal("duplicate source2 semantic record")
		}
		semantic[frame.Key] = frame.Data
		counts[frame.Key.Pose]++
	}
	if counts[9] != 10 || counts[12] != 5 || counts[13] != 8 {
		t.Fatal("incomplete source2 public poses", counts)
	}
	called := false
	walkChuActor(t, "fighter2-projectile-boundaries", func(d *json.Decoder, original AnimationLibrary) {
		var row json.RawMessage
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if !called && !reflect.DeepEqual(semantic, original) {
			t.Fatal("runtime semantic data differs from CPU-tested records")
		}
		called = true
	})
	if !called {
		t.Fatal("no CPU semantic record evidence")
	}
}

func TestFighter2ProjectileErrorsAndPausedInit(t *testing.T) {
	base := Fighter2ProjectileState{Projectile: ChuProjectileState{Behavior: 0x12dd4, Size: 256, Stage: 0,
		Animation: AnimationActorState{ObjectID: 0x8500, Pose: 9, WorldX: -1, WorldY: 260 << 16,
			Clock: AnimationClock{FrameIndex: 0, Duration: 4, Ticks: 0}}, ScreenOffsetX: 7}}
	env := ChuProjectileEnvironment{ArenaLeft: 10, ArenaRight: 600}
	check := func(name string, input Fighter2ProjectileState) {
		t.Helper()
		got, outcome, err := StepFighter2Projectile(input, env, nil)
		if err == nil || !reflect.DeepEqual(got, input) || outcome != (ChuProjectileOutcome{}) {
			t.Fatal(name, "did not reject atomically", err, outcome)
		}
	}
	check("missing frame after init and paired allocation", base)
	base.Projectile.Pool.Cursor = 1
	check("invalid dust cursor", base)
	base.Projectile.Stage, base.Projectile.Size, base.Projectile.Animation.Clock.Completed = 2, 1, 1
	base.Projectile.Pool.Cursor = 0
	check("invalid clear prefix", base)
	base.Projectile.Size, base.HitStopTicks, base.Projectile.Shake = 256, 1, 5
	check("missing frame after hit-stop decrement and jitter", base)
	base.Projectile.Stage, base.Projectile.Animation.Clock.FrameIndex, base.Projectile.Animation.Clock.Flags = 0, 2, 0xaa
	base.HitStopTicks = 4
	env.PauseFlags = 128
	got, out, err := StepFighter2Projectile(base, env, nil)
	if err != nil || out != (ChuProjectileOutcome{Draw: true}) || got.Projectile.Stage != 2 || got.Projectile.Health != 255 || got.HitStopTicks != 4 ||
		got.Projectile.Animation.Clock.FrameIndex != 2 || got.Projectile.Animation.Clock.Flags != 0xaa || got.Projectile.Animation.Clock.Completed != 1 ||
		int16(got.Projectile.Animation.WorldX>>16) != 10 || uint16(got.Projectile.Animation.WorldX) != 65535 || got.Projectile.ScreenOffsetX != 7 {
		t.Fatal("paused init lost owned/retained fields or skipped clamp", err, out)
	}
}
