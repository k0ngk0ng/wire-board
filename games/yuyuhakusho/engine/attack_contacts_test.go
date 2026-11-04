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

// These tests precede attack_contacts.go. The adapter writes semantic fields
// into each original input, then compares every actor/buffer/pool output byte.
type attackContactSnapshot struct {
	AttackCount  uint16            `json:"attack_count"`
	HurtCount    uint16            `json:"hurt_count"`
	AttackBytes  string            `json:"attack_bytes"`
	HurtBytes    string            `json:"hurt_bytes"`
	Actors       map[uint16]string `json:"actors"`
	Effects      string            `json:"effects"`
	Cursor       uint16            `json:"cursor"`
	PendingSound uint8             `json:"pending_sound"`
	SlowTicks    uint16            `json:"slow_ticks"`
	ComboFlags   uint8             `json:"combo_flags"`
	ArenaLeft    uint16            `json:"arena_left"`
	ArenaRight   uint16            `json:"arena_right"`
}

func attackContactBytes(t *testing.T, s string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != size {
		t.Fatalf("invalid original attack-contact bytes: %v, size=%d/%d", err, len(b), size)
	}
	return b
}

func attackContactActor(b []byte) AttackContactActor {
	w := func(i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	l := func(i int) int32 { return int32(binary.BigEndian.Uint32(b[i:])) }
	return AttackContactActor{X: l(0x40), Y: l(0x44), Ground: l(0x9c), VelocityX: l(0x10), VelocityY: l(0x14), Gravity: l(0x4c),
		Attributes: w(0x24), Layer: b[0x26], Depth: w(0x3e), CharacterID: b[0x58], FrameIndex: b[0x59], FrameDuration: b[0x5a],
		FrameTicks: b[0x5b], FrameFlags: b[0x5c], ActionKind: b[0x5d], Pose: b[0x5f], Completed: b[0x60], Facing: b[0x61],
		GuardMask: b[0x63], AttackFlags: b[0x64], AttackValue: b[0x65], StunDamage: b[0x66], EffectKind: b[0x67], Reaction: b[0x68],
		HitStopTicks: b[0x69], Shake: b[0x6a], ActionFlags: b[0x6c], CancelTicks: b[0x6d], Team: b[0x6e], TargetBit: b[0x70],
		RetainedHeld: b[0x71], InputRetention: b[0x75], Auxiliary77: b[0x77], Health: int16(w(0x78)), StunValue: int16(w(0x7e)),
		StunTicks: w(0x84), StatusDelay: b[0x86], AttackDelay: b[0x88], ContactFlags: b[0x89], TargetMask: uint32(l(0x94)),
		Residual: l(0x98), AuxiliaryB8: l(0xb8), ComboCount: b[0xc6]}
}

func encodeAttackContactActor(a AttackContactActor, previous []byte) []byte {
	b := append([]byte(nil), previous...)
	w := func(i int, v uint16) { binary.BigEndian.PutUint16(b[i:], v) }
	l := func(i int, v int32) { binary.BigEndian.PutUint32(b[i:], uint32(v)) }
	l(0x40, a.X)
	l(0x44, a.Y)
	l(0x9c, a.Ground)
	l(0x10, a.VelocityX)
	l(0x14, a.VelocityY)
	l(0x4c, a.Gravity)
	w(0x24, a.Attributes)
	b[0x26] = a.Layer
	w(0x3e, a.Depth)
	b[0x58], b[0x59], b[0x5a], b[0x5b] = a.CharacterID, a.FrameIndex, a.FrameDuration, a.FrameTicks
	b[0x5c], b[0x5d], b[0x5f], b[0x60], b[0x61] = a.FrameFlags, a.ActionKind, a.Pose, a.Completed, a.Facing
	b[0x63], b[0x64], b[0x65], b[0x66], b[0x67], b[0x68] = a.GuardMask, a.AttackFlags, a.AttackValue, a.StunDamage, a.EffectKind, a.Reaction
	b[0x69], b[0x6a], b[0x6c], b[0x6d], b[0x6e], b[0x70] = a.HitStopTicks, a.Shake, a.ActionFlags, a.CancelTicks, a.Team, a.TargetBit
	b[0x71], b[0x75], b[0x77] = a.RetainedHeld, a.InputRetention, a.Auxiliary77
	w(0x78, uint16(a.Health))
	w(0x7e, uint16(a.StunValue))
	w(0x84, a.StunTicks)
	b[0x86], b[0x88], b[0x89] = a.StatusDelay, a.AttackDelay, a.ContactFlags
	l(0x94, int32(a.TargetMask))
	l(0x98, a.Residual)
	l(0xb8, a.AuxiliaryB8)
	b[0xc6] = a.ComboCount
	return b
}

func attackContactBuffer(t *testing.T, count uint16, encoded string) AnimationShapeBuffer {
	t.Helper()
	b := attackContactBytes(t, encoded, 256)
	buffer := AnimationShapeBuffer{Offset: count}
	for i := range buffer.Records {
		raw := b[i*16:]
		w := func(j int) uint16 { return binary.BigEndian.Uint16(raw[j:]) }
		buffer.Records[i] = AnimationShapeRecord{X: int16(w(0)), Y: int16(w(4)), Width: raw[8], Height: raw[9], Owner: w(12),
			Preserved: [4]uint16{w(2), w(6), w(10), w(14)}}
	}
	return buffer
}

func attackContactState(t *testing.T, input attackContactSnapshot) AttackContactState {
	t.Helper()
	state := AttackContactState{Actors: make(map[uint16]AttackContactActor, len(input.Actors)),
		Attack: attackContactBuffer(t, input.AttackCount, input.AttackBytes), Hurt: attackContactBuffer(t, input.HurtCount, input.HurtBytes),
		PendingSound: input.PendingSound, SlowTicks: input.SlowTicks, ComboFlags: input.ComboFlags,
		ArenaLeft: int16(input.ArenaLeft), ArenaRight: int16(input.ArenaRight)}
	for id, encoded := range input.Actors {
		state.Actors[id] = attackContactActor(attackContactBytes(t, encoded, 256))
	}
	var raw []byte
	state.Effects, raw = decodeEffectPool(t, input.Cursor, input.Effects)
	for i := range state.EffectParameters {
		state.EffectParameters[i] = binary.BigEndian.Uint16(raw[i*128+0x3c:])
	}
	return state
}

func TestAttackContactsOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"attack-contacts-oracle", "attack-contacts-boundaries"} {
		t.Run(name, func(t *testing.T) {
			compressed, err := os.ReadFile("testdata/" + name + ".json.gz")
			if err != nil {
				t.Fatal(err)
			}
			z, err := gzip.NewReader(bytes.NewReader(compressed))
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			raw, err := io.ReadAll(z)
			if err != nil {
				t.Fatal(err)
			}
			manifestRaw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var m struct {
				Pass     bool
				Baseline bool `json:"frames_match_original_core"`
				Records  int
				Contacts int
				Guards   int
				Hits     int
				KO       int    `json:"knockouts"`
				SHA      string `json:"fixture_sha256"`
				GzipSHA  string `json:"gzip_sha256"`
			}
			if err = json.Unmarshal(manifestRaw, &m); err != nil {
				t.Fatal(err)
			}
			h, zh := sha256.Sum256(raw), sha256.Sum256(compressed)
			if !m.Pass || !m.Baseline || m.Records == 0 || m.SHA != hex.EncodeToString(h[:]) || m.GzipSHA != hex.EncodeToString(zh[:]) {
				t.Fatal("unverified original attack-contact fixture")
			}
			var rows []struct {
				Name          string
				Before, After attackContactSnapshot
				Contacts      int
				Guards        int
				Hits          int
				Knockouts     int
			}
			if err = json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			if len(rows) != m.Records {
				t.Fatal("missing original complete calls")
			}
			contacts, guards, hits, knockouts := 0, 0, 0, 0
			for i, r := range rows {
				input := attackContactState(t, r.Before)
				frozen := attackContactState(t, r.Before)
				want := attackContactState(t, r.After)
				got, err := ResolveAttackContacts(input)
				if err != nil || !reflect.DeepEqual(got, want) {
					for id, actor := range want.Actors {
						if got.Actors[id] != actor {
							t.Logf("actor %x got %+v want %+v", id, got.Actors[id], actor)
						}
					}
					t.Fatalf("complete call %d %s differs: err=%v", i, r.Name, err)
				}
				if !reflect.DeepEqual(input, frozen) {
					t.Fatal("contact resolution mutated caller's owner map")
				}
				for id, actor := range got.Actors {
					before := attackContactBytes(t, r.Before.Actors[id], 256)
					after := attackContactBytes(t, r.After.Actors[id], 256)
					if !bytes.Equal(encodeAttackContactActor(actor, before), after) {
						t.Fatalf("complete actor %x differs in call %d", id, i)
					}
				}
				poolBefore := attackContactBytes(t, r.Before.Effects, 0x800)
				poolAfter := attackContactBytes(t, r.After.Effects, 0x800)
				pool := encodeEffectPool(got.Effects, poolBefore)
				for slot, parameter := range got.EffectParameters {
					binary.BigEndian.PutUint16(pool[slot*128+0x3c:], parameter)
				}
				if !bytes.Equal(pool, poolAfter) {
					t.Fatalf("complete effect pool differs in call %d", i)
				}
				contacts += r.Contacts
				guards += r.Guards
				hits += r.Hits
				knockouts += r.Knockouts
			}
			if contacts != m.Contacts || guards != m.Guards || hits != m.Hits || knockouts != m.KO {
				t.Fatal("missing contact branches")
			}
			if name == "attack-contacts-boundaries" && (contacts == 0 || guards == 0 || hits == 0 || knockouts == 0) {
				t.Fatal("isolated matrix omitted guard/hit/KO")
			}
			t.Logf("complete calls=%d contacts=%d guard=%d hit=%d KO=%d", len(rows), contacts, guards, hits, knockouts)
		})
	}
}

