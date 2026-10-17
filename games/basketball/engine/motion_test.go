package engine

// Failure cases declared before implementation: low-byte carry reaching the
// third byte, negative velocities, vertical acceleration applied in the wrong
// order, 16-bit velocity wrap, 24-bit position wrap, and changing other fields.
// The CSV is recorded from the original game before/after its movement block.

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"
)

func TestMotionAgainstOriginalExecution(t *testing.T) {
	f, err := os.Open("testdata/motion.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows)-1 != 2377 {
		t.Fatalf("unexpected oracle length: %d", len(rows)-1)
	}
	value := func(row []string, index int) int32 {
		v, err := strconv.ParseInt(row[index], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		return int32(v)
	}
	signed24 := func(v int32) Fixed { return Fixed(v << 8 >> 8) }
	signed16 := func(v int32) Fixed { return Fixed(int16(v)) }
	for _, r := range rows[1:] {
		input := Body{
			Position: Position{X: signed24(value(r, 3)), Depth: Fixed(value(r, 4)), Height: signed24(value(r, 5))},
			Velocity: Velocity{X: signed16(value(r, 6)), Depth: signed16(value(r, 7)), Height: signed16(value(r, 8))},
			Gravity:  signed16(value(r, 9)),
		}
		want := input
		want.Position = Position{X: signed24(value(r, 10)), Depth: Fixed(value(r, 11)), Height: signed24(value(r, 12))}
		want.Velocity.Height = signed16(value(r, 13))
		if got := IntegrateMotion(input); got != want {
			t.Fatalf("%s frame=%s object=%s\ninput=%+v\ngot=%+v\nwant=%+v", r[0], r[1], r[2], input, got, want)
		}
	}
	t.Log("matched 2377 original three-axis movement executions")
}

func TestMotionArithmeticBoundaries(t *testing.T) {
	for _, c := range []struct {
		name string
		in   Body
		want Body
	}{
		{"propagated carry", Body{Position: Position{X: 64640}, Velocity: Velocity{X: 960}}, Body{Position: Position{X: 65600}, Velocity: Velocity{X: 960}}},
		{"signed motion", Body{Position: Position{X: 256, Depth: 256, Height: 256}, Velocity: Velocity{X: -257, Depth: -257, Height: -257}}, Body{Position: Position{X: -1, Depth: 65535, Height: -1}, Velocity: Velocity{X: -257, Depth: -257, Height: -257}}},
		{"gravity before movement", Body{Velocity: Velocity{Height: 1024}, Gravity: 256}, Body{Position: Position{Height: 768}, Velocity: Velocity{Height: 768}, Gravity: 256}},
		{"velocity wrap", Body{Velocity: Velocity{Height: -32768}, Gravity: 1}, Body{Position: Position{Height: 32767}, Velocity: Velocity{Height: 32767}, Gravity: 1}},
		{"position wrap", Body{Position: Position{X: 0x7fffff, Height: 0x7fffff}, Velocity: Velocity{X: 1, Height: 1}}, Body{Position: Position{X: -0x800000, Height: -0x800000}, Velocity: Velocity{X: 1, Height: 1}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := IntegrateMotion(c.in); got != c.want {
				t.Fatalf("got %+v want %+v", got, c.want)
			}
		})
	}
}
