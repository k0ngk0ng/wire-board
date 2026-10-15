package xiangqi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"time"
)

type Options struct {
	Difficulty  string `json:"difficulty"`
	TurnSeconds int    `json:"turnSeconds"`
}
type Event struct {
	ID      int   `json:"id"`
	Seat    int   `json:"seat"`
	From    int   `json:"from"`
	To      int   `json:"to"`
	Piece   int   `json:"piece"`
	Capture int   `json:"capture"`
	Check   bool  `json:"check"`
	At      int64 `json:"at"`
}
type State struct {
	Options   Options  `json:"options"`
	Round     string   `json:"round"`
	Red       int      `json:"red"`
	Turn      int      `json:"turn"`
	Ply       int      `json:"ply"`
	Board     [90]int  `json:"board"`
	IDs       [90]int  `json:"ids"`
	History   []Record `json:"history"`
	NoCapture int      `json:"noCapture"`
	Phase     string   `json:"phase"`
	Deadline  int64    `json:"deadline"`
	Winner    int      `json:"winner"`
	Reason    string   `json:"reason"`
	Warning   string   `json:"warning"`
	Offer     int      `json:"offer"`
	Captured  [2][]int `json:"captured"`
	Last      Event    `json:"last"`
	Moves     []Move   `json:"moves"`
	Log       []string `json:"log"`
}
type View struct {
	State
	Legal       []Move `json:"legal"`
	CheckSquare int    `json:"checkSquare"`
	CanMove     bool   `json:"canMove"`
	CanRespond  bool   `json:"canRespond"`
	CanResign   bool   `json:"canResign"`
}
type Action struct {
	Type      string `json:"type"`
	Round     string `json:"round"`
	Ply       int    `json:"ply"`
	From      *int   `json:"from,omitempty"`
	To        *int   `json:"to,omitempty"`
	OfferDraw bool   `json:"offerDraw,omitempty"`
	Accept    *bool  `json:"accept,omitempty"`
}

