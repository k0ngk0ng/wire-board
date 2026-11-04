package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type reactionMotionSnapshot struct {
	ActorID    uint16 `json:"actor_id"`
	Body       FighterBodyState
	ActorBytes string `json:"actor_bytes"`
	Controls   FighterControls
	GuardMask  uint8  `json:"guard_mask"`
	StunTicks  uint16 `json:"stun_ticks"`
	Charge     int32
	Pool       struct {
		Cursor uint16
		Bytes  string
	}
	OccupiedLanes uint8  `json:"occupied_lanes"`
	PendingSound  uint8  `json:"pending_sound"`
	GlobalF568    uint16 `json:"global_f568"`
	GlobalF56A    uint16 `json:"global_f56a"`
	Left, Right   int16
	Parameters    JumpParameters
}
type reactionMotionCase struct {
	Kind          uint8
	Name          string
	Before, After reactionMotionSnapshot
}

func decodeReactionMotion(t *testing.T, raw reactionMotionSnapshot) (ReactionMotionState, []byte, []byte) {
	t.Helper()
	state := ReactionMotionState{ActorID: raw.ActorID, Body: raw.Body, Controls: raw.Controls, GuardMask: raw.GuardMask, StunTicks: raw.StunTicks, Charge: raw.Charge,
		OccupiedLanes: raw.OccupiedLanes, PendingSound: raw.PendingSound, GlobalF568: raw.GlobalF568, GlobalF56A: raw.GlobalF56A, Left: raw.Left, Right: raw.Right}
	var pool []byte
	state.Pool, pool = decodeEffectPool(t, raw.Pool.Cursor, raw.Pool.Bytes)
	for i := range state.EffectExtras {
		b := pool[i*128:]
		state.EffectExtras[i] = ReactionEffectExtra{Owner: binary.BigEndian.Uint16(b[0x48:]), OffsetY: int16(binary.BigEndian.Uint16(b[0x56:])), Phase: b[0x5a]}
	}
	actor, err := hex.DecodeString(raw.ActorBytes)
	if err != nil || len(actor) != 256 {
		t.Fatal("invalid original reaction actor", err)
	}
	return state, actor, pool
}

func encodeReactionMotion(state ReactionMotionState, previousActor, previousPool []byte) ([]byte, []byte) {
	a := encodeJumpBody(state.Body, previousActor)
	binary.BigEndian.PutUint16(a[0x78:], uint16(state.Body.Health))
	binary.BigEndian.PutUint16(a[0x7e:], uint16(state.Body.Auxiliary7E))
	binary.BigEndian.PutUint16(a[0x80:], uint16(state.Body.Auxiliary80))
	a[0xc8] = state.Body.TransitionDelay
	a[0x63] = state.GuardMask
	binary.BigEndian.PutUint16(a[0x84:], state.StunTicks)
	binary.BigEndian.PutUint32(a[0xb8:], uint32(state.Charge))
	a[0x71], a[0xc0] = uint8(state.Controls.Held), uint8(state.Controls.Held>>8)
	a[0x72], a[0xc1] = uint8(state.Controls.Pressed), uint8(state.Controls.Pressed>>8)
	a[0x73], a[0xc2] = uint8(state.Controls.Changed), uint8(state.Controls.Changed>>8)
	a[0x74], a[0x75] = state.Controls.RawHeld, state.Controls.RetainTicks
	p := encodeEffectPool(state.Pool, previousPool)
	for i, e := range state.EffectExtras {
		b := p[i*128:]
		binary.BigEndian.PutUint16(b[0x48:], e.Owner)
		binary.BigEndian.PutUint16(b[0x56:], uint16(e.OffsetY))
		b[0x5a] = e.Phase
	}
	return a, p
}

