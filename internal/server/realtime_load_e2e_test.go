package server

// Opt-in, repeatable host load evidence. Synthetic game counters isolate host
// serialization/locking/SQLite from game strategy. Every seat is a real signed-in
// HTTP/WebSocket client. Never infer real-game capacity from this fixture alone.
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type loadPeer struct {
	client  *testClient
	socket  *websocket.Conn
	room    string
	version int
	index   int
}
type loadSamples struct {
	mu                                        sync.Mutex
	rtts, lags                                []float64
	pending                                   map[string]time.Time
	bytes, gets, frames, inputs, acks, errors atomic.Int64
}

func (s *loadSamples) latency(nonce string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if at, ok := s.pending[nonce]; ok {
		s.rtts = append(s.rtts, float64(time.Since(at).Microseconds())/1000)
		delete(s.pending, nonce)
		s.acks.Add(1)
	}
}
func loadPercentiles(values []float64) map[string]any {
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	if len(v) == 0 {
		return map[string]any{"count": 0}
	}
	return map[string]any{"count": len(v), "p50MS": v[(len(v)-1)/2], "p95MS": v[(len(v)-1)*95/100], "maxMS": v[len(v)-1]}
}

func TestRealtimeLoadEvidence(t *testing.T) {
	mode := os.Getenv("WIRE_BOARD_REALTIME_LOAD")
	if mode == "" {
		t.Skip("opt-in host load evidence")
	}
	if mode != "legacy" && mode != "stream" {
		t.Fatal("mode must be legacy or stream")
	}
	rooms, _ := strconv.Atoi(os.Getenv("WIRE_BOARD_REALTIME_ROOMS"))
	if rooms < 1 || rooms > 16 {
		t.Fatal("rooms must be 1..16")
	}
	samples := &loadSamples{pending: make(map[string]time.Time)}
	var measuring atomic.Bool
	module := realtimeFixture(true, 100*time.Millisecond, 0, "")
	module.MinPlayers, module.MaxPlayers = 8, 8
	module.Start = func(c gameapi.Context, _ json.RawMessage) (gameapi.Match, error) {
		data, _ := json.Marshal(realtimePosition{Inputs: make([]int, 8), Drive: make([]int, 8)})
		return gameapi.Match{Schema: 1, Data: data, Actors: []int{0, 1, 2, 3, 4, 5, 6, 7}, Attention: []int{}, SystemDeadline: c.Now.Add(100 * time.Millisecond).UnixMilli()}, nil
	}
	module.AcceptStale = func(gameapi.Context, gameapi.Match, json.RawMessage) bool { return true }
	timer := module.OnTimer
	module.OnTimer = func(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
		if measuring.Load() {
			samples.mu.Lock()
			samples.lags = append(samples.lags, float64(time.Now().UnixMilli()-m.SystemDeadline))
			samples.mu.Unlock()
		}
		return timer(c, m)
	}
	host, err := New(Config{DataDir: t.TempDir(), InviteCode: "test-invite", Modules: []gameapi.Module{module}}, fstest.MapFS{"index.html": {Data: []byte("load fixture")}})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	httpServer := httptest.NewServer(host.Handler())
	defer httpServer.Close()
	peers := []loadPeer{}
	for roomIndex := 0; roomIndex < rooms; roomIndex++ {
		// Setup is not an authentication-rate benchmark. All identities still
		// register/login publicly; reset only this fresh fixture's IP buckets.
		host.mu.Lock()
		host.limits = make(map[string]bucket)
		host.mu.Unlock()
		group := []*testClient{}
		for seat := 0; seat < 8; seat++ {
			c := newClient(t, httpServer.URL)
			c.register(fmt.Sprintf("负载房%d席%d", roomIndex, seat))
			group = append(group, c)
		}
		created := group[0].post("/api/rooms", map[string]any{"kind": module.ID, "name": "负载基线", "capacity": 8}, 201)
		id := created["id"].(string)
		for _, c := range group[1:] {
			c.post("/api/rooms/"+id, map[string]any{"type": "join", "version": current(group[0])["version"], "nonce": randomID(12)}, 200)
		}
		for _, c := range group {
			c.command(current(c), "ready", nil, 200)
		}
		group[0].command(current(group[0]), "start", nil, 200)
		version := int(current(group[0])["version"].(float64))
		for _, c := range group {
			peers = append(peers, loadPeer{c, nil, id, version, len(peers)})
		}
	}
	for i := range peers {
		peers[i].socket = streamDial(t, peers[i].client, mode == "stream")
		streamRead(t, peers[i].socket)
	}
	// Count encoded snapshot work without collecting user data or tracing filesystem
	// internals. TEMP counters add identical instrumentation to both variants.
	if _, err = host.db.Exec(`CREATE TEMP TABLE load_counts(updates INTEGER, bytes INTEGER); INSERT INTO load_counts VALUES(0,0); CREATE TEMP TRIGGER load_room_write AFTER UPDATE ON rooms BEGIN UPDATE load_counts SET updates=updates+1,bytes=bytes+length(NEW.snapshot); END;`); err != nil {
		t.Fatal(err)
	}
	var actionsBefore int
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var readers, writers sync.WaitGroup
	for _, peer := range peers {
		peer := peer
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				_, raw, err := peer.socket.Read(ctx)
				if err != nil {
					return
				}
				if !measuring.Load() {
					continue
				}
				samples.bytes.Add(int64(len(raw)))
				var message struct {
					Type   string `json:"type"`
					Nonce  string `json:"nonce"`
					Status any    `json:"status"`
				}
				if json.Unmarshal(raw, &message) != nil {
					samples.errors.Add(1)
					continue
				}
				if message.Type == "ack" {
					if message.Status != float64(200) {
						samples.errors.Add(1)
					}
					samples.latency(message.Nonce)
				}
				if message.Type == "game" {
					samples.frames.Add(1)
				}
				if message.Type == "changed" {
					response, err := peer.client.client.Get(peer.client.base + "/api/state")
					if err != nil {
						if ctx.Err() == nil {
							samples.errors.Add(1)
						}
						return
					}
					body, err := io.ReadAll(response.Body)
					response.Body.Close()
					if err != nil || response.StatusCode != 200 {
						samples.errors.Add(1)
						return
					}
					samples.bytes.Add(int64(len(body)))
					samples.gets.Add(1)
					samples.frames.Add(1)
				}
			}
		}()
	}
	// Drain lifecycle/bootstrap messages before recording either protocol.
	time.Sleep(250 * time.Millisecond)
	var cpuFile *os.File
	if os.Getenv("WIRE_BOARD_REALTIME_PROFILE") == "1" {
		path := filepath.Join("..", "..", ".local", "realtime-efficiency", fmt.Sprintf("load-%s-%d", mode, rooms))
		cpuFile, err = os.Create(path + "-cpu.pprof")
		if err != nil {
			t.Fatal(err)
		}
		if err = pprof.StartCPUProfile(cpuFile); err != nil {
			t.Fatal(err)
		}
		oldFraction := runtime.SetMutexProfileFraction(1)
		defer func() {
			runtime.SetMutexProfileFraction(oldFraction)
			file, err := os.Create(path + "-mutex.pprof")
			if err == nil {
				_ = pprof.Lookup("mutex").WriteTo(file, 0)
				file.Close()
			}
		}()
	}
	start := time.Now()
	until := start.Add(2 * time.Second)
	host.mu.Lock()
	_, _ = host.db.Exec("UPDATE load_counts SET updates=0,bytes=0")
	_ = host.db.QueryRow("SELECT count(*) FROM actions").Scan(&actionsBefore)
	measuring.Store(true)
	host.mu.Unlock()
	for _, peer := range peers {
		peer := peer
		writers.Add(1)
		go func() {
			defer writers.Done()
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()
			serial := 0
			for time.Now().Before(until) {
				serial++
				nonce := fmt.Sprintf("load-%d-%d", peer.index, serial)
				request := map[string]any{"type": "input", "roomId": peer.room, "version": peer.version, "nonce": nonce, "action": map[string]any{"drive": 1}}
				samples.mu.Lock()
				samples.pending[nonce] = time.Now()
				samples.mu.Unlock()
				samples.inputs.Add(1)
				if mode == "stream" {
					raw, _ := json.Marshal(request)
					if peer.socket.Write(ctx, websocket.MessageText, raw) != nil {
						samples.errors.Add(1)
						return
					}
				} else {
					request["type"] = "action"
					delete(request, "roomId")
					raw, _ := json.Marshal(request)
					response, err := peer.client.client.Post(peer.client.base+"/api/rooms/"+peer.room, "application/json", bytes.NewReader(raw))
					if err != nil {
						samples.errors.Add(1)
						return
					}
					body, err := io.ReadAll(response.Body)
					response.Body.Close()
					samples.bytes.Add(int64(len(body)))
					if err != nil || response.StatusCode != 200 {
						samples.errors.Add(1)
					}
					samples.latency(nonce)
				}
				<-ticker.C
			}
		}()
	}
	writers.Wait()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); {
		samples.mu.Lock()
		pending := len(samples.pending)
		samples.mu.Unlock()
		if pending == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	for _, peer := range peers {
		peer.socket.CloseNow()
	}
	readers.Wait()
	measuring.Store(false)
	host.mu.Lock()
	var snapshotUpdates, snapshotBytes, actionsAfter int64
	host.db.QueryRow("SELECT updates,bytes FROM load_counts").Scan(&snapshotUpdates, &snapshotBytes)
	host.db.QueryRow("SELECT count(*) FROM actions").Scan(&actionsAfter)
	host.mu.Unlock()
	elapsed := time.Since(start).Milliseconds()
	if cpuFile != nil {
		pprof.StopCPUProfile()
		cpuFile.Close()
	}
	samples.mu.Lock()
	defer samples.mu.Unlock()
	evidence := map[string]any{"mode": mode, "rooms": rooms, "humanSeats": len(peers), "elapsedMS": elapsed, "inputs": samples.inputs.Load(), "acks": samples.acks.Load(), "pending": len(samples.pending), "errors": samples.errors.Load(), "stateGETs": samples.gets.Load(), "gameSnapshots": samples.frames.Load(), "applicationResponseBytes": samples.bytes.Load(), "snapshotUpdates": snapshotUpdates, "encodedSnapshotBytes": snapshotBytes, "journalRows": actionsAfter - int64(actionsBefore), "inputAck": loadPercentiles(samples.rtts), "systemFrameLateness": loadPercentiles(samples.lags), "scope": "synthetic host-counter module; fixture auth IP buckets reset during setup; 8 real authenticated humans per room, 5 inputs/s each; 2s input window plus ACK drain; TEMP SQLite counters; excludes TLS and HTTP headers"}
	streamEvidence(t, fmt.Sprintf("load-%s-%d", mode, rooms), evidence)
	if samples.errors.Load() != 0 || len(samples.pending) != 0 || samples.acks.Load() != samples.inputs.Load() {
		t.Fatal("load violated acknowledgement contract", evidence)
	}
}
