package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalManualCheckpointReloadAndOldPageHTTP(t *testing.T) {
	if os.Getenv("PAL_CHECKPOINT_E2E") != "1" {
		t.Skip("explicit actual original PAL manual-checkpoint integration")
	}
	core := ".local/pal/runtime-candidates/presentation/original/rules.wasm"
	if os.Getenv("PAL_FRESH_CI") == "1" {
		core = ".local/pal/native/rules.wasm"
	}
	m := palBoardModuleAt(t, core)
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}}, fstest.MapFS{"index.html": {Data: []byte("original PAL checkpoint")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()
	defer s.Close()
	owner := newClient(t, ts.URL)
	owner.register("仙剑手动读档本人")
	room := owner.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版剧情回退", "capacity": 1}, 201)
	id := room["id"].(string)
	owner.command(current(owner), "ready", nil, 200)
	owner.command(current(owner), "start", nil, 200)
	matchID := current(owner)["matchId"]
	data := func() map[string]any { return current(owner)["game"].(map[string]any)["data"].(map[string]any) }
	waitDialogue := func(different any) map[string]any {
		t.Helper()
		for until := time.Now().Add(30 * time.Second); time.Now().Before(until); {
			v := data()
			if d, ok := v["dialogue"].(map[string]any); ok && (different == nil || d["id"] != different) {
				return v
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("original PAL did not reach the expected dialogue page")
		return nil
	}
	waitDialogue(nil)
	owner.command(current(owner), "save", nil, 200)
	s.mu.Lock()
	if s.rooms[id].Checkpoint == nil {
		s.mu.Unlock()
		t.Fatal("actual PAL save did not retain an independent manual checkpoint")
	}
	frozen, err := m.View(s.rooms[id].Checkpoint.Match, 0)
	baseline, encodeErr := json.Marshal(s.rooms[id].Checkpoint)
	s.mu.Unlock()
	if err != nil || encodeErr != nil {
		t.Fatal(err, encodeErr)
	}
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	first := waitDialogue(nil)
	oldPage := map[string]any{"type": "read", "epoch": first["epoch"], "prompt": first["dialogue"].(map[string]any)["id"]}
	for i := 0; i < 3; i++ {
		v := data()
		d := v["dialogue"].(map[string]any)
		owner.command(current(owner), "action", map[string]any{"type": "read", "epoch": v["epoch"], "prompt": d["id"]}, 200)
		waitDialogue(d["id"])
	}
	before := data()
	owner.command(current(owner), "load_checkpoint", nil, 200)
	after := waitDialogue(nil)
	if after["epoch"].(float64) <= before["epoch"].(float64) {
		t.Fatal("PAL rollback reused an old native input generation", first["epoch"], before["epoch"], after["epoch"])
	}
	normalize := func(v any) map[string]any {
		b, _ := json.Marshal(v)
		var out map[string]any
		json.Unmarshal(b, &out)
		for _, key := range []string{"epoch", "seq", "appliedSeq", "discardedSeq", "controlAt", "local"} {
			delete(out, key)
		}
		return out
	}
	beforeLocal := before["local"].(map[string]any)
	afterLocal := after["local"].(map[string]any)
	if beforeLocal["core"] == nil || beforeLocal["core"] != afterLocal["core"] || afterLocal["generation"].(float64) <= beforeLocal["generation"].(float64) {
		t.Fatal("PAL manual reload lost its original core or retained old local inputs", beforeLocal, afterLocal)
	}
	if !reflect.DeepEqual(normalize(frozen), normalize(after)) {
		different := []string{}
		for key, value := range normalize(frozen) {
			if !reflect.DeepEqual(value, normalize(after)[key]) {
				different = append(different, key)
			}
		}
		t.Fatal("PAL manual reload changed the saved original page, drawing, party or resources", different)
	}
	owner.command(current(owner), "action", oldPage, 400)
	time.Sleep(250 * time.Millisecond)
	if !reflect.DeepEqual(normalize(after), normalize(data())) {
		t.Fatal("old original PAL page or held input advanced the restored checkpoint")
	}
	s.mu.Lock()
	retained, err := json.Marshal(s.rooms[id].Checkpoint)
	s.mu.Unlock()
	if err != nil || !reflect.DeepEqual(baseline, retained) || current(owner)["matchId"] != matchID {
		t.Fatal("original PAL checkpoint changed during manual reload", err)
	}
	out := filepath.Join("..", "..", ".local/saved-games")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	report, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "actual original new-game PAL dialogue, personal save/resume, three ordinary original pages, independent manual rollback with unchanged drawing/resources and higher control generation, stale original first-page input sent using current platform version rejected; no native battle-death or browser acceptance", "oldEpoch": before["epoch"], "restoredEpoch": after["epoch"], "frozenView": frozen, "restoredView": after}, "", "  ")
	output := filepath.Join(out, "pal-checkpoint-dialogue.json")
	if os.Getenv("PAL_STATE_JSON_E2E") == "1" {
		output = filepath.Join("..", "..", ".local/pal/verification/native-json-checkpoint-http.json")
	}
	if err = os.WriteFile(output, append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
