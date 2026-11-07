package contra

// Before resource changes: decimal 81/82 must not be confused with hex 81/82;
// whole-catalog and stage-six loads must both support the original giant's
// palette slot seven; the existing snow additions must remain byte-identical;
// source offsets/digests and real CEE3 generation compatibility must match.
import (
	"encoding/json"
	"os"
	"testing"
)

func machinePaletteResources(t *testing.T, name string) *LevelResources {
	t.Helper()
	raw, err := os.ReadFile("leveldata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeLevelResources(raw)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLevelResourcesEnergyGiantPalettes(t *testing.T) {
	for _, name := range []string{"levels.json", "stage-6.json"} {
		t.Run(name, func(t *testing.T) {
			r := machinePaletteResources(t, name)
			stage, err := r.Stage(5)
			if err != nil {
				t.Fatal(err)
			}
			for id, want := range map[uint8][3]uint8{0x51: {32, 19, 39}, 0x52: {32, 22, 39}} {
				header := stage.Initial
				header.PaletteIndices[7] = id
				var p PaletteOutput
				result, err := p.Load(32, 0, header, r)
				if err != nil {
					t.Fatalf("giant palette decimal %d / hex %02x: %v", id, id, err)
				}
				if p.Desired[28] != 15 || [3]uint8(p.Desired[29:32]) != want || p.RequestedColors != 32 {
					t.Fatalf("giant palette %02x: %+v", id, p)
				}
				wantBias := uint8(0xd6)
				if id == 0x52 {
					wantBias = 0xd9
				}
				if !result.GenerationBiasChanged || result.GenerationBiasValue != wantBias {
					t.Fatal("wrong original palette pointer compatibility", result, wantBias)
				}
			}
		})
	}
}

func TestMachinePaletteExportPreservesSnowColors(t *testing.T) {
	for _, name := range []string{"levels.json", "stage-5.json"} {
		r := machinePaletteResources(t, name)
		for id, want := range map[uint8][3]uint8{0x3f: {32, 0, 23}, 0x41: {7, 0, 23}, 0x5f: {32, 39, 23}, 0x60: {16, 38, 6}, 0x61: {16, 22, 7}} {
			var ids [8]uint8
			for i := range ids {
				ids[i] = id
			}
			colors, err := r.Palette(ids)
			if err != nil {
				t.Fatal(name, id, err)
			}
			for i := range 8 {
				if colors[i*4] != 15 || [3]uint8(colors[i*4+1:i*4+4]) != want {
					t.Fatal("existing snow colors changed", name, id, colors)
				}
			}
		}
	}
}

func TestEnergyGiantPaletteSourceIdentity(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/research/nes-contra/evidence/palette-additional-colors.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		RomSHA256 string
		Colors    []struct {
			ID     int
			Colors [3]uint8
			Source struct {
				CPU, RomOffset int
				SHA256         string
			}
		}
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	if source.RomSHA256 != "6ba53139fa88b8de1ae527c438bda6f1541d1ee7df26d63dec5164a32d166bfe" {
		t.Fatal("wrong original ROM")
	}
	want := map[int]struct {
		cpu, offset int
		colors      [3]uint8
		hash        string
	}{
		81: {0xd1d6, 119270, [3]uint8{32, 19, 39}, "f282d504bcb301799fcdabb1a21a87fb885b8524cda77684884461be7384011e"},
		82: {0xd1d9, 119273, [3]uint8{32, 22, 39}, "1cff724f0e567136a7e030dc70340dc5bb34091aa2ef411fdda7142098036ae1"},
	}
	for _, color := range source.Colors {
		if v, ok := want[color.ID]; ok {
			if color.Colors != v.colors || color.Source.CPU != v.cpu || color.Source.RomOffset != v.offset || color.Source.SHA256 != v.hash {
				t.Fatal("wrong palette source", color)
			}
			delete(want, color.ID)
		}
	}
	if len(want) != 0 {
		t.Fatal("missing source-bound machine palettes", want)
	}
}
