package production

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Failure cases: an unbound production executable creates its database, reaches
// listen, or silently omits the native game. Exercise the actual entry point.
func TestStartupRejectsUnboundBuildBeforeDatabase(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	binary := filepath.Join(scratch, "unbound-app")
	directory := filepath.Join(scratch, "not-created")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pkg := "github.com/k0ngk0ng/wire-board/games/contra/runtime."
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-ldflags=-X "+pkg+"CompiledRulesBuildID= -X "+pkg+"CompiledRuntimeBuildID=", "-o", binary, ".")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build unbound application: %v\n%s", err, out)
	}
	t.Setenv("DATA_DIR", directory)
	t.Setenv("ADDR", "invalid listen address")
	t.Setenv("INVITE_CODE", "startup-test-only")
	command := exec.CommandContext(ctx, binary)
	command.Dir = root
	out, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "initialize contra") {
		t.Fatalf("unexpected startup result: %v\n%s", err, out)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("unbound build initialized database directory: %v", err)
	}
}
