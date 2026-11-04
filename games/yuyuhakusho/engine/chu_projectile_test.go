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

type chuProjectileSnapshot struct {
	ActorBytes  string `json:"actor_bytes"`
	ParentBytes string `json:"parent_bytes"`
	Pool        struct {
		Cursor uint16
		Bytes  string
	}
	Buffers      string
	Offsets      [4]uint16
	PendingSound uint8    `json:"pending_sound"`
	SoundQueue   [4]uint8 `json:"sound_queue"`
	Environment  ChuProjectileEnvironment
}

func decodeChuProjectile(t *testing.T, raw chuProjectileSnapshot, id uint16) ChuProjectileState {
	t.Helper()
	b := attackContactBytes(t, raw.ActorBytes, 256)
	w := func(i int) uint16 { return binary.BigEndian.Uint16(b[i:]) }
	l := func(i int) uint32 { return binary.BigEndian.Uint32(b[i:]) }
	s := ChuProjectileState{Behavior: l(0), Size: w(4), Stage: w(6), GraphicSet: l(0x1c), TileBase: l(0x20),
		VelocityX: int32(l(0x10)), VerticalVelocity: int32(l(0x14)), Ground: int32(l(0x9c)), Power: w(0x7c), TrailPhase: b[0x48],
		Health: int16(w(0x78)), Shake: b[0x6a], ContactFlags: b[0x89], Layer: b[0x26], Depth: w(0x3e), Team: b[0x6e], TargetBit: b[0x70],
		ScreenX: int16(w(8)), ScreenY: int16(w(12)), ScreenOffsetX: int16(w(0x54)), ScreenOffsetY: int16(w(0x56)),
		PendingSound: raw.PendingSound, SoundQueue: raw.SoundQueue}
	s.Animation = AnimationActorState{ObjectID: id, AnimationSet: l(0x50), ShapeSet: l(0x8c), Pose: b[0x5f],
		Clock:  AnimationClock{FrameIndex: b[0x59], Duration: b[0x5a], Ticks: b[0x5b], Flags: b[0x5c], Completed: b[0x60]},
		Facing: b[0x61], ActionFlags: b[0x6d], EnabledStages: b[0x6c], Attributes: w(0x24), GraphicOffsetX: int16(w(0x28)), GraphicOffsetY: int16(w(0x2a)),
		WorldX: int32(l(0x40)), WorldY: int32(l(0x44)), GraphicParameter: w(0x3c), Auxiliary94: l(0x94), Auxiliary64: b[0x64],
		Value65: b[0x65], Value66: b[0x66], Value67: b[0x67], Value68: b[0x68]}
	for i := range s.GraphicCache {
		s.GraphicCache[i] = l(0x2c + i*4)
	}
	s.Pool, _ = decodeEffectPool(t, raw.Pool.Cursor, raw.Pool.Bytes)
	buffer := attackContactBytes(t, raw.Buffers, 1024)
	for i := range s.Buffers {
		s.Buffers[i] = attackContactBuffer(t, raw.Offsets[i], hex.EncodeToString(buffer[i*256:(i+1)*256]))
	}
	return s
}

