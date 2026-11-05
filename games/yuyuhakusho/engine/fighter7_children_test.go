package engine

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type fighter7ChildrenRaw struct {
	fighter3AirRaw
	Parents [4]struct {
		ObjectID uint16 `json:"object_id"`
		Bytes    string
	}
	PanAmplitude uint16 `json:"pan_amplitude"`
	TileCursor   uint8  `json:"tile_cursor"`
}

type fighter7ChildrenDelta struct{ Offset, Before, After int }

type fighter7ChildrenCase struct {
	fighter3AirCase
	Before, After fighter7ChildrenRaw
	Entry, Exit   string
	Source        string     `json:"source_register"`
	EntrySP       uint32     `json:"entry_sp"`
	DrawRequest   *WorldDraw `json:"draw_request"`
	Paths         []string
	RAMChanges    []fighter7ChildrenDelta `json:"ram_changes"`
	Interrupts    []struct {
		Entry, Exit   string
		EntryRegister string `json:"entry_register"`
		ExitRegister  string `json:"exit_register"`
		EntrySP       uint32 `json:"entry_sp"`
		ExitSP        uint32 `json:"exit_sp"`
		Changes       []fighter7ChildrenDelta
	}
}

type fighter7ChildrenModel struct {
	Base fighter3AirModel
	Raw  fighter7ChildrenRaw
}

func decodeFighter7Children(t *testing.T, raw fighter7ChildrenRaw) fighter7ChildrenModel {
	t.Helper()
	m := fighter7ChildrenModel{Base: fighter3AirModel{State: ChuWorldState{Objects: make(WorldObjects)}, Raw: raw.fighter3AirRaw}, Raw: raw}
	for _, region := range []struct {
		Raw               string
		Base, Count, Size int
	}{{raw.Family, int(raw.ObjectID), 4, 256}, {raw.Pool.Bytes, 0x9400, 16, 128}} {
		b := attackContactBytes(t, region.Raw, region.Count*region.Size)
		for i := 0; i < region.Count; i++ {
			id := uint16(region.Base + i*region.Size)
			padded := append(append([]byte(nil), b[i*region.Size:(i+1)*region.Size]...), make([]byte, 256-region.Size)...)
			m.Base.State.Objects[id] = decodeWorldObjectTest(t, id, hex.EncodeToString(padded))
		}
	}
	if raw.ObjectID != 0x8400 || m.Base.State.Objects[raw.ObjectID].Tag != 7 {
		t.Fatal("source7 family identity is unproven")
	}
	for i, parent := range raw.Parents {
		if parent.ObjectID != uint16(0x8400+i*0x400) {
			t.Fatal("wrong four-parent identity")
		}
		o := decodeWorldObjectTest(t, parent.ObjectID, parent.Bytes)
		if parent.ObjectID == raw.ObjectID {
			if o != m.Base.State.Objects[parent.ObjectID] {
				t.Fatal("duplicate self parent differs")
			}
		} else {
			m.Base.State.Objects[parent.ObjectID] = o
		}
	}
	b := attackContactBytes(t, raw.Buffers, 1024)
	for i := range m.Base.State.Buffers {
		m.Base.State.Buffers[i] = attackContactBuffer(t, raw.Offsets[i], hex.EncodeToString(b[i*256:(i+1)*256]))
	}
	s := &m.Base.State
	s.PendingSound, s.SoundQueue, s.RandomSeed, s.EffectCursor = raw.PendingSound, raw.SoundQueue, raw.RandomSeed, raw.Pool.Cursor
	s.Clock.PauseFlags, s.Clock.SlowFlag, s.Clock.ScreenShake, s.Clock.PanAmplitude = raw.Environment.PauseFlags, raw.Environment.SlowFlag, uint16(raw.Environment.Shake), raw.PanAmplitude
	s.Camera.Left, s.Camera.Right, s.Camera.X, s.Camera.Y = raw.Environment.ArenaLeft, raw.Environment.ArenaRight, raw.Environment.CameraX, raw.Environment.CameraY
	s.Match.AttributeMask, s.EffectTileCursor = raw.Environment.PaletteAttributes, raw.TileCursor
	return m
}

