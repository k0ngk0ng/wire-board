package cs16_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type highRateTrace struct {
	Seat            int     `json:"seat"`
	Sent            int     `json:"sent"`
	Acknowledged    int     `json:"acknowledged"`
	LastSeq         int64   `json:"lastSeq"`
	ElapsedMS       int64   `json:"elapsedMS"`
	MaxInFlight     int64   `json:"maxInFlight"`
	WindowWaits     int     `json:"windowWaits"`
	AckMicroseconds []int64 `json:"ackMicroseconds"`
	Error           string  `json:"error,omitempty"`
}

type highRateReport struct {
	Passed             bool            `json:"passed"`
	Peers              []highRateTrace `json:"peers"`
	Clock              []inputClock    `json:"clock"`
	ElapsedMS          int64           `json:"elapsedMS"`
	Inputs             int             `json:"inputs"`
	InputsPerSecond    int64           `json:"inputsPerSecond"`
	AckP95Microseconds int64           `json:"ackP95Microseconds"`
}

func highRatePeer(parent context.Context, peer inputPeer) highRateTrace {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	trace := highRateTrace{Seat: peer.seat, AckMicroseconds: []int64{}}
	var latestFrame, sent, acknowledged, maximum atomic.Int64
	latestFrame.Store(peer.frame)
	marks := make([]time.Time, 101)
	var lock sync.Mutex
	notice := make(chan struct{}, 1)
	failure := make(chan error, 1)
	done := make(chan struct{})
	prefix := fmt.Sprintf("cs16-high-rate-%d-", peer.seat)
	go func() {
		defer close(done)
		for {
			_, raw, err := peer.connection.Read(ctx)
			if err != nil {
				select {
				case failure <- err:
				default:
				}
				return
			}
			var packet map[string]any
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.UseNumber()
			if decoder.Decode(&packet) != nil {
				continue
			}
			if packet["type"] == "game" {
				if game, ok := packet["game"].(map[string]any); ok {
					if data, ok := game["data"].(map[string]any); ok {
						if frame, ok := data["frame"].(json.Number); ok {
							latestFrame.Store(max(latestFrame.Load(), integer(frame)))
						}
					}
				}
			}
			if packet["type"] != "ack" {
				continue
			}
			nonce, ok := packet["nonce"].(string)
			if !ok || !strings.HasPrefix(nonce, prefix) {
				continue
			}
			ordinal, err := strconv.Atoi(strings.TrimPrefix(nonce, prefix))
			if err != nil || ordinal < 1 || ordinal > 100 {
				continue
			}
			if integer(packet["status"]) != 200 {
				select {
				case failure <- fmt.Errorf("input rejected: %s", raw):
				default:
				}
				return
			}
			lock.Lock()
			trace.AckMicroseconds = append(trace.AckMicroseconds, time.Since(marks[ordinal]).Microseconds())
			lock.Unlock()
			acknowledged.Add(1)
			select {
			case notice <- struct{}{}:
			default:
			}
		}
	}()
	started := time.Now()
	work := func() error {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for ordinal := 1; ordinal <= 100; ordinal++ {
			select {
			case <-ticker.C:
			case err := <-failure:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
			for sent.Load()-acknowledged.Load() >= 8 {
				trace.WindowWaits++
				select {
				case <-notice:
				case err := <-failure:
					return err
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			seq := peer.seq + int64(ordinal)
			action := map[string]any{"type": "input", "epoch": peer.epoch, "seq": seq, "life": peer.life, "viewFrame": latestFrame.Load(), "moveX": 0, "moveY": 0, "yaw": (ordinal*500 + peer.seat*1000) & 65535, "pitch": 0, "fire": false}
			message := map[string]any{"type": "input", "serverInstance": peer.instance, "roomId": peer.roomID, "version": peer.version, "nonce": prefix + strconv.Itoa(ordinal), "action": action}
			lock.Lock()
			marks[ordinal] = time.Now()
			lock.Unlock()
			sent.Add(1)
			maximum.Store(max(maximum.Load(), sent.Load()-acknowledged.Load()))
			if err := peer.connection.Write(ctx, websocket.MessageText, encode(message)); err != nil {
				return err
			}
			trace.LastSeq = seq
		}
		for acknowledged.Load() < sent.Load() {
			select {
			case <-notice:
			case err := <-failure:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	err := work()
	trace.ElapsedMS = time.Since(started).Milliseconds()
	trace.Sent, trace.Acknowledged, trace.MaxInFlight = int(sent.Load()), int(acknowledged.Load()), maximum.Load()
	if err != nil {
		trace.Error = err.Error()
	}
	cancel()
	<-done
	return trace
}

func tenHighRateInputs(t *testing.T, harness *httpRoom, peers []inputPeer) highRateReport {
	t.Helper()
	if os.Getenv("CS16_PROFILE_HIGH_RATE") == "1" || os.Getenv("GITHUB_ACTIONS") == "true" {
		directory := filepath.Join("..", "..", "..", ".local", "cs16", "verification", "server")
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		file, err := os.Create(filepath.Join(directory, "high-rate-pressure.pprof"))
		if err != nil {
			t.Fatal(err)
		}
		if err = pprof.StartCPUProfile(file); err != nil {
			file.Close()
			t.Fatal(err)
		}
		var before runtime.MemStats
		runtime.ReadMemStats(&before)
		readLimits := func() map[string]string {
			values := map[string]string{}
			for _, name := range []string{"/sys/fs/cgroup/cpu.max", "/sys/fs/cgroup/cpu.stat", "/sys/fs/cgroup/memory.max"} {
				if raw, err := os.ReadFile(name); err == nil {
					values[name] = strings.TrimSpace(string(raw))
				}
			}
			if raw, err := os.ReadFile("/proc/cpuinfo"); err == nil {
				for _, line := range strings.Split(string(raw), "\n") {
					if strings.HasPrefix(line, "model name") {
						values["cpuModel"] = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
						break
					}
				}
			}
			return values
		}
		limitsBefore := readLimits()
		defer func() {
			pprof.StopCPUProfile()
			file.Close()
			var after runtime.MemStats
			runtime.ReadMemStats(&after)
			metadata := map[string]any{"goVersion": runtime.Version(), "goos": runtime.GOOS, "goarch": runtime.GOARCH, "numCPU": runtime.NumCPU(), "gomaxprocs": runtime.GOMAXPROCS(0), "allocatedBytes": after.TotalAlloc - before.TotalAlloc, "garbageCollections": after.NumGC - before.NumGC, "limitsBefore": limitsBefore, "limitsAfter": readLimits()}
			t.Logf("CS16_PRESSURE_ENV %s", encode(metadata))
			if raw, err := os.ReadFile(file.Name()); err == nil {
				t.Logf("CS16_PRESSURE_PROFILE %s", base64.StdEncoding.EncodeToString(raw))
			}
		}()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	report := highRateReport{Peers: make([]highRateTrace, len(peers)), Clock: []inputClock{}}
	started := time.Now()
	var workers sync.WaitGroup
	for _, peer := range peers {
		workers.Add(1)
		go func(peer inputPeer) { defer workers.Done(); report.Peers[peer.seat] = highRatePeer(ctx, peer) }(peer)
	}
	complete := make(chan struct{})
	go func() { workers.Wait(); close(complete) }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	monitoring := true
	for monitoring {
		select {
		case <-complete:
			monitoring = false
		case <-ticker.C:
			data := harness.people[0].room()["game"].(map[string]any)["data"].(map[string]any)
			at := time.Now().UnixMilli()
			report.Clock = append(report.Clock, inputClock{At: at, Frame: integer(data["frame"]), LagMS: max(0, at-integer(data["serverAt"]))})
		}
	}
	report.ElapsedMS = time.Since(started).Milliseconds()
	acks := []int64{}
	report.Passed = true
	for _, trace := range report.Peers {
		report.Inputs += trace.Acknowledged
		acks = append(acks, trace.AckMicroseconds...)
		if trace.Error != "" || trace.Sent != 100 || trace.Acknowledged != 100 || trace.MaxInFlight > 8 {
			report.Passed = false
		}
	}
	report.InputsPerSecond = int64(report.Inputs) * 1000 / max(int64(1), report.ElapsedMS)
	report.AckP95Microseconds = percentile(acks, 95)
	if report.InputsPerSecond < 800 || report.AckP95Microseconds > 100000 || len(report.Clock) < 2 {
		report.Passed = false
	}
	if len(report.Clock) >= 2 {
		first, last := report.Clock[0], report.Clock[len(report.Clock)-1]
		if (last.Frame-first.Frame)*10+200 < last.At-first.At {
			report.Passed = false
		}
	}
	artifact(t, map[string]any{"passed": report.Passed, "scope": "same 10 real WebSocket connections sample yaw at 100Hz each for one second, max eight in flight, ACK readers maintain authoritative frame", "report": report})
	if !report.Passed {
		t.Fatalf("ten-human 100Hz capacity below frozen bound: %d inputs / %dms (%d/s), ACK P95 %dus", report.Inputs, report.ElapsedMS, report.InputsPerSecond, report.AckP95Microseconds)
	}
	for _, trace := range report.Peers {
		waitHTTP(t, harness.people[trace.Seat], func(data map[string]any) bool {
			return integer(data["players"].([]any)[trace.Seat].(map[string]any)["appliedSeq"]) >= trace.LastSeq
		})
	}
	return report
}
