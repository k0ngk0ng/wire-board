package shengji

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"slices"
)

type Action struct {
	Type   string `json:"type"`
	Window int    `json:"window"`
	Cards  []int  `json:"cards,omitempty"`
}

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
	if n != 4 {
		return nil, errors.New("上海升级需要坐满四人")
	}
	o := Options{Decks: 2, Rounds: 0, StartLevel: 2, TurnSeconds: 30, ThrowPenalty: 0, BottomMode: "cards"}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效升级设置")
		}
	}
	if o.Decks < 1 || o.Decks > 2 || o.Rounds < 0 || o.Rounds > 24 || o.StartLevel < 2 || o.StartLevel > 14 || o.TurnSeconds < 10 || o.TurnSeconds > 120 || (o.ThrowPenalty != 0 && o.ThrowPenalty != 10) || !slices.Contains([]string{"cards", "structure"}, o.BottomMode) {
		return nil, errors.New("无效副数、起始级牌、时限或甩牌／抠底设置")
	}
	o.ThrowPenalty = 0 // Retired option: invalid submissions never charge points.
	return json.Marshal(o)
}
func Module() gameapi.Module {
	return gameapi.Module{
		ID: "shengji", Name: "升级", MinPlayers: 4, MaxPlayers: 4, Configure: configure,
		Start: func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
			o, e := configure(ctx.Players, raw)
			if e != nil {
				return gameapi.Match{}, e
			}
			var opts Options
			_ = json.Unmarshal(o, &opts)
			seat, e := randomN(4)
			if e != nil {
				return gameapi.Match{}, e
			}
			dealer := -1
			if opts.Decks == 1 {
				dealer = seat
			}
			s := &State{Ruleset: ruleset(opts.Decks), Options: opts, Round: 1, Levels: []int{opts.StartLevel, opts.StartLevel}, Dealer: dealer, StartSeat: seat, WinnerTeam: -1, Results: []RoundResult{}, Events: []Event{}, Logs: []string{}}
			if e = s.deal(ctx.Now); e != nil {
				return gameapi.Match{}, e
			}
			return s.envelope()
		},
		Apply: apply, View: project, BotAction: botAction,
		Reclaim: func(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
			s, e := decode(m)
			if e != nil {
				return m, e
			}
			if s.canAct(ctx.Actor) {
				s.Due[ctx.Actor] = ctx.Now.UnixMilli() + int64(s.Options.TurnSeconds)*1000
			}
			return s.envelope()
		},
		AcceptStale: func(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
			s, e := decode(m)
			if e != nil {
				return false
			}
			var a Action
			if strict(raw, &a) != nil {
				return false
			}
			return (s.Phase == "declare" || s.Phase == "round_end") && a.Window == s.Window && s.canAct(ctx.Actor)
		},
	}
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(raw, &a) != nil {
		return m, errors.New("无效升级操作")
	}
	if a.Window != s.Window {
		return m, errors.New("牌局已更新，请重新选择")
	}
	p := ctx.Actor
	if !s.canAct(p) {
		return m, errors.New("现在不需要你操作")
	}
	switch s.Phase {
	case "declare":
		switch a.Type {
		case "declare":
			d, err := s.declarationFor(p, a.Cards)
			if err != nil {
				return m, err
			}
			s.Declaration = &d
			s.Trump = d.Trump
			if s.Rebellion {
				s.Level = s.Levels[p%2]
			}
			s.Revealed = append(s.Revealed, Reveal{p, append([]int{}, a.Cards...), "亮主／反主"})
			s.Passed = make([]bool, 4)
			s.Passed[p] = true
			s.event("declare", p, a.Cards, "", ctx.Now)
			s.note("玩家 %d 亮主／反主", p+1)
			s.nextWindow(ctx.Now)
		case "pass":
			s.Passed[p] = true
			s.Due[p] = 0
		case "rebel":
			if !s.canRebel(p) {
				return m, errors.New("仅单副无分牌可申请重发；庄家手牌与底牌都须无分")
			}
			proof := append([]int{}, s.Hands[p]...)
			if p == s.Dealer {
				proof = append(proof, s.Bottom...)
			}
			s.event("rebel", p, proof, "无分牌明示后重发", ctx.Now)
			s.note("玩家 %d 无分牌，明示后重发；双方按己方级牌抢庄", p+1)
			s.Dealer = -1
			s.StartSeat = p
			s.Rebellion = true
			if e = s.deal(ctx.Now); e != nil {
				return m, e
			}
		default:
			return m, errors.New("请选择亮主、不亮或无分牌重发")
		}
		if len(s.actors()) == 0 {
			if e = s.finishDeclaration(ctx.Now); e != nil {
				return m, e
			}
		}
	case "bury":
		if a.Type != "bury" || len(a.Cards) != s.bottomCount() || !owns(s.Hands[p], a.Cards) {
			return m, fmt.Errorf("请选择自己的 %d 张牌扣底", s.bottomCount())
		}
		chosen := map[int]bool{}
		for _, id := range a.Cards {
			chosen[id] = true
		}
		s.Hands[p] = slices.DeleteFunc(s.Hands[p], func(id int) bool { return chosen[id] })
		s.Bottom = append([]int{}, a.Cards...)
		s.Phase = "play"
		s.Turn = s.Dealer
		s.event("bury", p, nil, "", ctx.Now)
		s.nextWindow(ctx.Now)
	case "play":
		if a.Type != "play" || !owns(s.Hands[p], a.Cards) {
			return m, errors.New("请选择自己的牌出牌")
		}
		ids := append([]int{}, a.Cards...)
		if len(s.Trick) == 0 {
			actual, breaker, err := resolveThrow(ids, s.Hands, p, s.rules())
			if err != nil {
				return m, err
			}
			if breaker >= 0 {
				return m, errors.New("这些牌不能一起甩出，请拆开选择；本次未出牌，也不计罚分")
			}
			ids = actual
			shape, err := leadShape(ids, s.rules())
			if err != nil {
				return m, err
			}
			s.Lead = &shape
		} else {
			if e = validateFollow(s.Hands[p], ids, *s.Lead, s.rules()); e != nil {
				return m, e
			}
			if p == s.ChallengeSeat && !beatsTrick(ids, s.Trick[0].Cards, *s.Lead, s.rules()) {
				return m, errors.New("破甩者须用同门更大的对应牌型跟牌")
			}
		}
		chosen := map[int]bool{}
		for _, id := range ids {
			chosen[id] = true
		}
		s.Hands[p] = slices.DeleteFunc(s.Hands[p], func(id int) bool { return chosen[id] })
		s.Played = append(s.Played, ids...)
		s.Trick = append(s.Trick, Play{p, ids})
		s.event("play", p, ids, shapeName(*s.Lead), ctx.Now)
		if len(s.Trick) == 4 {
			s.finishTrick(ctx.Now)
		} else {
			s.Turn = (p + 1) % 4
			s.nextWindow(ctx.Now)
		}
	case "round_end":
		if a.Type != "ready" {
			return m, errors.New("请确认下一副")
		}
		s.Ready[p] = true
		s.Due[p] = 0
		if len(s.actors()) == 0 {
			s.Round++
			s.Dealer = s.NextDealer
			s.StartSeat = s.Dealer
			if e = s.deal(ctx.Now); e != nil {
				return m, e
			}
		}
	default:
		return m, errors.New("牌局已经结束")
	}
	return s.envelope()
}

