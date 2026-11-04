package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

type fighter3MotionCase struct {
	fighter0MotionCase
	SharedChargeBranch bool `json:"shared_charge_branch"`
}

func decodeFighter3Motion(t *testing.T, raw fighter0MotionSnapshot) (Fighter3MotionState, []byte, []byte, []byte) {
	t.Helper()
	base, actor, children, pool := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if actor[0x58] != 3 {
		t.Fatal("motion fixture is not actual source3")
	}
	state := Fighter3MotionState{ChuSpecialMotionState: base, PendingSound: raw.PendingSound}
	for i := range state.EffectExtras {
		b := pool[i*128:]
		state.EffectExtras[i] = ReactionEffectExtra{Owner: binary.BigEndian.Uint16(b[0x48:]), OffsetY: int16(binary.BigEndian.Uint16(b[0x56:])), Phase: b[0x5a]}
	}
	return state, actor, children, pool
}

func compareFighter3Motion(t *testing.T, state Fighter3MotionState, actor, children, pool []byte, want fighter0MotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	expected, wa, wc, wp := decodeFighter3Motion(t, want)
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
		t.Fatalf("source3 motion differs actor=%x children=%x pool=%x pending=%x/%x controls=%+v/%+v body=%+v/%+v", diff(actor, wa), diff(children, wc), diff(pool, wp), state.PendingSound, expected.PendingSound, state.Controls, expected.Controls, state.Body, expected.Body)
	}
	return actor, children, pool
}

// Original complete CPU cases and the failure ledger precede fighter3_motion.go.
func TestFighter3MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen := map[uint8]int{}
		external, auto := 0, 0
		count, wantExternal := walkChuActor(t, "fighter3-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter3MotionCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, actor, children, pool := decodeFighter3Motion(t, row.Before)
			if state.Body.ActionKind != row.Kind || row.SourceRegister != "0xffff8400" {
				t.Fatal("source3 original phase identity differs")
			}
			got, err := StepFighter3Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareFighter3Motion(t, got, actor, children, pool, row.After)
			if row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed {
				t.Fatal("pre-tail source3 motion consumed RNG")
			}
			if row.Kind == 0x24 {
				if !row.SharedChargeBranch {
					t.Fatal("auto24 did not actually enter original shared charge")
				}
				auto++
			}
			if len(row.Clears) != 0 {
				external++
				for _, clear := range row.Clears {
					if (clear.PC != "0x37c56" && clear.PC != "0x37cac") || clear.Address < "0xf80a" || clear.Address > "0xf80d" {
						t.Fatal("unidentified external audio mutation")
					}
				}
			} else if row.Before.SoundQueue != row.After.SoundQueue {
				t.Fatal("unexplained source3 audio queue change")
			}
			seen[row.Kind]++
		})
		if external != wantExternal {
			t.Fatal("missing independent audio observations")
		}
		if suffix == "boundaries" {
			for _, kind := range []uint8{0x0f, 0x11, 0x15, 0x17, 0x13, 0x21, 0x23, 0x24} {
				if seen[kind] == 0 {
					t.Fatal("missing accepted source3 phase", kind)
				}
			}
			if auto == 0 {
				t.Fatal("missing original auto-charge entries")
			}
		}
		t.Logf("source3 complete motion %s=%d phases=%v auto24=%d queueRows=%d", suffix, count, seen, auto, external)
	}
}

func TestFighter3MotionRealTimingStreams(t *testing.T) {
	steps, autoRows := 0, 0
	count, external := walkChuActor(t, "fighter3-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Kind       uint8
			Name, Pool string
			Initial    fighter0MotionSnapshot
			Steps      []fighter3MotionCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("invalid real source3 timing stream", err)
		}
		state, actor, children, pool := decodeFighter3Motion(t, stream.Initial)
		for index, step := range stream.Steps {
			for key, value := range step.External {
				switch key {
				case "held":
					state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
				case "pressed":
					state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
				default:
					t.Fatal("undeclared source3 external input")
				}
			}
			if !step.AdvanceClock {
				t.Fatal("missing original clock19c96 call")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			actor, children, pool = compareFighter3Motion(t, state, actor, children, pool, step.Before)
			var err error
			state, err = StepFighter3Motion(state, stream.Initial.Parameters)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			actor, children, pool = compareFighter3Motion(t, state, actor, children, pool, step.After)
			key := AnimationFrameKey{AnimationSet: stream.Initial.AnimationSet, ShapeSet: stream.Initial.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("independent source3 record selection differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing actual source3 timing record", key)
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			actor, children, pool = compareFighter3Motion(t, state, actor, children, pool, step.AfterRecord)
			if len(step.Clears) != 0 || step.RNGCalls != 0 {
				t.Fatal("private stream contained undeclared audio or rule mutation")
			}
			if step.Kind == 0x24 {
				autoRows++
			}
			steps++
		}
		if state.Body.ActionKind != 0 {
			t.Fatal("source3 stream did not recover to standing", stream.Name, state.Body.ActionKind)
		}
	})
	if count != 26 || steps < 1500 || autoRows == 0 || external != 0 {
		t.Fatal("missing independently initialized source3 timing streams", count, steps, autoRows, external)
	}
	t.Logf("single-initial source3 real-record streams=%d phases=%d auto24steps=%d", count, steps, autoRows)
}

