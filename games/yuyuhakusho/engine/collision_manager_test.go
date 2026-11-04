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

type collisionManagerSnapshot struct {
	Actors       map[uint16]string
	Buffers      string
	Throws       string
	Counts       [5]uint16
	Effects      string
	Cursor       uint16
	PendingSound uint8    `json:"pending_sound"`
	SoundQueue   [4]uint8 `json:"sound_queue"`
	PauseFlags   uint8    `json:"pause_flags"`
	SlowTicks    uint16   `json:"slow_ticks"`
	SlowFlag     uint8    `json:"slow_flag"`
	ComboFlags   uint8    `json:"combo_flags"`
	ArenaLeft    uint16   `json:"arena_left"`
	ArenaRight   uint16   `json:"arena_right"`
}

func decodeCollisionManager(t *testing.T, raw collisionManagerSnapshot) CollisionManagerState {
	t.Helper()
	s := CollisionManagerState{Actors: make(map[uint16]CollisionActor), PendingSound: raw.PendingSound, SoundQueue: raw.SoundQueue,
		PauseFlags: raw.PauseFlags, SlowTicks: raw.SlowTicks, SlowFlag: raw.SlowFlag, ComboFlags: raw.ComboFlags,
		ArenaLeft: int16(raw.ArenaLeft), ArenaRight: int16(raw.ArenaRight)}
	for id, encoded := range raw.Actors {
		b := attackContactBytes(t, encoded, 256)
		s.Actors[id] = CollisionActor{AttackContactActor: attackContactActor(b), ThrowDelay: b[0xc8]}
	}
	b := attackContactBytes(t, raw.Buffers, 1024)
	for i := range s.Buffers {
		s.Buffers[i] = attackContactBuffer(t, raw.Counts[i], hex.EncodeToString(b[i*256:(i+1)*256]))
	}
	target := decodeThrowSnapshot(t, throwOriginalSnapshot{GrabOffset: raw.Counts[4], Grabs: raw.Throws, Hurts: hex.EncodeToString(b[512:768])})
	s.Throws = target.Throws
	var pool []byte
	s.Effects, pool = decodeEffectPool(t, raw.Cursor, raw.Effects)
	for i := range s.EffectParameters {
		s.EffectParameters[i] = binary.BigEndian.Uint16(pool[i*128+0x3c:])
	}
	return s
}

func encodeCollisionManager(t *testing.T, s CollisionManagerState, previous collisionManagerSnapshot) collisionManagerSnapshot {
	t.Helper()
	raw := previous
	raw.Actors = make(map[uint16]string, len(s.Actors))
	for id, a := range s.Actors {
		b := encodeAttackContactActor(a.AttackContactActor, attackContactBytes(t, previous.Actors[id], 256))
		b[0xc8] = a.ThrowDelay
		raw.Actors[id] = hex.EncodeToString(b)
	}
	b := make([]byte, 1024)
	for group, buffer := range s.Buffers {
		raw.Counts[group] = buffer.Offset
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
	raw.Counts[4] = s.Throws.Offset
	throw := encodeThrowSnapshot(t, ThrowContactState{Throws: s.Throws}, throwOriginalSnapshot{Grabs: previous.Throws, Hurts: hex.EncodeToString(b[512:768])})
	raw.Throws = throw.Grabs
	pool := encodeEffectPool(s.Effects, attackContactBytes(t, previous.Effects, 2048))
	for i, value := range s.EffectParameters {
		binary.BigEndian.PutUint16(pool[i*128+0x3c:], value)
	}
	raw.Effects = hex.EncodeToString(pool)
	raw.Cursor, raw.PendingSound, raw.SoundQueue = s.Effects.Cursor, s.PendingSound, s.SoundQueue
	raw.PauseFlags, raw.SlowTicks, raw.SlowFlag, raw.ComboFlags = s.PauseFlags, s.SlowTicks, s.SlowFlag, s.ComboFlags
	raw.ArenaLeft, raw.ArenaRight = uint16(s.ArenaLeft), uint16(s.ArenaRight)
	return raw
}

func readCollisionManagerFixture(t *testing.T, name string, target any) int {
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
	manifestBytes, err := os.ReadFile("testdata/" + name + "-manifest.json")
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
	if err := json.Unmarshal(manifestBytes, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.External != 0 || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified complete collision-manager fixture")
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return m.Records
}

type collisionManagerCall struct {
	Name          string
	Before, After collisionManagerSnapshot
}

// Original full manager fixtures and these tests precede collision_manager.go.
func TestCollisionManagerOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"collision-manager-oracle", "collision-manager-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []collisionManagerCall
			count := readCollisionManagerFixture(t, name, &rows)
			if len(rows) != count {
				t.Fatal("missing original manager calls")
			}
			for i, row := range rows {
				input := decodeCollisionManager(t, row.Before)
				copy := decodeCollisionManager(t, row.Before)
				got, err := ResolveCollisionManager(input)
				want := decodeCollisionManager(t, row.After)
				if err != nil || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeCollisionManager(t, got, row.Before), row.After) {
					t.Fatalf("manager %d %s complete state differs: %v", i, row.Name, err)
				}
				if !reflect.DeepEqual(input, copy) {
					t.Fatal("manager mutated caller-owned state")
				}
			}
			t.Logf("complete original manager calls=%d", len(rows))
		})
	}
}

