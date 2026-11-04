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

type chuAISnapshot struct {
	Actor, World string
	Seed         uint32
	Difficulty   uint16
	Left, Right  uint16
}

func chuAIBytes(t *testing.T, encoded string, size int) []byte {
	t.Helper()
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b) != size {
		t.Fatal("invalid original AI bytes", err, len(b), size)
	}
	return b
}

func decodeChuAI(t *testing.T, raw chuAISnapshot, id uint16) ChuAIState {
	t.Helper()
	a, world := chuAIBytes(t, raw.Actor, 256), chuAIBytes(t, raw.World, 4096)
	if id < 0x8400 || id > 0x9000 || id&0x3ff != 0 || !bytes.Equal(a, world[int(id)-0x8400:int(id)-0x8300]) {
		t.Fatal("source actor is not the captured world actor")
	}
	s := ChuAIState{ObjectID: id, RandomSeed: raw.Seed, Difficulty: raw.Difficulty, ArenaLeft: int16(raw.Left), ArenaRight: int16(raw.Right)}
	s.Actor = ChuAIActor{CharacterID: a[0x58], Facing: a[0x61], Pose: a[0x5f], Energy: int16(binary.BigEndian.Uint16(a[0xb8:])), Request: a[0x6b],
		Behavior: a[0xce], TargetCursor: binary.BigEndian.Uint16(a[0xd0:]), Countdown: binary.BigEndian.Uint16(a[0xd2:]), Elapsed: binary.BigEndian.Uint16(a[0xd4:]), FollowTarget: binary.BigEndian.Uint16(a[0xd6:]),
		Controls: FighterControls{Held: Buttons(a[0x71]) | Buttons(a[0xc0])<<8, Pressed: Buttons(a[0x72]) | Buttons(a[0xc1])<<8,
			Changed: Buttons(a[0x73]) | Buttons(a[0xc2])<<8, RawHeld: a[0x74], RetainTicks: a[0x75]}}
	for i := range s.Fighters {
		b, child := world[i*0x400:], world[i*0x400+0x100:]
		w := func(p []byte, off int) int16 { return int16(binary.BigEndian.Uint16(p[off:])) }
		s.Fighters[i] = ChuAIFighter{Size: binary.BigEndian.Uint16(b[4:]), Team: b[0x6e], Health: w(b, 0x78), X: w(b, 0x40), Y: w(b, 0x44), Ground: w(b, 0x9c),
			FrameFlags: b[0x5c], AttackFlags: b[0x64], ChildX: w(child, 0x40), ChildY: w(child, 0x44), ChildGround: w(child, 0x9c), ChildAttackFlags: child[0x64]}
	}
	return s
}

func encodeChuAI(t *testing.T, s ChuAIState, previous chuAISnapshot) chuAISnapshot {
	t.Helper()
	raw := previous
	world := chuAIBytes(t, previous.World, 4096)
	for i, f := range s.Fighters {
		b, child := world[i*0x400:], world[i*0x400+0x100:]
		put := func(p []byte, off int, value int16) { binary.BigEndian.PutUint16(p[off:], uint16(value)) }
		binary.BigEndian.PutUint16(b[4:], f.Size)
		b[0x6e], b[0x5c], b[0x64], child[0x64] = f.Team, f.FrameFlags, f.AttackFlags, f.ChildAttackFlags
		put(b, 0x78, f.Health)
		put(b, 0x40, f.X)
		put(b, 0x44, f.Y)
		put(b, 0x9c, f.Ground)
		put(child, 0x40, f.ChildX)
		put(child, 0x44, f.ChildY)
		put(child, 0x9c, f.ChildGround)
	}
	a := world[int(s.ObjectID)-0x8400 : int(s.ObjectID)-0x8300]
	f := s.Actor
	a[0x58], a[0x61], a[0x5f], a[0x6b], a[0xce] = f.CharacterID, f.Facing, f.Pose, f.Request, f.Behavior
	binary.BigEndian.PutUint16(a[0xb8:], uint16(f.Energy))
	binary.BigEndian.PutUint16(a[0xd0:], f.TargetCursor)
	binary.BigEndian.PutUint16(a[0xd2:], f.Countdown)
	binary.BigEndian.PutUint16(a[0xd4:], f.Elapsed)
	binary.BigEndian.PutUint16(a[0xd6:], f.FollowTarget)
	a[0x71], a[0xc0], a[0x72], a[0xc1], a[0x73], a[0xc2], a[0x74], a[0x75] = uint8(f.Controls.Held), uint8(f.Controls.Held>>8), uint8(f.Controls.Pressed), uint8(f.Controls.Pressed>>8), uint8(f.Controls.Changed), uint8(f.Controls.Changed>>8), f.Controls.RawHeld, f.Controls.RetainTicks
	raw.Actor, raw.World, raw.Seed = hex.EncodeToString(a), hex.EncodeToString(world), s.RandomSeed
	raw.Difficulty, raw.Left, raw.Right = s.Difficulty, uint16(s.ArenaLeft), uint16(s.ArenaRight)
	return raw
}

type chuAIOriginalCall struct {
	Name          string
	ObjectID      uint16 `json:"object_id"`
	Before, After chuAISnapshot
}

type chuAIOriginalFixture struct {
	Tables map[string][4]uint16
	Calls  json.RawMessage
}

