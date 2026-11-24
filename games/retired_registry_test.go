package games

import (
	"slices"
	"testing"
)

func TestRetiredIndependentModulesAbsent(t *testing.T) {
	retired := []string{"stoneage", "spf2t"}
	for _, module := range Modules() {
		if slices.Contains(retired, module.ID) {
			t.Fatalf("retired module remains registered: %s", module.ID)
		}
	}
	for _, kind := range retired {
		if ControllerDefinitionFor(kind) != nil {
			t.Fatalf("retired controller remains registered: %s", kind)
		}
	}
}
