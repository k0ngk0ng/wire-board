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

// Byte layout belongs to this original-program verification adapter only.
// Production objects use typed fields and never serialize back through RAM.
func decodeWorldObjectTest(t *testing.T, id uint16, encoded string) WorldObject {
	t.Helper()
	b := attackContactBytes(t, encoded, 256)
	w := func(i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	l := func(i int) int32 { return int32(binary.BigEndian.Uint32(b[i:])) }
	o := WorldObject{ObjectID: id, Header: MatchObjectHeader{Behavior: uint32(l(0)), Size: w(4), Stage: w(6)},
		Body: FighterBodyState{GroundActionState: GroundActionState{Clock: AnimationClock{FrameIndex: b[0x59], Duration: b[0x5a], Ticks: b[0x5b], Flags: b[0x5c], Completed: b[0x60]},
			ActionKind: b[0x5d], Pose: b[0x5f], Facing: b[0x61], Auxiliary64: b[0x64], Request: b[0x6b], EnabledStages: b[0x6c], ActionFlags: b[0x6d],
			Team: b[0x6e], BaseTeam: b[0x6f], Auxiliary77: b[0x77], VerticalVelocity: l(0x14)},
			X: l(0x40), Y: l(0x44), VelocityX: l(0x10), Gravity: l(0x4c), Ground: l(0x9c), ExtraJump: b[0x87],
			Attributes: w(0x24), Layer: b[0x26], Depth: w(0x3e), Health: int16(w(0x78)), Auxiliary7E: int16(w(0x7e)), Auxiliary80: int16(w(0x80)), TransitionDelay: b[0xc8]},
		Controls: FighterControls{Held: Buttons(b[0x71]) | Buttons(b[0xc0])<<8, Pressed: Buttons(b[0x72]) | Buttons(b[0xc1])<<8,
			Changed: Buttons(b[0x73]) | Buttons(b[0xc2])<<8, RawHeld: b[0x74], RetainTicks: b[0x75]},
		Tag: b[0x58], GraphicsSet: uint32(l(0x1c)), TilesBase: uint32(l(0x20)), MovementSource: uint32(l(0x90)), SkipGraphics: b[0x27],
		GraphicOffsetX: int16(w(0x28)), GraphicOffsetY: int16(w(0x2a)), GraphicParameter: w(0x3c), AnimationSet: uint32(l(0x50)), ShapeSet: uint32(l(0x8c)),
		ScreenX: int16(w(8)), ScreenY: int16(w(12)), Aux0A: w(10), Aux0E: w(14), ScreenOffsetX: int16(w(0x54)), ScreenOffsetY: int16(w(0x56)),
		Aux18: l(0x18), Aux48: l(0x48), Energy: l(0xb8), DisplayedHealth: int16(w(0x7a)), DisplayedEnergy: int16(w(0xbc)), Power: int16(w(0x7c)),
		GuardMask: b[0x63], AttackValue: b[0x65], StunDamage: b[0x66], EffectKind: b[0x67], Reaction: b[0x68],
		HitStopTicks: b[0x69], Shake: b[0x6a], StatusDelay: b[0x86], AttackDelay: b[0x88], ContactFlags: b[0x89], TargetBit: b[0x70],
		InputChannel: b[0x76], TargetMask: uint32(l(0x94)), Residual: l(0x98), StunTicks: w(0x84), HUDOffset: w(0x8a),
		AttackScale: w(0xc4), ComboCount: b[0xc6], ChargeTicks: b[0xc7], InputMode: int8(b[0xc9]), StreamPosition: uint32(l(0xca)), AIBehavior: b[0xce],
		AITargetCursor: w(0xd0), AICountdown: w(0xd2), AIElapsed: w(0xd4), AIFollowTarget: w(0xd6)}
	copy(o.Body.Commands[:], b[0xa8:0xb8])
	for i := range o.GraphicCache {
		o.GraphicCache[i] = uint32(l(0x2c + i*4))
	}
	return o
}

func encodeWorldObjectTest(t *testing.T, o WorldObject, previous string, clearPrefix uint16) string {
	t.Helper()
	b := attackContactBytes(t, previous, 256)
	if clearPrefix > 256 {
		t.Fatal("test clear outside object")
	}
	clear(b[:clearPrefix])
	b = encodeJumpBody(o.Body, b)
	w := func(i int, v uint16) { binary.BigEndian.PutUint16(b[i:], v) }
	l := func(i int, v uint32) { binary.BigEndian.PutUint32(b[i:], v) }
	l(0, o.Header.Behavior)
	w(4, o.Header.Size)
	w(6, o.Header.Stage)
	w(8, uint16(o.ScreenX))
	w(10, o.Aux0A)
	w(12, uint16(o.ScreenY))
	w(14, o.Aux0E)
	l(0x18, uint32(o.Aux18))
	l(0x1c, o.GraphicsSet)
	l(0x20, o.TilesBase)
	b[0x27] = o.SkipGraphics
	w(0x28, uint16(o.GraphicOffsetX))
	w(0x2a, uint16(o.GraphicOffsetY))
	for i, value := range o.GraphicCache {
		l(0x2c+i*4, value)
	}
	w(0x3c, o.GraphicParameter)
	l(0x48, uint32(o.Aux48))
	l(0x50, o.AnimationSet)
	w(0x54, uint16(o.ScreenOffsetX))
	w(0x56, uint16(o.ScreenOffsetY))
	b[0x58], b[0x63] = o.Tag, o.GuardMask
	b[0x65], b[0x66], b[0x67], b[0x68], b[0x69], b[0x6a] = o.AttackValue, o.StunDamage, o.EffectKind, o.Reaction, o.HitStopTicks, o.Shake
	b[0x70] = o.TargetBit
	b[0x71], b[0xc0], b[0x72], b[0xc1], b[0x73], b[0xc2], b[0x74], b[0x75] = uint8(o.Controls.Held), uint8(o.Controls.Held>>8), uint8(o.Controls.Pressed), uint8(o.Controls.Pressed>>8), uint8(o.Controls.Changed), uint8(o.Controls.Changed>>8), o.Controls.RawHeld, o.Controls.RetainTicks
	b[0x76] = o.InputChannel
	w(0x78, uint16(o.Body.Health))
	w(0x7a, uint16(o.DisplayedHealth))
	w(0x7c, uint16(o.Power))
	w(0x7e, uint16(o.Body.Auxiliary7E))
	w(0x80, uint16(o.Body.Auxiliary80))
	w(0x84, o.StunTicks)
	b[0x86], b[0x88], b[0x89] = o.StatusDelay, o.AttackDelay, o.ContactFlags
	w(0x8a, o.HUDOffset)
	l(0x8c, o.ShapeSet)
	l(0x90, o.MovementSource)
	l(0x94, o.TargetMask)
	l(0x98, uint32(o.Residual))
	l(0xb8, uint32(o.Energy))
	w(0xbc, uint16(o.DisplayedEnergy))
	w(0xc4, o.AttackScale)
	b[0xc6], b[0xc7], b[0xc8], b[0xc9] = o.ComboCount, o.ChargeTicks, o.Body.TransitionDelay, uint8(o.InputMode)
	l(0xca, o.StreamPosition)
	b[0xce] = o.AIBehavior
	w(0xd0, o.AITargetCursor)
	w(0xd2, o.AICountdown)
	w(0xd4, o.AIElapsed)
	w(0xd6, o.AIFollowTarget)
	return hex.EncodeToString(b)
}

func denseWorldObjectTest(t *testing.T, id uint16, seed int) (WorldObject, string) {
	t.Helper()
	b := make([]byte, 256)
	for i := range b {
		b[i] = uint8(i*73 + seed*47 + 17)
	}
	raw := hex.EncodeToString(b)
	return decodeWorldObjectTest(t, id, raw), raw
}

func TestWorldObjectsClearAgainstOriginal(t *testing.T) {
	for _, name := range []string{"world-objects-oracle", "world-objects-boundaries"} {
		t.Run(name, func(t *testing.T) {
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
				t.Fatal("unverified original object clear")
			}
			var rows []struct {
				ID     uint16 `json:"object_id"`
				Before string `json:"before_bytes"`
				After  string `json:"after_bytes"`
				Count  uint16 `json:"cleared_bytes"`
			}
			if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != m.Records {
				t.Fatal("missing original clears", err)
			}
			seen := map[uint16]bool{}
			for i, row := range rows {
				input := decodeWorldObjectTest(t, row.ID, row.Before)
				want := decodeWorldObjectTest(t, row.ID, row.After)
				got, err := ApplyObjectClearPrefix(input, row.Count)
				if err != nil || got != want || got.ObjectID != input.ObjectID || encodeWorldObjectTest(t, got, row.Before, row.Count) != row.After {
					t.Fatalf("original clear %d prefix=%d differs: %v", i, row.Count, err)
				}
				seen[row.Count] = true
			}
			if name == "world-objects-boundaries" && len(seen) != 16 {
				t.Fatal("missing all prefix boundaries")
			}
			t.Logf("original object clear calls=%d prefixes=%d", len(rows), len(seen))
		})
	}
}

