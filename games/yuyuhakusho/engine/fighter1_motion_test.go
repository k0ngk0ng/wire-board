package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func decodeFighter1Motion(t *testing.T, raw fighter0MotionSnapshot) (Fighter1MotionState, []byte, []byte, []byte) {
	t.Helper()
	base, a, c, p := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if a[0x58] != 1 {
		t.Fatal("motion fixture is not actual source1")
	}
	state := Fighter1MotionState{ChuSpecialMotionState: base}
	for i := range state.EffectExtras {
		b := p[i*128:]
		state.EffectExtras[i] = ReactionEffectExtra{Owner: binary.BigEndian.Uint16(b[0x48:]), OffsetY: int16(binary.BigEndian.Uint16(b[0x56:])), Phase: b[0x5a]}
	}
	return state, a, c, p
}

func compareFighter1Motion(t *testing.T, state Fighter1MotionState, a, c, p []byte, want fighter0MotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	expected, wa, wc, wp := decodeFighter1Motion(t, want)
	a, c, p = encodeChuSpecialMotion(state.ChuSpecialMotionState, a, c, p)
	for i, x := range state.EffectExtras {
		b := p[i*128:]
		binary.BigEndian.PutUint16(b[0x48:], x.Owner)
		binary.BigEndian.PutUint16(b[0x56:], uint16(x.OffsetY))
		b[0x5a] = x.Phase
	}
	if state != expected || !bytes.Equal(a, wa) || !bytes.Equal(c, wc) || !bytes.Equal(p, wp) {
		diff := func(a, b []byte) []int {
			var out []int
			for i := range a {
				if a[i] != b[i] {
					out = append(out, i)
				}
			}
			return out
		}
		t.Fatalf("source1 motion differs actor=%x children=%x pool=%x body=%+v/%+v power=%x/%x", diff(a, wa), diff(c, wc), diff(p, wp), state.Body, expected.Body, state.Power, expected.Power)
	}
	return a, c, p
}

// Complete CPU evidence and failure ledger precede fighter1_motion.go.
func TestFighter1MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen := map[uint8]int{}
		external := 0
		count, wantExternal := walkChuActor(t, "fighter1-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter0MotionCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, a, c, p := decodeFighter1Motion(t, row.Before)
			if row.Kind != state.Body.ActionKind || row.SourceRegister != "0xffff8400" {
				t.Fatal("source1 original kind/register differs")
			}
			got, err := StepFighter1Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareFighter1Motion(t, got, a, c, p, row.After)
			if row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed || row.Before.PendingSound != row.After.PendingSound {
				t.Fatal("source1 pre-tail RNG/request changed")
			}
			if len(row.Clears) != 0 {
				external++
				for _, cl := range row.Clears {
					if (cl.PC != "0x37c56" && cl.PC != "0x37cac") || cl.Address < "0xf80a" || cl.Address > "0xf80d" {
						t.Fatal("unidentified independent queue mutation")
					}
				}
			} else if row.Before.SoundQueue != row.After.SoundQueue {
				t.Fatal("unexplained source1 queue change")
			}
			seen[row.Kind]++
		})
		if external != wantExternal {
			t.Fatal("missing actual queue evidence")
		}
		if suffix == "boundaries" {
			for _, kind := range []uint8{0x0f, 0x11, 0x1a, 0x1b, 0x1c, 0x1d} {
				if seen[kind] == 0 {
					t.Fatal("missing original source1 phase", kind)
				}
			}
		}
		t.Logf("source1 complete original motion %s=%d kinds=%v queueRows=%d", suffix, count, seen, external)
	}
}

func TestFighter1MotionRealTimingStreams(t *testing.T) {
	steps := 0
	count, external := walkChuActor(t, "fighter1-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Kind       uint8
			Name, Pool string
			Initial    fighter0MotionSnapshot
			Steps      []fighter0MotionCase
			Terminal   uint8
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("incomplete source1 timing stream", err)
		}
		state, a, c, p := decodeFighter1Motion(t, stream.Initial)
		for index, step := range stream.Steps {
			if !step.AdvanceClock || len(step.External) != 0 {
				t.Fatal("undeclared input or missing actual19c96")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			a, c, p = compareFighter1Motion(t, state, a, c, p, step.Before)
			var err error
			state, err = StepFighter1Motion(state, stream.Initial.Parameters)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			a, c, p = compareFighter1Motion(t, state, a, c, p, step.After)
			key := AnimationFrameKey{AnimationSet: stream.Initial.AnimationSet, ShapeSet: stream.Initial.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("independent source1 timing selection differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing original source1 timing record", key)
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			a, c, p = compareFighter1Motion(t, state, a, c, p, step.AfterRecord)
			if len(step.Clears) != 0 || step.RNGCalls != 0 {
				t.Fatal("unexpected private stream external mutation")
			}
			steps++
		}
		if state.Body.ActionKind != 0 {
			t.Fatal("source1 stream did not reach standing", stream.Name, state.Body.ActionKind)
		}
	})
	if count != 22 || steps < 1500 || external != 0 {
		t.Fatal("missing source1 actual-record continuous flow", count, steps, external)
	}
	t.Logf("source1 real-record streams=%d complete phases=%d", count, steps)
}

