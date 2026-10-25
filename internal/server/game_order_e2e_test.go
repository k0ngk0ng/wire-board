package server

// Before implementation: unauthorized writes, duplicate/missing/unknown IDs,
// stale editors, failed audit/DB writes, restart loss, new games overwriting the
// order, and ordering accidentally changing publication or room state.
import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"testing/fstest"
)

func adminOrder(t *testing.T, c *testClient) []string {
	t.Helper()
	out := []string{}
	for _, item := range overview(t, c)["games"].([]any) {
		out = append(out, item.(map[string]any)["kind"].(string))
	}
	return out
}
func TestAdminGameOrderPersistenceAndValidationE2E(t *testing.T) {
	s, ts, root := adminServer(t)
	initial := adminOrder(t, root)
	order := slices.Clone(initial)
	slices.Reverse(order)
	player := newClient(t, ts.URL)
	player.register("OrderPlayer")
	body := map[string]any{"order": order, "expectedOrder": initial}
	player.post("/api/admin/games/order", body, 403)
	root.post("/api/admin/games/order", body, 200)
	if !slices.Equal(adminOrder(t, root), order) {
		t.Fatal("admin order not applied")
	}
	root.post("/api/admin/games/"+order[0], map[string]bool{"enabled": false}, 200)
	available := player.state()["availableGames"].([]any)
	if len(available) != len(order)-1 {
		t.Fatal("order changed publication")
	}
	for i, v := range available {
		if v != order[i+1] {
			t.Fatal("player order differs")
		}
	}
	for _, invalid := range [][]string{order[:1], append([]string{"unknown"}, order[1:]...), append([]string{order[1]}, order[1:]...)} {
		root.post("/api/admin/games/order", map[string]any{"order": invalid, "expectedOrder": order}, 400)
	}
	root.post("/api/admin/games/order", map[string]any{"order": initial, "expectedOrder": initial}, 409)
	if _, e := s.db.Exec("CREATE TRIGGER fail_order_audit BEFORE INSERT ON admin_audit BEGIN SELECT RAISE(ABORT,'test'); END"); e != nil {
		t.Fatal(e)
	}
	root.post("/api/admin/games/order", map[string]any{"order": initial, "expectedOrder": order}, 500)
	if !slices.Equal(adminOrder(t, root), order) {
		t.Fatal("failed transaction changed memory")
	}
	s.db.Exec("DROP TRIGGER fail_order_audit")
	cfg := s.cfg
	ts.Close()
	s.Close()
	cfg.Modules = []gameapi.Module{acceptanceModule()}
	restored, e := New(cfg, fstest.MapFS{})
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	next := httptest.NewServer(restored.Handler())
	defer next.Close()
	c := newClient(t, next.URL)
	c.post("/api/login", map[string]string{"name": "RootKeeper", "password": "root-test-password-123"}, 200)
	want := append(slices.Clone(order), "acceptance")
	if !slices.Equal(adminOrder(t, c), want) {
		t.Fatal("restart/new registration lost order", adminOrder(t, c), want)
	}
	if !restored.hiddenGames[order[0]] {
		t.Fatal("restart republished hidden game")
	}
	var count int
	restored.db.QueryRow("SELECT count(*) FROM admin_audit WHERE action='game_order'").Scan(&count)
	if count != 1 {
		t.Fatal("audit was not atomic", count)
	}
	out := filepath.Join("..", "..", ".local", "near-order")
	os.MkdirAll(out, 0755)
	raw, _ := json.MarshalIndent(map[string]any{"passed": true, "initial": initial, "saved": order, "restored": want, "publicationPreserved": true, "rejectedStaleAndInvalid": true, "auditAtomic": true}, "", "  ")
	os.WriteFile(filepath.Join(out, "admin-http.json"), raw, 0600)
}
