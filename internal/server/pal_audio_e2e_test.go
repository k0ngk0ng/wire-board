package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalAudioLogicalClockSaveLoadHTTP(t *testing.T) {
	if os.Getenv("PAL_AUDIO_HTTP") != "1" {
		t.Skip("explicit native audio and logical-clock integration")
	}
	m := palBoardModuleAt(t, ".local/pal/runtime-candidates/audio/original/rules.wasm")
	cfg := Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{m}, ClientLoading: true}
	files := fstest.MapFS{"index.html": {Data: []byte("PAL actual audio clock")}}
	s, err := New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	defer func() { ts.Close(); s.Close() }()
	owner := newClient(t, ts.URL)
	owner.register("仙剑音乐时钟本人")
	room := owner.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版音乐存档", "capacity": 1}, 201)
	id := room["id"].(string)
	owner.command(current(owner), "ready", nil, 200)
	owner.command(current(owner), "start", nil, 200)
	releaseLoad := func() {
		t.Helper()
		gate := loadInfo(t, owner)
		for _, op := range []string{"load_begin", "load_ready"} {
			if code, body := loadingCommand(owner, current(owner), op, gate["token"].(string), gate["build"].(string), "pal-audio-clock-client"); code != 200 {
				t.Fatal(op, code, body)
			}
		}
	}
	releaseLoad()
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
		t.Fatal("actual original dialogue did not arrive")
		return nil
	}
	first := waitDialogue(nil)
	audio, ok := first["audio"].(map[string]any)
	if !ok || audio["music"] == nil {
		t.Fatal("original opening did not expose its active music", first["audio"])
	}
	music := audio["music"].(map[string]any)
	started := int64(music["startedAt"].(float64))
	sample := func() (int64, []byte) {
		s.mu.Lock()
		defer s.mu.Unlock()
		r := s.rooms[id]
		var private struct {
			Native []byte `json:"native"`
		}
		if err := json.Unmarshal(r.Module.Data, &private); err != nil {
			t.Fatal(err)
		}
		return r.gameNow(time.Now()).UnixMilli(), private.Native
	}
	before, native := sample()
	time.Sleep(350 * time.Millisecond)
	after, sameNative := sample()
	if after-before < 300 || !bytes.Equal(native, sameNative) || !reflect.DeepEqual(music, data()["audio"].(map[string]any)["music"]) {
		t.Fatal("dialogue music clock froze or changed original rules")
	}
	if _, err = s.db.Exec("CREATE TRIGGER pal_audio_save_failure BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'audio save rollback'); END"); err != nil {
		t.Fatal(err)
	}
	owner.command(current(owner), "save", nil, 500)
	if _, err = s.db.Exec("DROP TRIGGER pal_audio_save_failure"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(music, data()["audio"].(map[string]any)["music"]) {
		t.Fatal("failed save changed music origin")
	}
	oldEpoch := data()["epoch"]
	owner.command(current(owner), "save", nil, 200)
	savedAt, savedNative := sample()
	if savedAt <= started || !bytes.Equal(native, savedNative) {
		t.Fatal("saving changed original native state")
	}
	time.Sleep(250 * time.Millisecond)
	stillFrozen, _ := sample()
	if stillFrozen != savedAt {
		t.Fatal("offline music clock advanced")
	}
	ts.Close()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	cfg.Modules = []gameapi.Module{palBoardModuleAt(t, ".local/pal/runtime-candidates/audio/original/rules.wasm")}
	s, err = New(cfg, files)
	if err != nil {
		t.Fatal(err)
	}
	ts = httptest.NewServer(s.Handler())
	owner.base = ts.URL
	owner.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	resumed := data()
	if resumed["epoch"] == oldEpoch || !reflect.DeepEqual(music, resumed["audio"].(map[string]any)["music"]) {
		t.Fatal("restart/resume lost music origin")
	}
	loadingAt, loadingNative := sample()
	time.Sleep(300 * time.Millisecond)
	loadingStill, _ := sample()
	if loadingAt != savedAt || loadingStill != savedAt || !bytes.Equal(native, loadingNative) {
		t.Fatal("loading consumed saved music or original rules")
	}
	releaseLoad()
	time.Sleep(250 * time.Millisecond)
	continued, _ := sample()
	if continued-savedAt < 200 || continued-savedAt > 500 {
		t.Fatal("resume counted offline/loading time", continued-savedAt)
	}
	for i := 0; i < 2; i++ {
		v := waitDialogue(nil)
		d := v["dialogue"].(map[string]any)
		owner.command(current(owner), "action", map[string]any{"type": "read", "epoch": v["epoch"], "prompt": d["id"]}, 200)
		waitDialogue(d["id"])
	}
	beforeLoad := data()["epoch"]
	owner.command(current(owner), "load_checkpoint", nil, 200)
	reloaded := data()
	if reloaded["epoch"].(float64) <= beforeLoad.(float64) || !reflect.DeepEqual(music, reloaded["audio"].(map[string]any)["music"]) {
		t.Fatal("manual rollback lost original music position")
	}
	checkpointAt, _ := sample()
	if checkpointAt != savedAt {
		t.Fatal("manual rollback lost saved logical music phase")
	}
	releaseLoad()
	result, _ := json.MarshalIndent(map[string]any{"passed": true, "scope": "authenticated HTTP/SQLite actual original opening music, reading-time advance with unchanged native rules, failed-save rollback, offline freeze, process restart, loading gate, personal resume and manual-checkpoint clock restoration; no browser playback or full campaign claim", "music": music, "readingElapsedMS": after - before, "savedElapsedMS": savedAt - started, "resumedElapsedMS": continued - started, "checkpointElapsedMS": checkpointAt - started}, "", "  ")
	if err = os.WriteFile("../../.local/pal/verification/audio-http.json", append(result, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
