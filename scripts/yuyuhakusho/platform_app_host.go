//go:build ignore

// Local E2E host: the real composition root, authentication, SQLite and sockets.
// The second listener delays TCP writes without rewriting application messages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/k0ngk0ng/wire-board/games"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

type shapedListener struct {
	net.Listener
	delay       atomic.Int64
	mu          sync.Mutex
	connections map[*shapedConn]bool
}
type shapedConn struct {
	net.Conn
	owner *shapedListener
}

func (l *shapedListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	s := &shapedConn{Conn: c, owner: l}
	l.mu.Lock()
	l.connections[s] = true
	l.mu.Unlock()
	return s, nil
}
func (c *shapedConn) Write(b []byte) (int, error) {
	if d := c.owner.delay.Load(); d > 0 {
		time.Sleep(time.Duration(d) * time.Millisecond)
	}
	return c.Conn.Write(b)
}
func (c *shapedConn) Close() error {
	c.owner.mu.Lock()
	delete(c.owner.connections, c)
	c.owner.mu.Unlock()
	return c.Conn.Close()
}
func (l *shapedListener) disconnect() {
	l.mu.Lock()
	connections := make([]*shapedConn, 0, len(l.connections))
	for c := range l.connections {
		connections = append(connections, c)
	}
	l.mu.Unlock()
	for _, c := range connections {
		_ = c.Close()
	}
}

func main() {
	build := flag.String("build", "", "local-only app dist")
	data := flag.String("data", "", "workspace database directory")
	listen := flag.String("listen", "127.0.0.1:0", "local review listen address")
	flag.Parse()
	s, err := server.New(server.Config{DataDir: *data, InviteCode: "yuyu-local-e2e", ClientLoading: true, Modules: games.Modules()}, os.DirFS(*build))
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	direct, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	slowSocket, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	slow := &shapedListener{Listener: slowSocket, connections: map[*shapedConn]bool{}}
	target, _ := url.Parse("http://" + direct.Addr().String())
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.FlushInterval = -1
	go func() {
		if err := http.Serve(direct, s.Handler()); err != nil {
			log.Print(err)
		}
	}()
	go func() {
		if err := http.Serve(slow, proxy); err != nil {
			log.Print(err)
		}
	}()
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"direct": target.String() + "/", "shaped": "http://" + slow.Addr().String() + "/"})
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var command struct {
			DelayMS    int64 `json:"delayMs"`
			Disconnect bool  `json:"disconnect"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &command); err != nil || command.DelayMS < 0 || command.DelayMS > 2000 {
			fmt.Fprintln(os.Stderr, "invalid local network command")
			continue
		}
		slow.delay.Store(command.DelayMS)
		if command.Disconnect {
			slow.disconnect()
		}
		_ = json.NewEncoder(os.Stdout).Encode(command)
	}
	slow.disconnect()
	_ = slow.Close()
	_ = direct.Close()
}
