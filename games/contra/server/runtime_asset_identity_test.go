package contra

// Before runtime asset binding: a stage confirmation for the same rules can
// silently acknowledge a different executable/loader; an application restart
// with a new artifact can retain an old human ready flag. Only the existing
// resource barrier may be renewed; Session, revision and input sequence stay.
import (
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"strings"
	"testing"
)

func TestRuntimeArtifactStageConfirmationAndRestore(t *testing.T) {
	cfg := boundaryVisualConfig(t, strings.Repeat("a", 64))
	cfg.RuntimeBuildID = strings.Repeat("b", 64)
	m, err := ModuleWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, ctx, g := initialWithModule(t, m, 2)
	state := saved(t, g)
	state.StageLoading = &StageLoading{Level: 0, Ready: []bool{true, false}, RuntimeBuildID: cfg.RuntimeBuildID}
	g, err = store(g, &state)
	if err != nil {
		t.Fatal(err)
	}
	bundle, _ := engine.BuiltinLevelBundle()
	action := map[string]any{"type": "stage-ready", "epoch": state.Epoch, "sequence": 1, "bundleID": state.BundleID, "rulesBuildID": cfg.RulesBuildID, "level": 0, "stageSHA256": bundle.Manifest().Stages[0].SHA256}
	for _, runtimeID := range []string{"", strings.Repeat("c", 64)} {
		action["runtimeBuildID"] = runtimeID
		raw, _ := json.Marshal(action)
		if _, err = m.Apply(ctx, g, raw); err == nil {
			t.Fatal("wrong runtime acknowledged")
		}
	}
	action["runtimeBuildID"] = cfg.RuntimeBuildID
	raw, _ := json.Marshal(action)
	if _, err = m.Apply(ctx, g, raw); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(state.Session)
	cfg.RuntimeBuildID = strings.Repeat("d", 64)
	next, err := ModuleWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := next.Restore(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	result := saved(t, restored)
	after, _ := json.Marshal(result.Session)
	if string(before) != string(after) || result.Inputs[0].Sequence != state.Inputs[0].Sequence || result.StageLoading.Ready[0] || result.StageLoading.RuntimeBuildID != cfg.RuntimeBuildID {
		t.Fatal("artifact change reset rules or retained an obsolete readiness flag")
	}
	state.StageLoading = nil
	g, err = store(g, &state)
	if err != nil {
		t.Fatal(err)
	}
	restored, err = next.Restore(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	if saved(t, restored).StageLoading != nil {
		t.Fatal("artifact update invented a reconnect barrier")
	}
}
