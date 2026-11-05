package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type fighter6MotionSnapshot struct {
	fighter0MotionSnapshot
	StunTicks uint16 `json:"stun_ticks"`
}
type fighter6MainCase struct {
	Kind           uint8
	Name, Scenario string
	SourceRegister string `json:"source_register"`
	Before, After  fighter6MotionSnapshot
	RNGCalls       int                            `json:"rng_calls"`
	Clears         []struct{ PC, Address string } `json:"external_queue_clears"`
	AdvanceClock   bool                           `json:"advance_clock"`
	External       map[string]uint8               `json:"external_input"`
	RecordAfter    AnimationFrameKey              `json:"record_after"`
	AfterRecord    fighter6MotionSnapshot         `json:"after_record"`
}

func decodeFighter6Main(t *testing.T, raw fighter6MotionSnapshot) (Fighter6MotionState, []byte, []byte, []byte) {
	t.Helper()
	base, a, c, p := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if a[0x58] != 6 {
		t.Fatal("MAIN fixture did not use source6")
	}
	return Fighter6MotionState{ChuSpecialMotionState: base, StunTicks: raw.StunTicks}, a, c, p
}

func compareFighter6Main(t *testing.T, state Fighter6MotionState, a, c, p []byte, want fighter6MotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	a, c, p = encodeChuSpecialMotion(state.ChuSpecialMotionState, a, c, p)
	a[0x84], a[0x85] = uint8(state.StunTicks>>8), uint8(state.StunTicks)
	expected := want.chuSpecialMotionSnapshot
	// The shared legacy encoder does not own+84. Preserve this independently
	// encoded word while comparing every other source byte and typed field.
	if state.StunTicks != want.StunTicks {
		t.Fatalf("source6 StunTicks/counter differs %x/%x", state.StunTicks, want.StunTicks)
	}
	return compareChuSpecialMotion(t, state.ChuSpecialMotionState, a, c, p, expected)
}

// All MAIN scenarios and original CPU captures precede fighter6_motion.go.
func TestFighter6MotionMainOriginal(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		var m struct {
			Records  int
			Pass     bool
			Baseline bool `json:"frames_match_original_core"`
		}
		b, err := os.ReadFile("testdata/fighter6-motion-" + suffix + "-manifest.json")
		if err != nil || json.Unmarshal(b, &m) != nil || !m.Pass || !m.Baseline {
			t.Fatal("unverified source6 MAIN manifest", err)
		}
		if m.Records == 0 {
			if suffix != "oracle" {
				t.Fatal("missing MAIN private records")
			}
			t.Log("declared ordinary natural input contains no source6 MAIN motion;no natural coverage inferred")
			continue
		}
		seen := map[uint8]int{}
		external := 0
		count, wantExternal := walkChuActor(t, "fighter6-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter6MainCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, a, c, p := decodeFighter6Main(t, row.Before)
			if row.SourceRegister != "0xffff8400" || state.Body.ActionKind != row.Kind {
				t.Fatal("MAIN original identity differs")
			}
			got, err := StepFighter6Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareFighter6Main(t, got, a, c, p, row.After)
			if row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed || row.Before.PendingSound != row.After.PendingSound {
				t.Fatal("MAIN phase changed RNG/request")
			}
			if len(row.Clears) != 0 {
				external++
				for _, cl := range row.Clears {
					if (cl.PC != "0x37c56" && cl.PC != "0x37cac") || cl.Address < "0xf80a" || cl.Address > "0xf80d" {
						t.Fatal("unidentified audio consumer")
					}
				}
			} else if row.Before.SoundQueue != row.After.SoundQueue {
				t.Fatal("unexplained audio queue difference")
			}
			seen[row.Kind]++
		})
		if external != wantExternal {
			t.Fatal("missing MAIN external queue evidence")
		}
		if suffix == "boundaries" {
			for _, kind := range []uint8{0x33, 0x34, 0x35, 0x36, 0x38} {
				if seen[kind] == 0 {
					t.Fatal("missing MAIN slot", kind)
				}
			}
		}
		t.Logf("MAIN original %s=%d kinds=%v", suffix, count, seen)
	}
}

func TestFighter6MotionMainRealTiming(t *testing.T) {
	steps, air, stand, exit := 0, 0, 0, 0
	count, external := walkChuActor(t, "fighter6-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Kind       uint8
			Name, Pool string
			Initial    fighter6MotionSnapshot
			Steps      []fighter6MainCase
			Terminal   uint8
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("incomplete source6 MAIN timing", err)
		}
		state, a, c, p := decodeFighter6Main(t, stream.Initial)
		for index, step := range stream.Steps {
			for key, value := range step.External {
				switch key {
				case "held":
					state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
				case "pressed":
					state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
				default:
					t.Fatal("undeclared MAIN input")
				}
			}
			if !step.AdvanceClock {
				t.Fatal("missing original19c96")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			a, c, p = compareFighter6Main(t, state, a, c, p, step.Before)
			var err error
			state, err = StepFighter6Motion(state, stream.Initial.Parameters)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			a, c, p = compareFighter6Main(t, state, a, c, p, step.After)
			key := AnimationFrameKey{AnimationSet: stream.Initial.AnimationSet, ShapeSet: stream.Initial.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("MAIN independent timing key differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing actual source6 timing record", key)
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			a, c, p = compareFighter6Main(t, state, a, c, p, step.AfterRecord)
			if step.RNGCalls != 0 || len(step.Clears) != 0 {
				t.Fatal("unexpected private MAIN mutation")
			}
			steps++
		}
		if state.Body.ActionKind != stream.Terminal {
			t.Fatal("MAIN recovery endpoint differs", stream.Name, state.Body.ActionKind, stream.Terminal)
		}
		switch stream.Terminal {
		case 0:
			stand++
		case 2:
			air++
		case 12:
			exit++
		default:
			t.Fatal("unknown MAIN terminal")
		}
	})
	if count != 20 || steps < 2700 || air != 6 || stand != 8 || exit != 6 || external != 0 {
		t.Fatal("missing MAIN singleinitial flows", count, steps, air, stand, exit)
	}
	t.Logf("MAIN real-record streams=%d phases=%d air=%d ground=%d exit18682=%d", count, steps, air, stand, exit)
}

