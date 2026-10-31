package server

// Failure cases BEFORE implementation: admin cannot inspect closed history;
// pagination drops records; ordinary users gain private history; role revocation
// retains access; match snapshots disclose private hands or account secrets.
import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAdminInspectsUserHistoryThroughPublicHTTP(t *testing.T) {
	_, ts, root := adminServer(t)
	owner, other, manager := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	owner.register("战绩被查看者")
	other.register("普通查看者")
	manager.register("战绩管理员")
	id := owner.state()["user"].(map[string]any)["id"].(string)
	managerID := manager.state()["user"].(map[string]any)["id"].(string)
	root.post("/api/admin/users/"+managerID, map[string]any{"action": "role", "role": "admin"}, 200)
	for i := 0; i < 21; i++ {
		owner.post("/api/rooms", map[string]any{"kind": "splendor", "name": fmt.Sprintf("真实中止记录 %02d", i), "capacity": 2}, 201)
		owner.command(current(owner), "add_bot", nil, 200)
		owner.command(current(owner), "ready", nil, 200)
		owner.command(current(owner), "start", nil, 200)
		owner.command(current(owner), "close", nil, 200)
		owner.command(current(owner), "leave", nil, 200)
	}
	path := "/api/players/" + id
	for _, viewer := range []*testClient{root, manager, owner} {
		code, p := viewer.request("GET", path, nil)
		if code != 200 || p["total"] != float64(21) || len(p["history"].([]any)) != 20 || p["hasMore"] != true {
			t.Fatal("incomplete managed history", code, p)
		}
		stats := p["stats"].(map[string]any)["splendor"].(map[string]any)
		if stats["played"] != float64(0) || stats["wins"] != float64(0) {
			t.Fatal("closed match changed game statistics", stats)
		}
		raw, _ := json.Marshal(p)
		for _, secret := range []string{"reserved", "tokens", "password", "chat"} {
			if strings.Contains(string(raw), secret) {
				t.Fatal("private data leaked", secret)
			}
		}
		_, p = viewer.request("GET", path+"?offset=20", nil)
		if len(p["history"].([]any)) != 1 || p["hasMore"] != false {
			t.Fatal("pagination", p)
		}
	}
	_, p := other.request("GET", path, nil)
	if p["total"] != float64(0) {
		t.Fatal("ordinary user sees closed history", p)
	}
	root.post("/api/admin/users/"+managerID, map[string]any{"action": "role", "role": "player"}, 200)
	_, p = manager.request("GET", path, nil)
	if p["total"] != float64(0) {
		t.Fatal("revoked administrator retained access", p)
	}
	if code, _ := newClient(t, ts.URL).request("GET", path, nil); code != 401 {
		t.Fatal("anonymous profile access", code)
	}
}
