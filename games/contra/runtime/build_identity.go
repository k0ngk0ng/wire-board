package runtime

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"sort"
	"strings"
)

// CompiledRulesBuildID is supplied by the native/Wasm build, never inferred from
// a map bundle or read from the repository inside game callbacks.
var CompiledRulesBuildID string

// CompiledRuntimeBuildID is the exact manifest.json SHA-256 for the immutable
// browser artifact. It is linked into native hosts after the Wasm is built;
// injecting it into Wasm would create a self-referential content digest.
var CompiledRuntimeBuildID string

type RulesSourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type RulesBuildManifest struct {
	Format string            `json:"format"`
	Files  []RulesSourceFile `json:"files"`
}

func validRulesBuildID(id string) bool {
	if len(id) != 64 || strings.ToLower(id) != id {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func RulesBuildID(files []RulesSourceFile) (string, error) {
	if len(files) == 0 {
		return "", errors.New("missing shared engine inputs")
	}
	ordered := append([]RulesSourceFile(nil), files...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })
	for i, f := range ordered {
		if !strings.HasPrefix(f.Path, "games/contra/engine/") || path.Clean(f.Path) != f.Path || strings.Contains(f.Path, "\\") || strings.HasSuffix(f.Path, "_test.go") || !validRulesBuildID(f.SHA256) || i > 0 && ordered[i-1].Path == f.Path {
			return "", errors.New("invalid shared engine input manifest")
		}
	}
	raw, err := json.Marshal(RulesBuildManifest{Format: "contra-rules-build-v1", Files: ordered})
	if err != nil {
		return "", err
	}
	return sha(raw), nil
}
func selectedRulesBuild(ids []string) (string, error) {
	id := CompiledRulesBuildID
	if len(ids) > 1 {
		return "", errors.New("multiple rules build identities")
	}
	if len(ids) == 1 {
		id = ids[0]
	}
	if !validRulesBuildID(id) {
		return "", errors.New("missing or invalid compiled rules build identity")
	}
	return id, nil
}
