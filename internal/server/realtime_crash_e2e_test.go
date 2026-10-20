package server

// Failure inventory: SIGKILL bypasses graceful flush and loses the last
// checkpoint; recovery revives held input; new instance is indistinguishable
// from the crashed process. Child runs the real HTTP server and SQLite file.
import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"
)

func TestRealtimeCrashChild(t *testing.T) {
	dir := os.Getenv("WIRE_BOARD_CHECKPOINT_CRASH_DIR")
	if dir == "" {
		t.Skip("child process fixture")
	}
	cfg := Config{DataDir: dir, InviteCode: "test-invite", AdminUsername: "ModuleRoot", AdminPassword: "module-root-test-123", Modules: nil}
	cfg.Modules = append(cfg.Modules, realtimeFixture(true, time.Hour, 0, ""))
	s, err := New(cfg, fstest.MapFS{"index.html": {Data: []byte("crash fixture")}})
	if err != nil {
		t.Fatal(err)
	}
	s.cancel()
	<-s.done // Control the crash window; production periodic flush has separate coverage.
	ts := httptest.NewServer(s.Handler())
	if err = os.WriteFile(filepath.Join(dir, "child-ready.json"), []byte(`{"url":"`+ts.URL+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	select {}
}
func TestRealtimeSIGKILLRecoversCheckpoint(t *testing.T) {
	f := batchTable(t)
	checkpointInput(t, f, "checkpoint-before-crash")
	if err := flushCheckpoint(t, f); err != nil {
		t.Fatal(err)
	}
	f.ts.Close()
	if err := f.s.Close(); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(f.cfg.DataDir, "child.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	child := exec.Command(executable, "-test.run=^TestRealtimeCrashChild$", "-test.timeout=30s")
	child.Env = append(os.Environ(), "WIRE_BOARD_CHECKPOINT_CRASH_DIR="+f.cfg.DataDir)
	child.Stdout, child.Stderr = logFile, logFile
	if err = child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill()
	var ready struct {
		URL string `json:"url"`
	}
	for until := time.Now().Add(10 * time.Second); time.Now().Before(until); {
		raw, e := os.ReadFile(filepath.Join(f.cfg.DataDir, "child-ready.json"))
		if e == nil && json.Unmarshal(raw, &ready) == nil && ready.URL != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ready.URL == "" {
		raw, _ := os.ReadFile(logPath)
		t.Fatal("child did not start", string(raw))
	}
	f.a.base = ready.URL
	c := streamDial(t, f.a, true)
	bootstrap := streamUntil(t, c, "state")["state"].(map[string]any)
	before := current(f.a)
	streamSend(t, c, map[string]any{"type": "input", "roomId": f.id, "serverInstance": bootstrap["serverInstance"], "version": before["version"], "nonce": "volatile-after-checkpoint", "action": map[string]any{"drive": 1}})
	ack := streamUntil(t, c, "ack")
	if ack["status"] != float64(200) || ack["durable"] != false || realtimeData(t, current(f.a)).Inputs[0] != 2 {
		t.Fatal("child did not accept volatile step", ack)
	}
	if err = child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err = child.Wait(); err == nil {
		t.Fatal("child exited without crash")
	}
	c.CloseNow()
	f.s, err = New(f.cfg, f.s.files)
	if err != nil {
		t.Fatal(err)
	}
	f.s.cancel()
	<-f.s.done
	recovered := realtimeDataFromStored(t, f.s, f.id)
	if recovered.Inputs[0] != 1 || recovered.Drive[0] != 0 || f.s.instance == bootstrap["serverInstance"] {
		t.Fatal("crash recovery did not restore checkpoint safely", recovered)
	}
	streamEvidence(t, "sigkill-checkpoint", map[string]any{"passed": true, "ack": ack, "recovered": recovered, "crashedInstance": bootstrap["serverInstance"], "recoveredInstance": f.s.instance})
}