func TestFighter1MotionPrefixesPowerAndEffects(t *testing.T) {
	beam, shot, burst, busy, four, rush := 0, 0, 0, 0, 0, 0
	walkChuActor(t, "fighter1-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter0MotionCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		b, _, _, bp := decodeFighter1Motion(t, row.Before)
		a, _, _, ap := decodeFighter1Motion(t, row.After)
		if (row.Kind == 0x1a || row.Kind == 0x1b) && b.Body.Clock.Completed != 1 && b.Body.Clock.FrameIndex == 2 && b.Body.Clock.Ticks == 0 && b.Children[0].Size == 0 {
			old, n := b.Children[0], a.Children[0]
			if row.Kind == 0x1a {
				if n.Behavior != 0x12dd4 || n.Pose != 9 || n.Attributes&0x6000 != 0x6000 || n.VelocityX != old.VelocityX || n.VerticalVelocity != old.VerticalVelocity || n.Power != old.Power {
					t.Fatal("1a wrote wrong prefix or old velocity/Power")
				}
				beam++
			} else {
				vx := b.Body.VelocityX
				if b.Body.Facing != 0 {
					vx = -vx
				}
				if n.Behavior != 0x12ee8 || n.Attributes != b.Body.Attributes+0x60 || n.VelocityX != vx || n.VerticalVelocity != old.VerticalVelocity || n.Power != old.Power || n.Pose != old.Pose {
					t.Fatal("1b full speed/prefix/unwritten fields differ")
				}
				shot++
			}
		}
		if row.Kind == 0x1c && b.Body.Clock.Completed != 1 && b.Body.Clock.Ticks == 1 {
			idx := b.Body.Clock.FrameIndex
			emit := idx == 0 || idx == 1 || idx == 4 || idx == 8 || idx == 10 || idx == 11
			if emit {
				cursor := uint16(b.Power) + 0x100
				if cursor == 0x300 {
					cursor = 0
				}
				if cursor > 0x200 || cursor&255 != 0 || uint16(a.Power) != cursor {
					t.Fatal("1c Power cursor update/word wrap differs")
				}
				child := cursor / 0x100
				if idx == 0 {
					if len(row.Allocations) != 4 {
						t.Fatal("frame0 did not actually call fourFX")
					}
					for order, slot := range row.Allocations {
						if slot < 0 {
							continue
						}
						want := bp[slot*128+0x5a]
						if order == 0 || order == 3 {
							want = 8
						}
						if ap[slot*128+0x5a] != want {
							t.Fatal("fourFX did not preserve middle duration")
						}
					}
					four++
				}
				if b.Children[child].Size != 0 {
					if b.Children != a.Children {
						t.Fatal("busy1300c modified child")
					}
					busy++
				} else {
					old, n := b.Children[child], a.Children[child]
					if n.Behavior != 0x1300c || n.X != old.X || n.VelocityX != b.Body.X || n.VerticalVelocity != -0x28000 || n.Pose != old.Pose || n.Power != old.Power || n.Attributes != b.Body.Attributes+0x40+(cursor>>4) {
						t.Fatal("1c changed old X/Pose/Power or repaired original VX=parentX")
					}
					burst++
				}
			}
		}
		if row.Kind == 0x1d {
			if a.Body.VelocityX != b.Body.VelocityX {
				t.Fatal("segmented displacement overwrote storedVX")
			}
			rush++
		}
	})
	if beam == 0 || shot == 0 || burst == 0 || busy == 0 || four == 0 || rush == 0 {
		t.Fatal("missing exact source1 prefix/normal busy/FX cases", beam, shot, burst, busy, four, rush)
	}
}

func TestFighter1MotionErrorsAreAtomic(t *testing.T) {
	base := Fighter1MotionState{ChuSpecialMotionState: ChuSpecialMotionState{Energy: 223<<16 | 0xabcd, AlternateVelocity: 0x12345678}}
	base.Body.ActionKind = 0x15
	got, err := StepFighter1Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("unsupported source1 phase changed state")
	}
	base.Body.ActionKind = 0x1c
	base.Body.Clock.FrameIndex = 0
	base.Body.Clock.Ticks = 1
	base.Power = 0x300
	got, err = StepFighter1Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("late invalid Power cursor leaked fourFX/extras/word update")
	}
	base.Body.Clock.Ticks = 0
	got, err = StepFighter1Motion(base, JumpParameters{})
	if err != nil || got != base {
		t.Fatal("unused Power cursor consulted before ticks gate", err)
	}
	base.Body.Clock.Ticks = 1
	base.Power = 0
	base.Pool.Cursor = 1
	got, err = StepFighter1Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("fourFX allocator diagnostic leaked changes")
	}
	base.Body.ActionKind = 0x1d
	base.Body.Clock.FrameIndex = 18
	base.Body.Clock.Ticks = 0
	got, err = StepFighter1Motion(base, JumpParameters{})
	if err == nil || got != base {
		t.Fatal("segmented movement diagnostic leaked position/frame")
	}
}
