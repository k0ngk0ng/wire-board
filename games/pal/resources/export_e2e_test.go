package resources_test

// Before exporter implementation: wrong palette scaling/night fallback; pixel
// index lost in PNG encoding; atlas frame packing changes source dimensions;
// map layers lost in compression; source/manifest size/hash mismatch; writing
// outside output; client assets mixed into authority rule metadata.

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestOriginalVisualExport(t *testing.T) {
	root := filepath.Join("..", "..", "..", ".local", "pal", "original-98")
	if _, err := os.Stat(root); err != nil {
		t.Skip("fetch original resources first")
	}
	out := t.TempDir()
	index, err := resources.ExportVisuals(os.DirFS(root), out)
	if err != nil {
		t.Fatal(err)
	}
	if index.Version != 1 || len(index.Files) < 1000 {
		t.Fatal("missing original visual assets", len(index.Files))
	}
	for name, file := range index.Files {
		data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if int64(len(data)) != file.Size || hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Fatal("asset manifest differs", name)
		}
	}
	for _, group := range []string{"mgo", "abc"} {
		var atlas resources.Atlas
		meta, err := os.ReadFile(filepath.Join(out, "atlases", group, "2.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(meta, &atlas); err != nil {
			t.Fatal(err)
		}
		imageBytes, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(atlas.Image)))
		if err != nil {
			t.Fatal(err)
		}
		image, err := png.Decode(bytes.NewReader(imageBytes))
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, group+".mkf"))
		if err != nil {
			t.Fatal(err)
		}
		a, _ := resources.OpenArchive(data)
		chunk, _ := a.Chunk(2)
		original, err := resources.DecompressYJ2(chunk, 8*1024*1024)
		if err != nil {
			t.Fatal(err)
		}
		frames, err := resources.SpriteAddressableMetrics(original)
		if err != nil {
			t.Fatal(err)
		}
		if len(atlas.Frames) != len(frames) {
			t.Fatal("atlas dropped animation frames")
		}
		if group == "abc" {
			nominal, err := resources.SpriteMetrics(original)
			if err != nil || len(nominal) != 2 || len(frames) != 3 || frames[2].Width != 25 || frames[2].Height != 77 {
				t.Fatal("original enemy extra frame changed", nominal, frames, err)
			}
		}
		for i, frame := range frames {
			bitmap, err := resources.DecodeRLE(original[frame.Offset:])
			if err != nil {
				t.Fatal(err)
			}
			rect := atlas.Frames[i]
			if rect.Width != bitmap.Width || rect.Height != bitmap.Height {
				t.Fatal("atlas changed source dimensions", i)
			}
			for y := 0; y < bitmap.Height; y++ {
				for x := 0; x < bitmap.Width; x++ {
					r, _, _, alpha := image.At(rect.X+x, rect.Y+y).RGBA()
					p := y*bitmap.Width + x
					if byte(alpha>>8) != bitmap.Alpha[p] || bitmap.Alpha[p] != 0 && byte(r>>8) != bitmap.Pixels[p] {
						t.Fatal("atlas changed original palette indices", i, x, y)
					}
				}
			}
		}
	}
	maps, err := os.ReadFile(filepath.Join(out, "maps", "12.bin.gz"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(maps))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "map.mkf"))
	if err != nil {
		t.Fatal(err)
	}
	archive, _ := resources.OpenArchive(raw)
	source, _ := archive.Chunk(12)
	wanted, err := resources.DecompressYJ2(source, 65536)
	if err != nil || !bytes.Equal(decoded, wanted) {
		t.Fatal("original visual map changed", err)
	}
	var palettes [][2][256][3]uint8
	colors, err := os.ReadFile(filepath.Join(out, "palettes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(colors, &palettes); err != nil {
		t.Fatal(err)
	}
	pat, err := os.ReadFile(filepath.Join(root, "pat.mkf"))
	if err != nil {
		t.Fatal(err)
	}
	pa, _ := resources.OpenArchive(pat)
	for i := 0; i < pa.Count(); i++ {
		p, _ := pa.Chunk(i)
		for night := 0; night < 2; night++ {
			offset := 0
			if len(p) > 768 {
				offset = night * 768
			}
			for c := 0; c < 256; c++ {
				for k := 0; k < 3; k++ {
					if palettes[i][night][c][k] != p[offset+c*3+k]<<2 {
						t.Fatal("original palette changed", i, night, c, k)
					}
				}
			}
		}
	}
	reportDir := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	if err = os.MkdirAll(reportDir, 0755); err != nil {
		t.Fatal(err)
	}
	indexBytes, err := os.ReadFile(filepath.Join(out, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	indexHash := sha256.Sum256(indexBytes)
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "assets": len(index.Files), "output": out, "indexSHA256": hex.EncodeToString(indexHash[:]), "scope": "isolated temporary original map, palette and indexed PNG export; hero and enemy atlas palette/alpha pixels, including original enemy extra frame beyond nominal count; no gameplay completion claim", "enemyExtraFrame": map[string]int{"atlas": 2, "nominalFrames": 2, "addressableFrames": 3, "extraWidth": 25, "extraHeight": 77}}, "", "  ")
	if err = os.WriteFile(filepath.Join(reportDir, "visual-export.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
