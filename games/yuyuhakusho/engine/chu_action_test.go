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

type chuActionSnapshot struct {
	Tail       chuTailSnapshot
	Children   string
	WalkSpeed  int32 `json:"walk_speed"`
	Left       int16
	Right      int16
	GlobalF568 uint16 `json:"global_f568"`
	GlobalF56A uint16 `json:"global_f56a"`
	GlobalA038 uint16 `json:"global_a038"`
}

func decodeChuAction(t *testing.T, raw chuActionSnapshot, id uint16) ChuActionState {
	t.Helper()
	tail := decodeChuTail(t, raw.Tail, id)
	a := attackContactBytes(t, raw.Tail.ActorBytes, 256)
	s := ChuActionState{ChuTailState: tail, EnergyFraction: binary.BigEndian.Uint16(a[0xba:]), HitStopTicks: a[0x69], Shake: a[0x6a],
		ScreenOffsetX: int16(binary.BigEndian.Uint16(a[0x54:])), StatusDelay: a[0x86], Residual: int32(binary.BigEndian.Uint32(a[0x98:])),
		StunTicks: binary.BigEndian.Uint16(a[0x84:]), TargetBit: a[0x70], Power: int16(binary.BigEndian.Uint16(a[0x7c:])),
		AlternateVelocity: int32(binary.BigEndian.Uint32(a[0x48:])), GlobalF568: raw.GlobalF568, GlobalF56A: raw.GlobalF56A, Left: raw.Left, Right: raw.Right,
		MovementSource: binary.BigEndian.Uint32(a[0x90:]), DragonBankCursor: raw.GlobalA038}
	fullEnergy := int32(binary.BigEndian.Uint32(a[0xb8:]))
	special, _, _, pool := decodeChuSpecialMotion(t, chuSpecialMotionSnapshot{ObjectID: id, Body: s.Body, ActorBytes: raw.Tail.ActorBytes,
		Controls: s.Controls, Energy: fullEnergy, Power: s.Power, AlternateVelocity: s.AlternateVelocity, ContactFlags: s.ContactFlags,
		Children: raw.Children, Pool: raw.Tail.Pool})
	s.Children = special.Children
	childBytes := attackContactBytes(t, raw.Children, 768)
	for i := range s.ChildExtras {
		b := childBytes[i*256:]
		s.ChildExtras[i] = Fighter4ChildExtra{Direction: int32(binary.BigEndian.Uint32(b[0x48:])), Held: b[0x71], Pressed: b[0x72], ContactFlags: b[0x89], DragonBank: binary.BigEndian.Uint16(b[0xfe:])}
		if a[0x58] == 5 {
			s.ChildExtras[i].DragonBank = 0
			s.Fighter5ChildExtras[i] = Fighter5ChildExtra{Gravity: int32(binary.BigEndian.Uint32(b[0x4c:])), Owner: binary.BigEndian.Uint16(b[0xfe:])}
		}
	}
	for i := range s.EffectExtras {
		b := pool[i*128:]
		s.EffectExtras[i] = ReactionEffectExtra{Owner: binary.BigEndian.Uint16(b[0x48:]), OffsetY: int16(binary.BigEndian.Uint16(b[0x56:])), Phase: b[0x5a]}
	}
	return s
}