func (m *fighter7ChildrenModel) encode(t *testing.T, cleared map[uint16]uint16) fighter7ChildrenRaw {
	t.Helper()
	raw := m.Raw
	raw.fighter3AirRaw = m.Base.encode(t, cleared)
	for i, parent := range raw.Parents {
		if parent.ObjectID == raw.ObjectID {
			raw.Parents[i].Bytes = raw.Family[:512]
			continue
		}
		// Other families are not owned by the P1 family codec. These are the
		// only cross-family fields consumed/written by the covered callbacks.
		b := attackContactBytes(t, parent.Bytes, 256)
		o := m.Base.State.Objects[parent.ObjectID]
		binary.BigEndian.PutUint16(b[0x78:], uint16(o.Body.Health))
		b[0x69] = o.HitStopTicks
		raw.Parents[i].Bytes = hex.EncodeToString(b)
	}
	raw.PanAmplitude, raw.TileCursor = m.Base.State.Clock.PanAmplitude, m.Base.State.EffectTileCursor
	m.Raw = raw
	return raw
}

func compareFighter7Children(t *testing.T, got, want fighter7ChildrenRaw) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	for name, pair := range map[string][2]string{"family": {got.Family, want.Family}, "pool": {got.Pool.Bytes, want.Pool.Bytes}, "buffers": {got.Buffers, want.Buffers},
		"parent0": {got.Parents[0].Bytes, want.Parents[0].Bytes}, "parent1": {got.Parents[1].Bytes, want.Parents[1].Bytes}, "parent2": {got.Parents[2].Bytes, want.Parents[2].Bytes}, "parent3": {got.Parents[3].Bytes, want.Parents[3].Bytes}} {
		a, b := attackContactBytes(t, pair[0], len(pair[0])/2), attackContactBytes(t, pair[1], len(pair[1])/2)
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("source7 %s byte%x got%x want%x", name, i, a[i], b[i])
			}
		}
	}
	t.Fatalf("source7 child metadata differs: got=%+v want=%+v", got, want)
}

