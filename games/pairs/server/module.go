package pairs

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const inactivityMS int64 = 120000

type Tools struct {
	Hint    int `json:"hint"`
	Shuffle int `json:"shuffle"`
	Hammer  int `json:"hammer"`
}
type Event struct {
	ID       int     `json:"id"`
	Type     string  `json:"type"`
	Path     []Point `json:"path,omitempty"`
	Pair     *Pair   `json:"pair,omitempty"`
	At       int64   `json:"at"`
	Recovery bool    `json:"recovery,omitempty"`
}
type Player struct {
	Board       *Board `json:"board"`
	Tools       Tools  `json:"tools"`
	Revision    int    `json:"revision"`
	Due         int64  `json:"due"`
	Remaining   int    `json:"remaining"`
	Combo       int    `json:"combo"`
	MaxCombo    int    `json:"maxCombo"`
	LastMatchAt int64  `json:"lastMatchAt"`
	Event       Event  `json:"event"`
}
type State struct {
	Round       string   `json:"round"`
	Seed        uint32   `json:"seed"`
	Phase       string   `json:"phase"`
	StartAt     int64    `json:"startAt"`
	FinishAt    int64    `json:"finishAt"`
	Number      int      `json:"number"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Mechanic    string   `json:"mechanic"`
	Difficulty  string   `json:"difficulty"`
	Total       int      `json:"total"`
	Players     []Player `json:"players"`
	Winner      int      `json:"winner"`
	Cursor      int      `json:"cursor"`
	Log         []string `json:"log"`
}
type View struct {
	Round       string   `json:"round"`
	Phase       string   `json:"phase"`
	StartAt     int64    `json:"startAt"`
	FinishAt    int64    `json:"finishAt"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Mechanic    string   `json:"mechanic"`
	Difficulty  string   `json:"difficulty"`
	Total       int      `json:"total"`
	Players     []Player `json:"players"`
	Winner      int      `json:"winner"`
	CanAct      bool     `json:"canAct"`
}
type Action struct {
	Type     string `json:"type"`
	Round    string `json:"round"`
	Revision int    `json:"revision"`
	A        int    `json:"a,omitempty"`
	B        int    `json:"b,omitempty"`
}

