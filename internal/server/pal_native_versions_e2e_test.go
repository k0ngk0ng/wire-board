package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALRetainedNativeVersionsRestartHTTP(t *testing.T) {
	if os.Getenv("PAL_VERSION_CATALOG") != "1" {
		t.Skip("explicit retained native platform acceptance")
	}
	load := func(path string) *engine.Runtime {
		t.Helper()
		data, err := os.ReadFile("../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		r, err := engine.NewRuntime(context.Background(), data, os.DirFS("../../.local/pal/original-98"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close(context.Background()) })
		return r
	}
	old := load(".local/pal/runtime-versions/7ca83b5bcf13e71c1020ef287d9817e5675adf2cbdf7c627644f4cb3566dd03a/rules.wasm")
	currentCore := load(".local/pal/runtime-candidates/original/rules.wasm")
	onlyNew, err := engine.NewRuntimeCatalog(currentCore)
	if err != nil {
		t.Fatal(err)
	}
	retained, err := engine.NewRuntimeCatalog(currentCore, old)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{pal.NewModule(old)}}
	files := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("native version platform acceptance")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("旧核心存档本人")
	b.register("核心更新陌生人")
	r := a.post("/api/rooms", map[string]any{"kind": "pal", "name": "升级前旅程", "capacity": 1}, 201)
	id := r["id"].(string)
	a.command(current(a), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	data := func(client *testClient) map[string]any {
		return current(client)["game"].(map[string]any)["data"].(map[string]any)
	}
	waitPage := func(client *testClient) {
		t.Helper()
		for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); {
			if data(client)["phase"] == "dialogue" {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("original native reading did not arrive")
	}
	waitPage(a)
	before, oldPage := data(a), current(a)
	a.command(current(a), "save", nil, 200)
	if a.state()["room"] != nil {
		t.Fatal("old save did not release original owner")
	}
	stored := func() []byte {
		t.Helper()
		var raw []byte
		if err := s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return raw
	}
	originalStored := stored()
	restart := func(module gameapi.Module) {
		t.Helper()
		ts.Close()
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		cfg.Modules = []gameapi.Module{module}
		var err error
		s, err = New(cfg, files)
		if err != nil {
			t.Fatal(err)
		}
		ts = httptest.NewServer(s.Handler())
		a.base, b.base = ts.URL, ts.URL
	}
	restart(pal.NewVersionedModule(onlyNew))
	missingResponse := a.post("/api/saves/"+id+"/resume", map[string]any{}, 400)
	message, _ := missingResponse["error"].(string)
	if !strings.Contains(message, "存档已保留") || strings.Contains(message, old.Identity().BuildID) {
		t.Fatal("missing-version player feedback is technical or fails to preserve confidence", message)
	}
	if !bytes.Equal(originalStored, stored()) || a.state()["room"] != nil || len(a.state()["saves"].([]any)) != 1 {
		t.Fatal("missing core mutated saved data or occupied original owner")
	}
	b.post("/api/saves/"+id+"/resume", map[string]any{}, 404)
	newRoom := b.post("/api/rooms", map[string]any{"kind": "pal", "name": "更新后新旅程", "capacity": 1}, 201)
	b.command(current(b), "ready", nil, 200)
	b.command(current(b), "start", nil, 200)
	waitPage(b)
	native := func(roomID string) []byte {
		t.Helper()
		s.mu.Lock()
		defer s.mu.Unlock()
		var state struct {
			Native []byte `json:"native"`
		}
		if err := json.Unmarshal(s.rooms[roomID].Module.Data, &state); err != nil {
			t.Fatal(err)
		}
		return state.Native
	}
	newNative := native(newRoom["id"].(string))
	newSnapshot, err := currentCore.Decode(newNative)
	if err != nil || newSnapshot.BuildID != currentCore.Identity().BuildID {
		t.Fatal("new game did not use current native core", err)
	}
	b.command(current(b), "save", nil, 200)
	restart(pal.NewVersionedModule(retained))
	a.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	after := data(a)
	for _, key := range []string{"scene", "map", "x", "y", "cash", "party", "dialogue", "menu", "draws", "overlay", "raster"} {
		if !reflect.DeepEqual(before[key], after[key]) {
			t.Fatal("retained original native continuation changed", key)
		}
	}
	if current(a)["matchId"] != oldPage["matchId"] || after["epoch"] == before["epoch"] {
		t.Fatal("retained version reset match or control identity")
	}
	resumedNative := native(id)
	resumedSnapshot, err := old.Decode(resumedNative)
	if err != nil || resumedSnapshot.BuildID != old.Identity().BuildID {
		t.Fatal("retained save was relabeled", err)
	}
	if _, err = currentCore.Decode(resumedNative); err == nil {
		t.Fatal("old native memory was migrated into current ABI")
	}
	d := after["dialogue"].(map[string]any)
	a.command(oldPage, "action", map[string]any{"type": "read", "epoch": before["epoch"], "prompt": d["id"]}, 409)
	a.command(current(a), "action", map[string]any{"type": "read", "epoch": after["epoch"], "prompt": d["id"]}, 200)
	advanced := false
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		v := data(a)
		if v["phase"] == "dialogue" && v["dialogue"].(map[string]any)["id"] != d["id"] {
			advanced = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !advanced {
		t.Fatal("retained core could not continue original reading")
	}
	var history, ratings int
	if err = s.db.QueryRow("SELECT COUNT(*) FROM match_history").Scan(&history); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow("SELECT COUNT(*) FROM rating_ledger").Scan(&ratings); err != nil {
		t.Fatal(err)
	}
	if history != 0 || ratings != 0 {
		t.Fatal("version continuation settled a live save")
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original native gameplay with authenticated HTTP, SQLite and ticker; service upgrade/restart, missing-version rollback with byte-identical saved row, private original owner, new game on current core, exact old page/party on retained old core, unchanged match identity, old control rejection and no settlement; not native memory migration", "old": old.Identity(), "current": currentCore.Identity(), "missingCorePreservedSave": true, "oldNativeIdentityPreserved": true, "newGameCurrentCore": true, "privateOwnerOnly": true, "oldInputRejected": true, "history": history, "ratings": ratings}, "", "  ")
	output := "../../.local/pal/verification/native-version-http.json"
	if os.Getenv("PAL_STATE_JSON_E2E") == "1" {
		output = "../../.local/pal/verification/native-json-version-http.json"
	}
	if err = os.WriteFile(output, report, 0600); err != nil {
		t.Fatal(err)
	}
}
