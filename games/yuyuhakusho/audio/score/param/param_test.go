package param

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"reflect"
	"slices"
	"testing"
)

//go:embed testdata/source.json.gz
var sourceFixture []byte

type fixture struct {
	Patches    []FMPatch `json:"patches"`
	PatchLoads []struct {
		Patch  string            `json:"patch"`
		Level  uint8             `json:"level"`
		Writes []FMRegisterWrite `json:"writes"`
	} `json:"patch_loads"`
	Levels []struct {
		Patch    string          `json:"patch"`
		Level    uint8           `json:"level"`
		Initial  []FMLevelUpdate `json:"initial"`
		Ordinary []FMLevelUpdate `json:"ordinary"`
	} `json:"levels"`
	Pitch []struct {
		Definition PitchEnvelope `json:"definition"`
		Expected   []PitchTick   `json:"expected"`
	} `json:"pitch"`
	Volume []struct {
		Definition VolumeEnvelope `json:"definition"`
		Expected   []VolumeTick   `json:"expected"`
	} `json:"volume"`
	Patterns []PanPattern `json:"patterns"`
	Pan      []struct {
		Definition PanDefinition `json:"definition"`
		Calls      []struct {
			Note bool `json:"note"`
			Off  bool `json:"off"`
		} `json:"calls"`
		Expected []PanTick `json:"expected"`
	} `json:"pan"`
}

