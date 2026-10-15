package gomoku

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const Size = 15

type Options struct {
	Rules       string `json:"rules"`
	Difficulty  string `json:"difficulty"`
	TurnSeconds int    `json:"turnSeconds"`
}
type Event struct {
	ID   int   `json:"id"`
	Seat int   `json:"seat"`
	X    int   `json:"x"`
	Y    int   `json:"y"`
	At   int64 `json:"at"`
}
type State struct {
	Options   Options  `json:"options"`
	Round     string   `json:"round"`
	Board     []int    `json:"board"`
	Black     int      `json:"black"`
	Turn      int      `json:"turn"`
	Ply       int      `json:"ply"`
	Deadline  int64    `json:"deadline"`
	Phase     string   `json:"phase"`
	Winner    int      `json:"winner"`
	Reason    string   `json:"reason"`
	Line      []int    `json:"line"`
	Last      Event    `json:"last"`
	Offer     int      `json:"offer"`
	LastOffer [2]int   `json:"lastOffer"`
	Log       []string `json:"log"`
}
type View struct {
	State
	CanPlace   bool `json:"canPlace"`
	CanOffer   bool `json:"canOffer"`
	CanRespond bool `json:"canRespond"`
	CanResign  bool `json:"canResign"`
}
type Action struct {
	Type   string `json:"type"`
	Round  string `json:"round"`
	Ply    int    `json:"ply"`
	X      *int   `json:"x,omitempty"`
	Y      *int   `json:"y,omitempty"`
	Accept *bool  `json:"accept,omitempty"`
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
func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	if players != 2 {
		return nil, errors.New("五子棋需要两名玩家")
	}
	o := Options{Rules: "freestyle", Difficulty: "standard", TurnSeconds: 120}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效五子棋设置")
		}
	}
	if (o.Rules != "freestyle" && o.Rules != "exact-five") || (o.Difficulty != "easy" && o.Difficulty != "standard") || o.TurnSeconds < 30 || o.TurnSeconds > 300 {
		return nil, errors.New("请选择支持的规则、电脑难度和 30～300 秒行动时限")
	}
	return json.Marshal(o)
}
func decode(m gameapi.Match) (*State, error) {
	var s State
	if m.Schema != 1 || json.Unmarshal(m.Data, &s) != nil || len(s.Board) != Size*Size || s.Round == "" || s.Black < 0 || s.Black > 1 || s.Turn < 0 || s.Turn > 1 || s.Ply < 0 || s.Ply > Size*Size || s.Offer < -1 || s.Offer > 1 {
		return nil, errors.New("无效五子棋存档")
	}
	raw, _ := json.Marshal(s.Options)
	if _, e := configure(2, raw); e != nil {
		return nil, e
	}
	counts := [3]int{}
	for _, v := range s.Board {
		if v < 0 || v > 2 {
			return nil, errors.New("无效棋子")
		}
		counts[v]++
	}
	if counts[1]+counts[2] != s.Ply || counts[1] != (s.Ply+1)/2 || counts[2] != s.Ply/2 || s.Turn != (s.Black+s.Ply)%2 {
		return nil, errors.New("棋盘和手数不一致")
	}
	if s.Phase != "playing" && s.Phase != "finished" || s.Phase == "playing" && s.Deadline <= 0 {
		return nil, errors.New("无效对局阶段")
	}
	return &s, nil
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
			r.Ranks[1-s.Winner] = 2
			r.Winners = []int{s.Winner}
		}
		r.Details, _ = json.Marshal(map[string]any{"reason": s.Reason, "plies": s.Ply, "rules": s.Options.Rules, "line": s.Line})
		m.Result = r
	}
	return m, e
}
func (s *State) colour(seat int) int {
	if seat == s.Black {
		return 1
	}
	return 2
}
func (s *State) note(line string) {
	s.Log = append(s.Log, line)
	if len(s.Log) > 80 {
		s.Log = append([]string(nil), s.Log[len(s.Log)-80:]...)
	}
}
func (s *State) end(winner int, reason string) {
	s.Phase, s.Winner, s.Reason, s.Deadline, s.Offer = "finished", winner, reason, 0, -1
}
func inBounds(x, y int) bool { return x >= 0 && y >= 0 && x < Size && y < Size }
func winningLine(board []int, x, y int, rules string) []int {
	if !inBounds(x, y) || board[y*Size+x] == 0 {
		return nil
	}
	color := board[y*Size+x]
	for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} {
		sx, sy := x, y
		for inBounds(sx-d[0], sy-d[1]) && board[(sy-d[1])*Size+sx-d[0]] == color {
			sx -= d[0]
			sy -= d[1]
		}
		line := []int{}
		for inBounds(sx, sy) && board[sy*Size+sx] == color {
			line = append(line, sy*Size+sx)
			sx += d[0]
			sy += d[1]
		}
		if len(line) == 5 || rules == "freestyle" && len(line) > 5 {
			return line
		}
	}
	return nil
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "gomoku", Name: "五子棋", MinPlayers: 2, MaxPlayers: 2, Configure: configure, Start: start, Apply: apply, View: view, BotAction: bot, Reclaim: reclaim,
		BotSearch: &gameapi.BotSearch{Input: botInput, Think: think, Budget: 500 * time.Millisecond}}
}
func start(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	o, e := configure(c.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var options Options
	_ = json.Unmarshal(o, &options)
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d", c.Now.Format(time.RFC3339Nano), c.Version, c.Actor)))
	black := int(hash[0] & 1)
	s := State{Options: options, Round: fmt.Sprintf("%x", hash[:12]), Board: make([]int, Size*Size), Black: black, Turn: black, Deadline: c.Now.Add(time.Duration(options.TurnSeconds) * time.Second).UnixMilli(), Phase: "playing", Winner: -1, Offer: -1, LastOffer: [2]int{-1, -1}, Line: []int{}, Log: []string{fmt.Sprintf("玩家 %d 执黑先行；本局为无禁手规则", black+1)}}
	if options.Rules == "exact-five" {
		s.Log[0] += "，只认恰好五子，长连不胜"
	} else {
		s.Log[0] += "，连成五子或以上获胜"
	}
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
	if a.Type != "place" && (a.X != nil || a.Y != nil) {
		return m, errors.New("此操作不包含落点")
	}
	switch a.Type {
	case "place":
		if c.Actor != s.Turn || a.X == nil || a.Y == nil || a.Accept != nil {
			return m, errors.New("请等待自己的回合并选择空落点")
		}
		x, y := *a.X, *a.Y
		if !inBounds(x, y) || s.Board[y*Size+x] != 0 {
			return m, errors.New("这个位置不能落子")
		}
		s.Board[y*Size+x] = s.colour(c.Actor)
		s.Ply++
		s.Last = Event{ID: s.Ply, Seat: c.Actor, X: x, Y: y, At: c.Now.UnixMilli()}
		s.Offer = -1
		s.Turn = 1 - s.Turn
		s.note(fmt.Sprintf("玩家 %d 落子 %c%d", c.Actor+1, 'A'+x, Size-y))
		if line := winningLine(s.Board, x, y, s.Options.Rules); line != nil {
			s.Line = line
			s.end(c.Actor, "five")
			s.note(fmt.Sprintf("玩家 %d 连成五子，获得胜利", c.Actor+1))
		} else if s.Ply == Size*Size {
			s.end(-1, "board_full")
			s.note("棋盘已满，双方和棋")
		} else {
			s.Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		}
	case "offer_draw":
		if c.Actor != s.Turn || s.Ply < 2 || s.Offer != -1 || s.LastOffer[c.Actor] == s.Ply || a.Accept != nil {
			return m, errors.New("当前不能提议和棋；每次落子前只能提议一次")
		}
		s.Offer = c.Actor
		s.LastOffer[c.Actor] = s.Ply
		s.note(fmt.Sprintf("玩家 %d 提议和棋，行动时限继续计时", c.Actor+1))
	case "respond_draw":
		if s.Offer < 0 || s.Offer == c.Actor || a.Accept == nil {
			return m, errors.New("没有需要你回应的和棋提议")
		}
		if *a.Accept {
			s.end(-1, "agreement")
			s.note("双方同意和棋")
		} else {
			s.Offer = -1
			s.note(fmt.Sprintf("玩家 %d 继续对弈", c.Actor+1))
		}
	case "resign":
		if a.Accept != nil {
			return m, errors.New("无效认输操作")
		}
		s.end(1-c.Actor, "resign")
		s.note(fmt.Sprintf("玩家 %d 认输", c.Actor+1))
	default:
		return m, errors.New("未知五子棋操作")
	}
	return s.envelope()
}
func view(m gameapi.Match, seat int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	active := s.Phase == "playing" && seat >= 0 && seat < 2
	return View{State: *s, CanPlace: active && seat == s.Turn, CanOffer: active && seat == s.Turn && s.Ply >= 2 && s.Offer == -1 && s.LastOffer[seat] != s.Ply, CanRespond: active && s.Offer >= 0 && s.Offer != seat, CanResign: active}, nil
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
