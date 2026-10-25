package server

// Failure cases written before the realtime pause fix: the first controlled
// seat pauses but later seats still act in the same frame; a timer/input pause
// or resume is published/acknowledged before durable storage; SQL failure leaks
// a paused frame or resumes the game; a duplicate or retry changes it twice.
// These use authenticated HTTP/WS, production frame dispatch and real SQLite.
import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func pausedRealtimeFixture(source string, calls *atomic.Int64) gameapi.Module {
	m := realtimeFixture(true, time.Hour, 0, "")
	apply, timer := m.Apply, m.OnTimer
	pause := func(v gameapi.Match) gameapi.Match {
		v.Paused = true
		v.Deadline, v.SystemDeadline, v.TimeoutActors, v.Attention = 0, 0, nil, []int{}
		return v
	}
	m.Apply = func(c gameapi.Context, v gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var a struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(raw, &a)
		if a.Type == "pause" {
			next, err := apply(c, v, json.RawMessage(`{"drive":1}`))
			return pause(next), err
		}
		if a.Type == "resume" {
			v.Paused = false
			v.SystemDeadline = c.Now.Add(time.Hour).UnixMilli()
			return v, nil
		}
		return apply(c, v, raw)
	}
	m.BotAction = func(c gameapi.Context, v gameapi.Match) (json.RawMessage, error) {
		calls.Add(1)
		if source == "bot" {
			if c.Actor == 1 {
				return json.RawMessage(`{"type":"pause"}`), nil
			}
			return json.RawMessage(`{"drive":1}`), nil
		}
		return nil, nil
	}
	m.OnTimer = func(c gameapi.Context, v gameapi.Match) (gameapi.Match, error) {
		next, err := timer(c, v)
		if source == "timer" {
			next = pause(next)
		}
		return next, err
	}
	return m
}

func pauseStoredRoom(t *testing.T, f *realtimeTable) Room {
	t.Helper()
	var raw []byte
	if err := f.s.db.QueryRow("SELECT snapshot FROM rooms WHERE id=?", f.id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var room Room
	if err := json.Unmarshal(raw, &room); err != nil {
		t.Fatal(err)
	}
	return room
}

func pauseRealtimeProof(t *testing.T, name string, proof any) {
	t.Helper()
	out := filepath.Join("..", "..", ".local", "module-pause")
	if err := os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(proof, "", "  ")
	if err := os.WriteFile(filepath.Join(out, name+".json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestModulePausedRealtimeFrameHTTP(t *testing.T) {
	for _, source := range []string{"bot", "timer"} {
		for _, fail := range []bool{false, true} {
			name := source
			if fail {
				name += "-rollback"
			}
			t.Run(name, func(t *testing.T) {
				var calls atomic.Int64
				f := newRealtimeTable(t, pausedRealtimeFixture(source, &calls), false)
				before, storedBefore := current(f.a), pauseStoredRoom(t, f)
				if fail {
					if _, err := f.s.db.Exec("CREATE TRIGGER fail_pause_frame BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'pause frame rollback'); END"); err != nil {
						t.Fatal(err)
					}
				}
				f.s.mu.Lock()
				err := f.s.runRealtimeModuleFrame(f.s.rooms[f.id], time.UnixMilli(f.s.rooms[f.id].Module.SystemDeadline))
				f.s.mu.Unlock()
				if fail {
					if err == nil {
						t.Fatal("pause frame bypassed durable storage")
					}
					if !reflect.DeepEqual(before, current(f.a)) || !reflect.DeepEqual(storedBefore, pauseStoredRoom(t, f)) {
						t.Fatal("failed pause frame leaked")
					}
					if _, err := f.s.db.Exec("DROP TRIGGER fail_pause_frame"); err != nil {
						t.Fatal(err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					after := current(f.a)
					stored := pauseStoredRoom(t, f)
					if !stored.Module.Paused || stored.Version != f.s.rooms[f.id].Version {
						t.Fatal("paused frame published before durable snapshot", stored.Module.Paused, stored.Version)
					}
					data := realtimeData(t, after)
					if source == "bot" && (calls.Load() != 1 || !reflect.DeepEqual(data.Inputs, []int{0, 1, 0, 0}) || data.Frames != 0) {
						t.Fatal("later frame callbacks ran after pause", calls.Load(), data)
					}
					if source == "timer" && data.Frames != 1 {
						t.Fatal("timer pause failed", data)
					}
					f.frame(time.Now().Add(48 * time.Hour))
					if !reflect.DeepEqual(after, current(f.a)) {
						t.Fatal("paused realtime frame advanced")
					}
				}
				pauseRealtimeProof(t, "realtime-frame-"+name, map[string]any{"test": t.Name(), "passed": !t.Failed(), "room": current(f.a), "stored": pauseStoredRoom(t, f), "callbacks": calls.Load()})
			})
		}
	}
}

func TestModulePausedRealtimeInputDurabilityHTTP(t *testing.T) {
	var calls atomic.Int64
	f := newRealtimeTable(t, pausedRealtimeFixture("input", &calls), false)
	c := streamDial(t, f.a, true)
	streamUntil(t, c, "state")
	request := map[string]any{"type": "input", "roomId": f.id, "version": current(f.a)["version"], "nonce": "durable-pause", "action": map[string]any{"type": "pause"}}
	streamSend(t, c, request)
	ack := streamUntil(t, c, "ack")
	if ack["status"] != float64(200) || ack["durable"] != true || !pauseStoredRoom(t, f).Module.Paused {
		t.Fatal("pause ACK not durable", ack)
	}
	paused := current(f.a)
	streamSend(t, c, request)
	repeated := streamUntil(t, c, "ack")
	if repeated["version"] != ack["version"] || !reflect.DeepEqual(roomWithoutNetwork(t, paused), roomWithoutNetwork(t, current(f.a))) {
		t.Fatal("pause duplicated", repeated)
	}
	if _, err := f.s.db.Exec("CREATE TRIGGER fail_pause_resume BEFORE UPDATE ON rooms BEGIN SELECT RAISE(ABORT,'resume rollback'); END"); err != nil {
		t.Fatal(err)
	}
	resume := map[string]any{"type": "input", "roomId": f.id, "version": paused["version"], "nonce": "durable-resume", "action": map[string]any{"type": "resume"}}
	streamSend(t, c, resume)
	rejected := streamUntil(t, c, "ack")
	if rejected["status"] != float64(500) || !reflect.DeepEqual(roomWithoutNetwork(t, paused), roomWithoutNetwork(t, current(f.a))) || !pauseStoredRoom(t, f).Module.Paused {
		t.Fatal("failed resume leaked", rejected)
	}
	if _, err := f.s.db.Exec("DROP TRIGGER fail_pause_resume"); err != nil {
		t.Fatal(err)
	}
	streamSend(t, c, resume)
	resumed := streamUntil(t, c, "ack")
	if resumed["status"] != float64(200) || resumed["durable"] != true || pauseStoredRoom(t, f).Module.Paused {
		t.Fatal("resume ACK not durable", resumed)
	}
	pauseRealtimeProof(t, "realtime-input-durability", map[string]any{"test": t.Name(), "passed": !t.Failed(), "pause": ack, "repeat": repeated, "rejected": rejected, "resume": resumed, "stored": pauseStoredRoom(t, f)})
}