func TestAttackContactsMalformedInputsAreAtomic(t *testing.T) {
	base := AttackContactState{Actors: map[uint16]AttackContactActor{1: {Team: 1}, 2: {Team: 2}},
		Attack: AnimationShapeBuffer{Offset: 16}, Hurt: AnimationShapeBuffer{Offset: 16}, PendingSound: 91}
	base.Attack.Records[0].Owner, base.Hurt.Records[0].Owner = 1, 2
	cases := []AttackContactState{}
	for _, offset := range []uint16{1, 15, 257, 0xffff} {
		state := base
		state.Attack.Offset = offset
		cases = append(cases, state)
		state = base
		state.Hurt.Offset = offset
		cases = append(cases, state)
	}
	state := base
	state.Hurt.Records[0].Owner = 3
	cases = append(cases, state)
	state = base
	state.Effects.Cursor = 1
	cases = append(cases, state)
	state = base
	state.Actors = map[uint16]AttackContactActor{1: {Team: 1, TargetMask: 4, Reaction: 11}, 2: {Team: 2, TargetBit: 2}}
	cases = append(cases, state)
	state = base
	state.Actors = map[uint16]AttackContactActor{1: {Team: 1, TargetMask: 4, Reaction: 0, AttackValue: 1}, 2: {Team: 2, TargetBit: 2, CharacterID: 11}}
	cases = append(cases, state)
	for _, input := range cases {
		frozen, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ResolveAttackContacts(input)
		after, e := json.Marshal(input)
		if err == nil || e != nil || !reflect.DeepEqual(got, input) || !bytes.Equal(frozen, after) {
			t.Fatal("malformed attack-contact state was accepted or partially changed")
		}
	}
}
