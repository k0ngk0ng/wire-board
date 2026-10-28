//go:build !js

package runtime

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	engine "github.com/k0ngk0ng/wire-board/games/contra/engine"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// DevelopmentHandler is a private acceptance host, not a registered game or a
// production room protocol. Automatic mode owns the 60Hz clock; HTTP accepts
// held pads only. The explicit /frame endpoint exists only in manual E2E mode.
type DevelopmentHandler struct {
	mu        sync.Mutex
	bridge    *Bridge
	games     map[uint64]*developmentSession
	automatic bool
}
type developmentSession struct {
	epoch     string
	latest    Snapshot
	sequence  uint64
	pads      [2]int
	pending   [][2]int
	history   []developmentFrame
	lastInput time.Time
	running   bool
	fault     string
}
type developmentFrame struct {
	Kind     string   `json:"kind"`
	Pads     [2]int   `json:"pads"`
	Snapshot Snapshot `json:"snapshot"`
}

func NewDevelopmentHandler(resources []byte, ids ...string) (*DevelopmentHandler, error) {
	b := New(ids...)
	if _, err := b.Call(Request{Op: "resources", Resources: resources}); err != nil {
		return nil, err
	}
	return &DevelopmentHandler{bridge: b, games: map[uint64]*developmentSession{}}, nil
}
func NewBundleDevelopmentHandler(manifest engine.LevelBundleManifest, stageZero []byte, ids ...string) (*DevelopmentHandler, error) {
	b := New(ids...)
	if _, err := b.Call(Request{Op: "bundle", Bundle: &manifest}); err != nil {
		return nil, err
	}
	if _, err := b.Call(Request{Op: "stage", Level: 0, StageData: string(stageZero)}); err != nil {
		return nil, err
	}
	return &DevelopmentHandler{bridge: b, games: map[uint64]*developmentSession{}}, nil
}
func (h *DevelopmentHandler) Run(stop <-chan struct{}) {
	h.mu.Lock()
	h.automatic = true
	h.mu.Unlock()
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			h.mu.Lock()
			for id, g := range h.games {
				if !g.running || g.fault != "" || g.latest.Finalized {
					continue
				}
				previousPads := g.pads
				if len(g.pending) > 0 {
					g.pads = g.pending[0]
					g.pending = g.pending[1:]
				} else if now.Sub(g.lastInput) > 750*time.Millisecond {
					g.pads = [2]int{}
				}
				resume := g.latest.Paused && g.pads[0]&16 != 0 && previousPads[0]&16 == 0
				if resume {
					value, err := h.bridge.Call(Request{Op: "resume", Session: id, Epoch: g.epoch, Revision: g.latest.Revision + 1})
					if err != nil {
						g.fault = err.Error()
						continue
					}
					g.latest = value.(Snapshot)
					g.history = append(g.history, developmentFrame{Kind: "resume", Pads: g.pads, Snapshot: g.latest})
				}
				kind := "pads"
				if g.latest.Paused && !resume {
					kind = "present-paused"
				}
				op := kind
				if op == "present-paused" {
					op = "paused"
				}
				request := Request{Op: op, Session: id, Epoch: g.epoch, Frame: g.latest.Frame + 1, Pads: g.pads[:]}
				if kind == "present-paused" {
					request.Revision = g.latest.Revision + 1
				}
				value, err := h.bridge.Call(request)
				if err != nil {
					g.fault = err.Error()
					continue
				}
				g.latest = value.(Snapshot)
				g.history = append(g.history, developmentFrame{Kind: kind, Pads: g.pads, Snapshot: g.latest})
				if g.latest.Phase == "finishing" {
					value, err := h.bridge.Call(Request{Op: "terminal", Session: id, Epoch: g.epoch, Revision: g.latest.Revision + 1})
					if err != nil {
						g.fault = err.Error()
						continue
					}
					g.latest = value.(Snapshot)
					g.history = append(g.history, developmentFrame{Kind: "terminal", Pads: g.pads, Snapshot: g.latest})
				}
				if len(g.history) > 120 {
					g.history = g.history[len(g.history)-120:]
				}
			}
			h.mu.Unlock()
		}
	}
}
func (h *DevelopmentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(status int, value any) { w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value) }
	fail := func(err error) { write(400, map[string]string{"error": err.Error()}) }
	if r.Method != "POST" {
		write(405, map[string]string{"error": "POST required"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(err)
		return
	}
	if r.URL.Path == "/new" {
		var request struct {
			Players int `json:"players"`
		}
		if err = strict(raw, &request); err != nil {
			fail(err)
			return
		}
		var token [24]byte
		if _, err = rand.Read(token[:]); err != nil {
			fail(err)
			return
		}
		epoch := hex.EncodeToString(token[8:])
		seed := strconv.FormatUint(binary.LittleEndian.Uint64(token[:8]), 10)
		value, e := h.bridge.Call(Request{Op: "new", Seed: seed, Players: request.Players, Epoch: epoch})
		if e != nil {
			fail(e)
			return
		}
		snapshot := value.(Snapshot)
		h.games[snapshot.Session] = &developmentSession{epoch: epoch, latest: snapshot, lastInput: time.Now()}
		write(200, snapshot)
		return
	}
	if r.URL.Path == "/restore" {
		write(404, map[string]string{"error": "authority never accepts client checkpoints"})
		return
	}
	var request struct {
		Session  uint64  `json:"session"`
		Epoch    string  `json:"epoch"`
		Frame    uint64  `json:"frame,string,omitempty"`
		Revision *uint64 `json:"revision,string,omitempty"`
		Pads     []int   `json:"pads,omitempty"`
		Sequence uint64  `json:"sequence,omitempty"`
	}
	if err = strict(raw, &request); err != nil {
		fail(err)
		return
	}
	g := h.games[request.Session]
	if g == nil || g.epoch != request.Epoch {
		fail(errors.New("runtime session/epoch mismatch"))
		return
	}
	switch r.URL.Path {
	case "/frame":
		if h.automatic {
			write(404, map[string]string{"error": "manual stepping disabled"})
			return
		}
		value, e := h.bridge.Call(Request{Op: "pads", Session: request.Session, Epoch: request.Epoch, Frame: request.Frame, Pads: request.Pads})
		if e != nil {
			fail(e)
			return
		}
		g.latest = value.(Snapshot)
		write(200, g.latest)
	case "/checkpoint":
		value, e := h.bridge.Call(Request{Op: "checkpoint", Session: request.Session, Epoch: request.Epoch})
		if e != nil {
			fail(e)
			return
		}
		write(200, value)
	case "/ready":
		g.running = true
		write(200, map[string]bool{"ready": true})
	case "/input":
		if len(request.Pads) != 2 || request.Pads[0] < 0 || request.Pads[0] > 255 || request.Pads[1] < 0 || request.Pads[1] > 255 || request.Sequence == 0 {
			fail(errors.New("invalid normalized controller input"))
			return
		}
		if request.Sequence > g.sequence {
			if len(g.pending) >= 32 {
				fail(errors.New("controller queue full"))
				return
			}
			g.sequence = request.Sequence
			g.lastInput = time.Now()
			g.pending = append(g.pending, [2]int{request.Pads[0], request.Pads[1]})
		}
		write(200, map[string]uint64{"sequence": g.sequence})
	case "/poll":
		if g.fault != "" {
			write(500, map[string]string{"error": g.fault})
			return
		}
		frames := []developmentFrame{}
		cursor := request.Frame
		at := func(snapshot Snapshot) uint64 { return snapshot.Frame }
		if request.Revision != nil {
			cursor = *request.Revision
			at = func(snapshot Snapshot) uint64 { return snapshot.Revision }
		}
		for _, frame := range g.history {
			if at(frame.Snapshot) > cursor {
				frames = append(frames, frame)
			}
		}
		result := map[string]any{"snapshot": g.latest, "frames": frames}
		if len(frames) > 0 && at(frames[0].Snapshot) != cursor+1 {
			checkpoint, e := h.bridge.Call(Request{Op: "checkpoint", Session: request.Session, Epoch: request.Epoch})
			if e != nil {
				fail(e)
				return
			}
			result["checkpoint"] = checkpoint
			result["frames"] = []developmentFrame{}
		}
		write(200, result)
	case "/close":
		g.running = false
		_, e := h.bridge.Call(Request{Op: "dispose", Session: request.Session, Epoch: request.Epoch})
		if e != nil {
			fail(e)
			return
		}
		delete(h.games, request.Session)
		write(200, map[string]bool{"closed": true})
	default:
		write(404, map[string]string{"error": "unknown development endpoint"})
	}
}
