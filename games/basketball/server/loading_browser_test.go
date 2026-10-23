package basketball_test

// E2E: actual production UI with verified OSS-byte mirrors, delayed Wasm and
// one deliberate failed image. Assert both-human barrier, progress, retry,
// current-court-only requests, no audio preload, live reload and 320px layout.
import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBasketballBrowserLoading(t *testing.T) {
	if os.Getenv("BASKETBALL_BROWSER_E2E") != "1" {
		t.Skip("set BASKETBALL_BROWSER_E2E=1")
	}
	root, _ := filepath.Abs("../../..")
	h := gametest.NewHTTP(t, 2, observedModule(t), "basketball")
	h.Config.AssetsBaseURL = "https://cdn.ichenj.com"
	h.Start(map[string]any{"mode": "versus", "court": "court-6"})
	h.Restart()
	response, e := h.People[0].HTTP.Get(h.Server.URL + "/")
	if e != nil {
		t.Fatal(e)
	}
	policy := response.Header.Get("Content-Security-Policy")
	response.Body.Close()
	mux := http.NewServeMux()
	mux.Handle("/api/", h.Host.Handler())
	mux.Handle("/__oss/", basketballOSSMirror(t, root))
	mux.Handle("/", http.FileServer(http.Dir(filepath.Join(root, "web/dist"))))
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		mux.ServeHTTP(w, r)
	}))
	defer host.Close()
	out := filepath.Join(root, ".local/basketball/loading-browser")
	os.MkdirAll(out, 0755)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "scripts/basketball/accept_loading.py")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "BASKETBALL_HTTP_URL="+host.URL, "BASKETBALL_BROWSER_OUTPUT="+out)
	output, e := cmd.CombinedOutput()
	os.WriteFile(filepath.Join(out, "driver.log"), output, 0600)
	if e != nil {
		t.Fatalf("browser loading: %v\n%s", e, output)
	}
}

func basketballOSSMirror(t *testing.T, root string) http.Handler {
	t.Helper()
	pathsRaw, e := exec.Command("node", "--input-type=module", "-e", `import {prepareBasketballAssets} from "`+root+`/games/basketball/build/assets.mjs";import {readFileSync} from "node:fs";import assert from "node:assert/strict";import {prepareBasketballRuntime} from "`+root+`/games/basketball/build/runtime.mjs";const mediaRoot=process.env.BASKETBALL_TEST_MEDIA_ROOT||"`+root+`";assert.equal(JSON.parse(readFileSync(mediaRoot+"/games/basketball/assets/release-manifest.json")).contentSHA256,JSON.parse(readFileSync("`+root+`/games/basketball/assets/release-manifest.json")).contentSHA256);console.log(JSON.stringify({media:prepareBasketballAssets(mediaRoot),runtime:prepareBasketballRuntime("`+root+`") }));`).Output()
	if e != nil {
		t.Fatal(e)
	}
	var paths map[string]string
	if e = json.Unmarshal(pathsRaw, &paths); e != nil {
		t.Fatal(e)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/__oss/")
		if strings.HasSuffix(p, "/basketball.wasm") {
			http.ServeFile(w, r, filepath.Join(paths["runtime"], "basketball/v1/runtime/basketball.wasm"))
			return
		}
		i := strings.Index(p, "/basketball/v1/")
		if i < 0 {
			http.NotFound(w, r)
			return
		}
		relative := p[i+1:]
		if !filepath.IsLocal(relative) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(paths["media"], relative))
	})
}

// Local candidate descriptor for browser acceptance before publishing its
// immutable Wasm object. The browser mirror serves the exact compiled bytes.
func basketballRuntimeFixture(t *testing.T, root string) http.Handler {
	t.Helper()
	command := exec.Command("node", "--input-type=module", "-e", `import {prepareBasketballRuntime} from "`+root+`/games/basketball/build/runtime.mjs";console.log(prepareBasketballRuntime("`+root+`"))`)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(strings.TrimSpace(string(output)), "basketball/v1/runtime")
	raw, err := os.ReadFile(filepath.Join(dir, "build.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	file := manifest["files"].(map[string]any)["basketball.wasm"].(map[string]any)
	manifest["wasm"] = map[string]any{"sourceSHA256": manifest["sourceSHA256"], "sha256": file["sha256"], "bytes": file["bytes"], "url": fmt.Sprintf("https://cdn.ichenj.com/bga/basketball/runtime-%s/basketball.wasm", file["sha256"])}
	descriptor, _ := json.Marshal(manifest)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch filepath.Base(r.URL.Path) {
		case "build.json":
			w.Header().Set("Content-Type", "application/json")
			w.Write(descriptor)
		case "wasm_exec.js":
			http.ServeFile(w, r, filepath.Join(dir, "wasm_exec.js"))
		default:
			http.NotFound(w, r)
		}
	})
}
