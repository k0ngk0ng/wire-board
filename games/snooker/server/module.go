package snooker

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"slices"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
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
		return nil, errors.New("斯诺克仅支持 2 人单打或 4 人双打")
	}
	o := Options{BestOf: 1, AimAssist: "line", Teams: make([]int, n)}
	for i := range o.Teams {
		o.Teams[i] = i % 2
	}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效斯诺克设置")
		}
	}
	if !slices.Contains([]int{1, 3, 5}, o.BestOf) || !slices.Contains([]string{"line", "none"}, o.AimAssist) || len(o.Teams) != n {
		return nil, errors.New("无效赛制、辅助线或分队设置")
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
	return gameapi.Module{ID: "snooker", Name: "斯诺克", MinPlayers: 2, MaxPlayers: 4, Configure: configure,
		Start: func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
			options, e := configure(ctx.Players, raw)
			if e != nil {
				return gameapi.Match{}, e
			}
			var o Options
			_ = json.Unmarshal(options, &o)
			s := State{Ruleset: "snooker-wpbsa-2024-v1", Options: o, Players: ctx.Players, Teams: append([]int{}, o.Teams...), Frame: 1, Window: 1, Winner: -1, BestBreak: make([]int, ctx.Players), Results: []FrameResult{}, Logs: []string{}}
			s.NextFirst = [2]int{s.first(0), s.first(1)}
			s.rack()
			s.Phase = "start_choice"
			s.setTurn(s.first(toss()))
			s.note("标准十五红球；抽签胜方选择开球队伍")
			s.syncClocks("", -1, ctx.Now)
			return s.match()
		}, Apply: apply, View: project, BotAction: botAction, Reclaim: reclaim, Restore: restore,
		AcceptStale: func(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
			s, e := decode(m)
			if e != nil {
				return false
			}
			var a Action
			if strict(raw, &a) != nil {
				return false
			}
			return independentVote(a.Type) && a.Window == s.Window && slices.Contains(s.actors(), ctx.Actor)
		},
		OnTimer: func(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
			s, e := decode(m)
			if e != nil {
				return m, e
			}
			if s.Phase != "rolling" || ctx.Now.UnixMilli() < s.Flight.End {
				return m, nil
			}
			previousPhase, previousTurn := s.Phase, s.Turn
			if e = s.settle(ctx.Now); e != nil {
				return m, e
			}
			s.syncClocks(previousPhase, previousTurn, ctx.Now)
			next, e := s.match()
			if e != nil {
				return m, e
			}
			return next, nil
		},
	}
}

