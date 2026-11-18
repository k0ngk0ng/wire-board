package resources_test

// Before implementation: malformed frame tables/RLE headers; the known PAL
// wrapped frame offset; losing collision/height bits; changing animation frame
// counts or sprite dimensions; retaining visual pixels in authority data;
// replacing non-visual DATA tables; invalid or oversized compressed metadata.
// Extra-address failures: a valid frame after the nominal count is omitted;
// retaining it changes the nominal count; a sentinel becomes a visible frame;
// an alias/wrapped offset or truncated extra RLE is read out of bounds.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestSpriteMetricsBounds(t *testing.T) {
	// Two PAL frames; first word is both frame-zero offset and frame count+1.
	data := []byte{3, 0, 5, 0, 9, 0, 4, 0, 2, 0, 2, 0, 0, 0, 8, 0, 3, 0}
	frames, err := resources.SpriteMetrics(data)
	if err != nil || len(frames) != 2 || frames[0].Width != 4 || frames[0].Height != 2 || frames[1].Width != 8 || frames[1].Height != 3 {
		t.Fatal("original sprite metrics differ", frames, err)
	}
	for _, bad := range [][]byte{{1}, {0, 0}, {200, 0, 0, 0}, {2, 0, 255, 255}} {
		if _, err := resources.SpriteMetrics(bad); err == nil {
			t.Fatal("invalid frame table accepted", bad)
		}
	}
}

func TestSpriteAddressableMetricsRetainsExtraFrame(t *testing.T) {
	data := []byte{3, 0, 6, 0, 9, 0, 1, 0, 1, 0, 1, 77, 1, 0, 1, 0, 1, 78, 1, 0, 1, 0, 1, 79}
	nominal, err := resources.SpriteMetrics(data)
	if err != nil || len(nominal) != 2 {
		t.Fatal("nominal frame count changed", nominal, err)
	}
	frames, err := resources.SpriteAddressableMetrics(data)
	if err != nil || len(frames) != 3 || frames[2].Offset != 18 || frames[2].Width != 1 || frames[2].Height != 1 {
		t.Fatal("valid original extra frame lost", frames, err)
	}
	alias := bytes.Clone(data)
	alias[4] = 3
	frames, err = resources.SpriteAddressableMetrics(alias)
	if err != nil || len(frames) != 3 || frames[2].Offset != 6 {
		t.Fatal("original extra frame alias lost", frames, err)
	}
	sentinel := bytes.Clone(data)
	sentinel[4] = 12
	frames, err = resources.SpriteAddressableMetrics(sentinel)
	if err != nil || len(frames) != 2 {
		t.Fatal("end sentinel became a frame", frames, err)
	}
	frames, err = resources.SpriteAddressableMetrics(data[:23])
	if err != nil || len(frames) != 2 {
		t.Fatal("truncated extra RLE became a frame", frames, err)
	}
}

