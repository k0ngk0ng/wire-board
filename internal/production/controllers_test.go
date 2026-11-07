package production

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	runtime "github.com/k0ngk0ng/wire-board/games/contra/runtime"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

// Failure inventory: the development fixture supplies controller metadata but
// the production composition omits it, hiding the profile or rejecting saves.
func TestProductionControllerProfileE2E(t *testing.T) {
	if runtime.CompiledRulesBuildID == "" || runtime.CompiledRuntimeBuildID == "" {
		t.Skip("requires verified runtime linker identities")
	}
	modules, err := Modules()
	if err != nil {
		t.Fatal(err)
	}
	app, err := server.New(server.Config{DataDir: t.TempDir(), InviteCode: "production-profile-test", Modules: modules}, fstest.MapFS{"index.html": {Data: []byte("controller")}})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	ts := httptest.NewServer(app.Handler())
	defer ts.Close()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	response, err := client.Post(ts.URL+"/api/register", "application/json", bytes.NewBufferString(`{"name":"ProductionPad","password":"production-profile-only","invite":"production-profile-test"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("register", response.StatusCode)
	}
	response, err = client.Get(ts.URL + "/api/controllers?game=contra")
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
		Profiles map[string]json.RawMessage `json:"profiles"`
	}
	err = json.NewDecoder(response.Body).Decode(&result)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 || len(result.Models) != 1 || result.Models[0].ID != "game:contra" || len(result.Profiles) != 1 {
		t.Fatalf("production profile: status=%d result=%+v err=%v", response.StatusCode, result, err)
	}
	profile := result.Profiles["game:contra"]
	if len(profile) == 0 {
		t.Fatal("missing production profile")
	}
	response, err = client.Post(ts.URL+"/api/controllers/game:contra", "application/json", bytes.NewReader(profile))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("production defaults cannot be saved", response.StatusCode)
	}
}
