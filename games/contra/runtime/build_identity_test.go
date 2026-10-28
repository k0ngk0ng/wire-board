package runtime

// New code-identity failure inventory before implementation: platform wrappers
// or tests contaminate the shared ID; embedded rule data are omitted; directory
// order changes the hash; duplicate/outside paths or placeholder IDs are used;
// a resource-bundle ID substitutes for code identity; mismatched code snapshots
// restore despite equal epoch/resource data. Collection happens in test setup.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRulesBuild(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	id, manifest, err := ReadRulesBuild(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 64 || len(manifest.Files) == 0 {
		t.Fatal("missing actual rules build")
	}
	return id
}
func TestRulesBuildIdentityAndCollection(t *testing.T) {
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	id, m, err := ReadRulesBuild(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		seen[f.Path] = true
		if !strings.HasPrefix(f.Path, "games/contra/engine/") || strings.HasSuffix(f.Path, "_test.go") {
			t.Fatal("wrapper/test in engine ID", f)
		}
	}
	for _, name := range []string{"session.go", "audio_control_data.json", "presentation_sprites_data.json", "leveldata/stage-1.json", "leveldata/stage-8.json"} {
		if !seen["games/contra/engine/"+name] {
			t.Fatal("engine/embed input omitted", name)
		}
	}
	copy := append([]RulesSourceFile(nil), m.Files...)
	for a, b := 0, len(copy)-1; a < b; a, b = a+1, b-1 {
		copy[a], copy[b] = copy[b], copy[a]
	}
	other, err := RulesBuildID(copy)
	if err != nil || other != id {
		t.Fatal("manifest ordering changed ID", err)
	}
	copy[0].SHA256 = strings.Repeat("0", 64)
	other, err = RulesBuildID(copy)
	if err != nil || other == id {
		t.Fatal("changed input retained code identity")
	}
	if _, err = RulesBuildID(append(copy, copy[0])); err == nil {
		t.Fatal("duplicate source accepted")
	}
	if _, err = RulesBuildID([]RulesSourceFile{{Path: "games/contra/runtime/bridge.go", SHA256: strings.Repeat("0", 64)}}); err == nil {
		t.Fatal("platform wrapper counted as shared rules")
	}
}
func TestRuntimeCodeIdentityCannotBeSubstituted(t *testing.T) {
	raw, err := os.ReadFile("../engine/leveldata/stage-1.json")
	if err != nil {
		t.Fatal(err)
	}
	code := testRulesBuild(t)
	b := New(code)
	if _, err = b.Call(Request{Op: "resources", Resources: raw}); err != nil {
		t.Fatal(err)
	}
	created, err := b.Call(Request{Op: "new", Seed: "1", Players: 1, Epoch: "code-bound"})
	if err != nil {
		t.Fatal(err)
	}
	view := created.(Snapshot)
	if view.RulesBuildID != code || view.Resources == code {
		t.Fatal("resource identity substituted for code")
	}
	checkpoint, err := b.Call(Request{Op: "checkpoint", Session: view.Session, Epoch: view.Epoch})
	if err != nil {
		t.Fatal(err)
	}
	c := checkpoint.(Checkpoint)
	if c.RulesBuildID != code {
		t.Fatal("checkpoint missing code identity")
	}
	c.RulesBuildID = strings.Repeat("0", 64)
	if _, err = b.Call(Request{Op: "restore", Epoch: view.Epoch, Checkpoint: &c}); err == nil {
		t.Fatal("wrong build checkpoint accepted")
	}
	for _, bad := range []string{"", "dev-v1", view.Resources[:16]} {
		badBridge := New(bad)
		if _, err = badBridge.Call(Request{Op: "resources", Resources: json.RawMessage(raw)}); err == nil {
			t.Fatal("placeholder code ID accepted", bad)
		}
	}
}
