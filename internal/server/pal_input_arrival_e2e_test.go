package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	pal "github.com/k0ngk0ng/wire-board/games/pal/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func TestPALOriginalMenuInputTimeThroughHTTPAndWebSocket(t *testing.T) {
	if os.Getenv("PAL_INPUT_ARRIVAL") != "1" {
		t.Skip("explicit original menu and authenticated transport regression")
	}
	base, build := palPerformanceModule(t)
	module := palPerformanceMenuEntry(palFieldCheckpointModule(base))
	type observation struct{ arrived, applied int64 }
	observations := make(chan observation, 32)
	apply := module.Apply
	module.Apply = func(c gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		var action pal.Action
		if json.Unmarshal(raw, &action) == nil && action.Type == "input" {
			observations <- observation{c.InputReceivedAt.UnixMilli(), c.Now.UnixMilli()}
		}
		return apply(c, match, raw)
	}
	s, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{module}}, fstest.MapFS{"index.html": {Data: []byte("original menu transport regression")}})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(func() { ts.Close(); s.Close() })
	client := newClient(t, ts.URL)
	client.client.Timeout = 75 * time.Second
	client.register("原版菜单输入到达")
	id := client.post("/api/rooms", map[string]any{"kind": "pal", "name": "原版菜单队列验收", "capacity": 1}, 201)["id"].(string)
	client.command(current(client), "ready", nil, 200)
	client.command(current(client), "start", nil, 200)
	conn := streamDial(t, client, true)
	streamUntil(t, conn, "state")
	decodeView := func(room map[string]any) pal.View {
		t.Helper()
		raw, err := json.Marshal(room["game"].(map[string]any)["data"])
		if err != nil {
			t.Fatal(err)
		}
		var view pal.View
		if err := json.Unmarshal(raw, &view); err != nil || view.Scene != 2 || view.Phase != "menu" || view.Menu.Kind != 1 || len(view.Menu.Items) != 4 {
			t.Fatal("expected the actual original inn menu", err, view.Phase, view.Menu, view.Cash)
		}
		return view
	}
	initial := decodeView(current(client))
	if initial.Menu.Selected != 1 {
		t.Fatal("unexpected original initial selection", initial.Menu.Selected)
	}
	type trial struct {
		name     string
		http     bool
		delay    time.Duration
		lock     time.Duration
		age      time.Duration
		discard  bool
		key      uint32
		selected int
	}
	trials := []trial{
		{"ws-live-before-host-wait", false, 360 * time.Millisecond, 0, 250 * time.Millisecond, false, 4, 2},
		{"http-live-before-host-wait", true, 360 * time.Millisecond, 0, 250 * time.Millisecond, false, 16, 1},
		{"ws-arrival-before-host-lock", false, 0, 350 * time.Millisecond, 260 * time.Millisecond, false, 4, 2},
		{"expired-before-arrival", false, 0, 0, 650 * time.Millisecond, true, 16, 2},
		{"expired-in-host-queue", false, 600 * time.Millisecond, 0, 20 * time.Millisecond, true, 16, 2},
		{"future-input", false, 360 * time.Millisecond, 0, -250 * time.Millisecond, true, 16, 2},
		{"old-neutral-release", false, 0, 0, 2 * time.Second, false, 0, 2},
		{"fresh-after-discard", false, 0, 0, 50 * time.Millisecond, false, 16, 1},
	}
	rows := []map[string]any{}
	for index, test := range trials {
		room := current(client)
		view := decodeView(room)
		if view.Epoch != initial.Epoch {
			t.Fatal("menu selection changed the native control generation")
		}
		s.inputMu.Lock()
		s.inputBatchDelay = test.delay
		s.inputMu.Unlock()
		started := time.Now()
		controlAt := started.Add(-test.age).UnixMilli()
		seq := uint64(index + 1)
		nonce := "pal-arrival-" + test.name
		action := pal.Action{Type: "input", Epoch: view.Epoch, Seq: seq, ControlAt: controlAt, Direction: 4, Keys: test.key}
		if test.http {
			client.post("/api/rooms/"+id, map[string]any{"type": "action", "version": room["version"], "nonce": nonce, "action": action}, 200)
		} else {
			send := func() {
				streamSend(t, conn, map[string]any{"type": "input", "roomId": id, "version": room["version"], "nonce": nonce, "action": action})
			}
			if test.lock > 0 {
				func() {
					s.mu.Lock()
					defer s.mu.Unlock()
					send()
					time.Sleep(test.lock)
				}()
			} else {
				send()
			}
			ack := streamUntil(t, conn, "ack")
			if ack["status"] != float64(200) || ack["nonce"] != nonce {
				t.Fatal(test.name, ack)
			}
		}
		observed := <-observations
		now := observed.applied
		if now < started.Add(test.delay+test.lock-10*time.Millisecond).UnixMilli() || observed.arrived < started.UnixMilli() || observed.arrived > now {
			t.Fatal("missing arrival clock or rule execution time was backdated", test.name, observed, started)
		}
		if !test.discard && test.delay+test.lock > 0 && (now-controlAt <= 500 || observed.arrived-controlAt > 500 || now-observed.arrived < (test.delay+test.lock).Milliseconds()-100) {
			t.Fatal("fresh input did not cross the old commit-time budget", test.name, observed, controlAt)
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			view = decodeView(current(client))
			if view.Seq == seq && (view.DiscardedSeq == seq || view.AppliedSeq == seq) {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("native menu input did not retire", test.name, seq, view.Seq, view.AppliedSeq)
			}
			time.Sleep(30 * time.Millisecond)
		}
		if view.Menu.Selected != test.selected || (view.DiscardedSeq == seq) != test.discard || view.Epoch != initial.Epoch || view.Cash != initial.Cash {
			t.Fatalf("%s: native selection=%d discarded=%d applied=%d commit=%d source=%d", test.name, view.Menu.Selected, view.DiscardedSeq, view.AppliedSeq, now, controlAt)
		}
		rows = append(rows, map[string]any{"case": test.name, "seq": seq, "discardedSeq": view.DiscardedSeq, "appliedSeq": view.AppliedSeq, "selected": view.Menu.Selected, "selectionBefore": room["game"].(map[string]any)["data"].(map[string]any)["menu"].(map[string]any)["selected"], "sentWallMS": started.UnixMilli(), "sourceMS": controlAt, "receivedMS": observed.arrived, "ruleTimeMS": now, "configuredBatchMS": test.delay.Milliseconds(), "hostLockMS": test.lock.Milliseconds()})
	}
	room := current(client)
	queued := decodeView(room)
	s.inputMu.Lock()
	s.inputBatchDelay = 450 * time.Millisecond
	s.inputMu.Unlock()
	nonce := "pal-arrival-pre-save"
	streamSend(t, conn, map[string]any{"type": "input", "roomId": id, "version": room["version"], "nonce": nonce, "action": pal.Action{Type: "input", Epoch: queued.Epoch, Seq: queued.Seq + 1, ControlAt: queued.ControlAt, Direction: 4, Keys: 4}})
	for deadline := time.Now().Add(250 * time.Millisecond); ; {
		s.inputMu.Lock()
		job := s.inputQueues[id]
		waiting := job != nil && len(job.pending) == 1 && job.pending[0].input.Nonce == nonce
		s.inputMu.Unlock()
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("original input did not reach the host queue before save")
		}
		time.Sleep(2 * time.Millisecond)
	}
	client.command(current(client), "save", nil, 200)
	if client.state()["room"] != nil {
		t.Fatal("original save did not free the occupied seat")
	}
	client.post("/api/saves/"+id+"/resume", map[string]any{}, 200)
	ack := streamUntil(t, conn, "ack")
	resumed := decodeView(current(client))
	if ack["nonce"] != nonce || ack["status"] == float64(200) || resumed.Epoch == queued.Epoch || resumed.Menu.Selected != queued.Menu.Selected || resumed.Seq != 0 {
		t.Fatal("queued input escaped its original save generation", ack, resumed.Menu.Selected, resumed.Seq)
	}
	rows = append(rows, map[string]any{"case": "waiting-input-cannot-enter-resumed-menu", "ackStatus": ack["status"], "oldEpoch": queued.Epoch, "resumedEpoch": resumed.Epoch, "selectionBefore": queued.Menu.Selected, "selected": resumed.Menu.Selected})
	report, err := json.MarshalIndent(map[string]any{"passed": true, "cases": rows, "buildID": build, "scope": "actual original solo inn menu, real HTTP/WebSocket, bounded server batch waits and current rule execution time"}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	evidence := "menu-input-arrival-http.json"
	if os.Getenv("PAL_INPUT_ARRIVAL_RACE") == "1" {
		evidence = "menu-input-arrival-race-http.json"
	}
	if err = os.WriteFile(filepath.Join("..", "..", ".local/pal/verification", evidence), append(report, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
