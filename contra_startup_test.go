package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
)

// Missing production identities must fail before database creation or listen.
// A compatibility registry must not silently start a server without Contra.
func TestContraStartupRejectsUnboundBuildBeforeDatabase(t *testing.T) {
	rules, artifact := runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID
	t.Cleanup(func() { runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID = rules, artifact })
	runtime.CompiledRulesBuildID, runtime.CompiledRuntimeBuildID = "", ""
	directory := filepath.Join(t.TempDir(), "not-created")
	t.Setenv("DATA_DIR", directory)
	t.Setenv("ADDR", "invalid listen address")
	t.Setenv("INVITE_CODE", "contra-startup-test-only")
	err := run()
	if err == nil || !strings.Contains(err.Error(), "initialize contra") {
		t.Errorf("unexpected startup result: %v", err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Errorf("unbound build initialized database directory: %v", err)
	}
}
