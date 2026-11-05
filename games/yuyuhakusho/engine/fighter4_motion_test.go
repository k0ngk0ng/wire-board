package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type fighter4MotionSnapshot struct {
	fighter0MotionSnapshot
	MovementSource   uint32 `json:"movement_source"`
	DragonBankCursor uint16 `json:"dragon_bank_cursor"`
}

type fighter4MotionCase struct {
	Kind           uint8
	Name, Scenario string
	SourceRegister string `json:"source_register"`
	Before, After  fighter4MotionSnapshot
	RNGCalls       int                            `json:"rng_calls"`
	Clears         []struct{ PC, Address string } `json:"external_queue_clears"`
	Registers      []struct {
		PC string
		D0 uint32
	} `json:"register_observations"`
	AdvanceClock bool                   `json:"advance_clock"`
	External     map[string]uint8       `json:"external_input"`
	RecordAfter  AnimationFrameKey      `json:"record_after"`
	AfterRecord  fighter4MotionSnapshot `json:"after_record"`
}

func decodeFighter4Motion(t *testing.T, raw fighter4MotionSnapshot) (Fighter4MotionState, []byte, []byte, []byte) {
	t.Helper()
	base, a, c, p := decodeChuSpecialMotion(t, raw.chuSpecialMotionSnapshot)
	if a[0x58] != 4 || binary.BigEndian.Uint32(a[0x90:]) != raw.MovementSource {
		t.Fatal("source4 identity or canonical movement source differs")
	}
	state := Fighter4MotionState{ChuSpecialMotionState: base, MovementSource: raw.MovementSource, DragonBankCursor: raw.DragonBankCursor}
	for i := range state.ChildExtras {
		b := c[i*256:]
		state.ChildExtras[i] = Fighter4ChildExtra{Direction: int32(binary.BigEndian.Uint32(b[0x48:])),
			Held: b[0x71], Pressed: b[0x72], ContactFlags: b[0x89], DragonBank: binary.BigEndian.Uint16(b[0xfe:])}
	}
	for i := range state.EffectExtras {
		b := p[i*128:]
		state.EffectExtras[i] = ReactionEffectExtra{Owner: binary.BigEndian.Uint16(b[0x48:]), OffsetY: int16(binary.BigEndian.Uint16(b[0x56:])), Phase: b[0x5a]}
	}
	return state, a, c, p
}

func compareFighter4Motion(t *testing.T, state Fighter4MotionState, a, c, p []byte, want fighter4MotionSnapshot) ([]byte, []byte, []byte) {
	t.Helper()
	expected, wa, wc, wp := decodeFighter4Motion(t, want)
	a, c, p = encodeChuSpecialMotion(state.ChuSpecialMotionState, a, c, p)
	binary.BigEndian.PutUint32(a[0x90:], state.MovementSource)
	for i, extra := range state.ChildExtras {
		b := c[i*256:]
		binary.BigEndian.PutUint32(b[0x48:], uint32(extra.Direction))
		b[0x71], b[0x72], b[0x89] = extra.Held, extra.Pressed, extra.ContactFlags
		binary.BigEndian.PutUint16(b[0xfe:], extra.DragonBank)
	}
	for i, extra := range state.EffectExtras {
		b := p[i*128:]
		binary.BigEndian.PutUint16(b[0x48:], extra.Owner)
		binary.BigEndian.PutUint16(b[0x56:], uint16(extra.OffsetY))
		b[0x5a] = extra.Phase
	}
	if state != expected || !bytes.Equal(a, wa) || !bytes.Equal(c, wc) || !bytes.Equal(p, wp) {
		diff := func(a, b []byte) []int {
			var offsets []int
			for i := range a {
				if a[i] != b[i] {
					offsets = append(offsets, i)
				}
			}
			return offsets
		}
		t.Fatalf("source4 MAIN differs actor=%x children=%x pool=%x source=%x/%x bankCursor=%x/%x body=%+v/%+v extras=%+v/%+v", diff(a, wa), diff(c, wc), diff(p, wp), state.MovementSource, expected.MovementSource, state.DragonBankCursor, expected.DragonBankCursor, state.Body, expected.Body, state.ChildExtras, expected.ChildExtras)
	}
	return a, c, p
}

