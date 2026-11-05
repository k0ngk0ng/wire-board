package engine

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type fighter5ProjectileCase struct {
	fighter3AirCase
	Entry, Exit string
	EntrySP     uint32                                `json:"entry_sp"`
	DrawRequest *WorldDraw                            `json:"draw_request"`
	RAMChanges  []struct{ Offset, Before, After int } `json:"ram_changes"`
}

func decodeFighter5Projectile(t *testing.T, raw fighter3AirRaw) fighter3AirModel {
	t.Helper()
	m := fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw}
	for _, region := range []struct {
		Raw               string
		Base, Count, Size int
	}{{raw.Family, int(raw.ObjectID), 4, 256}, {raw.Pool.Bytes, 0x9400, 16, 128}} {
		b := attackContactBytes(t, region.Raw, region.Count*region.Size)
		for i := 0; i < region.Count; i++ {
			id := uint16(region.Base + i*region.Size)
			padded := append(append([]byte(nil), b[i*region.Size:(i+1)*region.Size]...), make([]byte, 256-region.Size)...)
			m.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(padded))
		}
	}
	if m.State.Objects[raw.ObjectID].Tag != 5 {
		t.Fatal("not a proven source5 family")
	}
	b := attackContactBytes(t, raw.Buffers, 1024)
	for i := range m.State.Buffers {
		m.State.Buffers[i] = attackContactBuffer(t, raw.Offsets[i], hex.EncodeToString(b[i*256:(i+1)*256]))
	}
	m.State.PendingSound, m.State.SoundQueue, m.State.RandomSeed, m.State.EffectCursor = raw.PendingSound, raw.SoundQueue, raw.RandomSeed, raw.Pool.Cursor
	e := raw.Environment
	m.State.Clock.PauseFlags, m.State.Clock.SlowFlag, m.State.Clock.ScreenShake = e.PauseFlags, e.SlowFlag, uint16(e.Shake)
	m.State.Camera.Left, m.State.Camera.Right, m.State.Camera.X, m.State.Camera.Y = e.ArenaLeft, e.ArenaRight, e.CameraX, e.CameraY
	m.State.Match.AttributeMask = e.PaletteAttributes
	return m
}

func fighter5ProjectileView(o WorldObject, s ChuWorldState, pool EffectPool) Fighter5ProjectileState {
	return Fighter5ProjectileState{Visual: fighter4Visual(o, s), VelocityX: o.Body.VelocityX, VerticalVelocity: o.Body.VerticalVelocity,
		Ground: o.Body.Ground, Aux48: o.Aux48, Gravity: o.Body.Gravity, Depth: o.Body.Depth, Layer: o.Body.Layer, Team: o.Body.Team,
		ActionKind: o.Body.ActionKind, ContactFlags: o.ContactFlags, HitStopTicks: o.HitStopTicks, Owner: o.DragonBank, Pool: pool}
}

func fighter5ProjectileParent(o WorldObject) Fighter5ProjectileParent {
	return Fighter5ProjectileParent{ObjectID: o.ObjectID, X: o.Body.X, Y: o.Body.Y, Pose: o.Body.Pose, Held: o.Controls.Held,
		HitStopTicks: o.HitStopTicks, RetainTicks: o.Controls.RetainTicks}
}

