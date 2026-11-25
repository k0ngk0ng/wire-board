package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func gameConfigFlag(t *testing.T, c *testClient, kind string) bool {
	t.Helper()
	for _, entry := range overview(t, c)["games"].([]any) {
		item := entry.(map[string]any)
		if item["kind"] == kind {
			config, ok := item["config"].(map[string]any)
			if !ok {
				t.Fatal("missing base config", kind, item)
			}
			flag, ok := config["autoFullscreen"].(bool)
			if !ok {
				t.Fatal("missing fullscreen flag", config)
			}
			return flag
		}
	}
	t.Fatal("missing game", kind)
	return false
}

func TestAdminGameConfigPersistenceAndAtomicityE2E(t *testing.T) {
	s, ts, root := adminServer(t)
	player := newClient(t, ts.URL)
	player.register("ConfigPlayer")
	guest := newClient(t, ts.URL)
	path := "/api/admin/games/rail/config"
	on := map[string]bool{"autoFullscreen": true}
	if gameConfigFlag(t, root, "rail") {
		t.Fatal("base config must default to false")
	}
	guest.post(path, on, 401)
	player.post(path, on, 403)
	root.post("/api/admin/games/unknown/config", on, 404)
	for _, body := range []map[string]any{{}, {"autoFullscreen": nil}, {"autoFullscreen": "true"}, {"autoFullscreen": 1}} {
		root.post(path, body, 400)
	}
	room := player.post("/api/rooms", map[string]any{"kind": "rail", "capacity": 2, "name": "配置验收"}, 201)
	order, _ := json.Marshal(adminOrder(t, root))
	root.post(path, on, 200)
	if !gameConfigFlag(t, root, "rail") || gameConfigFlag(t, root, "splendor") {
		t.Fatal("game configuration leaked")
	}
	state := player.state()
	if state["gameConfigs"].(map[string]any)["rail"].(map[string]any)["autoFullscreen"] != true || state["room"].(map[string]any)["status"] != "waiting" {
		t.Fatal("player config delivery changed waiting room", room)
	}
	currentOrder, _ := json.Marshal(adminOrder(t, root))
	if string(order) != string(currentOrder) {
		t.Fatal("base config changed game order")
	}
	for _, trigger := range []string{
		"CREATE TRIGGER fail_config BEFORE UPDATE ON game_base_configs BEGIN SELECT RAISE(ABORT,'test'); END",
		"CREATE TRIGGER fail_config BEFORE INSERT ON admin_audit BEGIN SELECT RAISE(ABORT,'test'); END",
	} {
		if _, err := s.db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		root.post(path, map[string]bool{"autoFullscreen": false}, 500)
		if !gameConfigFlag(t, root, "rail") {
			t.Fatal("failed save changed memory")
		}
		var enabled bool
		if err := s.db.QueryRow("SELECT auto_fullscreen FROM game_base_configs WHERE kind='rail'").Scan(&enabled); err != nil || !enabled {
			t.Fatal("failed save changed database", err)
		}
		if _, err := s.db.Exec("DROP TRIGGER fail_config"); err != nil {
			t.Fatal(err)
		}
	}
	root.post("/api/admin/games/rail", map[string]bool{"enabled": false}, 200)
	root.post(path, map[string]bool{"autoFullscreen": false}, 200)
	if gameConfigFlag(t, root, "rail") || !s.hiddenGames["rail"] {
		t.Fatal("configuration changed publication")
	}
	root.post(path, on, 200)
	var count int
	if err := s.db.QueryRow("SELECT count(*) FROM admin_audit WHERE action='game_config'").Scan(&count); err != nil || count != 3 {
		t.Fatal("configuration audit was not atomic", count, err)
	}
	cfg := s.cfg
	ts.Close()
	s.Close()
	cfg.Modules = []gameapi.Module{acceptanceModule()}
	restored, err := New(cfg, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	next := httptest.NewServer(restored.Handler())
	defer next.Close()
	client := newClient(t, next.URL)
	client.post("/api/login", map[string]string{"name": "RootKeeper", "password": "root-test-password-123"}, 200)
	if !gameConfigFlag(t, client, "rail") || gameConfigFlag(t, client, "acceptance") || !restored.hiddenGames["rail"] {
		t.Fatal("restart lost configuration or publication")
	}
	out := filepath.Join("..", "..", ".local", "admin-game-config")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, err := json.MarshalIndent(map[string]any{"passed": true, "authorization": true, "validation": true, "waitingRoomPreserved": true, "orderPreserved": true, "publicationPreserved": true, "atomicDatabaseAndAudit": true, "restart": true, "newGameDefaults": true}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "http-result.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestAdminGameConfigLegacyDatabaseE2E(t *testing.T) {
	s, ts, root := adminServer(t)
	root.post("/api/admin/games/rail", map[string]bool{"enabled": false}, 200)
	if _, err := s.db.Exec("DROP TABLE game_base_configs"); err != nil {
		t.Fatal(err)
	}
	cfg := s.cfg
	ts.Close()
	s.Close()
	restored, err := New(cfg, fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	next := httptest.NewServer(restored.Handler())
	defer next.Close()
	client := newClient(t, next.URL)
	client.post("/api/login", map[string]string{"name": "RootKeeper", "password": "root-test-password-123"}, 200)
	if gameConfigFlag(t, client, "rail") || !restored.hiddenGames["rail"] {
		t.Fatal("legacy upgrade changed defaults or publication")
	}
	client.post("/api/admin/games/rail/config", map[string]bool{"autoFullscreen": true}, 200)
	if !gameConfigFlag(t, client, "rail") {
		t.Fatal("upgraded database cannot save config")
	}
}
