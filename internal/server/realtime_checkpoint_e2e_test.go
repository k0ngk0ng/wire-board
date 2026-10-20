package server

// Failure inventory is in docs/research/realtime-efficiency/PERSISTENCE.md.
// These tests enter through authenticated HTTP/WS, observe production SQLite,
// and retain machine-readable evidence; only checkpoint fault timing is forced.
import (
	"encoding/json"
	"testing"
	"time"
)

func checkpointCounts(t *testing.T, f *realtimeTable) (int, int) {
	t.Helper()
	var writes, actions int
	if err := f.s.db.QueryRow("SELECT updates FROM batch_counts").Scan(&writes); err != nil {
		t.Fatal(err)
	}
	if err := f.s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=?", f.id).Scan(&actions); err != nil {
		t.Fatal(err)
	}
	return writes, actions
}
func checkpointInput(t *testing.T, f *realtimeTable, nonce string) map[string]any {
	t.Helper()
	c := streamDial(t, f.a, true)
	defer c.CloseNow()
	streamUntil(t, c, "state")
	streamSend(t, c, map[string]any{"type": "input", "roomId": f.id, "version": current(f.a)["version"], "nonce": nonce, "action": map[string]any{"drive": 1}})
	return streamUntil(t, c, "ack")
}
func flushCheckpoint(t *testing.T, f *realtimeTable) error {
	t.Helper()
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	return f.s.flushRealtimeCheckpoints(time.Now())
}
func TestRealtimeMemoryInputsFramesAndCheckpoint(t *testing.T) {
	f := batchTable(t)
	_, actionsBefore := checkpointCounts(t, f)
	var acks []any
	for _, nonce := range []string{"memory-one", "memory-two", "memory-one"} {
		ack := checkpointInput(t, f, nonce)
		acks = append(acks, ack)
		if ack["status"] != float64(200) || ack["durable"] != false {
			t.Fatal("ordinary input must explicitly acknowledge memory acceptance", ack)
		}
	}
	for i := 0; i < 5; i++ {
		f.frame(time.UnixMilli(f.s.rooms[f.id].Module.SystemDeadline))
	}
	writes, actions := checkpointCounts(t, f)
	state := realtimeData(t, current(f.a))
	if writes != 0 || actions != actionsBefore || state.Inputs[0] != 2 || state.Frames != 5 {
		t.Fatal("ordinary steps persisted or duplicate input replayed", writes, actions, state)
	}
	if err := flushCheckpoint(t, f); err != nil {
		t.Fatal(err)
	}
	writes, actions = checkpointCounts(t, f)
	if writes != 1 || actions != actionsBefore {
		t.Fatal("checkpoint must save one snapshot without per-step journal", writes, actions)
	}
	stored := checkpointStoredData(t, f.s, f.id)
	if stored.Inputs[0] != 2 || stored.Frames != 5 {
		t.Fatal("checkpoint missing latest memory state", stored)
	}
	if err := flushCheckpoint(t, f); err != nil {
		t.Fatal(err)
	}
	writes, _ = checkpointCounts(t, f)
	if writes != 1 {
		t.Fatal("unchanged room rewritten", writes)
	}
	streamEvidence(t, "memory-checkpoint", map[string]any{"passed": true, "acks": acks, "ordinaryWrites": 0, "checkpointWrites": writes, "state": state})
}
func TestRealtimeCheckpointFailureRetryAndCriticalControl(t *testing.T) {
	f := batchTable(t)
	checkpointInput(t, f, "before-failed-checkpoint")
	if _, err := f.s.db.Exec(`CREATE TEMP TRIGGER fail_checkpoint BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'injected checkpoint failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if flushCheckpoint(t, f) == nil {
		t.Fatal("checkpoint failure swallowed")
	}
	if got := checkpointStoredData(t, f.s, f.id); got.Inputs[0] != 0 {
		t.Fatal("failed checkpoint leaked", got)
	}
	if ack := checkpointInput(t, f, "during-storage-failure"); ack["status"] != float64(200) || ack["durable"] != false {
		t.Fatal("ordinary movement depends on storage availability", ack)
	}
	if _, err := f.s.db.Exec("DROP TRIGGER fail_checkpoint"); err != nil {
		t.Fatal(err)
	}
	if err := flushCheckpoint(t, f); err != nil {
		t.Fatal(err)
	}
	if got := checkpointStoredData(t, f.s, f.id); got.Inputs[0] != 2 {
		t.Fatal("retry lost dirty state", got)
	}
	checkpointInput(t, f, "before-control")
	setAutoPlay(f.a, current(f.a), true, 200)
	var raw []byte
	if err := f.s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", f.id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved Room
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Seats[0].AutoPlay || checkpointStoredData(t, f.s, f.id).Inputs[0] != 3 {
		t.Fatal("critical control did not save latest state")
	}
	before, _ := checkpointCounts(t, f)
	if err := flushCheckpoint(t, f); err != nil {
		t.Fatal(err)
	}
	after, _ := checkpointCounts(t, f)
	if after != before {
		t.Fatal("critical commit left room dirty")
	}
	streamEvidence(t, "checkpoint-retry-control", map[string]any{"passed": true, "writes": after, "savedVersion": saved.Version})
}
func TestRealtimeLivePeriodicCheckpoint(t *testing.T) {
	f := newRealtimeTable(t, realtimeFixture(true, 100*time.Millisecond, 0, ""), true)
	start := time.Now()
	for time.Since(start) < 2500*time.Millisecond {
		if checkpointStoredData(t, f.s, f.id).Frames >= 5 {
			streamEvidence(t, "live-checkpoint", map[string]any{"passed": true, "elapsedMS": time.Since(start).Milliseconds(), "stored": checkpointStoredData(t, f.s, f.id)})
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("live clock failed to save checkpoint")
}

func checkpointStoredData(t *testing.T, s *Server, id string) realtimePosition {
	t.Helper()
	var raw []byte
	if err := s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var r Room
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	var p realtimePosition
	if err := json.Unmarshal(r.Module.Data, &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRealtimeInstanceRejectsOldControlsAndRestoreClearsHeldInput(t *testing.T) {
	f := batchTable(t)
	c := streamDial(t, f.a, true)
	first := streamUntil(t, c, "state")
	instance, ok := first["state"].(map[string]any)["serverInstance"].(string)
	if !ok || instance == "" {
		t.Fatal("missing server instance", first)
	}
	streamSend(t, c, map[string]any{"type": "input", "serverInstance": "old-instance", "roomId": f.id, "version": current(f.a)["version"], "nonce": "old-instance-control", "action": map[string]any{"drive": 1}})
	ack := streamUntil(t, c, "ack")
	if ack["status"] != float64(409) || realtimeData(t, current(f.a)).Inputs[0] != 0 {
		t.Fatal("old instance control accepted", ack)
	}
	checkpointInput(t, f, "held-control-before-restart")
	c.CloseNow()
	f.ts.Close()
	if err := f.s.Close(); err != nil {
		t.Fatal(err)
	}
	var err error
	f.s, err = New(f.cfg, f.s.files)
	if err != nil {
		t.Fatal(err)
	}
	f.s.cancel()
	<-f.s.done
	got := realtimeDataFromStored(t, f.s, f.id)
	if got.Inputs[0] != 1 || got.Drive[0] != 0 {
		t.Fatal("restore lost game or revived held control", got)
	}
	streamEvidence(t, "instance-restore-control", map[string]any{"passed": true, "rejected": ack, "restored": got})
}

func TestRealtimeCriticalResultRequiresDurableRoom(t *testing.T) {
	f := newRealtimeTable(t, realtimeFixture(true, time.Hour, 1, ""), false)
	before := current(f.a)
	if _, err := f.s.db.Exec("DELETE FROM rooms WHERE id=?", f.id); err != nil {
		t.Fatal(err)
	}
	f.frame(time.UnixMilli(f.s.rooms[f.id].Module.SystemDeadline))
	after := current(f.a)
	if after["status"] != "playing" || after["version"] != before["version"] {
		t.Fatal("published result without durable room", after)
	}
	var histories int
	if err := f.s.db.QueryRow("SELECT COUNT(*) FROM match_history").Scan(&histories); err != nil {
		t.Fatal(err)
	}
	if histories != 0 {
		t.Fatal("orphan result archived", histories)
	}
	streamEvidence(t, "critical-result-missing-row", map[string]any{"passed": true, "historyRows": histories})
}
