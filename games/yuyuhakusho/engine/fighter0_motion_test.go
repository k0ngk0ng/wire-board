package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

type fighter0MotionSnapshot struct {
	chuSpecialMotionSnapshot
	PendingSound uint8    `json:"pending_sound"`
	SoundQueue   [4]uint8 `json:"sound_queue"`
	RandomSeed   uint32   `json:"random_seed"`
	AnimationSet uint32   `json:"animation_set"`
	ShapeSet     uint32   `json:"shape_set"`
}

type fighter0MotionCase struct {
	Kind           uint8
	Name, Scenario string
	SourceRegister string `json:"source_register"`
	Before, After  fighter0MotionSnapshot
	Allocations    []int
	RNGCalls       int                            `json:"rng_calls"`
	Clears         []struct{ PC, Address string } `json:"external_queue_clears"`
	AdvanceClock   bool                           `json:"advance_clock"`
	External       map[string]uint8               `json:"external_input"`
	RecordAfter    AnimationFrameKey              `json:"record_after"`
	AfterRecord    fighter0MotionSnapshot         `json:"after_record"`
}

func decodeFighter0Motion(t *testing.T, raw fighter0MotionSnapshot) (Fighter0MotionState, []byte, []byte, []byte) {
	t.Helper()
	base, actor, children, pool := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if actor[0x58] != 0 {
		t.Fatal("motion source is not Genkai/source0")
	}
	state := Fighter0MotionState{ChuSpecialMotionState: base, PendingSound: raw.PendingSound}
	for i := range state.EffectExtras {
		b := pool[i*128:]
		state.EffectExtras[i] = ReactionEffectExtra{Owner: binary.BigEndian.Uint16(b[0x48:]), OffsetY: int16(binary.BigEndian.Uint16(b[0x56:])), Phase: b[0x5a]}
	}
	return state, actor, children, pool
}

func compareFighter0Motion(t *testing.T, state Fighter0MotionState, actor, children, pool []byte, want fighter0MotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	expected, wa, wc, wp := decodeFighter0Motion(t, want)
	actor, children, pool = encodeChuSpecialMotion(state.ChuSpecialMotionState, actor, children, pool)
	for i, extra := range state.EffectExtras {
		b := pool[i*128:]
		binary.BigEndian.PutUint16(b[0x48:], extra.Owner)
		binary.BigEndian.PutUint16(b[0x56:], uint16(extra.OffsetY))
		b[0x5a] = extra.Phase
	}
	if state != expected || !bytes.Equal(actor, wa) || !bytes.Equal(children, wc) || !bytes.Equal(pool, wp) {
		diff := func(a, b []byte) []int {
			var out []int
			for i := range a {
				if a[i] != b[i] {
					out = append(out, i)
				}
			}
			return out
		}
		t.Fatalf("source0 motion differs actor=%x children=%x pool=%x pending=%x/%x body=%+v/%+v", diff(actor, wa), diff(children, wc), diff(pool, wp), state.PendingSound, expected.PendingSound, state.Body, expected.Body)
	}
	return actor, children, pool
}

// The failure ledger and original CPU fixtures precede fighter0_motion.go.
func TestFighter0MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen := map[uint8]int{}
		allocations, external := 0, 0
		count, wantExternal := walkChuActor(t, "fighter0-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter0MotionCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, actor, children, pool := decodeFighter0Motion(t, row.Before)
			if row.Kind != state.Body.ActionKind || row.SourceRegister != "0xffff8400" {
				t.Fatal("original action/register identity differs")
			}
			got, err := StepFighter0Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareFighter0Motion(t, got, actor, children, pool, row.After)
			if row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed {
				t.Fatal("pre-tail motion unexpectedly consumed RNG")
			}
			if len(row.Clears) != 0 {
				external++
				for _, clear := range row.Clears {
					if (clear.PC != "0x37c56" && clear.PC != "0x37cac") || clear.Address < "0xf80a" || clear.Address > "0xf80d" {
						t.Fatal("unidentified independent audio queue write")
					}
				}
			} else if row.Before.SoundQueue != row.After.SoundQueue {
				t.Fatal("unexplained audio queue mutation")
			}
			seen[row.Kind]++
			allocations += len(row.Allocations)
		})
		if external != wantExternal {
			t.Fatal("missing independent audio evidence")
		}
		if suffix == "boundaries" {
			for _, action := range []uint8{0x0e, 0x11, 0x15, 0x16, 0x17, 0x18, 0x19} {
				if seen[action] == 0 {
					t.Fatal("missing phase", action)
				}
			}
		}
		t.Logf("complete original source0 motion %s calls=%d actions=%v allocations=%d naturalQueueEvents=%d", suffix, count, seen, allocations, external)
	}
}

