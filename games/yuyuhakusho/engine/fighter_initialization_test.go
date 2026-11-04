package engine

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"
)

func initializationSpecs(t *testing.T) (FighterSpecLibrary, []byte) {
	t.Helper()
	raw, err := os.ReadFile("../data/fighter-specs.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Schema int
		ROMSHA string `json:"rom_sha256"`
		Specs  []FighterSpec
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if f.Schema != 1 || f.ROMSHA != "c39d52b2e2298b3784ada3de70d64b26b67e172114c7d18494faad70fc4d7c06" || len(f.Specs) != 11 {
		t.Fatal("unbound fighter specification file")
	}
	library := make(FighterSpecLibrary, 11)
	for _, s := range f.Specs {
		if s.CharacterID >= 11 {
			t.Fatal("invalid source character")
		}
		if _, ok := library[s.CharacterID]; ok {
			t.Fatal("duplicate fighter source")
		}
		library[s.CharacterID] = s
	}
	return library, raw
}

func encodeFighterInitialization(t *testing.T, s FighterInitializationState, before string) []byte {
	t.Helper()
	b, err := hex.DecodeString(before)
	if err != nil || len(b) != 256 {
		t.Fatal("invalid original initialized actor", err)
	}
	b = encodeJumpBody(s.Body, b)
	b[0x58], b[0x27] = s.CharacterID, s.SkipGraphics
	for _, f := range []struct {
		offset int
		value  uint32
	}{{0x1c, s.GraphicsSet}, {0x20, s.TilesBase}, {0x50, s.AnimationSet}, {0x8c, s.ShapeSet}, {0x90, s.MovementSource}, {0xb8, uint32(s.Energy)}} {
		binary.BigEndian.PutUint32(b[f.offset:], f.value)
	}
	for i, v := range s.GraphicCache {
		binary.BigEndian.PutUint32(b[0x2c+i*4:], v)
	}
	for _, f := range []struct {
		offset int
		value  uint16
	}{{0x28, uint16(s.GraphicOffsetX)}, {0x2a, uint16(s.GraphicOffsetY)}, {0x3c, s.GraphicParameter}, {0x78, uint16(s.Body.Health)}, {0x7a, uint16(s.DisplayedHealth)}, {0x7e, uint16(s.Body.Auxiliary7E)}, {0x80, uint16(s.Body.Auxiliary80)}, {0xbc, uint16(s.DisplayedEnergy)}, {0xc4, s.AttackScale}} {
		binary.BigEndian.PutUint16(b[f.offset:], f.value)
	}
	b[0xc8] = s.Body.TransitionDelay
	return b
}

// Original fixtures, dirty-input matrices and these tests precede the rule.
func TestFighterInitializationAgainstOriginal(t *testing.T) {
	library, specRaw := initializationSpecs(t)
	beforeLibrary, _ := json.Marshal(library)
	for _, name := range []string{"fighter-initialization-oracle", "fighter-initialization-boundaries"} {
		t.Run(name, func(t *testing.T) {
			zipped, err := os.ReadFile("testdata/" + name + ".json.gz")
			if err != nil {
				t.Fatal(err)
			}
			z, err := gzip.NewReader(bytes.NewReader(zipped))
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			raw, err := io.ReadAll(z)
			if err != nil {
				t.Fatal(err)
			}
			mb, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var m struct {
				Pass       bool
				Records    int
				Baseline   bool           `json:"frames_match_original_core"`
				SHA        string         `json:"fixture_sha256"`
				Gzip       string         `json:"gzip_sha256"`
				SpecSHA    string         `json:"spec_sha256"`
				Characters map[string]int `json:"character_counts"`
			}
			if err := json.Unmarshal(mb, &m); err != nil {
				t.Fatal(err)
			}
			h, zh, sh := sha256.Sum256(raw), sha256.Sum256(zipped), sha256.Sum256(specRaw)
			if !m.Pass || !m.Baseline || m.Records == 0 || hex.EncodeToString(h[:]) != m.SHA || hex.EncodeToString(zh[:]) != m.Gzip || hex.EncodeToString(sh[:]) != m.SpecSHA {
				t.Fatal("unverified original initialization evidence")
			}
			var rows []struct {
				Name          string
				Before, After FighterInitializationState
				ActorBefore   string `json:"actor_before"`
				ActorAfter    string `json:"actor_after"`
				Mask          uint16 `json:"attribute_mask"`
			}
			if err := json.Unmarshal(raw, &rows); err != nil || len(rows) != m.Records {
				t.Fatal("missing initializer calls", err)
			}
			seen := map[uint8]bool{}
			for i, row := range rows {
				got, err := InitializeFighter(row.Before, row.Mask, library)
				want, decodeErr := hex.DecodeString(row.ActorAfter)
				if err != nil || decodeErr != nil || got != row.After || !bytes.Equal(encodeFighterInitialization(t, got, row.ActorBefore), want) {
					t.Fatalf("initializer %d %s differs: err=%v got=%+v want=%+v", i, row.Name, err, got, row.After)
				}
				seen[row.Before.CharacterID] = true
			}
			if name == "fighter-initialization-boundaries" && len(seen) != 11 {
				t.Fatal("missing source character initialization")
			}
			t.Logf("complete original initialization calls=%d source characters=%d", len(rows), len(seen))
		})
	}
	afterLibrary, _ := json.Marshal(library)
	if !bytes.Equal(beforeLibrary, afterLibrary) {
		t.Fatal("initializer modified immutable fighter specs")
	}
}

func TestFighterSpecsConsumedMovementSources(t *testing.T) {
	library, raw := initializationSpecs(t)
	var f struct {
		Sources []struct {
			CharacterID uint8  `json:"character_id"`
			Address     uint32 `json:"movement_address"`
			Bytes       string `json:"movement_bytes"`
			SHA         string `json:"movement_sha256"`
			Unassigned  struct {
				Offset, Length int
				Bytes          string
			} `json:"unassigned_parameter_span"`
		}
	}
	if err := json.Unmarshal(raw, &f); err != nil || len(f.Sources) != 11 {
		t.Fatal("missing parameter source spans", err)
	}
	seen := map[uint8]bool{}
	for _, source := range f.Sources {
		b, err := hex.DecodeString(source.Bytes)
		h := sha256.Sum256(b)
		if err != nil || len(b) != 30 || hex.EncodeToString(h[:]) != source.SHA || seen[source.CharacterID] {
			t.Fatal("unbound parameter bytes")
		}
		seen[source.CharacterID] = true
		spec, ok := library[source.CharacterID]
		if !ok || spec.MovementSource != source.Address {
			t.Fatal("wrong movement source identity")
		}
		l := func(i int) int32 { return int32(binary.BigEndian.Uint32(b[i:])) }
		want := FighterMovementSpec{WalkSpeed: l(0), JumpParameters: JumpParameters{VerticalVelocity: l(4), Gravity: l(8), HorizontalVelocity: l(12)}, BurstSpeed: l(24), EnergyDrain: int16(binary.BigEndian.Uint16(b[28:]))}
		if spec.Movement != want || source.Unassigned.Offset != 16 || source.Unassigned.Length != 8 || source.Unassigned.Bytes != hex.EncodeToString(b[16:24]) {
			t.Fatal("consumed parameter decoding or unassigned range differs")
		}
	}
}

func TestFighterInitializationPreservesUnownedState(t *testing.T) {
	library, _ := initializationSpecs(t)
	before := FighterInitializationState{CharacterID: 9, Body: FighterBodyState{GroundActionState: GroundActionState{Clock: AnimationClock{Completed: 127}, Facing: 3, Auxiliary64: 0xa5, Request: 6, ActionFlags: 0x55, Auxiliary77: 0xaa, BaseTeam: 3, Commands: [16]uint8{1, 2, 3, 4}, VerticalVelocity: -12345}, X: -1, Y: 0x12345678, VelocityX: 0x7fffffff, Gravity: -99, Depth: 0xd8, Layer: 255, ExtraJump: 4, TransitionDelay: 7}, Energy: int32(-559038737)}
	got, err := InitializeFighter(before, 0x8000, library)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body.Clock.Completed != before.Body.Clock.Completed || got.Body.Auxiliary64 != before.Body.Auxiliary64 || got.Body.Request != before.Body.Request || got.Body.ActionFlags != before.Body.ActionFlags || got.Body.Auxiliary77 != before.Body.Auxiliary77 || got.Body.Commands != before.Body.Commands || got.Body.Facing != before.Body.Facing || got.Body.X != before.Body.X || got.Body.Y != before.Body.Y || got.Body.VerticalVelocity != before.Body.VerticalVelocity || got.Body.VelocityX != before.Body.VelocityX || got.Body.Gravity != before.Body.Gravity || got.Body.Depth != before.Body.Depth || got.Body.Layer != before.Body.Layer || got.Body.ExtraJump != before.Body.ExtraJump || got.Body.TransitionDelay != before.Body.TransitionDelay || uint16(got.Energy) != uint16(before.Energy) {
		t.Fatal("initialization acted as a generic action/body reset")
	}
}

func TestFighterInitializationErrorsAreAtomic(t *testing.T) {
	library, _ := initializationSpecs(t)
	before := FighterInitializationState{CharacterID: 255, Body: FighterBodyState{GroundActionState: GroundActionState{Request: 99}, Attributes: 0x1234}, Energy: -1}
	got, err := InitializeFighter(before, 0xffff, library)
	if err == nil || got != before {
		t.Fatal("unknown source caused partial initialization")
	}
	before.CharacterID = 9
	got, err = InitializeFighter(before, 0xffff, nil)
	if err == nil || got != before {
		t.Fatal("missing library caused partial initialization")
	}
	copyLibrary := make(FighterSpecLibrary, len(library))
	for id, spec := range library {
		copyLibrary[id] = spec
	}
	wrong := copyLibrary[9]
	wrong.CharacterID = 0
	copyLibrary[9] = wrong
	got, err = InitializeFighter(before, 0xffff, copyLibrary)
	if err == nil || !reflect.DeepEqual(got, before) {
		t.Fatal("mismatched specification caused partial initialization")
	}
}
