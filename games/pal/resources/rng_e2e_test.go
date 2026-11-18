package resources_test

// Test first. Failure cases: lost unchanged pixels, palette 0/255 corruption,
// wrong skip/literal/repeat counts, row crossings, truncated operands/pixels,
// out-of-surface writes or mutation on rejection, invalid archive offsets,
// different pixels after resuming mid-clip, leaked movie pixels in authority
// data, missing exported frames or changed original C timing/frame counts.
import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestRNGFrameCommandsAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name       string
		data, want []byte
	}{
		{"skip", []byte{2, 3, 0, 4, 0, 0, 6, 0, 255, 0}, []byte{77, 77, 77, 77, 77, 77, 0, 255}},
		{"literal", []byte{8, 1, 2, 3, 4, 5, 6, 19}, []byte{1, 2, 3, 4, 5, 6}},
		{"literal-byte-count", []byte{11, 1, 1, 2, 3, 4, 0}, []byte{1, 2, 3, 4}},
		{"literal-word-count", []byte{12, 0, 0, 0, 255, 0}, []byte{0, 255}},
		{"repeat", []byte{15, 0, 255, 0}, []byte{0, 255, 0, 255, 0, 255, 0, 255}},
		{"repeat-byte-count", []byte{17, 2, 1, 2, 0}, []byte{1, 2, 1, 2, 1, 2}},
		{"repeat-word-count", []byte{18, 1, 0, 3, 4, 0}, []byte{3, 4, 3, 4}},
		{"original-unused-opcodes", []byte{1, 5, 6, 3, 4, 0, 255}, []byte{3, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pixels := bytes.Repeat([]byte{77}, 64000)
			if err := resources.ApplyRNGFrame(tc.data, pixels); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(pixels[:len(tc.want)], tc.want) || !bytes.Equal(pixels[len(tc.want):], bytes.Repeat([]byte{77}, 64000-len(tc.want))) {
				t.Fatal("original RNG command changed pixels")
			}
		})
	}
	for _, bad := range [][]byte{{3}, {4, 0}, {6, 1}, {11, 255, 1, 2}, {12, 0}, {17, 1, 1}, {18, 0, 0, 1}, {4, 255, 255, 6, 1, 2}} {
		pixels := bytes.Repeat([]byte{77}, 64000)
		before := bytes.Clone(pixels)
		if err := resources.ApplyRNGFrame(bad, pixels); err == nil || !bytes.Equal(before, pixels) {
			t.Fatal("invalid RNG changed destination", bad, err)
		}
	}
}

