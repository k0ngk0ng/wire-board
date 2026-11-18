package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/games/pal/engine"
	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALAuthorityArchiveRestartAndManualCheckpointHTTP(t *testing.T) {
	if os.Getenv("PAL_AUTHORITY_HTTP") != "1" {
		t.Skip("explicit original raw-to-authority package save compatibility")
	}
	core := ".local/pal/runtime-candidates/audio/original/rules.wasm"
	if os.Getenv("PAL_FRESH_CI") == "1" {
		core = ".local/pal/native/rules.wasm"
	}
	raw := palBoardModuleAt(t, core)
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{raw}, ClientLoading: true}
	files := fstest.MapFS{"index.html": {Data: []byte("prepared PAL authority protocol")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer func() { ts.Close(); s.Close() }()
	owner, stranger := newClient(t, ts.URL), newClient(t, ts.URL)
	owner.register("规则包续档本人")
	stranger.register("规则包续档陌生账号")
	room := owner.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版存档规则包升级", "capacity": 1}, 201)
	id := room["id"].(string)
	owner.command(current(owner), "ready", nil, 200)
	owner.command(current(owner), "start", nil, 200)
	releaseLoad := func() {
		t.Helper()
		gate := loadInfo(t, owner)
		for _, command := range []string{"load_begin", "load_ready"} {
			if code, body := loadingCommand(owner, current(owner), command, gate["token"].(string), gate["build"].(string), "pal-authority-client"); code != 200 {
				t.Fatal(command, code, body)
			}
		}
	}
	data := func() map[string]any { return current(owner)["game"].(map[string]any)["data"].(map[string]any) }
	waitReading := func(previous any) map[string]any {
		t.Helper()
		for until := time.Now().Add(30 * time.Second); time.Now().Before(until); {
			value := data()
			if dialogue, ok := value["dialogue"].(map[string]any); ok && (previous == nil || dialogue["id"] != previous) {
				return value
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("actual original reading did not arrive")
		return nil
	}
	releaseLoad()
	before := waitReading(nil)
	oldPage := current(owner)
	matchID := oldPage["matchId"]
	owner.command(oldPage, "save", nil, 200)
	if owner.state()["room"] != nil || len(stranger.state()["saves"].([]any)) != 0 {
		t.Fatal("private original save did not release its owner safely")
	}
	stranger.post("/api/saves/"+id+"/resume", map[string]any{}, 404)
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	archive, err := os.ReadFile("../../.local/pal/authority-prepared/authority.zip")
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.ReadFile("../../" + core)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := engine.NewAuthorityRuntime(context.Background(), program, archive)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close(context.Background())
	cfg.Modules = []gameapi.Module{pal.NewModule(runtime)}
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	owner.base, stranger.base = ts.URL, ts.URL
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	if current(owner)["loading"] == nil {
		t.Fatal("prepared authority resume bypassed the public loading gate")
	}
	releaseLoad()
	after := waitReading(nil)
	if !reflect.DeepEqual(before["dialogue"], after["dialogue"]) || !reflect.DeepEqual(before["party"], after["party"]) || before["cash"] != after["cash"] || current(owner)["matchId"] != matchID || before["epoch"] == after["epoch"] {
		t.Fatal("prepared authority changed the original saved reading, party, money or match")
	}
	dialogue := before["dialogue"].(map[string]any)
	owner.command(oldPage, "action", map[string]any{"type": "read", "epoch": before["epoch"], "prompt": dialogue["id"]}, 409)
	owner.command(current(owner), "action", map[string]any{"type": "read", "epoch": after["epoch"], "prompt": dialogue["id"]}, 200)
	advanced := waitReading(dialogue["id"])
	owner.command(current(owner), "load_checkpoint", nil, 200)
	releaseLoad()
	loaded := waitReading(nil)
	if !reflect.DeepEqual(loaded["dialogue"], before["dialogue"]) || !reflect.DeepEqual(loaded["party"], before["party"]) || loaded["cash"] != before["cash"] || loaded["epoch"].(float64) <= advanced["epoch"].(float64) {
		t.Fatal("prepared authority did not restore the independent original manual save")
	}
	hash := sha256.Sum256(archive)
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual authenticated HTTP/SQLite raw-resource original opening save, restart with archive-only runtime, private owner resume, loading gate, same native story/party/money/match, stale page rejection, normal reading and manual checkpoint rollback; no native identity edits or production registration claim", "archiveSHA256": hex.EncodeToString(hash[:]), "privateSave": true, "preparedRestart": true, "manualCheckpoint": true, "strangerRejected": true, "oldPageRejected": true}, "", "  ")
	if err = os.WriteFile("../../.local/pal/verification/authority-http.json", append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
