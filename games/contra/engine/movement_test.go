package contra

// Failure scenarios established before implementation:
// - pressed repeats while held, release loses the next edge, or P2 merges in co-op;
// - gravity is clamped, sign-extended incorrectly, or loses 16-bit carry/wrap;
// - position drops its fraction or fails to carry/borrow across the screen byte;
// - beginning a jump clears the preserved subpixel position, uses the wrong
//   indoor velocity/facing, or restarts an already active jump;
// - synthetic expectations replace the captured original execution oracle.

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOriginalMovementTrace(t *testing.T) {
	raw, err := os.ReadFile("testdata/original-movement.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("testdata/original-movement-source.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		FixtureSHA256 string `json:"fixture_sha256"`
		Original      bool   `json:"source_is_original_execution"`
	}
	if err := json.Unmarshal(metadata, &source); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if !source.Original || hex.EncodeToString(digest[:]) != source.FixtureSHA256 {
		t.Fatal("original oracle identity changed")
	}
	f, err := os.Open("testdata/original-movement.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var records []struct {
		Kind, Scenario                         string
		Frame, Actor                           int
		StageType, Flags                       uint8
		Before, After                          PlayerMotion
		Previous, First, Second, Held, Pressed [2]Buttons
	}
	if err := json.NewDecoder(z).Decode(&records); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, r := range records {
		counts[r.Kind]++
		if r.Kind == "input" {
			if r.First != r.Second {
				t.Fatal("fixture includes unstable hardware reads; logical input oracle is invalid")
			}
			input := Controls{Held: r.Previous}
			input.Sample(r.First, r.Flags&4 != 0)
			if input.Held != r.Held || input.Pressed != r.Pressed {
				t.Fatalf("%s frame %d controller got %+v, want held %v pressed %v", r.Scenario, r.Frame, input, r.Held, r.Pressed)
			}
			continue
		}
		actual := r.Before
		switch r.Kind {
		case "beginJump":
			actual.BeginJump(r.StageType&1 != 0)
		case "gravity":
			actual.ApplyGravity()
		case "advanceY":
			actual.AdvanceY()
		default:
			t.Fatalf("unknown oracle kind %q", r.Kind)
		}
		if actual != r.After {
			t.Fatalf("%s frame %d actor %d %s: got %+v want %+v; before %+v", r.Scenario, r.Frame, r.Actor, r.Kind, actual, r.After, r.Before)
		}
	}
	want := map[string]int{"input": 1440, "beginJump": 11, "gravity": 714, "advanceY": 714}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("original observation coverage changed: %v", counts)
	}
	t.Logf("verified original observations: %v", counts)
}

func TestVerticalArithmeticBoundaries(t *testing.T) {
	for _, tc := range []struct {
		position int32
		velocity int16
		want     int32
	}{
		{0x00fffa, 10, 0x010004}, {0, -1, -1}, {-1, 1, 0},
		{0x7fffff, 1, -0x800000}, {-0x800000, -1, 0x7fffff},
		{0x64d0, -1040, 0x60c0}, {0x64d0, -880, 0x6160},
	} {
		m := PlayerMotion{Position: tc.position, Velocity: tc.velocity}
		m.AdvanceY()
		if m.Position != tc.want || m.Velocity != tc.velocity {
			t.Fatalf("position %d velocity %d: %+v want %d", tc.position, tc.velocity, m, tc.want)
		}
	}
	for _, tc := range []struct{ before, after int16 }{{-35, 0}, {-1040, -1005}, {32767, -32734}, {-32768, -32733}} {
		m := PlayerMotion{Velocity: tc.before}
		m.ApplyGravity()
		if m.Velocity != tc.after {
			t.Fatalf("gravity %d got %d want %d", tc.before, m.Velocity, tc.after)
		}
	}
}

func TestJumpRetainsSubpixelAndExistingJump(t *testing.T) {
	for _, indoor := range []bool{false, true} {
		for _, aim := range []uint8{0, 4, 5, 10} {
			before := PlayerMotion{Position: 0x64d0, Velocity: 257, Aim: aim, AnimationTimer: 7, AnimationFrame: 5}
			m := before
			m.BeginJump(indoor)
			velocity := int16(-1040)
			if indoor {
				velocity = -880
			}
			jump := uint8(0x11)
			if aim >= 5 {
				jump = 0x91
			}
			if m.Position != before.Position || m.Velocity != velocity || m.Jump != jump || m.Aim != aim || m.AnimationTimer != 0 || m.AnimationFrame != 0 {
				t.Fatalf("jump indoor %v aim %d: %+v", indoor, aim, m)
			}
			m.AnimationTimer = 3
			m.AnimationFrame = 2
			active := m
			m.BeginJump(!indoor)
			if m != active {
				t.Fatalf("active jump reset: %+v -> %+v", active, m)
			}
		}
	}
}

func TestControlsLogicalBitsAndRelease(t *testing.T) {
	if ButtonA != 0x80 || ButtonB != 0x40 || ButtonSelect != 0x20 || ButtonStart != 0x10 || ButtonUp != 8 || ButtonDown != 4 || ButtonLeft != 2 || ButtonRight != 1 {
		t.Fatal("logical controller bits differ from original")
	}
	var c Controls
	pads := [2]Buttons{ButtonRight | ButtonA, ButtonLeft | ButtonB}
	c.Sample(pads, true)
	if c.Held != pads || c.Pressed != pads {
		t.Fatal(c)
	}
	c.Sample(pads, true)
	if c.Pressed != [2]Buttons{} {
		t.Fatal("held keys repeated edge", c)
	}
	c.Sample([2]Buttons{}, true)
	if c.Held != [2]Buttons{} || c.Pressed != [2]Buttons{} {
		t.Fatal("release retained input", c)
	}
	c.Sample(pads, false)
	if c.Held[0] != pads[0]|pads[1] || c.Held[1] != pads[1] || c.Pressed != c.Held {
		t.Fatal("single-player port merge differs", c)
	}
}
