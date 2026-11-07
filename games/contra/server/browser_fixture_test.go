package contra

// Explicit browser-only host: production registry stays unchanged. This serves
// the actual platform shell, auth, rooms and loading barrier with a private DB.
// Runtime assets are mounted only below their immutable rules-build directory.
import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	hostserver "github.com/k0ngk0ng/wire-board/internal/server"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestContraPlatformBrowserFixture(t *testing.T) {
	if os.Getenv("CONTRA_PLATFORM_BROWSER") != "1" {
		t.Skip("explicit platform browser fixture only")
	}
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	inside := func(value string) string {
		p, e := filepath.Abs(value)
		if e != nil {
			t.Fatal(e)
		}
		rel, e := filepath.Rel(root, p)
		if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatal("fixture path outside workspace")
		}
		return p
	}
	public := os.Getenv("CONTRA_PLATFORM_BROWSER_PUBLIC")
	if public == "" {
		public = filepath.Join(root, ".local/contra-browser/build/public")
	}
	public = inside(public)
	web := os.Getenv("CONTRA_PLATFORM_BROWSER_WEB")
	if web == "" {
		web = filepath.Join(root, "web/dist")
	}
	web = inside(web)
	if _, e = os.Stat(filepath.Join(web, "index.html")); e != nil {
		t.Fatal("build the real platform frontend first", e)
	}
	webManifest, e := os.ReadFile(filepath.Join(web, "game-load-manifest.json"))
	if e != nil {
		t.Fatal("frontend loading manifest is required for build binding", e)
	}
	webDigest := sha256.Sum256(webManifest)
	raw, e := os.ReadFile(filepath.Join(public, "manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var manifest struct {
		RulesBuildID string `json:"rulesBuildID"`
	}
	if e = json.Unmarshal(raw, &manifest); e != nil {
		t.Fatal(e)
	}
	build, _, e := runtime.ReadRulesBuild(root)
	if e != nil {
		t.Fatal(e)
	}
	if manifest.RulesBuildID != build {
		t.Fatal("runtime assets and current engine differ; rebuild both before browser acceptance")
	}
	runtimeDigest := sha256.Sum256(raw)
	runtimeBuild := hex.EncodeToString(runtimeDigest[:])
	visuals, e := LoadVisualResources(os.DirFS(public), raw, build)
	if e != nil {
		t.Fatal("load the same verified visual assets served to players", e)
	}
	_, productionVisualID, e := ProductionVisualManifest()
	if e != nil || productionVisualID != visuals.ID() {
		t.Fatal("browser pixels differ from native production catalogue", e)
	}
	bundle, e := engine.BuiltinLevelBundle()
	if e != nil {
		t.Fatal(e)
	}
	vision, e := engine.NewBuiltinBotVision(visuals.Catalog(), bundle)
	if e != nil {
		t.Fatal("construct the real immutable visual knowledge", e)
	}
	mod, e := ModuleWithConfig(Config{RulesBuildID: build, RuntimeBuildID: runtimeBuild, Visuals: visuals, ObserveBot: func(frame engine.BotVisualFrame) (engine.BotObservation, error) {
		return engine.ObserveBotFrame(frame, vision)
	}})
	if e != nil {
		t.Fatal(e)
	}
	base := filepath.Join(root, ".local/contra-platform")
	if e = os.MkdirAll(base, 0755); e != nil {
		t.Fatal(e)
	}
	out := os.Getenv("CONTRA_PLATFORM_BROWSER_OUT")
	if out == "" {
		out, e = os.MkdirTemp(base, "browser-")
		if e != nil {
			t.Fatal(e)
		}
	} else {
		out = inside(out)
		if e = os.Mkdir(out, 0700); e != nil {
			t.Fatal("use a new evidence directory", e)
		}
	}
	var victory *victoryBoundary
	var victoryGate atomic.Bool
	victoryRelease := ""
	if directory := os.Getenv("CONTRA_VICTORY_BOUNDARY"); directory != "" {
		victory, e = loadVictoryBoundary(directory)
		if e != nil {
			t.Fatal(e)
		}
		mod = withVictoryBoundary(mod, victory, &victoryGate)
		victoryRelease = filepath.Join(out, "release-victory")
	}
	password := "contra-browser-test-only"
	platform, e := hostserver.New(hostserver.Config{DataDir: filepath.Join(out, "db"), InviteCode: "contra-browser", AdminUsername: "魂斗罗验收管理员", AdminPassword: password, Modules: []gameapi.Module{mod}, ClientLoading: true}, os.DirFS(web))
	if e != nil {
		t.Fatal(e)
	}
	defer platform.Close()
	prefix := "/contra/runtime-" + runtimeBuild + "/"
	mux := http.NewServeMux()
	mux.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(public))))
	mux.Handle("/", platform.Handler())
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	defer server.Close()
	go server.Serve(listener)
	url := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 10 * time.Second}
	for _, name := range []string{"魂斗罗验收队友", "魂斗罗验收观战"} {
		body, _ := json.Marshal(map[string]string{"name": name, "password": password, "invite": "contra-browser"})
		response, e := client.Post(url+"/api/register", "application/json", bytes.NewReader(body))
		if e != nil {
			t.Fatal(e)
		}
		b, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatal("fixture registration", response.StatusCode, string(b))
		}
	}
	response, e := client.Get(url + prefix + "manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	served, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 || !bytes.Equal(served, raw) {
		t.Fatal("immutable manifest route differs")
	}
	info := map[string]any{"url": url, "rulesBuildID": build, "runtimeBuildID": runtimeBuild, "visualsID": visuals.ID(), "manifest": url + prefix + "manifest.json", "webRoot": web, "publicRoot": public, "webManifestSHA256": hex.EncodeToString(webDigest[:]), "stop": filepath.Join(out, "stop"), "users": []map[string]string{{"name": "魂斗罗验收管理员", "password": password}, {"name": "魂斗罗验收队友", "password": password}, {"name": "魂斗罗验收观战", "password": password}}, "scope": "Private real-platform browser fixture; no production registration"}
	if victory != nil {
		info["victory"] = map[string]any{"boundary": victory, "directory": victory.Directory, "release": victoryRelease, "ratingProjectionOnly": true}
	}
	encoded, _ := json.MarshalIndent(info, "", "  ")
	if e = os.WriteFile(filepath.Join(out, "fixture.json"), encoded, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(out, "url"), []byte(url), 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("platform browser fixture:", out)
	if os.Getenv("CONTRA_PLATFORM_BROWSER_SMOKE") == "1" {
		return
	}
	for started := time.Now(); time.Since(started) < 20*time.Minute; {
		if victory != nil {
			if _, e = os.Stat(victoryRelease); e == nil {
				victoryGate.Store(true)
			}
		}
		if _, e = os.Stat(filepath.Join(out, "stop")); e == nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("platform browser fixture exceeded its deadline")
}
