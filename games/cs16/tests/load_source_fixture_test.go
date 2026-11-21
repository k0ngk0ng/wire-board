package cs16_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/cs16/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func newSourceEndpoint(t *testing.T, handler http.Handler, source string) *httptest.Server {
	t.Helper()
	if net.ParseIP(source) == nil {
		t.Fatal("invalid simulated test source", source)
	}
	address := net.JoinHostPort(source, "32000")
	endpoint := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		forwarded := request.Clone(request.Context())
		forwarded.RemoteAddr = address
		handler.ServeHTTP(writer, forwarded)
	}))
	t.Cleanup(endpoint.Close)
	return endpoint
}

func newFortyHumanHTTPRoom(t *testing.T) *httpRoom {
	t.Helper()
	root := filepath.Join("..", "..", "..", ".local", "cs16", "tmp")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(root, "race-source-http-")
	if err != nil {
		t.Fatal(err)
	}
	module := game.Module()
	module.Controls = controls(t)
	config := server.Config{DataDir: directory, InviteCode: "cs16-local-invite", AdminUsername: "CS验收管理员", AdminPassword: "cs16-local-password", Modules: []gameapi.Module{module}}
	host, err := server.New(config, fstest.MapFS{"index.html": {Data: []byte("CS 1.6 race source identity acceptance")}})
	if err != nil {
		os.RemoveAll(directory)
		t.Fatal(err)
	}
	room := &httpRoom{host: host, config: config}
	t.Cleanup(func() { room.host.Close(); os.RemoveAll(directory) })
	handler := host.Handler()
	for index := 0; index <= 40; index++ {
		endpoint := newSourceEndpoint(t, handler, fmt.Sprintf("192.0.2.%d", index+1))
		if index == 0 {
			room.endpoint = endpoint
		}
		jar, _ := cookiejar.New(nil)
		person := &httpPerson{t: t, base: endpoint.URL, http: &http.Client{Jar: jar, Timeout: 10 * time.Second}}
		if index == 0 {
			person.request("POST", "/api/login", map[string]any{"name": "CS验收管理员", "password": "cs16-local-password"}, 200)
		} else {
			person.request("POST", "/api/register", map[string]any{"name": fmt.Sprintf("CS验收玩家%d", index), "password": "cs16-local-password", "invite": "cs16-local-invite"}, 200)
		}
		if index < 40 {
			room.people = append(room.people, person)
		} else {
			room.viewer = person
		}
	}
	return room
}
