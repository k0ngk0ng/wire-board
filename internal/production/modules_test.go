package production

import (
	"reflect"
	"strings"
	"testing"

	"github.com/k0ngk0ng/wire-board/games"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
)

// Failure cases: missing linked identities silently omit Contra, return a
// partial production list, panic, or register duplicate IDs without bot hooks.
func TestContraProductionRegistrationRequiresIdentities(t *testing.T) {
	rules, artifact := runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID
	t.Cleanup(func() { runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID = rules, artifact })
	runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID = "", ""
	modules, err := Modules()
	if err == nil || !strings.Contains(err.Error(), "contra") || modules != nil {
		t.Fatalf("missing identities must reject the complete production composition: modules=%d error=%v", len(modules), err)
	}
}

func TestContraProductionRegistrationWithLinkedBuild(t *testing.T) {
	if runtime.CompiledRulesBuildID == "" || runtime.CompiledRuntimeBuildID == "" {
		t.Skip("requires real verified runtime linker identities")
	}
	modules, err := Modules()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, module := range modules {
		if seen[module.ID] {
			t.Fatalf("duplicate module %s", module.ID)
		}
		seen[module.ID] = true
		if module.ID == "contra" && !reflect.DeepEqual(module.Controls, games.ControllerDefinitionFor("contra")) {
			t.Fatal("production Contra lost its controller definition")
		}
		if module.ID == "contra" && (module.BotAction != nil || module.Reclaim != nil || module.BotSearch != nil) {
			t.Fatal("Contra human-only release exposes computer/autoplay capabilities")
		}
	}
	if !seen["contra"] || len(modules) != len(games.Modules())+1 {
		t.Fatal("production game list is incomplete")
	}
}