type Move struct {
	Cards []int  `json:"cards"`
	Label string `json:"label"`
}
type Controls struct {
	CanAct       bool          `json:"canAct"`
	CanPass      bool          `json:"canPass"`
	CanReady     bool          `json:"canReady"`
	CanRebel     bool          `json:"canRebel"`
	Declarations []Declaration `json:"declarations"`
	Moves        []Move        `json:"moves"`
	BuryCount    int           `json:"buryCount"`
	Deadline     int64         `json:"deadline"`
}

func (s *State) controls(p int) Controls {
	c := Controls{CanAct: s.canAct(p), Declarations: []Declaration{}, Moves: []Move{}}
	if !c.CanAct {
		return c
	}
	c.Deadline = s.Due[p]
	switch s.Phase {
	case "declare":
		c.CanPass = true
		c.CanRebel = s.canRebel(p)
		c.Declarations = s.declarations(p)
	case "bury":
		c.BuryCount = s.bottomCount()
	case "play":
		c.Moves = s.moves(p)
	case "round_end":
		c.CanReady = true
	}
	return c
}
func project(m gameapi.Match, p int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	hand, bottom := []int{}, []int{}
	if p >= 0 && p < 4 {
		hand = orderedCards(s.Hands[p], s.rules())
	}
	if (p == s.Dealer && s.Phase != "declare") || s.Phase == "round_end" || s.Phase == "finished" {
		bottom = orderedCards(s.Bottom, s.rules())
	}
	players := []any{}
	for seat, h := range s.Hands {
		players = append(players, map[string]any{"count": len(h), "team": seat % 2, "level": s.Levels[seat%2], "dealer": seat == s.Dealer, "passed": s.Passed[seat], "ready": s.Ready[seat]})
	}
	attackPoints := 0
	if s.Dealer >= 0 {
		attackPoints = s.Captured[1-s.Dealer%2] + s.Penalty
	}
	return map[string]any{"options": s.Options, "ruleset": s.Ruleset, "phase": s.Phase, "turn": s.Turn, "window": s.Window, "round": s.Round, "dealSerial": s.DealSerial, "dealtAt": s.DealtAt, "hand": hand, "bottom": bottom, "players": players, "level": s.Level, "levels": s.Levels, "trump": s.Trump, "dealer": s.Dealer, "declaration": s.Declaration, "revealed": s.Revealed, "rebellion": s.Rebellion, "trick": displayTrick(s.Trick, s.rules()), "lead": s.Lead, "lastTrick": displayLastTrick(s.LastTrick, s.rules()), "trickNumber": s.TrickNumber, "challengeSeat": s.ChallengeSeat, "captured": s.Captured, "attackPoints": attackPoints, "penalty": s.Penalty, "roundResults": s.Results, "finishReason": s.FinishReason, "events": s.Events, "controls": s.controls(p)}, nil
}