func encodeChuAction(t *testing.T, s ChuActionState, previous chuActionSnapshot) chuActionSnapshot {
	t.Helper()
	raw := previous
	raw.Tail = encodeChuTail(t, s.ChuTailState, previous.Tail)
	fullEnergy := int32(uint32(uint16(s.Energy))<<16 | uint32(s.EnergyFraction))
	a, children, pool := encodeChuSpecialMotion(ChuSpecialMotionState{ObjectID: s.ObjectID, Body: s.Body, Controls: s.Controls,
		Energy: fullEnergy, Power: s.Power, AlternateVelocity: s.AlternateVelocity, ContactFlags: s.ContactFlags, TargetBit: s.TargetBit,
		Children: s.Children, Pool: s.Pool}, attackContactBytes(t, raw.Tail.ActorBytes, 256), attackContactBytes(t, previous.Children, 768), attackContactBytes(t, raw.Tail.Pool.Bytes, 2048))
	a[0x69], a[0x6a], a[0x86] = s.HitStopTicks, s.Shake, s.StatusDelay
	binary.BigEndian.PutUint16(a[0x54:], uint16(s.ScreenOffsetX))
	binary.BigEndian.PutUint32(a[0x98:], uint32(s.Residual))
	binary.BigEndian.PutUint16(a[0x84:], s.StunTicks)
	binary.BigEndian.PutUint32(a[0x90:], s.MovementSource)
	for i, extra := range s.ChildExtras {
		b := children[i*256:]
		binary.BigEndian.PutUint32(b[0x48:], uint32(extra.Direction))
		b[0x71], b[0x72], b[0x89] = extra.Held, extra.Pressed, extra.ContactFlags
		binary.BigEndian.PutUint16(b[0xfe:], extra.DragonBank)
		if a[0x58] == 5 {
			binary.BigEndian.PutUint32(b[0x4c:], uint32(s.Fighter5ChildExtras[i].Gravity))
			binary.BigEndian.PutUint16(b[0xfe:], s.Fighter5ChildExtras[i].Owner)
		}
	}
	for i, e := range s.EffectExtras {
		b := pool[i*128:]
		binary.BigEndian.PutUint16(b[0x48:], e.Owner)
		binary.BigEndian.PutUint16(b[0x56:], uint16(e.OffsetY))
		b[0x5a] = e.Phase
	}
	raw.Tail.ActorBytes, raw.Children = hex.EncodeToString(a), hex.EncodeToString(children)
	raw.Tail.ChildBytes = hex.EncodeToString(children[:256])
	raw.Tail.Pool.Bytes = hex.EncodeToString(pool)
	raw.GlobalF568, raw.GlobalF56A, raw.Left, raw.Right = s.GlobalF568, s.GlobalF56A, s.Left, s.Right
	raw.GlobalA038 = s.DragonBankCursor
	// Parameter metadata is a read-only view of the current source, just like
	// the original snapshot producer. It is not retained fighter state.
	oldSource := binary.BigEndian.Uint32(attackContactBytes(t, previous.Tail.ActorBytes, 256)[0x90:])
	if s.MovementSource != 0 && s.MovementSource != oldSource {
		movement, found := fighter4CompositionMovements(t)[s.MovementSource]
		if !found {
			t.Fatal("test snapshot movement metadata is unavailable", s.MovementSource)
		}
		raw.WalkSpeed = movement.WalkSpeed
		raw.Tail.Parameters = ChuTailParameters{JumpParameters: movement.JumpParameters, BurstSpeed: movement.BurstSpeed, EnergyDrain: movement.EnergyDrain}
	}
	return raw
}

type chuActionCall struct {
	Name, Scenario string
	ObjectID       uint16 `json:"object_id"`
	Before, After  chuActionSnapshot
	RNGCalls       int `json:"rng_calls"`
	Clears         []struct {
		PC      string
		Address string
		Before  uint8
		SR, SP  uint32
	} `json:"external_queue_clears"`
	External struct {
		Held, Pressed, Changed uint8
		RawHeld                uint8 `json:"raw_held"`
	} `json:"external_input"`
}

