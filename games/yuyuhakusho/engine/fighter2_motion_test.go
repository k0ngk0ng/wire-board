package engine

import (
	"encoding/json"
	"testing"
)

func decodeFighter2Motion(t *testing.T, raw fighter0MotionSnapshot) (ChuSpecialMotionState, []byte, []byte, []byte) {
	t.Helper()
	state, actor, children, pool := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if actor[0x58] != 2 {
		t.Fatal("motion fixture did not use actual source2")
	}
	return state, actor, children, pool
}

// Complete original CPU fixtures and failure ledger precede fighter2_motion.go.
func TestFighter2MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen := map[uint8]int{}
		external := 0
		count, wantExternal := walkChuActor(t, "fighter2-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter0MotionCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, actor, children, pool := decodeFighter2Motion(t, row.Before)
			if state.Body.ActionKind != row.Kind || row.SourceRegister != "0xffff8400" {
				t.Fatal("original source2 kind/register differs")
			}
			got, err := StepFighter2Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareChuSpecialMotion(t, got, actor, children, pool, row.After.chuSpecialMotionSnapshot)
			if row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed {
				t.Fatal("pre-tail source2 phase consumed RNG")
			}
			if row.Before.PendingSound != row.After.PendingSound {
				t.Fatal("unexpected source2 motion sound-request write")
			}
			if len(row.Clears) != 0 {
				external++
				for _, c := range row.Clears {
					if (c.PC != "0x37c56" && c.PC != "0x37cac") || c.Address < "0xf80a" || c.Address > "0xf80d" {
						t.Fatal("unidentified independent queue consumer")
					}
				}
			} else if row.Before.SoundQueue != row.After.SoundQueue {
				t.Fatal("unexplained source2 sound queue change")
			}
			seen[row.Kind]++
		})
		if external != wantExternal {
			t.Fatal("missing actual audio evidence")
		}
		if suffix == "boundaries" {
			for _, kind := range []uint8{0x0f, 0x11, 0x1e, 0x1f, 0x20} {
				if seen[kind] == 0 {
					t.Fatal("missing original source2 phase", kind)
				}
			}
		}
		t.Logf("source2 original motion %s=%d kinds=%v externalQueueRows=%d", suffix, count, seen, external)
	}
}

func TestFighter2MotionRealTimingStreams(t *testing.T) {
	steps := 0
	count, external := walkChuActor(t, "fighter2-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Kind       uint8
			Name, Pool string
			Initial    fighter0MotionSnapshot
			Steps      []fighter0MotionCase
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("incomplete source2 real clock stream", err)
		}
		state, actor, children, pool := decodeFighter2Motion(t, stream.Initial)
		for index, step := range stream.Steps {
			if !step.AdvanceClock || len(step.External) != 0 {
				t.Fatal("undeclared source2 stream input or missing real19c96")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			actor, children, pool = compareChuSpecialMotion(t, state, actor, children, pool, step.Before.chuSpecialMotionSnapshot)
			var err error
			state, err = StepFighter2Motion(state, stream.Initial.Parameters)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			actor, children, pool = compareChuSpecialMotion(t, state, actor, children, pool, step.After.chuSpecialMotionSnapshot)
			key := AnimationFrameKey{AnimationSet: stream.Initial.AnimationSet, ShapeSet: stream.Initial.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("independent source2 record selection differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing original source2 timing record", key)
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			actor, children, pool = compareChuSpecialMotion(t, state, actor, children, pool, step.AfterRecord.chuSpecialMotionSnapshot)
			if len(step.Clears) != 0 || step.RNGCalls != 0 {
				t.Fatal("private stream external mutation")
			}
			steps++
		}
		if state.Body.ActionKind != 0 {
			t.Fatal("source2 stream did not recover to standing", stream.Name, state.Body.ActionKind)
		}
	})
	if count != 18 || steps < 750 || external != 0 {
		t.Fatal("missing source2 independently initialized streams", count, steps, external)
	}
	t.Logf("source2 real record timing streams=%d phases=%d; no tail/child/FX callbacks", count, steps)
}

