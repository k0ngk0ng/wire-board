package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type chuSpecialMotionSnapshot struct {
	ObjectID          uint16 `json:"object_id"`
	Body              FighterBodyState
	ActorBytes        string `json:"actor_bytes"`
	Controls          FighterControls
	Energy            int32
	Power             int16
	AlternateVelocity int32 `json:"alternate_velocity"`
	ContactFlags      uint8 `json:"contact_flags"`
	Children          string
	Pool              struct {
		Cursor uint16
		Bytes  string
	}
	Parameters JumpParameters
}
type chuSpecialMotionCase struct {
	Kind          uint8
	Name          string
	Before, After chuSpecialMotionSnapshot
	AdvanceClock  bool             `json:"advance_clock"`
	ExternalInput map[string]uint8 `json:"external_input"`
}

func decodeChuSpecialMotion(t *testing.T, raw chuSpecialMotionSnapshot) (ChuSpecialMotionState, []byte, []byte, []byte) {
	t.Helper()
	state := ChuSpecialMotionState{ObjectID: raw.ObjectID, Body: raw.Body, Controls: raw.Controls, Energy: raw.Energy, Power: raw.Power,
		AlternateVelocity: raw.AlternateVelocity, ContactFlags: raw.ContactFlags}
	var pool []byte
	state.Pool, pool = decodeEffectPool(t, raw.Pool.Cursor, raw.Pool.Bytes)
	actor, err := hex.DecodeString(raw.ActorBytes)
	if err != nil || len(actor) != 256 {
		t.Fatal("invalid original Chu actor", err)
	}
	state.TargetBit = actor[0x70]
	children, err := hex.DecodeString(raw.Children)
	if err != nil || len(children) != 768 {
		t.Fatal("invalid original Chu children", err)
	}
	for i := range state.Children {
		b := children[i*256:]
		w := func(o int) uint16 { return binary.BigEndian.Uint16(b[o:]) }
		l := func(o int) int32 { return int32(binary.BigEndian.Uint32(b[o:])) }
		state.Children[i] = ChuSpecialChild{Behavior: uint32(l(0)), Size: w(4), Stage: w(6), VelocityX: l(0x10), VerticalVelocity: l(0x14),
			Attributes: w(0x24), Layer: b[0x26], Depth: w(0x3e), X: l(0x40), Y: l(0x44), Facing: b[0x61], Team: b[0x6e], TargetBit: b[0x70], Auxiliary76: b[0x76],
			Power: int16(w(0x7c)), Pose: b[0x5f], TargetMask: uint32(l(0x94)), Ground: l(0x9c)}
	}
	return state, actor, children, pool
}

func encodeChuSpecialMotion(state ChuSpecialMotionState, previousActor, previousChildren, previousPool []byte) ([]byte, []byte, []byte) {
	a := encodeJumpBody(state.Body, previousActor)
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(a[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(a[0x80:], uint16(state.Body.Auxiliary80))
	a[0xc8] = state.Body.TransitionDelay
	binary.BigEndian.PutUint32(a[0xb8:], uint32(state.Energy))
	binary.BigEndian.PutUint16(a[0x7c:], uint16(state.Power))
	binary.BigEndian.PutUint32(a[0x48:], uint32(state.AlternateVelocity))
	a[0x89] = state.ContactFlags
	a[0x70] = state.TargetBit
	a[0x71], a[0xc0] = uint8(state.Controls.Held), uint8(state.Controls.Held>>8)
	a[0x72], a[0xc1] = uint8(state.Controls.Pressed), uint8(state.Controls.Pressed>>8)
	a[0x73], a[0xc2] = uint8(state.Controls.Changed), uint8(state.Controls.Changed>>8)
	a[0x74], a[0x75] = state.Controls.RawHeld, state.Controls.RetainTicks
	children := append([]byte(nil), previousChildren...)
	for i, c := range state.Children {
		b := children[i*256:]
		w := func(o int, v uint16) { binary.BigEndian.PutUint16(b[o:], v) }
		l := func(o int, v int32) { binary.BigEndian.PutUint32(b[o:], uint32(v)) }
		l(0, int32(c.Behavior))
		w(4, c.Size)
		w(6, c.Stage)
		l(0x10, c.VelocityX)
		l(0x14, c.VerticalVelocity)
		w(0x24, c.Attributes)
		b[0x26] = c.Layer
		w(0x3e, c.Depth)
		l(0x40, c.X)
		l(0x44, c.Y)
		b[0x61], b[0x6e], b[0x70], b[0x76], b[0x5f] = c.Facing, c.Team, c.TargetBit, c.Auxiliary76, c.Pose
		w(0x7c, uint16(c.Power))
		l(0x94, int32(c.TargetMask))
		l(0x9c, c.Ground)
	}
	return a, children, encodeEffectPool(state.Pool, previousPool)
}

func compareChuSpecialMotion(t *testing.T, got ChuSpecialMotionState, a, c, p []byte, want chuSpecialMotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	expected, wa, wc, wp := decodeChuSpecialMotion(t, want)
	a, c, p = encodeChuSpecialMotion(got, a, c, p)
	if got != expected || !bytes.Equal(a, wa) || !bytes.Equal(c, wc) || !bytes.Equal(p, wp) {
		diff := func(a, b []byte) []int {
			out := []int{}
			for i := range a {
				if a[i] != b[i] {
					out = append(out, i)
				}
			}
			return out
		}
		t.Fatalf("Chu special differs: actor=%x children=%x pool=%x body=%+v want=%+v", diff(a, wa), diff(c, wc), diff(p, wp), got.Body, expected.Body)
	}
	return a, c, p
}

func readChuSpecialMotion(t *testing.T, name string, target any) {
	t.Helper()
	readLaneMotion(t, name, target)
	raw, err := os.ReadFile("testdata/" + name + "-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Baseline   bool `json:"frames_match_original_core"`
		Incomplete int  `json:"incomplete_calls"`
		Records    int
	}
	if err = json.Unmarshal(raw, &m); err != nil || !m.Baseline || m.Incomplete != 0 || m.Records == 0 {
		t.Fatal("incomplete original special evidence", err)
	}
}

// Original full parent/three-child/pool fixtures and tests precede implementation.
func TestChuSpecialMotionOriginalPhases(t *testing.T) {
	for _, name := range []string{"chu-special-motion-oracle", "chu-special-motion-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []chuSpecialMotionCase
			readChuSpecialMotion(t, name, &rows)
			if len(rows) == 0 {
				t.Fatal("missing Chu special phases")
			}
			kinds := map[uint8]int{}
			for i, row := range rows {
				input, a, c, p := decodeChuSpecialMotion(t, row.Before)
				if input.Body.ActionKind != row.Kind {
					t.Fatal("phase identity differs")
				}
				got, err := StepChuSpecialMotion(input, row.Before.Parameters)
				if err != nil {
					t.Fatal(i, row.Name, err)
				}
				compareChuSpecialMotion(t, got, a, c, p, row.After)
				kinds[row.Kind]++
			}
			if name == "chu-special-motion-boundaries" {
				for _, kind := range []uint8{0xf, 0x14, 0x15, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46} {
					if kinds[kind] == 0 {
						t.Fatal("missing action", kind)
					}
				}
			}
			t.Logf("complete original special phases=%d kinds=%v", len(rows), kinds)
		})
	}
}