func TestOriginalRNGRuleAndVisualArchives(t *testing.T) {
	root := filepath.Join("..", "..", "..", ".local", "pal")
	input := os.DirFS(filepath.Join(root, "original-98"))
	original, err := os.ReadFile(filepath.Join(root, "original-98", "rng.mkf"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := resources.OpenArchive(original)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := resources.BuildRuleBundle(input)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := resources.OpenArchive(bundle.Files["rng.mkf"])
	if err != nil {
		t.Fatal("authority RNG metadata missing", err)
	}
	if rules.Count() != a.Count() || len(bundle.Files["rng.mkf"]) > 65536 {
		t.Fatal("RNG pixels leaked or clip count changed")
	}
	out := filepath.Join(root, "visual", "pal", "v1")
	index, err := resources.ExportVisuals(input, out)
	if err != nil {
		t.Fatal(err)
	}
	for clip := 0; clip < a.Count(); clip++ {
		chunk, _ := a.Chunk(clip)
		frames, err := resources.DecodeRNGClip(chunk)
		if err != nil {
			t.Fatal(err)
		}
		stripped, _ := rules.Chunk(clip)
		meta, err := resources.OpenArchive(stripped)
		if err != nil {
			t.Fatal(err)
		}
		if meta.Count() != len(frames) {
			t.Fatal("RNG terminal record changed", clip)
		}
		var visual struct {
			Version, Width, Height, Frames int
			Data                           string
		}
		name := filepath.Join("movies", fmt.Sprint(clip)+".json")
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &visual); err != nil {
			t.Fatal(err)
		}
		if visual.Version != 1 || visual.Width != 320 || visual.Height != 200 || visual.Frames != len(frames) {
			t.Fatal("visual RNG metadata changed", clip, visual)
		}
		b, err = os.ReadFile(filepath.Join(out, visual.Data))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		if index.Files[visual.Data].SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatal("RNG manifest differs")
		}
		reader, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		pixels, err := resources.OpenArchive(decoded)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range frames {
			visualFrame, _ := pixels.Chunk(i)
			ruleFrame, _ := meta.Chunk(i)
			if !bytes.Equal(visualFrame, want) {
				t.Fatal("visual RNG command bytes changed", clip, i)
			}
			if len(want) == 0 {
				if len(ruleFrame) != 0 {
					t.Fatal("empty RNG record became a frame", clip, i)
				}
			} else {
				frame, err := resources.DecodeRuleChunk(ruleFrame, 65000)
				if err != nil || !bytes.Equal(frame, []byte{0}) {
					t.Fatal("authority RNG frame contains pixels or invalid timing marker", clip, i, err)
				}
			}
		}
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "clips": a.Count(), "authorityBytes": len(bundle.Files["rng.mkf"]), "scope": "original RNG clip/frame/empty-stop indices retained without pixels; client gzip command archives round-trip every original decoded frame"}, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "verification", "rng-archives.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestOriginalRNGAllFramesAndSavedPixels(t *testing.T) {
	root := filepath.Join("..", "..", "..", ".local", "pal")
	data, err := os.ReadFile(filepath.Join(root, "original-98", "rng.mkf"))
	if err != nil {
		t.Fatal(err)
	}
	archive, err := resources.OpenArchive(data)
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Clips   int `json:"clips"`
		Vectors []struct {
			Clip, Frame, Fill int
			SHA256            string `json:"sha256"`
		} `json:"vectors"`
	}
	b, err := os.ReadFile("testdata/rng-reference.json")
	if err != nil {
		t.Fatal("generate original C RNG reference first", err)
	}
	if err = json.Unmarshal(b, &reference); err != nil {
		t.Fatal(err)
	}
	if reference.Clips != 12 || len(reference.Vectors) != 2928 {
		t.Fatal("incomplete original RNG reference")
	}
	frames := map[int][][]byte{}
	for clip := 0; clip < archive.Count(); clip++ {
		chunk, _ := archive.Chunk(clip)
		frames[clip], err = resources.DecodeRNGClip(chunk)
		if err != nil {
			t.Fatal(clip, err)
		}
	}
	clip, fill := -1, -1
	var pixels, restored []byte
	for _, v := range reference.Vectors {
		if clip != v.Clip || fill != v.Fill {
			clip, fill = v.Clip, v.Fill
			pixels = make([]byte, 64000)
			for i := range pixels {
				if fill == 0 {
					pixels[i] = byte(i % 251)
				} else {
					pixels[i] = 255
				}
			}
			restored = nil
		}
		if err := resources.ApplyRNGFrame(frames[clip][v.Frame], pixels); err != nil {
			t.Fatal(clip, v.Frame, err)
		}
		hash := sha256.Sum256(pixels)
		if hex.EncodeToString(hash[:]) != v.SHA256 {
			t.Fatal("original C RNG pixels differ", clip, v.Frame, fill)
		}
		if restored != nil {
			if err := resources.ApplyRNGFrame(frames[clip][v.Frame], restored); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(pixels, restored) {
				t.Fatal("RNG continuation changed prior pixels", clip, v.Frame)
			}
		}
		if v.Frame == len(frames[clip])/2 {
			restored = bytes.Clone(pixels)
		}
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "clips": 12, "frames": 1464, "emptyTerminalRecords": 12, "pixelComparisons": len(reference.Vectors), "savedMidClipPixels": true, "scope": "all original RNG delta frames compared to pinned original C blitter on patterned and solid destinations; not native playback/browser acceptance"}, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "verification", "rng-pixels.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
