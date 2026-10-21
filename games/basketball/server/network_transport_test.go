package basketball_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// The deterministic queue tests cannot prove that stale room versions and
// authenticated HTTP retries obey the same contract. Use the shipped TS
// transport with deliberately delayed actual requests to cover that boundary.
func TestBasketballHTTPJavaScriptTransport(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node is required for the JavaScript/HTTP transport integration")
	}
	h := gametest.NewHTTP(t, 4, observedModule(t), "basketball")
	h.Start(map[string]any{})
	basketballReadyHTTP(t, h)
	a := h.People[0]
	v := await(t, a, func(v game.View) bool { return v.Runtime.Playing != nil })
	a.Action(buttons(v, 0, engine.ButtonStart), 200)
	v = await(t, a, func(v game.View) bool { return v.Phase == "paused" })
	a.Action(map[string]any{"type": "release", "epoch": v.Epoch, "sequence": v.Inputs[0].Sequence + 1}, 200)
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	testContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(testContext, "node", "--experimental-strip-types", "scripts/basketball/accept_network_transport.mjs")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "BASKETBALL_HTTP_URL="+h.Server.URL)
	output, err := cmd.CombinedOutput()
	dir := filepath.Join(root, ".local/basketball/server-http")
	if e := os.MkdirAll(dir, 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(dir, "javascript-transport.jsonl"), output, 0600); e != nil {
		t.Fatal(e)
	}
	if err != nil {
		t.Fatalf("JavaScript transport acceptance: %v\n%s", err, output)
	}
}
