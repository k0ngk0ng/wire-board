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

type throwOriginalSnapshot struct {
	GrabOffset  uint16 `json:"grab_offset"`
	HurtOffset  uint16 `json:"hurt_offset"`
	Grabs       string
	Hurts       string
	Actors      map[uint16]string
	Left, Right uint16
}

func decodeThrowSnapshot(t *testing.T, raw throwOriginalSnapshot) ThrowContactState {
	t.Helper()
	s := ThrowContactState{Actors: make(map[uint16]ThrowContactActor), Left: int16(raw.Left), Right: int16(raw.Right)}
	b, err := hex.DecodeString(raw.Grabs)
	if err != nil || len(b) != 512 {
		t.Fatal("invalid original throw buffer", err)
	}
	s.Throws.Offset = raw.GrabOffset
	s.Throws.Records = make([]ThrowContactRecord, len(b)/32)
	for i := range s.Throws.Records {
		v := b[i*32 : (i+1)*32]
		s.Throws.Records[i] = ThrowContactRecord{X: int16(binary.BigEndian.Uint16(v)), Y: int16(binary.BigEndian.Uint16(v[2:])), Width: v[4], Height: v[5], SourcePose: v[6], TargetPose: v[7], SourceAction: v[8], TargetAction: v[9], TargetFacing: v[10], Owner: binary.BigEndian.Uint16(v[12:]), OffsetX: int16(binary.BigEndian.Uint16(v[16:])), OffsetY: int16(binary.BigEndian.Uint16(v[18:]))}
	}
	b, err = hex.DecodeString(raw.Hurts)
	if err != nil || len(b) != 256 {
		t.Fatal("invalid original hurt buffer", err)
	}
	s.Hurts.Offset = raw.HurtOffset
	for i := range s.Hurts.Records {
		v := b[i*16 : (i+1)*16]
		s.Hurts.Records[i] = AnimationShapeRecord{X: int16(binary.BigEndian.Uint16(v)), Y: int16(binary.BigEndian.Uint16(v[4:])), Width: v[8], Height: v[9], Owner: binary.BigEndian.Uint16(v[12:]), Preserved: [4]uint16{binary.BigEndian.Uint16(v[2:]), binary.BigEndian.Uint16(v[6:]), binary.BigEndian.Uint16(v[10:]), binary.BigEndian.Uint16(v[14:])}}
	}
	for id, encoded := range raw.Actors {
		a, err := hex.DecodeString(encoded)
		if err != nil || len(a) != 256 {
			t.Fatal("invalid original throw owner", err)
		}
		s.Actors[id] = ThrowContactActor{X: int32(binary.BigEndian.Uint32(a[0x40:])), Y: int32(binary.BigEndian.Uint32(a[0x44:])), Ground: int32(binary.BigEndian.Uint32(a[0x9c:])), Team: a[0x6e], Delay: a[0xc8], EnabledStages: a[0x6c], FrameFlags: a[0x5c], ContactFlags: a[0x89], TargetMask: binary.BigEndian.Uint32(a[0x94:]), Facing: a[0x61], InputFlags: a[0x71], Layer: a[0x26], Pose: a[0x5f], Action: a[0x5d], FrameIndex: a[0x59], FrameDuration: a[0x5a], FrameTicks: a[0x5b], Auxiliary60: a[0x60], AttackFlags: a[0x64], RetainedPress: a[0x77], CancelTicks: a[0x6d]}
	}
	return s
}

func encodeThrowSnapshot(t *testing.T, s ThrowContactState, previous throwOriginalSnapshot) throwOriginalSnapshot {
	t.Helper()
	raw := previous
	raw.GrabOffset, raw.HurtOffset, raw.Left, raw.Right = s.Throws.Offset, s.Hurts.Offset, uint16(s.Left), uint16(s.Right)
	grabs, err := hex.DecodeString(previous.Grabs)
	if err != nil {
		t.Fatal(err)
	}
	for i, g := range s.Throws.Records {
		v := grabs[i*32 : (i+1)*32]
		binary.BigEndian.PutUint16(v, uint16(g.X))
		binary.BigEndian.PutUint16(v[2:], uint16(g.Y))
		v[4], v[5], v[6], v[7], v[8], v[9], v[10] = g.Width, g.Height, g.SourcePose, g.TargetPose, g.SourceAction, g.TargetAction, g.TargetFacing
		binary.BigEndian.PutUint16(v[12:], g.Owner)
		binary.BigEndian.PutUint16(v[16:], uint16(g.OffsetX))
		binary.BigEndian.PutUint16(v[18:], uint16(g.OffsetY))
	}
	raw.Grabs = hex.EncodeToString(grabs)
	hurts, err := hex.DecodeString(previous.Hurts)
	if err != nil {
		t.Fatal(err)
	}
	for i, h := range s.Hurts.Records {
		v := hurts[i*16 : (i+1)*16]
		binary.BigEndian.PutUint16(v, uint16(h.X))
		binary.BigEndian.PutUint16(v[4:], uint16(h.Y))
		v[8], v[9] = h.Width, h.Height
		binary.BigEndian.PutUint16(v[12:], h.Owner)
		for j, off := range []int{2, 6, 10, 14} {
			binary.BigEndian.PutUint16(v[off:], h.Preserved[j])
		}
	}
	raw.Hurts = hex.EncodeToString(hurts)
	raw.Actors = make(map[uint16]string, len(s.Actors))
	for id, a := range s.Actors {
		v, err := hex.DecodeString(previous.Actors[id])
		if err != nil || len(v) != 256 {
			t.Fatal("missing original throw owner")
		}
		binary.BigEndian.PutUint32(v[0x40:], uint32(a.X))
		binary.BigEndian.PutUint32(v[0x44:], uint32(a.Y))
		binary.BigEndian.PutUint32(v[0x9c:], uint32(a.Ground))
		binary.BigEndian.PutUint32(v[0x94:], a.TargetMask)
		v[0x6e], v[0xc8], v[0x6c], v[0x5c], v[0x89], v[0x61], v[0x71], v[0x26] = a.Team, a.Delay, a.EnabledStages, a.FrameFlags, a.ContactFlags, a.Facing, a.InputFlags, a.Layer
		v[0x5f], v[0x5d], v[0x59], v[0x5a], v[0x5b], v[0x60], v[0x64], v[0x77], v[0x6d] = a.Pose, a.Action, a.FrameIndex, a.FrameDuration, a.FrameTicks, a.Auxiliary60, a.AttackFlags, a.RetainedPress, a.CancelTicks
		raw.Actors[id] = hex.EncodeToString(v)
	}
	return raw
}

