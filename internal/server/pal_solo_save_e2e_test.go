package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalSoloPrivateSaveRestartHTTP(t *testing.T) {
	m := palBoardModule(t)
	if m.MinPlayers != 1 || m.MaxPlayers != 1 {
		t.Fatal("PAL is not single player")
	}
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m, savedGameFixture()}}
	files := fstest.MapFS{"index.html": {Data: []byte("original single-player PAL")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	a, stranger := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("仙剑个人存档")
	stranger.register("仙剑存档陌生账号")
	r := a.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版个人旅程", "capacity": 1}, 201)
	id := r["id"].(string)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	data := func() map[string]any { return current(a)["game"].(map[string]any)["data"].(map[string]any) }
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && data()["phase"] != "dialogue" {
		time.Sleep(30 * time.Millisecond)
	}
	before := data()
	unchanged := func(old, next map[string]any) bool {
		copyWithoutClock := func(source map[string]any) map[string]any {
			copy := make(map[string]any, len(source))
			for key, value := range source {
				if key != "controlAt" {
					if key == "local" {
						metadata := make(map[string]any)
						for name, entry := range value.(map[string]any) {
							if name != "serverAt" {
								metadata[name] = entry
							}
						}
						copy[key] = metadata
						continue
					}
					copy[key] = value
				}
			}
			return copy
		}
		return reflect.DeepEqual(copyWithoutClock(old), copyWithoutClock(next))
	}
	if before["phase"] != "dialogue" || len(before["dialogue"].(map[string]any)["read"].([]any)) != 1 {
		t.Fatal("original solo opening did not arrive")
	}
	d := before["dialogue"].(map[string]any)
	oldPage := current(a)
	if _, err = s.db.Exec("CREATE TRIGGER fail_solo_pal_read BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'solo original read rollback'); END"); err != nil {
		t.Fatal(err)
	}
	a.command(current(a), "action", map[string]any{"type": "read", "epoch": before["epoch"], "prompt": d["id"]}, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_solo_pal_read"); err != nil {
		t.Fatal(err)
	}
	if !unchanged(before, data()) {
		t.Fatal("failed solo reading changed the original page")
	}
	if _, err = s.db.Exec("CREATE TRIGGER fail_solo_pal_save BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'solo original save rollback'); END"); err != nil {
		t.Fatal(err)
	}
	a.command(current(a), "save", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER fail_solo_pal_save"); err != nil {
		t.Fatal(err)
	}
	if a.state()["room"] == nil || !unchanged(before, data()) {
		t.Fatal("failed solo save released the seat or changed the original page")
	}
	a.command(current(a), "save", nil, 200)
	if a.state()["room"] != nil || len(a.state()["saves"].([]any)) != 1 {
		t.Fatal("personal save did not release the only seat")
	}
	if len(stranger.state()["saves"].([]any)) != 0 {
		t.Fatal("stranger discovered private PAL save")
	}
	stranger.post("/api/saves/"+id+"/resume", map[string]any{}, 404)
	other := a.post("/api/rooms", map[string]any{"kind": "saved-fixture", "name": "保存后切换游戏", "capacity": 1}, 201)
	a.command(other, "leave", nil, 200)
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	a.base, stranger.base = ts.URL, ts.URL
	a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	resumed := data()
	if current(a)["status"] != "playing" || resumed["epoch"] == before["epoch"] || !reflect.DeepEqual(resumed["dialogue"], before["dialogue"]) || !reflect.DeepEqual(resumed["party"], before["party"]) || resumed["cash"] != before["cash"] || current(a)["matchId"] != oldPage["matchId"] {
		t.Fatal("single account could not immediately continue the unchanged original match")
	}
	a.command(oldPage, "action", map[string]any{"type": "read", "epoch": before["epoch"], "prompt": d["id"]}, 409)
	a.command(current(a), "action", map[string]any{"type": "read", "epoch": resumed["epoch"], "prompt": d["id"]}, 200)
	deadline = time.Now().Add(20 * time.Second)
	continued := false
	for time.Now().Before(deadline) {
		next := data()
		if next["phase"] == "dialogue" && next["dialogue"].(map[string]any)["id"] != d["id"] {
			continued = true
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if !continued {
		t.Fatal("personal continuation did not advance original reading")
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original solo opening, authenticated HTTP/SQLite/ticker, reading/save rollback, personal seat release, private ownership, switching games, restart and immediate one-account continuation with stale-page rejection; not full campaign", "players": 1, "readRollback": true, "saveRollback": true, "privateOriginalAccountOnly": true, "switchGames": true, "restartAndImmediateContinue": true, "originalMatchAndReadingPreserved": true, "stalePageRejected": true}, "", "  ")
	output := "../../.local/pal/verification/solo-save-http.json"
	if os.Getenv("PAL_STATE_JSON_E2E") == "1" {
		output = "../../.local/pal/verification/native-json-solo-save-http.json"
	}
	if err := os.WriteFile(output, report, 0600); err != nil {
		t.Fatal(err)
	}
}
