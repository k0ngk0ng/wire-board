package spf2t_test

// Real HTTP/SQLite acceptance for the production spf2t module. The failure
// inventory is ../CONTRACT.md: unpublished creation, select/input privacy,
// nonce replay, concurrent seats, restart preservation and autoplay reclaim.

import (
	"sync"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/spf2t/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func spf2tData(t *testing.T, c *gametest.Client) map[string]any {
	t.Helper()
	return gametest.Data(c.Room())
}

func spf2tPlayer(t *testing.T, data map[string]any, seat int) map[string]any {
	t.Helper()
	players, ok := data["players"].([]any)
	if !ok || seat < 0 || seat >= len(players) {
		t.Fatalf("missing player %d in %v", seat, data)
	}
	return players[seat].(map[string]any)
}

func spf2tCharacters(t *testing.T, data map[string]any) []string {
	t.Helper()
	list, ok := data["characters"].([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("missing characters in %v", data)
	}
	out := make([]string, 0, len(list))
	for _, raw := range list {
		out = append(out, raw.(map[string]any)["id"].(string))
	}
	return out
}

func waitSPF2TPlaying(t *testing.T, c *gametest.Client) map[string]any {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data := spf2tData(t, c)
		if data["phase"] == "playing" {
			return data
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("spf2t did not leave countdown")
	return nil
}

func spf2tBatch(epoch string, seq int, kinds ...string) map[string]any {
	events := make([]map[string]any, 0, len(kinds))
	for _, kind := range kinds {
		events = append(events, map[string]any{"kind": kind})
	}
	return map[string]any{"type": "inputs", "epoch": epoch, "seq": seq, "events": events}
}

func seatAutoPlay(room map[string]any, seat int) bool {
	seats, _ := room["seats"].([]any)
	if seat < 0 || seat >= len(seats) {
		return false
	}
	value, _ := seats[seat].(map[string]any)["autoPlay"].(bool)
	return value
}

func TestSPF2THTTPSpectatorPrivacyAndIdempotence(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "spf2t")
	h.AssertUnpublished()
	h.Start(map[string]any{"format": game.FormatBestOf3})
	selectSPF2T(t, h)
	data := waitSPF2TPlaying(t, h.People[0])

	spectator := spf2tData(t, h.Spectator)
	for i := 0; i < 2; i++ {
		player := spf2tPlayer(t, spectator, i)
		if _, ok := player["seq"]; ok {
			t.Fatalf("spectator saw seat %d seq", i)
		}
	}
	for seat := 0; seat < 2; seat++ {
		data := spf2tData(t, h.People[seat])
		other := spf2tPlayer(t, data, 1-seat)
		if _, ok := other["seq"]; ok {
			t.Fatalf("seat %d saw opponent seq", seat)
		}
		own := spf2tPlayer(t, data, seat)
		if _, ok := own["gems"]; !ok {
			t.Fatalf("seat %d missing public gems", seat)
		}
	}

	room := h.People[0].Room()
	id := room["id"].(string)
	epoch := data["epoch"].(string)
	req := h.People[0].ActionRequest(spf2tBatch(epoch, 1, "hardDrop"))
	before := spf2tPlayer(t, spf2tData(t, h.People[0]), 0)
	beforeGems := len(before["gems"].([]any))
	h.People[0].Request("POST", "/api/rooms/"+id, req, 200)
	after := spf2tPlayer(t, spf2tData(t, h.People[0]), 0)
	afterGems := len(after["gems"].([]any))
	if afterGems <= beforeGems {
		t.Fatalf("hard drop did not stamp gems: %d -> %d", beforeGems, afterGems)
	}
	h.People[0].Request("POST", "/api/rooms/"+id, req, 200)
	replayed := spf2tPlayer(t, spf2tData(t, h.People[0]), 0)
	if len(replayed["gems"].([]any)) != afterGems {
		t.Fatalf("nonce replay changed gems: %d -> %d", afterGems, len(replayed["gems"].([]any)))
	}
	h.People[0].Action(spf2tBatch(epoch, 1, "hardDrop"), 400)
	h.People[0].Action(spf2tBatch(epoch, 1<<20, "hardDrop"), 400)
	h.People[0].Action(spf2tBatch(epoch, 2, "teleport"), 400)
	h.Record("privacy, nonce replay and sequence bounds")
}

func selectSPF2T(t *testing.T, h *gametest.HTTP) {
	t.Helper()
	data := spf2tData(t, h.People[0])
	epoch := data["epoch"].(string)
	chars := spf2tCharacters(t, data)
	for i, c := range h.People {
		c.Action(map[string]any{"type": "select", "epoch": epoch, "character": chars[i]}, 200)
	}
}

func TestSPF2THTTPConcurrentActions(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "spf2t")
	h.Start(map[string]any{"format": game.FormatSingle})
	selectSPF2T(t, h)
	data := waitSPF2TPlaying(t, h.People[0])
	epoch := data["epoch"].(string)
	var wg sync.WaitGroup
	for _, client := range h.People {
		wg.Add(1)
		go func(c *gametest.Client) {
			defer wg.Done()
			c.Action(spf2tBatch(epoch, 1, "left", "rotateCW", "hardDrop"), 200)
		}(client)
	}
	wg.Wait()
	for seat := 0; seat < 2; seat++ {
		player := spf2tPlayer(t, spf2tData(t, h.People[seat]), seat)
		if player["lost"] == true {
			t.Fatalf("seat %d unexpectedly lost", seat)
		}
		if len(player["gems"].([]any)) == 0 {
			t.Fatalf("concurrent action did not reach seat %d: %v", seat, player)
		}
	}
	h.Record("concurrent seat actions")
}

func TestSPF2THTTPRestartAndAutoPlay(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "spf2t")
	h.Start(map[string]any{"format": game.FormatBestOf3})
	selectSPF2T(t, h)
	data := waitSPF2TPlaying(t, h.People[0])
	epoch := data["epoch"].(string)
	h.People[0].Action(spf2tBatch(epoch, 1, "hardDrop"), 200)
	before := spf2tData(t, h.People[0])
	seq := int(spf2tPlayer(t, before, 0)["seq"].(float64))
	h.Restart()
	after := spf2tData(t, h.People[0])
	if after["epoch"] != epoch {
		t.Fatalf("restart changed epoch: %v -> %v", epoch, after["epoch"])
	}
	if got := int(spf2tPlayer(t, after, 0)["seq"].(float64)); got != seq {
		t.Fatalf("restart changed seq: %d -> %d", seq, got)
	}
	autoplaySeat := 1
	h.People[autoplaySeat].Command("autoplay", map[string]any{"enabled": true}, 200)
	if !seatAutoPlay(h.People[autoplaySeat].Room(), autoplaySeat) {
		t.Fatal("autoplay did not start")
	}
	h.People[autoplaySeat].Command("autoplay", map[string]any{"enabled": false}, 200)
	if seatAutoPlay(h.People[autoplaySeat].Room(), autoplaySeat) {
		t.Fatal("autoplay did not stop")
	}
	current := spf2tData(t, h.People[autoplaySeat])
	nextSeq := 1
	if value, ok := spf2tPlayer(t, current, autoplaySeat)["seq"].(float64); ok {
		nextSeq = int(value) + 1
	}
	h.People[autoplaySeat].Action(spf2tBatch(current["epoch"].(string), nextSeq, "hardDrop"), 200)
	h.Record("restart preservation and reclaim")
}

func TestSPF2THTTPOptionsAndUnpublished(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "spf2t")
	h.AssertUnpublished()
	h.People[0].Request("POST", "/api/admin/games/spf2t", map[string]bool{"enabled": true}, 200)
	h.People[0].Request("POST", "/api/rooms", map[string]any{
		"kind": "spf2t", "name": "bad format", "capacity": 2,
		"options": map[string]any{"format": "unknown"},
	}, 400)
	h.People[0].Request("POST", "/api/rooms", map[string]any{
		"kind": "spf2t", "name": "solo", "capacity": 1,
		"options": map[string]any{"format": game.FormatSingle},
	}, 400)
	h.People[0].Request("POST", "/api/admin/games/spf2t", map[string]bool{"enabled": false}, 200)
	h.People[0].Request("POST", "/api/rooms", map[string]any{
		"kind": "spf2t", "name": "off shelf", "capacity": 2,
		"options": map[string]any{"format": game.FormatSingle},
	}, 403)
}
