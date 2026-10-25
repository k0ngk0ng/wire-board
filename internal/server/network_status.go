package server

import (
	"context"
	"github.com/coder/websocket"
	"net/http"
	"time"
)

type networkSample struct {
	RTT time.Duration
	At  time.Time
}
type networkInfo struct {
	Connections int    `json:"connections,omitempty"`
	Connected   bool   `json:"connected"`
	RTTMS       *int64 `json:"rttMs"`
	SampledAt   int64  `json:"sampledAt"`
}
type networkSeat struct {
	Seat
	Network *networkInfo `json:"network,omitempty"`
}

// Caller holds mu. Multiple tabs use the slowest fresh measured connection.
func (s *Server) networkFor(user string) networkInfo {
	out := networkInfo{}
	now := time.Now()
	for ch, id := range s.connections {
		if id != user {
			continue
		}
		out.Connected = true
		out.Connections++
		if sample, ok := s.networkSamples[ch]; ok && now.Sub(sample.At) < 15*time.Second {
			ms := sample.RTT.Milliseconds()
			if out.RTTMS == nil || ms > *out.RTTMS {
				out.RTTMS = &ms
				out.SampledAt = sample.At.UnixMilli()
			}
		}
	}
	return out
}
func (s *Server) networkSeats(seats []Seat) []networkSeat {
	out := make([]networkSeat, len(seats))
	for i, seat := range seats {
		out[i].Seat = seat
		if !seat.Bot && !seat.Left {
			info := s.networkFor(seat.ID)
			out[i].Network = &info
		}
	}
	return out
}
func (s *Server) notifyNetwork(user string) {
	room := s.current(user)
	for ch, id := range s.connections {
		if id != user && (room == nil || (s.current(id) != room && s.watching(id) != room)) {
			continue
		}
		target := s.streamNetworks[ch]
		if target == nil {
			target = ch
		}
		select {
		case target <- struct{}{}:
		default:
		}
	}
}
func (s *Server) connectionNetwork(ch chan struct{}) networkInfo {
	info := networkInfo{Connected: s.connections[ch] != "", Connections: 1}
	if sample, ok := s.networkSamples[ch]; ok && time.Since(sample.At) < 15*time.Second {
		ms := sample.RTT.Milliseconds()
		info.RTTMS = &ms
		info.SampledAt = sample.At.UnixMilli()
	}
	return info
}
func (s *Server) networkPacket(r *http.Request, ch chan struct{}) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, e := s.user(r)
	if e != nil {
		return nil, false
	}
	room := s.current(u.ID)
	if room == nil {
		room = s.watching(u.ID)
	}
	players := map[string]networkInfo{}
	roomID := ""
	if room != nil {
		roomID = room.ID
		for _, p := range room.Seats {
			if !p.Bot && !p.Left {
				players[p.ID] = s.networkFor(p.ID)
			}
		}
	}
	return map[string]any{"type": "network", "userId": u.ID, "serverInstance": s.instance, "serverNow": time.Now().UnixMilli(), "roomId": roomID, "self": s.connectionNetwork(ch), "players": players}, true
}
func (s *Server) measureNetwork(ctx context.Context, cancel context.CancelFunc, conn *websocket.Conn, r *http.Request, ch chan struct{}, user string) {
	defer cancel()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if _, e := s.user(r); e != nil {
			return
		}
		probe, stop := context.WithTimeout(ctx, 5*time.Second)
		began := time.Now()
		e := conn.Ping(probe)
		elapsed := time.Since(began)
		stop()
		if e != nil {
			return
		}
		s.mu.Lock()
		if s.connections[ch] == user && ctx.Err() == nil {
			s.networkSamples[ch] = networkSample{elapsed, time.Now()}
			s.seen(user)
			s.notifyNetwork(user)
		}
		s.mu.Unlock()
		timer.Reset(5 * time.Second)
	}
}