func TestFighter6MotionMainWordsAndRecovery(t *testing.T) {
	rise, land, constant, counter, exit := 0, 0, 0, 0, 0
	walkChuActor(t, "fighter6-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter6MainCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		b, _, _, _ := decodeFighter6Main(t, row.Before)
		a, _, _, _ := decodeFighter6Main(t, row.After)
		if row.Kind == 0x33 {
			next := uint16(uint32(b.AlternateVelocity)>>16) + 1
			if a.Body.Y != b.Body.Y+b.Body.VerticalVelocity || uint16(a.AlternateVelocity) != uint16(b.AlternateVelocity) || uint16(uint32(a.AlternateVelocity)>>16) != next {
				t.Fatal("33 order/highword/lowword differs")
			}
			if next == 16 && (a.Body.ActionKind != 2 || a.Body.Pose != 7 || a.Body.VelocityX != 0 || a.Body.VerticalVelocity != 0 || a.Body.ExtraJump != 0 || a.Body.Gravity != row.Before.Parameters.Gravity) {
				t.Fatal("33 did not invoke unconditional1860c after moving")
			}
			rise++
		}
		if row.Kind == 0x34 {
			y := b.Body.Y + b.Body.VerticalVelocity
			if int16(y>>16) >= int16(b.Body.Ground>>16) {
				if a.Body.Y != b.Body.Ground || a.Body.ActionKind != 0 || a.Body.Pose != 0x0e || a.Body.ExtraJump != 0 {
					t.Fatal("34 landing/ExtraJump order differs")
				}
				land++
			}
		}
		if (row.Kind == 0x35 || row.Kind == 0x36) && b.Body.Clock.Completed != 1 {
			if a.Body.VelocityX != b.Body.VelocityX {
				t.Fatal("MAIN constant movement overwrote storedVX")
			}
			constant++
		}
		if row.Kind == 0x38 {
			if b.Body.Pose == 0xaa {
				next := b.StunTicks + 1
				if a.StunTicks != next {
					t.Fatal("38 duplicated/lost canonical word counter")
				}
				if int16(next) > 256 || b.Controls.Pressed&0x70 != 0 {
					if a.Body.Pose != 0xa9 || a.Body.X != b.Body.X || a.Body.VelocityX != b.Body.VelocityX {
						t.Fatal("38 early stop moved before counter/input transition")
					}
				}
				counter++
			} else if a.StunTicks != b.StunTicks {
				t.Fatal("38 counter changed outside active pose")
			}
			if b.Body.Pose == 0xa9 && b.Body.Clock.Completed == 1 {
				if int16(b.StunTicks) > 128 {
					if a.Body.ActionKind != 12 || a.Body.Pose != 0x2b || a.Body.Auxiliary7E != b.Body.Auxiliary80 || a.Body.Gravity != b.Body.Gravity || a.Body.VerticalVelocity != b.Body.VerticalVelocity || a.Body.ExtraJump != b.Body.ExtraJump {
						t.Fatal("18682 incorrectly used air or new counter storage")
					}
					exit++
				} else if a.Body.ActionKind != 0 {
					t.Fatal("short vortex exit did not enter standing")
				}
			}
		}
	})
	if rise == 0 || land == 0 || constant == 0 || counter == 0 || exit == 0 {
		t.Fatal("missing exact MAIN word/recovery edges", rise, land, constant, counter, exit)
	}
}

func TestFighter6MotionMainErrorsAreAtomic(t *testing.T) {
	base := Fighter6MotionState{ChuSpecialMotionState: ChuSpecialMotionState{Energy: 223<<16 | 0xabcd, AlternateVelocity: 0x12345678}, StunTicks: 129}
	base.Body.ActionKind = 0x15
	got, err := StepFighter6Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported source6 phase changed state")
	}
	base.Body.ActionKind = 0x34
	base.Body.Y = 0
	base.Body.Ground = 0
	base.Body.VerticalVelocity = 0x60000
	base.Body.ExtraJump = 255
	base.Pool.Cursor = 1
	got, err = StepFighter6Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("landing diagnostic leaked clamp/ExtraJump/reset")
	}
	base.Body.ActionKind = 0x35
	base.Body.Clock.FrameIndex = 4
	base.Body.Clock.Ticks = 0
	got, err = StepFighter6Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("short MAIN trail diagnostic leaked movement/frame")
	}
	base.Body.ActionKind = 0x36
	base.Body.Clock.FrameIndex = 8
	got, err = StepFighter6Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("long MAIN trail diagnostic leaked movement/frame")
	}
}