func strict(raw json.RawMessage, v any) error {
	if len(raw) > 2048 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(n int, raw json.RawMessage) (json.RawMessage, error) {
	if n < 1 || n > 6 {
		return nil, errors.New("连连看需要 1～6 人")
	}
	if len(raw) > 0 {
		if err := strict(raw, &struct{}{}); err != nil {
			return nil, errors.New("地图和难度由系统随机选择")
		}
	}
	return json.RawMessage(`{}`), nil
}
func decode(m gameapi.Match) (*State, error) {
	var s State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return nil, err
	}
	if len(s.Players) < 1 || len(s.Players) > 6 || s.Round == "" {
		return nil, errors.New("无效连连看存档")
	}
	return &s, nil
}
func (s *State) envelope() (gameapi.Match, error) {
	raw, err := json.Marshal(s)
	m := gameapi.Match{Schema: 1, Data: raw, Log: s.Log, Attention: []int{}}
	switch s.Phase {
	case "countdown":
		m.SystemDeadline = s.StartAt
	case "playing":
		for i := range s.Players {
			p := (s.Cursor + i) % len(s.Players)
			m.Actors = append(m.Actors, p)
			due := s.Players[p].Due
			if m.Deadline == 0 || due < m.Deadline {
				m.Deadline = due
			}
		}
		for p, v := range s.Players {
			if v.Due == m.Deadline {
				m.TimeoutActors = append(m.TimeoutActors, p)
			}
		}
	case "finished":
		ranks := make([]int, len(s.Players))
		for p := range ranks {
			ranks[p] = 2
		}
		ranks[s.Winner] = 1
		details, _ := json.Marshal(map[string]any{"reason": "first_clear", "elapsedMs": s.FinishAt - s.StartAt, "map": s.Name, "difficulty": s.Difficulty})
		m.Result = &gameapi.Result{Ranks: ranks, Winners: []int{s.Winner}, Details: details}
	}
	return m, err
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "pairs", Name: "连连看", MinPlayers: 1, MaxPlayers: 6, Configure: configure, Start: start, Apply: apply, View: view, BotAction: bot, Reclaim: reclaim, OnTimer: timer, AcceptStale: acceptStale}
}
func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	if _, e := configure(ctx.Players, raw); e != nil {
		return gameapi.Match{}, e
	}
	hash := sha256.Sum256([]byte(ctx.Now.Format(time.RFC3339Nano)))
	seed := binary.LittleEndian.Uint32(hash[28:])
	r := rng{seed}
	number := 1 + int(r.next()*70)
	l := GenerateLevel(number, seed)
	difficulty := "标准"
	if number < 12 || l.Mechanic == "reward" {
		difficulty = "轻松"
	} else if number >= 30 {
		difficulty = "困难"
	}
	s := &State{Round: fmt.Sprintf("%x", hash[:8]), Seed: seed, Phase: "countdown", StartAt: ctx.Now.Add(3 * time.Second).UnixMilli(), Number: number, Name: l.Name, Description: l.Description, Mechanic: l.Mechanic, Difficulty: difficulty, Total: TileCount(l.Board), Winner: -1, Players: make([]Player, ctx.Players), Log: []string{"随机地图已生成，倒计时 3 秒后同时开始；最先清空棋盘获胜"}}
	for p := range s.Players {
		b := l.Board.Clone()
		s.Players[p] = Player{Board: &b, Tools: Tools{3, 2, 2}, Remaining: s.Total, Due: s.StartAt + inactivityMS}
	}
	return s.envelope()
}
func timer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Actor != -1 || ctx.Players != len(s.Players) || s.Phase != "countdown" || ctx.Now.UnixMilli() < s.StartAt {
		return m, errors.New("尚未开始")
	}
	s.Phase = "playing"
	return s.envelope()
}
func validActor(ctx gameapi.Context, s *State) bool {
	return ctx.Actor >= 0 && ctx.Actor < len(s.Players) && ctx.Players == len(s.Players)
}
func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil || !validActor(ctx, s) || s.Phase != "playing" {
		return false
	}
	var a Action
	return strict(raw, &a) == nil && a.Round == s.Round && a.Revision == s.Players[ctx.Actor].Revision
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(raw, &a) != nil {
		return m, errors.New("无效连连看操作")
	}
	if !validActor(ctx, s) || s.Phase != "playing" || ctx.Now.UnixMilli() < s.StartAt {
		return m, errors.New("当前不能操作")
	}
	p := &s.Players[ctx.Actor]
	if a.Round != s.Round || a.Revision != p.Revision {
		return m, errors.New("棋盘已更新，请重新选择")
	}
	now := ctx.Now.UnixMilli()
	event := Event{ID: p.Event.ID + 1, Type: a.Type, At: now}
	switch a.Type {
	case "match":
		path := FindPath(*p.Board, a.A, a.B, false)
		if path == nil {
			return m, errors.New("这两个图案不能连接，路线最多转两次弯")
		}
		b, _ := RemovePair(*p.Board, a.A, a.B)
		p.Board = &b
		event.Path = path
		pair := Pair{a.A, a.B}
		event.Pair = &pair
		if p.LastMatchAt > 0 && now-p.LastMatchAt <= 3000 {
			p.Combo++
		} else {
			p.Combo = 1
		}
		p.LastMatchAt = now
		p.MaxCombo = max(p.MaxCombo, p.Combo)
	case "hint":
		if p.Tools.Hint <= 0 {
			return m, errors.New("提示已用完")
		}
		pair := FindMove(*p.Board)
		if pair == nil {
			return m, errors.New("没有可提示的配对")
		}
		p.Tools.Hint--
		event.Pair = pair
		event.Path = FindPath(*p.Board, pair[0], pair[1], false)
	case "shuffle":
		if p.Tools.Shuffle <= 0 {
			return m, errors.New("重排已用完")
		}
		p.Tools.Shuffle--
		b, _ := Reshuffle(*p.Board, s.Seed^uint32((p.Revision+1)*1777))
		p.Board = &b
		p.Combo = 0
		p.LastMatchAt = 0
	case "hammer":
		if p.Tools.Hammer <= 0 || a.A < 0 || a.A >= len(p.Board.Cells) {
			return m, errors.New("请选择有效障碍，并确认破障锤有剩余")
		}
		c := p.Board.Cells[a.A]
		if c == nil || (c.Kind == "tile" && !c.Ice) {
			return m, errors.New("破障锤只能清除石块、木箱或冰层")
		}
		if c.Kind == "tile" {
			c.Ice = false
		} else {
			p.Board.Cells[a.A] = nil
		}
		p.Tools.Hammer--
	default:
		return m, errors.New("未知连连看操作")
	}
	p.Remaining = TileCount(*p.Board)
	p.Revision++
	p.Due = now + inactivityMS
	if p.Remaining == 0 {
		s.Phase = "finished"
		s.FinishAt = now
		s.Winner = ctx.Actor
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 最先清空棋盘，用时 %.2f 秒，获得本局胜利；其余玩家并列第二", ctx.Actor+1, float64(now-s.StartAt)/1000))
	} else if FindMove(*p.Board) == nil {
		b, _ := Reshuffle(*p.Board, s.Seed+uint32(p.Revision*67))
		p.Board = &b
		event.Recovery = true
	}
	p.Event = event
	s.Cursor = (ctx.Actor + 1) % len(s.Players)
	return s.envelope()
}
func view(m gameapi.Match, actor int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	players := s.Players
	if s.Phase == "countdown" {
		for i := range players {
			players[i].Board = nil
		}
	}
	return View{Round: s.Round, Phase: s.Phase, StartAt: s.StartAt, FinishAt: s.FinishAt, Name: s.Name, Description: s.Description, Mechanic: s.Mechanic, Difficulty: s.Difficulty, Total: s.Total, Players: players, Winner: s.Winner, CanAct: actor >= 0 && actor < len(players) && s.Phase == "playing"}, nil
}
func bot(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if !validActor(ctx, s) || s.Phase != "playing" {
		return nil, nil
	}
	p := s.Players[ctx.Actor]
	pair := FindMove(*p.Board)
	if pair == nil {
		return nil, errors.New("棋盘缺少合法配对")
	}
	return json.Marshal(Action{Type: "match", Round: s.Round, Revision: p.Revision, A: pair[0], B: pair[1]})
}
func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if !validActor(ctx, s) || s.Phase != "playing" {
		return m, nil
	}
	p := &s.Players[ctx.Actor]
	if p.Due <= ctx.Now.UnixMilli() {
		p.Due = ctx.Now.UnixMilli() + inactivityMS
		return s.envelope()
	}
	return m, nil
}
