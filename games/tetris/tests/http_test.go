package tetris_test

// Real HTTP/SQLite acceptance for the production tetris module. Failure
// inventory: unpublished game creation, invalid options, private queue leakage,
// nonce replay dropping a second piece, stale/future sequence acceptance,
// concurrent seats losing a board, restart resetting the clock, and autoplay
// not restoring the returned seat.
import (
	"fmt"
	"sync"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/tetris/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
)

func tetrisData(t *testing.T, c *gametest.Client) map[string]any {
	t.Helper()
	return gametest.Data(c.Room())
}

func tetrisPlayer(t *testing.T, data map[string]any, seat int) map[string]any {
	t.Helper()
	players, ok := data["players"].([]any)
	if !ok || seat < 0 || seat >= len(players) {
		t.Fatalf("missing player %d in %v", seat, data)
	}
	return players[seat].(map[string]any)
}

func waitTetrisPlaying(t *testing.T, c *gametest.Client) map[string]any {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		data := tetrisData(t, c)
		if data["phase"] == "playing" {
			return data
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("tetris did not leave countdown")
	return nil
}

func tetrisBatch(epoch string, seq int, kinds ...string) map[string]any {
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

func TestTetrisHTTPSpectatorPrivacyAndIdempotence(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "tetris")
	h.AssertUnpublished()
	h.Start(map[string]any{"mode": "survival"})
	waitTetrisPlaying(t, h.People[0])

	spectator := tetrisData(t, h.Spectator)
	for i := 0; i < 2; i++ {
		player := tetrisPlayer(t, spectator, i)
		if player["queue"] != nil || player["hold"] != nil || player["seq"] != nil || player["ghostY"] != nil {
			t.Fatalf("spectator leaked private data for seat %d: %v", i, player)
		}
	}
	for seat := 0; seat < 2; seat++ {
		data := tetrisData(t, h.People[seat])
		own := tetrisPlayer(t, data, seat)
		if own["queue"] == nil {
			t.Fatalf("own queue missing for seat %d", seat)
		}
		other := tetrisPlayer(t, data, 1-seat)
		if other["queue"] != nil || other["hold"] != nil || other["seq"] != nil {
			t.Fatalf("seat %d saw opponent private data: %v", seat, other)
		}
	}

	room := h.People[0].Room()
	id := room["id"].(string)
	data := tetrisData(t, h.People[0])
	epoch := data["epoch"].(string)
	action := tetrisBatch(epoch, 1, "hardDrop")
	req := h.People[0].ActionRequest(action)
	before := int(tetrisPlayer(t, data, 0)["score"].(float64))
	h.People[0].Request("POST", "/api/rooms/"+id, req, 200)
	after := int(tetrisPlayer(t, tetrisData(t, h.People[0]), 0)["score"].(float64))
	if after <= before {
		t.Fatalf("hard drop did not score: %d -> %d", before, after)
	}
	h.People[0].Request("POST", "/api/rooms/"+id, req, 200)
	replayed := int(tetrisPlayer(t, tetrisData(t, h.People[0]), 0)["score"].(float64))
	if replayed != after {
		t.Fatalf("nonce replay changed score: %d -> %d", after, replayed)
	}
	h.People[0].Action(tetrisBatch(epoch, 1, "hardDrop"), 400)
	h.People[0].Action(tetrisBatch(epoch, 1<<20, "hardDrop"), 400)
	h.People[0].Action(tetrisBatch(epoch, 2, "teleport"), 400)
	h.Record("privacy, nonce replay and sequence bounds")
}

func TestTetrisHTTPConcurrentActions(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "tetris")
	h.Start(map[string]any{"mode": "survival"})
	waitTetrisPlaying(t, h.People[0])
	var wg sync.WaitGroup
	for seat := 0; seat < 2; seat++ {
		wg.Add(1)
		go func(client *gametest.Client) {
			defer wg.Done()
			data := tetrisData(t, client)
			epoch := data["epoch"].(string)
			client.Action(tetrisBatch(epoch, 1, "left", "rotateCW", "hardDrop"), 200)
		}(h.People[seat])
	}
	wg.Wait()
	for seat := 0; seat < 2; seat++ {
		player := tetrisPlayer(t, tetrisData(t, h.People[seat]), seat)
		if player["alive"] != true {
			t.Fatalf("seat %d unexpectedly eliminated: %v", seat, player)
		}
		lost := false
		for _, row := range player["board"].([]any) {
			if fmt.Sprint(row) != "0000000000" {
				lost = true
			}
		}
		if !lost {
			t.Fatalf("concurrent action did not reach seat %d: %v", seat, player)
		}
	}
	h.Record("concurrent seat actions")
}

