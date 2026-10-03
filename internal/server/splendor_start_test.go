package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/k0ngk0ng/wire-board/internal/game"
)

func TestSplendorNonHostOpeningAndRematch(t *testing.T) {
	s, ts := setupServer(t)
	// Choose a non-host deterministically here; the engine tests random selection.
	opening := 1
	s.newGame = func(kind string, n int) (*game.State, error) {
		g, err := game.New(kind, n)
		if err == nil {
			g.Turn, g.Splendor.StartPlayer = opening, opening
		}
		return g, err
	}
	a, b := newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("先手房主")
	b.register("先手朋友")
	r := startRoom(t, a, b, "splendor")
	view := r["game"].(map[string]any)
	if view["turn"] != float64(1) || view["splendor"].(map[string]any)["startPlayer"] != float64(1) {
		t.Fatal("host replaced selected starter")
	}
	a.command(r, "action", game.Action{Type: "reserve", Tier: 1}, 400)
	b.command(r, "action", game.Action{Type: "reserve", Tier: 1}, 200)
	v := current(a)["game"].(map[string]any)["splendor"].(map[string]any)
	event := v["cardEvents"].([]any)[0].(map[string]any)
	if event["player"] != float64(1) || event["source"] != "deck" {
		t.Fatal("wrong animation recipient", event)
	}
	if _, exists := event["card"]; exists {
		t.Fatal("hidden card leaked over HTTP")
	}
	a.command(current(a), "close", nil, 200)
	a.command(current(a), "rematch", nil, 200)
	opening = 0
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	r = current(a)
	view = r["game"].(map[string]any)
	if view["turn"] != float64(0) || view["splendor"].(map[string]any)["startPlayer"] != float64(0) {
		t.Fatal("rematch reused previous starter")
	}
	before, _ := json.Marshal(r)
	ts.Close()
	s.Close()
	next, err := New(s.cfg, s.files)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	ts2 := httptest.NewServer(next.Handler())
	defer ts2.Close()
	a.base = ts2.URL
	after, _ := json.Marshal(current(a))
	if string(before) != string(after) {
		t.Fatal("restart rerolled starter or changed game")
	}
}
