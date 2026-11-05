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

type chuTailSnapshot struct {
	Body       FighterBodyState
	ActorBytes string            `json:"actor_bytes"`
	ChildBytes string            `json:"child_bytes"`
	Others     map[uint16]string `json:"other_actors"`
	Pool       struct {
		Cursor uint16
		Bytes  string
	}
	Occupied     uint8
	Buffers      string
	Offsets      [4]uint16
	Throws       string
	ThrowOffset  uint16   `json:"throw_offset"`
	PendingSound uint8    `json:"pending_sound"`
	SoundQueue   [4]uint8 `json:"sound_queue"`
	RandomSeed   uint32   `json:"random_seed"`
	BattleState  int16    `json:"battle_state"`
	Parameters   ChuTailParameters
}

func decodeChuTail(t *testing.T, raw chuTailSnapshot, id uint16) ChuTailState {
	t.Helper()
	a := attackContactBytes(t, raw.ActorBytes, 256)
	s := ChuTailState{Body: raw.Body, ObjectID: id, OtherFighters: make(map[uint16]TailFighterEnergy),
		Energy: int16(binary.BigEndian.Uint16(a[0xb8:])), ChildSize: binary.BigEndian.Uint16(attackContactBytes(t, raw.ChildBytes, 256)[4:]),
		Controls: FighterControls{Held: Buttons(a[0x71]) | Buttons(a[0xc0])<<8, Pressed: Buttons(a[0x72]) | Buttons(a[0xc1])<<8,
			Changed: Buttons(a[0x73]) | Buttons(a[0xc2])<<8, RawHeld: a[0x74], RetainTicks: a[0x75]},
		ChargeTicks: a[0xc7], GuardMask: a[0x63], ContactFlags: a[0x89], AttackValue: a[0x65], StunDamage: a[0x66], EffectKind: a[0x67], Reaction: a[0x68],
		TargetMask: binary.BigEndian.Uint32(a[0x94:]), AnimationSet: binary.BigEndian.Uint32(a[0x50:]), ShapeSet: binary.BigEndian.Uint32(a[0x8c:]),
		GraphicOffsetX: int16(binary.BigEndian.Uint16(a[0x28:])), GraphicOffsetY: int16(binary.BigEndian.Uint16(a[0x2a:])), GraphicParameter: binary.BigEndian.Uint16(a[0x3c:]),
		AttackScale: binary.BigEndian.Uint16(a[0xc4:]), OccupiedLanes: raw.Occupied, PendingSound: raw.PendingSound, SoundQueue: raw.SoundQueue,
		RandomSeed: raw.RandomSeed, BattleState: raw.BattleState}
	s.Pool, _ = decodeEffectPool(t, raw.Pool.Cursor, raw.Pool.Bytes)
	for owner, encoded := range raw.Others {
		b := attackContactBytes(t, encoded, 256)
		s.OtherFighters[owner] = TailFighterEnergy{Size: binary.BigEndian.Uint16(b[4:]), Team: b[0x6e], Health: int16(binary.BigEndian.Uint16(b[0x78:])), Energy: int16(binary.BigEndian.Uint16(b[0xb8:])), HitStopTicks: b[0x69]}
	}
	b := attackContactBytes(t, raw.Buffers, 1024)
	for i := range s.Buffers {
		s.Buffers[i] = attackContactBuffer(t, raw.Offsets[i], hex.EncodeToString(b[i*256:(i+1)*256]))
	}
	s.Throws = decodeThrowSnapshot(t, throwOriginalSnapshot{GrabOffset: raw.ThrowOffset, Grabs: raw.Throws, Hurts: hex.EncodeToString(make([]byte, 256))}).Throws
	return s
}

