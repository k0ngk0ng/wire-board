package server

import (
	"encoding/json"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"time"
)

// moduleInputReleaseAllowed only relaxes the autoplay guard for an explicitly
// enabled module. It never applies controls or changes ownership. Even a buggy
// predicate cannot mutate the room, request journal or caller's action buffer.
func (s *Server) moduleInputReleaseAllowed(r *Room, actor, version int, action json.RawMessage, now time.Time) (allowed bool) {
	defer func() {
		if recover() != nil {
			allowed = false
		}
	}()
	m, ok := s.modules[r.Kind]
	if !ok || !m.Realtime || m.IsInputRelease == nil || r.Module == nil || r.Status != "playing" || r.Loading != nil || r.Module.Result != nil || actor < 0 || actor >= len(r.Seats) {
		return false
	}
	seat := r.Seats[actor]
	if !seat.AutoPlay || seat.Bot || seat.Left {
		return false
	}
	raw, e := json.Marshal(r.Module)
	if e != nil {
		return false
	}
	var detached gameapi.Match
	if e = json.Unmarshal(raw, &detached); e != nil {
		return false
	}
	return m.IsInputRelease(moduleContext(r, actor, version, now), detached, append(json.RawMessage(nil), action...))
}
