package platformtest

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/yuyuhakusho/server"
	"github.com/k0ngk0ng/wire-board/internal/gametest"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

// The browser regression is retained separately in platform-app-browser/play02.
// This independently verifies the actual host's two gates and immutable auth.
func TestRealHostPublicLoadingPrecedesGameLoading(t *testing.T) {
	h := gametest.NewHTTP(t, 2, registered(t), "yuyuhakusho")
	h.Config.ClientLoading = true
	// Restart() records an existing room; this pre-room setup deliberately
	// reopens the public helper fields before creating the first room.
	h.Server.Close()
	h.Host.Close()
	var err error
	h.Host, err = server.New(h.Config, fstest.MapFS{"index.html": {Data: []byte("Yuyu public loading acceptance")}})
	if err != nil {
		t.Fatal(err)
	}
	h.Server = httptest.NewServer(h.Host.Handler())
	for _, c := range append(append([]*gametest.Client{}, h.People...), h.Spectator) {
		c.Base = h.Server.URL
	}
	h.Start(map[string]any{"arena": 10, "characters": []int{8, 9}})
	r := h.People[0].Room()
	gate, ok := r["loading"].(map[string]any)
	if !ok {
		t.Fatal("public loading gate missing")
	}
	v := wire(t, h.People[0])
	if v.Frame != 0 || v.Phase != "loading" {
		t.Fatal("game bypassed initial gate")
	}
	reject(t, h.People[0], map[string]any{"type": "loaded", "epoch": v.Epoch})
	for i, c := range h.People {
		c.Command("load_begin", map[string]any{"loadToken": gate["token"], "loadBuild": gate["build"], "loadClient": fmt.Sprintf("actual-client-%d", i)}, 200)
	}
	loadReady := func(i int) {
		h.People[i].Command("load_ready", map[string]any{"loadToken": gate["token"], "loadBuild": gate["build"], "loadClient": fmt.Sprintf("actual-client-%d", i)}, 200)
	}
	loadReady(0)
	time.Sleep(100 * time.Millisecond)
	if wire(t, h.People[0]).Frame != 0 || h.People[0].Room()["loading"] == nil {
		t.Fatal("one human bypassed shared gate")
	}
	loadReady(1)
	if h.People[0].Room()["loading"] != nil {
		t.Fatal("both humans did not release shared gate")
	}
	if wire(t, h.People[0]).Frame != 0 {
		t.Fatal("platform gate bypassed game initialization")
	}
	h.People[0].Action(map[string]any{"type": "loaded", "epoch": v.Epoch}, 200)
	time.Sleep(100 * time.Millisecond)
	if wire(t, h.People[0]).Frame != 0 {
		t.Fatal("one game initialized player started both")
	}
	h.People[1].Action(map[string]any{"type": "loaded", "epoch": v.Epoch}, 200)
	current := wait(t, h.People[0], func(v game.View) bool { return v.Frame > 2 })
	checkpoint(t, h.People[1])
	if h.People[0].Room()["loading"] != nil || wire(t, h.People[0]).Epoch != current.Epoch {
		t.Fatal("recovery recreated loading or match")
	}
	h.Record("real public load_begin/load_ready followed by per-game initialized readiness; recovery does not reset gates")
}