func TestOriginal98RuleBundle(t *testing.T) {
	root := filepath.Join("..", "..", "..", ".local", "pal", "original-98")
	if _, err := os.Stat(root); err != nil {
		t.Skip("fetch the pinned original 98 resources first")
	}
	bundle, err := resources.BuildRuleBundle(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(bundle.Files) != 13 || len(bundle.SourceHashes) < 13 {
		t.Fatal("missing headless rule resources", len(bundle.Files), len(bundle.SourceHashes))
	}
	read := func(name string) *resources.Archive {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		a, err := resources.OpenArchive(data)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	originalMaps := read("map.mkf")
	maps, err := resources.OpenArchive(bundle.Files["map.mkf"])
	if err != nil {
		t.Fatal(err)
	}
	if maps.Count() != originalMaps.Count() {
		t.Fatal("original map count changed")
	}
	mapCount := 0
	for i := 1; i < maps.Count(); i++ {
		input, _ := originalMaps.Chunk(i)
		if len(input) == 0 {
			continue
		}
		original, err := resources.DecompressYJ2(input, 65536)
		if err != nil {
			t.Fatal(i, err)
		}
		chunk, _ := maps.Chunk(i)
		rules, err := resources.DecodeRuleChunk(chunk, 65536)
		if err != nil {
			t.Fatal(i, err)
		}
		if len(rules) != 65536 || len(original) != len(rules) {
			t.Fatal("map dimensions changed", i)
		}
		for j := 0; j < len(original); j += 4 {
			if binary.LittleEndian.Uint32(rules[j:]) != binary.LittleEndian.Uint32(original[j:])&0x0f002f00 {
				t.Fatal("collision or terrain height changed", i, j)
			}
		}
		mapCount++
	}
	spriteCount := 0
	for _, name := range []string{"mgo.mkf", "abc.mkf", "f.mkf", "fire.mkf"} {
		a := read(name)
		b, err := resources.OpenArchive(bundle.Files[name])
		if err != nil {
			t.Fatal(err)
		}
		if a.Count() != b.Count() {
			t.Fatal("sprite resource count changed", name)
		}
		for i := 0; i < a.Count(); i++ {
			input, _ := a.Chunk(i)
			if len(input) == 0 {
				continue
			}
			raw, err := resources.DecompressYJ2(input, 8*1024*1024)
			if err != nil {
				t.Fatal(name, i, err)
			}
			nominal, err := resources.SpriteMetrics(raw)
			if err != nil {
				t.Fatal(name, i, err)
			}
			wanted, err := resources.SpriteAddressableMetrics(raw)
			if err != nil {
				t.Fatal(name, i, err)
			}
			chunk, _ := b.Chunk(i)
			metadata, err := resources.DecodeRuleChunk(chunk, 8*1024*1024)
			if err != nil {
				t.Fatal(name, i, err)
			}
			gotNominal, err := resources.SpriteMetrics(metadata)
			if err != nil || len(gotNominal) != len(nominal) {
				t.Fatal("original nominal sprite count changed", name, i, err)
			}
			got, err := resources.SpriteAddressableMetrics(metadata)
			if err != nil || len(got) != len(wanted) {
				t.Fatal("headless frame count differs", name, i, err)
			}
			for j := range wanted {
				if wanted[j].Width != got[j].Width || wanted[j].Height != got[j].Height {
					t.Fatal("original sprite dimensions changed", name, i, j, wanted[j], got[j])
				}
			}
			spriteCount++
		}
	}
	originalData := read("data.mkf")
	ruleData, err := resources.OpenArchive(bundle.Files["data.mkf"])
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 11, 13, 14} {
		a, _ := originalData.Chunk(i)
		b, _ := ruleData.Chunk(i)
		if !bytes.Equal(a, b) {
			t.Fatal("non-visual rule table changed", i)
		}
	}
	// Original palette code still runs its timing; authority-only RGB storage
	// contains no client pixels or palette assets.
	pat, err := resources.OpenArchive(bundle.Files["pat.mkf"])
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < pat.Count(); i++ {
		colors, _ := pat.Chunk(i)
		for _, color := range colors {
			if color != 0 {
				t.Fatal("visual palette leaked into authority bundle")
			}
		}
	}
	total := 0
	for _, data := range bundle.Files {
		total += len(data)
	}
	if total > 4*1024*1024 {
		t.Fatal("headless rules include excessive visual data", total)
	}
	bad := []byte{1, 0, 0, 0, 'W', 'B', 'Z', '1', 255, 255, 255, 127}
	if _, err := resources.DecodeRuleChunk(bad, 65536); err == nil {
		t.Fatal("invalid rule compression accepted")
	}
	out := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "maps": mapCount, "spriteGroups": spriteCount, "bundleBytes": total, "sourceHashes": bundle.SourceHashes, "scope": "authority geometry/animation metrics preserve original rules; visual pixels remain separate"}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "rule-bundle.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