func TestWorldObjectsIndependentComponentViews(t *testing.T) {
	zero256, zero128, zero1024, zero2048 := hex.EncodeToString(make([]byte, 256)), hex.EncodeToString(make([]byte, 128)), hex.EncodeToString(make([]byte, 1024)), hex.EncodeToString(make([]byte, 2048))
	for seed := 0; seed < 4; seed++ {
		o, raw := denseWorldObjectTest(t, 0x8500, seed)
		p := decodeChuProjectile(t, chuProjectileSnapshot{ActorBytes: raw, ParentBytes: zero256, Pool: struct {
			Cursor uint16
			Bytes  string
		}{Bytes: zero2048}, Buffers: zero1024}, o.ObjectID)
		if o.AsChuProjectile() != p || o.AsAnimation() != p.Animation {
			t.Fatal("projectile/animation view disagrees with independent original-byte adapter")
		}
		for _, kind := range []string{"charge", "release"} {
			a, _ := decodeAttachmentOriginal(t, kind, attachmentOriginalSnapshot{ObjectID: o.ObjectID, ActorBytes: raw, ParentBytes: zero256, Buffers: zero1024})
			if o.AsChuAttachment(a.Kind) != a {
				t.Fatal("attachment view disagrees with original-byte adapter")
			}
		}
		a := attackContactActor(attackContactBytes(t, raw, 256))
		if got := o.AsCollisionActor(); got.AttackContactActor != a || got.ThrowDelay != o.Body.TransitionDelay {
			t.Fatal("collision view disagrees with original-byte adapter")
		}
		fx, fxRaw := denseWorldObjectTest(t, 0x9400, seed)
		pool := make([]byte, 2048)
		copy(pool, attackContactBytes(t, fxRaw, 256)[:128])
		motion, _ := decodeEffectMotion(t, effectMotionSnapshot{ActorBytes: fxRaw[:256], PoolBytes: hex.EncodeToString(pool)}, fx.ObjectID)
		if fx.AsEffectMotion() != motion {
			t.Fatal("generic FX view disagrees with independent byte adapter")
		}
		for _, name := range []string{"combo", "status"} {
			s, _ := decodeAttachedFX(t, name, attachedFXSnapshot{ObjectID: fx.ObjectID, ActorBytes: fxRaw[:256], ParentBytes: zero256, Pool: hex.EncodeToString(pool)})
			if fx.AsAttachedEffect(s.Kind) != s {
				t.Fatal("attached FX view disagrees with independent byte adapter")
			}
		}
		_ = zero128
	}
}

