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

type attachmentOriginalSnapshot struct {
	ObjectID     uint16 `json:"object_id"`
	ActorBytes   string `json:"actor_bytes"`
	ParentBytes  string `json:"parent_bytes"`
	Buffers      string
	Offsets      [4]uint16
	PendingSound uint8    `json:"pending_sound"`
	SoundQueue   [4]uint8 `json:"sound_queue"`
	World        ChuAttachmentWorld
}

func attachmentOriginalBytes(t *testing.T, encoded string, count int) []byte {
	t.Helper()
	b, err := hex.DecodeString(encoded)
	if err != nil || len(b) != count {
		t.Fatalf("invalid original attachment bytes: %v %d/%d", err, len(b), count)
	}
	return b
}

func attachmentOriginalKind(t *testing.T, kind string) ChuAttachmentKind {
	t.Helper()
	if kind == "charge" {
		return ChuChargeAttachment
	}
	if kind == "release" {
		return ChuReleaseAttachment
	}
	t.Fatal("unknown original attachment callback", kind)
	return 0
}

func decodeAttachmentOriginal(t *testing.T, kind string, s attachmentOriginalSnapshot) (ChuAttachmentState, ChuAttachmentParent) {
	t.Helper()
	a := attachmentOriginalBytes(t, s.ActorBytes, 256)
	w := func(i int) uint16 { return binary.BigEndian.Uint16(a[i:]) }
	l := func(i int) uint32 { return binary.BigEndian.Uint32(a[i:]) }
	state := ChuAttachmentState{Kind: attachmentOriginalKind(t, kind), Behavior: l(0), Size: w(4), Stage: w(6),
		GraphicsSet: l(0x1c), TilesBase: l(0x20), Power: int16(w(0x7c)), Health: int16(w(0x78)),
		ScreenX: int16(w(8)), ScreenY: int16(w(12)), ScreenOffsetX: int16(w(0x54)), ScreenOffsetY: int16(w(0x56)),
		SourceOffsetX: int16(w(0x10)), SourceOffsetY: int16(w(0x14)), PendingSound: s.PendingSound, SoundQueue: s.SoundQueue,
		Actor: AnimationActorState{ObjectID: s.ObjectID, AnimationSet: l(0x50), ShapeSet: l(0x8c), Pose: a[0x5f],
			Clock:  AnimationClock{FrameIndex: a[0x59], Duration: a[0x5a], Ticks: a[0x5b], Flags: a[0x5c], Completed: a[0x60]},
			Facing: a[0x61], ActionFlags: a[0x6d], EnabledStages: a[0x6c], Attributes: w(0x24),
			GraphicOffsetX: int16(w(0x28)), GraphicOffsetY: int16(w(0x2a)), WorldX: int32(l(0x40)), WorldY: int32(l(0x44)),
			GraphicParameter: w(0x3c), Auxiliary94: l(0x94), Auxiliary64: a[0x64], Value65: a[0x65], Value66: a[0x66], Value67: a[0x67], Value68: a[0x68]}}
	for i := range state.GraphicCache {
		state.GraphicCache[i] = l(0x2c + i*4)
	}
	b := attachmentOriginalBytes(t, s.Buffers, 1024)
	for group := range state.Buffers {
		state.Buffers[group].Offset = s.Offsets[group]
		for i := range state.Buffers[group].Records {
			raw := b[group*256+i*16:]
			rw := func(j int) uint16 { return binary.BigEndian.Uint16(raw[j:]) }
			state.Buffers[group].Records[i] = AnimationShapeRecord{X: int16(rw(0)), Y: int16(rw(4)), Width: raw[8], Height: raw[9], Owner: rw(12),
				Preserved: [4]uint16{rw(2), rw(6), rw(10), rw(14)}}
		}
	}
	p := attachmentOriginalBytes(t, s.ParentBytes, 256)
	parent := ChuAttachmentParent{CharacterID: p[0x58], FrameIndex: p[0x59], Pose: p[0x5f], Power: int16(binary.BigEndian.Uint16(p[0x7c:])),
		X: int32(binary.BigEndian.Uint32(p[0x40:])), Y: int32(binary.BigEndian.Uint32(p[0x44:]))}
	return state, parent
}