func (m *fighter7ChildrenModel) step(t *testing.T, row fighter7ChildrenCase, frames AnimationLibrary, effects EffectMotionLibrary) ChuProjectileOutcome {
	t.Helper()
	s := &m.Base.State
	id := row.ObjectID
	o := s.Objects[id]
	if row.Kind == "clock" {
		o.Body.Clock = StepAnimationClock(o.Body.Clock)
		s.Objects[id] = o
		return ChuProjectileOutcome{}
	}
	if row.Kind == "record" {
		v, err := SubmitAnimationRecord(AnimationSubmissionState{Actor: o.AsAnimation(), Buffers: s.Buffers, PendingSound: s.PendingSound, SoundQueue: s.SoundQueue}, frames)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		o, err = o.MergeAnimation(v.Actor)
		if err != nil {
			t.Fatal(err)
		}
		s.Objects[id] = o
		s.Buffers, s.PendingSound, s.SoundQueue = v.Buffers, v.PendingSound, v.SoundQueue
		return ChuProjectileOutcome{}
	}
	pool, err := BuildWorldEffectPool(s.Objects, s.EffectCursor)
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(row.Kind, "creator") {
		v := Fighter7MotionState{ChuSpecialMotionState: ChuSpecialMotionState{ObjectID: id, Body: o.Body, Controls: o.Controls, Energy: o.Energy, Power: o.Power,
			AlternateVelocity: o.Aux48, ContactFlags: o.ContactFlags, TargetBit: o.TargetBit, Pool: pool}, Left: s.Camera.Left, OtherFighters: make(map[uint16]TailFighterEnergy)}
		for i := range v.Children {
			c := s.Objects[id+uint16(i+1)*256]
			v.Children[i] = c.AsChuChild()
			v.ChildExtras[i] = Fighter4ChildExtra{Direction: c.Aux48, Held: uint8(c.Controls.Held), Pressed: uint8(c.Controls.Pressed), ContactFlags: c.ContactFlags, DragonBank: c.DragonBank}
		}
		for _, parent := range m.Raw.Parents {
			if parent.ObjectID != id {
				p := s.Objects[parent.ObjectID]
				v.OtherFighters[p.ObjectID] = TailFighterEnergy{Size: p.Header.Size, Team: p.Body.Team, Health: p.Body.Health, Energy: int16(p.Energy >> 16), HitStopTicks: p.HitStopTicks}
			}
		}
		v, err = StepFighter7Motion(v, m.Raw.Parameters)
		if err != nil {
			t.Fatal(row.Name, err)
		}
		o.Body, o.Controls, o.Energy, o.Power, o.Aux48, o.ContactFlags, o.TargetBit = v.Body, v.Controls, v.Energy, v.Power, v.AlternateVelocity, v.ContactFlags, v.TargetBit
		s.Objects[id] = o
		for i, c := range v.Children {
			cid := id + uint16(i+1)*256
			co := s.Objects[cid].MergeChuChild(c)
			co.Aux48 = v.ChildExtras[i].Direction
			co.Controls.Held = co.Controls.Held&0xff00 | Buttons(v.ChildExtras[i].Held)
			co.Controls.Pressed = co.Controls.Pressed&0xff00 | Buttons(v.ChildExtras[i].Pressed)
			co.ContactFlags, co.DragonBank = v.ChildExtras[i].ContactFlags, v.ChildExtras[i].DragonBank
			s.Objects[cid] = co
		}
		for pid, p := range v.OtherFighters {
			po := s.Objects[pid]
			po.HitStopTicks = p.HitStopTicks
			s.Objects[pid] = po
		}
		s.Objects, err = MergeWorldEffectPool(s.Objects, v.Pool)
		if err != nil {
			t.Fatal(err)
		}
		s.EffectCursor = v.Pool.Cursor
		return ChuProjectileOutcome{}
	}
	e := m.Raw.Environment
	e.PauseFlags, e.SlowFlag = s.Clock.PauseFlags, s.Clock.SlowFlag
	sub := Fighter7ChildSubmission{Buffers: s.Buffers, PendingSound: s.PendingSound, SoundQueue: s.SoundQueue}
	var out ChuProjectileOutcome
	switch row.Kind {
	case "first":
		p := s.Objects[id-0x100]
		link := s.Objects[id+0x100]
		ctx := Fighter7FirstChildContext{Parent: Fighter7ChildParent{ObjectID: p.ObjectID, Pose: p.Body.Pose}, Link: Fighter7ChildLink{ObjectID: link.ObjectID, VelocityX: link.Body.VelocityX, VerticalVelocity: link.Body.VerticalVelocity, Aux48: link.Aux48}, Submission: sub}
		o, ctx, out, err = StepFighter7FirstChild(o, ctx, e, frames)
		link.Body.VelocityX, link.Body.VerticalVelocity, link.Aux48 = ctx.Link.VelocityX, ctx.Link.VerticalVelocity, ctx.Link.Aux48
		s.Objects[link.ObjectID] = link
		sub = ctx.Submission
	case "second":
		p := s.Objects[id-0x200]
		ctx := Fighter7SecondChildContext{Parent: Fighter7ChildParent{ObjectID: p.ObjectID, Pose: p.Body.Pose}, Pool: pool, Submission: sub}
		for i, parent := range m.Raw.Parents {
			po := s.Objects[parent.ObjectID]
			ctx.Fighters[i] = Fighter7HealingTarget{ObjectID: po.ObjectID, Size: po.Header.Size, Health: po.Body.Health, X: po.Body.X, Y: po.Body.Y}
		}
		for i := range ctx.EffectExtras {
			fx := s.Objects[uint16(0x9400+i*128)]
			ctx.EffectExtras[i] = Fighter7ChildEffectExtra{VerticalVelocity: fx.Body.VerticalVelocity, Delay: fx.Body.Clock.Duration}
		}
		o, ctx, out, err = StepFighter7SecondChild(o, ctx, e, frames)
		if err == nil {
			for _, p := range ctx.Fighters {
				po := s.Objects[p.ObjectID]
				po.Body.Health = p.Health
				s.Objects[p.ObjectID] = po
			}
			s.Objects, err = MergeWorldEffectPool(s.Objects, ctx.Pool)
			s.EffectCursor = ctx.Pool.Cursor
			for i, x := range ctx.EffectExtras {
				fid := uint16(0x9400 + i*128)
				fx := s.Objects[fid]
				fx.Body.VerticalVelocity, fx.Body.Clock.Duration = x.VerticalVelocity, x.Delay
				s.Objects[fid] = fx
			}
		}
		sub = ctx.Submission
	case "pan":
		o, s.Clock.PanAmplitude, out, err = StepFighter7Pan(o, s.Clock.PanAmplitude, e)
	case "effect":
		owners := make(map[uint16]EffectMotionOwner)
		for oid, owner := range s.Objects {
			owners[oid] = EffectMotionOwner{X: owner.Body.X, Y: owner.Body.Y}
		}
		v, w, result, fxerr := StepEffectMotion(o.AsEffectMotion(), EffectMotionWorld{PauseFlags: e.PauseFlags, SlowFlag: e.SlowFlag, PaletteAttributes: e.PaletteAttributes, TileCursor: s.EffectTileCursor, SoundQueue: s.SoundQueue, CameraX: e.CameraX, CameraY: e.CameraY, Shake: e.Shake}, owners, effects)
		err = fxerr
		out = ChuProjectileOutcome{Draw: result.Draw, Deleted: result.Deleted, ClearedBytes: result.ClearedBytes}
		if err == nil {
			o, err = ApplyObjectClearPrefix(o, out.ClearedBytes)
			if err == nil {
				o, err = o.MergeEffectMotion(v)
			}
		}
		s.EffectTileCursor, s.SoundQueue = w.TileCursor, w.SoundQueue
		sub.SoundQueue = w.SoundQueue
	default:
		t.Fatal("unknown source7 child fixture kind", row.Kind)
	}
	if err != nil {
		t.Fatal(row.Name, err)
	}
	s.Objects[id] = o
	s.Buffers, s.PendingSound, s.SoundQueue = sub.Buffers, sub.PendingSound, sub.SoundQueue
	return out
}

