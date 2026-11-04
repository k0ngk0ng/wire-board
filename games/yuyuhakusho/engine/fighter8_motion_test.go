package engine

import (
	"encoding/json"
	"testing"
)

func decodeFighter8Motion(t *testing.T, raw fighter0MotionSnapshot) (ChuSpecialMotionState, []byte, []byte, []byte) {
	t.Helper()
	s, a, c, p := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if a[0x58] != 8 {
		t.Fatal("motion fixture is not actual source8")
	}
	return s, a, c, p
}

// Actual CPU matrices/streams and the failure ledger precede fighter8_motion.go.
func TestFighter8MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen := map[uint8]int{}
		external := 0
		count, wantExternal := walkChuActor(t, "fighter8-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter0MotionCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, a, c, p := decodeFighter8Motion(t, row.Before)
			if state.Body.ActionKind != row.Kind || row.SourceRegister != "0xffff8400" {
				t.Fatal("original source8 identity differs")
			}
			got, err := StepFighter8Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareChuSpecialMotion(t, got, a, c, p, row.After.chuSpecialMotionSnapshot)
			if row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed || row.Before.PendingSound != row.After.PendingSound {
				t.Fatal("source8 pre-tail RNG/request changed")
			}
			if len(row.Clears) != 0 {
				external++
				for _, clear := range row.Clears {
					if (clear.PC != "0x37c56" && clear.PC != "0x37cac") || clear.Address < "0xf80a" || clear.Address > "0xf80d" {
						t.Fatal("unidentified audio queue consumer")
					}
				}
			} else if row.Before.SoundQueue != row.After.SoundQueue {
				t.Fatal("unexplained audio queue difference")
			}
			seen[row.Kind]++
		})
		if external != wantExternal {
			t.Fatal("missing external source8 audio evidence")
		}
		if suffix == "boundaries" {
			for _, kind := range []uint8{0x0f, 0x11, 0x3e, 0x3f, 0x40} {
				if seen[kind] == 0 {
					t.Fatal("missing original source8 phase", kind)
				}
			}
		}
		t.Logf("source8 original motion %s=%d kinds=%v queueRows=%d", suffix, count, seen, external)
	}
}

func TestFighter8MotionRealTimingStreams(t *testing.T) {
	steps, air, stand := 0, 0, 0
	count, external := walkChuActor(t, "fighter8-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Kind       uint8
			Name, Pool string
			Initial    fighter0MotionSnapshot
			Steps      []fighter0MotionCase
			Terminal   uint8
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("incomplete original source8 stream", err)
		}
		state, a, c, p := decodeFighter8Motion(t, stream.Initial)
		for index, step := range stream.Steps {
			for key, value := range step.External {
				switch key {
				case "held":
					state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
				case "pressed":
					state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
				default:
					t.Fatal("undeclared source8 external input")
				}
			}
			if !step.AdvanceClock {
				t.Fatal("missing original clock19c96")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			a, c, p = compareChuSpecialMotion(t, state, a, c, p, step.Before.chuSpecialMotionSnapshot)
			var err error
			state, err = StepFighter8Motion(state, stream.Initial.Parameters)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			a, c, p = compareChuSpecialMotion(t, state, a, c, p, step.After.chuSpecialMotionSnapshot)
			key := AnimationFrameKey{AnimationSet: stream.Initial.AnimationSet, ShapeSet: stream.Initial.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("source8 independent timing-record key differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing actual source8 timing record", key)
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			a, c, p = compareChuSpecialMotion(t, state, a, c, p, step.AfterRecord.chuSpecialMotionSnapshot)
			if len(step.Clears) != 0 || step.RNGCalls != 0 {
				t.Fatal("unexpected private source8 external mutation")
			}
			steps++
		}
		if state.Body.ActionKind != stream.Terminal {
			t.Fatal("source8 did not reach its actual recovery", stream.Name, state.Body.ActionKind, stream.Terminal)
		}
		if stream.Terminal == 2 {
			air++
		} else if stream.Terminal == 0 {
			stand++
		} else {
			t.Fatal("unknown motion recovery")
		}
	})
	if count != 20 || steps < 2000 || air != 6 || stand != 14 || external != 0 {
		t.Fatal("missing source8 real-record flows", count, steps, air, stand, external)
	}
	t.Logf("source8 single-initial real-clock streams=%d phases=%d airRecoveries=%d standing=%d", count, steps, air, stand)
}

