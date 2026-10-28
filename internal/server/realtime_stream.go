package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

// A memory response lets both transports use the same authenticated command
// rules, nonce deduplication and permissions. No loopback HTTP request.
type bufferedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (b *bufferedResponse) Header() http.Header {
	if b.header == nil {
		b.header = make(http.Header)
	}
	return b.header
}
func (b *bufferedResponse) WriteHeader(status int) { b.status = status }
func (b *bufferedResponse) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.body.Write(p)
}

type streamInput struct {
	ServerInstance string          `json:"serverInstance,omitempty"`
	Type           string          `json:"type"`
	RoomID         string          `json:"roomId"`
	Version        int             `json:"version"`
	Nonce          string          `json:"nonce"`
	Action         json.RawMessage `json:"action"`
}
type streamAck struct {
	Durable bool   `json:"durable"`
	Type    string `json:"type"`
	Nonce   string `json:"nonce"`
	Status  int    `json:"status"`
	Version int    `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (s *Server) streamCommand(r *http.Request, raw []byte) streamAck {
	var input streamInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return streamAck{Type: "ack", Status: 400, Error: "无效实时输入"}
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || input.Type != "input" || len(input.RoomID) > 100 {
		return streamAck{Type: "ack", Nonce: input.Nonce, Status: 400, Error: "无效实时输入"}
	}
	s.mu.Lock()
	room := s.rooms[input.RoomID]
	supported := room != nil && s.modules[room.Kind].Realtime
	s.mu.Unlock()
	if room != nil && !supported {
		return streamAck{Type: "ack", Nonce: input.Nonce, Status: 400, Error: "该游戏不使用实时输入"}
	}
	body, _ := json.Marshal(map[string]any{"type": "action", "version": input.Version, "nonce": input.Nonce, "action": input.Action, "serverInstance": input.ServerInstance})
	request := r.Clone(r.Context())
	request.Method = http.MethodPost
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	request.SetPathValue("id", input.RoomID)
	response := &bufferedResponse{}
	s.command(response, request)
	ack := streamAck{Type: "ack", Nonce: input.Nonce, Status: response.status}
	var result struct {
		Version int    `json:"version"`
		Durable bool   `json:"durable"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(response.body.Bytes(), &result)
	ack.Version, ack.Error, ack.Durable = result.Version, result.Error, result.Durable
	return ack
}

// The high frequency packet deliberately excludes lobby, configuration, chat
// and room metadata. View is still computed per authenticated viewer; private
// data must never be shared between seats by a room-wide serialization cache.
type streamConnectionCursor struct {
	RoomID string
	Viewer int
	Value  string
}

func (s *Server) streamGame(r *http.Request) (any, bool) {
	packet, _, ok := s.streamGameCursor(r, streamConnectionCursor{})
	return packet, ok
}

func (s *Server) streamGameCursor(r *http.Request, previous streamConnectionCursor) (any, streamConnectionCursor, bool) {
	s.mu.Lock()
	user, err := s.user(r)
	if err != nil {
		s.mu.Unlock()
		return nil, streamConnectionCursor{}, false
	}
	room := s.current(user.ID)
	if room == nil {
		room = s.watching(user.ID)
	}
	if room == nil || room.Module == nil || !s.modules[room.Kind].Realtime || room.Status != "playing" {
		s.mu.Unlock()
		return nil, streamConnectionCursor{}, true
	}
	module := s.modules[room.Kind]
	viewer := seatIndex(room, user.ID)
	envelope := func(view any) map[string]any {
		return map[string]any{"serverInstance": s.instance, "type": "game", "roomId": room.ID, "setupVersion": room.SetupVersion, "version": room.Version, "status": room.Status, "turnDeadline": room.TurnDeadline, "loading": room.loadingView(), "clockOffsetMs": room.ClockOffsetMS, "serverNow": time.Now().UnixMilli(), "game": view}
	}
	if module.StreamView == nil {
		// Existing games retain their original callback serialization contract.
		view, err := s.moduleView(room, viewer)
		packet := envelope(view)
		s.mu.Unlock()
		return packet, streamConnectionCursor{}, err == nil
	}
	next := streamConnectionCursor{RoomID: room.ID, Viewer: viewer}
	cursor := ""
	if previous.RoomID == room.ID && previous.Viewer == viewer {
		cursor = previous.Value
	}
	// Both the match (including Data) and public envelope are deep snapshots
	// from one room version. No room-owned pointers escape the platform lock.
	rawMatch, err := json.Marshal(room.Module)
	var rawEnvelope []byte
	if err == nil {
		rawEnvelope, err = json.Marshal(envelope(moduleViewData(room, *room.Module, nil)))
	}
	s.mu.Unlock()
	if err != nil {
		return nil, next, false
	}
	var match gameapi.Match
	if json.Unmarshal(rawMatch, &match) != nil {
		return nil, next, false
	}
	value, err := module.StreamView(match, viewer, cursor)
	if err != nil {
		return nil, next, false
	}
	var packet map[string]any
	if json.Unmarshal(rawEnvelope, &packet) != nil {
		return nil, next, false
	}
	packet["game"].(map[string]any)["data"] = value.Data
	next.Value = value.Cursor
	return packet, next, true
}

