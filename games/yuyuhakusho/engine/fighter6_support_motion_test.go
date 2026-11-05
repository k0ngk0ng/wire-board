package engine

import (
	"encoding/hex"
	"encoding/json"
	"testing"
)

func decodeFighter6SupportModel(t *testing.T, raw fighter3AirRaw) fighter3AirModel {
	t.Helper()
	m := fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw}
	family := attackContactBytes(t, raw.Family, 1024)
	for i := 0; i < 4; i++ {
		id := raw.ObjectID + uint16(i)*256
		m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(family[i*256:(i+1)*256]))
	}
	if m.State.Objects[raw.ObjectID].Tag != 6 {
		t.Fatal("not a real source6 actor")
	}
	pool := attackContactBytes(t, raw.Pool.Bytes, 2048)
	for i := 0; i < 16; i++ {
		id := uint16(0x9400 + i*128)
		padded := append(append([]byte(nil), pool[i*128:(i+1)*128]...), make([]byte, 128)...)
		m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(padded))
	}
	buffer := attackContactBytes(t, raw.Buffers, 1024)
	for i := range m.State.Buffers {
		m.State.Buffers[i] = attackContactBuffer(t, raw.Offsets[i], hex.EncodeToString(buffer[i*256:(i+1)*256]))
	}
	m.State.PendingSound, m.State.SoundQueue, m.State.RandomSeed, m.State.EffectCursor = raw.PendingSound, raw.SoundQueue, raw.RandomSeed, raw.Pool.Cursor
	e := raw.Environment
	m.State.Clock.PauseFlags, m.State.Clock.SlowFlag, m.State.Clock.ScreenShake = e.PauseFlags, e.SlowFlag, uint16(e.Shake)
	m.State.Camera.Left, m.State.Camera.Right, m.State.Camera.X, m.State.Camera.Y = e.ArenaLeft, e.ArenaRight, e.CameraX, e.CameraY
	m.State.Match.AttributeMask = e.PaletteAttributes
	return m
}

func fighter6SupportView(t *testing.T, m *fighter3AirModel, id uint16) Fighter6MotionState {
	t.Helper()
	o := m.State.Objects[id]
	pool, err := BuildWorldEffectPool(m.State.Objects, m.State.EffectCursor)
	if err != nil {
		t.Fatal(err)
	}
	s := Fighter6MotionState{ChuSpecialMotionState: ChuSpecialMotionState{ObjectID: id, Body: o.Body, Controls: o.Controls, Energy: o.Energy, Power: o.Power, AlternateVelocity: o.Aux48, ContactFlags: o.ContactFlags, TargetBit: o.TargetBit, Pool: pool}, StunTicks: o.StunTicks}
	for i := range s.Children {
		s.Children[i] = m.State.Objects[id+uint16(i+1)*256].AsChuChild()
	}
	return s
}

func stepFighter6SupportModel(t *testing.T, m *fighter3AirModel, row fighter3AirCase, library AnimationLibrary) {
	t.Helper()
	if row.Kind == "clock" || row.Kind == "record" {
		m.step(t, row, ChuWorldLibrary{Animations: library})
		return
	}
	id := row.ObjectID
	o := m.State.Objects[id]
	if row.Kind == "exit" {
		enterFighter6Exit(&o.Body)
		m.State.Objects[id] = o
		return
	}
	s := fighter6SupportView(t, m, id)
	got, err := StepFighter6SupportMotion(s, m.Raw.Parameters)
	if err != nil {
		t.Fatal(row.Name, err)
	}
	o.Body, o.Controls, o.Energy, o.Power = got.Body, got.Controls, got.Energy, got.Power
	o.Aux48, o.ContactFlags, o.TargetBit, o.StunTicks = got.AlternateVelocity, got.ContactFlags, got.TargetBit, got.StunTicks
	m.State.Objects[id] = o
	for i, child := range got.Children {
		childID := id + uint16(i+1)*256
		m.State.Objects[childID] = m.State.Objects[childID].MergeChuChild(child)
	}
	m.State.Objects, err = MergeWorldEffectPool(m.State.Objects, got.Pool)
	if err != nil {
		t.Fatal(err)
	}
	m.State.EffectCursor = got.Pool.Cursor
}

func verifyFighter6SupportCases(t *testing.T, name string) (int, map[string]int) {
	t.Helper()
	seen := map[string]int{}
	count, external := walkChuActor(t, name, func(d *json.Decoder, library AnimationLibrary) {
		var row fighter3AirCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if row.RNGCalls != 0 || len(row.Unexplained) != 0 || len(row.Clears) != 0 {
			t.Fatal("unexplained source6 support mutation")
		}
		for _, change := range row.ExternalChanges {
			value, found := change.Before, false
			for _, irq := range row.Interrupts {
				if irq.Entry != "0xfff000" || (irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a") {
					t.Fatal("unidentified IRQ")
				}
				for _, delta := range irq.Changes {
					if delta.Offset == change.Offset {
						if value != delta.Before {
							t.Fatal("IRQ chain differs")
						}
						value, found = delta.After, true
					}
				}
			}
			if !found || value != change.After {
				t.Fatal("unattributed outside byte")
			}
		}
		m := decodeFighter6SupportModel(t, row.Before)
		stepFighter6SupportModel(t, &m, row, library)
		compareFighter3AirRaw(t, m.encode(t, nil), row.After)
		seen[row.Kind]++
	})
	if external != 0 {
		t.Fatal("independent audio queue scope not covered")
	}
	return count, seen
}

