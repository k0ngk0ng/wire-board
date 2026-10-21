package basketball_test

// E2E: actual production UI with verified OSS-byte mirrors, delayed Wasm and
// one deliberate failed image. Assert both-human barrier, progress, retry,
// current-court-only requests, no audio preload, live reload and 320px layout.
import (
	"context"
	"encoding/json"
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
	pathsRaw, e := exec.Command("node", "--input-type=module", "-e", `import {prepareBasketballAssets} from "`+root+`/games/basketball/build/assets.mjs";import {prepareBasketballRuntime} from "`+root+`/games/basketball/build/runtime.mjs";console.log(JSON.stringify({media:prepareBasketballAssets("`+root+`"),runtime:prepareBasketballRuntime("`+root+`") }));`).Output()
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
