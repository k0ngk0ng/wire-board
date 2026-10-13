package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/game"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"golang.org/x/crypto/bcrypt"
)

func (s *Server) createModule(w http.ResponseWriter, user User, module gameapi.Module, name, password string, capacity int, raw json.RawMessage) {
	if capacity < module.MinPlayers || capacity > module.MaxPlayers {
		fail(w, 400, "人数不符合游戏要求")
		return
	}
	options, err := module.Configure(capacity, raw)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	if !json.Valid(options) || string(options) == "null" {
		fail(w, 400, "无效游戏设置")
		return
	}
	hash := ""
	if password != "" {
		value, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			fail(w, 500, "无法设置房间密码")
			return
		}
		hash = string(value)
	}
	now := time.Now().Unix()
	r := &Room{ID: randomID(4), Name: name, Kind: module.ID, Options: options, Host: user.ID, Capacity: capacity, Seats: []Seat{{User: user}}, Version: 1, Status: "waiting", Password: hash, Updated: now, LastActive: now}
	if err = s.save(r); err != nil {
		fail(w, 500, "无法保存房间")
		return
	}
	s.rooms[r.ID] = r
	s.broadcast()
	respond(w, 201, summary(r))
}

func (s *Server) initModules() error {
	s.modules = map[string]gameapi.Module{}
	s.gameNames = map[string]string{}
	s.gameKinds = append([]string(nil), gameKinds...)
	for k, v := range gameNames {
		s.gameNames[k] = v
	}
	for _, module := range s.cfg.Modules {
		if err := module.Validate(); err != nil {
			return err
		}
		if s.gameNames[module.ID] != "" {
			return fmt.Errorf("duplicate game registration: %s", module.ID)
		}
		s.modules[module.ID], s.gameNames[module.ID] = module, module.Name
		s.gameKinds = append(s.gameKinds, module.ID)
	}
	return nil
}

func moduleContext(r *Room, actor, version int, now time.Time) gameapi.Context {
	return gameapi.Context{Actor: actor, Players: len(r.Seats), Version: r.Version, RequestedVersion: version, Now: now}
}

func (r *Room) setModule(match gameapi.Match) error {
	if err := match.Validate(len(r.Seats)); err != nil {
		return err
	}
	for actor, seat := range r.Seats {
		if seat.Left && slices.Contains(match.Actors, actor) {
			return errors.New("departed player cannot act")
		}
	}
	if match.Log == nil {
		match.Log = []string{}
	}
	r.Module = &match
	r.TurnDeadline = match.Deadline
	// The old room envelope remains readable by platform history/chat code.
	// No game-specific rule or payload is placed in this compatibility header.
	r.Game = &game.State{Kind: r.Kind, Phase: "module", Turn: -1, Round: 1, Log: match.Log, Winners: []int{}}
	if len(match.Actors) != 0 {
		r.Game.Turn = match.Actors[0]
	}
	if match.Result != nil {
		r.Game.Finished, r.Game.Winners = true, match.Result.Winners
		r.Status = "finished"
	}
	return nil
}

func (s *Server) startModule(r *Room, actor int, now time.Time) error {
	m := s.modules[r.Kind]
	if len(r.Seats) < m.MinPlayers || len(r.Seats) > m.MaxPlayers {
		return errors.New("游戏人数不符合要求")
	}
	options, err := m.Configure(len(r.Seats), r.Options)
	if err != nil {
		return err
	}
	if !json.Valid(options) || string(options) == "null" {
		return errors.New("无效游戏设置")
	}
	r.Options = options
	match, err := m.Start(moduleContext(r, actor, r.Version, now), r.Options)
	if err != nil {
		return err
	}
	if err = m.ValidateMatch(match, len(r.Seats)); err != nil {
		return err
	}
	r.Status = "playing"
	if err = r.setModule(match); err != nil {
		return err
	}
	r.MatchID, r.SetupVersion, r.Rated = randomID(12), r.Version+1, true
	for _, seat := range r.Seats {
		if seat.Bot {
			r.Rated = false
		}
	}
	return nil
}

func (s *Server) applyModule(r *Room, actor, version int, action json.RawMessage, now time.Time) error {
	if r.Module == nil || r.Module.Result != nil || !slices.Contains(r.Module.Actors, actor) {
		return errors.New("当前不能执行游戏行动")
	}
	match, err := s.modules[r.Kind].Apply(moduleContext(r, actor, version, now), *r.Module, action)
	if err != nil {
		return err
	}
	if err = s.modules[r.Kind].ValidateMatch(match, len(r.Seats)); err != nil {
		return err
	}
	return r.setModule(match)
}

func (s *Server) moduleView(r *Room, viewer int) (map[string]any, error) {
	// View callbacks receive a detached snapshot, just like action callbacks.
	raw, err := json.Marshal(r.Module)
	if err != nil {
		return nil, err
	}
	var match gameapi.Match
	if err = json.Unmarshal(raw, &match); err != nil {
		return nil, err
	}
	data, err := s.modules[r.Kind].View(match, viewer)
	if err != nil {
		return nil, err
	}
	attention := match.Attention
	if attention == nil {
		attention = match.Actors
	}
	return map[string]any{"kind": r.Kind, "data": data, "actors": match.Actors, "attention": attention, "eliminated": match.Eliminated, "systemDeadline": match.SystemDeadline, "finished": match.Result != nil, "winners": r.Game.Winners, "log": r.Game.Log, "phase": "module", "turn": r.Game.Turn, "round": 1}, nil
}