func compareReactionMotion(t *testing.T, got ReactionMotionState, beforeActor, beforePool []byte, want reactionMotionSnapshot) ([]byte, []byte) {
	t.Helper()
	expected, actor, pool := decodeReactionMotion(t, want)
	a, p := encodeReactionMotion(got, beforeActor, beforePool)
	if got != expected || !bytes.Equal(a, actor) || !bytes.Equal(p, pool) {
		diff := []int{}
		for i := range a {
			if a[i] != actor[i] {
				diff = append(diff, i)
			}
		}
		poolDiff := []int{}
		for i := range p {
			if p[i] != pool[i] {
				poolDiff = append(poolDiff, i)
			}
		}
		t.Fatalf("reaction differs: actor offsets=%x pool offsets=%x got=%+v want=%+v", diff, poolDiff, got, expected)
	}
	return a, p
}

func readReactionFixture(t *testing.T, name string, target any) {
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
		t.Fatal("missing complete unmodified reference evidence", err)
	}
}

// Failure scenarios, original fixtures and these tests precede reaction_motion.go.
func TestReactionMotionOriginalPhases(t *testing.T) {
	for _, name := range []string{"reaction-motion-oracle", "reaction-motion-boundaries"} {
		t.Run(name, func(t *testing.T) {
			var rows []reactionMotionCase
			readReactionFixture(t, name, &rows)
			if len(rows) == 0 {
				t.Fatal("missing original phases")
			}
			kinds := map[uint8]int{}
			for i, row := range rows {
				input, a, p := decodeReactionMotion(t, row.Before)
				if input.Body.ActionKind != row.Kind {
					t.Fatal("wrong original phase identity")
				}
				got, err := StepReactionMotion(input, row.Before.Parameters)
				if err != nil {
					t.Fatalf("reaction call %d %s: %v", i, row.Name, err)
				}
				compareReactionMotion(t, got, a, p, row.After)
				kinds[row.Kind]++
			}
			if name == "reaction-motion-boundaries" {
				for _, kind := range []uint8{3, 4, 5, 6, 7, 8, 9, 12, 13} {
					if kinds[kind] == 0 {
						t.Fatal("missing original action", kind)
					}
				}
			}
			t.Logf("original complete phases=%d kinds=%v", len(rows), kinds)
		})
	}
}

func TestReactionMotionSingleInitialState(t *testing.T) {
	var sequences []struct {
		Kind    uint8
		Pool    string
		Initial reactionMotionSnapshot
		Steps   []reactionMotionCase
	}
	readReactionFixture(t, "reaction-motion-sequences", &sequences)
	if len(sequences) != 20 {
		t.Fatal("missing original phase trajectories")
	}
	total := 0
	for _, sequence := range sequences {
		state, a, p := decodeReactionMotion(t, sequence.Initial)
		for index, step := range sequence.Steps {
			var err error
			state, err = StepReactionMotion(state, sequence.Initial.Parameters)
			if err != nil {
				t.Fatal(sequence.Kind, index, err)
			}
			a, p = compareReactionMotion(t, state, a, p, step.After)
			total++
		}
		if state.Body.ActionKind == sequence.Kind {
			t.Fatal("trajectory did not reach its action transition")
		}
	}
	if total != 368 {
		t.Fatal("missing continuous trajectory steps", total)
	}
}

func TestReactionMotionErrorsAreAtomic(t *testing.T) {
	input := ReactionMotionState{Body: FighterBodyState{GroundActionState: GroundActionState{ActionKind: 0}}, PendingSound: 91, GlobalF568: 7, GlobalF56A: 99}
	got, err := StepReactionMotion(input, JumpParameters{})
	if err == nil || got != input {
		t.Fatal("unsupported action changed state")
	}
	input.Pool.Cursor = 1
	for _, body := range []FighterBodyState{
		{GroundActionState: GroundActionState{ActionKind: 8, Clock: AnimationClock{FrameIndex: 0, Ticks: 1}}},
		{GroundActionState: GroundActionState{ActionKind: 5, Pose: 0x25}, Y: 10, Ground: 10, Gravity: 1},
		{GroundActionState: GroundActionState{ActionKind: 5, Pose: 0x24}, Y: 10, Ground: 10, Gravity: 1},
	} {
		input.Body = body
		input.Controls.Pressed = ButtonY
		got, err = StepReactionMotion(input, JumpParameters{})
		if err == nil || got != input {
			t.Fatal("failed allocation/control leaked partial state")
		}
	}
}