type Action struct {
	Type     string  `json:"type"`
	Window   int     `json:"window"`
	Angle    float64 `json:"angle"`
	Power    float64 `json:"power"`
	Top      float64 `json:"top"`
	Side     float64 `json:"side"`
	Nominate int     `json:"nominate"`
	FreeBall int     `json:"freeBall"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Team     int     `json:"team"`
	Seat     int     `json:"seat"`
	Choice   string  `json:"choice"`
}

func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(raw, &a) != nil {
		return m, errors.New("无效斯诺克操作")
	}
	if a.Window != s.Window {
		return m, errors.New("球局已更新，请按当前局面操作")
	}
	if ctx.Actor < 0 || ctx.Actor >= s.Players || !slices.Contains(s.actors(), ctx.Actor) {
		return m, errors.New("现在不能执行这个操作")
	}
	common := a.Type == "concede_frame" || a.Type == "resign" || a.Type == "rerack" || a.Type == "cancel_vote" || a.Type == "ready" || a.Type == "lead"
	if !common && ctx.Actor != s.Turn {
		return m, errors.New("请等待当前击球者完成操作")
	}
	previousPhase, previousTurn := s.Phase, s.Turn
	advanceWindow := !independentVote(a.Type)
	switch a.Type {
	case "break_choice":
		if s.Phase != "start_choice" || a.Team < 0 || a.Team > 1 {
			return m, errors.New("请选择开球队伍")
		}
		s.OpeningTeam = a.Team
		s.setTurn(s.NextFirst[a.Team])
		s.OpeningSeat = s.Turn
		s.Phase = "aim"
		s.note("%s 队开球，玩家 %d 击球", []string{"A", "B"}[a.Team], s.Turn+1)
	case "place":
		if s.Phase != "aim" || !s.InHand {
			return m, errors.New("当前不能摆放白球")
		}
		p := Vec{a.X, a.Y}
		if !finite(a.X) || !finite(a.Y) || a.X > BaulkX+1e-9 || p.sub(Vec{BaulkX, TableHeight / 2}).length() > DRadius+1e-9 || !clearSpot(p, s.Balls, 0) {
			return m, errors.New("请把白球放在 D 区内且不与其他球接触")
		}
		for i := range s.Balls {
			if s.Balls[i].ID == 0 {
				s.Balls[i].Pos = p
				s.Balls[i].Down = false
			}
		}
	case "shoot":
		if s.Phase != "aim" {
			return m, errors.New("请等待球停止并完成判罚")
		}
		shot := Strike{Angle: a.Angle, Power: a.Power, Top: a.Top, Side: a.Side, Nominate: a.Nominate, FreeBall: a.FreeBall}
		if s.Target == "colour" && (a.Nominate < 2 || a.Nominate > 7) {
			return m, errors.New("请选择本杆彩球")
		}
		if a.FreeBall > 0 {
			b, ok := findBall(s.Balls, a.FreeBall)
			if !s.FreeBall || !ok || b.Down || slices.Contains(onBalls(s.context(), a.Nominate), a.FreeBall) {
				return m, errors.New("请选择可提名的自由球")
			}
		}
		motion, e := Simulate(s.Balls, shot)
		if e != nil {
			return m, e
		}
		s.ShotID++
		s.Flight = &Flight{ID: s.ShotID, Start: ctx.Now.UnixMilli(), End: ctx.Now.UnixMilli() + int64(math.Ceil(motion.Duration*1000)), Seat: s.Turn, Context: s.context(), Strike: shot, Motion: motion}
		s.Flight.Integrity = flightHash(s.Flight)
		s.Phase = "rolling"
	case "foul_choice":
		if s.Phase != "foul" || s.Foul == nil {
			return m, errors.New("当前没有待处理判罚")
		}
		f := s.Foul
		switch a.Choice {
		case "accept":
			s.FreeBall = freeBallAvailable(s.Balls, onBalls(s.context(), 0), s.InHand)
			s.MissRun = nil
		case "replay":
			s.setTurn(f.Offender)
			s.FreeBall = false
			s.MissRun = nil
		case "restore":
			if !f.Ruling.Miss {
				return m, errors.New("本次未判解球未中，不能要求原位重打")
			}
			s.Balls = append([]Ball{}, f.Before.Balls...)
			s.Target = f.Before.Target
			s.Clear = f.Before.Clear
			s.InHand = false // Original-position replay fixes the cue at its restored location.
			s.FreeBall = false
			s.setTurn(f.Offender)
		default:
			return m, errors.New("请选择自己打、让对手继续或原位重打")
		}
		s.Phase = "aim"
		s.Foul = nil
		s.Break = 0
	case "black_choice":
		if s.Phase != "black_choice" || a.Team < 0 || a.Team > 1 {
			return m, errors.New("请选择黑球决胜先打队伍")
		}
		s.setTurn(s.first(a.Team))
		s.Phase = "black_player"
	case "black_player":
		if s.Phase != "black_player" || a.Seat < 0 || a.Seat >= s.Players || s.Teams[a.Seat] != s.Teams[s.Turn] {
			return m, errors.New("请选择先打队伍的一名球员")
		}
		s.setTurn(a.Seat)
		s.BlackStarter = a.Seat
		s.Phase = "aim"
	case "lead":
		if s.Phase != "frame_end" || a.Seat < 0 || a.Seat >= s.Players || s.Teams[a.Seat] != s.Teams[ctx.Actor] {
			return m, errors.New("仅可在局间选择本队先后顺序")
		}
		team := s.Teams[ctx.Actor]
		s.NextFirst[team] = a.Seat
		for p, v := range s.Teams {
			if v == team {
				s.Ready[p] = false
			}
		}
	case "ready":
		if s.Phase != "frame_end" {
			return m, errors.New("请等待本局结束")
		}
		s.Ready[ctx.Actor] = true
		if !slices.Contains(s.Ready, false) {
			s.Frame++
			s.OpeningTeam = 1 - s.OpeningTeam
			s.rack()
			s.note("第 %d 局，%s 队开球", s.Frame, []string{"A", "B"}[s.OpeningTeam])
		}
	case "claim_frame":
		if !s.canClaim() {
			return m, errors.New("仅剩黑球且当前一方领先超过 7 分时才能宣告本局胜利")
		}
		s.finishFrame(s.Teams[ctx.Actor], "claimed")
	case "concede_frame":
		if s.Phase != "aim" || absInt(s.Scores[0]-s.Scores[1]) <= Remaining(s.context()) {
			return m, errors.New("尚无人需要罚分才能取胜，不能提前认输本局")
		}
		s.Concede[ctx.Actor] = true
		team := s.Teams[ctx.Actor]
		all := true
		for p, v := range s.Teams {
			if v == team && !s.Concede[p] {
				all = false
			}
		}
		if all {
			s.OfferTeam = team
			s.OfferTurn = s.Turn
			s.setTurn(s.first(1 - team))
			s.Phase = "concede"
			s.note("%s 队提出认输本局，等待对方决定", []string{"A", "B"}[team])
		}
	case "concede_reply":
		if s.Phase != "concede" {
			return m, errors.New("当前没有认输本局的提议")
		}
		switch a.Choice {
		case "accept":
			s.finishFrame(1-s.OfferTeam, "conceded")
		case "continue":
			s.setTurn(s.OfferTurn)
			s.OfferTeam = -1
			s.OfferTurn = -1
			s.Concede = make([]bool, s.Players)
			s.Phase = "aim"
			s.note("对方选择继续本局")
		default:
			return m, errors.New("请选择接受认输或继续本局")
		}
	case "resign":
		if s.Phase != "aim" && s.Phase != "frame_end" {
			return m, errors.New("请在击球之间确认认输整场")
		}
		s.Resign[ctx.Actor] = true
		team := s.Teams[ctx.Actor]
		all := true
		for p, v := range s.Teams {
			if v == team && !s.Resign[p] {
				all = false
			}
		}
		if all {
			s.Winner = 1 - team
			s.Phase = "finished"
			s.FinishReason = "resigned"
			s.note("%s 队认输本场", []string{"A", "B"}[team])
			s.note("%s", s.finishExplanation("resigned"))
		}
	case "rerack":
		if s.Phase != "aim" {
			return m, errors.New("请在击球之间提出重新摆球")
		}
		s.Rerack[ctx.Actor] = true
		if !slices.Contains(s.Rerack, false) {
			s.Due = nil
			advanceWindow = true
			if s.BlackDecider {
				for i := range s.Balls {
					if s.Balls[i].ID == 21 {
						s.Balls[i].Pos = ColourSpot(7)
						s.Balls[i].Down = false
					} else if s.Balls[i].ID == 0 {
						s.Balls[i].Down = true
					}
				}
				s.setTurn(s.BlackStarter)
				s.InHand = true
				s.Flight = nil
				s.Rerack = make([]bool, s.Players)
			} else {
				opener := s.OpeningSeat
				s.rack()
				s.setTurn(opener)
				s.OpeningSeat = opener
			}
			s.note("双方同意重新摆球，保留本局开球顺序")
		}
	case "cancel_vote":
		s.Concede[ctx.Actor] = false
		s.Resign[ctx.Actor] = false
		s.Rerack[ctx.Actor] = false
	default:
		return m, errors.New("未知斯诺克操作")
	}
	if advanceWindow || s.Phase != previousPhase {
		s.Window++
	}
	s.syncClocks(previousPhase, previousTurn, ctx.Now)
	next, e := s.match()
	if e != nil {
		return m, e
	}
	return next, nil
}
func project(m gameapi.Match, p int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	can := p >= 0 && p < s.Players && p == s.Turn
	// All ball positions and scores are public. Never expose a future ruling or
	// future score while the authoritative motion is still being displayed.
	return map[string]any{"endExplanation": s.endExplanation(), "concede": s.Concede, "offerTeam": s.OfferTeam, "winner": s.Winner, "finishReason": s.FinishReason, "ruleset": s.Ruleset, "options": s.Options, "phase": s.Phase, "window": s.Window, "balls": s.Balls, "turn": s.Turn, "nextSeat": s.nextSeat(), "teams": s.Teams, "order": s.Order, "nextFirst": s.NextFirst, "scores": s.Scores, "framesWon": s.FramesWon, "frame": s.Frame, "break": s.Break, "bestBreak": s.BestBreak, "target": s.Target, "clear": s.Clear, "inHand": s.InHand, "freeBall": s.FreeBall, "flight": s.Flight, "foul": s.Foul, "missRun": s.MissRun, "blackDecider": s.BlackDecider, "ready": s.Ready, "resign": s.Resign, "rerack": s.Rerack, "results": s.Results, "remaining": Remaining(s.context()), "controls": map[string]any{"canClaim": can && s.canClaim(), "canConcede": p >= 0 && p < s.Players && s.Phase == "aim" && absInt(s.Scores[0]-s.Scores[1]) > Remaining(s.context()), "canAct": can && s.Phase != "rolling" && s.Phase != "finished", "canShoot": can && s.Phase == "aim", "canPlace": can && s.Phase == "aim" && s.InHand}}, nil
}

func (s *State) canClaim() bool {
	if s.Phase != "aim" || s.Target != "clear" || s.Clear != 7 || s.Scores[s.Teams[s.Turn]]-s.Scores[1-s.Teams[s.Turn]] <= 7 {
		return false
	}
	for _, b := range s.Balls {
		if b.ID > 0 && b.ID != 21 && !b.Down {
			return false
		}
	}
	return true
}

// Independent confirmations share a decision window. A phase transition or a
// changed table invalidates it; shot/placement/order edits never accept stale
// room revisions merely because another command was an independent vote.
func independentVote(kind string) bool {
	return slices.Contains([]string{"ready", "resign", "concede_frame", "rerack", "cancel_vote"}, kind)
}