func TestWorldObjectsRoundTripsAndIdentityErrors(t *testing.T) {
	for seed := 0; seed < 4; seed++ {
		o, _ := denseWorldObjectTest(t, 0x8400, seed)
		children := [3]WorldObject{}
		for i := range children {
			children[i], _ = denseWorldObjectTest(t, o.ObjectID+uint16(i+1)*256, seed+i+1)
		}
		fighter, err := o.AsChuFighter(children)
		if err != nil {
			t.Fatal(err)
		}
		f, err := o.MergeChuFighter(fighter)
		if err != nil || f != o {
			t.Fatal("fighter view lost unrelated canonical fields")
		}
		checks := []struct {
			got WorldObject
			err error
		}{}
		v, err := o.MergeAnimation(o.AsAnimation())
		checks = append(checks, struct {
			got WorldObject
			err error
		}{v, err})
		v, err = o.MergeChuProjectile(o.AsChuProjectile())
		checks = append(checks, struct {
			got WorldObject
			err error
		}{v, err})
		for _, kind := range []ChuAttachmentKind{ChuChargeAttachment, ChuReleaseAttachment} {
			v, err = o.MergeChuAttachment(o.AsChuAttachment(kind))
			checks = append(checks, struct {
				got WorldObject
				err error
			}{v, err})
		}
		v, err = o.MergeEffectMotion(o.AsEffectMotion())
		checks = append(checks, struct {
			got WorldObject
			err error
		}{v, err})
		for _, kind := range []AttachedEffectKind{AttachedComboNumber, AttachedStunEffect} {
			v, err = o.MergeAttachedEffect(o.AsAttachedEffect(kind))
			checks = append(checks, struct {
				got WorldObject
				err error
			}{v, err})
		}
		for _, check := range checks {
			if check.err != nil || check.got != o {
				t.Fatal("component merge round trip changed unrelated fields", check.err)
			}
		}
		if o.MergeChuChild(o.AsChuChild()) != o || o.MergeEffectSlot(o.AsEffectSlot()) != o || o.MergeCollisionActor(o.AsCollisionActor()) != o || o.MergeFighterInitialization(o.AsFighterInitialization()) != o || o.MergeReactionExtra(o.AsReactionExtra()) != o {
			t.Fatal("owned-field round trip lost canonical fields")
		}
		bad := o.AsAnimation()
		bad.ObjectID++
		v, err = o.MergeAnimation(bad)
		if err == nil || v != o {
			t.Fatal("cross-slot animation write was accepted")
		}
		badChildren := children
		badChildren[2].ObjectID++
		if _, err = o.AsChuFighter(badChildren); err == nil {
			t.Fatal("wrong child identity accepted")
		}
		for _, count := range []uint16{1, 15, 17, 255, 257, 65535} {
			v, err = ApplyObjectClearPrefix(o, count)
			if err == nil || v != o {
				t.Fatal("invalid prefix changed canonical object")
			}
		}
		v, err = ApplyObjectClearPrefix(o, 0)
		if err != nil || v != o {
			t.Fatal("zero request was treated as deletion")
		}
	}
}

