package basketball_test

import (
	"context"
	"github.com/k0ngk0ng/wire-board/internal/nettest"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/games/basketball/engine"
	game "github.com/k0ngk0ng/wire-board/games/basketball/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Explicitly opt into actual browser/Wasm assets, native input and HTTP. Normal
// host unit tests do not launch a browser or depend on local extracted artwork.
func TestBasketballBrowserNetworkBoard(t *testing.T) {
	if os.Getenv("BASKETBALL_BROWSER_E2E") != "1" {
		t.Skip("set BASKETBALL_BROWSER_E2E=1 for native browser acceptance")
	}
	incident := os.Getenv("BASKETBALL_INCIDENT_E2E") == "1"
	players, module, options := 2, observedModule(t), map[string]any{"mode": "versus"}
	if incident {
		players, module = 1, hoopIncidentModule(t)
		options = map[string]any{"mode": "solo", "court": "court-4"}
	}
	h := gametest.NewHTTP(t, players, module, "basketball")
	h.Start(options)
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
	out := filepath.Join(root, ".local/basketball/network-board")
	delayMS := 0
	if value := os.Getenv("BASKETBALL_CHECKPOINT_DELAY_MS"); value != "" {
		delayMS, err = strconv.Atoi(value)
		if err != nil || delayMS < 0 || delayMS > 2000 {
			t.Fatal("invalid checkpoint delay")
		}
		out += "-checkpoint" + value
	}
	streamTest := os.Getenv("BASKETBALL_INPUT_SYNC_E2E") == "1"
	rtt := 0
	if streamTest {
		rtt, err = strconv.Atoi(os.Getenv("BASKETBALL_BROWSER_RTT_MS"))
		if err != nil || rtt < 0 || rtt > 1000 {
			t.Fatal("invalid RTT")
		}
		out = filepath.Join(root, ".local/basketball/input-sync-rtt"+strconv.Itoa(rtt))
		if cpu := os.Getenv("BASKETBALL_BROWSER_CPU_RATE"); cpu != "" {
			out += "-cpu" + cpu
		}
	}
	if incident {
		out += "-incident"
	}
	if os.Getenv("BASKETBALL_MEDIA_E2E") == "1" {
		out += "-media"
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if delayMS > 0 && strings.HasSuffix(r.URL.Path, "/sync") && r.URL.Query().Get("probe") == "" {
			captured := httptest.NewRecorder()
			h.Host.Handler().ServeHTTP(captured, r)
			time.Sleep(time.Duration(delayMS) * time.Millisecond)
			for key, values := range captured.Header() {
				w.Header()[key] = values
			}
			w.WriteHeader(captured.Code)
			w.Write(captured.Body.Bytes())
			return
		}
		h.Host.Handler().ServeHTTP(w, r)
	}))
	mux.Handle("/__oss/", basketballOSSMirror(t, root))
	mux.Handle("/basketball/v1/runtime/", basketballRuntimeFixture(t, root))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		files := map[string]string{"/": "index.html", "/board.js": "board.js", "/board.css": "board.css", "/basketball/v1/runtime/wasm_exec.js": "wasm_exec.js", "/basketball/v1/runtime/basketball.wasm": "basketball.wasm"}
		if file, ok := files[r.URL.Path]; ok {
			http.ServeFile(w, r, filepath.Join(out, file))
			return
		}
		const prefix = "/basketball/v1/graphics/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			rel := strings.TrimPrefix(r.URL.Path, "/")
			if filepath.IsLocal(rel) && (filepath.Ext(rel) == ".png" || filepath.Ext(rel) == ".json") {
				http.ServeFile(w, r, filepath.Join(root, "web/dist", rel))
				return
			}
		}
		http.NotFound(w, r)
	})
	// The fixture must enforce the real application's browser policy. Serving
	// the page without CSP would hide a Wasm/CDN integration failure.
	policyResponse, e := h.People[0].HTTP.Get(h.Server.URL + "/")
	if e != nil {
		t.Fatal(e)
	}
	policy := policyResponse.Header.Get("Content-Security-Policy")
	policyResponse.Body.Close()
	browserHost := httptest.NewServer(nettest.Delay(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		mux.ServeHTTP(w, r)
	}), time.Duration(rtt)*time.Millisecond))
	defer browserHost.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	driver := "scripts/basketball/accept_network_board.py"
	if streamTest {
		driver = "scripts/basketball/accept_input_sync.py"
	}
	cmd := exec.CommandContext(ctx, "python3", driver)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "BASKETBALL_HTTP_URL="+browserHost.URL, "BASKETBALL_BROWSER_OUTPUT="+out)
	output, err := cmd.CombinedOutput()
	if e := os.WriteFile(filepath.Join(out, "driver.log"), output, 0600); e != nil {
		t.Fatal(e)
	}
	if err != nil {
		t.Fatalf("browser network board: %v\n%s", err, output)
	}
}
