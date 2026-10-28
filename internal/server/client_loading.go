package server

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"time"
)

// Loading is a platform barrier, not an action or timer owned by a game.
// Clients are persisted for stale-tab protection but never included in views.
type ClientLoadBarrier struct {
	Token     string   `json:"token"`
	Build     string   `json:"build"`
	StartedAt int64    `json:"startedAt"`
	FrozenAt  int64    `json:"frozenAt"`
	Ready     []bool   `json:"ready"`
	Clients   []string `json:"clients"`
	Remaining int64    `json:"remaining"`
}

func (s *Server) frontendBuild() string {
	data, err := fs.ReadFile(s.files, "index.html")
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (r *Room) gameNow(now time.Time) time.Time {
	if r.Loading != nil {
		return time.UnixMilli(r.Loading.FrozenAt)
	}
	return now.Add(-time.Duration(r.ClockOffsetMS) * time.Millisecond)
}
func (r *Room) loadingView() any {
	if r.Loading == nil {
		return nil
	}
	g := r.Loading
	return map[string]any{"token": g.Token, "build": g.Build, "startedAt": g.StartedAt, "frozenAt": g.FrozenAt, "ready": append([]bool{}, g.Ready...)}
}
func (s *Server) beginClientLoading(r *Room, now time.Time) {
	if !s.cfg.ClientLoading || r.Status != "playing" || r.Game == nil || r.Game.Finished {
		return
	}
	g := &ClientLoadBarrier{Token: randomID(12), Build: s.frontendBuild(), StartedAt: now.UnixMilli(), FrozenAt: r.gameNow(now).UnixMilli(), Ready: make([]bool, len(r.Seats)), Clients: make([]string, len(r.Seats)), Remaining: max(0, r.TurnDeadline-now.UnixMilli())}
	humans := 0
	for i, seat := range r.Seats {
		g.Ready[i] = seat.Bot || seat.Left
		if !g.Ready[i] {
			humans++
		}
	}
	if humans == 0 {
		return
	}
	r.Loading = g
	// Only this first-start barrier can defer challenge liability. Reconnecting
	// to an already running match never calls beginClientLoading.
	if r.RatingPolicy != "" {
		r.RatingStarted = false
	}
	r.TurnDeadline = 0
}
func (r *Room) releaseClientLoading(now time.Time) bool {
	g := r.Loading
	if g == nil {
		return false
	}
	for i, seat := range r.Seats {
		if !seat.Left && !seat.Bot && !g.Ready[i] {
			return false
		}
	}
	r.LoadCompletedToken = g.Token
	r.Loading = nil
	if r.RatingPolicy != "" {
		r.RatingStarted = true
	}
	if r.Module != nil {
		r.ClockOffsetMS += max(0, now.UnixMilli()-g.StartedAt)
		r.TurnDeadline = r.Module.Deadline
		if r.TurnDeadline > 0 {
			r.TurnDeadline += r.ClockOffsetMS
		}
	} else if g.Remaining > 0 {
		r.TurnDeadline = now.UnixMilli() + g.Remaining
	}
	return true
}
func (s *Server) clientLoadCommand(r *Room, actor int, kind, token, build, client string, now time.Time) error {
	if actor < 0 || actor >= len(r.Seats) || r.Seats[actor].Bot || r.Seats[actor].Left || r.Status != "playing" {
		return errors.New("只有当前真人玩家可以确认加载")
	}
	g := r.Loading
	if g == nil || g.Token != token {
		return errors.New("加载阶段已更新，请读取最新状态")
	}
	if build != g.Build || build != s.frontendBuild() {
		return errors.New("页面版本已更新，请刷新后重新加载")
	}
	if len(client) < 1 || len(client) > 80 {
		return errors.New("缺少有效加载会话")
	}
	if kind == "load_begin" {
		if g.Clients[actor] != client {
			g.Clients[actor] = client
			g.Ready[actor] = false
		}
		return nil
	}
	if g.Clients[actor] != client {
		return errors.New("加载会话已更新，旧页面不能确认")
	}
	g.Ready[actor] = true
	r.releaseClientLoading(now)
	return nil
}
func (s *Server) restoreClientLoading(r *Room) (bool, error) {
	if r.ClockOffsetMS < 0 {
		return false, errors.New("无效游戏时钟偏移")
	}
	g := r.Loading
	if g == nil {
		return false, nil
	}
	if r.Status != "playing" {
		r.Loading = nil
		return true, nil
	}
	if len(g.Ready) != len(r.Seats) || len(g.Clients) != len(r.Seats) || g.StartedAt <= 0 || g.FrozenAt <= 0 || g.Token == "" {
		return false, errors.New("无效资源加载阶段")
	}
	r.TurnDeadline = 0
	if g.Build == s.frontendBuild() {
		return false, nil
	}
	g.Build = s.frontendBuild()
	g.Token = randomID(12)
	for i, seat := range r.Seats {
		g.Ready[i] = seat.Bot || seat.Left
		g.Clients[i] = ""
	}
	return true, nil
}
