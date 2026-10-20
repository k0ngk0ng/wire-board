package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type queuedRealtimeInput struct {
	request *http.Request
	userID  string
	input   streamInput
	at      time.Time
	result  chan streamAck
}
type realtimeInputQueue struct {
	pending     []*queuedRealtimeInput
	outstanding int
}

// A short per-room collection window orders concurrent inputs in memory.
// Every accepted command keeps its own nonce, version and acknowledgement.
func (s *Server) submitRealtimeInput(r *http.Request, userID string, input streamInput) streamAck {
	job := &queuedRealtimeInput{request: r, userID: userID, input: input, at: time.Now(), result: make(chan streamAck, 1)}
	s.inputMu.Lock()
	if s.inputClosing {
		s.inputMu.Unlock()
		return streamAck{Type: "ack", Nonce: input.Nonce, Status: 503, Error: "服务正在关闭，请稍后重试"}
	}
	if s.inputQueues == nil {
		s.inputQueues = make(map[string]*realtimeInputQueue)
	}
	queue := s.inputQueues[input.RoomID]
	if queue != nil && queue.outstanding >= 64 {
		s.inputMu.Unlock()
		return streamAck{Type: "ack", Nonce: input.Nonce, Status: 429, Error: "实时输入过于频繁，请稍后重试"}
	}
	fresh := queue == nil
	if fresh {
		queue = &realtimeInputQueue{}
		s.inputQueues[input.RoomID] = queue
	}
	queue.pending = append(queue.pending, job)
	queue.outstanding++
	if fresh {
		s.inputWG.Add(1)
		go s.runRealtimeInputQueue(input.RoomID, queue)
	}
	s.inputMu.Unlock()
	select {
	case ack := <-job.result:
		return ack
	case <-r.Context().Done():
		return streamAck{Type: "ack", Nonce: input.Nonce, Status: 499, Error: "连接已中断，请重新同步"}
	}
}

func (s *Server) runRealtimeInputQueue(roomID string, queue *realtimeInputQueue) {
	defer s.inputWG.Done()
	for {
		s.inputMu.Lock()
		delay := s.inputBatchDelay
		if delay <= 0 {
			delay = time.Millisecond
		}
		due := queue.pending[0].at.Add(delay)
		s.inputMu.Unlock()
		timer := time.NewTimer(time.Until(due))
		<-timer.C
		s.inputMu.Lock()
		jobs := queue.pending
		queue.pending = nil
		s.inputMu.Unlock()
		s.commitRealtimeInputs(roomID, jobs)
		s.inputMu.Lock()
		queue.outstanding -= len(jobs)
		if len(queue.pending) == 0 {
			delete(s.inputQueues, roomID)
			s.inputMu.Unlock()
			return
		}
		s.inputMu.Unlock()
	}
}

type realtimeJournalEntry struct {
	job     *queuedRealtimeInput
	version int
	record  []byte
	at      int64
}
type realtimeNonce struct{ user, nonce string }

// Called by one worker per room, in admission order. The host lock still orders
// control/lifecycle changes and simulation. Ordinary batches publish in memory;
// critical phase transitions require a transaction before any success ACK.
func (s *Server) commitRealtimeInputs(roomID string, jobs []*queuedRealtimeInput) {
	s.mu.Lock()
	defer s.mu.Unlock()
	answers := make([]streamAck, len(jobs))
	for i, job := range jobs {
		answers[i] = streamAck{Type: "ack", Nonce: job.input.Nonce, Status: 500, Error: "保存失败，操作未生效，请重试"}
	}
	defer func() {
		for i, job := range jobs {
			job.result <- answers[i]
		}
	}()
	original := s.rooms[roomID]
	candidate := original
	critical := false
	var checkpoint *realtimeCheckpoint
	if original != nil {
		checkpoint = s.realtimeCheckpoint(original)
	}
	entries := []realtimeJournalEntry{}
	duplicates := map[realtimeNonce]int{}
	dependent := map[int]int{}
	for i, job := range jobs {
		if job.request.Context().Err() != nil {
			answers[i].Status, answers[i].Error = 499, "连接已中断，请重新同步"
			continue
		}
		user, err := s.user(job.request)
		if err != nil || user.ID != job.userID {
			answers[i].Status, answers[i].Error = 401, "请先登录"
			continue
		}
		if candidate == nil {
			answers[i].Status, answers[i].Error = 404, "房间不存在"
			continue
		}
		if job.input.ServerInstance != "" && job.input.ServerInstance != s.instance {
			answers[i].Status, answers[i].Error = 409, "服务器已重启，请重新同步后操作"
			continue
		}
		key := realtimeNonce{user.ID, job.input.Nonce}
		if earlier, ok := duplicates[key]; ok {
			dependent[i] = earlier
			continue
		}
		if prior, ok := checkpoint.receipts[key]; ok {
			prior.Durable = prior.Version <= checkpoint.savedVersion
			answers[i] = prior
			continue
		}
		next, record, status, err := s.prepareRealtimeInput(candidate, user, job.input, time.Now())
		if err != nil {
			answers[i].Status, answers[i].Error = status, err.Error()
			continue
		}
		critical = critical || realtimeCritical(candidate, next)
		candidate = next
		entries = append(entries, realtimeJournalEntry{job: job, version: next.Version, record: record, at: next.Updated})
		duplicates[key] = i
		// Stage success until the whole candidate is accepted (and, if critical, saved).
		answers[i].Version = next.Version
	}
	if len(entries) > 0 {
		var err error
		if critical {
			var snapshot []byte
			snapshot, err = json.Marshal(candidate)
			if err == nil {
				err = s.persistRealtimeInputBatch(original, candidate, snapshot, entries)
			}
		}
		if err != nil {
			log.Printf("save realtime input batch room %s: %v", roomID, err)
		} else {
			s.rooms[roomID] = candidate
			if critical {
				s.realtimeSaved(candidate)
			}
			for _, index := range duplicates {
				answers[index].Status, answers[index].Error, answers[index].Durable = 200, "", critical
				job := jobs[index]
				checkpoint.remember(realtimeNonce{job.userID, job.input.Nonce}, answers[index])
			}
			s.broadcastGameChange(candidate)
		}
	}
	for index, earlier := range dependent {
		answers[index] = answers[earlier]
	}
}

