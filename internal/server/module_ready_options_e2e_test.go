package server

// Before implementation: a dirty map draft is lost when the host readies;
// bots become unready; other humans retain approval of different rules;
// nonhosts change settings; invalid options partially mutate readiness;
// unchanged options reset other ready humans; restart loses saved setup.
import (
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestModuleSaveOptionsAndReadyAtomically(t *testing.T) {
	mod := acceptanceModule()
	mod.MaxPlayers = 4
	mod.Configure = func(n int, raw json.RawMessage) (json.RawMessage, error) {
		var o struct {
			Map string `json:"map"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &o); err != nil {
				return nil, err
			}
		}
		if o.Map == "" {
			o.Map = "a"
		}
		if n < 2 || n > 4 || (o.Map != "a" && o.Map != "b") {
			return nil, errors.New("invalid setup")
		}
		return json.Marshal(o)
	}
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{mod}}
	files := fstest.MapFS{"index.html": {Data: []byte("ready options")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer func() { ts.Close(); s.Close() }()
	publishModuleForTest(t, ts.URL, mod.ID)
	host, peer := newClient(t, ts.URL), newClient(t, ts.URL)
	host.register("准备房主")
	peer.register("准备同桌")
	r := host.post("/api/rooms", map[string]any{"kind": mod.ID, "name": "地图准备", "capacity": 3}, 201)
	peer.command(r, "join", nil, 200)
	host.command(current(host), "add_bot", nil, 200)
	readyOptions := func(c *testClient, mapID string, status int) {
		r := current(c)
		c.post("/api/rooms/"+r["id"].(string), map[string]any{"type": "ready", "version": r["version"], "nonce": randomID(12), "options": map[string]any{"map": mapID}}, status)
	}
	host.command(current(host), "ready", nil, 200)
	peer.command(current(peer), "ready", nil, 200)
	readyOptions(host, "b", 200)
	r = current(host)
	seats := r["seats"].([]any)
	if r["options"].(map[string]any)["map"] != "b" || !seats[0].(map[string]any)["ready"].(bool) || seats[1].(map[string]any)["ready"].(bool) || !seats[2].(map[string]any)["ready"].(bool) {
		t.Fatalf("map+ready not atomic or bot lost ready: %+v", r)
	}
	host.command(r, "start", nil, 400)
	before := current(host)
	readyOptions(peer, "a", 400)
	readyOptions(host, "invalid", 400)
	if !reflect.DeepEqual(before, current(host)) {
		t.Fatal("rejected settings changed room")
	}
	peer.command(current(peer), "ready", nil, 200)
	readyOptions(host, "b", 200)
	if !current(host)["seats"].([]any)[1].(map[string]any)["ready"].(bool) {
		t.Fatal("same settings reset another human")
	}
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	host.base = ts.URL
	peer.base = ts.URL
	r = current(host)
	for _, seat := range r["seats"].([]any) {
		if !seat.(map[string]any)["ready"].(bool) {
			t.Fatal("restart lost ready")
		}
	}
	if r["options"].(map[string]any)["map"] != "b" {
		t.Fatal("restart lost map")
	}
	host.command(r, "start", nil, 200)
	if current(host)["status"] != "playing" {
		t.Fatal("could not start with ready bot")
	}
	out := filepath.Join("..", "..", ".local", "racing-remake", "ready-options.json")
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "authenticated HTTP, three seats, bot ready, setup permissions, transactional rejection and real SQLite restart"}, "", "  ")
	if err = os.WriteFile(out, data, 0600); err != nil {
		t.Fatal(err)
	}
}
