package syncwire

// Specified before implementation: exact input edges/flags, bounded runs,
// consecutive counters, immutable sources, stable idle compression, and
// cancellation-only transport latches excluded from simulation checksums.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	"reflect"
	"strings"
	"testing"
)

func TestInputWindowRoundTrip(t *testing.T) {
	inputs := make([]engine.RecordedVideoInput, 120)
	for i := range inputs {
		inputs[i] = engine.RecordedVideoInput{Frame: uint64(i + 100), Held: [4]engine.Buttons{255, 128, 64, 1}, PriorHeld: [4]engine.Buttons{1, 2, 3, 4}, PendingPressed: [4]engine.Buttons{16, 32, 64, 128}, Automated: [4]bool{true, false, true, false}}
	}
	inputs[17].PendingPressed[0] = 0
	inputs[40].Held[1] = 0
	inputs[70].Automated[2] = false
	before, _ := json.Marshal(inputs)
	packed, e := Pack(inputs)
	if e != nil {
		t.Fatal(e)
	}
	wire, e := json.Marshal(packed)
	if e != nil {
		t.Fatal(e)
	}
	var restored InputWindow
	if e = json.Unmarshal(wire, &restored); e != nil {
		t.Fatal(e)
	}
	got, e := Unpack(restored)
	if e != nil || !reflect.DeepEqual(got, inputs) {
		t.Fatal("roundtrip changed original inputs", e)
	}
	after, _ := json.Marshal(inputs)
	if string(before) != string(after) {
		t.Fatal("mutated input source")
	}
	if len(wire) > 500 {
		t.Fatalf("idle runs still verbose: %d", len(wire))
	}
}
func TestInputWindowRejectsInvalid(t *testing.T) {
	for _, window := range []InputWindow{
		{From: 1, Runs: [][]uint64{{0, 0, 0, 0, 0}}},
		{From: 1, Runs: [][]uint64{{121, 0, 0, 0, 0}}},
		{From: 1, Runs: [][]uint64{{1, 1 << 32, 0, 0, 0}}},
		{From: 1, Runs: [][]uint64{{1, 0, 0, 0, 16}}},
		{From: 0, Runs: [][]uint64{{1, 0, 0, 0, 0}}},
		{From: 1, Runs: [][]uint64{{1, 0, 0}}},
		{From: 1<<53 - 1, Runs: [][]uint64{{2, 0, 0, 0, 0}}},
	} {
		if _, e := Unpack(window); e == nil {
			t.Fatal("accepted invalid run", window)
		}
	}
	if _, e := Pack([]engine.RecordedVideoInput{{Frame: 1}, {Frame: 3}}); e == nil {
		t.Fatal("accepted input gap")
	}
	p, e := Pack(nil)
	if e != nil {
		t.Fatal(e)
	}
	got, e := Unpack(p)
	if e != nil || len(got) != 0 {
		t.Fatal("empty window", e)
	}
}
func TestRuntimeDigestIgnoresOnlyPhysicalLatches(t *testing.T) {
	s, e := engine.NewRuntime(engine.DefaultRuntimeConfig())
	if e != nil {
		t.Fatal(e)
	}
	hash, e := RuntimeDigest(s)
	if e != nil || len(hash) != 64 {
		t.Fatal("invalid digest", e)
	}
	s.Held[0] = engine.ButtonRight
	s.PendingPressed[0] = engine.ButtonB
	same, e := RuntimeDigest(s)
	if e != nil || same != hash {
		t.Fatal("physical cancellation requires a checkpoint", e)
	}
	step, e := engine.StepVideo(s, [4]engine.Buttons{})
	if e != nil {
		t.Fatal(e)
	}
	different, e := RuntimeDigest(step.State)
	if e != nil || different == hash || strings.Trim(different, "0") == "" {
		t.Fatal("simulation changes missing from digest", e)
	}
	if s.Held[0] != engine.ButtonRight || s.PendingPressed[0] != engine.ButtonB {
		t.Fatal("digest modified caller")
	}
}

func TestPhysicalLatchesRoundTripWithoutTouchingWorld(t *testing.T) {
	s, e := engine.NewRuntime(engine.DefaultRuntimeConfig())
	if e != nil {
		t.Fatal(e)
	}
	original := s
	s.Held = [4]engine.Buttons{255, 128, 64, 1}
	s.PendingPressed = [4]engine.Buttons{32, 16, 8, 4}
	packed := PackLatches(s)
	restored := ApplyLatches(original, packed)
	if !reflect.DeepEqual(restored, s) {
		t.Fatal("latches changed simulation fields or lost controller bits")
	}
}