func TestFighter3MotionComboAndAutoOutputs(t *testing.T) {
	combo, auto, failedCost, successfulCost := 0, 0, 0, 0
	walkChuActor(t, "fighter3-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter3MotionCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		before, after := row.Before, row.After
		if row.Kind == 0x21 {
			old, updated := uint32(before.Body.VelocityX), uint32(after.Body.VelocityX)
			if old&^uint32(1<<24) != updated&^uint32(1<<24) {
				t.Fatal("combo lock changed other long bits")
			}
			if before.Body.Clock.Completed == 1 && updated&(1<<24) != 0 {
				t.Fatal("completed combo did not clear lock after reset")
			}
			combo++
		}
		if row.Kind == 0x24 {
			want := before.Controls.Pressed
			if before.Body.Clock.FrameIndex == 4 && before.Body.Clock.Ticks == 16 {
				want |= ButtonA
			}
			if after.Controls.Pressed != want {
				t.Fatal("24 did not expose exact pressed.A OR or destroyed extension/other bits")
			}
			auto++
		}
		if row.Kind == 0x23 && before.Body.Clock.Completed == 1 {
			energy := int16(before.Energy >> 16)
			if after.Body.Clock.Completed != 0 {
				t.Fatal("normal complete flag was not consumed before cost check")
			}
			if energy < 4 {
				if after.Body.ActionKind != 0x23 || after.Body.Pose != before.Body.Pose || after.Energy != before.Energy {
					t.Fatal("normal cost4 failure rolled back consumption or entered standing")
				}
				failedCost++
			} else {
				if after.Body.ActionKind != 0x24 || after.Body.Pose != 0xa4 || after.Power != 255 || int16(after.Energy>>16) != energy-4 || uint16(after.Energy) != uint16(before.Energy) {
					t.Fatal("1900c cost/resource/next action differs")
				}
				if uint16(after.Body.VelocityX) != uint16(before.Body.VelocityX) || int16(after.Body.VelocityX>>16) != 32 || uint16(after.Body.VerticalVelocity) != uint16(before.Body.VerticalVelocity) || int16(after.Body.VerticalVelocity>>16) != -48 {
					t.Fatal("1900c offset highword overwrote low word")
				}
				if uint32(after.Body.Gravity) != uint32(before.Body.Gravity)&0x00ffffff {
					t.Fatal("1900c gravity mode clear changed lower24 bits")
				}
				successfulCost++
			}
		}
	})
	if combo < 26 || auto == 0 || failedCost == 0 || successfulCost == 0 {
		t.Fatal("missing exact combo/auto/normal cost edge cases", combo, auto, failedCost, successfulCost)
	}
}

func TestFighter3MotionErrorsAreAtomic(t *testing.T) {
	base := Fighter3MotionState{ChuSpecialMotionState: ChuSpecialMotionState{Energy: 223<<16 | 0xabcd, AlternateVelocity: 0x601234}, PendingSound: 0x97}
	base.Body.ActionKind = 0x22
	got, err := StepFighter3Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("separately owned air22 entered core motion")
	}
	base.Body.ActionKind = 0x21
	base.Body.VelocityX = 0x00abcdef
	base.Controls.Pressed = ButtonB
	for _, index := range []uint8{26, 128, 255} {
		base.Body.Clock.FrameIndex = index
		got, err = StepFighter3Motion(base, JumpParameters{})
		if err == nil || got != base {
			t.Fatal("late combo lookup error leaked newly set lock", index)
		}
	}
	base.Body.Clock.Completed = 1
	got, err = StepFighter3Motion(base, JumpParameters{})
	if err != nil || got.Body.ActionKind != 0 {
		t.Fatal("completed combo unnecessarily consulted frame table", err)
	}
	base.Body.Clock.Completed = 0
	base.Body.ActionKind = 0x13
	base.Body.Clock.FrameIndex = 1
	base.Body.Clock.Ticks = 0
	base.Pool.Cursor = 1
	got, err = StepFighter3Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("slide FX diagnostic leaked partial state")
	}
	base.Body.ActionKind = 0x23
	base.Body.Clock.FrameIndex = 0
	got, err = StepFighter3Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("ultimate FX diagnostic leaked partial state")
	}
}
