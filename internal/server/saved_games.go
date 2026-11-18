package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sort"
	"time"
)

// Saved journeys retain their original room/match identity for idempotency and
// settlement, but occupy no seat and are absent from the active room directory.
// Their durable snapshot is not subject to active-room inactivity expiration.
type savedGameSummary struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	SavedAt int64    `json:"savedAt"`
	Version int      `json:"version"`
	Status  string   `json:"status"`
	Players []User   `json:"players"`
	Joined  []string `json:"joined"`
}

func savedSummary(r *Room) savedGameSummary {
	v := savedGameSummary{ID: r.ID, Name: r.Name, Kind: r.Kind, SavedAt: r.SavedAt, Version: r.Version, Status: r.Status, Players: []User{}, Joined: append([]string{}, r.ResumeJoined...)}
	for _, seat := range r.Seats {
		if !seat.Bot && !seat.Left {
			v.Players = append(v.Players, seat.User)
		}
	}
	return v
}

func (s *Server) savedGames(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, req)
	if !ok {
		return
	}
	respond(w, 200, map[string]any{"saves": s.savedGamesFor(u.ID)})
}

func (s *Server) savedGamesFor(userID string) []savedGameSummary {
	saves := []savedGameSummary{}
	for _, room := range s.rooms {
		if (room.Status == "saved" || room.Status == "restoring") && seatIndex(room, userID) >= 0 {
			saves = append(saves, savedSummary(room))
		}
	}
	sort.Slice(saves, func(i, j int) bool {
		if saves[i].SavedAt == saves[j].SavedAt {
			return saves[i].ID < saves[j].ID
		}
		return saves[i].SavedAt > saves[j].SavedAt
	})
	return saves
}

func (s *Server) voteSaveGame(r *Room, actor int, approve bool, now time.Time) error {
	m := s.modules[r.Kind]
	if !m.SupportsSave || !r.SupportsSave || r.Status != "playing" || r.Module == nil || r.Module.Result != nil || r.Loading != nil {
		return errors.New("当前对局不能保存退出")
	}
	if actor < 0 || actor >= len(r.Seats) || r.Seats[actor].Bot || r.Seats[actor].Left {
		return errors.New("只有本局原玩家可以确认保存")
	}
	id := r.Seats[actor].ID
	if m.SaveControl != nil && ((!approve && len(r.SaveVotes) != 0) || (approve && len(r.SaveVotes) == 0)) {
		match, err := m.SaveControl(moduleContext(r, actor, r.Version, now), *r.Module, approve)
		if err != nil {
			return err
		}
		if approve && (!match.Paused || match.SystemDeadline != 0) {
			return errors.New("保存确认必须停止游戏计时")
		}
		if err = s.setModule(r, match); err != nil {
			return err
		}
		// setModule invalidates approvals. This first pause intentionally starts
		// a new consent; cancellation discards every approval and resumes play.
		if !approve {
			return nil
		}
	}
	r.SaveVotes = slices.DeleteFunc(r.SaveVotes, func(v string) bool { return v == id })
	if approve {
		r.SaveVotes = append(r.SaveVotes, id)
	}
	for _, seat := range r.Seats {
		if !seat.Bot && !seat.Left && !slices.Contains(r.SaveVotes, seat.ID) {
			return nil
		}
	}
	r.SaveClock, r.SavedAt = r.gameNow(now).UnixMilli(), now.UnixMilli()
	if err := s.captureCheckpoint(r); err != nil {
		return err
	}
	r.Status, r.TurnDeadline, r.BotAt = "saved", 0, 0
	r.SaveVotes, r.ResumeJoined, r.Spectators = nil, nil, nil
	return nil
}

func (s *Server) resumeSavedGame(w http.ResponseWriter, req *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.needUser(w, req)
	if !ok {
		return
	}
	room := s.rooms[req.PathValue("id")]
	if room == nil || seatIndex(room, u.ID) < 0 {
		fail(w, 404, "存档不存在")
		return
	}
	var body struct{}
	if !decode(w, req, &body) {
		return
	}
	occupied := s.current(u.ID)
	if occupied != nil && occupied != room || s.watching(u.ID) != nil {
		fail(w, 409, "请先离开当前房间")
		return
	}
	if room.Status == "playing" && occupied == room && slices.Contains(room.ResumeJoined, u.ID) {
		respond(w, 200, summary(room))
		return
	}
	m, found := s.modules[room.Kind]
	if !found || !m.SupportsSave || !room.SupportsSave || room.Module == nil || room.Module.Result != nil || room.SaveClock <= 0 || room.Status != "saved" && room.Status != "restoring" {
		fail(w, 400, "此存档当前不能继续")
		return
	}
	if room.Status == "restoring" && slices.Contains(room.ResumeJoined, u.ID) {
		respond(w, 200, summary(room))
		return
	}
	raw, err := json.Marshal(room)
	if err != nil {
		fail(w, 500, "无法读取存档")
		return
	}
	var next Room
	if err = json.Unmarshal(raw, &next); err != nil {
		fail(w, 500, "无法读取存档")
		return
	}
	if next.Status == "saved" {
		next.ResumeJoined = nil
		for i := range next.Seats {
			next.Seats[i].Ready = next.Seats[i].Bot || next.Seats[i].Left
		}
	}
	next.Status = "restoring"
	next.ResumeJoined = append(next.ResumeJoined, u.ID)
	next.Seats[seatIndex(&next, u.ID)].Ready = true
	ready := true
	for _, seat := range next.Seats {
		if !seat.Bot && !seat.Left && !slices.Contains(next.ResumeJoined, seat.ID) {
			ready = false
		}
	}
	now := time.Now()
	if ready {
		next.ClockOffsetMS = now.UnixMilli() - next.SaveClock
		if next.ClockOffsetMS < 0 {
			fail(w, 400, "存档时间无效")
			return
		}
		next.Status = "playing"
		// A previous page must not re-send pre-save controls into the restored
		// session, even when the game accepts stale versions within an input phase.
		next.SetupVersion = next.Version + 1
		match := *next.Module
		if m.ResumeSaved != nil {
			match, err = m.ResumeSaved(moduleContext(&next, -1, next.Version, now), match)
			if err != nil {
				fail(w, 400, err.Error())
				return
			}
		}
		if err = s.setModule(&next, match); err != nil {
			fail(w, 400, err.Error())
			return
		}
		// Resume keeps existing challenge liability; loading must not reset it.
		started := next.RatingStarted
		s.beginClientLoading(&next, now)
		next.RatingStarted = started
	}
	next.Version++
	next.Updated, next.LastActive = now.Unix(), now.Unix()
	if err = s.save(&next); err != nil {
		fail(w, 500, "保存失败，续档未生效，请重试")
		return
	}
	s.rooms[next.ID] = &next
	s.broadcast()
	respond(w, 200, summary(&next))
}