func TestFighter8MotionPartialWordsAndClone(t *testing.T) {
	recovered, clones, flips, counters, decelerations := 0, 0, 0, 0, 0
	walkChuActor(t, "fighter8-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter0MotionCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		b, _, _, _ := decodeFighter8Motion(t, row.Before)
		a, _, _, _ := decodeFighter8Motion(t, row.After)
		if row.Kind == 0x3e && b.Body.Clock.Completed == 1 {
			if a.Body.ActionKind != 2 || a.Body.Pose != 7 || a.Body.EnabledStages != 0x14 || a.Body.ExtraJump != 0 || a.Body.VelocityX != 0 || a.Body.VerticalVelocity != 0 || a.Body.Gravity != row.Before.Parameters.Gravity {
				t.Fatal("3e incorrectly recovered to ground or failed1860c")
			}
			recovered++
		}
		if row.Kind == 0x3f && b.Body.Clock.Completed != 1 {
			if b.Body.Clock.FrameIndex == 0 && b.Body.Clock.Ticks == 0 && b.Children[0].Size == 0 {
				old, n := b.Children[0], a.Children[0]
				if n.Behavior != 0x147ae || n.Team != b.Body.Team+16 || n.TargetBit != b.TargetBit+1 || n.Attributes != b.Body.Attributes+0x60 || n.Pose != 0xa6 || n.X != b.Body.X || n.Y != b.Body.Y || n.VelocityX != old.VelocityX || n.VerticalVelocity != old.VerticalVelocity || n.Power != old.Power {
					t.Fatal("3f clone prefix changed unwritten speed/power or used masked attrs")
				}
				clones++
			}
			if b.Body.Clock.FrameIndex == 1 && b.Body.Clock.Ticks == 0 {
				dx := b.Body.VelocityX
				if b.Body.Facing != 0 {
					dx = -dx
				}
				if a.Body.X != b.Body.X+dx || a.Body.Facing != b.Body.Facing^1 || a.Children != b.Children {
					t.Fatal("3f parent displacement/XOR order differs")
				}
				flips++
			}
		}
		if row.Kind == 0x40 {
			if b.Body.Pose == 0xad {
				counter := uint16(uint32(b.AlternateVelocity)>>16) + 1
				if uint16(a.AlternateVelocity) != uint16(b.AlternateVelocity) || uint16(uint32(a.AlternateVelocity)>>16) != counter {
					t.Fatal("40 counter must change Aux48 highword only")
				}
				if int16(counter) > 128 || b.Controls.Pressed&ButtonC != 0 {
					if a.Body.Pose != 0xae || a.Body.X != b.Body.X || a.Body.VelocityX != b.Body.VelocityX {
						t.Fatal("40 early deceleration transition moved before timeout/C check")
					}
				}
				counters++
			} else if b.AlternateVelocity != a.AlternateVelocity {
				t.Fatal("40 counter changed outside ad pose")
			}
			if b.Body.Pose == 0xae && b.Body.Clock.Completed != 1 {
				vx := b.Body.VelocityX >> 1
				if a.Body.VelocityX != vx || a.Body.X != b.Body.X+vx {
					t.Fatal("ae must halve stored velocity before moving without facing")
				}
				decelerations++
			}
		}
	})
	if recovered == 0 || clones == 0 || flips == 0 || counters == 0 || decelerations == 0 {
		t.Fatal("missing source8 exact-word/clone/recovery evidence", recovered, clones, flips, counters, decelerations)
	}
}

func TestFighter8MotionErrorsAreAtomic(t *testing.T) {
	base := ChuSpecialMotionState{Energy: 223<<16 | 0xabcd, AlternateVelocity: 0x12345678}
	base.Body.ActionKind = 0x15
	got, err := StepFighter8Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported source8 phase changed state")
	}
	base.Body.ActionKind = 0x11
	base.Body.Clock.FrameIndex = 8
	got, err = StepFighter8Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("shared retreat out-of-table diagnostic leaked state")
	}
	base.Body.ActionKind = 0x3e
	base.Body.Clock.FrameIndex = 7
	base.Body.Clock.Ticks = 0
	base.Pool.Cursor = 1
	base.Controls.Held = ButtonUp
	got, err = StepFighter8Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("flight FX diagnostic leaked vertical adjustment/movement")
	}
	base.Body.Clock.FrameIndex = 0
	got, err = StepFighter8Motion(base, JumpParameters{})
	if err != nil || got.Body.Y != base.Body.Y-0x10000 || got.Pool != base.Pool {
		t.Fatal("unused pool was validated before flight phase", err)
	}
}
