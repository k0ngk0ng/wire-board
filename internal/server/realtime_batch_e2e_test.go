package server

// Failure inventory before batching: one snapshot/transaction per simultaneous
// input; duplicate nonce within a batch applied twice; one rejected callback
// corrupts an earlier candidate; SQL failure acknowledges part of a batch;
// control changes/closure before commit permit queued input; acknowledged batch
// disappears on restart. Inputs use real sockets and the production database.
import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func batchTable(t *testing.T) *realtimeTable { return batchTableMode(t, false) }

func batchTableMode(t *testing.T, critical bool) *realtimeTable {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.AcceptStale = func(gameapi.Context, gameapi.Match, json.RawMessage) bool { return true }
	apply := module.Apply
	module.Apply = func(c gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		if strings.Contains(string(raw), `"drive":0`) {
			m.Data[0] = '!'
			return m, errors.New("rejected mutating input")
		}
		next, err := apply(c, m, raw)
		if critical && err == nil {
			var p realtimePosition
			_ = json.Unmarshal(next.Data, &p)
			next.CheckpointKey = fmt.Sprint(p.Inputs)
		}
		return next, err
	}
	f := newRealtimeTable(t, module, false)
	// Give concurrently dispatched network requests a reproducible collection
	// window under -race. Production defaults to 10ms, measured by load evidence.
	f.s.inputBatchDelay = 80 * time.Millisecond
	if _, err := f.s.db.Exec(`CREATE TEMP TABLE batch_counts(updates INTEGER); INSERT INTO batch_counts VALUES(0); CREATE TEMP TRIGGER batch_room_write AFTER UPDATE ON rooms BEGIN UPDATE batch_counts SET updates=updates+1; END;`); err != nil {
		t.Fatal(err)
	}
	return f
}
func TestRealtimeBatchAtomicDurableInputs(t *testing.T) {
	for _, mode := range []string{"success", "duplicate", "rejected", "database"} {
		t.Run(mode, func(t *testing.T) {
			f := batchTableMode(t, true)
			a, b := streamDial(t, f.a, true), streamDial(t, f.a, true)
			streamUntil(t, a, "state")
			streamUntil(t, b, "state")
			before := current(f.a)
			if mode == "database" {
				if _, err := f.s.db.Exec(`CREATE TRIGGER reject_second_batch_input BEFORE INSERT ON actions WHEN NEW.nonce='batch-two' BEGIN SELECT RAISE(ABORT,'injected batch failure'); END;`); err != nil {
					t.Fatal(err)
				}
			}
			first := map[string]any{"type": "input", "roomId": f.id, "version": before["version"], "nonce": "batch-one", "action": map[string]any{"drive": 1}}
			nonce := "batch-two"
			drive := 1
			if mode == "duplicate" {
				nonce = "batch-one"
			}
			if mode == "rejected" {
				drive = 0
			}
			second := map[string]any{"type": "input", "roomId": f.id, "version": before["version"], "nonce": nonce, "action": map[string]any{"drive": drive}}
			streamSend(t, a, first)
			streamSend(t, b, second)
			one, two := streamUntil(t, a, "ack"), streamUntil(t, b, "ack")
			after := current(f.a)
			var updates int
			if err := f.s.db.QueryRow("SELECT updates FROM batch_counts").Scan(&updates); err != nil {
				t.Fatal(err)
			}
			if mode == "database" {
				if one["status"] != float64(500) || two["status"] != float64(500) || updates != 0 || !reflect.DeepEqual(before, after) {
					t.Fatal("failed transaction leaked state/ACK", one, two, updates, after)
				}
			} else {
				if one["status"] != float64(200) || updates != 1 {
					t.Fatal("not one durable snapshot per batch", one, two, updates)
				}
				expected := 2
				if mode == "duplicate" {
					expected = 1
					if one["version"] != two["version"] || two["status"] != float64(200) {
						t.Fatal("duplicate not idempotent", one, two)
					}
				}
				if mode == "rejected" {
					expected = 1
					if two["status"] != float64(400) {
						t.Fatal("invalid input accepted", two)
					}
				}
				if got := realtimeData(t, after); got.Inputs[0] != expected {
					t.Fatal("batch input lost or leaked", got)
				}
				a.CloseNow()
				b.CloseNow()
				f.ts.Close()
				f.s.Close()
				var err error
				f.s, err = New(f.cfg, f.s.files)
				if err != nil {
					t.Fatal(err)
				}
				f.s.cancel()
				<-f.s.done
				if got := realtimeDataFromStored(t, f.s, f.id); got.Inputs[0] != expected {
					t.Fatal("durable batch lost on restart", got)
				}
			}
			streamEvidence(t, "batch-"+mode, map[string]any{"passed": true, "first": one, "second": two, "snapshotUpdates": updates, "before": before, "after": after})
		})
	}
}
func TestRealtimeQueuedInputRespectsChangedControl(t *testing.T) {
	f := batchTableMode(t, true)
	conn := streamDial(t, f.a, true)
	streamUntil(t, conn, "state")
	before := current(f.a)
	streamSend(t, conn, map[string]any{"type": "input", "roomId": f.id, "version": before["version"], "nonce": "queued-manual-input", "action": map[string]any{"drive": 1}})
	time.Sleep(10 * time.Millisecond)
	setAutoPlay(f.a, current(f.a), true, 200)
	ack := streamUntil(t, conn, "ack")
	if ack["status"] == float64(200) || realtimeData(t, current(f.a)).Inputs[0] != 0 {
		t.Fatal("queued human input survived manual takeover", ack)
	}
	if current(f.a)["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("queued input changed ownership")
	}
	streamEvidence(t, "batch-control", map[string]any{"passed": true, "ack": ack})
}

func TestRealtimeBatchMissingSnapshotRejectsAcknowledgement(t *testing.T) {
	f := batchTableMode(t, true)
	conn := streamDial(t, f.a, true)
	streamUntil(t, conn, "state")
	before := current(f.a)
	// Database fault: the durable room row disappears while the live cache still
	// exists. UPDATE affecting zero rows must never produce a successful ACK.
	if _, err := f.s.db.Exec("DELETE FROM rooms WHERE id=?", f.id); err != nil {
		t.Fatal(err)
	}
	streamSend(t, conn, map[string]any{"type": "input", "roomId": f.id, "version": before["version"], "nonce": "missing-snapshot-input", "action": map[string]any{"drive": 1}})
	ack := streamUntil(t, conn, "ack")
	if ack["status"] != float64(500) || !reflect.DeepEqual(before, current(f.a)) {
		t.Fatal("acknowledged input without durable room", ack)
	}
	var count int
	_ = f.s.db.QueryRow("SELECT COUNT(*) FROM actions WHERE room_id=? AND nonce=?", f.id, "missing-snapshot-input").Scan(&count)
	if count != 0 {
		t.Fatal("orphan input journal committed")
	}
	streamEvidence(t, "batch-missing-snapshot", map[string]any{"passed": true, "ack": ack})
}