func readFighter4MotionParameters(t *testing.T) map[uint32]JumpParameters {
	t.Helper()
	raw, err := os.ReadFile("testdata/fighter4-motion-parameters.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("testdata/fighter4-motion-sequences-manifest.json")
	var binding struct {
		SHA string `json:"movement_parameters_sha256"`
	}
	if err != nil || json.Unmarshal(manifest, &binding) != nil {
		t.Fatal("missing immutable movement binding", err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != binding.SHA {
		t.Fatal("movement parameter artifact digest differs")
	}
	var data struct {
		ROM     string `json:"rom_sha256"`
		Records []struct {
			Source uint32
			Raw    string `json:"raw_bytes"`
			Data   JumpParameters
		}
	}
	if err = json.Unmarshal(raw, &data); err != nil || data.ROM != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || len(data.Records) != 2 {
		t.Fatal("missing original source4 movement records", err)
	}
	parameters := make(map[uint32]JumpParameters)
	for _, record := range data.Records {
		b, err := hex.DecodeString(record.Raw)
		if err != nil || len(b) != 16 || (record.Source != 0x12578 && record.Source != 0x15946) {
			t.Fatal("invalid original movement record", err)
		}
		decoded := JumpParameters{VerticalVelocity: int32(binary.BigEndian.Uint32(b[4:])), Gravity: int32(binary.BigEndian.Uint32(b[8:])), HorizontalVelocity: int32(binary.BigEndian.Uint32(b[12:]))}
		if decoded != record.Data {
			t.Fatal("movement values differ from immutable original bytes")
		}
		if _, exists := parameters[record.Source]; exists {
			t.Fatal("duplicate movement source")
		}
		parameters[record.Source] = decoded
	}
	return parameters
}

// The failure ledger and full original CPU matrices precede any MAIN Go code.
func TestFighter4MotionOriginalPhases(t *testing.T) {
	for _, suffix := range []string{"oracle", "boundaries"} {
		seen, external := map[uint8]int{}, 0
		count, wantExternal := walkChuActor(t, "fighter4-motion-"+suffix, func(d *json.Decoder, _ AnimationLibrary) {
			var row fighter4MotionCase
			if err := d.Decode(&row); err != nil {
				t.Fatal(err)
			}
			state, a, c, p := decodeFighter4Motion(t, row.Before)
			if row.SourceRegister != "0xffff8400" || state.Body.ActionKind != row.Kind {
				t.Fatal("source4 MAIN original identity differs")
			}
			got, err := StepFighter4Motion(state, row.Before.Parameters)
			if err != nil {
				t.Fatal(row.Name, err)
			}
			compareFighter4Motion(t, got, a, c, p, row.After)
			if row.RNGCalls != 0 || row.Before.RandomSeed != row.After.RandomSeed || row.Before.PendingSound != row.After.PendingSound {
				t.Fatal("source4 MAIN changed RNG or sound request")
			}
			if len(row.Clears) == 0 {
				if row.Before.SoundQueue != row.After.SoundQueue {
					t.Fatal("unexplained MAIN audio queue mutation")
				}
			} else {
				external++
				for _, clear := range row.Clears {
					if (clear.PC != "0x37c56" && clear.PC != "0x37cac") || clear.Address < "0xf80a" || clear.Address > "0xf80d" {
						t.Fatal("unidentified independent audio consumer")
					}
				}
			}
			seen[row.Kind]++
		})
		if external != wantExternal {
			t.Fatal("missing source4 audio evidence")
		}
		if suffix == "boundaries" {
			for _, action := range []uint8{0x0e, 0x12, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c} {
				if seen[action] == 0 {
					t.Fatal("missing source4 MAIN boundary slot", action)
				}
			}
		}
		t.Logf("source4 MAIN %s=%d kinds=%v", suffix, count, seen)
	}
}

func TestFighter4MotionRealTiming(t *testing.T) {
	parameters := readFighter4MotionParameters(t)
	steps, air, ground, reaction, boosted := 0, 0, 0, 0, 0
	count, external := walkChuActor(t, "fighter4-motion-sequences", func(d *json.Decoder, library AnimationLibrary) {
		var stream struct {
			Name, Pool string
			Initial    fighter4MotionSnapshot
			Steps      []fighter4MotionCase
			Terminal   uint8
		}
		if err := d.Decode(&stream); err != nil || len(stream.Steps) == 0 {
			t.Fatal("incomplete source4 actual-clock stream", err)
		}
		state, a, c, p := decodeFighter4Motion(t, stream.Initial)
		for index, step := range stream.Steps {
			for channel, value := range step.External {
				switch channel {
				case "held":
					state.Controls.Held = state.Controls.Held&0xff00 | Buttons(value)
				case "pressed":
					state.Controls.Pressed = state.Controls.Pressed&0xff00 | Buttons(value)
				default:
					t.Fatal("undeclared source4 external input")
				}
			}
			if !step.AdvanceClock {
				t.Fatal("missing actual19c96 clock step")
			}
			state.Body.Clock = StepAnimationClock(state.Body.Clock)
			a, c, p = compareFighter4Motion(t, state, a, c, p, step.Before)
			current, ok := parameters[state.MovementSource]
			if !ok || current != step.Before.Parameters {
				t.Fatal("current movement source did not select original immutable parameters")
			}
			var err error
			state, err = StepFighter4Motion(state, current)
			if err != nil {
				t.Fatal(stream.Name, index, err)
			}
			a, c, p = compareFighter4Motion(t, state, a, c, p, step.After)
			key := AnimationFrameKey{AnimationSet: stream.Initial.AnimationSet, ShapeSet: stream.Initial.ShapeSet, Pose: state.Body.Pose, Index: state.Body.Clock.FrameIndex}
			if key != step.RecordAfter {
				t.Fatal("source4 independently selected record key differs")
			}
			record, ok := library[key]
			if !ok {
				t.Fatal("missing source4 immutable animation record")
			}
			state.Body.Clock.Duration, state.Body.Clock.Flags = record.Duration, record.Flags
			a, c, p = compareFighter4Motion(t, state, a, c, p, step.AfterRecord)
			if step.RNGCalls != 0 || len(step.Clears) != 0 {
				t.Fatal("unexpected private source4 global mutation")
			}
			steps++
		}
		if state.Body.ActionKind != stream.Terminal {
			t.Fatal("source4 endpoint differs", stream.Name, state.Body.ActionKind, stream.Terminal)
		}
		switch stream.Terminal {
		case 0:
			ground++
		case 2:
			air++
		case 3:
			reaction++
		default:
			t.Fatal("unexpected source4 endpoint")
		}
		if state.MovementSource == 0x15946 {
			boosted++
		}
	})
	if count != 40 || steps != 3224 || ground != 16 || air != 12 || reaction != 12 || boosted != 10 || external != 0 {
		t.Fatal("missing source4 independent complete timing streams", count, steps, ground, air, reaction, boosted, external)
	}
	t.Logf("source4 MAIN actual-clock streams=%d phases=%d ground=%d air=%d action03=%d boost=%d", count, steps, ground, air, reaction, boosted)
}

func TestFighter4MotionRegisterAndCanonicalOwnership(t *testing.T) {
	d0, created, swallowed, restored := 0, 0, 0, 0
	walkChuActor(t, "fighter4-motion-boundaries", func(d *json.Decoder, _ AnimationLibrary) {
		var row fighter4MotionCase
		if err := d.Decode(&row); err != nil {
			t.Fatal(err)
		}
		before, _, _, _ := decodeFighter4Motion(t, row.Before)
		after, _, _, _ := decodeFighter4Motion(t, row.After)
		if len(row.Registers) != 0 {
			if len(row.Registers) != 2 || row.Registers[0].PC != "0x1b282" || row.Registers[1].PC != "0x1b296" {
				t.Fatal("missing original D0 observations")
			}
			displacement := row.Registers[0].D0 | uint32(before.Body.Clock.Ticks&3)
			if before.Body.Clock.Ticks&3 == 0 {
				displacement = 0x149ac
			}
			if row.Registers[1].D0 != displacement {
				t.Fatal("original D0 byte residue/allocator clobber differs")
			}
			delta := int32(displacement)
			if before.Body.Facing != 0 {
				delta = -delta
			}
			if after.Body.X != before.Body.X+delta || after.Body.VelocityX != before.Body.VelocityX {
				t.Fatal("constant-register movement changed stored VX or lost residue")
			}
			d0++
		}
		if row.Kind == 0x29 && before.Children[0].Size == 0 && after.Children[0].Size != 0 {
			next := (before.DragonBankCursor + 0x200) & 0x7ff
			if after.DragonBankCursor != next || after.ChildExtras[0].DragonBank != next || after.ChildExtras[0].Pressed != before.ChildExtras[0].Pressed || after.Children[0].VelocityX != before.Children[0].VelocityX || after.Children[0].VerticalVelocity != before.Children[0].VerticalVelocity || after.Children[0].Pose != before.Children[0].Pose {
				t.Fatal("dragon creation duplicated/lost canonical owners or wrote retained fields")
			}
			created++
		}
		if row.Kind == 0x2a {
			if after.ChildExtras[0].Held != uint8(before.Controls.Held) || after.ChildExtras[0].Pressed != uint8(before.Controls.Pressed) {
				t.Fatal("dragon follow did not copy only actual low input bytes")
			}
			if after.MovementSource == 0x15946 {
				if after.Energy != replaceChuSpecialHighWord(before.Energy, 223) || after.ChildExtras[0].ContactFlags != before.ChildExtras[0].ContactFlags|16 || after.Body.EnabledStages != before.Body.EnabledStages {
					t.Fatal("swallow lost energy fraction/contact bits/stage owner")
				}
				swallowed++
			}
		}
		if row.Kind == 0x2c && before.Body.Clock.Completed == 1 {
			parameters := row.Before.Parameters
			vx := parameters.HorizontalVelocity
			if before.Body.Facing != 1 {
				vx = -vx
			}
			if after.Body.VerticalVelocity != (parameters.VerticalVelocity>>1)+(parameters.VerticalVelocity>>2) || after.Body.VelocityX != vx || after.Body.ExtraJump != before.Body.ExtraJump+1 || after.Body.Pose != 0xaf || after.Body.EnabledStages != 0 {
				t.Fatal("2c current-parameter recovery differs")
			}
			restored++
		}
	})
	if d0 == 0 || created == 0 || swallowed == 0 || restored == 0 {
		t.Fatal("missing original register/ownership edges", d0, created, swallowed, restored)
	}
}

func TestFighter4MotionErrorsAreAtomic(t *testing.T) {
	base := Fighter4MotionState{ChuSpecialMotionState: ChuSpecialMotionState{ObjectID: 0x8400, Energy: 223<<16 | 0xabcd}, MovementSource: 0x12578, DragonBankCursor: 0x7ff}
	base.Body.X, base.Body.Y, base.Body.Ground = 100<<16, 320<<16, 320<<16
	base.Body.ActionKind = 0x24
	base.ChildExtras[0] = Fighter4ChildExtra{Direction: 0x12345678, Held: 0xef, Pressed: 0x71, ContactFlags: 0xa5, DragonBank: 0x600}
	base.Controls.Held, base.Controls.Pressed = 0xab18, 0xcd20
	for _, action := range []uint8{0x24, 0x12, 0x26, 0x27} {
		state := base
		state.Body.ActionKind = action
		state.Body.Clock.FrameIndex = map[uint8]uint8{0x12: 8, 0x26: 7, 0x27: 11}[action]
		got, err := StepFighter4Motion(state, JumpParameters{})
		if err == nil || got != state {
			t.Fatal("unsupported slot/table diagnostic changed source4 state", action)
		}
	}
	for _, action := range []uint8{0x0e, 0x26, 0x29, 0x2a} {
		state := base
		state.Body.ActionKind, state.Body.Clock.FrameIndex, state.Body.Clock.Ticks = action, 0, 0
		state.Pool.Cursor = 1
		state.Children[0].X, state.Children[0].Y = state.Body.X, (320-32)<<16
		got, err := StepFighter4Motion(state, JumpParameters{})
		if err == nil || got != state {
			t.Fatal("FX diagnostic leaked movement/child input/source/cursor", action)
		}
	}
}
