package tabletennis_test

// Test-only TCP delay affects the actual HTTP and WebSocket byte streams. No
// game callback, clock, score or state is replaced. Browser acceptance controls
// only the transport delay through a local file, in round-trip milliseconds.
import (
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func networkProxy(t *testing.T, upstream, control string) string {
	t.Helper()
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	var delay atomic.Int64
	var jitter atomic.Int64
	var offline atomic.Bool
	var packets atomic.Int64
	stop := make(chan struct{})
	var mu sync.Mutex
	connections := map[net.Conn]bool{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		timer := time.NewTicker(20 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-stop:
				return
			case <-timer.C:
				b, _ := os.ReadFile(control)
				n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
				var settings struct {
					Delay   int64 `json:"delay"`
					Jitter  int64 `json:"jitter"`
					Offline bool  `json:"offline"`
				}
				if json.Unmarshal(b, &settings) == nil && len(b) > 0 && b[0] == '{' {
					n = settings.Delay
				}
				jitter.Store(max(0, min(250, settings.Jitter)))
				if settings.Offline && !offline.Swap(true) {
					mu.Lock()
					for c := range connections {
						c.Close()
					}
					mu.Unlock()
				} else if !settings.Offline {
					offline.Store(false)
				}
				if n < 0 {
					n = 0
				}
				if n > 1500 {
					n = 1500
				}
				delay.Store(n / 2)
			}
		}
	}()
	pipe := func(dst, src net.Conn) {
		defer dst.Close()
		defer src.Close()
		buffer := make([]byte, 32768)
		for {
			n, e := src.Read(buffer)
			if n > 0 {
				ms := delay.Load()
				if span := jitter.Load(); span > 0 {
					ms = max(0, ms+(packets.Add(1)*37)%(span*2+1)-span)
				}
				if ms > 0 {
					timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
					select {
					case <-stop:
						timer.Stop()
						return
					case <-timer.C:
					}
				}
				if _, err := dst.Write(buffer[:n]); err != nil {
					return
				}
			}
			if e != nil {
				return
			}
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			client, e := ln.Accept()
			if e != nil {
				return
			}
			if offline.Load() {
				client.Close()
				continue
			}
			remote, e := net.Dial("tcp", upstream)
			if e != nil {
				client.Close()
				continue
			}
			mu.Lock()
			connections[client] = true
			connections[remote] = true
			mu.Unlock()
			wg.Add(2)
			go func() { defer wg.Done(); pipe(remote, client); mu.Lock(); delete(connections, client); mu.Unlock() }()
			go func() { defer wg.Done(); pipe(client, remote); mu.Lock(); delete(connections, remote); mu.Unlock() }()
		}
	}()
	t.Cleanup(func() {
		close(stop)
		ln.Close()
		mu.Lock()
		for c := range connections {
			c.Close()
		}
		mu.Unlock()
		wg.Wait()
	})
	return ln.Addr().String()
}