func auditFighter7Children(t *testing.T, row fighter7ChildrenCase) {
	t.Helper()
	if row.RNGCalls != 0 || len(row.Unexplained) != 0 || row.Source != fmt.Sprintf("0xffff%04x", row.ObjectID) {
		t.Fatal("unproven original child callback", row.Name, row.Source)
	}
	for _, change := range row.ExternalChanges {
		value, found := change.Before, false
		for _, irq := range row.Interrupts {
			if irq.Entry != "0xfff000" || irq.Exit != "0xfff09a" && irq.Exit != "0xfff0a8" && irq.Exit != "0xfff13a" {
				t.Fatal("unknown original IRQ path")
			}
			if irq.EntryRegister != "0xfff000" && irq.EntryRegister != "0xfffff000" {
				t.Fatal("IRQ raw address was discarded")
			}
			for _, d := range irq.Changes {
				if d.Offset == change.Offset {
					if d.Before != value {
						t.Fatal("IRQ chain mismatch")
					}
					value, found = d.After, true
				}
			}
		}
		for _, c := range row.Clears {
			var address int
			if _, err := fmt.Sscanf(c.Address, "0x%x", &address); err != nil {
				t.Fatal(err)
			}
			if address == change.Offset {
				value, found = 0, true
			}
		}
		if !found || value != change.After {
			t.Fatal("unattributed outside RAM write", change)
		}
	}
	for _, d := range row.RAMChanges {
		a := d.Offset
		allowed := a >= 0x8400 && a < 0x8800 || a >= 0x9400 && a < 0x9c00 || a >= 0xa200 && a < 0xa600 || a >= 0xa004 && a < 0xa00c || a == 0xa030 || a == 0xa031 || a == 0xa036 || a == 0xa037 || a >= 0xf80a && a < 0xf80e
		if row.Kind == "pan" && (a == 0xf570 || a == 0xf571) {
			allowed = true
		}
		for _, p := range row.Before.Parents {
			if row.Kind == "second" && (a == int(p.ObjectID)+0x78 || a == int(p.ObjectID)+0x79) || row.Kind == "creator61" && a == int(p.ObjectID)+0x69 {
				allowed = true
			}
		}
		if uint16(row.EntrySP)-uint16(a) < 0x300 {
			allowed = true
		}
		for _, x := range row.ExternalChanges {
			if a == x.Offset && d.Before == x.Before && d.After == x.After {
				allowed = true
			}
		}
		if !allowed || d.Before == d.After {
			t.Fatal("unclassified64KB child delta", row.Name, d)
		}
	}
}