// CPU context matrices and these tests precede the new support implementation.
func TestFighter6SupportOriginalContexts(t *testing.T) {
	count, seen := verifyFighter6SupportCases(t, "fighter6-support-motion-boundaries")
	if count != 3000 {
		t.Fatal("incomplete support CPU matrix", count, seen)
	}
	for _, kind := range []string{"0e", "0f", "11", "13", "37", "exit"} {
		if seen[kind] == 0 {
			t.Fatal("missing support kind", kind)
		}
	}
	count, seen = verifyFighter6SupportCases(t, "fighter6-support-motion-oracle")
	if count != 34 || seen["11"] != 33 || seen["exit"] != 1 {
		t.Fatal("incomplete ordinary source6 support evidence", count, seen)
	}
}

func TestFighter6SupportNaturalContexts(t *testing.T) {
	count, seen := verifyFighter6SupportCases(t, "fighter6-support-motion-special-oracle")
	if count == 0 {
		t.Fatal("no actual support natural contexts")
	}
	t.Logf("source6 support natural calls=%d kinds=%v", count, seen)
}

func TestFighter6SupportRealClockFlows(t *testing.T) {
	steps := 0
	count, external := walkChuActor(t, "fighter6-support-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name    string
			Initial fighter3AirRaw
			Calls   []fighter3AirCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("missing support flow", err)
		}
		m := decodeFighter6SupportModel(t, stream.Initial)
		for _, row := range stream.Calls {
			if len(row.External) != 0 || row.RNGCalls != 0 || len(row.Clears) != 0 || len(row.ExternalChanges) != 0 {
				t.Fatal("undeclared support flow input")
			}
			compareFighter3AirRaw(t, m.encode(t, nil), row.Before)
			stepFighter6SupportModel(t, &m, row, library)
			compareFighter3AirRaw(t, m.encode(t, nil), row.After)
			steps++
		}
		kind := m.State.Objects[stream.Initial.ObjectID].Body.ActionKind
		if kind != 0 && kind != 2 {
			t.Fatal("support flow did not recover", stream.Name, kind)
		}
	})
	if count != 20 || steps < 1000 || external != 0 {
		t.Fatal("incomplete real clock support flows", count, steps, external)
	}
	t.Logf("support real clock streams=%d stages=%d", count, steps)
}

func TestFighter6SupportExitAndAtomicErrors(t *testing.T) {
	base := Fighter6MotionState{ChuSpecialMotionState: ChuSpecialMotionState{Energy: 223<<16 | 0xabcd, Power: 127, AlternateVelocity: 0x12345678}, StunTicks: 65535}
	got, err := StepFighter6SupportMotion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported support kind failed atomic rejection")
	}
	base.Body.ActionKind, base.Body.Clock.FrameIndex = 0x11, 8
	got, err = StepFighter6SupportMotion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("bad retreat index leaked state")
	}
	for _, kind := range []uint8{0x0e, 0x0f, 0x13} {
		base.Body.ActionKind, base.Body.Clock.FrameIndex, base.Body.Clock.Ticks = kind, 2, 0
		base.Body.Y, base.Body.Ground = 320<<16|0xffff, 320<<16
		base.Pool.Cursor = 1
		got, err = StepFighter6SupportMotion(base, JumpParameters{})
		if err == nil || got != base {
			t.Fatal("bad pool cursor leaked support state", kind)
		}
	}
	base.Body.ActionKind, base.Body.Clock.Completed = 0x37, 2
	got, err = StepFighter6SupportMotion(base, JumpParameters{})
	if err != nil || got != base {
		t.Fatal("37 read unused pool or treated non-one completed as done")
	}
	base.Body.Clock.Completed, base.Body.ExtraJump = 1, 255
	params := JumpParameters{Gravity: -0x80000000}
	got, err = StepFighter6SupportMotion(base, params)
	if err != nil || got.Body.ActionKind != 2 || got.Body.Pose != 7 || got.Body.EnabledStages != 0x14 || got.Body.ExtraJump != 0 || got.Body.VelocityX != 0 || got.Body.VerticalVelocity != 0 || got.Body.Gravity != params.Gravity || got.StunTicks != 65535 {
		t.Fatal("37 did not execute unconditional1860c", err)
	}
	body := base.Body
	body.Auxiliary7E, body.Auxiliary80 = -1, -32768
	vx, vy, gravity, extra := body.VelocityX, body.VerticalVelocity, body.Gravity, body.ExtraJump
	enterFighter6Exit(&body)
	if body.ActionKind != 12 || body.Pose != 0x2b || body.EnabledStages != 0 || body.Auxiliary7E != -32768 || body.Auxiliary80 != -32768 || body.VelocityX != vx || body.VerticalVelocity != vy || body.Gravity != gravity || body.ExtraJump != extra || body.Clock.FrameIndex != 0 || body.Clock.Duration != 8 || body.Clock.Ticks != 255 || body.Clock.Completed != 0 {
		t.Fatal("18682 state is not the original recovery constructor")
	}
}