func encodeChuProjectile(t *testing.T, s ChuProjectileState, outcome ChuProjectileOutcome, old chuProjectileSnapshot) chuProjectileSnapshot {
	t.Helper()
	raw := old
	b := attackContactBytes(t, old.ActorBytes, 256)
	if outcome.Deleted {
		if outcome.ClearedBytes == 0 || outcome.ClearedBytes > 256 || outcome.ClearedBytes&15 != 0 {
			t.Fatal("invalid declared deletion range")
		}
		clear(b[:outcome.ClearedBytes])
	}
	w := func(i int, v uint16) { binary.BigEndian.PutUint16(b[i:], v) }
	l := func(i int, v uint32) { binary.BigEndian.PutUint32(b[i:], v) }
	l(0, s.Behavior)
	w(4, s.Size)
	w(6, s.Stage)
	w(8, uint16(s.ScreenX))
	w(12, uint16(s.ScreenY))
	l(0x10, uint32(s.VelocityX))
	l(0x14, uint32(s.VerticalVelocity))
	l(0x1c, s.GraphicSet)
	l(0x20, s.TileBase)
	a := s.Animation
	w(0x24, a.Attributes)
	b[0x26] = s.Layer
	w(0x28, uint16(a.GraphicOffsetX))
	w(0x2a, uint16(a.GraphicOffsetY))
	for i, v := range s.GraphicCache {
		l(0x2c+i*4, v)
	}
	w(0x3c, a.GraphicParameter)
	w(0x3e, s.Depth)
	l(0x40, uint32(a.WorldX))
	l(0x44, uint32(a.WorldY))
	b[0x48] = s.TrailPhase
	l(0x50, a.AnimationSet)
	w(0x54, uint16(s.ScreenOffsetX))
	w(0x56, uint16(s.ScreenOffsetY))
	b[0x59], b[0x5a], b[0x5b], b[0x5c], b[0x5f], b[0x60] = a.Clock.FrameIndex, a.Clock.Duration, a.Clock.Ticks, a.Clock.Flags, a.Pose, a.Clock.Completed
	b[0x61], b[0x64], b[0x65], b[0x66], b[0x67], b[0x68], b[0x6a], b[0x6c], b[0x6d] = a.Facing, a.Auxiliary64, a.Value65, a.Value66, a.Value67, a.Value68, s.Shake, a.EnabledStages, a.ActionFlags
	b[0x6e], b[0x70] = s.Team, s.TargetBit
	w(0x78, uint16(s.Health))
	w(0x7c, s.Power)
	b[0x89] = s.ContactFlags
	l(0x8c, a.ShapeSet)
	l(0x94, a.Auxiliary94)
	l(0x9c, uint32(s.Ground))
	raw.ActorBytes = hex.EncodeToString(b)
	raw.Pool.Cursor = s.Pool.Cursor
	raw.Pool.Bytes = hex.EncodeToString(encodeEffectPool(s.Pool, attackContactBytes(t, old.Pool.Bytes, 2048)))
	buffer := make([]byte, 1024)
	for g, group := range s.Buffers {
		raw.Offsets[g] = group.Offset
		for i, r := range group.Records {
			v := buffer[g*256+i*16:]
			binary.BigEndian.PutUint16(v, uint16(r.X))
			binary.BigEndian.PutUint16(v[4:], uint16(r.Y))
			v[8], v[9] = r.Width, r.Height
			binary.BigEndian.PutUint16(v[12:], r.Owner)
			for j, offset := range []int{2, 6, 10, 14} {
				binary.BigEndian.PutUint16(v[offset:], r.Preserved[j])
			}
		}
	}
	raw.Buffers = hex.EncodeToString(buffer)
	raw.PendingSound, raw.SoundQueue = s.PendingSound, s.SoundQueue
	return raw
}

type chuProjectileCall struct {
	Name          string
	ObjectID      uint16 `json:"object_id"`
	Before, After chuProjectileSnapshot
	Draw, Deleted bool
	ClearedBytes  uint16 `json:"cleared_bytes"`
	External      struct {
		ContactFlags *uint8 `json:"contact_flags"`
	} `json:"external_input"`
}

type chuProjectileFixture struct {
	Library []struct {
		Key  AnimationFrameKey
		Data AnimationRecordData
	}
	Calls json.RawMessage
}