func TestCollisionManagerSingleInitialState(t *testing.T) {
	var streams []struct {
		Name    string
		Initial collisionManagerSnapshot
		Calls   []collisionManagerCall
	}
	count := readCollisionManagerFixture(t, "collision-manager-sequences", &streams)
	if len(streams) != count || count != 5 {
		t.Fatal("missing original manager streams")
	}
	steps := 0
	for _, stream := range streams {
		state := decodeCollisionManager(t, stream.Initial)
		encoded := stream.Initial
		for i, call := range stream.Calls {
			var err error
			state, err = ResolveCollisionManager(state)
			encoded = encodeCollisionManager(t, state, encoded)
			if err != nil || !reflect.DeepEqual(encoded, call.After) || !reflect.DeepEqual(state, decodeCollisionManager(t, call.After)) {
				t.Fatalf("manager stream %s step %d differs: %v", stream.Name, i, err)
			}
			steps++
		}
	}
	if steps != 50 {
		t.Fatal("missing continuous manager steps")
	}
}

func TestCollisionManagerPausedAndInvalidStates(t *testing.T) {
	input := CollisionManagerState{PauseFlags: 0x80, SlowTicks: 1, SlowFlag: 255, PendingSound: 0x97, Throws: ThrowContactBuffer{Offset: 1}, Buffers: [4]AnimationShapeBuffer{{Offset: 1}}}
	got, err := ResolveCollisionManager(input)
	if err != nil || !reflect.DeepEqual(got, input) {
		t.Fatal("pause did not bypass all consumers and tail state")
	}
	input.PauseFlags = 0
	got, err = ResolveCollisionManager(input)
	if err == nil || !reflect.DeepEqual(got, input) {
		t.Fatal("invalid early consumer caused partial manager mutation")
	}
	// Body failure occurs after a valid attack would change actor/effect state.
	var rows []collisionManagerCall
	readCollisionManagerFixture(t, "collision-manager-boundaries", &rows)
	for _, row := range rows {
		if row.Before.PauseFlags != 0 || row.Before.Counts[0] == 0 || row.Before.Counts[1] != 0 || row.Before.Counts[4] != 0 {
			continue
		}
		input = decodeCollisionManager(t, row.Before)
		input.Buffers[3].Offset = 16
		input.Buffers[3].Records[0].Owner = 0x1234
		before := encodeCollisionManager(t, input, row.Before)
		got, err = ResolveCollisionManager(input)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(encodeCollisionManager(t, input, row.Before), before) {
			t.Fatal("late consumer failure leaked prior actor/effect mutation")
		}
		return
	}
	t.Fatal("no real original attack case available for atomicity check")
}