// Complete original fixtures and these tests precede throw_contacts.go.
func TestThrowContactsOriginalCalls(t *testing.T) {
	for _, name := range []string{"throw-contacts-oracle", "throw-contacts-boundaries"} {
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
			data, err := io.ReadAll(z)
			if err != nil {
				t.Fatal(err)
			}
			manifestRaw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Pass              bool
				Records, Contacts int
				SHA               string `json:"fixture_sha256"`
				Gzip              string `json:"gzip_sha256"`
				Match             bool   `json:"frames_match_original_core"`
			}
			if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
				t.Fatal(err)
			}
			h, zh := sha256.Sum256(data), sha256.Sum256(compressed)
			if !manifest.Pass || !manifest.Match || manifest.Records == 0 || hex.EncodeToString(h[:]) != manifest.SHA || hex.EncodeToString(zh[:]) != manifest.Gzip {
				t.Fatal("unverified original throw evidence")
			}
			var calls []struct {
				Name          string
				Before, After throwOriginalSnapshot
				Contacts      int
			}
			if err := json.Unmarshal(data, &calls); err != nil || len(calls) != manifest.Records {
				t.Fatal("missing original throw calls", err)
			}
			contacts := 0
			for i, call := range calls {
				input, copy := decodeThrowSnapshot(t, call.Before), decodeThrowSnapshot(t, call.Before)
				got, err := ResolveThrowContacts(input)
				if err != nil || !reflect.DeepEqual(got, decodeThrowSnapshot(t, call.After)) || !reflect.DeepEqual(encodeThrowSnapshot(t, got, call.Before), call.After) {
					t.Fatalf("throw contact %d %s differs from complete original state: %v", i, call.Name, err)
				}
				if !reflect.DeepEqual(input, copy) {
					t.Fatal("throw resolution mutated caller state")
				}
				contacts += call.Contacts
			}
			if contacts != manifest.Contacts || name == "throw-contacts-boundaries" && contacts == 0 {
				t.Fatal("missing actual original contacts")
			}
			t.Logf("complete throw calls=%d contacts=%d", len(calls), contacts)
		})
	}
}

func TestThrowContactsInvalidStateIsAtomic(t *testing.T) {
	base := ThrowContactState{Actors: map[uint16]ThrowContactActor{1: {Team: 0}, 2: {Team: 1}}, Throws: ThrowContactBuffer{Offset: 32, Records: []ThrowContactRecord{{Owner: 1}}}, Hurts: AnimationShapeBuffer{Offset: 16}}
	base.Hurts.Records[0].Owner = 2
	for _, offset := range []uint16{1, 31, 64, 0xffff} {
		input := base
		input.Throws.Offset = offset
		got, err := ResolveThrowContacts(input)
		if err == nil || !reflect.DeepEqual(got, input) {
			t.Fatal("invalid throw offset changed state")
		}
	}
	for _, offset := range []uint16{1, 15, 257, 0xffff} {
		input := base
		input.Hurts.Offset = offset
		got, err := ResolveThrowContacts(input)
		if err == nil || !reflect.DeepEqual(got, input) {
			t.Fatal("invalid hurt offset changed state")
		}
	}
	input := base
	input.Throws = ThrowContactBuffer{Offset: 64, Records: []ThrowContactRecord{{Owner: 1}, {Owner: 3}}}
	got, err := ResolveThrowContacts(input)
	if err == nil || !reflect.DeepEqual(got, input) || base.Actors[1].ContactFlags != 0 {
		t.Fatal("missing later throw owner caused partial resolution")
	}
	input = base
	input.Hurts.Offset = 32
	input.Hurts.Records[1].Owner = 3
	got, err = ResolveThrowContacts(input)
	if err == nil || !reflect.DeepEqual(got, input) || base.Actors[1].ContactFlags != 0 {
		t.Fatal("missing later hurt owner caused partial resolution")
	}
}
