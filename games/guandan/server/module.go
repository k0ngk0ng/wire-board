package guandan

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Action struct {
	Type           string `json:"type"`
	Window         int    `json:"window"`
	Cards          []int  `json:"cards,omitempty"`
	Interpretation string `json:"interpretation,omitempty"`
	Card           int    `json:"card,omitempty"`
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
		return nil, errors.New("江苏掼蛋需要坐满四人")
	}
	o := Options{Rounds: 0, StartLevel: 2, TurnSeconds: 30}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效掼蛋设置")
		}
	}
	if o.Rounds < 0 || o.Rounds > 24 || o.StartLevel < 2 || o.StartLevel > 14 || o.TurnSeconds < 10 || o.TurnSeconds > 120 {
		return nil, errors.New("无效局数、起始级牌或时限")
	}
	return json.Marshal(o)
}
func Module() gameapi.Module {
	return gameapi.Module{
		ID: "guandan", Name: "掼蛋", MinPlayers: 4, MaxPlayers: 4, Configure: configure,
		Start: func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
			o, e := configure(ctx.Players, raw)
			if e != nil {
				return gameapi.Match{}, e
			}
			var opts Options
			_ = json.Unmarshal(o, &opts)
			s := &State{Ruleset: "guandan-jiangsu-2017-v1", Options: opts, Round: 1, Levels: []int{opts.StartLevel, opts.StartLevel}, DealerTeam: -1, WinnerTeam: -1, Results: []RoundResult{}, Events: []Event{}, Logs: []string{}}
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
			return slices.Contains([]string{"tribute", "return", "round_end"}, s.Phase) && a.Window == s.Window && s.canAct(ctx.Actor)
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
		return m, errors.New("无效掼蛋操作")
	}
	if a.Window != s.Window {
		return m, errors.New("牌局已更新，请重新选择")
	}
	p := ctx.Actor
	if !s.canAct(p) {
		return m, errors.New("现在不需要你操作")
	}
	switch s.Phase {
	case "tribute":
		if a.Type != "tribute" || !slices.Contains(s.tributeChoices(p), a.Card) {
			return m, errors.New("须进贡最大的非配牌")
		}
		for i := range s.Exchanges {
			if s.Exchanges[i].From == p {
				s.Exchanges[i].Given = a.Card
			}
		}
		s.Due[p] = 0
		if len(s.actors()) == 0 {
			s.settleTributes(ctx.Now)
		}
	case "return":
		if a.Type != "return" || !slices.Contains(s.returnChoices(p), a.Card) {
			return m, errors.New("请选择 10 以下的非级牌回贡；无此类牌时还最小非配牌")
		}
		for i := range s.Exchanges {
			x := &s.Exchanges[i]
			if x.To != p {
				continue
			}
			x.Returned = a.Card
			s.Hands[p] = slices.DeleteFunc(s.Hands[p], func(id int) bool { return id == a.Card })
			s.Hands[x.From] = append(s.Hands[x.From], a.Card)
			sortHand(s.Hands[x.From], s.Level)
			s.event("return", p, x.From, []int{a.Card}, true, "", ctx.Now)
			s.note("玩家 %d 已向玩家 %d 还牌", p+1, x.From+1)
		}
		s.Due[p] = 0
		if len(s.actors()) == 0 {
			s.Phase = "play"
			s.Turn = s.OpeningSeat
			s.nextWindow(ctx.Now)
		}
	case "play":
		if a.Type == "preview" {
			for _, id := range a.Cards {
				if !slices.Contains(s.Hands[p], id) {
					return m, errors.New("请选择自己的实体牌")
				}
			}
			cs, err := interpret(a.Cards, s.Level)
			if err != nil {
				return m, err
			}
			legal := []Combination{}
			for _, c := range cs {
				if s.LastPlay == nil || beats(c, s.LastPlay.Combination) {
					legal = append(legal, c)
				}
			}
			if len(legal) == 0 {
				return m, errors.New("所选牌型未压过上一手")
			}
			s.Preview = &Preview{Seat: p, Cards: append([]int{}, a.Cards...), Interpretations: legal}
		} else if a.Type == "pass" {
			if s.LastPlay == nil || s.LastPlay.Seat == p {
				return m, errors.New("领出一圈时不能不出")
			}
			s.Passes++
			s.Table = append(slices.DeleteFunc(s.Table, func(v Play) bool { return v.Seat == p }), Play{Seat: p, Cards: []int{}, Pass: true})
			s.event("pass", p, -1, nil, false, "", ctx.Now)
			if s.Passes >= s.responders() {
				lead := s.LastPlay.Seat
				if len(s.Hands[lead]) == 0 {
					lead = (lead + 2) % 4
					s.event("partner_lead", lead, -1, nil, false, "", ctx.Now)
					s.note("玩家 %d 对家接风", lead+1)
				}
				s.Turn = lead
				s.LastPlay = nil
				s.Passes = 0
				s.Table = []Play{}
			} else {
				s.Turn = s.nextLive(p)
			}
			s.nextWindow(ctx.Now)
		} else if a.Type == "play" {
			used := map[int]bool{}
			for _, id := range a.Cards {
				if used[id] || !slices.Contains(s.Hands[p], id) {
					return m, errors.New("请选择自己的实体牌")
				}
				used[id] = true
			}
			combos, err := interpret(a.Cards, s.Level)
			if err != nil {
				return m, err
			}
			selected := -1
			if a.Interpretation == "" && len(combos) == 1 {
				selected = 0
			} else {
				for i, c := range combos {
					if c.Key == a.Interpretation {
						selected = i
						break
					}
				}
			}
			if selected < 0 {
				return m, errors.New("配牌有多种含义，请选择要出的牌型")
			}
			combo := combos[selected]
			if s.LastPlay != nil && !beats(combo, s.LastPlay.Combination) {
				return m, errors.New("所选牌型未压过上一手")
			}
			a.Cards = orderedPlay(a.Cards, combo, s.Level)
			if s.LastPlay == nil {
				s.Table = []Play{}
			}
			s.Hands[p] = slices.DeleteFunc(s.Hands[p], func(id int) bool { return used[id] })
			s.Played = append(s.Played, a.Cards...)
			play := Play{Seat: p, Cards: append([]int{}, a.Cards...), Combination: combo}
			s.LastPlay = &play
			s.Table = append(slices.DeleteFunc(s.Table, func(v Play) bool { return v.Seat == p }), play)
			s.Passes = 0
			s.event("play", p, -1, a.Cards, false, combo.Kind, ctx.Now)
			if len(s.Hands[p]) == 0 {
				s.FinishOrder = append(s.FinishOrder, p)
				s.note("玩家 %d 第 %d 位出完", p+1, len(s.FinishOrder))
				s.event("out", p, -1, nil, false, "", ctx.Now)
			}
			if s.roundComplete() {
				s.finishRound(ctx.Now)
			} else {
				s.Turn = s.nextLive(p)
				s.nextWindow(ctx.Now)
			}
		} else {
			return m, errors.New("请选择出牌或不出")
		}
	case "round_end":
		if a.Type != "ready" {
			return m, errors.New("请确认下一副")
		}
		s.Ready[p] = true
		s.Due[p] = 0
		if len(s.actors()) == 0 {
			s.Round++
			if e = s.deal(ctx.Now); e != nil {
				return m, e
			}
		}
	default:
		return m, errors.New("牌局已经结束")
	}
	return s.envelope()
}

