package server

import (
	"net/http/httptest"
	"testing"
)

func TestSpectatorPrivacyMembershipAndPersistence(t *testing.T) {
	for _, kind := range []string{"splendor", "rail"} {
		t.Run(kind, func(t *testing.T) {
			s, ts := setupServer(t)
			a, b, v := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
			a.register("Alice")
			b.register("Bobby")
			v.register("Viewer")
			room := startRoom(t, a, b, kind)
			path := "/api/rooms/" + room["id"].(string) + "/watch"
			a.post(path, map[string]any{}, 409)
			newClient(t, ts.URL).post(path, map[string]any{}, 401)
			v.post(path, map[string]any{}, 200)
			v.post(path, map[string]any{}, 200)
			view := current(v)
			if view["spectating"] != true || view["you"] != float64(-1) || view["spectatorCount"] != float64(1) || len(view["seats"].([]any)) != 2 {
				t.Fatal("spectator occupied a seat or duplicated", view)
			}
			if current(a)["version"] != room["version"] || current(a)["turnDeadline"] != room["turnDeadline"] {
				t.Fatal("spectating interrupted the turn")
			}
			for _, cmd := range []string{"ready", "start", "close", "rematch", "kick_timeout", "leave", "add_bot"} {
				v.command(view, cmd, nil, 400)
			}
			v.command(view, "action", map[string]any{"type": "reserve", "tier": 1}, 400)
			v.post("/api/rooms", map[string]any{"name": "Other", "kind": kind, "capacity": 2}, 409)
			chat := v.post("/api/rooms/"+room["id"].(string)+"/chat", map[string]string{"text": "观战你好", "nonce": "watch-chat-test"}, 201)
			if chat["spectator"] != true {
				t.Fatal("observer message not marked")
			}
			if kind == "splendor" {
				a.command(room, "action", map[string]any{"type": "reserve", "tier": 1}, 200)
			}
			gameView := current(v)["game"].(map[string]any)
			if kind == "splendor" {
				g := gameView["splendor"].(map[string]any)
				if _, ok := g["decks"]; ok {
					t.Fatal("deck exposed")
				}
				for _, p := range g["players"].([]any) {
					if _, ok := p.(map[string]any)["reserved"]; ok {
						t.Fatal("reserved cards exposed")
					}
				}
				if g["players"].([]any)[0].(map[string]any)["reservedCount"] != float64(1) {
					t.Fatal("public count missing")
				}
			} else {
				g := gameView["rail"].(map[string]any)
				for _, key := range []string{"deck", "ticketDeck", "pending", "setupPending", "discard"} {
					if _, ok := g[key]; ok {
						t.Fatal("private railway data exposed", key)
					}
				}
				for _, p := range g["players"].([]any) {
					for _, key := range []string{"hand", "tickets"} {
						if _, ok := p.(map[string]any)[key]; ok {
							t.Fatal("private player data exposed", key)
						}
					}
				}
			}
			ts.Close()
			s.Close()
			next, err := New(s.cfg, s.files)
			if err != nil {
				t.Fatal(err)
			}
			defer next.Close()
			restart := httptest.NewServer(next.Handler())
			defer restart.Close()
			a.base = restart.URL
			b.base = restart.URL
			v.base = restart.URL
			if current(v)["spectating"] != true {
				t.Fatal("watch membership lost on restart")
			}
			a.command(current(a), "close", nil, 200)
			a.command(current(a), "rematch", nil, 200)
			if current(v)["status"] != "waiting" || len(current(a)["seats"].([]any)) != 2 {
				t.Fatal("rematch converted spectator to player")
			}
			v.post(path, map[string]any{"leave": true}, 200)
			if _, ok := v.state()["room"]; ok {
				t.Fatal("left spectator still watching")
			}
			v.post("/api/rooms/"+room["id"].(string)+"/chat", map[string]string{"text": "after leaving", "nonce": "watch-after-leave"}, 403)
		})
	}
}

func TestPasswordProtectedSpectatingAndRoomDeletion(t *testing.T) {
	_, ts := setupServer(t)
	a, b, v := newClient(t, ts.URL), newClient(t, ts.URL), newClient(t, ts.URL)
	a.register("Alice")
	b.register("Bobby")
	v.register("Viewer")
	r := a.post("/api/rooms", map[string]any{"name": "Private", "kind": "splendor", "capacity": 2, "password": "room-secret"}, 201)
	path := "/api/rooms/" + r["id"].(string)
	v.post(path+"/watch", map[string]any{"password": "room-secret"}, 400)
	b.post(path, map[string]any{"type": "join", "version": r["version"], "nonce": randomID(12), "password": "room-secret"}, 200)
	a.command(current(a), "ready", nil, 200)
	b.command(current(b), "ready", nil, 200)
	a.command(current(a), "start", nil, 200)
	v.post(path+"/watch", map[string]any{}, 403)
	v.post(path+"/watch", map[string]any{"password": "wrong"}, 403)
	v.post(path+"/watch", map[string]any{"password": "room-secret"}, 200)
	a.command(current(a), "close", nil, 200)
	a.command(current(a), "leave", nil, 200)
	b.command(current(b), "leave", nil, 200)
	if _, ok := v.state()["room"]; ok {
		t.Fatal("deleted room retained spectator")
	}
}