func reference(t *testing.T) fixture {
	t.Helper()
	sum := sha256.Sum256(sourceFixture)
	if hex.EncodeToString(sum[:]) != "4147e1607d241dd1510dbabaa484f6f6ea4b73b2230319e6923b3084aa2b8cbb" {
		t.Fatal("source fixture digest differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(sourceFixture))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	var f fixture
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestOriginalPatchRequestsAndAllLevels(t *testing.T) {
	f := reference(t)
	patches := map[string]FMPatch{}
	for _, p := range f.Patches {
		if err := p.Validate(); err != nil {
			t.Fatal(p.ID, err)
		}
		patches[p.ID] = p
	}
	if len(patches) != 100 || len(f.PatchLoads) != 2525 || len(f.Levels) != 25600 {
		t.Fatal(len(patches), len(f.PatchLoads), len(f.Levels))
	}
	for i, c := range f.PatchLoads {
		actual, err := PatchWrites(patches[c.Patch], c.Level)
		if err != nil || !slices.Equal(actual, c.Writes) {
			t.Fatalf("native patch %d: %v %v want %v", i, actual, err, c.Writes)
		}
	}
	for i, c := range f.Levels {
		for _, initial := range []bool{false, true} {
			want := c.Ordinary
			if initial {
				want = c.Initial
			}
			actual, err := FMLevelUpdates(patches[c.Patch], c.Level, initial)
			if err != nil || !slices.Equal(actual, want) {
				t.Fatalf("level %d initial=%v: %v %v want %v", i, initial, actual, err, want)
			}
		}
	}
}

func TestSourceEnvelopeSequences(t *testing.T) {
	f := reference(t)
	pitchCount, volumeCount := 0, 0
	for _, c := range f.Pitch {
		if err := c.Definition.Validate(); err != nil {
			t.Fatal(err)
		}
		state := EnvelopeCursor{}
		for i, want := range c.Expected {
			actual, next, err := c.Definition.Next(state)
			if err != nil || !reflect.DeepEqual(actual, want) {
				t.Fatalf("pitch %d/%d: %+v %v want %+v", c.Definition.ID, i, actual, err, want)
			}
			state = next
			pitchCount++
		}
		if c.Definition.Ended {
			_, next, err := c.Definition.Next(state)
			if err == nil || next != state {
				t.Fatal("terminal pitch cursor accepted or changed")
			}
		}
	}
	for _, c := range f.Volume {
		if err := c.Definition.Validate(); err != nil {
			t.Fatal(err)
		}
		state := EnvelopeCursor{}
		for i, want := range c.Expected {
			actual, next, err := c.Definition.Next(state)
			if err != nil || !reflect.DeepEqual(actual, want) {
				t.Fatalf("volume %d/%d: %+v %v want %+v", c.Definition.ID, i, actual, err, want)
			}
			state = next
			volumeCount++
		}
	}
	if pitchCount != 4270 || volumeCount != 22528 {
		t.Fatal(pitchCount, volumeCount)
	}
}

func TestPanSourceDispatch(t *testing.T) {
	f := reference(t)
	calls := 0
	for _, c := range f.Pan {
		if err := c.Definition.Validate(f.Patterns); err != nil {
			t.Fatal(err)
		}
		state := InitialPanState(c.Definition)
		if len(c.Calls) != len(c.Expected) {
			t.Fatal("pan fixture length")
		}
		for i, call := range c.Calls {
			actual, err := AdvancePan(c.Definition, f.Patterns, state, call.Note, call.Off)
			if err != nil || !reflect.DeepEqual(actual, c.Expected[i]) {
				t.Fatalf("pan %+v tick %d: %+v %v want %+v", c.Definition, i, actual, err, c.Expected[i])
			}
			state = actual.State
			calls++
		}
	}
	if calls != 52200 {
		t.Fatal(calls)
	}
}

func TestArithmeticAndInvalidParameters(t *testing.T) {
	negative := int8(-1)
	tick := PitchTick{Offset: &negative, ScaleAdd: 2}
	word, scale, err := PitchWord(tick, 127, 65530, -2, 9)
	if err != nil || word != 128 || scale != -127 {
		t.Fatal(word, scale, err)
	}
	fixed := uint16(131)
	tick = PitchTick{KeyOff: true, FixedPitchWord: &fixed}
	word, scale, err = PitchWord(tick, 7, 1234, 3, 4)
	if err != nil || word != 131 || scale != 7 {
		t.Fatal(word, scale, err)
	}
	for _, value := range []uint8{0, 1, 14, 15, 16, 127, 240, 255} {
		for _, base := range []uint8{0, 1, 15, 127, 255} {
			actual, write, err := PSGAttenuation(VolumeTick{Level: &value, WritesLevel: true}, base)
			want := uint8(uint16(value) + uint16(base))
			if want > 15 {
				want = 15
			}
			if err != nil || !write || actual != want {
				t.Fatal(value, base, actual, write, err)
			}
		}
	}
	if _, write, err := PSGAttenuation(VolumeTick{}, 5); err != nil || write {
		t.Fatal("no-write tick", write, err)
	}
	f := reference(t)
	p := f.Patches[0]
	p.Algorithm = 8
	if p.Validate() == nil {
		t.Fatal("algorithm accepted")
	}
	p = f.Patches[0]
	p.Operators[0].Carrier = !p.Operators[0].Carrier
	if p.Validate() == nil {
		t.Fatal("carrier mask accepted")
	}
	var decoded FMPatch
	if json.Unmarshal([]byte(`{"operators":[{},{},{},{},{}]}`), &decoded) == nil {
		t.Fatal("fifth operator accepted")
	}
	if (PitchEnvelope{}).Validate() == nil || (VolumeEnvelope{}).Validate() == nil {
		t.Fatal("empty envelope accepted")
	}
	state := EnvelopeCursor{Index: 65535}
	_, next, err := f.Pitch[0].Definition.Next(state)
	if err == nil || next != state {
		t.Fatal("invalid pitch cursor changed")
	}
	_, next, err = f.Volume[0].Definition.Next(state)
	if err == nil || next != state {
		t.Fatal("invalid volume cursor changed")
	}
	bad := PanDefinition{Mode: "unknown"}
	if bad.Validate(f.Patterns) == nil {
		t.Fatal("unknown pan mode")
	}
	old := PanState{Step: 254, Elapsed: 255}
	result, err := AdvancePan(bad, f.Patterns, old, true, false)
	if err == nil || result.State != old {
		t.Fatal("invalid pan partially committed")
	}
	bad = PanDefinition{Mode: PanAdvanceOnNote, Pattern: "absent"}
	if bad.Validate(f.Patterns) == nil {
		t.Fatal("unknown pan pattern")
	}
}
