package engine

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// The original natural and isolated fixtures and this test precede
// fighter_input.go. Held/pressed/changed are independently retained channels.
func TestFighterInputAgainstOriginal(t *testing.T) {
	for _, name := range []string{"fighter-input-oracle", "fighter-input-boundaries"} {
		t.Run(name, func(t *testing.T) {
			file, err := os.Open("testdata/" + name + ".json.gz")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			z, err := gzip.NewReader(file)
			if err != nil {
				t.Fatal(err)
			}
			defer z.Close()
			raw, err := io.ReadAll(z)
			if err != nil {
				t.Fatal(err)
			}
			var rows []struct {
				Before, After FighterControls
				Controls      ControllerFrame
			}
			if err = json.Unmarshal(raw, &rows); err != nil {
				t.Fatal(err)
			}
			manifestRaw, err := os.ReadFile("testdata/" + name + "-manifest.json")
			if err != nil {
				t.Fatal(err)
			}
			var manifest struct {
				Pass  bool
				Count int    `json:"complete_calls"`
				SHA   string `json:"fixture_sha256"`
			}
			if err = json.Unmarshal(manifestRaw, &manifest); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(raw)
			if !manifest.Pass || len(rows) == 0 || manifest.Count != len(rows) || manifest.SHA != hex.EncodeToString(digest[:]) {
				t.Fatal("unverified original input fixture")
			}
			for i, row := range rows {
				if got := LatchFighterInput(row.Before, row.Controls); got != row.After {
					t.Fatalf("call %d: got %+v want %+v; input %+v", i, got, row.After, row.Controls)
				}
			}
		})
	}
}
