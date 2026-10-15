package chess

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	engine "github.com/corentings/chess/v2"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const initialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type Options struct {
	Difficulty  string `json:"difficulty"`
	TurnSeconds int    `json:"turnSeconds"`
}
type Event struct {
	ID        int    `json:"id"`
	Seat      int    `json:"seat"`
	From      int    `json:"from"`
	To        int    `json:"to"`
	Piece     string `json:"piece"`
	Capture   string `json:"capture"`
	CaptureAt int    `json:"captureAt"`
	RookFrom  int    `json:"rookFrom"`
	RookTo    int    `json:"rookTo"`
	Promotion string `json:"promotion"`
	Check     bool   `json:"check"`
	At        int64  `json:"at"`
}
type State struct {
	Options    Options     `json:"options"`
	Round      string      `json:"round"`
	White      int         `json:"white"`
	Turn       int         `json:"turn"`
	Ply        int         `json:"ply"`
	FEN        string      `json:"fen"`
	InitialFEN string      `json:"initialFen"`
	Keys       []string    `json:"keys"`
	UCI        []string    `json:"uci"`
	SAN        []string    `json:"san"`
	Phase      string      `json:"phase"`
	Deadline   int64       `json:"deadline"`
	Winner     int         `json:"winner"`
	Reason     string      `json:"reason"`
	ClaimMove  string      `json:"claimMove"`
	Offer      int         `json:"offer"`
	Captured   [2][]string `json:"captured"`
	Last       Event       `json:"last"`
	Log        []string    `json:"log"`
}
type LegalMove struct {
	UCI       string `json:"uci"`
	From      int    `json:"from"`
	To        int    `json:"to"`
	Promotion string `json:"promotion,omitempty"`
	Capture   bool   `json:"capture"`
	Castle    bool   `json:"castle"`
}
type View struct {
	State
	Board       []string            `json:"board"`
	Legal       []LegalMove         `json:"legal"`
	CheckSquare int                 `json:"checkSquare"`
	Claims      []string            `json:"claims"`
	ClaimMoves  map[string][]string `json:"claimMoves"`
	CanMove     bool                `json:"canMove"`
	CanRespond  bool                `json:"canRespond"`
	CanResign   bool                `json:"canResign"`
	PGN         string              `json:"pgn"`
}
type Action struct {
	Type      string `json:"type"`
	Round     string `json:"round"`
	Ply       int    `json:"ply"`
	UCI       string `json:"uci,omitempty"`
	Reason    string `json:"reason,omitempty"`
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
		return nil, errors.New("国际象棋需要两名玩家")
	}
	o := Options{Difficulty: "standard", TurnSeconds: 120}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效国际象棋设置")
		}
	}
	if (o.Difficulty != "easy" && o.Difficulty != "standard") || o.TurnSeconds < 30 || o.TurnSeconds > 300 {
		return nil, errors.New("请选择支持的电脑难度和 30～300 秒行动时限")
	}
	return json.Marshal(o)
}
func parse(fen string) (g *engine.Game, err error) {
	defer func() {
		if recover() != nil {
			g = nil
			err = errors.New("无效国际象棋局面")
		}
	}()
	f, e := engine.FEN(fen)
	if e != nil {
		return nil, e
	}
	g = engine.NewGame(f)
	kings := [3]int{}
	for sq, p := range g.Position().Board().SquareMap() {
		if p.Type() == engine.King {
			kings[p.Color()]++
		}
		if p.Type() == engine.Pawn && (sq.Rank() == engine.Rank1 || sq.Rank() == engine.Rank8) {
			return nil, errors.New("末行兵必须升变")
		}
	}
	if kings[engine.White] != 1 || kings[engine.Black] != 1 {
		return nil, errors.New("每方必须保留一个王")
	}
	return g, nil
}
func decode(m gameapi.Match) (*State, *engine.Game, error) {
	var s State
	if m.Schema != 1 || json.Unmarshal(m.Data, &s) != nil || s.Round == "" || s.White < 0 || s.White > 1 || s.Turn < 0 || s.Turn > 1 || s.Ply < 0 || len(s.UCI) != s.Ply || len(s.SAN) != s.Ply || len(s.Keys) == 0 || len(s.Keys) > 151 || s.Offer < -1 || s.Offer > 1 {
		return nil, nil, errors.New("无效国际象棋存档")
	}
	if s.Phase != "playing" && s.Phase != "finished" || s.Phase == "playing" && s.Deadline <= 0 {
		return nil, nil, errors.New("无效对局阶段")
	}
	raw, _ := json.Marshal(s.Options)
	if _, e := configure(2, raw); e != nil {
		return nil, nil, e
	}
	g, e := parse(s.FEN)
	if e != nil {
		return nil, nil, e
	}
	if s.Turn != s.seat(g.Position().Turn()) || s.Keys[len(s.Keys)-1] != positionKey(g.Position()) {
		return nil, nil, errors.New("手番或重复局面历史不一致")
	}
	return &s, g, nil
}
func (s *State) colour(seat int) engine.Color {
	if seat == s.White {
		return engine.White
	}
	return engine.Black
}
func (s *State) seat(color engine.Color) int {
	if color == engine.White {
		return s.White
	}
	return 1 - s.White
}
func pieceCode(p engine.Piece) string {
	if p == engine.NoPiece {
		return ""
	}
	return p.Color().String() + strings.ToUpper(p.Type().String())
}
func (s *State) note(text string) {
	s.Log = append(s.Log, text)
	if len(s.Log) > 80 {
		s.Log = append([]string(nil), s.Log[len(s.Log)-80:]...)
	}
}
func (s *State) end(winner int, reason string) {
	s.Phase, s.Winner, s.Reason, s.Deadline, s.Offer = "finished", winner, reason, 0, -1
}
func (s *State) pgn() string {
	result := "*"
	if s.Phase == "finished" {
		result = "1/2-1/2"
		if s.Winner == s.White {
			result = "1-0"
		} else if s.Winner >= 0 {
			result = "0-1"
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Event \"围桌国际象棋\"]\n[Result \"%s\"]\n", result)
	if s.InitialFEN != initialFEN {
		fmt.Fprintf(&b, "[SetUp \"1\"]\n[FEN %q]\n", s.InitialFEN)
	}
	b.WriteByte('\n')
	for i, m := range s.SAN {
		if i%2 == 0 {
			fmt.Fprintf(&b, "%d. ", i/2+1)
		}
		b.WriteString(m + " ")
	}
	b.WriteString(result)
	return b.String()
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
		r.Details, _ = json.Marshal(map[string]any{"reason": s.Reason, "fen": s.FEN, "plies": s.Ply, "claimMove": s.ClaimMove, "pgn": s.pgn()})
		m.Result = r
	}
	return m, e
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "chess", Name: "国际象棋", MinPlayers: 2, MaxPlayers: 2, Configure: configure, Start: start, Apply: apply, View: view, BotAction: bot, Reclaim: reclaim, BotSearch: &gameapi.BotSearch{Input: botInput, Think: think, Budget: 500 * time.Millisecond}}
}
func start(c gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	o, e := configure(c.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var opts Options
	_ = json.Unmarshal(o, &opts)
	hash := sha256.Sum256([]byte(fmt.Sprintf("chess/%s/%d/%d", c.Now.Format(time.RFC3339Nano), c.Version, c.Actor)))
	white := int(hash[0] & 1)
	g := engine.NewGame()
	s := State{Options: opts, Round: fmt.Sprintf("%x", hash[:12]), White: white, Turn: white, FEN: initialFEN, InitialFEN: initialFEN, Keys: []string{positionKey(g.Position())}, UCI: []string{}, SAN: []string{}, Phase: "playing", Deadline: c.Now.Add(time.Duration(opts.TurnSeconds) * time.Second).UnixMilli(), Winner: -1, Offer: -1, Last: Event{From: -1, To: -1, CaptureAt: -1, RookFrom: -1, RookTo: -1}, Captured: [2][]string{{}, {}}, Log: []string{fmt.Sprintf("玩家 %d 执白先行；标准国际象棋，休闲每步计时，超时托管", white+1)}}
	return s.envelope()
}
func isCapture(m *engine.Move) bool {
	return m.HasTag(engine.Capture) || m.HasTag(engine.EnPassant)
}
func legalMove(p *engine.Position, uci string) (*engine.Move, error) {
	for _, m := range p.ValidMoves() {
		if m.String() == uci {
			return &m, nil
		}
	}
	return nil, errors.New("这一步不合法；升变必须选择后、车、象或马")
}
func apply(c gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, g, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(raw, &a) != nil || c.Actor < 0 || c.Actor > 1 || c.Players != 2 || s.Phase != "playing" {
		return m, errors.New("当前不能操作")
	}
	if a.Round != s.Round || a.Ply != s.Ply {
		return m, errors.New("棋盘已更新，请重新操作")
	}
	if a.Type != "move" && a.OfferDraw || a.Type != "respond_draw" && a.Accept != nil {
		return m, errors.New("无效操作字段")
	}
	switch a.Type {
	case "move":
		if c.Actor != s.Turn || a.Reason != "" {
			return m, errors.New("请等待自己的回合")
		}
		p := g.Position()
		mv, e := legalMove(p, a.UCI)
		if e != nil {
			return m, e
		}
		piece := p.Board().Piece(mv.S1())
		event := Event{ID: s.Ply + 1, Seat: c.Actor, From: int(mv.S1()), To: int(mv.S2()), Piece: pieceCode(piece), CaptureAt: -1, RookFrom: -1, RookTo: -1, Promotion: mv.Promo().String(), At: c.Now.UnixMilli()}
		if isCapture(mv) {
			event.CaptureAt = int(mv.S2())
			if mv.HasTag(engine.EnPassant) {
				if piece.Color() == engine.White {
					event.CaptureAt -= 8
				} else {
					event.CaptureAt += 8
				}
			}
			event.Capture = pieceCode(p.Board().Piece(engine.Square(event.CaptureAt)))
			s.Captured[c.Actor] = append(s.Captured[c.Actor], event.Capture)
		}
		if mv.HasTag(engine.KingSideCastle) || mv.HasTag(engine.QueenSideCastle) {
			base := int(mv.S1()) / 8 * 8
			event.RookFrom, event.RookTo = base+7, base+5
			if mv.HasTag(engine.QueenSideCastle) {
				event.RookFrom, event.RookTo = base, base+3
			}
		}
		san := engine.AlgebraicNotation{}.Encode(p, mv)
		if e = g.Move(mv, nil); e != nil {
			return m, e
		}
		s.Ply++
		s.UCI = append(s.UCI, mv.String())
		s.SAN = append(s.SAN, san)
		s.FEN = g.FEN()
		s.Turn = s.seat(g.Position().Turn())
		s.Offer = -1
		if piece.Type() == engine.Pawn || isCapture(mv) {
			s.Keys = nil
		}
		s.Keys = append(s.Keys, positionKey(g.Position()))
		event.Check = checkedSquare(g.Position()) >= 0
		s.Last = event
		s.note(fmt.Sprintf("玩家 %d：%s", c.Actor+1, san))
		s.Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		settle(s, g.Position())
		if s.Phase == "playing" && a.OfferDraw {
			s.Offer = c.Actor
			s.note(fmt.Sprintf("玩家 %d 随走子提议和棋", c.Actor+1))
		}
	case "claim_draw":
		if c.Actor != s.Turn || (a.Reason != "threefold" && a.Reason != "fifty_move") {
			return m, errors.New("只能在自己的回合申请满足条件的和棋")
		}
		p := g.Position()
		future := false
		if a.UCI != "" {
			mv, e := legalMove(p, a.UCI)
			if e != nil {
				return m, e
			}
			p = p.Update(mv)
			future = true
		}
		eligible := claims(s, p, future)
		if !has(eligible, a.Reason) {
			return m, errors.New("局面尚未满足该和棋条件")
		}
		s.ClaimMove = a.UCI
		s.end(-1, a.Reason+"_claim")
		s.note("和棋申请成立；预告走子不会实际移动棋子")
	case "respond_draw":
		if s.Offer < 0 || s.Offer == c.Actor || a.Accept == nil || a.UCI != "" || a.Reason != "" {
			return m, errors.New("没有需要你回应的和棋提议")
		}
		if *a.Accept {
			s.end(-1, "agreement")
			s.note("双方同意和棋")
		} else {
			s.Offer = -1
			s.note("和棋提议已拒绝，继续对弈")
		}
	case "resign":
		if a.UCI != "" || a.Reason != "" {
			return m, errors.New("无效认输操作")
		}
		if canMate(g.Position().Board(), s.colour(1-c.Actor)) {
			s.end(1-c.Actor, "resign")
			s.note(fmt.Sprintf("玩家 %d 认输", c.Actor+1))
		} else {
			s.end(-1, "resign_no_mate")
			s.note("认输方的对手不可能将杀，按规则判和")
		}
	default:
		return m, errors.New("未知国际象棋操作")
	}
	return s.envelope()
}
func view(m gameapi.Match, actor int) (any, error) {
	s, g, e := decode(m)
	if e != nil {
		return nil, e
	}
	p := g.Position()
	v := View{State: *s, Board: make([]string, 64), Legal: []LegalMove{}, Claims: []string{}, ClaimMoves: map[string][]string{}, CheckSquare: checkedSquare(p), PGN: s.pgn()}
	for sq, piece := range p.Board().SquareMap() {
		v.Board[int(sq)] = pieceCode(piece)
	}
	if s.Phase == "playing" {
		v.Claims = claims(s, p, false)
		for _, mv := range p.ValidMoves() {
			v.Legal = append(v.Legal, LegalMove{UCI: mv.String(), From: int(mv.S1()), To: int(mv.S2()), Promotion: mv.Promo().String(), Capture: isCapture(&mv), Castle: mv.HasTag(engine.KingSideCastle) || mv.HasTag(engine.QueenSideCastle)})
			if available := claims(s, p.Update(&mv), true); len(available) > 0 {
				v.ClaimMoves[mv.String()] = available
			}
		}
		active := actor >= 0 && actor < 2
		v.CanMove = active && actor == s.Turn
		v.CanResign = active
		v.CanRespond = active && s.Offer >= 0 && s.Offer != actor
	}
	return v, nil
}
func reclaim(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, _, e := decode(m)
	if e != nil {
		return m, e
	}
	if s.Phase == "playing" && c.Actor == s.Turn && s.Deadline <= c.Now.UnixMilli() {
		s.Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		return s.envelope()
	}
	return m, nil
}
