//go:build !js

package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ReadRulesBuild is a build/test-setup utility. Module callbacks must receive
// the returned ID from their host and must never call this filesystem collector.
// The engine is shared source; platform wrappers live outside its directory.
func ReadRulesBuild(root string) (string, RulesBuildManifest, error) {
	base := filepath.Join(root, "games", "contra", "engine")
	goFiles, err := filepath.Glob(filepath.Join(base, "*.go"))
	if err != nil {
		return "", RulesBuildManifest{}, err
	}
	names := map[string]bool{}
	for _, file := range goFiles {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		names[file] = true
		raw, e := os.ReadFile(file)
		if e != nil {
			return "", RulesBuildManifest{}, e
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(line, "//go:embed ") {
				continue
			}
			for _, pattern := range strings.Fields(strings.TrimPrefix(line, "//go:embed ")) {
				if strings.Contains(pattern, "..") || filepath.IsAbs(pattern) {
					return "", RulesBuildManifest{}, errors.New("embedded rules path escapes engine")
				}
				matches, e := filepath.Glob(filepath.Join(base, pattern))
				if e != nil || len(matches) == 0 {
					return "", RulesBuildManifest{}, errors.New("missing embedded rules input")
				}
				for _, match := range matches {
					info, e := os.Stat(match)
					if e != nil || !info.Mode().IsRegular() {
						return "", RulesBuildManifest{}, errors.New("embedded rules input must be a regular file")
					}
					names[match] = true
				}
			}
		}
	}
	manifest := RulesBuildManifest{Format: "contra-rules-build-v1", Files: []RulesSourceFile{}}
	for name := range names {
		raw, e := os.ReadFile(name)
		if e != nil {
			return "", RulesBuildManifest{}, e
		}
		relative, e := filepath.Rel(root, name)
		if e != nil {
			return "", RulesBuildManifest{}, e
		}
		manifest.Files = append(manifest.Files, RulesSourceFile{Path: filepath.ToSlash(relative), SHA256: sha(raw)})
	}
	sort.Slice(manifest.Files, func(i, j int) bool { return manifest.Files[i].Path < manifest.Files[j].Path })
	id, err := RulesBuildID(manifest.Files)
	return id, manifest, err
}