func TestFighter2MotionChildPrefixesAndCursor(t *testing.T) {
	sword, rush, scatter, busy, wrap := 0, 0, 0, 0, 0
	walkChuActor(t, "fighter2-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter0MotionCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		before, _, _, _ := decodeFighter2Motion(t, row.Before)
		after, _, _, _ := decodeFighter2Motion(t, row.After)
		if row.Kind == 0x1e && before.Body.Clock.Completed != 1 && before.Body.Clock.FrameIndex == 4 && before.Body.Clock.Ticks == 0 {
			if before.Children[0].Size == 0 {
				old, new := before.Children[0], after.Children[0]
				if new.Behavior != 0x12dd4 || new.Pose != 12 || new.VelocityX != old.VelocityX || new.VerticalVelocity != old.VerticalVelocity || new.Power != old.Power {
					t.Fatal("1e initialized unwritten child speed/Power or wrong kind")
				}
				if new.Attributes&0x6000 != 0x4000 {
					t.Fatal("1e did not use original OR4000 attributes")
				}
				sword++
			} else if before != after {
				t.Fatal("busy 1e modified state")
			}
		}
		if row.Kind == 0x1f {
			if before.Body.VelocityX != after.Body.VelocityX {
				t.Fatal("constant-displacement 1f overwrote stored VX")
			}
			rush++
		}
		if row.Kind == 0x20 && before.Body.Clock.Completed != 1 && before.Body.Clock.FrameIndex == 5 && before.Body.Clock.Ticks <= 2 {
			cursor := uint16(uint32(before.Body.Gravity)>>16) + 0x100
			if cursor == 0x300 {
				cursor = 0
			}
			if cursor > 0x200 || cursor&255 != 0 {
				t.Fatal("fixture used invalid derived scatter cursor")
			}
			vy := [3]int32{0, -0x30000, 0x30000}[before.Body.Clock.Ticks]
			if after.Body.VerticalVelocity != vy || uint16(after.Body.Gravity) != uint16(before.Body.Gravity) || uint16(uint32(after.Body.Gravity)>>16) != cursor {
				t.Fatal("busy/emit cursor or VY ordering differs")
			}
			index := cursor / 0x100
			if before.Children[index].Size != 0 {
				if before.Children != after.Children {
					t.Fatal("busy scatter changed a child")
				}
				busy++
			} else {
				old, new := before.Children[index], after.Children[index]
				vx := before.Body.VelocityX
				if before.Body.Facing != 0 {
					vx = -vx
				}
				if new.Behavior != 0x12c96 || new.Pose != 13 || new.VelocityX != vx || new.VerticalVelocity != vy || new.Power != old.Power || new.Attributes&0x6000 != 0x2000 {
					t.Fatal("20 reused wrong prefix, speed width, Power or OR4000 attributes")
				}
				scatter++
			}
			if uint16(uint32(before.Body.Gravity)>>16) == 0xff00 {
				if cursor != 0 {
					t.Fatal("valid highword wrap did not select +100")
				}
				wrap++
			}
		}
	})
	if sword == 0 || rush == 0 || scatter == 0 || busy == 0 || wrap == 0 {
		t.Fatal("missing exact child/cursor side effects", sword, rush, scatter, busy, wrap)
	}
}

func TestFighter2MotionErrorsAreAtomic(t *testing.T) {
	base := ChuSpecialMotionState{Energy: 223<<16 | 0xabcd, AlternateVelocity: 0x601234}
	base.Body.ActionKind = 0x15
	got, err := StepFighter2Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported source2 phase changed state")
	}
	base.Body.ActionKind = 0x20
	base.Body.Clock.FrameIndex = 5
	base.Body.Gravity = 0x0300abcd
	base.Body.VerticalVelocity = 0x76543210
	got, err = StepFighter2Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("invalid derived cursor leaked preceding VY/gravity writes")
	}
	base.Body.Clock.Ticks = 3
	got, err = StepFighter2Motion(base, JumpParameters{})
	if err != nil || got != base {
		t.Fatal("unused invalid cursor was validated before tick gate", err)
	}
	base.Body.Clock.Completed = 1
	got, err = StepFighter2Motion(base, JumpParameters{})
	if err != nil || got.Body.ActionKind != 0 {
		t.Fatal("completed scatter consulted an unused cursor", err)
	}
	base.Body.Clock.Completed = 0
	base.Body.ActionKind = 0x1f
	base.Body.Clock.FrameIndex = 4
	base.Body.Clock.Ticks = 0
	base.Pool.Cursor = 1
	got, err = StepFighter2Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("trail allocator diagnostic leaked movement or frame skip")
	}
}