func readChuProjectile(t *testing.T, name string) (chuProjectileFixture, AnimationLibrary, int, int) {
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
		Calls    int
		Frames   int    `json:"library_frames"`
		External int    `json:"external_queue_calls"`
		Baseline bool   `json:"frames_match_original_core"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.External != 0 || m.Records == 0 || m.Frames != 14 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original projectile rules fixture")
	}
	var f chuProjectileFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	library := make(AnimationLibrary, len(f.Library))
	for _, frame := range f.Library {
		if _, exists := library[frame.Key]; exists {
			t.Fatal("duplicate projectile frame")
		}
		library[frame.Key] = frame.Data
	}
	if len(library) != 14 {
		t.Fatal("missing projectile pose frames")
	}
	return f, library, m.Records, m.Calls
}

// Full original rules fixtures and these tests precede chu_projectile.go.
func TestChuProjectileOriginalRules(t *testing.T) {
	for _, name := range []string{"chu-projectile-oracle", "chu-projectile-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, library, count, _ := readChuProjectile(t, name)
			var rows []chuProjectileCall
			if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count {
				t.Fatal("missing original projectile rules calls", err)
			}
			beforeLibrary, _ := json.Marshal(f.Library)
			deleted := 0
			for i, r := range rows {
				input := decodeChuProjectile(t, r.Before, r.ObjectID)
				copy := decodeChuProjectile(t, r.Before, r.ObjectID)
				got, result, err := StepChuProjectile(input, r.Before.Environment, library)
				wantOutcome := ChuProjectileOutcome{Draw: r.Draw, Deleted: r.Deleted, ClearedBytes: r.ClearedBytes}
				if err != nil || result != wantOutcome || !reflect.DeepEqual(got, decodeChuProjectile(t, r.After, r.ObjectID)) || !reflect.DeepEqual(encodeChuProjectile(t, got, result, r.Before), r.After) {
					t.Fatalf("projectile %d %s differs: err=%v result=%+v/%+v", i, r.Name, err, result, wantOutcome)
				}
				if !reflect.DeepEqual(input, copy) {
					t.Fatal("projectile changed input")
				}
				if r.Deleted {
					deleted++
				}
			}
			for i := range f.Library {
				f.Library[i].Data = library[f.Library[i].Key]
			}
			afterLibrary, _ := json.Marshal(f.Library)
			if !bytes.Equal(beforeLibrary, afterLibrary) || deleted == 0 {
				t.Fatal("missing deletion or changed immutable projectile data")
			}
			t.Logf("projectile complete rules=%d deletions=%d", count, deleted)
		})
	}
}

func TestChuProjectileSingleInitialState(t *testing.T) {
	f, library, count, calls := readChuProjectile(t, "chu-projectile-sequences")
	var rows []struct {
		Initial chuProjectileSnapshot
		Calls   []chuProjectileCall
	}
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count || count != 18 {
		t.Fatal("missing projectile trajectories", err)
	}
	steps := 0
	for _, r := range rows {
		if len(r.Calls) == 0 || !r.Calls[len(r.Calls)-1].Deleted {
			t.Fatal("trajectory does not end at actual deletion")
		}
		s := decodeChuProjectile(t, r.Initial, r.Calls[0].ObjectID)
		encoded := r.Initial
		for i, call := range r.Calls {
			if call.External.ContactFlags != nil {
				s.ContactFlags = *call.External.ContactFlags
			}
			var result ChuProjectileOutcome
			var err error
			s, result, err = StepChuProjectile(s, r.Initial.Environment, library)
			encoded = encodeChuProjectile(t, s, result, encoded)
			if err != nil || result != (ChuProjectileOutcome{Draw: call.Draw, Deleted: call.Deleted, ClearedBytes: call.ClearedBytes}) || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(s, decodeChuProjectile(t, call.After, call.ObjectID)) {
				t.Fatalf("projectile trajectory step %d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != calls || steps != 948 {
		t.Fatal("missing projectile trajectory steps")
	}
}

func TestChuProjectileErrorsAreAtomic(t *testing.T) {
	f, library, _, _ := readChuProjectile(t, "chu-projectile-boundaries")
	var rows []chuProjectileCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	base := decodeChuProjectile(t, rows[0].Before, rows[0].ObjectID)
	env := rows[0].Before.Environment
	base.Stage, base.Power, base.ContactFlags = 2, 0, 0
	base.Animation.WorldX, base.Animation.WorldY, base.VerticalVelocity = 100<<16, 260<<16, -1
	got, result, err := StepChuProjectile(base, env, nil)
	if err == nil || !reflect.DeepEqual(got, base) || result != (ChuProjectileOutcome{}) {
		t.Fatal("missing animation data leaked earlier motion")
	}
	base.Power, base.TrailPhase, base.ContactFlags = 8, 3, 8
	got, result, err = StepChuProjectile(base, env, library)
	if err == nil || !reflect.DeepEqual(got, base) || result != (ChuProjectileOutcome{}) {
		t.Fatal("unsupported impact table leaked trail allocation")
	}
	base.Power, base.Size = 0, 1
	got, result, err = StepChuProjectile(base, env, library)
	if err == nil || !reflect.DeepEqual(got, base) || result != (ChuProjectileOutcome{}) {
		t.Fatal("corrupt deletion size was silently emulated or partially applied")
	}
	env.PauseFlags = 0x80
	base.Stage = 0
	got, result, err = StepChuProjectile(base, env, nil)
	if err != nil || !result.Draw || result.Deleted || got.Stage != 2 || got.Health != 255 {
		t.Fatal("paused initialization incorrectly requires animation data")
	}
}
