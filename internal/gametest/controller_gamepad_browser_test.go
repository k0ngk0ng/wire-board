package gametest

// Opt-in browser fixture: production auth/settings/SQLite and registered game
// modules. Only the browser hardware source is virtualized by the JS harness.
import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/games"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	contra "github.com/k0ngk0ng/wire-board/games/contra/server"
	"github.com/k0ngk0ng/wire-board/internal/server"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestControllerGamepadBrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_GAMEPAD_BROWSER") != "1" {
		t.Skip("explicit gamepad browser acceptance only")
	}
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	inside := func(value string) string {
		p, e := filepath.Abs(value)
		if e != nil {
			t.Fatal(e)
		}
		r, e := filepath.Rel(root, p)
		if e != nil || r == "." || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
			t.Fatal("fixture paths must be inside workspace")
		}
		return p
	}
	out := inside(os.Getenv("WIRE_BOARD_GAMEPAD_BROWSER_OUT"))
	web := inside(os.Getenv("WIRE_BOARD_GAMEPAD_BROWSER_WEB"))
	if err = os.Mkdir(out, 0700); err != nil {
		t.Fatal("use a fresh fixture directory", err)
	}
	modules := games.Modules()
	mux := http.NewServeMux()
	if public := os.Getenv("WIRE_BOARD_GAMEPAD_CONTRA_PUBLIC"); public != "" {
		public = inside(public)
		raw, err := os.ReadFile(filepath.Join(public, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct {
			RulesBuildID string `json:"rulesBuildID"`
		}
		if err = json.Unmarshal(raw, &manifest); err != nil {
			t.Fatal(err)
		}
		build, _, err := runtime.ReadRulesBuild(root)
		if err != nil || build != manifest.RulesBuildID {
			t.Fatal("rebuild Contra runtime for current sources", err)
		}
		digest := sha256.Sum256(raw)
		runtimeID := hex.EncodeToString(digest[:])
		visuals, err := contra.LoadVisualResources(os.DirFS(public), raw, build)
		if err != nil {
			t.Fatal(err)
		}
		bundle, err := engine.BuiltinLevelBundle()
		if err != nil {
			t.Fatal(err)
		}
		vision, err := engine.NewBuiltinBotVision(visuals.Catalog(), bundle)
		if err != nil {
			t.Fatal(err)
		}
		mod, err := contra.ModuleWithConfig(contra.Config{RulesBuildID: build, RuntimeBuildID: runtimeID, Visuals: visuals, ObserveBot: func(frame engine.BotVisualFrame) (engine.BotObservation, error) {
			return engine.ObserveBotFrame(frame, vision)
		}})
		if err != nil {
			t.Fatal(err)
		}
		mod.Controls = games.ControllerDefinitionFor("contra")
		modules = append(modules, mod)
		prefix := "/contra/runtime-" + runtimeID + "/"
		mux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(public))))
	}
	host, err := server.New(server.Config{DataDir: filepath.Join(out, "data"), InviteCode: "gamepad-browser", AdminUsername: "手柄验收管理员", AdminPassword: "gamepad-browser-test-only", Modules: modules, ClientLoading: true}, os.DirFS(web))
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mux.Handle("/", host.Handler())
	httpServer := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer httpServer.Close()
	go httpServer.Serve(listener)
	if err = os.WriteFile(filepath.Join(out, "url"), []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	for start := time.Now(); time.Since(start) < 10*time.Minute; {
		if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("gamepad fixture exceeded its deadline")
}