func encodeAttachmentOriginal(t *testing.T, state ChuAttachmentState, parent ChuAttachmentParent, world ChuAttachmentWorld, outcome ChuAttachmentOutcome, previous attachmentOriginalSnapshot) attachmentOriginalSnapshot {
	t.Helper()
	s := previous
	a := attachmentOriginalBytes(t, previous.ActorBytes, 256)
	if outcome.ClearedBytes > 256 {
		t.Fatal("clear escaped retained object storage")
	}
	clear(a[:outcome.ClearedBytes])
	w := func(i int, v uint16) { binary.BigEndian.PutUint16(a[i:], v) }
	l := func(i int, v uint32) { binary.BigEndian.PutUint32(a[i:], v) }
	l(0, state.Behavior)
	w(4, state.Size)
	w(6, state.Stage)
	w(8, uint16(state.ScreenX))
	w(12, uint16(state.ScreenY))
	w(0x10, uint16(state.SourceOffsetX))
	w(0x14, uint16(state.SourceOffsetY))
	l(0x1c, state.GraphicsSet)
	l(0x20, state.TilesBase)
	for i, v := range state.GraphicCache {
		l(0x2c+i*4, v)
	}
	w(0x78, uint16(state.Health))
	w(0x7c, uint16(state.Power))
	w(0x54, uint16(state.ScreenOffsetX))
	w(0x56, uint16(state.ScreenOffsetY))
	v := state.Actor
	l(0x50, v.AnimationSet)
	l(0x8c, v.ShapeSet)
	w(0x24, v.Attributes)
	w(0x28, uint16(v.GraphicOffsetX))
	w(0x2a, uint16(v.GraphicOffsetY))
	w(0x3c, v.GraphicParameter)
	l(0x40, uint32(v.WorldX))
	l(0x44, uint32(v.WorldY))
	a[0x59], a[0x5a], a[0x5b], a[0x5c], a[0x60], a[0x5f], a[0x61] = v.Clock.FrameIndex, v.Clock.Duration, v.Clock.Ticks, v.Clock.Flags, v.Clock.Completed, v.Pose, v.Facing
	a[0x6c], a[0x6d], a[0x64], a[0x65], a[0x66], a[0x67], a[0x68] = v.EnabledStages, v.ActionFlags, v.Auxiliary64, v.Value65, v.Value66, v.Value67, v.Value68
	l(0x94, v.Auxiliary94)
	s.ActorBytes = hex.EncodeToString(a)
	p := attachmentOriginalBytes(t, previous.ParentBytes, 256)
	p[0x58], p[0x59], p[0x5f] = parent.CharacterID, parent.FrameIndex, parent.Pose
	binary.BigEndian.PutUint16(p[0x7c:], uint16(parent.Power))
	binary.BigEndian.PutUint32(p[0x40:], uint32(parent.X))
	binary.BigEndian.PutUint32(p[0x44:], uint32(parent.Y))
	s.ParentBytes = hex.EncodeToString(p)
	b := make([]byte, 1024)
	for group, buffer := range state.Buffers {
		s.Offsets[group] = buffer.Offset
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
	s.Buffers, s.PendingSound, s.SoundQueue, s.World = hex.EncodeToString(b), state.PendingSound, state.SoundQueue, world
	return s
}

type attachmentOriginalFixture struct {
	Sequences []struct {
		ID     uint32
		Frames []ChuAttachmentFrame
	}
	Animations []struct {
		Key  AnimationFrameKey
		Data AnimationRecordData
	}
	Calls json.RawMessage
}

func readAttachmentOriginal(t *testing.T, name string) (attachmentOriginalFixture, ChuAttachmentLibrary, int) {
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
		Baseline bool   `json:"frames_match_original_core"`
		External int    `json:"external_queue_calls"`
		SHA      string `json:"fixture_sha256"`
		Gzip     string `json:"gzip_sha256"`
	}
	if err := json.Unmarshal(mb, &m); err != nil {
		t.Fatal(err)
	}
	h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
	if !m.Pass || !m.Baseline || m.Records == 0 || m.External != 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
		t.Fatal("unverified original attachment fixture")
	}
	var f attachmentOriginalFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	lib := ChuAttachmentLibrary{Sequences: make(map[uint32][]ChuAttachmentFrame), Animations: make(AnimationLibrary)}
	for _, sequence := range f.Sequences {
		lib.Sequences[sequence.ID] = sequence.Frames
	}
	for _, frame := range f.Animations {
		lib.Animations[frame.Key] = frame.Data
	}
	return f, lib, m.Records
}

type attachmentOriginalCall struct {
	Kind, Name    string
	Before, After attachmentOriginalSnapshot
	Draw          bool
	ClearedBytes  uint16 `json:"cleared_bytes"`
}

// Original full-rule fixtures and tests precede chu_attachments.go.
func TestChuAttachmentsOriginalRules(t *testing.T) {
	for _, name := range []string{"chu-attachments-oracle", "chu-attachments-boundaries"} {
		t.Run(name, func(t *testing.T) {
			f, lib, count := readAttachmentOriginal(t, name)
			var rows []attachmentOriginalCall
			if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count {
				t.Fatal("missing original attachment rules", err)
			}
			immutableSequences, _ := json.Marshal(f.Sequences)
			immutableAnimations, _ := json.Marshal(f.Animations)
			for i, row := range rows {
				input, parent := decodeAttachmentOriginal(t, row.Kind, row.Before)
				want, _ := decodeAttachmentOriginal(t, row.Kind, row.After)
				got, out, err := StepChuAttachment(input, parent, row.Before.World, lib)
				if err != nil || out.Draw != row.Draw || out.ClearedBytes != row.ClearedBytes || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(encodeAttachmentOriginal(t, got, parent, row.Before.World, out, row.Before), row.After) {
					t.Fatalf("attachment %d %s/%s differs: %v; outcome=%+v want draw=%v clear=%d", i, row.Kind, row.Name, err, out, row.Draw, row.ClearedBytes)
				}
			}
			for i := range f.Sequences {
				f.Sequences[i].Frames = lib.Sequences[f.Sequences[i].ID]
			}
			for i := range f.Animations {
				f.Animations[i].Data = lib.Animations[f.Animations[i].Key]
			}
			afterSequences, _ := json.Marshal(f.Sequences)
			afterAnimations, _ := json.Marshal(f.Animations)
			if !bytes.Equal(immutableSequences, afterSequences) || !bytes.Equal(immutableAnimations, afterAnimations) {
				t.Fatal("attachment rules changed immutable animation data")
			}
			t.Logf("complete attachment rule calls=%d", count)
		})
	}
}