func TestTetrisHTTPRestartAndAutoPlay(t *testing.T) {
	h := gametest.NewHTTP(t, 2, game.Module(), "tetris")
	h.Start(map[string]any{"mode": "survival"})
	data := waitTetrisPlaying(t, h.People[0])
	epoch := data["epoch"].(string)
	h.People[0].Action(tetrisBatch(epoch, 1, "hardDrop"), 200)
	before := tetrisData(t, h.People[0])
	score := int(tetrisPlayer(t, before, 0)["score"].(float64))
	h.Restart()
	after := tetrisData(t, h.People[0])
	if after["epoch"] != epoch {
		t.Fatalf("restart changed epoch: %v -> %v", epoch, after["epoch"])
	}
	if got := int(tetrisPlayer(t, after, 0)["score"].(float64)); got != score {
		t.Fatalf("restart changed score: %d -> %d", score, got)
	}
	h.People[0].Command("autoplay", map[string]any{"enabled": true}, 200)
	if !seatAutoPlay(h.People[0].Room(), 0) {
		t.Fatal("autoplay did not start")
	}
	h.People[0].Command("autoplay", map[string]any{"enabled": false}, 200)
	if seatAutoPlay(h.People[0].Room(), 0) {
		t.Fatal("autoplay did not stop")
	}
	current := tetrisData(t, h.People[0])
	nextSeq := int(tetrisPlayer(t, current, 0)["seq"].(float64)) + 1
	h.People[0].Action(tetrisBatch(current["epoch"].(string), nextSeq, "hardDrop"), 200)
	h.Record("restart preservation and reclaim")
}

func TestTetrisHTTPOptionsAndPractiseRoom(t *testing.T) {
	h := gametest.NewHTTP(t, 1, game.Module(), "tetris")
	h.AssertUnpublished()
	h.People[0].Request("POST", "/api/rooms", map[string]any{
		"kind": "tetris", "name": "bad mode", "capacity": 1,
		"options": map[string]any{"mode": "unknown"},
	}, 403)
	admin := h.People[0]
	admin.Request("POST", "/api/admin/games/tetris", map[string]bool{"enabled": true}, 200)
	for _, options := range []map[string]any{
		{"mode": "unknown"},
		{"mode": "survival", "extra": true},
	} {
		admin.Request("POST", "/api/rooms", map[string]any{
			"kind": "tetris", "name": "bad options", "capacity": 1, "options": options,
		}, 400)
	}
	admin.Request("POST", "/api/rooms", map[string]any{
		"kind": "tetris", "name": "single sprint", "capacity": 1,
		"options": map[string]any{"mode": "sprint"},
	}, 201)
	admin.Command("ready", nil, 200)
	admin.Command("start", nil, 200)
	data := waitTetrisPlaying(t, admin)
	if len(data["players"].([]any)) != 1 {
		t.Fatalf("practice room has %d players", len(data["players"].([]any)))
	}
	if data["options"].(map[string]any)["mode"] != "sprint" {
		t.Fatalf("practice options = %v", data["options"])
	}
	h.Record("options validation and one-seat practice room")
}
