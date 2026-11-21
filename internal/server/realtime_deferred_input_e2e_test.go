package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func deferredInputFixture(calls *atomic.Int64) gameapi.Module {
	module := realtimeFixture(true, time.Hour, 0, "")
	module.RealtimeDeadlineFrames = true
	module.SyncSnapshot = module.View
	module.AcceptStale = func(gameapi.Context, gameapi.Match, json.RawMessage) bool { return true }
	module.RealtimeDeferredInput = func(raw json.RawMessage) bool {
		var action struct {
			Type string
		}
		return json.Unmarshal(raw, &action) == nil && action.Type == "queued"
	}
	apply := module.Apply
	module.Apply = func(ctx gameapi.Context, match gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
		next, err := apply(ctx, match, raw)
		if err != nil {
			return next, err
		}
		var action struct {
			Type       string
			Transition string
		}
		if err := json.Unmarshal(raw, &action); err != nil {
			return next, err
		}
		if action.Type == "cancel" {
			var position realtimePosition
			_ = json.Unmarshal(next.Data, &position)
			position.Drive[ctx.Actor] = 0
			next.Data, _ = json.Marshal(position)
		}
		switch action.Transition {
		case "checkpoint":
			next.CheckpointKey = "critical-checkpoint"
		case "actors":
			next.Actors = []int{0}
		case "attention":
			next.Attention = []int{0}
		case "deadline":
			next.SystemDeadline--
		case "pause":
			next.Paused, next.SystemDeadline = true, 0
			next.Attention = []int{}
		case "result":
			next.SystemDeadline, next.Deadline = 0, 0
			next.Actors = nil
			next.Result = &gameapi.Result{Ranks: []int{1, 2, 3, 4}, Winners: []int{0}}
		}
		return next, nil
	}
	module.StreamView = func(match gameapi.Match, viewer int, previous string) (gameapi.StreamViewResult, error) {
		calls.Add(1)
		var position realtimePosition
		if err := json.Unmarshal(match.Data, &position); err != nil {
			return gameapi.StreamViewResult{}, err
		}
		return gameapi.StreamViewResult{Data: map[string]any{"frame": position.Frames, "inputs": position.Inputs, "viewer": viewer, "previous": previous}, Cursor: fmt.Sprintf("%d:%d", position.Frames, position.Inputs[0])}, nil
	}
	return module
}