func TestWorldObjectsParentAndAllocatorOwnership(t *testing.T) {
	objects := WorldObjects{}
	for i := 0; i < 4; i++ {
		id := uint16(0x8400 + i*256)
		objects[id], _ = denseWorldObjectTest(t, id, i)
	}
	for i := 0; i < 16; i++ {
		id := uint16(0x9400 + i*128)
		o, _ := denseWorldObjectTest(t, id, i+4)
		o.Header.Size = 128
		objects[id] = o
	}
	free := objects[0x9480]
	free.Header.Size = 0
	objects[0x9480] = free
	before := make(WorldObjects, len(objects))
	for id, o := range objects {
		before[id] = o
	}
	f, err := BuildChuActorFighter(objects, 0x8400)
	if err != nil {
		t.Fatal(err)
	}
	f.Body.X++
	f.Energy = 123
	f.Children[0].Behavior = 0x1313e
	f.Children[0].Size = 256
	f.Children[0].Pose = 7
	f.ChildAttackFlags = [3]uint8{}
	merged, err := MergeWorldChuActor(objects, f)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(objects, before) || merged[0x8400].Body.X != f.Body.X || merged[0x8400].Energy != 123 {
		t.Fatal("parent merge mutated caller or lost owned state")
	}
	for i := 0; i < 3; i++ {
		id := uint16(0x8500 + i*256)
		old, new := objects[id], merged[id]
		if old.Body.Clock != new.Body.Clock || old.GraphicCache != new.GraphicCache || old.GraphicsSet != new.GraphicsSet || old.ShapeSet != new.ShapeSet || old.Body.Auxiliary64 != new.Body.Auxiliary64 || old.Controls != new.Controls || old.Energy != new.Energy {
			t.Fatal("parent child-subset write erased live child state")
		}
	}
	pool, err := BuildWorldEffectPool(objects, 0)
	if err != nil {
		t.Fatal(err)
	}
	pool, index, err := AllocateEffect(pool)
	if err != nil || index != 1 {
		t.Fatal("allocator fixture mismatch", index, err)
	}
	fx, err := MergeWorldEffectPool(objects, pool)
	if err != nil {
		t.Fatal(err)
	}
	old, new := objects[0x9480], fx[0x9480]
	if new.AsEffectSlot() != pool.Slots[index] || old.Body.VerticalVelocity != new.Body.VerticalVelocity || old.Body.Clock != new.Body.Clock || old.Aux48 != new.Aux48 || old.GraphicCache != new.GraphicCache || old.AnimationSet != new.AnimationSet || old.ScreenOffsetY != new.ScreenOffsetY {
		t.Fatal("allocator merge overwrote lifecycle fields")
	}
	if !reflect.DeepEqual(objects, before) {
		t.Fatal("effect pool merge changed caller")
	}
	missing, err := LookupWorldObject(objects, 0x9c00)
	if err != nil || missing != (WorldObject{ObjectID: 0x9c00}) {
		t.Fatal("missing slot is not inactive canonical state")
	}
	broken := make(WorldObjects, len(objects))
	for id, o := range objects {
		broken[id] = o
	}
	bad := broken[0x9580]
	bad.ObjectID++
	broken[0x9580] = bad
	got, err := MergeWorldEffectPool(broken, pool)
	if err == nil || !reflect.DeepEqual(got, broken) {
		t.Fatal("late pool identity error leaked partial merge")
	}
}

func TestWorldObjectsCollisionResultsAgainstOriginal(t *testing.T) {
	var rows []struct{ Before, After attackContactSnapshot }
	readLaneMotion(t, "attack-contacts-boundaries", &rows)
	if len(rows) != 915 {
		t.Fatal("missing original attack matrix")
	}
	for i, row := range rows {
		state := attackContactState(t, row.Before)
		objects := WorldObjects{}
		for id, raw := range row.Before.Actors {
			o := decodeWorldObjectTest(t, id, raw)
			objects[id] = o
			state.Actors[id] = o.AsCollisionActor().AttackContactActor
		}
		got, err := ResolveAttackContacts(state)
		if err != nil {
			t.Fatal(err)
		}
		for id, a := range got.Actors {
			old := objects[id]
			merged := old.MergeCollisionActor(CollisionActor{AttackContactActor: a, ThrowDelay: old.Body.TransitionDelay})
			if encodeWorldObjectTest(t, merged, row.Before.Actors[id], 0) != row.After.Actors[id] {
				t.Fatalf("original collision merge %d owner=%x differs", i, id)
			}
		}
	}
}