func (s *Server) streamSocket(w http.ResponseWriter, r *http.Request, user User) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(16 * 1024)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	changed, frames := make(chan struct{}, 1), make(chan struct{}, 1)
	networkChanged := make(chan struct{}, 1)
	replies := make(chan streamAck, 32)
	s.mu.Lock()
	if s.streamFrames == nil {
		s.streamFrames = make(map[chan struct{}]chan struct{})
	}
	if s.streamNetworks == nil {
		s.streamNetworks = make(map[chan struct{}]chan struct{})
		s.networkSamples = make(map[chan struct{}]networkSample)
	}
	s.watchers[changed], s.connections[changed], s.streamFrames[changed] = true, user.ID, frames
	s.streamNetworks[changed] = networkChanged
	s.seen(user.ID)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.watchers, changed)
		delete(s.connections, changed)
		delete(s.streamFrames, changed)
		delete(s.streamNetworks, changed)
		delete(s.networkSamples, changed)
		s.notifyNetwork(user.ID)
		s.mu.Unlock()
	}()
	go func() {
		defer cancel()
		for {
			kind, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			if kind != websocket.MessageText {
				return
			}
			ack := s.streamCommand(r, raw)
			select {
			case replies <- ack:
			case <-ctx.Done():
				return
			}
		}
	}()
	write := func(packet any) bool {
		raw, err := json.Marshal(packet)
		if err != nil {
			return false
		}
		writeCtx, stop := context.WithTimeout(ctx, 5*time.Second)
		defer stop()
		return conn.Write(writeCtx, websocket.MessageText, raw) == nil
	}
	state := func() bool {
		status, raw := s.statePayload(r)
		if status != http.StatusOK {
			return false
		}
		return write(struct {
			Type  string          `json:"type"`
			State json.RawMessage `json:"state"`
		}{"state", raw})
	}
	if !state() {
		return
	}
	go s.measureNetwork(ctx, cancel, conn, r, changed, user.ID)
	// A single pending frame always refers to current state. Slow clients cannot
	// accumulate an unbounded queue of old snapshots or hold the platform lock.
	push := time.NewTicker(50 * time.Millisecond)
	defer push.Stop()
	pending := false
	cursor := streamConnectionCursor{}
	for {
		select {
		case <-ctx.Done():
			return
		case ack := <-replies:
			if !write(ack) {
				return
			}
		case <-networkChanged:
			packet, valid := s.networkPacket(r, changed)
			if !valid || !write(packet) {
				return
			}
		case <-changed:
			if !state() {
				return
			}
			pending = false
		case <-frames:
			pending = true
		case <-push.C:
			if !pending {
				continue
			}
			pending = false
			packet, nextCursor, valid := s.streamGameCursor(r, cursor)
			if !valid {
				return
			}
			if packet != nil {
				if !write(packet) {
					return
				}
				cursor = nextCursor
			} else if !state() {
				return
			}
		}
	}
}