func stepFighter5ProjectileModel(t *testing.T, m *fighter3AirModel, row fighter5ProjectileCase, library AnimationLibrary) ChuProjectileOutcome {
	t.Helper()
	s := &m.State
	id := row.ObjectID
	o := s.Objects[id]
	if row.Kind == "clock" {
		o.Body.Clock = StepAnimationClock(o.Body.Clock)
		s.Objects[id] = o
		return ChuProjectileOutcome{}
	}
	if row.Kind == "record" {
		v := fighter4Visual(o, *s)
		next, err := SubmitAnimationRecord(AnimationSubmissionState{Actor: v.Animation, Buffers: v.Buffers, PendingSound: v.PendingSound, SoundQueue: v.SoundQueue}, library)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		o, err = o.MergeAnimation(next.Actor)
		if err != nil {
			t.Fatal(err)
		}
		s.Objects[id] = o
		s.Buffers, s.PendingSound, s.SoundQueue = next.Buffers, next.PendingSound, next.SoundQueue
		return ChuProjectileOutcome{}
	}
	pool, err := BuildWorldEffectPool(s.Objects, s.EffectCursor)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(row.Kind, "creator") {
		view := Fighter5MotionState{ChuSpecialMotionState: ChuSpecialMotionState{ObjectID: id, Body: o.Body, Controls: o.Controls, Energy: o.Energy, Power: o.Power,
			AlternateVelocity: o.Aux48, ContactFlags: o.ContactFlags, TargetBit: o.TargetBit, Pool: pool}}
		for i := range view.Children {
			child := s.Objects[id+uint16(i+1)*256]
			view.Children[i] = child.AsChuChild()
			view.ChildExtras[i] = Fighter5ChildExtra{Gravity: child.Body.Gravity, Owner: child.DragonBank}
		}
		view, err = StepFighter5Motion(view, m.Raw.Parameters)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		o.Body, o.Controls, o.Energy, o.Power = view.Body, view.Controls, view.Energy, view.Power
		o.Aux48, o.ContactFlags, o.TargetBit = view.AlternateVelocity, view.ContactFlags, view.TargetBit
		s.Objects[id] = o
		for i, child := range view.Children {
			childID := id + uint16(i+1)*256
			c := s.Objects[childID].MergeChuChild(child)
			c.Body.Gravity, c.DragonBank = view.ChildExtras[i].Gravity, view.ChildExtras[i].Owner
			s.Objects[childID] = c
		}
		s.Objects, err = MergeWorldEffectPool(s.Objects, view.Pool)
		if err != nil {
			t.Fatal(err)
		}
		s.EffectCursor = view.Pool.Cursor
		return ChuProjectileOutcome{}
	}
	view := fighter5ProjectileView(o, *s, pool)
	parentID := view.Owner
	if o.Header.Behavior == 0x146ba {
		parentID = id - 0x200
	}
	parentObject, found := s.Objects[parentID]
	if !found {
		parentObject = WorldObject{ObjectID: parentID}
	}
	parent := fighter5ProjectileParent(parentObject)
	e := m.Raw.Environment
	e.PauseFlags, e.SlowFlag = s.Clock.PauseFlags, s.Clock.SlowFlag
	var next Fighter5ProjectileState
	var parentNext Fighter5ProjectileParent
	var out ChuProjectileOutcome
	if row.Kind == "linear_stop" || row.Kind == "steered_stop" {
		next, out, err = StepFighter5ProjectileHitStop(view, e, library)
		parentNext = parent
	} else {
		next, parentNext, out, err = StepFighter5Projectile(view, parent, e, library)
	}
	if err != nil {
		t.Fatal(row.Name, err)
	}
	o = mergeFighter4Visual(t, o, next.Visual, out.ClearedBytes)
	o.Body.VelocityX, o.Body.VerticalVelocity, o.Body.Ground, o.Aux48, o.Body.Gravity = next.VelocityX, next.VerticalVelocity, next.Ground, next.Aux48, next.Gravity
	o.Body.Depth, o.Body.Layer, o.Body.Team, o.Body.ActionKind = next.Depth, next.Layer, next.Team, next.ActionKind
	o.ContactFlags, o.HitStopTicks, o.DragonBank = next.ContactFlags, next.HitStopTicks, next.Owner
	s.Objects[id] = o
	if parentNext != parent {
		if !found {
			t.Fatal("callback modified absent parent")
		}
		parentObject.Body.X, parentObject.Body.Y, parentObject.Body.Pose = parentNext.X, parentNext.Y, parentNext.Pose
		parentObject.Controls.Held, parentObject.HitStopTicks, parentObject.Controls.RetainTicks = parentNext.Held, parentNext.HitStopTicks, parentNext.RetainTicks
		s.Objects[parentID] = parentObject
	}
	s.Objects, err = MergeWorldEffectPool(s.Objects, next.Pool)
	if err != nil {
		t.Fatal(err)
	}
	s.EffectCursor, s.Buffers, s.PendingSound, s.SoundQueue = next.Pool.Cursor, next.Visual.Buffers, next.Visual.PendingSound, next.Visual.SoundQueue
	return out
}

