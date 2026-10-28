package contra

// Written before implementation. Failure scenarios: object/HUD ordering,
// cycling origin and stride, finishing a meta-sprite across capacity, preserving
// hidden slot bytes, linear-mode differences, zero/max/game-over/demo lives,
// snow X borrow changing that tick's Y advance, and shared-pointer writes to
// generation bias even when the byte's value does not change. Whole original
// composition is the oracle; no expected OAM is produced by the implementation.

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type originalCompositionState struct {
	OAM      string
	Rotation uint8
	Snow     [16]SnowParticle
}

func (s originalCompositionState) state(t *testing.T) SpriteCompositionState {
	t.Helper()
	raw, err := hex.DecodeString(s.OAM)
	if err != nil || len(raw) != 256 {
		t.Fatal("invalid original OAM", err, len(raw))
	}
	result := SpriteCompositionState{Rotation: s.Rotation, Snow: s.Snow}
	copy(result.OAM[:], raw)
	return result
}

type originalCompositionSnapshot struct {
	Input          SpriteCompositionInput
	State          originalCompositionState
	GenerationBias uint8
}

type originalCompositionRecord struct {
	Frame, EndFrame  int
	Before, After    originalCompositionSnapshot
	GenerationWrites []struct {
		PCAfter uint16
		Value   uint8
	}
}

type originalCompositionTrace struct {
	Name    string
	Records []originalCompositionRecord
}

func compositionFixture(t *testing.T) (natural, controlled []originalCompositionTrace, initial [16]SnowParticle) {
	t.Helper()
	raw, err := os.ReadFile("testdata/original-presentation-sprites.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("testdata/original-presentation-sprites-source.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct{ FixtureSHA256, RuntimeDataSHA256 string }
	if err = json.Unmarshal(metadata, &source); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != source.FixtureSHA256 {
		t.Fatal("original composition fixture identity")
	}
	data, err := os.ReadFile("presentation_sprites_data.json")
	if err != nil {
		t.Fatal(err)
	}
	digest = sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != source.RuntimeDataSHA256 {
		t.Fatal("composition data identity")
	}
	f, err := os.Open("testdata/original-presentation-sprites.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		Natural, Controlled struct {
			Records int
			Traces  []originalCompositionTrace
		}
		ExpectedInitialSnow [16]SnowParticle
	}
	if err = json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Natural.Records != 4868 || fixture.Controlled.Records != 39 {
		t.Fatal("original coverage changed")
	}
	return fixture.Natural.Traces, fixture.Controlled.Traces, fixture.ExpectedInitialSnow
}

func TestOriginalFullSpriteComposition(t *testing.T) {
	natural, controlled, _ := compositionFixture(t)
	count, continuous, writes, sameValueWrites := 0, 0, 0, 0
	for _, traces := range [][]originalCompositionTrace{natural, controlled} {
		for _, trace := range traces {
			t.Run(trace.Name, func(t *testing.T) {
				state := trace.Records[0].Before.State.state(t)
				for _, r := range trace.Records {
					before := r.Before.State.state(t)
					if strings.Contains(trace.Name, "-boot-") {
						// Boot and original level initialization own explicit clears
						// outside this phase; normal stage traces stay continuous.
						state = before
					} else {
						if state != before {
							t.Fatalf("frame %d lost persistent composition state", r.Frame)
						}
						continuous++
					}
					input := r.Before.Input
					result, err := state.Compose(input)
					if err != nil {
						t.Fatal(err)
					}
					if input != r.Before.Input {
						t.Fatal("input mutated")
					}
					want := r.After.State.state(t)
					if state != want {
						for i := range state.OAM {
							if state.OAM[i] != want.OAM[i] {
								t.Fatalf("frame %d OAM byte %d got %02x want %02x", r.Frame, i, state.OAM[i], want.OAM[i])
							}
						}
						t.Fatalf("frame %d persistent state got rotation %d snow %+v want rotation %d snow %+v", r.Frame, state.Rotation, state.Snow, want.Rotation, want.Snow)
					}
					changed := len(r.GenerationWrites) > 0
					if result.GenerationBiasChanged != changed {
						t.Fatalf("frame %d generation write event got %+v want %v", r.Frame, result, changed)
					}
					value := r.Before.GenerationBias
					if changed {
						value = result.GenerationBiasValue
						writes++
						if value != r.GenerationWrites[len(r.GenerationWrites)-1].Value {
							t.Fatal("wrong final shared pointer low byte")
						}
						if value == r.Before.GenerationBias {
							sameValueWrites++
						}
					}
					if value != r.After.GenerationBias {
						t.Fatalf("frame %d generation bias %02x want %02x", r.Frame, value, r.After.GenerationBias)
					}
					count++
				}
			})
		}
	}
	if count != 4907 || continuous != 2087 || writes == 0 || sameValueWrites == 0 {
		t.Fatalf("coverage %d continuous %d writes %d same-value %d", count, continuous, writes, sameValueWrites)
	}
	t.Logf("verified %d complete original calls, %d consecutive-state calls, %d generation-write calls (%d unchanged final values)", count, continuous, writes, sameValueWrites)
}

func TestSpriteCompositionRestoreAndSnowInitialization(t *testing.T) {
	natural, _, initial := compositionFixture(t)
	s, err := NewSpriteCompositionState()
	if err != nil {
		t.Fatal(err)
	}
	if s.Snow != initial || s.Rotation != 0 {
		t.Fatal("original snow initialization")
	}
	for i := 0; i < 64; i++ {
		if s.OAM[i*4] != 0xf4 {
			t.Fatal("new state exposes uninitialized sprite")
		}
	}
	for _, trace := range natural {
		if !strings.Contains(trace.Name, "-stage-") {
			continue
		}
		s = trace.Records[0].Before.State.state(t)
		for _, record := range trace.Records[:16] {
			before, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			var restored SpriteCompositionState
			if err = json.Unmarshal(before, &restored); err != nil {
				t.Fatal(err)
			}
			a, err := s.Compose(record.Before.Input)
			if err != nil {
				t.Fatal(err)
			}
			b, err := restored.Compose(record.Before.Input)
			if err != nil {
				t.Fatal(err)
			}
			if s != restored || a != b {
				t.Fatal("restored scene changed composition")
			}
		}
	}
	before := s
	if err = s.ResetSnow(); err != nil {
		t.Fatal(err)
	}
	if s.Snow != initial || s.OAM != before.OAM || s.Rotation != before.Rotation {
		t.Fatal("snow reset changed other presentation state")
	}
}

func TestSpriteCompositionRejectsInvalidStateAtomically(t *testing.T) {
	s, err := NewSpriteCompositionState()
	if err != nil {
		t.Fatal(err)
	}
	s.Rotation = 252
	s.OAM[3] = 91
	before := s
	if _, err = s.Compose(SpriteCompositionInput{Level: 8, Flicker: true}); err == nil || s != before {
		t.Fatal("unsupported level partially committed")
	}
	s.Rotation = 3
	before = s
	if _, err = s.Compose(SpriteCompositionInput{Flicker: true}); err == nil || s != before {
		t.Fatal("unaligned cycling state partially committed")
	}
}