func verifyFighter7Children(t *testing.T, m *fighter7ChildrenModel, row fighter7ChildrenCase, frames AnimationLibrary, effects EffectMotionLibrary) {
	t.Helper()
	auditFighter7Children(t, row)
	out := m.step(t, row, frames, effects)
	if out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes || out.Deleted != (row.ClearedBytes != 0) {
		t.Fatal("source7 child outcome differs", row.Name, out, row.Draw, row.ClearedBytes)
	}
	if (row.DrawRequest != nil) != row.Draw || row.Draw && m.Base.State.Objects[row.ObjectID].DrawRequest() != *row.DrawRequest {
		t.Fatal("source7 callback-time draw differs", row.Name)
	}
	for _, c := range row.Clears {
		var a uint16
		if c.PC != "0x37c56" && c.PC != "0x37cac" {
			t.Fatal("unknown external sound consumer")
		}
		if _, err := fmt.Sscanf(c.Address, "0x%x", &a); err != nil || a < 0xf80a || a > 0xf80d {
			t.Fatal("invalid sound clear address")
		}
		m.Base.State.SoundQueue[a-0xf80a] = 0
	}
	compareFighter7Children(t, m.encode(t, map[uint16]uint16{row.ObjectID: out.ClearedBytes}), row.After)
}

// The complete originalCPU evidence and failure list precede this test and
// every implementation symbol it names. The adapters never execute a ROM.
func TestFighter7ChildrenOriginalCallbacks(t *testing.T) {
	for _, suffix := range []string{"boundaries", "oracle"} {
		t.Run(suffix, func(t *testing.T) {
			seen, paths := map[string]int{}, map[string]int{}
			effects := chuWorldLibrariesTest(t).Effects
			count, _ := walkChuActor(t, "fighter7-children-"+suffix, func(d *json.Decoder, frames AnimationLibrary) {
				var row fighter7ChildrenCase
				if err := d.Decode(&row); err != nil {
					t.Fatal(err)
				}
				m := decodeFighter7Children(t, row.Before)
				verifyFighter7Children(t, &m, row, frames, effects)
				seen[row.Kind]++
				for _, p := range row.Paths {
					paths[p]++
				}
			})
			if count == 0 || seen["first"] == 0 || seen["second"] == 0 || suffix == "boundaries" && (seen["pan"] == 0 || paths["first_hit_stop"] == 0 || paths["second_hit_stop"] == 0 || paths["heal"] == 0) {
				t.Fatal("missing source7 original classes", count, seen, paths)
			}
			t.Logf("source7 %s callbacks=%d kinds=%v paths=%v", suffix, count, seen, paths)
		})
	}
}

func TestFighter7ChildrenSingleInitialLifecycles(t *testing.T) {
	seen := map[string]int{}
	steps := 0
	effects := chuWorldLibrariesTest(t).Effects
	count, _ := walkChuActor(t, "fighter7-children-sequences", func(d *json.Decoder, frames AnimationLibrary) {
		var stream struct {
			Name    string
			Initial fighter7ChildrenRaw
			Calls   []fighter7ChildrenCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Calls) == 0 {
			t.Fatal("missing source7 single-initial flow", err)
		}
		m := decodeFighter7Children(t, stream.Initial)
		for _, row := range stream.Calls {
			for k, v := range row.External {
				switch k {
				case "pause_flags":
					m.Base.State.Clock.PauseFlags = v
				case "held":
					o := m.Base.State.Objects[0x8400]
					o.Controls.Held = o.Controls.Held&0xff00 | Buttons(v)
					m.Base.State.Objects[0x8400] = o
				case "pressed":
					o := m.Base.State.Objects[0x8400]
					o.Controls.Pressed = o.Controls.Pressed&0xff00 | Buttons(v)
					m.Base.State.Objects[0x8400] = o
				default:
					t.Fatal("undeclared external input", k)
				}
			}
			compareFighter7Children(t, m.encode(t, nil), row.Before)
			verifyFighter7Children(t, &m, row, frames, effects)
			seen[row.Kind]++
			steps++
		}
		for _, id := range []uint16{0x8500, 0x8600, 0x8700} {
			if m.Base.State.Objects[id].Header.Size != 0 {
				t.Fatal("child stream did not clear", stream.Name, id)
			}
		}
		for id := uint16(0x9400); id < 0x9c00; id += 128 {
			if m.Base.State.Objects[id].Header.Size != 0 {
				t.Fatal("derivedFX stream did not clear", stream.Name, id)
			}
		}
	})
	if count != 8 || steps != 7948 || seen["creator59"] == 0 || seen["creator60"] == 0 || seen["creator61"] == 0 || seen["first"] == 0 || seen["second"] == 0 || seen["pan"] == 0 || seen["effect"] == 0 {
		t.Fatal("missing source7 creator/clock/childFX phases", count, steps, seen)
	}
	t.Logf("source7 single-initial flows=%d callbacks=%d kinds=%v", count, steps, seen)
}

