package contra

// Resource integration failures fixed before implementation: treating base-room
// perspective aliases as separate rooms; truncating waterfall row eight; using
// ordinary maps/tiles for base Boss; loading maps into the wrong half-window;
// retaining whole-campaign resources when a single stage was requested; using
// the base attribute page instead of the replacement page for compatibility;
// returning mutable references into shared definitions; accepting corrupt IDs.

import (
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func levelResources(t *testing.T, name string) *LevelResources {
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

func TestAllOriginalLevelMapLoads(t *testing.T) {
	resources := levelResources(t, "levels.json")
	f, err := os.Open("../../../docs/research/nes-contra/evidence/levels-loader-original.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var oracle struct {
		Observations []struct {
			Stage, RequestedIndex int
			Mode                  string
			Loader                struct {
				Offset        int
				Before, After string
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&oracle); err != nil {
		t.Fatal(err)
	}
	if len(oracle.Observations) != 146 {
		t.Fatal("incomplete original loader matrix")
	}
	bytesChecked := 0
	for _, row := range oracle.Observations {
		variant := RegularMap
		if row.Mode == "boss-loader" {
			variant = BaseBossMap
		}
		before, _ := hex.DecodeString(row.Loader.Before)
		want, _ := hex.DecodeString(row.Loader.After)
		if len(before) != 128 || len(want) != 128 {
			t.Fatal("bad source map window")
		}
		window := [128]uint8(before)
		if err := resources.LoadMap(row.Stage-1, variant, row.RequestedIndex, row.Loader.Offset, &window); err != nil {
			t.Fatal(row.Stage, row.RequestedIndex, err)
		}
		if !reflect.DeepEqual(window[:], want) {
			t.Fatalf("stage %d %s map %d differs from real E09C", row.Stage, row.Mode, row.RequestedIndex)
		}
		screen, err := resources.Screen(row.Stage-1, variant, row.RequestedIndex, false)
		if err != nil {
			t.Fatal(err)
		}
		bytesChecked += len(screen.SupertileIDs)
	}
	if bytesChecked != 8256 {
		t.Fatal("wrong original byte coverage", bytesChecked)
	}
}

func TestStageResourcesRemainLocalAndIndependent(t *testing.T) {
	all := levelResources(t, "levels.json")
	counts := []int{14, 24, 10, 32, 22, 14, 16, 12}
	for level := range 8 {
		one := levelResources(t, fmt.Sprintf("stage-%d.json", level+1))
		info, err := one.Stage(level)
		if err != nil {
			t.Fatal(err)
		}
		if info.MapEntries != counts[level] || info.Rows != map[bool]int{false: 7, true: 8}[level == 2] {
			t.Fatal("wrong map kind/count", level, info)
		}
		if _, err = one.Stage((level + 1) % 8); err == nil {
			t.Fatal("single-stage payload includes another stage")
		}
		for index := 0; index < counts[level]; index++ {
			a, err := one.Screen(level, RegularMap, index, false)
			if err != nil {
				t.Fatal(err)
			}
			b, err := all.Screen(level, RegularMap, index, false)
			if err != nil || !reflect.DeepEqual(a, b) {
				t.Fatal("single-stage result differs", level, index, err)
			}
		}
		if info.Boss.Available {
			boss, err := one.Screen(level, BaseBossMap, info.Boss.MapIndex, true)
			if err != nil || len(boss.SupertileIDs) != 56 {
				t.Fatal("base Boss resources missing", level, err)
			}
		}
	}
	first, _ := all.Screen(0, RegularMap, 0, false)
	first.Tiles[0] ^= 255
	again, _ := all.Screen(0, RegularMap, 0, false)
	if first.Tiles[0] == again.Tiles[0] {
		t.Fatal("screen copy aliases source")
	}
	jungle, err := JungleScreen(0)
	if err != nil || !reflect.DeepEqual(jungle, again) {
		t.Fatal("generic first-stage expansion diverged", err)
	}
}

func TestLevelGraphicsAndAppearanceDefinitions(t *testing.T) {
	r := levelResources(t, "levels.json")
	jungle, err := r.Graphics(0, RegularMap)
	if err != nil {
		t.Fatal(err)
	}
	previous, _ := JungleGraphicsTileset()
	if !reflect.DeepEqual(jungle, previous) {
		t.Fatal("first-stage replacement data changed")
	}
	snow, err := r.Graphics(4, RegularMap)
	if err != nil || snow.SourceAttributePage != 0x9e {
		t.Fatal("replacement attribute high page must be 9E, not base 9D", snow.SourceAttributePage, err)
	}
	info, _ := r.Stage(0)
	if info.Alternate.TriggerScreen != 11 || !info.Alternate.HasCHR || info.Alternate.CHRPreset != 13 || info.Alternate.Palette.CollisionLimits != [3]uint8{6, 168, 168} {
		t.Fatal("jungle alternate differs", info.Alternate)
	}
	chr, err := r.CHRPreset(info.CHRPreset)
	if err != nil || chr != [8]uint8{1, 4, 6, 7, 68, 69, 70, 71} {
		t.Fatal("wrong first-stage pattern preset", chr, err)
	}
	palette, err := r.Palette(info.Initial.PaletteIndices)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString("0f1929080f2718080f1630100f1121300f37120f0f36160f0f2026160f20000f")
	if !reflect.DeepEqual(palette[:], want) {
		t.Fatal("first-stage initial palette differs from original", palette)
	}
}

func TestLevelResourceRejectionIsAtomic(t *testing.T) {
	raw, err := os.ReadFile("leveldata/stage-1.json")
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err = json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	object["romSHA256"] = "wrong"
	bad, _ := json.Marshal(object)
	if _, err = DecodeLevelResources(bad); err == nil {
		t.Fatal("wrong source version accepted")
	}
	r := levelResources(t, "stage-1.json")
	var target [128]uint8
	for i := range target {
		target[i] = 199
	}
	before := target
	for _, v := range []struct {
		level, index, offset int
		variant              MapVariant
	}{{0, 99, 0, RegularMap}, {0, 0, 1, RegularMap}, {2, 0, 0, RegularMap}, {0, 0, 0, BaseBossMap}} {
		if err = r.LoadMap(v.level, v.variant, v.index, v.offset, &target); err == nil {
			t.Fatal("invalid map request accepted", v)
		}
		if target != before {
			t.Fatal("failed load changed map window")
		}
	}
}

// New coverage precedes adding color groups required by the recovered cycle
// routines. Initial-header colors alone are not a complete palette resource.
func TestLevelResourcesSnowEnemyPalettes(t *testing.T) {
	r := levelResources(t, "stage-5.json")
	for id, want := range map[uint8][3]uint8{
		0x3f: {32, 0, 23}, 0x41: {7, 0, 23}, 0x5f: {32, 39, 23},
		0x60: {16, 38, 6}, 0x61: {16, 22, 7},
	} {
		var indices [8]uint8
		for i := range indices {
			indices[i] = id
		}
		colors, err := r.Palette(indices)
		if err != nil {
			t.Fatalf("snow enemy palette %02x: %v", id, err)
		}
		for i := range 8 {
			if colors[i*4] != 15 || [3]uint8(colors[i*4+1:i*4+4]) != want {
				t.Fatalf("snow enemy palette %02x: %v", id, colors)
			}
		}
	}
}

func TestLevelResourcesIncludeRuntimePaletteCycles(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/research/nes-contra/evidence/palette-additional-colors.json")
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Colors []struct {
			ID     uint8
			Colors [3]uint8
		}
	}
	if err = json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"levels.json", "stage-1.json", "stage-2.json", "stage-8.json"} {
		r := levelResources(t, file)
		for _, color := range source.Colors {
			var indices [8]uint8
			for i := range indices {
				indices[i] = color.ID
			}
			palette, err := r.Palette(indices)
			if err != nil {
				t.Fatalf("%s cycle color %d: %v", file, color.ID, err)
			}
			for i := range 8 {
				if palette[i*4] != 15 || [3]uint8(palette[i*4+1:i*4+4]) != color.Colors {
					t.Fatal("wrong original cycle color", file, color.ID)
				}
			}
		}
	}
}