func strict(raw []byte, v any) error {
	if len(raw) > 4096 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(n int, raw json.RawMessage) (json.RawMessage, error) {
	if n != 2 {
		return nil, errors.New("中国象棋需要两名玩家")
	}
	o := Options{"standard", 120}
	if len(raw) > 0 {
		if strict(raw, &o) != nil {
			return nil, errors.New("无效中国象棋设置")
		}
	}
	if (o.Difficulty != "easy" && o.Difficulty != "standard") || o.TurnSeconds < 30 || o.TurnSeconds > 300 {
		return nil, errors.New("请选择支持的电脑难度和 30～300 秒行动时限")
	}
	return json.Marshal(o)
}
func (s *State) color(seat int) int {
	if seat == s.Red {
		return 1
	}
	return -1
}
func decode(m gameapi.Match) (*State, error) {
	var s State
	if m.Schema != 1 || json.Unmarshal(m.Data, &s) != nil || s.Round == "" || s.Red < 0 || s.Red > 1 || s.Turn < 0 || s.Turn > 1 || s.Ply < 0 || len(s.Moves) != s.Ply || len(s.History) == 0 || len(s.History) > 101 || s.NoCapture < 0 || s.NoCapture > 100 || s.Offer < -1 || s.Offer > 1 {
		return nil, errors.New("无效中国象棋存档")
	}
	if s.Phase != "playing" && s.Phase != "finished" || s.Phase == "playing" && s.Deadline <= 0 {
		return nil, errors.New("无效对局阶段")
	}
	o, _ := json.Marshal(s.Options)
	if _, e := configure(2, o); e != nil {
		return nil, e
	}
	kings := [2]int{}
	ids := map[int]bool{}
	for i, p := range s.Board {
		if abs(p) > Pawn {
			return nil, errors.New("无效棋子")
		}
		if p != 0 {
			if s.IDs[i] <= 0 || ids[s.IDs[i]] {
				return nil, errors.New("棋子标识重复")
			}
			ids[s.IDs[i]] = true
		}
		if p == King {
			kings[0]++
		}
		if p == -King {
			kings[1]++
		}
	}
	if kings != [2]int{1, 1} || s.History[len(s.History)-1].Key != PositionKey(s.Board, s.Turn) {
		return nil, errors.New("将帅或局面历史不一致")
	}
	return &s, nil
}
func (s *State) note(text string) {
	s.Log = append(s.Log, text)
	if len(s.Log) > 80 {
		s.Log = append([]string(nil), s.Log[len(s.Log)-80:]...)
	}
}
func (s *State) end(winner int, reason string) {
	s.Phase, s.Winner, s.Reason, s.Deadline, s.Offer = "finished", winner, reason, 0, -1
	labels := map[string]string{
		"checkmate": "将杀", "stalemate_loss": "困毙，无法走子方判负",
		"dead_position": "双方无法取胜，和棋", "perpetual_check": "长将未变着，责任方判负",
		"perpetual_chase": "长捉未变着，责任方判负", "repetition": "循环局面和棋",
		"fifty_move": "连续 50 回合没有吃子，和棋", "agreement": "双方同意和棋", "resign": "认输",
	}
	s.note("对局结束：" + labels[reason])
}
func (s *State) envelope() (gameapi.Match, error) {
	raw, e := json.Marshal(s)
	m := gameapi.Match{Schema: 1, Data: raw, Log: append([]string(nil), s.Log...)}
	if s.Phase == "playing" {
		m.Actors = []int{0, 1}
		m.Attention = []int{s.Turn}
		m.TimeoutActors = []int{s.Turn}
		m.Deadline = s.Deadline
	} else {
		r := &gameapi.Result{Ranks: []int{1, 1}, Winners: []int{}}
		if s.Winner < 0 {
			r.Draw = true
		} else {
			r.Winners = []int{s.Winner}
			r.Ranks[1-s.Winner] = 2
		}
		r.Details, _ = json.Marshal(map[string]any{"reason": s.Reason, "plies": s.Ply, "noCapture": s.NoCapture})
		m.Result = r
	}
	return m, e
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "xiangqi", Name: "中国象棋", MinPlayers: 2, MaxPlayers: 2, Configure: configure, Start: start, Apply: apply, View: view, Reclaim: reclaim, BotAction: bot, BotSearch: &gameapi.BotSearch{Input: botInput, Think: think, Budget: 600 * time.Millisecond}}
}
func start(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	o, e := configure(c.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var opts Options
	_ = json.Unmarshal(o, &opts)
	hash := sha256.Sum256([]byte(fmt.Sprintf("xiangqi/%s/%d/%d", c.Now.Format(time.RFC3339Nano), c.Version, c.Actor)))
	red := int(hash[0] & 1)
	s := State{Options: opts, Round: fmt.Sprintf("%x", hash[:12]), Red: red, Turn: red, Phase: "playing", Winner: -1, Offer: -1, Deadline: c.Now.Add(time.Duration(opts.TurnSeconds) * time.Second).UnixMilli(), Last: Event{From: -1, To: -1}, Captured: [2][]int{{}, {}}, Moves: []Move{}, Log: []string{fmt.Sprintf("玩家 %d 执红先行；休闲每步计时，超时托管", red+1)}}
	back := [9]int{Rook, Horse, Elephant, Advisor, King, Advisor, Elephant, Horse, Rook}
	for x, p := range back {
		s.Board[x] = -p
		s.Board[81+x] = p
	}
	for _, x := range []int{1, 7} {
		s.Board[18+x] = -Cannon
		s.Board[63+x] = Cannon
	}
	for _, x := range []int{0, 2, 4, 6, 8} {
		s.Board[27+x] = -Pawn
		s.Board[54+x] = Pawn
	}
	id := 1
	for i, p := range s.Board {
		if p != 0 {
			s.IDs[i] = id
			id++
		}
	}
	s.History = []Record{{Key: PositionKey(s.Board, s.Turn), Seat: -1}}
	return s.envelope()
}
func apply(c gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(raw, &a) != nil || c.Players != 2 || c.Actor < 0 || c.Actor > 1 || s.Phase != "playing" {
		return m, errors.New("当前不能操作")
	}
	if a.Round != s.Round || a.Ply != s.Ply {
		return m, errors.New("棋盘已更新，请重新操作")
	}
	if a.Type != "move" && (a.From != nil || a.To != nil || a.OfferDraw) || a.Type != "respond_draw" && a.Accept != nil {
		return m, errors.New("无效操作字段")
	}
	switch a.Type {
	case "move":
		if c.Actor != s.Turn || a.From == nil || a.To == nil || *a.From < 0 || *a.From >= 90 || *a.To < 0 || *a.To >= 90 {
			return m, errors.New("请选择自己的合法着法")
		}
		mv := Move{*a.From, *a.To}
		valid := false
		for _, candidate := range legal(s.Board, s.color(c.Actor)) {
			if sameMove(candidate, mv) {
				valid = true
				break
			}
		}
		if !valid {
			return m, errors.New("这一步不合法，不能送将或违反棋子走法")
		}
		before, oldIDs := s.Board, s.IDs
		previous := s.History[len(s.History)-1]
		capture := s.Board[mv.To]
		pc := s.Board[mv.From]
		s.Board = moved(s.Board, mv)
		s.IDs[mv.To], s.IDs[mv.From] = s.IDs[mv.From], 0
		s.Ply++
		s.NoCapture++
		s.Moves = append(s.Moves, mv)
		s.Offer = -1
		if capture != 0 {
			s.NoCapture = 0
			s.History = nil
			s.Captured[c.Actor] = append(s.Captured[c.Actor], capture)
		}
		check := checked(s.Board, -s.color(c.Actor))
		chases := newChases(before, s.Board, oldIDs, s.IDs, s.color(c.Actor), oldIDs[mv.From])
		avoided := evaded(previous, s.Board, s.IDs, -s.color(c.Actor))
		s.Turn = 1 - s.Turn
		s.History = append(s.History, Record{Key: PositionKey(s.Board, s.Turn), Seat: c.Actor, Check: check, Chases: chases, Evaded: avoided})
		s.Last = Event{s.Ply, c.Actor, mv.From, mv.To, pc, capture, check, c.Now.UnixMilli()}
		s.Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		s.note(fmt.Sprintf("玩家 %d：%s → %s", c.Actor+1, label(mv.From), label(mv.To)))
		settle(s)
		if s.Phase == "playing" && a.OfferDraw {
			s.Offer = c.Actor
			s.note("随走子提议和棋")
		}
	case "respond_draw":
		if a.Accept == nil || s.Offer < 0 || s.Offer == c.Actor {
			return m, errors.New("没有需要你回应的和棋提议")
		}
		if *a.Accept {
			s.end(-1, "agreement")
		} else {
			s.Offer = -1
			s.note("和棋提议已拒绝，继续对弈")
		}
	case "resign":
		s.end(1-c.Actor, "resign")
	default:
		return m, errors.New("未知中国象棋操作")
	}
	return s.envelope()
}
func label(sq int) string { return fmt.Sprintf("%c%d", 'a'+sq%9, 9-sq/9) }
func view(m gameapi.Match, actor int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	v := View{State: *s, Legal: []Move{}, CheckSquare: -1}
	color := s.color(s.Turn)
	if checked(s.Board, color) {
		for i, p := range s.Board {
			if p == color*King {
				v.CheckSquare = i
				break
			}
		}
	}
	if s.Phase == "playing" {
		v.Legal = legal(s.Board, color)
		active := actor >= 0 && actor < 2
		v.CanMove = active && actor == s.Turn
		v.CanResign = active
		v.CanRespond = active && s.Offer >= 0 && s.Offer != actor
	}
	return v, nil
}
func reclaim(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if s.Phase == "playing" && c.Actor == s.Turn && s.Deadline <= c.Now.UnixMilli() {
		s.Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		return s.envelope()
	}
	return m, nil
}
