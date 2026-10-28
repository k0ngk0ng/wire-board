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

// The source fixture and this test precede jump.go. The adapter overlays only
// named state fields on the input bytes and compares the entire actor and pool.
func encodeJumpBody(state FighterBodyState, previous []byte) []byte {
	b := append([]byte(nil), previous...)
	b[0x59], b[0x5a], b[0x5b], b[0x5c], b[0x60] = state.Clock.FrameIndex, state.Clock.Duration, state.Clock.Ticks, state.Clock.Flags, state.Clock.Completed
	b[0x5d], b[0x5f], b[0x61], b[0x64], b[0x6b] = state.ActionKind, state.Pose, state.Facing, state.Auxiliary64, state.Request
	b[0x6c], b[0x6d], b[0x6e], b[0x6f], b[0x77] = state.EnabledStages, state.ActionFlags, state.Team, state.BaseTeam, state.Auxiliary77
	copy(b[0xa8:0xb8], state.Commands[:])
	for _, field := range []struct {
		offset int
		value  int32
	}{{0x10, state.VelocityX}, {0x14, state.VerticalVelocity}, {0x40, state.X}, {0x44, state.Y}, {0x4c, state.Gravity}, {0x9c, state.Ground}} {
		binary.BigEndian.PutUint32(b[field.offset:], uint32(field.value))
	}
	b[0x87] = state.ExtraJump
	b[0x26] = state.Layer
	binary.BigEndian.PutUint16(b[0x24:], state.Attributes)
	binary.BigEndian.PutUint16(b[0x3e:], state.Depth)
	return b
}

func TestJumpOriginalCalls(t *testing.T) {
	for _, name := range []string{"jump-oracle", "jump-boundaries"} {
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
			var rows []struct {
				Kind          string `json:"kind"`
				Before, After FighterBodyState
				Parameters    JumpParameters
				ActorBefore   string `json:"actor_before"`
				ActorAfter    string `json:"actor_after"`
				PoolBefore    struct {
					Cursor uint16
					Bytes  string
				} `json:"pool_before"`
				PoolAfter struct {
					Cursor uint16
					Bytes  string
				} `json:"pool_after"`
				Slots  []int `json:"returned_slots"`
				Landed bool  `json:"landed"`
			}
			if err = json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			mraw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Pass    bool
				Count   int    `json:"complete_calls"`
				SHA     string `json:"fixture_sha256"`
				GzipSHA string `json:"gzip_sha256"`
				Match   bool   `json:"frames_match_original_core"`
			}
			if err = json.Unmarshal(mraw, &manifest); err != nil {
				t.Fatal(err)
			}
			digest, zipDigest := sha256.Sum256(raw), sha256.Sum256(compressed)
			if !manifest.Pass || !manifest.Match || manifest.Count != len(rows) || len(rows) == 0 || manifest.SHA != hex.EncodeToString(digest[:]) || manifest.GzipSHA != hex.EncodeToString(zipDigest[:]) {
				t.Fatal("unverified jump evidence")
			}
			counts := map[string]int{}
			for i, r := range rows {
				pool, poolBefore := decodeEffectPool(t, r.PoolBefore.Cursor, r.PoolBefore.Bytes)
				wantPool, poolAfter := decodeEffectPool(t, r.PoolAfter.Cursor, r.PoolAfter.Bytes)
				actorBefore, err := hex.DecodeString(r.ActorBefore)
				if err != nil || len(actorBefore) != 256 {
					t.Fatal("invalid original actor")
				}
				actorAfter, err := hex.DecodeString(r.ActorAfter)
				if err != nil || len(actorAfter) != 256 {
					t.Fatal("invalid original output actor")
				}
				if !bytes.Equal(encodeJumpBody(r.Before, actorBefore), actorBefore) {
					t.Fatal("fixture semantic actor differs from its original bytes")
				}
				var got FighterBodyState
				var gotPool EffectPool
				var selected [2]int
				var landed bool
				switch r.Kind {
				case "jump":
					got, gotPool, selected, err = BeginJump(r.Before, pool, r.Parameters)
				case "landing":
					got, gotPool, selected, err = BeginLanding(r.Before, pool)
				case "air":
					got, gotPool, selected, landed, err = StepAirAction(r.Before, pool)
				default:
					t.Fatalf("unknown original stage %q", r.Kind)
				}
				if err != nil || got != r.After || gotPool != wantPool || !reflect.DeepEqual(selected[:], r.Slots) || (r.Kind == "air" && landed != r.Landed) || !bytes.Equal(encodeJumpBody(got, actorBefore), actorAfter) || !bytes.Equal(encodeEffectPool(gotPool, poolBefore), poolAfter) {
					t.Fatalf("case %d %s differs: err=%v body=%+v want=%+v slots=%v/%v landed=%v/%v", i, r.Kind, err, got, r.After, selected, r.Slots, landed, r.Landed)
				}
				counts[r.Kind]++
			}
			if counts["jump"] == 0 || counts["landing"] == 0 || counts["air"] == 0 {
				t.Fatal("missing jump stage")
			}
			t.Logf("complete original stages: %v", counts)
		})
	}
}

func TestJumpInvalidPoolDoesNotPartiallyModifyState(t *testing.T) {
	pool := EffectPool{Cursor: 1}
	body := FighterBodyState{GroundActionState: GroundActionState{Pose: 55, VerticalVelocity: 0, Clock: AnimationClock{Completed: 9}}, X: 12, Y: 0, Ground: 0, ExtraJump: 3}
	params := JumpParameters{VerticalVelocity: -1234, Gravity: 99, HorizontalVelocity: 111}
	got, next, slots, err := BeginJump(body, pool, params)
	if err == nil || got != body || next != pool || slots != [2]int{-1, -1} {
		t.Fatal("invalid jump returned partial state")
	}
	got, next, slots, err = BeginLanding(body, pool)
	if err == nil || got != body || next != pool || slots != [2]int{-1, -1} {
		t.Fatal("invalid landing returned partial state")
	}
	got, next, slots, landed, err := StepAirAction(body, pool)
	if err == nil || got != body || next != pool || slots != [2]int{-1, -1} || landed {
		t.Fatal("invalid air landing returned partial state")
	}
}