func encodeChuTail(t *testing.T, s ChuTailState, old chuTailSnapshot) chuTailSnapshot {
	t.Helper()
	raw := old
	raw.Body = s.Body
	a := encodeJumpBody(s.Body, attackContactBytes(t, old.ActorBytes, 256))
	binary.BigEndian.PutUint16(a[0xb8:], uint16(s.Energy))
	a[0x71], a[0xc0], a[0x72], a[0xc1], a[0x73], a[0xc2], a[0x74], a[0x75] = uint8(s.Controls.Held), uint8(s.Controls.Held>>8), uint8(s.Controls.Pressed), uint8(s.Controls.Pressed>>8), uint8(s.Controls.Changed), uint8(s.Controls.Changed>>8), s.Controls.RawHeld, s.Controls.RetainTicks
	a[0xc7], a[0x63], a[0x89], a[0x65], a[0x66], a[0x67], a[0x68] = s.ChargeTicks, s.GuardMask, s.ContactFlags, s.AttackValue, s.StunDamage, s.EffectKind, s.Reaction
	binary.BigEndian.PutUint32(a[0x94:], s.TargetMask)
	binary.BigEndian.PutUint32(a[0x50:], s.AnimationSet)
	binary.BigEndian.PutUint32(a[0x8c:], s.ShapeSet)
	binary.BigEndian.PutUint16(a[0x28:], uint16(s.GraphicOffsetX))
	binary.BigEndian.PutUint16(a[0x2a:], uint16(s.GraphicOffsetY))
	binary.BigEndian.PutUint16(a[0x3c:], s.GraphicParameter)
	binary.BigEndian.PutUint16(a[0xc4:], s.AttackScale)
	raw.ActorBytes = hex.EncodeToString(a)
	raw.Others = make(map[uint16]string, len(s.OtherFighters))
	for id, f := range s.OtherFighters {
		b := attackContactBytes(t, old.Others[id], 256)
		binary.BigEndian.PutUint16(b[4:], f.Size)
		b[0x6e] = f.Team
		b[0x69] = f.HitStopTicks
		binary.BigEndian.PutUint16(b[0x78:], uint16(f.Health))
		binary.BigEndian.PutUint16(b[0xb8:], uint16(f.Energy))
		raw.Others[id] = hex.EncodeToString(b)
	}
	b := make([]byte, 1024)
	for group, buffer := range s.Buffers {
		raw.Offsets[group] = buffer.Offset
		for i, r := range buffer.Records {
			v := b[group*256+i*16:]
			binary.BigEndian.PutUint16(v, uint16(r.X))
			binary.BigEndian.PutUint16(v[4:], uint16(r.Y))
			v[8], v[9] = r.Width, r.Height
			binary.BigEndian.PutUint16(v[12:], r.Owner)
			for j, off := range []int{2, 6, 10, 14} {
				binary.BigEndian.PutUint16(v[off:], r.Preserved[j])
			}
		}
	}
	raw.Buffers = hex.EncodeToString(b)
	raw.Throws = encodeThrowSnapshot(t, ThrowContactState{Throws: s.Throws}, throwOriginalSnapshot{Grabs: old.Throws, Hurts: hex.EncodeToString(make([]byte, 256))}).Grabs
	raw.ThrowOffset = s.Throws.Offset
	raw.Pool.Cursor = s.Pool.Cursor
	raw.Pool.Bytes = hex.EncodeToString(encodeEffectPool(s.Pool, attackContactBytes(t, old.Pool.Bytes, 2048)))
	raw.Occupied, raw.PendingSound, raw.SoundQueue, raw.RandomSeed, raw.BattleState = s.OccupiedLanes, s.PendingSound, s.SoundQueue, s.RandomSeed, s.BattleState
	return raw
}

type chuTailCall struct {
	Name, Mode    string
	ObjectID      uint16 `json:"object_id"`
	Before, After chuTailSnapshot
	Clears        []struct {
		PC      string
		Address string
		Before  uint8
		SR, SP  uint32
	} `json:"external_queue_clears"`
}

type chuTailFixture struct {
	Library []struct {
		Key  AnimationFrameKey
		Data AnimationRecordData
	}
	Calls json.RawMessage
}

func readChuTail(t *testing.T, name string) (chuTailFixture, AnimationLibrary, int, int) {
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
		Pass     bool
		Records  int
		External int    `json:"external_queue_calls"`
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original full Chu tail fixture")
	}
	var f chuTailFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	library := make(AnimationLibrary, len(f.Library))
	for _, frame := range f.Library {
		if _, exists := library[frame.Key]; exists {
			t.Fatal("duplicate immutable tail frame")
		}
		library[frame.Key] = frame.Data
	}
	return f, library, m.Records, m.External
}

func tailMode(t *testing.T, name string) ChuTailMode {
	t.Helper()
	if name == "normal" {
		return ChuTailNormal
	}
	if name == "hit-stop" {
		return ChuTailHitStop
	}
	t.Fatal("unknown original tail entry", name)
	return 0
}

