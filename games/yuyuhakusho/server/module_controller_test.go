package yuyuhakusho

import "testing"

// Prior failure range: absent MD definition, accidental FC/default profile,
// redundant action overrides, or one caller mutating the next module definition.
func TestModuleControllerUsesMDProfile(t *testing.T) {
	module := Module()
	if module.Controls == nil || module.Controls.Layout != "md-6" || module.Controls.LegacyProfile != "md-6" || len(module.Controls.Actions) != 0 {
		t.Fatal("module did not select the platform MD profile", module.Controls)
	}
	module.Controls.Layout = "mutated"
	if next := Module(); next.Controls == nil || next.Controls.Layout != "md-6" {
		t.Fatal("module controller definition shared mutable state")
	}
}
