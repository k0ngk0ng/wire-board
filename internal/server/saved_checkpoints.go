package server

import (
	"errors"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type SavedCheckpoint struct {
	Format  int           `json:"format"`
	MatchID string        `json:"matchId"`
	Match   gameapi.Match `json:"match"`
	Clock   int64         `json:"clock"`
	SavedAt int64         `json:"savedAt"`
}

func withCheckpointSummary(r *Room, v map[string]any) map[string]any {
	if r.SupportsCheckpoint {
		v["supportsCheckpoint"] = true
		if r.Checkpoint != nil && r.Checkpoint.MatchID == r.MatchID {
			v["checkpointSavedAt"] = r.Checkpoint.SavedAt
		}
	}
	return v
}

func (s *Server) captureCheckpoint(r *Room) error {
	if !s.modules[r.Kind].SupportsCheckpoint {
		return nil
	}
	if r.Module == nil || !r.Module.Unrated || r.Module.Result != nil || len(r.Seats) != 1 || r.Seats[0].Bot || r.Seats[0].Left || r.MatchID == "" || r.SaveClock <= 0 || r.SavedAt <= 0 {
		return errors.New("当前对局不支持回退存档")
	}
	r.SupportsCheckpoint = true
	if r.Module.PreserveSavedCheckpoint {
		return nil
	}
	r.Checkpoint = &SavedCheckpoint{Format: 1, MatchID: r.MatchID, Match: cloneModuleMatch(*r.Module), Clock: r.SaveClock, SavedAt: r.SavedAt}
	return nil
}

func (s *Server) loadCheckpoint(r *Room, actor int, now time.Time) error {
	m := s.modules[r.Kind]
	if !m.SupportsCheckpoint || !r.SupportsCheckpoint || !r.SupportsSave || m.RestoreCheckpoint == nil || r.Status != "playing" || r.Module == nil || !r.Module.Unrated || r.Module.Result != nil || r.Loading != nil || len(r.SaveVotes) != 0 {
		return errors.New("当前对局不能读取手动存档")
	}
	if actor != 0 || len(r.Seats) != 1 || r.Seats[0].Bot || r.Seats[0].Left {
		return errors.New("只有本人可以读取手动存档")
	}
	point := r.Checkpoint
	if point == nil {
		return errors.New("尚无手动存档")
	}
	if point.Format != 1 || point.MatchID == "" || point.MatchID != r.MatchID || point.Clock <= 0 || point.SavedAt <= 0 || !point.Match.Unrated || point.Match.Result != nil {
		return errors.New("手动存档无效，原局面已保留")
	}
	if err := m.ValidateMatch(point.Match, 1); err != nil {
		return err
	}
	r.ClockOffsetMS = now.UnixMilli() - point.Clock
	if r.ClockOffsetMS < 0 {
		return errors.New("存档时间无效")
	}
	match, err := m.RestoreCheckpoint(moduleContext(r, actor, r.Version, now), cloneModuleMatch(point.Match), cloneModuleMatch(*r.Module))
	if err != nil {
		return err
	}
	if match.Result != nil {
		return errors.New("读取存档不能直接结算对局")
	}
	if err = s.setModule(r, match); err != nil {
		return err
	}
	r.SetupVersion = r.Version + 1
	r.SaveVotes, r.ResumeJoined = nil, nil
	r.Seats[0].AutoPlay, r.Seats[0].TimeoutAutoPlay = false, false
	started := r.RatingStarted
	s.beginClientLoading(r, now)
	r.RatingStarted = started
	return nil
}

func (s *Server) restartCheckpointJourney(r *Room, actor int, now time.Time) error {
	m := s.modules[r.Kind]
	if !m.SupportsCheckpoint || !r.SupportsCheckpoint || r.Status != "playing" || r.Module == nil || !r.Module.Unrated || r.Module.Result != nil || !r.Module.Restartable || !r.Module.PreserveSavedCheckpoint || r.Loading != nil || len(r.SaveVotes) != 0 {
		return errors.New("当前旅程不能重新开始")
	}
	if actor != 0 || len(r.Seats) != 1 || r.Seats[0].Bot || r.Seats[0].Left {
		return errors.New("只有本人可以重新开始旅程")
	}
	r.ClockOffsetMS, r.SaveClock, r.SavedAt = 0, 0, 0
	r.SaveVotes, r.ResumeJoined = nil, nil
	r.LoadCompletedToken, r.CloseReason = "", ""
	r.Seats[0].AutoPlay, r.Seats[0].TimeoutAutoPlay = false, false
	if err := s.startModule(r, actor, now); err != nil {
		return err
	}
	s.beginClientLoading(r, now)
	return nil
}
