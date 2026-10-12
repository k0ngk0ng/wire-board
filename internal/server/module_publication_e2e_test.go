package server

// Failure scenarios before the default-publication change: a newly registered
// module appears without approval; direct create bypasses hiding; a player can
// publish; startup resets an explicit choice; registering another module turns
// it on; legacy availability is changed by the migration.
import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func publishModuleForTest(t *testing.T, base, kind string) {
	t.Helper()
	admin := newClient(t, base)
	admin.post("/api/login", map[string]string{"name": "ModuleRoot", "password": "module-root-test-123"}, 200)
	admin.post("/api/admin/games/"+kind, map[string]bool{"enabled": true}, 200)
}

func TestNewModuleRequiresAdminPublicationE2E(t *testing.T) {
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: []gameapi.Module{acceptanceModule()}}
	files := fstest.MapFS{"index.html": {Data: []byte("ok")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	root, player := newClient(t, ts.URL), newClient(t, ts.URL)
	root.post("/api/login", map[string]string{"name": "ModuleRoot", "password": "module-root-test-123"}, 200)
	player.register("默认下架玩家")
	visible := func(c *testClient, kind string) bool {
		for _, id := range c.state()["availableGames"].([]any) {
			if id == kind {
				return true
			}
		}
		return false
	}
	listed := func(kind string) bool {
		for _, entry := range overview(t, root)["games"].([]any) {
			game := entry.(map[string]any)
			if game["kind"] == kind {
				return game["enabled"].(bool)
			}
		}
		t.Fatal("module absent from admin", kind)
		return false
	}
	trace := []any{}
	t.Cleanup(func() {
		out := filepath.Join("..", "..", ".local", "monopoly", "acceptance", "publication")
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Error(err)
			return
		}
		raw, _ := json.MarshalIndent(map[string]any{"passed": !t.Failed(), "test": t.Name(), "trace": trace}, "", "  ")
		if err := os.WriteFile(filepath.Join(out, "result.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	if listed("acceptance") || visible(player, "acceptance") || visible(root, "acceptance") {
		t.Fatal("new module was auto-published")
	}
	if !visible(player, "splendor") {
		t.Fatal("legacy default unexpectedly changed")
	}
	player.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "不能创建", "capacity": 2}, 403)
	root.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "管理员也需先上架", "capacity": 2}, 403)
	player.post("/api/admin/games/acceptance", map[string]bool{"enabled": true}, 403)
	trace = append(trace, map[string]any{"newModuleVisible": false, "playerPublishRejected": true})
	root.post("/api/admin/games/acceptance", map[string]bool{"enabled": true}, 200)
	root.post("/api/admin/games/splendor", map[string]bool{"enabled": false}, 200)
	if !visible(player, "acceptance") || !listed("acceptance") {
		t.Fatal("admin publication failed")
	}
	r := player.post("/api/rooms", map[string]any{"kind": "acceptance", "name": "已上架", "capacity": 2}, 201)
	player.command(r, "leave", nil, 200)
	ts.Close()
	s.Close()
	second := acceptanceModule()
	second.ID = "second_module"
	second.Name = "后续新增"
	cfg.Modules = append(cfg.Modules, second)
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	root.base, player.base = ts.URL, ts.URL
	if !visible(player, "acceptance") || visible(player, "second_module") || listed("second_module") || visible(player, "splendor") {
		t.Fatal("restart/new registration changed publication decisions")
	}
	root.post("/api/admin/games/acceptance", map[string]bool{"enabled": false}, 200)
	ts.Close()
	s.Close()
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	root.base, player.base = ts.URL, ts.URL
	if visible(player, "acceptance") {
		t.Fatal("restart republished hidden module")
	}
	trace = append(trace, map[string]any{"explicitPublicationPersisted": true, "laterModuleDefaultHidden": true, "explicitUnpublicationPersisted": true, "legacyChoicePreserved": true})
}

// Composition check: shipping the module must not silently publish it. After
// the administrator enables it, the production registry must run the real game.
func TestProductionMonopolyDefaultOffE2E(t *testing.T) {
	s, err := New(Config{Modules: games.Modules(), DataDir: t.TempDir(), InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123"}, fstest.MapFS{"index.html": {Data: []byte("ok")}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	root := newClient(t, ts.URL)
	root.post("/api/login", map[string]string{"name": "ModuleRoot", "password": "module-root-test-123"}, 200)
	found := false
	for _, entry := range overview(t, root)["games"].([]any) {
		game := entry.(map[string]any)
		if game["kind"] == "monopoly" {
			found = true
			if game["enabled"].(bool) {
				t.Fatal("production monopoly starts published")
			}
		}
	}
	if !found {
		t.Fatal("production monopoly not registered")
	}
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("正式棋手甲")
	b.register("正式棋手乙")
	a.post("/api/rooms", map[string]any{"kind": "monopoly", "name": "未上架强手棋", "capacity": 2}, 403)
	root.post("/api/admin/games/monopoly", map[string]bool{"enabled": true}, 200)
	r := a.post("/api/rooms", map[string]any{"kind": "monopoly", "name": "正式强手棋", "capacity": 2}, 201)
	b.command(r, "join", nil, 200)
	r = a.state()["room"].(map[string]any)
	a.command(r, "ready", nil, 200)
	r = b.state()["room"].(map[string]any)
	b.command(r, "ready", nil, 200)
	r = a.state()["room"].(map[string]any)
	a.command(r, "start", nil, 200)
	r = a.state()["room"].(map[string]any)
	if r["status"] != "playing" {
		t.Fatal("real game did not start", r)
	}
	game := r["game"].(map[string]any)["data"].(map[string]any)
	if len(game["board"].([]any)) != 40 || len(game["players"].([]any)) != 2 {
		t.Fatal("real game view missing")
	}
}