type Controls struct {
	CanAct          bool          `json:"canAct"`
	CanPass         bool          `json:"canPass"`
	CanReady        bool          `json:"canReady"`
	Moves           []Move        `json:"moves"`
	Choices         []int         `json:"choices"`
	Deadline        int64         `json:"deadline"`
	PreviewCards    []int         `json:"previewCards"`
	Interpretations []Combination `json:"interpretations"`
}

func (s *State) controls(p int) Controls {
	c := Controls{CanAct: s.canAct(p), Moves: []Move{}, Choices: []int{}, PreviewCards: []int{}, Interpretations: []Combination{}}
	if !c.CanAct {
		return c
	}
	c.Deadline = s.Due[p]
	switch s.Phase {
	case "play":
		if s.Preview != nil && s.Preview.Seat == p {
			c.PreviewCards = append(c.PreviewCards, s.Preview.Cards...)
			c.Interpretations = append(c.Interpretations, s.Preview.Interpretations...)
		}
		c.CanPass = s.LastPlay != nil
		for _, m := range movesFor(s.Hands[p], s.Level) {
			if s.LastPlay == nil || beats(m.Combination, s.LastPlay.Combination) {
				c.Moves = append(c.Moves, m)
			}
		}
		if s.LastPlay != nil {
			sortResponseHints(c.Moves, s.Hands[p], s.Level)
		}
	case "tribute":
		c.Choices = s.tributeChoices(p)
	case "return":
		c.Choices = s.returnChoices(p)
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
	hand := []int{}
	if p >= 0 && p < 4 {
		hand = append(hand, s.Hands[p]...)
	}
	players := []any{}
	for i, h := range s.Hands {
		place := 0
		for j, seat := range s.FinishOrder {
			if seat == i {
				place = j + 1
			}
		}
		players = append(players, map[string]any{"count": len(h), "team": i % 2, "level": s.Levels[i%2], "place": place, "ready": s.Ready[i]})
	}
	exchanges := []any{}
	for _, x := range s.Exchanges {
		entry := map[string]any{"from": x.From, "to": x.To, "given": -1, "returned": -1, "selected": x.Given >= 0, "done": x.Returned >= 0}
		if s.Phase != "tribute" || p == x.From {
			entry["given"] = x.Given
		}
		if p == x.From || p == x.To {
			entry["returned"] = x.Returned
		}
		exchanges = append(exchanges, entry)
	}
	events := append([]Event{}, s.Events...)
	for i := range events {
		e := &events[i]
		if e.Private && p != e.Seat && p != e.Target {
			e.Cards = nil
		}
	}
	return map[string]any{"options": s.Options, "ruleset": s.Ruleset, "phase": s.Phase, "turn": s.Turn, "window": s.Window, "round": s.Round, "dealSerial": s.DealSerial, "dealtAt": s.DealtAt, "hand": hand, "players": players, "level": s.Level, "levels": s.Levels, "dealerTeam": s.DealerTeam, "lastPlay": displayPlay(s.LastPlay, s.Level), "table": displayTable(s.Table, s.Level), "finishOrder": s.FinishOrder, "exchanges": exchanges, "revealed": s.Revealed, "antiTribute": s.AntiTribute, "roundResults": s.Results, "finishReason": s.FinishReason, "events": events, "controls": s.controls(p)}, nil
}