func TestFighter7ChildrenErrorsAreAtomic(t *testing.T) {
	var chosen *fighter7ChildrenCase
	var frames AnimationLibrary
	walkChuActor(t, "fighter7-children-boundaries", func(d *json.Decoder, lib AnimationLibrary) {
		var row fighter7ChildrenCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if chosen == nil && row.Kind == "first" && row.Before.Environment.PauseFlags == 0 && row.Before.Environment.SlowFlag == 0 {
			copy := row
			chosen = &copy
			frames = lib
		}
	})
	if chosen == nil {
		t.Fatal("missing proven first-child context")
	}
	m := decodeFighter7Children(t, chosen.Before)
	o := m.Base.State.Objects[chosen.ObjectID]
	o.Header.Stage = 2
	o.Body.ActionKind = 0
	o.Body.Pose = 0x3e
	o.Body.Clock = AnimationClock{Duration: 8}
	o.HitStopTicks = 0
	sub := Fighter7ChildSubmission{Buffers: m.Base.State.Buffers, PendingSound: m.Base.State.PendingSound, SoundQueue: m.Base.State.SoundQueue}
	first := Fighter7FirstChildContext{Parent: Fighter7ChildParent{ObjectID: 0x8400, Pose: 0xa7}, Link: Fighter7ChildLink{ObjectID: 0x8600, VelocityX: 123, VerticalVelocity: -456, Aux48: 0x12345678}, Submission: sub}
	env := chosen.Before.Environment
	for _, name := range []string{"behavior", "action", "parent", "link", "animation", "clear"} {
		t.Run(name, func(t *testing.T) {
			a, c, lib := o, first, frames
			switch name {
			case "behavior":
				a.Header.Behavior = 1
			case "action":
				a.Body.ActionKind = 255
			case "parent":
				c.Parent.ObjectID = 0x8800
			case "link":
				c.Link.ObjectID = 0x8700
			case "animation":
				lib = nil
			case "clear":
				a.Header.Size = 272
				a.Body.ActionKind = 1
				a.Body.Clock.Completed = 1
			}
			got, ctx, out, err := StepFighter7FirstChild(a, c, env, lib)
			if err == nil || got != a || !reflect.DeepEqual(ctx, c) || out != (ChuProjectileOutcome{}) {
				t.Fatal("first-child error committed partial state", err)
			}
		})
	}
	secondObject := o
	secondObject.ObjectID, secondObject.Header.Behavior, secondObject.Body.ActionKind, secondObject.Body.Pose = 0x8600, 0x1391e, 2, 0x40
	secondObject.Controls.Held = ButtonB
	second := Fighter7SecondChildContext{Parent: first.Parent, Submission: sub}
	for i := range second.Fighters {
		second.Fighters[i] = Fighter7HealingTarget{ObjectID: uint16(0x8400 + i*0x400), Size: 256, Health: 200, X: secondObject.Body.X, Y: secondObject.Body.Y + 32<<16}
	}
	second.Pool.Cursor = 1
	got, ctx, out, err := StepFighter7SecondChild(secondObject, second, env, frames)
	if err == nil || got != secondObject || !reflect.DeepEqual(ctx, second) || out != (ChuProjectileOutcome{}) {
		t.Fatal("second-child allocation error is not atomic", err)
	}
	pan := o
	pan.ObjectID, pan.Header.Behavior, pan.Header.Size, pan.Body.VerticalVelocity = 0x8700, 0x15094, 272, 16<<16|0x1234
	got, amplitude, out, err := StepFighter7Pan(pan, 0xa55a, env)
	if err == nil || got != pan || amplitude != 0xa55a || out != (ChuProjectileOutcome{}) {
		t.Fatal("pan clear error is not atomic", err)
	}
}
