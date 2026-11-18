// Command rules builds a source-bound authority bundle without visual pixels.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0ngk0ng/wire-board/games/pal/resources"
)

func main() {
	source := flag.String("source", ".local/pal/original-98", "original 98 data directory")
	out := flag.String("out", ".local/pal/rules", "authority output directory")
	flag.Parse()
	if err := run(*source, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(source, out string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	target, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || len(rel) > 3 && rel[:3] == ".."+string(os.PathSeparator) {
		return fmt.Errorf("output must stay in the current workspace")
	}
	bundle, err := resources.BuildRuleBundle(os.DirFS(source))
	if err != nil {
		return err
	}
	if err = os.MkdirAll(target, 0755); err != nil {
		return err
	}
	hashes := map[string]string{}
	for name, data := range bundle.Files {
		if err = os.WriteFile(filepath.Join(target, name), data, 0644); err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	manifest, _ := json.MarshalIndent(map[string]any{"format": 1, "scope": "authority-only geometry and animation metadata", "sources": bundle.SourceHashes, "files": hashes}, "", "  ")
	if err = os.WriteFile(filepath.Join(target, "manifest.json"), manifest, 0644); err != nil {
		return err
	}
	fmt.Println("wrote authority rule bundle", target)
	return nil
}
