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
	"testing"
)

type animationOriginalSnapshot struct {
	Actor        AnimationActorState `json:"actor"`
	ActorBytes   string              `json:"actor_bytes"`
	Buffers      string              `json:"buffers"`
	Offsets      [4]uint16           `json:"offsets"`
	PendingSound uint8               `json:"pending_sound"`
	SoundQueue   [4]uint8            `json:"sound_queue"`
}

func decodeAnimationSnapshot(t *testing.T, s animationOriginalSnapshot) (AnimationSubmissionState, []byte, []byte) {
	t.Helper()
	actor, err := hex.DecodeString(s.ActorBytes)
	if err != nil || len(actor) != 256 {
		t.Fatal("invalid original actor")
	}
	raw, err := hex.DecodeString(s.Buffers)
	if err != nil || len(raw) != 1024 {
		t.Fatal("invalid original shape buffers")
	}
	state := AnimationSubmissionState{Actor: s.Actor, PendingSound: s.PendingSound, SoundQueue: s.SoundQueue}
	for group := range state.Buffers {
		state.Buffers[group].Offset = s.Offsets[group]
		for index := range state.Buffers[group].Records {
			b := raw[group*256+index*16 : group*256+(index+1)*16]
			state.Buffers[group].Records[index] = AnimationShapeRecord{X: int16(binary.BigEndian.Uint16(b)), Y: int16(binary.BigEndian.Uint16(b[4:])), Width: b[8], Height: b[9], Owner: binary.BigEndian.Uint16(b[12:]), Preserved: [4]uint16{binary.BigEndian.Uint16(b[2:]), binary.BigEndian.Uint16(b[6:]), binary.BigEndian.Uint16(b[10:]), binary.BigEndian.Uint16(b[14:])}}
		}
	}
	return state, actor, raw
}

func encodeAnimationSnapshot(state AnimationSubmissionState, oldActor, oldBuffers []byte) ([]byte, []byte) {
	a := append([]byte(nil), oldActor...)
	b := append([]byte(nil), oldBuffers...)
	s := state.Actor
	a[0x5a] = s.Clock.Duration
	a[0x5c] = s.Clock.Flags
	a[0x6c] = s.EnabledStages
	binary.BigEndian.PutUint16(a[0x24:], s.Attributes)
	binary.BigEndian.PutUint16(a[0x28:], uint16(s.GraphicOffsetX))
	binary.BigEndian.PutUint16(a[0x2a:], uint16(s.GraphicOffsetY))
	binary.BigEndian.PutUint16(a[0x3c:], s.GraphicParameter)
	binary.BigEndian.PutUint32(a[0x94:], s.Auxiliary94)
	a[0x64] = s.Auxiliary64
	a[0x65] = s.Value65
	a[0x66] = s.Value66
	a[0x67] = s.Value67
	a[0x68] = s.Value68
	for group, buffer := range state.Buffers {
		for index, r := range buffer.Records {
			v := b[group*256+index*16 : group*256+(index+1)*16]
			binary.BigEndian.PutUint16(v, uint16(r.X))
			binary.BigEndian.PutUint16(v[4:], uint16(r.Y))
			v[8] = r.Width
			v[9] = r.Height
			binary.BigEndian.PutUint16(v[12:], r.Owner)
			for n, offset := range []int{2, 6, 10, 14} {
				binary.BigEndian.PutUint16(v[offset:], r.Preserved[n])
			}
		}
	}
	return a, b
}

