package resources_test

// Before implementation: truncated bitstreams, oversized output allocation,
// invalid back references, short/long output, missing terminator, and adaptive
// Huffman rescaling changing original map/sprite/background bytes. Real original
// chunks are checked against hashes produced by the read-only SDLPAL C oracle.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func TestYJ2RejectInvalidStreams(t *testing.T) {
	for name, data := range map[string][]byte{
		"short": {0, 0, 0}, "missing_bits": {8, 0, 0, 0}, "oversized_output": {255, 255, 255, 127, 0}, "invalid_reference": {1, 0, 0, 0, 255, 255, 255, 255, 255, 255},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := resources.DecompressYJ2(data, 65536); err == nil {
				t.Fatal("invalid compressed stream accepted")
			}
		})
	}
}

func TestOriginal98Decompression(t *testing.T) {
	root := filepath.Join("..", "..", "..", ".local", "pal", "original-98")
	if _, err := os.Stat(root); err != nil {
		t.Skip("original data not downloaded; run games/pal/assets/fetch.py")
	}
	raw, err := os.ReadFile(filepath.Join("testdata", "yj2-reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Vectors []struct {
			Archive string `json:"archive"`
			Chunk   int    `json:"chunk"`
			Size    int    `json:"size"`
			SHA256  string `json:"sha256"`
		} `json:"vectors"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Vectors) < 15 {
		t.Fatal("missing original reference cases")
	}
	for _, v := range fixture.Vectors {
		file, err := os.ReadFile(filepath.Join(root, v.Archive))
		if err != nil {
			t.Fatal(err)
		}
		a, err := resources.OpenArchive(file)
		if err != nil {
			t.Fatal(err)
		}
		chunk, err := a.Chunk(v.Chunk)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := resources.DecompressYJ2(chunk, 8*1024*1024)
		if err != nil {
			t.Fatal(v.Archive, v.Chunk, err)
		}
		sum := sha256.Sum256(decoded)
		if len(decoded) != v.Size || hex.EncodeToString(sum[:]) != v.SHA256 {
			t.Fatal("original decompression differs", v.Archive, v.Chunk, len(decoded), hex.EncodeToString(sum[:]))
		}
		if len(chunk) > 8 {
			if _, err := resources.DecompressYJ2(chunk[:len(chunk)/2], 8*1024*1024); err == nil {
				t.Fatal("truncated original compressed chunk accepted", v.Archive, v.Chunk)
			}
		}
	}
	out := filepath.Join("..", "..", "..", ".local", "pal", "verification")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"test": t.Name(), "passed": true, "vectors": len(fixture.Vectors), "reference": "read-only SDLPAL YJ2 C routine on pinned original 98 data", "scope": "byte-exact decompression, not gameplay completion"}, "", "  ")
	if err = os.WriteFile(filepath.Join(out, "decompression.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}
