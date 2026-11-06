package escape_test

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestEscapeAutoPlayMixedCampaignHTTP(t *testing.T) {
	for _, scene := range []string{"station", "archive"} {
		t.Run(scene, func(t *testing.T) {
			cfg := server.Config{DataDir: t.TempDir(), InviteCode: "escape-complete", AdminUsername: "合作验收", AdminPassword: "escape-complete-only", Modules: []gameapi.Module{module(t)}}
			app, e := server.New(cfg, fstest.MapFS{"index.html": {Data: []byte("escape complete HTTP")}})
			if e != nil {
				t.Fatal(e)
			}
			srv := httptest.NewServer(app.Handler())
			defer func() { srv.Close(); app.Close() }()
			jar, _ := cookiejar.New(nil)
			a := &httpPlayer{t: t, client: &http.Client{Jar: jar, Timeout: 10 * time.Second}, base: srv.URL}
			a.request("POST", "/api/login", map[string]any{"name": "合作验收", "password": "escape-complete-only"}, 200)
			a.request("POST", "/api/rooms", map[string]any{"kind": "escape", "name": "完整合作验收", "capacity": 2, "options": map[string]any{"scenario": scene, "difficulty": "easy", "seed": "http-complete-29"}}, 201)
			a.command("add_bot", nil, 200)
			a.command("ready", nil, 200)
			a.command("start", nil, 200)
			// Start human, exercise an ordinary action, then explicitly enable the same
			// autoplay entry used in production. No injected clocks or terminal state.
			a.action(map[string]any{"type": "inspect", "target": "briefing"}, 200)
			time.Sleep(150 * time.Millisecond)
			a.command("autoplay", map[string]any{"enabled": true}, 200)
			until := time.Now().Add(150 * time.Second)
			var final map[string]any
			for time.Now().Before(until) {
				r := a.room()
				if boolv(obj(r["game"]), "finished") {
					final = r
					break
				}
				time.Sleep(200 * time.Millisecond)
			}
			if final == nil {
				t.Fatal("real server clock did not complete mixed campaign")
			}
			game := obj(final["game"])
			result := obj(final["result"])
			if len(array(game, "winners")) != 2 || !boolv(result, "cooperative") || boolv(result, "rated") {
				t.Fatal("incorrect shared mixed result", result, game)
			}
			id := result["id"]
			srv.Close()
			app.Close()
			app, e = server.New(cfg, fstest.MapFS{"index.html": {Data: []byte("finished restore")}})
			if e != nil {
				t.Fatal(e)
			}
			srv = httptest.NewServer(app.Handler())
			a.base = srv.URL
			restored := a.room()
			if obj(restored["result"])["id"] != id || len(array(obj(restored["game"]), "winners")) != 2 {
				t.Fatal("terminal restore duplicated or lost result")
			}
			evidence(t, fmt.Sprint("mixed-http-", scene), map[string]any{"passed": true, "scope": "real HTTP/SQLite/production timer; one human explicitly toggles autoplay with one explicit computer; natural shared result and terminal restart", "final": final, "restored": restored})
		})
	}
}