func TestFighter0MotionRealTimingStreams(t *testing.T) {
	steps := 0
	count, external := walkChuActor(t, "fighter0-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Kind       uint8
			Name, Pool string
			Initial    fighter0MotionSnapshot
			Steps      []fighter0MotionCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("incomplete source0 motion stream", err)
		}
		state, actor, children, pool := decodeFighter0Motion(t, stream.Initial)
		for index, step := range stream.Steps {
			for key, value := range step.External {
				switch key {
				case "held":
					state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
				case "pressed":
					state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
				default:
					t.Fatal("undeclared external motion input")
				}
			}
			if !step.AdvanceClock {
				t.Fatal("missing actual original19c96 call")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			actor, children, pool = compareFighter0Motion(t, state, actor, children, pool, step.Before)
			var err error
			state, err = StepFighter0Motion(state, stream.Initial.Parameters)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			actor, children, pool = compareFighter0Motion(t, state, actor, children, pool, step.After)
			key := AnimationFrameKey{AnimationSet: stream.Initial.AnimationSet, ShapeSet: stream.Initial.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("independent source0 record selection differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing actual source0 timing record", key)
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			actor, children, pool = compareFighter0Motion(t, state, actor, children, pool, step.AfterRecord)
			if len(step.Clears) != 0 || step.RNGCalls != 0 {
				t.Fatal("private stream contained external rule or audio mutation")
			}
			steps++
		}
		if state.Body.ActionKind != 0 {
			t.Fatal("source0 stream did not recover to standing", stream.Name, state.Body.ActionKind)
		}
	})
	if count != 20 || steps < 1000 || external != 0 {
		t.Fatal("missing real record timing streams", count, steps, external)
	}
	t.Logf("single-initial-state streams=%d complete phases=%d; actual readonly record timing, no shared tail or child/FX updates", count, steps)
}

func TestFighter0MotionFourEffectsAndPartialChildren(t *testing.T) {
	four, partial, rotation, refund := 0, 0, 0, 0
	walkChuActor(t, "fighter0-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter0MotionCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		if len(row.Name) >= 8 && row.Name[:8] == "four-fx-" {
			if len(row.Allocations) != 4 {
				t.Fatal("four independent FX allocations were not executed")
			}
			before := attackContactBytes(t, row.Before.Pool.Bytes, 2048)
			after := attackContactBytes(t, row.After.Pool.Bytes, 2048)
			for index, slot := range row.Allocations {
				if slot < 0 {
					continue
				}
				want := before[slot*128+0x5a]
				if index == 0 || index == 3 {
					want = 8
				}
				if after[slot*128+0x5a] != want {
					t.Fatal("only first/fourth allocations may change Duration", row.Name, index, slot)
				}
			}
			four++
		}
		if len(row.Name) >= 14 && row.Name[:14] == "partial-child-" {
			before := attackContactBytes(t, row.Before.Children, 768)
			after := attackContactBytes(t, row.After.Children, 768)
			if binary.BigEndian.Uint16(before[516:]) != 0 {
				if binary.BigEndian.Uint32(after[:]) != 0x1313e || row.After.Body.Clock.FrameIndex != 4 || row.After.Power == 0 || !bytes.Equal(before[512:], after[512:]) {
					t.Fatal("busy release slot rolled back completed main projectile or cleared power")
				}
				partial++
			}
		}
		if row.Kind == 0x18 && row.Before.Body.Clock.Ticks == 0 && row.Before.Body.Clock.Completed != 1 && row.Before.Body.Clock.FrameIndex == 3 {
			if uint16(row.Before.Body.VelocityX) != uint16(row.After.Body.VelocityX) {
				t.Fatal("scatter cursor update changed low word")
			}
			rotation++
		}
		if row.Kind == 0x19 && row.Before.Body.Clock.Completed != 1 && row.Before.Body.Clock.FrameIndex == 31 && row.Before.Body.Clock.Ticks == 0 {
			if uint16(row.Before.Energy) != uint16(row.After.Energy) || int16(row.After.Energy>>16) != int16(row.Before.Energy>>16)+16 {
				t.Fatal("frame31 energy refund changed fraction or failed word wrapping")
			}
			refund++
		}
	})
	if four < 6 || partial != 4 || rotation == 0 || refund == 0 {
		t.Fatal("missing compound normal side-effect cases", four, partial, rotation, refund)
	}
}

func TestFighter0MotionErrorsAreAtomic(t *testing.T) {
	base := Fighter0MotionState{ChuSpecialMotionState: ChuSpecialMotionState{Energy: 223<<16 | 0xabcd, AlternateVelocity: 0x601234}, PendingSound: 0x97}
	got, err := StepFighter0Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported source0 phase changed state")
	}
	base.Body.ActionKind = 0x16
	for _, index := range []uint8{13, 128, 255} {
		base.Body.Clock.FrameIndex = index
		got, err = StepFighter0Motion(base, JumpParameters{})
		if err == nil || got != base {
			t.Fatal("source0 frame table out-of-range write leaked", index)
		}
	}
	base.Body.Clock.Completed = 1
	got, err = StepFighter0Motion(base, JumpParameters{})
	if err != nil || got.Body.ActionKind != 0 {
		t.Fatal("completed source0 phase incorrectly read frame table", err)
	}
	base.Body.Clock.Completed = 0
	base.Body.Clock.FrameIndex = 3
	base.Pool.Cursor = 1
	got, err = StepFighter0Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("four-FX allocation diagnostic leaked partial motion/extras")
	}
	base.Pool.Cursor = 0
	base.Body.ActionKind = 0x18
	base.Body.VelocityX = 0x0300abcd
	base.Body.VerticalVelocity = 0x76543210
	got, err = StepFighter0Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("late invalid child cursor leaked velocity/cursor writes")
	}
	base.Body.Clock.Ticks = 1
	got, err = StepFighter0Motion(base, JumpParameters{})
	if err != nil || got != base {
		t.Fatal("unused invalid cursor was read before ticks gate", err)
	}
}