// Original fixtures and these tests precede animation_records.go. The complete
// actor, shape-buffer, offset and sound-queue results are compared without masks.
func TestAnimationRecordsAgainstOriginal(t *testing.T) {
	for _, name := range []string{"animation-records-oracle", "animation-records-boundaries"} {
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
			var fixture struct {
				Library []struct {
					Key  AnimationFrameKey
					Data AnimationRecordData
				}
				Calls []struct {
					Frame         int
					Name          string
					Key           AnimationFrameKey
					Data          AnimationRecordData
					Before, After animationOriginalSnapshot
					EntrySR       uint32 `json:"entry_sr"`
					ReturnStack   uint32 `json:"return_stack"`
					SoundWrites   []struct {
						PC      string
						Address string
						Width   int
						Value   uint32
						Frame   int
						SR      uint32
						Stack   uint32
					} `json:"sound_writes"`
				}
			}
			if err = json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			mraw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Pass                   bool
				Count                  int    `json:"complete_calls"`
				SHA                    string `json:"fixture_sha256"`
				GzipSHA                string `json:"gzip_sha256"`
				ExternalSoundConsumers int    `json:"external_sound_consumer_calls"`
			}
			if err = json.Unmarshal(mraw, &manifest); err != nil {
				t.Fatal(err)
			}
			h, zHash := sha256.Sum256(raw), sha256.Sum256(compressed)
			if !manifest.Pass || manifest.Count != len(fixture.Calls) || len(fixture.Calls) == 0 || hex.EncodeToString(h[:]) != manifest.SHA || hex.EncodeToString(zHash[:]) != manifest.GzipSHA {
				t.Fatal("unverified animation fixture")
			}
			library := AnimationLibrary{}
			for _, frame := range fixture.Library {
				library[frame.Key] = frame.Data
			}
			fullStates, externalConsumers := 0, 0
			for i, row := range fixture.Calls {
				input, actorBefore, buffersBefore := decodeAnimationSnapshot(t, row.Before)
				want, actorAfter, buffersAfter := decodeAnimationSnapshot(t, row.After)
				data := library
				if name == "animation-records-boundaries" {
					data = AnimationLibrary{row.Key: row.Data}
				}
				got, err := SubmitAnimationRecord(input, data)
				actualActor, actualBuffers := encodeAnimationSnapshot(got, actorBefore, buffersBefore)
				if err != nil || got.Actor != want.Actor || got.Buffers != want.Buffers || got.PendingSound != want.PendingSound || !bytes.Equal(actualActor, actorAfter) || !bytes.Equal(actualBuffers, buffersAfter) {
					t.Fatalf("animation call %d frame %d %s differs: err=%v actor=%+v want=%+v sound=%v/%v queue=%v/%v offsets=%v/%v", i, row.Frame, row.Name, err, got.Actor, want.Actor, got.PendingSound, want.PendingSound, got.SoundQueue, want.SoundQueue, got.Buffers[0].Offset, want.Buffers[0].Offset)
				}
				if got.SoundQueue == want.SoundQueue {
					fullStates++
					continue
				}
				// Keep the raw mismatch visible: this function did not enqueue or
				// consume anything. Independently observed higher-priority code
				// cleared all four slots while the animation call was suspended.
				// We never pass those observed output values into the Go model.
				if name != "animation-records-oracle" || len(row.SoundWrites) != 4 || got.SoundQueue != input.SoundQueue || want.SoundQueue != [4]uint8{} || input.PendingSound != 0 || want.PendingSound != 0 {
					t.Fatalf("unexplained sound state difference at call %d", i)
				}
				addresses := [4]string{"0xfff80d", "0xfff80c", "0xfff80b", "0xfff80a"}
				for n, w := range row.SoundWrites {
					if w.PC != "0x37c56" || w.Address != addresses[n] || w.Width != 1 || w.Value != 0 || w.Frame != row.Frame || w.SR&0x700 <= row.EntrySR&0x700 || w.Stack >= row.ReturnStack {
						t.Fatalf("unverified external queue consumption at call %d: %+v", i, w)
					}
				}
				externalConsumers++
			}
			if externalConsumers != manifest.ExternalSoundConsumers {
				t.Fatalf("external sound consumers %d want %d", externalConsumers, manifest.ExternalSoundConsumers)
			}
			t.Logf("actor/buffer/pending matched=%d; complete states matched=%d; independently traced external queue consumers=%d (not modeled by animation submission)", len(fixture.Calls), fullStates, externalConsumers)
		})
	}
}

func TestAnimationRecordsRejectMissingOrMalformedDataAtomically(t *testing.T) {
	state := AnimationSubmissionState{PendingSound: 0x92, Actor: AnimationActorState{AnimationSet: 1, ShapeSet: 2, Pose: 3, Clock: AnimationClock{FrameIndex: 4}}}
	key := AnimationFrameKey{AnimationSet: 1, ShapeSet: 2, Pose: 3, Index: 4}
	for _, library := range []AnimationLibrary{nil, {key: AnimationRecordData{ShapeFlags: 0x40}}, {key: AnimationRecordData{ShapeFlags: 8, Groups: [4][]AnimationShapeDefinition{nil, nil, nil, {{Width: 1}, {Width: 2}}}}}} {
		got, err := SubmitAnimationRecord(state, library)
		if err == nil || got != state {
			t.Fatal("invalid data partially mutated animation state")
		}
	}
	state.Buffers[0].Offset = 1
	library := AnimationLibrary{key: AnimationRecordData{ShapeFlags: 0x40, Groups: [4][]AnimationShapeDefinition{{{Width: 0x81}}}}}
	got, err := SubmitAnimationRecord(state, library)
	if err == nil || got != state {
		t.Fatal("unaligned active shape buffer accepted")
	}
}
