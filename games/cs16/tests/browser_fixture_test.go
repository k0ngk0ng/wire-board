package cs16_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	game "github.com/k0ngk0ng/wire-board/games/cs16/server"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"github.com/k0ngk0ng/wire-board/internal/server"
)

func TestCS16BrowserFixture(t *testing.T) {
	if os.Getenv("WIRE_BOARD_CS16_BROWSER") != "1" {
		t.Skip("browser acceptance only")
	}
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, ".local", "cs16", "browser")
	data := filepath.Join(out, "data", time.Now().Format("20060102-150405.000000000"))
	if err = os.MkdirAll(data, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(data)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	program, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(program)
	identity, _ := json.Marshal(map[string]any{"path": executable, "bytes": len(program), "sha256": hex.EncodeToString(digest[:]), "goVersion": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH})
	if err = os.WriteFile(filepath.Join(out, "server-binary.json"), identity, 0600); err != nil {
		t.Fatal(err)
	}
	controls, err := os.ReadFile(filepath.Join(root, "games", "cs16", "controls.json"))
	if err != nil {
		t.Fatal(err)
	}
	module := game.Module()
	module.Controls = &gameapi.ControllerDefinition{}
	if err = json.Unmarshal(controls, module.Controls); err != nil {
		t.Fatal(err)
	}
	s, err := server.New(server.Config{DataDir: data, InviteCode: "cs16-browser", AdminUsername: "CS浏览器管理员", AdminPassword: "cs16-browser-password", ClientLoading: true, Modules: []gameapi.Module{module}}, os.DirFS(filepath.Join(root, "web", "dist")))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	defer host.Close()
	go host.Serve(listener)
	main := cs16Proxy(t, listener.Addr().String(), filepath.Join(out, "network-main.json"))
	watch := cs16Proxy(t, listener.Addr().String(), filepath.Join(out, "network-watch.json"))
	peer := cs16Proxy(t, listener.Addr().String(), filepath.Join(out, "network-peer.json"))
	urls, _ := json.Marshal(map[string]string{"main": "http://" + main, "peer": "http://" + peer, "watch": "http://" + watch})
	if err = os.WriteFile(filepath.Join(out, "urls.json"), urls, 0600); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(25 * time.Minute)
	defer deadline.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err = os.Stat(filepath.Join(out, "stop")); err == nil {
				return
			}
		case <-deadline.C:
			t.Fatal("browser fixture timed out")
		}
	}
}

func cs16Proxy(t *testing.T, upstream, control string) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var delay, jitter, ordinal atomic.Int64
	var offline atomic.Bool
	stop := make(chan struct{})
	var lock sync.Mutex
	connections := map[net.Conn]bool{}
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				var config struct {
					RTT     int64 `json:"rtt"`
					Jitter  int64 `json:"jitter"`
					Offline bool  `json:"offline"`
				}
				body, _ := os.ReadFile(control)
				_ = json.Unmarshal(body, &config)
				delay.Store(max(0, min(1500, config.RTT)) / 2)
				jitter.Store(max(0, min(250, config.Jitter)))
				if config.Offline && !offline.Swap(true) {
					lock.Lock()
					for connection := range connections {
						connection.Close()
					}
					lock.Unlock()
				} else if !config.Offline {
					offline.Store(false)
				}
			}
		}
	}()
	pipe := func(target, source net.Conn) {
		type packet struct {
			data []byte
			at   time.Time
		}
		queue := make(chan packet, 128)
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer target.Close()
			defer source.Close()
			for {
				select {
				case <-stop:
					return
				case current, ok := <-queue:
					if !ok {
						return
					}
					timer := time.NewTimer(max(0, time.Until(current.at)))
					select {
					case <-stop:
						timer.Stop()
						return
					case <-timer.C:
					}
					if _, err := target.Write(current.data); err != nil {
						return
					}
				}
			}
		}()
		defer func() { close(queue); <-done }()
		buffer := make([]byte, 32768)
		for {
			count, err := source.Read(buffer)
			if count > 0 {
				milliseconds := delay.Load()
				if span := jitter.Load(); span > 0 {
					milliseconds = max(0, milliseconds+(ordinal.Add(1)*37)%(span*2+1)-span)
				}
				current := packet{data: append([]byte(nil), buffer[:count]...), at: time.Now().Add(time.Duration(milliseconds) * time.Millisecond)}
				select {
				case <-stop:
					return
				case <-done:
					return
				case queue <- current:
				}
			}
			if err != nil {
				return
			}
		}
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			if offline.Load() {
				client.Close()
				continue
			}
			remote, err := net.Dial("tcp", upstream)
			if err != nil {
				client.Close()
				continue
			}
			lock.Lock()
			connections[client], connections[remote] = true, true
			lock.Unlock()
			workers.Add(2)
			go func() {
				defer workers.Done()
				pipe(remote, client)
				lock.Lock()
				delete(connections, client)
				lock.Unlock()
			}()
			go func() {
				defer workers.Done()
				pipe(client, remote)
				lock.Lock()
				delete(connections, remote)
				lock.Unlock()
			}()
		}
	}()
	t.Cleanup(func() {
		close(stop)
		listener.Close()
		lock.Lock()
		for connection := range connections {
			connection.Close()
		}
		lock.Unlock()
		workers.Wait()
	})
	return listener.Addr().String()
}