func (s *Server) persistRealtimeInputBatch(before, after *Room, snapshot []byte, entries []realtimeJournalEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updated, err := tx.Exec("UPDATE rooms SET snapshot=? WHERE id=?", snapshot, after.ID)
	if err != nil {
		return err
	}
	rows, err := updated.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("durable room row missing")
	}
	if err = archiveGame(tx, before); err != nil {
		return err
	}
	if err = archiveGame(tx, after); err != nil {
		return err
	}
	for _, entry := range entries {
		if _, err = tx.Exec("INSERT INTO actions VALUES(?,?,?,?,?,?)", after.ID, entry.job.userID, entry.job.input.Nonce, entry.version, entry.record, entry.at); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Shares the ordinary module Apply/ValidateMatch path. Rechecking ownership and
// stale-version permission at commit time prevents queued controls surviving a
// takeover, departure, close or rematch. A failed callback owns only its clone.
func (s *Server) prepareRealtimeInput(room *Room, user User, input streamInput, now time.Time) (next *Room, record []byte, status int, err error) {
	status = 400
	defer func() {
		if recovered := recover(); recovered != nil {
			next = nil
			status = 500
			err = fmt.Errorf("实时操作处理失败")
		}
	}()
	module := s.modules[room.Kind]
	if !module.Realtime || room.Module == nil || room.Status != "playing" || room.Module.Result != nil {
		return nil, nil, 400, fmt.Errorf("无法执行游戏行动")
	}
	actor := seatIndex(room, user.ID)
	if actor < 0 {
		return nil, nil, 400, fmt.Errorf("只有牌桌玩家可以操作游戏")
	}
	if room.Seats[actor].AutoPlay {
		return nil, nil, 400, fmt.Errorf("当前由电脑托管，请先取消托管再手动操作")
	}
	if input.Version != room.Version {
		allowed := false
		if input.Version >= room.SetupVersion && input.Version < room.Version && module.AcceptStale != nil {
			raw, e := json.Marshal(room.Module)
			if e != nil {
				return nil, nil, 500, e
			}
			var match gameapi.Match
			if e = json.Unmarshal(raw, &match); e != nil {
				return nil, nil, 500, e
			}
			allowed = module.AcceptStale(moduleContext(room, actor, input.Version, now), match, input.Action)
		}
		if !allowed {
			return nil, nil, 409, fmt.Errorf("局面已更新，请根据最新画面重试")
		}
	}
	next, err = cloneBotRoom(room)
	if err != nil {
		return nil, nil, 500, err
	}
	record, err = json.Marshal(map[string]any{"type": "action", "nonce": input.Nonce, "version": input.Version, "action": input.Action, "appliedAt": now.UnixMilli()})
	if err != nil {
		return nil, nil, 400, err
	}
	if err = s.applyModule(next, actor, input.Version, input.Action, now); err != nil {
		return nil, nil, 400, err
	}
	next.Version++
	next.BotAt = now.Add(900 * time.Millisecond).UnixMilli()
	next.Updated, next.LastActive = now.Unix(), now.Unix()
	return next, record, 200, nil
}