func checkFighter5ProjectileAudit(t *testing.T, row fighter5ProjectileCase) {
	t.Helper()
	if row.RNGCalls != 0 || len(row.Unexplained) != 0 {
		t.Fatal("unexplained original callback mutation", row.Name)
	}
	for _, change := range row.ExternalChanges {
		value, found := change.Before, false
		for _, irq := range row.Interrupts {
			if irq.Entry != "0xfff000" || irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a" {
				t.Fatal("unidentified original IRQ")
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
	for _, change := range row.RAMChanges {
		o := change.Offset
		allowed := o >= int(row.Before.ObjectID) && o < int(row.Before.ObjectID)+1024 || o >= 0x9400 && o < 0x9c00 || o >= 0xa200 && o < 0xa600 || o >= 0xa004 && o < 0xa00c || o == 0xa030 || o == 0xa036 || o == 0xa037 || o >= 0xf80a && o < 0xf80e
		if uint16(row.EntrySP)-uint16(o) < 0x180 {
			allowed = true
		}
		for _, external := range row.ExternalChanges {
			if change == external {
				allowed = true
			}
		}
		if !allowed || change.Before == change.After {
			t.Fatal("unclassified full64KB delta", change)
		}
	}
}

func verifyFighter5ProjectileRow(t *testing.T, m *fighter3AirModel, row fighter5ProjectileCase, library AnimationLibrary) {
	t.Helper()
	checkFighter5ProjectileAudit(t, row)
	out := stepFighter5ProjectileModel(t, m, row, library)
	if out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes || out.Deleted != (row.ClearedBytes != 0) {
		t.Fatal("callback outcome differs", row.Name, out, row.Draw, row.ClearedBytes)
	}
	if (row.DrawRequest != nil) != row.Draw || row.Draw && m.State.Objects[row.ObjectID].DrawRequest() != *row.DrawRequest {
		t.Fatal("callback-time draw value differs", row.Name)
	}
	for _, c := range row.Clears {
		var a uint16
		if c.PC != "0x37c56" && c.PC != "0x37cac" {
			t.Fatal("unidentified external audio consumer")
		}
		if _, err := fmt.Sscanf(c.Address, "0x%x", &a); err != nil || a < 0xf80a || a > 0xf80d {
			t.Fatal("invalid external audio queue address")
		}
		m.State.SoundQueue[a-0xf80a] = 0
	}
	compareFighter3AirRaw(t, m.encode(t, map[uint16]uint16{row.ObjectID: out.ClearedBytes}), row.After)
}

// Original CPU fixtures and failure scenarios precede fighter5_projectile.go.
func TestFighter5ProjectileOriginalBoundaries(t *testing.T) {
	seen := map[string]int{}
	count, _ := walkChuActor(t, "fighter5-projectile-boundaries", func(d *json.Decoder, library AnimationLibrary) {
		var row fighter5ProjectileCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		model := decodeFighter5Projectile(t, row.Before)
		verifyFighter5ProjectileRow(t, &model, row, library)
		seen[row.Kind]++
	})
	if count != 6363 || seen["linear"] != 2763 || seen["steered"] != 2833 || seen["follower"] != 639 || seen["linear_stop"] != 64 || seen["steered_stop"] != 64 {
		t.Fatal("missing original source5 boundary classes", count, seen)
	}
	t.Logf("source5 original boundary callbacks=%d kinds=%v", count, seen)
}

func TestFighter5ProjectileNaturalCallbacks(t *testing.T) {
	seen := map[string]int{}
	count, _ := walkChuActor(t, "fighter5-projectile-oracle", func(d *json.Decoder, library AnimationLibrary) {
		var row fighter5ProjectileCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		model := decodeFighter5Projectile(t, row.Before)
		verifyFighter5ProjectileRow(t, &model, row, library)
		seen[row.Kind]++
	})
	if count == 0 || seen["linear"] == 0 || seen["steered"] == 0 || seen["follower"] == 0 {
		t.Fatal("missing natural source5 callbacks", count, seen)
	}
	t.Logf("source5 actual natural callbacks=%d kinds=%v", count, seen)
}

func TestFighter5ProjectileCreatorLifecycles(t *testing.T) {
	steps := 0
	seen := map[string]int{}
	count, _ := walkChuActor(t, "fighter5-projectile-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name    string
			Initial fighter3AirRaw
			Calls   []fighter5ProjectileCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("missing single-initial source5 stream", err)
		}
		model := decodeFighter5Projectile(t, stream.Initial)
		for _, row := range stream.Calls {
			for key, value := range row.External {
				if key != "pause_flags" {
					t.Fatal("undeclared source5 external input")
				}
				model.State.Clock.PauseFlags = value
			}
			compareFighter3AirRaw(t, model.encode(t, nil), row.Before)
			verifyFighter5ProjectileRow(t, &model, row, library)
			seen[row.Kind]++
			steps++
		}
		for i := 1; i < 4; i++ {
			if model.State.Objects[stream.Initial.ObjectID+uint16(i)*256].Header.Size != 0 {
				t.Fatal("single-initial child lifecycle is unfinished", stream.Name)
			}
		}
	})
	if count != 16 || steps < 200 || seen["creator47"] == 0 || seen["creator48"] == 0 || seen["creator49"] == 0 || seen["creator50"] == 0 || seen["follower"] == 0 {
		t.Fatal("missing source5 creator-clock-child stream", count, steps, seen)
	}
	t.Logf("source5 single-initial streams=%d callbacks=%d kinds=%v", count, steps, seen)
}