// Full-return original fixtures and these tests precede chu_tail.go.
func TestChuTailOriginalFullCalls(t *testing.T) {
	for _, name := range []string{"chu-tail-oracle", "chu-tail-boundaries"} {
		t.Run(name, func(t *testing.T) {
			fixture, library, count, externalCount := readChuTail(t, name)
			var rows []chuTailCall
			if err := json.Unmarshal(fixture.Calls, &rows); err != nil || len(rows) != count {
				t.Fatal("missing original full tails", err)
			}
			beforeLibrary, _ := json.Marshal(fixture.Library)
			external, queueDifferences := 0, 0
			for i, row := range rows {
				input := decodeChuTail(t, row.Before, row.ObjectID)
				copy := decodeChuTail(t, row.Before, row.ObjectID)
				got, err := StepChuActionTail(input, row.Before.Parameters, library, tailMode(t, row.Mode))
				want := decodeChuTail(t, row.After, row.ObjectID)
				encoded, expected := encodeChuTail(t, got, row.Before), row.After
				if len(row.Clears) != 0 {
					external++
					for _, c := range row.Clears {
						if c.PC != "0x37c56" && c.PC != "0x37cac" || c.Address < "0xf80a" || c.Address > "0xf80d" || c.SR&0x700 != 0x700 {
							t.Fatal("unidentified external sound write")
						}
					}
					if got.SoundQueue != want.SoundQueue {
						queueDifferences++
					}
					// Only the independently observed consumer's queue is excluded;
					// its changes are never passed into the production function.
					want.SoundQueue, expected.SoundQueue = got.SoundQueue, got.SoundQueue
				}
				if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encoded, expected) {
					t.Fatalf("full Chu tail %d %s/%s differs: %v", i, row.Mode, row.Name, err)
				}
				if !reflect.DeepEqual(input, copy) {
					t.Fatal("tail mutated caller-owned opponent or throw state")
				}
			}
			for i := range fixture.Library {
				fixture.Library[i].Data = library[fixture.Library[i].Key]
			}
			afterLibrary, _ := json.Marshal(fixture.Library)
			if !bytes.Equal(beforeLibrary, afterLibrary) || external != externalCount {
				t.Fatal("immutable library changed or external evidence missing")
			}
			t.Logf("complete tails=%d externally interleaved=%d queue differences=%d", count, external, queueDifferences)
		})
	}
}

func TestChuTailSingleInitialState(t *testing.T) {
	f, library, count, external := readChuTail(t, "chu-tail-sequences")
	var rows []struct {
		Initial chuTailSnapshot
		Calls   []chuTailCall
	}
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count || count != 5 || external != 0 {
		t.Fatal("missing original full-tail streams", err)
	}
	steps := 0
	for _, row := range rows {
		if len(row.Calls) == 0 {
			t.Fatal("empty tail stream")
		}
		state := decodeChuTail(t, row.Initial, row.Calls[0].ObjectID)
		encoded := row.Initial
		for i, call := range row.Calls {
			var err error
			state, err = StepChuActionTail(state, row.Initial.Parameters, library, tailMode(t, call.Mode))
			encoded = encodeChuTail(t, state, encoded)
			if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeChuTail(t, call.After, call.ObjectID)) {
				t.Fatalf("tail stream step %d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != 30 {
		t.Fatal("missing tail steps")
	}
}

func TestChuTailErrorsAreAtomic(t *testing.T) {
	f, library, _, _ := readChuTail(t, "chu-tail-boundaries")
	var rows []chuTailCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	input := decodeChuTail(t, rows[0].Before, rows[0].ObjectID)
	for _, mode := range []ChuTailMode{ChuTailMode(255), ChuTailNormal} {
		got, err := StepChuActionTail(input, rows[0].Before.Parameters, nil, mode)
		if err == nil || !reflect.DeepEqual(got, input) {
			t.Fatal("invalid mode or missing animation data caused partial tail")
		}
	}
	input.Body.EnabledStages, input.Body.Pose, input.BattleState = 2, 0, 0
	input.Controls.Changed = 0x70
	input.OtherFighters = nil
	got, err := StepChuActionTail(input, rows[0].Before.Parameters, library, ChuTailNormal)
	if err == nil || !reflect.DeepEqual(got, input) {
		t.Fatal("missing drained fighter caused partial tail")
	}
}
