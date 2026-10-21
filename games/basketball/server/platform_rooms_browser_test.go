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

	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Explicitly opt into actual browser/Wasm assets, native input and HTTP. Normal
// host unit tests do not launch a browser or depend on local extracted artwork.
func TestBasketballBrowserPlatformRooms(t *testing.T) {
	if os.Getenv("BASKETBALL_BROWSER_E2E") != "1" {
		t.Skip("set BASKETBALL_BROWSER_E2E=1 for native browser acceptance")
	}
	h := gametest.NewHTTP(t, 2, observedModule(t), "basketball")
	h.People[0].Request("POST", "/api/admin/games/basketball", map[string]bool{"enabled": true}, 200)
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local/basketball/platform-rooms")
	if rtt := os.Getenv("BASKETBALL_BROWSER_RTT_MS"); rtt != "" {
		ms, e := strconv.Atoi(rtt)
		if e != nil || ms < 0 || ms > 1000 {
			t.Fatal("invalid test RTT")
		}
		out += "-rtt" + strconv.Itoa(ms)
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", h.Host.Handler())
	mux.Handle("/__oss/", basketballOSSMirror(t, root))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		relative := strings.TrimPrefix(r.URL.Path, "/")
		if relative == "" {
			relative = "index.html"
		}
		if filepath.IsLocal(relative) {
			http.ServeFile(w, r, filepath.Join(root, "web/dist", relative))
			return
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
	rtt, _ := strconv.Atoi(os.Getenv("BASKETBALL_BROWSER_RTT_MS"))
	if rtt < 0 || rtt > 1000 {
		t.Fatal("invalid test RTT")
	}
	browserHost := httptest.NewServer(nettest.Delay(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		mux.ServeHTTP(w, r)
	}), time.Duration(rtt)*time.Millisecond))
	defer browserHost.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "python3", "scripts/basketball/accept_platform_rooms.py")
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