func TestFighter5ProjectileErrorsAreAtomic(t *testing.T) {
	base := Fighter5ProjectileState{Visual: Fighter8ProjectileState{Header: MatchObjectHeader{Behavior: 0x1414e, Size: 256, Stage: 0}, Animation: AnimationActorState{ObjectID: 0x8500, WorldX: 100 << 16, WorldY: 200 << 16}}, Ground: 320 << 16, Owner: 0x8400}
	parent := Fighter5ProjectileParent{ObjectID: 0x8400, HitStopTicks: 19, RetainTicks: 29}
	env := ChuProjectileEnvironment{ArenaRight: 320}
	for _, scenario := range []string{"missing-data", "bad-size", "bad-callback", "bad-parent", "bad-pool", "bad-follower-index"} {
		state := base
		switch scenario {
		case "bad-size":
			state.Visual.Header.Size = 1
			state.Visual.Animation.WorldY = state.Ground
		case "bad-callback":
			state.Visual.Header.Behavior = 0x3d8
		case "bad-parent":
			state.ContactFlags = 4
			state.Owner = 0x8800
		case "bad-pool":
			state.Aux48 = 0x03abcdef
			state.Visual.Animation.WorldY = 300 << 16
			state.Pool.Cursor = 1
		case "bad-follower-index":
			state.Visual.Header.Behavior = 0x146ba
			state.Visual.Animation.ObjectID = 0x8600
			state.Visual.Header.Stage = 2
			state.Visual.Animation.Clock.FrameIndex = 8
			state.Visual.Animation.Clock.Ticks = 1
		}
		got, p, out, err := StepFighter5Projectile(state, parent, env, nil)
		if err == nil || !reflect.DeepEqual(got, state) || p != parent || out != (ChuProjectileOutcome{}) {
			t.Fatal("source5 error leaked state", scenario, err)
		}
	}
	stop := base
	stop.Visual.Header.Stage = 2
	stop.HitStopTicks = 1
	stop.Visual.Shake = 255
	got, out, err := StepFighter5ProjectileHitStop(stop, ChuProjectileEnvironment{}, nil)
	if err == nil || !reflect.DeepEqual(got, stop) || out != (ChuProjectileOutcome{}) {
		t.Fatal("slave AN error leaked hitstop/shake/offset")
	}
}
