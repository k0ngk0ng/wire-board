package stoneage_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/stoneage/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

// Only full-match HTTP matrices accelerate their clock. Production rules,
// actions, encoded native animation lengths, SQLite and the host scheduler are
// unchanged. The closure survives HTTP restarts through Config.Modules.
// Browser fixtures never use this adapter.
func acceleratedHTTPModule() gameapi.Module {
	module := game.Module()
	origin := time.Now().UTC()
	base := origin.UnixMilli()
	context := func(c gameapi.Context) gameapi.Context { c.Now = origin.Add(c.Now.Sub(origin) * 4); return c }
	incoming := func(m gameapi.Match) gameapi.Match {
		convert := func(deadline int64) int64 {
			if deadline == 0 {
				return 0
			}
			return base + (deadline-base)*4
		}
		m.Deadline = convert(m.Deadline)
		m.SystemDeadline = convert(m.SystemDeadline)
		return m
	}
	envelope := func(m gameapi.Match) gameapi.Match {
		convert := func(deadline int64) int64 {
			if deadline == 0 {
				return 0
			}
			return base + (deadline-base+3)/4
		}
		m.Deadline = convert(m.Deadline)
		m.SystemDeadline = convert(m.SystemDeadline)
		return m
	}
	start, apply, bot := module.Start, module.Apply, module.BotAction
	stale := module.AcceptStale
	module.AcceptStale = func(c gameapi.Context, m gameapi.Match, a json.RawMessage) bool {
		return stale(context(c), incoming(m), a)
	}
	module.Start = func(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
		m, e := start(context(c), raw)
		return envelope(m), e
	}
	module.Apply = func(c gameapi.Context, m gameapi.Match, a json.RawMessage) (gameapi.Match, error) {
		next, e := apply(context(c), incoming(m), a)
		return envelope(next), e
	}
	module.BotAction = func(c gameapi.Context, m gameapi.Match) (json.RawMessage, error) { return bot(context(c), incoming(m)) }
	wrap := func(fn func(gameapi.Context, gameapi.Match) (gameapi.Match, error)) func(gameapi.Context, gameapi.Match) (gameapi.Match, error) {
		return func(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
			next, e := fn(context(c), incoming(m))
			return envelope(next), e
		}
	}
	module.OnTimer = wrap(module.OnTimer)
	module.Reclaim = wrap(module.Reclaim)
	module.Restore = wrap(module.Restore)
	module.PreciseTimers = true
	return module
}

func nativeManualDecision(t *testing.T, client *gametest.Client) {
	t.Helper()
	until := time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		room := client.Room()
		var view game.View
		if err := json.Unmarshal(raw(gametest.Data(room)), &view); err != nil {
			t.Fatal(err)
		}
		action := game.Suggest(view)
		if action == nil {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		payload, _ := json.Marshal(map[string]any{"type": "action", "version": room["version"], "nonce": fmt.Sprintf("stoneage-manual-%d", time.Now().UnixNano()), "action": action})
		request, err := http.NewRequest("POST", client.Base+"/api/rooms/"+room["id"].(string), bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.HTTP.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		err = json.NewDecoder(response.Body).Decode(&result)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode == 200 {
			return
		}
		if response.StatusCode != 409 || result["error"] != "局面已更新，请根据最新画面重试" {
			t.Fatal("manual decision failed", response.StatusCode, result)
		}
	}
	t.Fatal("no manual decision window after autoplay cancellation")
}

// Bots may commit between the read and a room-level autoplay command. Retry
// only the explicit stale-version response, keeping one idempotency nonce.
func nativeHTTPCommand(t *testing.T, client *gametest.Client, kind string, extra map[string]any) {
	t.Helper()
	nonce := fmt.Sprintf("stoneage-native-%d-%d", client.Index, time.Now().UnixNano())
	for attempt := 0; attempt < 16; attempt++ {
		room := client.Room()
		body := map[string]any{"type": kind, "version": room["version"], "nonce": nonce}
		for k, v := range extra {
			body[k] = v
		}
		payload, _ := json.Marshal(body)
		request, err := http.NewRequest("POST", client.Base+"/api/rooms/"+room["id"].(string), bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.HTTP.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		err = json.NewDecoder(response.Body).Decode(&result)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode == 200 {
			return
		}
		if response.StatusCode != 409 || result["error"] != "局面已更新，请根据最新画面重试" {
			t.Fatal("room command failed", kind, response.StatusCode, result)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("room command kept conflicting", kind)
}