// Called under s.mu, before player timeouts. System events neither enable
// autoplay nor renew player activity. A failed callback or commit is retryable
// from the unchanged persisted snapshot on the next tick.
func (s *Server) expireModuleTimers(now time.Time) {
	for _, room := range s.rooms {
		if room.Status != "playing" || room.Module == nil || room.Module.Result != nil || room.Module.SystemDeadline == 0 || room.Module.SystemDeadline > now.UnixMilli() {
			continue
		}
		if err := s.runModuleTimer(room, now); err != nil {
			log.Printf("module timer room %s: %v", room.ID, err)
		}
	}
}

func (s *Server) runModuleTimer(room *Room, now time.Time) error {
	m := s.modules[room.Kind]
	if m.OnTimer == nil {
		return errors.New("missing system timer callback")
	}
	raw, err := json.Marshal(room)
	if err != nil {
		return err
	}
	var next Room
	if err = json.Unmarshal(raw, &next); err != nil {
		return err
	}
	due := next.Module.SystemDeadline
	match, err := m.OnTimer(moduleContext(&next, -1, next.Version, now), *next.Module)
	if err != nil {
		return err
	}
	if match.SystemDeadline != 0 && match.SystemDeadline <= now.UnixMilli() {
		return errors.New("system timer must clear or advance its deadline")
	}
	if err = m.ValidateMatch(match, len(next.Seats)); err != nil {
		return err
	}
	if err = next.setModule(match); err != nil {
		return err
	}
	next.Version++
	// A recurring system clock must not postpone an already scheduled bot.
	// The bot scheduler keeps its own pacing after an actual player decision.
	next.Updated = now.Unix()
	snapshot, err := json.Marshal(&next)
	if err != nil {
		return err
	}
	record, err := json.Marshal(map[string]any{"type": "system_timer", "deadline": due})
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("UPDATE rooms SET snapshot=? WHERE id=?", snapshot, next.ID); err != nil {
		return err
	}
	if err = archiveGame(tx, &next); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO actions VALUES(?,?,?,?,?,?)", next.ID, "", fmt.Sprintf("timer-%d", next.Version), next.Version, record, now.Unix()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.rooms[next.ID] = &next
	s.broadcast()
	return nil
}

func (s *Server) runModuleBot(room *Room, now time.Time, expired bool) {
	m := s.modules[room.Kind]
	if m.BotAction == nil {
		return
	}
	raw, err := json.Marshal(room)
	if err != nil {
		return
	}
	var next Room
	if err = json.Unmarshal(raw, &next); err != nil {
		return
	}
	actors := next.Module.Actors
	if expired {
		actors = next.Module.TimeoutActors
		for _, actor := range actors {
			seat := &next.Seats[actor]
			if !seat.Bot && !seat.AutoPlay {
				seat.AutoPlay, seat.TimeoutAutoPlay = true, true
			}
		}
	}
	actor := -1
	var action json.RawMessage
	for _, candidate := range actors {
		if !next.Seats[candidate].computerControlled() {
			continue
		}
		proposal, err := m.BotAction(moduleContext(&next, candidate, next.Version, now), *next.Module)
		if err != nil {
			log.Printf("module bot room %s: %v", room.ID, err)
			return
		}
		if proposal == nil {
			continue
		}
		actor, action = candidate, proposal
		break
	}
	if actor < 0 {
		return
	}
	err = s.applyModule(&next, actor, next.Version, action, now)
	if err != nil {
		log.Printf("module bot room %s: %v", room.ID, err)
		return
	}
	next.Version++
	next.Updated, next.BotAt = now.Unix(), now.Add(900*time.Millisecond).UnixMilli()
	if !expired && !next.Seats[actor].TimeoutAutoPlay {
		next.LastActive = next.Updated
	}
	snapshot, err := json.Marshal(&next)
	if err != nil {
		return
	}
	record, err := json.Marshal(map[string]any{"type": "action", "action": json.RawMessage(action), "bot": next.Seats[actor].Bot, "autoPlay": next.Seats[actor].AutoPlay})
	if err != nil {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	_, err = tx.Exec("UPDATE rooms SET snapshot=? WHERE id=?", snapshot, next.ID)
	if err == nil {
		err = archiveGame(tx, &next)
	}
	if err == nil {
		_, err = tx.Exec("INSERT INTO actions VALUES(?,?,?,?,?,?)", next.ID, next.Seats[actor].ID, fmt.Sprintf("bot-%d", next.Version), next.Version, record, now.Unix())
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		log.Printf("save module bot room %s: %v", room.ID, err)
		return
	}
	s.rooms[next.ID] = &next
	s.broadcast()
}