func readChuAction(t *testing.T, name string) (chuTailFixture, AnimationLibrary, int, int) {
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
		Pass, OneRNG bool
		Records      int
		External     int    `json:"external_queue_calls"`
		Baseline     bool   `json:"frames_match_original_core"`
		SHA          string `json:"fixture_sha256"`
		Gzip         string `json:"gzip_sha256"`
		OneEach      bool   `json:"one_rng_call_each"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || !m.OneEach || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original complete action fixture")
	}
	var f chuTailFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	library := make(AnimationLibrary, len(f.Library))
	for _, row := range f.Library {
		if _, exists := library[row.Key]; exists {
			t.Fatal("duplicate complete-action frame key")
		}
		library[row.Key] = row.Data
	}
	return f, library, m.Records, m.External
}

func actionCommandLibrary(t *testing.T) CommandLibrary {
	t.Helper()
	raw, err := os.ReadFile("../data/command-programs.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct{ Library CommandLibrary }
	if err := json.Unmarshal(raw, &data); err != nil || len(data.Library) != 16 {
		t.Fatal("missing verified immutable command programs", err)
	}
	return data.Library
}

func actionParameters(raw chuActionSnapshot) ChuActionParameters {
	return ChuActionParameters{ChuTailParameters: raw.Tail.Parameters, WalkSpeed: raw.WalkSpeed}
}

// Complete original call fixtures and these tests precede chu_action.go.
func TestChuActionOriginalCompleteCalls(t *testing.T) {
	commands := actionCommandLibrary(t)
	for _, name := range []string{"chu-action-oracle", "chu-action-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, library, count, externalCount := readChuAction(t, name)
			var rows []chuActionCall
			if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count {
				t.Fatal("missing complete original action calls", err)
			}
			oldFrames, _ := json.Marshal(f.Library)
			oldCommands, _ := json.Marshal(commands)
			external, differences := 0, 0
			for i, row := range rows {
				input := decodeChuAction(t, row.Before, row.ObjectID)
				copy := decodeChuAction(t, row.Before, row.ObjectID)
				got, err := StepChuAction(input, actionParameters(row.Before), commands, library)
				want := decodeChuAction(t, row.After, row.ObjectID)
				encoded, expected := encodeChuAction(t, got, row.Before), row.After
				if len(row.Clears) != 0 {
					external++
					for _, c := range row.Clears {
						if c.PC != "0x37c56" && c.PC != "0x37cac" || c.Address < "0xf80a" || c.Address > "0xf80d" {
							t.Fatal("unidentified external queue mutation")
						}
					}
					if got.SoundQueue != want.SoundQueue {
						differences++
					}
					want.SoundQueue, expected.Tail.SoundQueue = got.SoundQueue, got.SoundQueue
				}
				if err != nil || row.RNGCalls != 1 || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encoded, expected) {
					t.Fatalf("complete action %d %s/%s action=%x differs: err=%v", i, row.Scenario, row.Name, input.Body.ActionKind, err)
				}
				if !reflect.DeepEqual(input, copy) {
					t.Fatal("complete action mutated caller-owned state")
				}
			}
			for i := range f.Library {
				f.Library[i].Data = library[f.Library[i].Key]
			}
			newFrames, _ := json.Marshal(f.Library)
			newCommands, _ := json.Marshal(commands)
			if external != externalCount || !bytes.Equal(oldFrames, newFrames) || !bytes.Equal(oldCommands, newCommands) {
				t.Fatal("missing external evidence or mutated immutable rules")
			}
			t.Logf("complete action calls=%d external queue calls=%d queue differences=%d", count, external, differences)
		})
	}
}

func TestChuActionSingleInitialState(t *testing.T) {
	f, library, count, external := readChuAction(t, "chu-action-sequences")
	var streams []struct {
		Initial chuActionSnapshot
		Calls   []chuActionCall
	}
	if err := json.Unmarshal(f.Calls, &streams); err != nil || len(streams) != count || count != 8 || external != 0 {
		t.Fatal("missing complete-action streams", err)
	}
	commands := actionCommandLibrary(t)
	steps := 0
	for _, stream := range streams {
		if len(stream.Calls) == 0 {
			t.Fatal("empty full-action stream")
		}
		state := decodeChuAction(t, stream.Initial, stream.Calls[0].ObjectID)
		encoded := stream.Initial
		for i, call := range stream.Calls {
			state.Controls.Held = state.Controls.Held&0xff00 | Buttons(call.External.Held)
			state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(call.External.Pressed)
			state.Controls.Changed = state.Controls.Changed&0xff00 | Buttons(call.External.Changed)
			state.Controls.RawHeld = call.External.RawHeld
			var err error
			state, err = StepChuAction(state, actionParameters(stream.Initial), commands, library)
			encoded = encodeChuAction(t, state, encoded)
			if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeChuAction(t, call.After, call.ObjectID)) {
				t.Fatalf("complete-action stream step %d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != 790 {
		t.Fatal("missing complete-action stream steps")
	}
}

func TestChuActionErrorsAreAtomic(t *testing.T) {
	f, library, _, _ := readChuAction(t, "chu-action-boundaries")
	var rows []chuActionCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	base := decodeChuAction(t, rows[0].Before, rows[0].ObjectID)
	commands := actionCommandLibrary(t)
	for _, c := range []struct {
		State ChuActionState
		Cmd   CommandLibrary
		Anim  AnimationLibrary
	}{{base, nil, library}, {base, commands, nil}} {
		got, err := StepChuAction(c.State, actionParameters(rows[0].Before), c.Cmd, c.Anim)
		if err == nil || !reflect.DeepEqual(got, c.State) {
			t.Fatal("missing early/late rule data leaked action state")
		}
	}
	base.HitStopTicks, base.Body.ActionKind = 0, 255
	got, err := StepChuAction(base, actionParameters(rows[0].Before), commands, library)
	if err == nil || !reflect.DeepEqual(got, base) {
		t.Fatal("unsupported action leaked command/prelude state")
	}
}