func readChuAI(t *testing.T, name string) (chuAIOriginalFixture, int) {
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
		Baseline bool           `json:"frames_match_original_core"`
		SHA      string         `json:"fixture_sha256"`
		Gzip     string         `json:"gzip_sha256"`
		Paths    map[string]int `json:"paths"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified complete original Chu AI fixture")
	}
	if name == "chu-ai-boundaries" {
		for _, pc := range []string{"0x1d2fa", "0x1d312", "0x1cf4c", "0x1cf78", "0x1cfa4", "0x1cfd2", "0x1d000", "0x1d032", "0x1d074", "0x1d0b6", "0x1d0cc", "0x1d0e2", "0x1d114", "0x1d14c", "0x1d186", "0x1d1cc", "0x1d212", "0x1e1e4", "0x1e562", "0x1e5be", "0x1e5d0", "0x1e5fc", "0x1e60e", "0x1e64c", "0x1e65e", "0x1e670", "0x1e682", "0x1e6ee", "0x1e748", "0x1e766", "0x1e7a2", "0x1e7fc", "0x1e836"} {
			if m.Paths[pc] == 0 {
				t.Fatal("missing reached original AI behavior/decision", pc)
			}
		}
	}
	var f chuAIOriginalFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if f.Tables["period_scales"] != chuAIPeriodScales || f.Tables["decision_thresholds"] != chuAIDecisionThresholds || f.Tables["reaction_thresholds"] != chuAIReactionThresholds || f.Tables["lane_masks"] != chuAILaneMasks {
		t.Fatal("immutable original AI table differs")
	}
	return f, m.Records
}

// Complete original CPU returns and these tests precede chu_ai.go.
func TestChuAIOriginalCompleteCalls(t *testing.T) {
	for _, name := range []string{"chu-ai-oracle", "chu-ai-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, count := readChuAI(t, name)
			var rows []chuAIOriginalCall
			if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count {
				t.Fatal("missing original AI calls", err)
			}
			for i, row := range rows {
				input := decodeChuAI(t, row.Before, row.ObjectID)
				copy := input
				got, err := StepChuAI(input)
				want := decodeChuAI(t, row.After, row.ObjectID)
				if err != nil || got != want || !reflect.DeepEqual(encodeChuAI(t, got, row.Before), row.After) {
					t.Fatalf("AI %d %s differs: %v\ngot %+v\nwant %+v", i, row.Name, err, got.Actor, want.Actor)
				}
				if input != copy {
					t.Fatal("AI mutated caller-owned state")
				}
			}
			t.Logf("complete original Chu AI calls=%d", count)
		})
	}
}

func TestChuAISingleInitialState(t *testing.T) {
	f, count := readChuAI(t, "chu-ai-sequences")
	var streams []struct {
		Initial chuAISnapshot
		Calls   []chuAIOriginalCall
		Xs      []int16 `json:"external_target_x"`
	}
	if err := json.Unmarshal(f.Calls, &streams); err != nil || len(streams) != count || count != 17 {
		t.Fatal("missing original AI streams", err)
	}
	steps := 0
	for _, stream := range streams {
		if len(stream.Calls) == 0 || len(stream.Calls) != len(stream.Xs) {
			t.Fatal("missing external target input")
		}
		state := decodeChuAI(t, stream.Initial, stream.Calls[0].ObjectID)
		encoded := stream.Initial
		for i, call := range stream.Calls {
			state.Fighters[0].X = stream.Xs[i] // Only the declared world input changes.
			var err error
			state, err = StepChuAI(state)
			encoded = encodeChuAI(t, state, encoded)
			if err != nil || state != decodeChuAI(t, call.After, call.ObjectID) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("AI stream step %d differs: %v", i, err)
			}
			steps++
		}
	}
	if steps != 408 {
		t.Fatal("missing single-initial-state AI steps")
	}
}

func TestChuAIInvalidStateIsAtomic(t *testing.T) {
	base := ChuAIState{ObjectID: 0x9000, Difficulty: 6, Actor: ChuAIActor{CharacterID: 9, Behavior: 2, Countdown: 10, FollowTarget: 0x8400}}
	base.Fighters[0] = ChuAIFighter{Size: 256, Team: 1, Health: 223}
	base.Fighters[3] = ChuAIFighter{Size: 256, Team: 0, Health: 223}
	variants := []ChuAIState{base, base, base, base}
	variants[0].ObjectID = 0x8500
	variants[1].Difficulty = 1
	variants[2].Actor.CharacterID = 0
	variants[3].Actor.TargetCursor = 1
	for _, input := range variants {
		got, err := StepChuAI(input)
		if err == nil || got != input {
			t.Fatal("malformed AI state changed output")
		}
	}
	// A valid target is chosen before the invalid behavior is consumed; failure
	// must still roll back elapsed/countdown/cursor and shared RNG changes.
	base.Actor.Behavior, base.Actor.TargetCursor, base.Actor.Energy, base.RandomSeed = 255, 6, 100, 1
	got, err := StepChuAI(base)
	if err == nil || got != base {
		t.Fatal("late invalid behavior leaked partial AI state")
	}
	base.Actor.Behavior, base.Actor.Elapsed, base.Actor.FollowTarget = 16, 0, 0x8500
	got, err = StepChuAI(base)
	if err == nil || got != base {
		t.Fatal("invalid follow target leaked partial AI state")
	}
}
