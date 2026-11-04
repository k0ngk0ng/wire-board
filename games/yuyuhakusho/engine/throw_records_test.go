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

type throwSubmissionSnapshot struct {
	ObjectID   uint16 `json:"object_id"`
	ActorBytes string `json:"actor_bytes"`
	Records    string
	Offset     uint16
}

func decodeThrowSubmission(t *testing.T, original throwSubmissionSnapshot) ChuThrowSubmissionState {
	t.Helper()
	a, err := hex.DecodeString(original.ActorBytes)
	if err != nil || len(a) != 256 {
		t.Fatal("invalid original throw submit actor", err)
	}
	s := ChuThrowSubmissionState{ObjectID: original.ObjectID, X: int32(binary.BigEndian.Uint32(a[0x40:])), Y: int32(binary.BigEndian.Uint32(a[0x44:])), Held: a[0x71]}
	s.Actor = GroundActionState{Clock: AnimationClock{FrameIndex: a[0x59], Duration: a[0x5a], Ticks: a[0x5b], Flags: a[0x5c], Completed: a[0x60]},
		ActionKind: a[0x5d], Pose: a[0x5f], Facing: a[0x61], Auxiliary64: a[0x64], Request: a[0x6b], EnabledStages: a[0x6c], ActionFlags: a[0x6d],
		Team: a[0x6e], BaseTeam: a[0x6f], Auxiliary77: a[0x77], VerticalVelocity: int32(binary.BigEndian.Uint32(a[0x14:]))}
	copy(s.Actor.Commands[:], a[0xa8:0xb8])
	raw, err := hex.DecodeString(original.Records)
	if err != nil || len(raw) != 1024 {
		t.Fatal("invalid original throw submit storage", err)
	}
	s.Buffer.Offset, s.Buffer.Records = original.Offset, make([]ThrowContactRecord, len(raw)/32)
	for i := range s.Buffer.Records {
		b := raw[i*32 : (i+1)*32]
		s.Buffer.Records[i] = ThrowContactRecord{X: int16(binary.BigEndian.Uint16(b)), Y: int16(binary.BigEndian.Uint16(b[2:])), Width: b[4], Height: b[5],
			SourcePose: b[6], TargetPose: b[7], SourceAction: b[8], TargetAction: b[9], TargetFacing: b[10], Owner: binary.BigEndian.Uint16(b[12:]),
			OffsetX: int16(binary.BigEndian.Uint16(b[16:])), OffsetY: int16(binary.BigEndian.Uint16(b[18:]))}
	}
	return s
}

func encodeThrowSubmission(t *testing.T, s ChuThrowSubmissionState, original throwSubmissionSnapshot) throwSubmissionSnapshot {
	t.Helper()
	result := original
	result.ObjectID, result.Offset = s.ObjectID, s.Buffer.Offset
	a, err := hex.DecodeString(original.ActorBytes)
	if err != nil {
		t.Fatal(err)
	}
	b := s.Actor
	a[0x59], a[0x5a], a[0x5b], a[0x5c], a[0x60] = b.Clock.FrameIndex, b.Clock.Duration, b.Clock.Ticks, b.Clock.Flags, b.Clock.Completed
	a[0x5d], a[0x5f], a[0x61], a[0x64], a[0x6b], a[0x6c], a[0x6d] = b.ActionKind, b.Pose, b.Facing, b.Auxiliary64, b.Request, b.EnabledStages, b.ActionFlags
	a[0x6e], a[0x6f], a[0x77], a[0x71] = b.Team, b.BaseTeam, b.Auxiliary77, s.Held
	binary.BigEndian.PutUint32(a[0x14:], uint32(b.VerticalVelocity))
	binary.BigEndian.PutUint32(a[0x40:], uint32(s.X))
	binary.BigEndian.PutUint32(a[0x44:], uint32(s.Y))
	copy(a[0xa8:0xb8], b.Commands[:])
	result.ActorBytes = hex.EncodeToString(a)
	raw, err := hex.DecodeString(original.Records)
	if err != nil {
		t.Fatal(err)
	}
	for i, record := range s.Buffer.Records {
		v := raw[i*32 : (i+1)*32]
		binary.BigEndian.PutUint16(v, uint16(record.X))
		binary.BigEndian.PutUint16(v[2:], uint16(record.Y))
		v[4], v[5], v[6], v[7], v[8], v[9], v[10] = record.Width, record.Height, record.SourcePose, record.TargetPose, record.SourceAction, record.TargetAction, record.TargetFacing
		binary.BigEndian.PutUint16(v[12:], record.Owner)
		binary.BigEndian.PutUint16(v[16:], uint16(record.OffsetX))
		binary.BigEndian.PutUint16(v[18:], uint16(record.OffsetY))
	}
	result.Records = hex.EncodeToString(raw)
	return result
}

// Complete original fixtures and these tests precede throw_records.go.
func TestChuThrowSubmissionAgainstOriginal(t *testing.T) {
	for _, name := range []string{"throw-records-oracle", "throw-records-boundaries"} {
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
			mraw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var m struct {
				Pass, Baseline bool
				Records        int
				SHA            string `json:"fixture_sha256"`
				Gzip           string `json:"gzip_sha256"`
				Match          bool   `json:"frames_match_original_core"`
			}
			if err := json.Unmarshal(mraw, &m); err != nil {
				t.Fatal(err)
			}
			h, zh := sha256.Sum256(raw), sha256.Sum256(zipped)
			if !m.Pass || !m.Match || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip {
				t.Fatal("unverified original throw submission fixture")
			}
			var rows []struct {
				Name          string
				Before, After throwSubmissionSnapshot
			}
			if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != m.Records {
				t.Fatal("missing original throw submission calls", err)
			}
			for i, row := range rows {
				input := decodeThrowSubmission(t, row.Before)
				copy := decodeThrowSubmission(t, row.Before)
				got, err := SubmitChuThrow(input)
				if err != nil || !reflect.DeepEqual(got, decodeThrowSubmission(t, row.After)) || !reflect.DeepEqual(encodeThrowSubmission(t, got, row.Before), row.After) {
					t.Fatalf("throw submission %d %s differs: %v", i, row.Name, err)
				}
				if !reflect.DeepEqual(input, copy) {
					t.Fatal("throw producer mutated caller's record storage")
				}
			}
			t.Logf("Complete original Chu throw submissions=%d", len(rows))
		})
	}
}

func TestChuThrowSubmissionInvalidStorageIsAtomic(t *testing.T) {
	for _, c := range []struct {
		Offset uint16
		Length int
	}{{0, 0}, {1, 32}, {31, 32}, {1024, 32}, {65504, 32}} {
		input := ChuThrowSubmissionState{Actor: GroundActionState{Pose: 123, Facing: 255, Team: 1, BaseTeam: 2}, Buffer: ThrowContactBuffer{Offset: c.Offset, Records: make([]ThrowContactRecord, c.Length)}}
		copy := input
		copy.Buffer.Records = append([]ThrowContactRecord{}, input.Buffer.Records...)
		got, err := SubmitChuThrow(input)
		if err == nil || !reflect.DeepEqual(got, input) || !reflect.DeepEqual(input, copy) {
			t.Fatal("invalid throw storage changed state")
		}
	}
}
