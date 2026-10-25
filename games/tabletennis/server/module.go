package tabletennis

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"math"
	"slices"
)

func strict(raw json.RawMessage, v any) error {
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
	if n != 2 && n != 4 {
		return nil, errors.New("乒乓球支持 2 人单打或 4 人双打")
	}
	o := Options{BestOf: 1, Teams: make([]int, n), Difficulty: "normal", Court: "club"}
	for i := range o.Teams {
		o.Teams[i] = i % 2
	}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效乒乓球设置")
		}
	}
	if !slices.Contains([]int{1, 3}, o.BestOf) || !slices.Contains([]string{"easy", "normal", "hard"}, o.Difficulty) || !slices.Contains([]string{"club", "arena", "heritage"}, o.Court) || len(o.Teams) != n {
		return nil, errors.New("请选择有效的赛制、场馆、难度与分队")
	}
	counts := [2]int{}
	for _, team := range o.Teams {
		if team < 0 || team > 1 {
			return nil, errors.New("无效分队")
		}
		counts[team]++
	}
	if counts[0] != n/2 || counts[1] != n/2 {
		return nil, errors.New("两队人数必须相等")
	}
	return json.Marshal(o)
}
func Module() gameapi.Module {
	return gameapi.Module{ID: "tabletennis", Name: "乒乓球", MinPlayers: 2, MaxPlayers: 4, Realtime: true, Configure: configure, Start: start, Apply: apply, View: project, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer}
}
func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	oRaw, e := configure(ctx.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var o Options
	_ = json.Unmarshal(oRaw, &o)
	var token [12]byte
	if _, e = rand.Read(token[:]); e != nil {
		return gameapi.Match{}, e
	}
	now := ctx.Now.UnixMilli()
	teams := [2][]int{}
	for i, t := range o.Teams {
		teams[t] = append(teams[t], i)
	}
	first := int(token[0]) % 2
	order := []int{}
	for i := 0; i < ctx.Players/2; i++ {
		order = append(order, teams[first][i], teams[1-first][i])
	}
	s := State{Paddles: initialPaddles(o.Teams, now), Options: o, Epoch: hex.EncodeToString(token[:]), Phase: "countdown", ServerAt: now, PhaseUntil: now + 1800, Game: 1, Rally: 1, Order: order, InitialOrder: append([]int{}, order...), Seq: make([]int64, ctx.Players), Games: []GameResult{}, Events: []Event{}, LastHitter: -1, PointWinner: -1, Log: []string{"11 分制，领先 2 分获胜；倒数结束后划动发球"}}
	s.Expected, s.Receiver = s.service()
	return s.match()
}
func validateAction(ctx gameapi.Context, s State, raw json.RawMessage) (Action, error) {
	var a Action
	if strict(raw, &a) != nil {
		return a, errors.New("无效挥拍操作")
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Seq) || s.Result != nil {
		return a, errors.New("当前不能挥拍")
	}
	sameBall := a.Rally == s.Rally && a.Shot == s.Shot
	if a.Type == "move" {
		if a.Epoch != s.Epoch || a.MotionSeq <= 0 || a.MotionSeq > 1<<31-1 || a.MotionSeq-s.Paddles[ctx.Actor].MotionSeq > 1024 || a.Ends < 0 || a.Ends > 1 || !finite(a.RacketX) || math.Abs(a.RacketX) > 8 || a.RacketY == nil || !finite(*a.RacketY) || *a.RacketY < -4 || *a.RacketY > 8 || !finite(a.RacketZ) || a.RacketZ < -2 || a.RacketZ > 3 || a.At > ctx.Now.UnixMilli()+80 {
			return a, errors.New("无效球拍位置")
		}
		return a, nil
	}
	oldCancel := a.Type == "cancel" && a.Rally >= 1 && a.Rally <= s.Rally && a.Shot >= 0 && (a.Rally < s.Rally || a.Shot <= s.Shot)
	if a.Epoch != s.Epoch || (!sameBall && !oldCancel) {
		return a, errors.New("来球已更新，请跟随当前球路挥拍")
	}
	if (a.Seq <= s.Seq[ctx.Actor] && a.Type != "cancel") || a.Seq <= 0 || a.Seq > 1<<31-1 || a.Seq-s.Seq[ctx.Actor] > 1024 {
		return a, errors.New("挥拍已处理或序号无效")
	}
	if a.Type != "swing" && a.Type != "cancel" {
		return a, errors.New("未知操作")
	}
	if a.Type == "cancel" {
		return a, nil
	}
	now := ctx.Now.UnixMilli()
	if a.At < now-LateGraceMS || a.At > now+80 {
		return a, errors.New("挥拍时间已过期，请重新连接")
	}
	reach := 1.
	if !finite(a.RacketRadius) || a.RacketRadius < 0 || a.RacketRadius > .45 || (a.RacketRadius > 0 && a.RacketRadius < .04) {
		return a, errors.New("无效球拍范围")
	}
	if a.RacketY != nil {
		reach = 8
		if !finite(a.RacketZ) || a.RacketZ < -2 || a.RacketZ > 3 {
			return a, errors.New("无效球拍位置")
		}
	}
	if !finite(a.RacketX) || math.Abs(a.RacketX) > reach || (a.RacketY != nil && (!finite(*a.RacketY) || *a.RacketY < -4 || *a.RacketY > 8)) || !finite(a.Lift) || a.Lift < 0 || a.Lift > 1 || !finite(a.Aim) || !finite(a.Power) || a.Aim < -1 || a.Aim > 1 || a.Power < 0 || a.Power > 1 || !slices.Contains([]string{"drive", "chop", "block", "smash"}, a.Kind) {
		return a, errors.New("无效落点、力度或挥拍类型")
	}
	return a, nil
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	a, e := validateAction(ctx, s, raw)
	if e != nil {
		return m, e
	}
	now := ctx.Now.UnixMilli()
	if a.Type == "cancel" {
		if a.Seq <= s.Seq[ctx.Actor] {
			return m, nil
		}
		// Cancellation may overtake or follow an old request. It only clears
		// an unexecuted return for the exact ball, never a committed serve.
		if s.Phase == "rally" && a.Rally == s.Rally && a.Shot == s.Shot && s.Pending != nil && s.Pending.Seat == ctx.Actor {
			s.Pending = nil
		}
		s.Seq[ctx.Actor] = a.Seq
		return s.match()
	}
	if a.Type == "move" {
		p := &s.Paddles[ctx.Actor]
		if a.Ends != s.Ends || a.MotionSeq <= p.MotionSeq || a.At < p.MotionAt || a.At < now-LateGraceMS {
			return m, nil
		}
		p.X, p.Y, p.Z = a.RacketX, *a.RacketY, a.RacketZ
		p.MotionSeq, p.MotionAt = a.MotionSeq, min(a.At, now)
		return s.match()
	}
	advance(&s, now)
	if s.Result != nil || s.Rally != a.Rally || s.Shot != a.Shot || s.Expected != ctx.Actor {
		return m, errors.New("请等待轮到自己接球")
	}
	if s.Pending != nil {
		return m, errors.New("本次挥拍已准备")
	}
	switch s.Phase {
	case "serve":
		if a.Kind == "smash" {
			return m, errors.New("发球不能扣杀")
		}
		if a.At < s.PhaseUntil {
			return m, errors.New("请在发球开始后挥拍")
		}
		if s.ServeDeadline > 0 && a.At > s.ServeDeadline {
			return m, errors.New("发球时间已结束")
		}
		began := a.At
		tolerance := a.RacketRadius
		if tolerance == 0 {
			tolerance = .22
		}
		if a.RacketY != nil && math.Hypot(a.RacketX*.9-.34, *a.RacketY-1.1) > tolerance {
			s.Seq[ctx.Actor] = a.Seq
			s.recordPaddle(a, ctx.Actor, now, "miss")
			return s.match()
		}
		if began > now {
			began = now
		}
		launchAt := began + 180
		if a.RacketY == nil {
			y := 1.1
			a.RacketY = &y
			a.RacketX = .34 / .9
		}
		a.RacketZ = 1.28
		s.recordPaddle(a, ctx.Actor, launchAt, "armed")
		s.Seq[ctx.Actor] = a.Seq
		s.Pending = &PendingSwing{Action: a, Seat: ctx.Actor, At: launchAt}
		s.Phase = "serve_toss"
		s.PhaseUntil = launchAt
		s.ServeDeadline = 0
		s.emit("toss", began, ctx.Actor, Vec{}, "")
		if launchAt <= now {
			s.launch(a, ctx.Actor, launchAt, true)
		}
	case "rally":
		f := s.Flight
		if f == nil || f.ReceiveAt == 0 || f.Reason == "let" {
			return m, errors.New("等待来球落台")
		}
		at := a.At
		if at > now {
			at = now
		}
		if at < f.ContactStart-EarlyBufferMS || at > f.ContactEnd {
			return m, errors.New("请在来球弹起后挥拍")
		}
		if at < f.ContactStart {
			at = f.ContactStart
		}
		p, _ := f.at(at)
		s.Seq[ctx.Actor] = a.Seq
		racketX := sign(s.side(s.Options.Teams[ctx.Actor])) * a.RacketX * .9
		// Same fixed racket/ball radius plus light assistance on every device.
		height := p.Y
		if a.RacketY != nil {
			height = *a.RacketY
		}
		tolerance := a.RacketRadius
		if tolerance == 0 {
			tolerance = .22
		}
		if math.Hypot(p.X-racketX, p.Y-height) > tolerance {
			s.recordPaddle(a, ctx.Actor, now, "miss")
			s.emit("whiff", now, ctx.Actor, Vec{racketX, height, p.Z}, a.Kind)
			return s.match()
		}
		s.recordPaddle(a, ctx.Actor, at, "armed")
		if a.RacketY == nil {
			a.RacketY = &height
			a.RacketZ = p.Z * sign(s.side(s.Options.Teams[ctx.Actor]))
		}
		if a.Kind == "smash" && p.Y < 1.18 {
			return m, errors.New("来球过低，适合抽球或削球")
		}
		s.Seq[ctx.Actor] = a.Seq
		if at > now {
			s.Pending = &PendingSwing{Action: a, Seat: ctx.Actor, At: at}
		} else {
			s.launch(a, ctx.Actor, at, false)
		}
	default:
		return m, errors.New("等待下一分开始")
	}
	return s.match()
}
func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil {
		return false
	}
	_, e = validateAction(ctx, s, raw)
	return e == nil
}
func emitFlight(s *State, until int64) {
	if s.Flight == nil {
		return
	}
	for s.FlightCursor < len(s.Flight.Events) {
		e := s.Flight.Events[s.FlightCursor]
		if e.At > until {
			break
		}
		// A legal timestamped return may still replace the terminal impact.
		// Publish it only after the bounded acceptance window closes.
		if s.Flight.ReceiveAt > 0 && e.At >= s.Flight.EndAt && until < s.Flight.EndAt+LateGraceMS {
			break
		}
		s.emit(e.Type, e.At, s.LastHitter, e.P, "")
		s.FlightCursor++
	}
}
func advance(s *State, now int64) {
	if s.Result != nil || now < s.ServerAt {
		return
	}
	s.ServerAt = now
	switch s.Phase {
	case "countdown":
		if now >= s.PhaseUntil {
			s.beginServe(now)
		}
	case "serve":
		if s.ServeDeadline > 0 && now >= s.ServeDeadline+LateGraceMS {
			s.award(1-s.Options.Teams[s.Expected], "serve_timeout", now)
		}
	case "point_break":
		if now >= s.PhaseUntil {
			s.Rally++
			s.Shot = 0
			s.PointWinner = -1
			s.beginServe(now)
		}
	case "game_break":
		if now >= s.PhaseUntil {
			s.newGame(now)
		}
	case "serve_toss":
		if s.Pending != nil && now >= s.Pending.At {
			p := *s.Pending
			s.launch(p.Action, p.Seat, p.At, true)
		}
	case "rally":
		if s.Flight == nil {
			return
		}
		if s.Pending != nil && now >= s.Pending.At {
			p := *s.Pending
			emitFlight(s, p.At)
			s.launch(p.Action, p.Seat, p.At, false)
		}
		emitFlight(s, now)
		if s.Flight != nil && now >= s.Flight.EndAt+LateGraceMS {
			if s.Flight.Reason == "let" {
				s.emit("let", s.Flight.EndAt, s.LastHitter, Vec{}, "")
				s.Rally++
				s.Shot = 0
				s.Phase = "countdown"
				s.PhaseUntil = now + 1000
				s.Flight = nil
				s.Pending = nil
				s.Expected, s.Receiver = s.service()
			} else {
				team := s.Flight.WinnerSide ^ s.Ends
				s.award(team, s.Flight.Reason, now)
			}
		}
	}
}
func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if s.Result != nil || ctx.Now.UnixMilli() < s.ServerAt {
		return m, nil
	}
	advance(&s, ctx.Now.UnixMilli())
	return s.match()
}
func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Seq) || s.Result != nil {
		return m, nil
	}
	if s.Pending != nil && s.Pending.Seat == ctx.Actor && s.Phase == "rally" {
		s.Pending = nil
	}

	return s.match()
}
func restore(_ gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	_, e := decode(m)
	return m, e
}
func project(m gameapi.Match, seat int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	effects := s.Events
	if len(effects) > 16 {
		effects = effects[len(effects)-16:]
	}
	v := View{Paddles: s.Paddles, Options: s.Options, Epoch: s.Epoch, Phase: s.Phase, ServerAt: s.ServerAt, PhaseUntil: s.PhaseUntil, ServeDeadline: s.ServeDeadline, Score: s.Score, GamesWon: s.GamesWon, Game: s.Game, Games: s.Games, Order: s.Order, Ends: s.Ends, Rally: s.Rally, Shot: s.Shot, Expected: s.Expected, Receiver: s.Receiver, LastHitter: s.LastHitter, PointWinner: s.PointWinner, Flight: s.Flight, Events: effects}
	v.ReachDepth = ReachDepth
	if seat >= 0 && seat < len(s.Seq) && s.Result == nil {
		v.Controls.Seq = s.Seq[seat]
		paddle := s.Paddles[seat]
		if paddle.Result == "miss" && paddle.Rally == s.Rally && paddle.Shot == s.Shot {
			v.Controls.RetrySeq = paddle.Seq
		}
		v.Controls.Armed = s.Pending != nil && s.Pending.Seat == seat
		v.Controls.CanSwing = seat == s.Expected && (s.Phase == "serve" || s.Phase == "rally") && !v.Controls.Armed
		if s.Phase == "serve" {
			v.Controls.OpenAt = s.PhaseUntil
			v.Controls.CloseAt = s.ServeDeadline
			if s.ServeDeadline > 0 && s.ServerAt > s.ServeDeadline {
				v.Controls.CanSwing = false
			}
		} else if s.Flight != nil && s.Flight.ReceiveAt > 0 {
			v.Controls.OpenAt = s.Flight.ContactStart - EarlyBufferMS
			v.Controls.CloseAt = s.Flight.ContactEnd
		} else {
			v.Controls.CanSwing = false
		}
	}
	return v, nil
}