func TestRealtimeDeferredInputAcknowledgesWithoutWakingConnections(t *testing.T) {
	var calls atomic.Int64
	fixture := newUnchangedTable(t, deferredInputFixture(&calls))
	connections := make([]*websocket.Conn, 10)
	packets := make([]<-chan unchangedPacket, 10)
	stats := make([]*unchangedStats, 10)
	for index := range connections {
		connections[index] = streamDial(t, fixture.a, true)
		packets[index], stats[index] = unchangedReader(t, connections[index]), &unchangedStats{}
		unchangedKind(t, packets[index], stats[index], "state")
	}
	version := current(fixture.a)["version"]
	streamSend(t, connections[0], map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": "deferred-bootstrap", "action": map[string]any{"type": "buy", "drive": 1}})
	ack, _ := unchangedAckAndFrame(t, packets[0], stats[0], "deferred-bootstrap", 0)
	version = ack["version"]
	for index := 1; index < len(connections); index++ {
		unchangedKind(t, packets[index], stats[index], "game")
	}
	before := calls.Load()
	searchContext, cancel := context.WithCancel(context.Background())
	defer cancel()
	fixture.s.mu.Lock()
	room := fixture.s.rooms[fixture.id]
	fixture.s.botSearches[fixture.id] = &moduleBotSearch{room: fixture.id, match: room.MatchID, kind: room.Kind, version: room.Version, actor: 1, seat: room.Seats[1].ID, ctx: searchContext, cancel: cancel}
	fixture.s.mu.Unlock()
	for index := range 100 {
		nonce := "deferred-queued-" + strconv.Itoa(index)
		streamSend(t, connections[0], map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": nonce, "action": map[string]any{"type": "queued", "drive": 1}})
		ack := unchangedKind(t, packets[0], stats[0], "ack")
		if ack["nonce"] != nonce || unchangedInteger(t, ack["status"]) != 200 {
			t.Fatal("queued input lost its immediate ACK", ack)
		}
		version = ack["version"]
	}
	streamSend(t, connections[0], map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": "deferred-queued-0", "action": map[string]any{"type": "queued", "drive": 1}})
	retry := unchangedKind(t, packets[0], stats[0], "ack")
	if unchangedInteger(t, retry["status"]) != 200 || retry["nonce"] != "deferred-queued-0" {
		t.Fatal("deferred input retry lost its receipt", retry)
	}
	time.Sleep(30 * time.Millisecond)
	if calls.Load() != before || stats[0].Games != 1 {
		t.Fatal("ordinary queued input still projected to all connections", before, calls.Load(), stats[0])
	}
	if searchContext.Err() == nil {
		t.Fatal("suppressed broadcast left a stale bot search alive")
	}
	fixture.s.mu.Lock()
	deadline := fixture.s.rooms[fixture.id].Module.SystemDeadline
	fixture.s.mu.Unlock()
	fixture.frame(time.UnixMilli(deadline))
	for index := range connections {
		game := unchangedKind(t, packets[index], stats[index], "game")
		data := game["game"].(map[string]any)["data"].(map[string]any)
		if unchangedInteger(t, data["frame"]) != 1 || unchangedInteger(t, data["inputs"].([]any)[0]) != 101 {
			t.Fatal("next rule frame omitted accepted queued inputs", data)
		}
	}
	fresh := streamDial(t, fixture.a, true)
	freshPackets, freshStats := unchangedReader(t, fresh), &unchangedStats{}
	state := unchangedKind(t, freshPackets, freshStats, "state")
	data := state["state"].(map[string]any)["room"].(map[string]any)["game"].(map[string]any)["data"].(map[string]any)
	if unchangedInteger(t, data["inputs"].([]any)[0]) != 101 {
		t.Fatal("reconnect lost queued input state", data)
	}
	fixture.a.post("/api/rooms/"+fixture.id, map[string]any{"type": "autoplay", "version": current(fixture.a)["version"], "nonce": "deferred-autoplay", "enabled": true}, 200)
	changed := unchangedKind(t, packets[0], stats[0], "state")
	roomState := changed["state"].(map[string]any)["room"].(map[string]any)
	if roomState["seats"].([]any)[0].(map[string]any)["autoPlay"] != true {
		t.Fatal("deferred input capability swallowed autoplay notification", roomState)
	}
	streamSend(t, connections[0], map[string]any{"type": "input", "roomId": fixture.id, "version": roomState["version"], "nonce": "deferred-no-autoplay-control", "action": map[string]any{"type": "queued", "drive": 1}})
	denied := unchangedKind(t, packets[0], stats[0], "ack")
	if unchangedInteger(t, denied["status"]) == 200 {
		t.Fatal("deferred input bypassed current autoplay permission", denied)
	}
	unchangedEvidence(t, "deferred-input-fanout", map[string]any{"passed": true, "connections": 10, "acknowledgedQueuedInputs": 100, "projectionsBeforeRuleFrame": calls.Load() - before - 10, "nextFrameInputs": 101, "staleSearchCancelled": true, "reconnect": true, "idempotentRetry": true, "autoplayState": true, "permissionPreserved": true})
}

func TestRealtimeDeferredInputMixedAndTransitionsStillBroadcast(t *testing.T) {
	for _, mode := range []string{"buy", "cancel", "checkpoint", "actors", "attention", "deadline", "pause", "result", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int64
			module := deferredInputFixture(&calls)
			if mode == "legacy" {
				module.RealtimeDeferredInput = nil
			}
			fixture := newUnchangedTable(t, module)
			fixture.s.inputBatchDelay = 100 * time.Millisecond
			first, second := streamDial(t, fixture.a, true), streamDial(t, fixture.a, true)
			a, b := unchangedReader(t, first), unchangedReader(t, second)
			aStats, bStats := &unchangedStats{}, &unchangedStats{}
			unchangedKind(t, a, aStats, "state")
			unchangedKind(t, b, bStats, "state")
			version := current(fixture.a)["version"]
			streamSend(t, first, map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": "mixed-queued", "action": map[string]any{"type": "queued", "drive": 1}})
			admitted := false
			for until := time.Now().Add(time.Second); time.Now().Before(until); {
				fixture.s.inputMu.Lock()
				queue := fixture.s.inputQueues[fixture.id]
				admitted = queue != nil && len(queue.pending) == 1 && queue.pending[0].input.Nonce == "mixed-queued"
				fixture.s.inputMu.Unlock()
				if admitted {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if !admitted {
				t.Fatal("first socket input was not admitted before the mixed transition")
			}
			kind, transition := "queued", mode
			if mode == "buy" || mode == "cancel" {
				kind, transition = mode, ""
			}
			streamSend(t, second, map[string]any{"type": "input", "roomId": fixture.id, "version": version, "nonce": "mixed-immediate", "action": map[string]any{"type": kind, "transition": transition, "drive": 1}})
			ackA := unchangedKind(t, a, aStats, "ack")
			ackB := unchangedKind(t, b, bStats, "ack")
			if unchangedInteger(t, ackA["status"]) != 200 || unchangedInteger(t, ackB["status"]) != 200 {
				t.Fatal("mixed batch input rejected", ackA, ackB)
			}
			for _, pair := range []struct {
				packets <-chan unchangedPacket
				stats   *unchangedStats
			}{{a, aStats}, {b, bStats}} {
				if pair.stats.Games == 0 && pair.stats.State == 1 {
					expected := "game"
					if mode == "result" {
						expected = "state"
					}
					unchangedKind(t, pair.packets, pair.stats, expected)
				}
			}
			if mode == "cancel" && realtimeData(t, current(fixture.a)).Drive[0] != 0 {
				t.Fatal("cancel did not clear held input before the next rule frame")
			}
			if mode == "checkpoint" || mode == "pause" || mode == "result" {
				if ackA["durable"] != true || ackB["durable"] != true {
					t.Fatal("critical batch lost durable ACK semantics", ackA, ackB)
				}
			}
		})
	}
	unchangedEvidence(t, "deferred-input-transitions", map[string]any{"passed": true, "mixed": []string{"buy", "cancel"}, "forced": []string{"checkpoint", "actors", "attention", "deadline", "pause", "result"}, "legacyCompatible": true})
}
