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

type fighter10MotionSnapshot struct {
	ObjectID     uint16 `json:"object_id"`
	CharacterID  uint8  `json:"character_id"`
	Body         FighterBodyState
	ActorBytes   string `json:"actor_bytes"`
	Controls     FighterControls
	Aux48        int32 `json:"aux_48"`
	ContactFlags uint8 `json:"contact_flags"`
	Children     string
	Pool         struct {
		Cursor uint16
		Bytes  string
	}
	Parameters JumpParameters
}

type fighter10MotionOriginal struct {
	Kind          uint8
	Name          string
	Before, After fighter10MotionSnapshot
	AdvanceClock  bool             `json:"advance_clock"`
	ExternalInput map[string]uint8 `json:"external_input"`
}

func readFighter10Motion(t *testing.T, name string, target any) int {
	t.Helper()
	readLaneMotion(t, name, target)
	raw, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Records    int
		Baseline   bool `json:"frames_match_original_core"`
		Incomplete int  `json:"incomplete_calls"`
		Identity   struct {
			ROM  string `json:"rom_sha256"`
			Core string `json:"core_sha256"`
		}
		Retreat struct{ Bytes, SHA string } `json:"-"`
		Table   struct {
			Bytes string
			SHA   string `json:"sha256"`
		} `json:"retreat_table"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if !manifest.Baseline || manifest.Incomplete != 0 || manifest.Identity.ROM != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || manifest.Identity.Core != "e87739d0804e62423fda890377fa46a8932fe1a6fa0132b1ad7f13b8b3af7306" || manifest.Table.Bytes != "0044000e000c000a0008000600380036" {
		t.Fatal("incomplete or mismatched original fighter10 motion evidence")
	}
	decoded, err := hex.DecodeString(manifest.Table.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(decoded)
	if hex.EncodeToString(h[:]) != manifest.Table.SHA {
		t.Fatal("unbound original retreat table")
	}
	return manifest.Records
}

func decodeFighter10Motion(t *testing.T, raw fighter10MotionSnapshot) (Fighter10MotionState, []byte, []byte, []byte) {
	t.Helper()
	state := Fighter10MotionState{ObjectID: raw.ObjectID, Body: raw.Body, Controls: raw.Controls, Aux48: raw.Aux48, ContactFlags: raw.ContactFlags}
	actor, err := hex.DecodeString(raw.ActorBytes)
	if err != nil || len(actor) != 256 {
		t.Fatal("invalid full original actor", err, len(actor))
	}
	children, err := hex.DecodeString(raw.Children)
	if err != nil || len(children) != 768 {
		t.Fatal("invalid complete original children", err, len(children))
	}
	state.Pool, _ = decodeEffectPool(t, raw.Pool.Cursor, raw.Pool.Bytes)
	pool, err := hex.DecodeString(raw.Pool.Bytes)
	if err != nil || len(pool) != 2048 {
		t.Fatal("invalid full original FX pool", err, len(pool))
	}
	return state, actor, children, pool
}

func compareFighter10Motion(t *testing.T, state Fighter10MotionState, actor, children, pool []byte, want fighter10MotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	expected, expectedActor, expectedChildren, expectedPool := decodeFighter10Motion(t, want)
	actor = encodeJumpBody(state.Body, actor)
	binary.BigEndian.PutUint16(actor[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(actor[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(actor[0x80:], uint16(state.Body.Auxiliary80))
	actor[0xc8] = state.Body.TransitionDelay
	binary.BigEndian.PutUint32(actor[0x48:], uint32(state.Aux48))
	actor[0x89] = state.ContactFlags
	actor[0x71], actor[0xc0] = uint8(state.Controls.Held), uint8(state.Controls.Held>>8)
	actor[0x72], actor[0xc1] = uint8(state.Controls.Pressed), uint8(state.Controls.Pressed>>8)
	actor[0x73], actor[0xc2] = uint8(state.Controls.Changed), uint8(state.Controls.Changed>>8)
	actor[0x74], actor[0x75] = state.Controls.RawHeld, state.Controls.RetainTicks
	pool = encodeEffectPool(state.Pool, pool)
	if state != expected || !bytes.Equal(actor, expectedActor) || !bytes.Equal(children, expectedChildren) || !bytes.Equal(pool, expectedPool) {
		diff := func(a, b []byte) []int {
			out := []int{}
			for i := range a {
				if a[i] != b[i] {
					out = append(out, i)
				}
			}
			return out
		}
		t.Fatalf("source10 continuation differs: actor=%x children=%x pool=%x body=%+v expected=%+v aux48=%x/%x", diff(actor, expectedActor), diff(children, expectedChildren), diff(pool, expectedPool), state.Body, expected.Body, state.Aux48, expected.Aux48)
	}
	return actor, children, pool
}

// Failure scenarios and actual CPU fixtures precede fighter10_motion.go.
func TestFighter10MotionOriginalPhases(t *testing.T) {
	for _, name := range []string{"fighter10-motion-oracle", "fighter10-motion-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []fighter10MotionOriginal
			count := readFighter10Motion(t, name, &rows)
			if count != len(rows) {
				t.Fatal("incomplete original records")
			}
			kinds := map[uint8]int{}
			for index, row := range rows {
				state, actor, children, pool := decodeFighter10Motion(t, row.Before)
				if state.Body.ActionKind != row.Kind {
					t.Fatal("wrong source action slot")
				}
				if name == "fighter10-motion-boundaries" && row.Before.CharacterID != 10 {
					t.Fatal("private source identity differs")
				}
				next, err := StepFighter10Motion(state, row.Before.Parameters)
				if err != nil {
					t.Fatal(index, row.Name, err)
				}
				compareFighter10Motion(t, next, actor, children, pool, row.After)
				kinds[row.Kind]++
			}
			if name == "fighter10-motion-boundaries" {
				want := map[uint8]int{0x11: 1152, 0x2d: 630, 0x47: 3856, 0x48: 1636, 0x49: 1312}
				if len(rows) != 8586 {
					t.Fatal("incomplete private original matrix", len(rows))
				}
				for kind, count := range want {
					if kinds[kind] != count {
						t.Fatalf("action %x count %d != %d", kind, kinds[kind], count)
					}
				}
			}
			t.Logf("actual original rows=%d kinds=%v; natural count is not inferred from isolated coverage", len(rows), kinds)
		})
	}
}

func TestFighter10MotionSingleInitialState(t *testing.T) {
	var sequences []struct {
		Kind                  uint8
		Name, Pool, Milestone string
		Initial               fighter10MotionSnapshot
		Steps                 []fighter10MotionOriginal
	}
	if readFighter10Motion(t, "fighter10-motion-sequences", &sequences) != 27 || len(sequences) != 27 {
		t.Fatal("missing independent private trajectories")
	}
	total := 0
	for _, sequence := range sequences {
		state, actor, children, pool := decodeFighter10Motion(t, sequence.Initial)
		for index, step := range sequence.Steps {
			for key, value := range step.ExternalInput {
				if key != "pressed" {
					t.Fatal("undeclared private input")
				}
				state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
			}
			if !step.AdvanceClock {
				t.Fatal("missing real original animation-clock call")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			compareFighter10Motion(t, state, actor, children, pool, step.Before)
			var err error
			state, err = StepFighter10Motion(state, sequence.Initial.Parameters)
			if err != nil {
				t.Fatal(sequence.Name, index, err)
			}
			actor, children, pool = compareFighter10Motion(t, state, actor, children, pool, step.After)
			total++
		}
		switch sequence.Milestone {
		case "action-transition":
			if state.Body.ActionKind == sequence.Kind {
				t.Fatal("no actual action transition")
			}
		case "frame-six":
			if state.Body.Clock.FrameIndex != 6 {
				t.Fatal("seven-frame motion window incomplete")
			}
		case "frame-fourteen":
			if state.Body.Clock.FrameIndex != 14 {
				t.Fatal("repeat frame14 milestone missing")
			}
		case "frame-thirty-one":
			if state.Body.Clock.FrameIndex != 31 {
				t.Fatal("counter and speed-decay window incomplete")
			}
		default:
			t.Fatal("unknown milestone")
		}
	}
	if total != 285 {
		t.Fatal("private continuous step count differs", total)
	}
	t.Logf("single-initial-state private streams=%d steps=%d; no animation record refresh or shared tail", len(sequences), total)
}

func TestFighter10MotionRetreatPoseRange(t *testing.T) {
	zipped, err := os.ReadFile("../data/animation-catalog.json.gz")
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
	var catalog struct {
		Frames []struct {
			CharacterID uint8 `json:"character_id"`
			Key         AnimationFrameKey
			Data        AnimationRecordData
		}
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	indices := map[uint8]bool{}
	for _, row := range catalog.Frames {
		if row.CharacterID == 10 && row.Key.Pose == 12 {
			indices[row.Key.Index] = true
			if row.Key.Index == 6 && row.Data.Flags&128 == 0 {
				t.Fatal("actual last retreat record is not terminal")
			}
		}
	}
	if len(indices) != 7 {
		t.Fatal("source10 retreat pose actual record count differs", len(indices))
	}
	for index := uint8(0); index < 7; index++ {
		if !indices[index] {
			t.Fatal("missing actual retreat frame", index)
		}
	}
}

func TestFighter10MotionErrorsAreAtomic(t *testing.T) {
	base := Fighter10MotionState{Aux48: int32(0x03123456)}
	got, err := StepFighter10Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported slot changed state")
	}
	base.Body.ActionKind = 0x11
	for _, index := range []uint8{8, 128, 255} {
		base.Body.Clock.FrameIndex = index
		got, err = StepFighter10Motion(base, JumpParameters{})
		if err == nil || got != base {
			t.Fatal("retreat read outside eight-entry structure", index)
		}
	}
	base.Body.Clock.Completed = 1
	got, err = StepFighter10Motion(base, JumpParameters{})
	if err != nil || got.Body.ActionKind != 0 {
		t.Fatal("completed retreat incorrectly consulted frame table", err)
	}
	base.Body.Clock.Completed = 0
	base.Pool.Cursor = 1
	for _, kind := range []uint8{0x47, 0x48, 0x49} {
		base.Body.ActionKind, base.Body.Clock.FrameIndex, base.Body.Clock.Ticks = kind, 4, 0
		base.Body.Y, base.Body.Ground = 300<<16, 320<<16
		got, err = StepFighter10Motion(base, JumpParameters{})
		if err == nil || got != base {
			t.Fatalf("late allocation error leaked slot %x body, +48 counter or pool", kind)
		}
	}
	base.Body.ActionKind, base.Body.Clock.FrameIndex, base.Body.Clock.Ticks = 0x49, 3, 0
	got, err = StepFighter10Motion(base, JumpParameters{})
	if err != nil || got != base {
		t.Fatal("unused invalid pool was accessed before ultimate frame4", err)
	}
}
