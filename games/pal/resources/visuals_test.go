package resources_test

// Written before pixel decoding/export. Failure cases: RLE optional header;
// opaque palette index zero mistaken for transparent; clipped/truncated runs;
// runs crossing rows; oversized dimensions; invalid sprite offsets; palette
// night fallback/scaling; atlas frame changes; original map/art mismatches.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestRLETransparencyAndBounds(t *testing.T) {
	data := []byte{2, 0, 0, 0, 2, 0, 2, 0, 0x81, 1, 0, 2, 1, 255}
	b, err := resources.DecodeRLE(data)
	if err != nil {
		t.Fatal(err)
	}
	if b.Width != 2 || b.Height != 2 || !bytes.Equal(b.Pixels, []byte{0, 0, 1, 255}) || !bytes.Equal(b.Alpha, []byte{0, 255, 255, 255}) {
		t.Fatal("palette zero or transparency changed", b)
	}
	plain, err := resources.DecodeRLE(data[4:])
	if err != nil || !bytes.Equal(plain.Pixels, b.Pixels) || !bytes.Equal(plain.Alpha, b.Alpha) {
		t.Fatal("optional RLE header changed pixels", err)
	}
	clipped, err := resources.DecodeRLE([]byte{2, 0, 2, 0, 5, 1, 2, 3, 4, 5})
	if err != nil || !bytes.Equal(clipped.Pixels, []byte{1, 2, 3, 4}) {
		t.Fatal("original clipped literal behavior changed", err)
	}
	for _, bad := range [][]byte{{1}, {255, 255, 255, 255}, {2, 0, 2, 0, 4, 1}, {2, 0, 2, 0, 2, 1}, {2, 0, 2, 0, 0, 0}, {0, 0, 255, 255}} {
		if _, err := resources.DecodeRLE(bad); err == nil {
			t.Fatal("corrupt RLE accepted", bad)
		}
	}
}

func TestOriginal98Pixels(t *testing.T) {
	root := filepath.Join("..", "..", "..", ".local", "pal", "original-98")
	if _, err := os.Stat(root); err != nil {
		t.Skip("fetch original resources first")
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "rle-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Vectors []struct {
			Archive string `json:"archive"`
			Chunk   int    `json:"chunk"`
			Frame   int    `json:"frame"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			SHA256  string `json:"sha256"`
		} `json:"vectors"`
	}
	if err = json.Unmarshal(raw, &reference); err != nil {
		t.Fatal(err)
	}
	if len(reference.Vectors) < 20 {
		t.Fatal("missing original raster reference")
	}
	extraEnemyFrame := false
	for _, v := range reference.Vectors {
		extraEnemyFrame = extraEnemyFrame || v.Archive == "abc.mkf" && v.Chunk == 2 && v.Frame == 2
		input, err := os.ReadFile(filepath.Join(root, v.Archive))
		if err != nil {
			t.Fatal(err)
		}
		a, err := resources.OpenArchive(input)
		if err != nil {
			t.Fatal(err)
		}
		data, err := a.Chunk(v.Chunk)
		if err != nil {
			t.Fatal(err)
		}
		if v.Archive != "gop.mkf" && v.Archive != "data.mkf" && v.Archive != "rgm.mkf" && v.Archive != "ball.mkf" {
			data, err = resources.DecompressYJ2(data, 8*1024*1024)
			if err != nil {
				t.Fatal(err)
			}
		}
		if v.Frame >= 0 {
			frames, err := resources.SpriteAddressableMetrics(data)
			if err != nil {
				t.Fatal(err)
			}
			if v.Frame >= len(frames) {
				t.Fatal("reference frame outside original sprite", v.Archive, v.Chunk, v.Frame)
			}
			data = data[frames[v.Frame].Offset:]
		}
		bitmap, err := resources.DecodeRLE(data)
		if err != nil {
			t.Fatal(v.Archive, v.Chunk, v.Frame, err)
		}
		pixels := make([]byte, len(bitmap.Pixels)*2)
		for i, index := range bitmap.Pixels {
			pixels[i*2] = index
			pixels[i*2+1] = bitmap.Alpha[i]
		}
		sum := sha256.Sum256(pixels)
		if bitmap.Width != v.Width || bitmap.Height != v.Height || hex.EncodeToString(sum[:]) != v.SHA256 {
			t.Fatal("original C raster differs", v.Archive, v.Chunk, v.Frame)
		}
	}
	if !extraEnemyFrame {
		t.Fatal("missing original C reference for enemy frame beyond nominal count")
	}
	out := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "vectors": len(reference.Vectors), "reference": "original SDLPAL RLE blitter with two destination fills; palette-index/alpha byte comparison"}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "pixels.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