func TestChuAttachmentsSingleInitialLifetimes(t *testing.T) {
	f, lib, count := readAttachmentOriginal(t, "chu-attachments-sequences")
	var rows []struct {
		Kind    string
		Initial attachmentOriginalSnapshot
		Calls   []struct {
			attachmentOriginalCall
			External struct {
				X, Y        int32
				ParentPower int16 `json:"parent_power"`
				ParentPose  uint8 `json:"parent_pose"`
				Pause       uint8
				Slow        uint8
			}
		}
	}
	if err := json.Unmarshal(f.Calls, &rows); err != nil || len(rows) != count || count != 8 {
		t.Fatal("missing original attachment lifetimes", err)
	}
	steps, deleted := 0, 0
	for _, stream := range rows {
		state, parent := decodeAttachmentOriginal(t, stream.Kind, stream.Initial)
		world := stream.Initial.World
		encoded := stream.Initial
		for i, call := range stream.Calls {
			parent.X, parent.Y, parent.Power, parent.Pose = call.External.X, call.External.Y, call.External.ParentPower, call.External.ParentPose
			world.PauseFlags, world.SlowFlag = call.External.Pause, call.External.Slow
			var out ChuAttachmentOutcome
			var err error
			state, out, err = StepChuAttachment(state, parent, world, lib)
			encoded = encodeAttachmentOriginal(t, state, parent, world, out, encoded)
			want, _ := decodeAttachmentOriginal(t, stream.Kind, call.After)
			if err != nil || out.Draw != call.Draw || out.ClearedBytes != call.ClearedBytes || !reflect.DeepEqual(state, want) || !reflect.DeepEqual(encoded, call.After) {
				t.Fatalf("attachment lifetime %s step %d differs: %v", stream.Kind, i, err)
			}
			steps++
			if out.ClearedBytes != 0 {
				deleted++
			}
		}
	}
	if steps != 236 || deleted != 8 {
		t.Fatal("missing complete attachment lifetime coverage", steps, deleted)
	}
}

func TestChuAttachmentsErrorsAreAtomic(t *testing.T) {
	f, lib, _ := readAttachmentOriginal(t, "chu-attachments-oracle")
	var rows []attachmentOriginalCall
	if err := json.Unmarshal(f.Calls, &rows); err != nil {
		t.Fatal(err)
	}
	base, parent := decodeAttachmentOriginal(t, "charge", rows[0].Before)
	world := ChuAttachmentWorld{}
	for _, size := range []uint16{1, 15, 272, 65535} {
		input := base
		input.Size, input.Stage = size, 2
		input.Actor.Clock.FrameIndex, input.Actor.Clock.Ticks = 255, 1
		input.Power = 0
		parent.CharacterID, parent.Pose = 9, 0xa7
		got, out, err := StepChuAttachment(input, parent, world, lib)
		if err == nil || out != (ChuAttachmentOutcome{}) || !reflect.DeepEqual(got, input) {
			t.Fatal("unsupported clear storage leaked mutations")
		}
	}
	base.Kind = ChuAttachmentKind(255)
	got, out, err := StepChuAttachment(base, parent, world, lib)
	if err == nil || out != (ChuAttachmentOutcome{}) || !reflect.DeepEqual(got, base) {
		t.Fatal("unknown attachment kind changed state")
	}
	base.Kind, base.Stage, base.Power = ChuChargeAttachment, 0, 0
	got, out, err = StepChuAttachment(base, parent, world, ChuAttachmentLibrary{})
	if err == nil || out != (ChuAttachmentOutcome{}) || !reflect.DeepEqual(got, base) {
		t.Fatal("missing charge sequence leaked initialization")
	}
	base.Kind, base.Stage, base.Actor.Clock.FrameIndex, base.Actor.Clock.Completed = ChuReleaseAttachment, 0, 0, 0
	parent.Pose = 0xa7
	got, out, err = StepChuAttachment(base, parent, world, ChuAttachmentLibrary{})
	if err == nil || out != (ChuAttachmentOutcome{}) || !reflect.DeepEqual(got, base) {
		t.Fatal("missing release frame leaked initialization")
	}
}