func TestChuSpecialMotionSingleInitialState(t *testing.T) {
	var sequences []struct {
		Kind      uint8
		Pool      string
		Initial   chuSpecialMotionSnapshot
		Steps     []chuSpecialMotionCase
		Milestone string
	}
	readChuSpecialMotion(t, "chu-special-motion-sequences", &sequences)
	if len(sequences) != 22 {
		t.Fatal("missing single-initial-state trajectories")
	}
	total := 0
	for _, sequence := range sequences {
		state, a, c, p := decodeChuSpecialMotion(t, sequence.Initial)
		for _, step := range sequence.Steps {
			for key, value := range step.ExternalInput {
				if key != "held" {
					t.Fatal("undeclared external input")
				}
				state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
			}
			if !step.AdvanceClock {
				t.Fatal("missing declared original clock step")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			// Compare the independently advanced clock before the motion phase too.
			compareChuSpecialMotion(t, state, a, c, p, step.Before)
			var err error
			state, err = StepChuSpecialMotion(state, sequence.Initial.Parameters)
			if err != nil {
				t.Fatal(err)
			}
			a, c, p = compareChuSpecialMotion(t, state, a, c, p, step.After)
			total++
		}
		if sequence.Milestone == "action-transition" {
			if state.Body.ActionKind == sequence.Kind {
				t.Fatal("no action transition")
			}
		} else if sequence.Milestone == "projectile-created" {
			if state.Children[0].Size == 0 {
				t.Fatal("no projectile")
			}
		} else {
			t.Fatal("unknown trajectory milestone")
		}
	}
	if total != 202 {
		t.Fatal("missing continuous steps", total)
	}
}

func TestChuSpecialMotionErrorsAreAtomic(t *testing.T) {
	base := ChuSpecialMotionState{Body: FighterBodyState{GroundActionState: GroundActionState{ActionKind: 0}}, Energy: 123}
	got, err := StepChuSpecialMotion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("invalid action changed state")
	}
	base.Body.ActionKind = 0xf
	base.Body.Y = base.Body.Ground
	base.Body.Clock.Ticks = 0
	base.Pool.Cursor = 1
	got, err = StepChuSpecialMotion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("dust allocation failure leaked state")
	}
	base.Body.ActionKind = 0x41
	base.Body.Clock.FrameIndex = 1
	base.Body.Clock.Ticks = 0
	got, err = StepChuSpecialMotion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("trail allocation failure leaked state")
	}
	base.Pool.Cursor = 0
	base.Body.ActionKind = 0x43
	for _, index := range []uint8{0, 3} {
		base.Body.Clock.FrameIndex = index
		for _, power := range []int16{-1, 256, 32767, -32768} {
			base.Power = power
			got, err = StepChuSpecialMotion(base, JumpParameters{})
			if err == nil || got != base {
				t.Fatal("unsupported charge table index changed state")
			}
		}
	}
}
