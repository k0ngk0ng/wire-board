package doudizhu

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Action struct {
	Reports  []int  `json:"reports,omitempty"`
	OpenHand bool   `json:"openHand,omitempty"`
	Continue bool   `json:"continue,omitempty"`
	Type     string `json:"type"`
	Window   int    `json:"window"`
	Cards    []int  `json:"cards,omitempty"`
	Bid      int    `json:"bid,omitempty"`
	Double   bool   `json:"double,omitempty"`
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
		return nil, errors.New("上海斗地主需要坐满 4 人")
	}
	o := Options{Mode: modeFor(n), Rounds: 1, TurnSeconds: 60, AllowStraight: true}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效斗地主设置")
		}
	}
	if o.Mode != modeFor(n) {
		return nil, errors.New("当前仅开放四人上海斗地主")
	}
	if o.Rounds < 1 || o.Rounds > 10 || o.TurnSeconds < 10 || o.TurnSeconds > 120 || (n == 3 && !o.AllowStraight) {
		return nil, errors.New("无效局数、时限或顺子设置")
	}
	o.AllowStraight = true
	o.PersonalDoubling = false
	return json.Marshal(o)
}
func Module() gameapi.Module {
	return gameapi.Module{
		ID: "doudizhu", Name: "斗地主", MinPlayers: 4, MaxPlayers: 4, Configure: configure,
		Start: func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
			normalized, e := configure(ctx.Players, raw)
			if e != nil {
				return gameapi.Match{}, e
			}
			var o Options
			_ = json.Unmarshal(normalized, &o)
			first, e := randomN(ctx.Players)
			if e != nil {
				return gameapi.Match{}, e
			}
			s := &State{Options: o, Ruleset: huanleRuleset, Players: ctx.Players, Round: 1, BidStart: first - 1, Scores: make([]int, ctx.Players), RoundResults: []RoundResult{}, Events: []Event{}, Logs: []string{}}
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
			return (s.Phase == "double" || s.Phase == "round_end") && a.Window == s.Window && s.canAct(ctx.Actor)
		},
	}
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if e = strict(raw, &a); e != nil {
		return m, errors.New("无效斗地主操作")
	}
	if a.Window != s.Window {
		return m, errors.New("牌局已更新，请按当前局面操作")
	}
	p := ctx.Actor
	if !s.canAct(p) {
		return m, errors.New("现在不需要你操作")
	}
	switch s.Phase {
	case "bid":
		if a.Type != "bid" || a.Bid < 0 || a.Bid > 3 || (a.Bid > 0 && a.Bid <= s.Bid) {
			if s.isHuanle() {
				return m, errors.New("请选择不叫或更高的档位")
			}
			return m, errors.New("请选择不叫或更高的叫分")
		}
		if s.isHuanle() && s.BidSteps == 0 && a.Bid == 3 {
			s.TouLiao = true
		}
		s.Bids[p] = a.Bid
		s.BidDone[p] = true
		s.BidSteps++
		if a.Bid > 0 {
			s.Bid = a.Bid
			s.Landlord = p
			if s.isHuanle() {
				s.note("玩家 %d 叫 %d 档", p+1, a.Bid)
			} else {
				s.note("玩家 %d 叫 %d 分", p+1, a.Bid)
			}
		} else {
			s.note("玩家 %d 不叫", p+1)
		}
		s.event("bid", p, nil, "", ctx.Now)
		if a.Bid == 3 || s.BidSteps == s.Players {
			if s.Bid == 0 {
				if s.isHuanle() {
					s.HuangFan = true
					s.note("四家不叫档，重发为荒番局")
				} else {
					s.note("本副无人叫分，重新洗牌发牌")
				}
				if e = s.deal(ctx.Now); e != nil {
					return m, e
				}
			} else {
				s.awardBottom(ctx.Now)
			}
		} else {
			s.Turn = (p + 1) % s.Players
			s.nextWindow(ctx.Now)
		}
	case "landlord_choice":
		if a.Type != "landlord" {
			return m, errors.New("请选择是否摊打、报到及继续打")
		}
		if e = s.chooseLandlord(a, ctx.Now); e != nil {
			return m, e
		}
	case "double":
		if a.Type != "double" {
			return m, errors.New("请选择加倍或不加倍")
		}
		if a.Double {
			s.DoubleFactors[p] = 2
		}
		s.DoubleReady[p] = true
		s.Due[p] = 0
		s.event("double", p, nil, "", ctx.Now)
		if len(s.actors()) == 0 {
			any := false
			for i, v := range s.DoubleFactors {
				if i != s.Landlord && v == 2 {
					any = true
				}
			}
			if any {
				s.Phase = "redouble"
				s.Turn = s.Landlord
				s.nextWindow(ctx.Now)
			} else {
				s.startPlay(ctx.Now)
			}
		}
	case "redouble":
		if a.Type != "double" {
			return m, errors.New("请选择再加倍或不加倍")
		}
		if a.Double {
			s.LandlordFactor = 2
		}
		s.event("double", p, nil, "", ctx.Now)
		s.startPlay(ctx.Now)
	case "play":
		if a.Type == "pass" {
			if s.LastPlay == nil || s.LastPlay.Seat == p {
				return m, errors.New("领出一圈时不能不出")
			}
			s.Passes++
			s.Table = append(slices.DeleteFunc(s.Table, func(v Play) bool { return v.Seat == p }), Play{Seat: p, Cards: []int{}, Pass: true})
			s.event("pass", p, nil, "", ctx.Now)
			if s.Passes == s.Players-1 {
				s.Turn = s.LastPlay.Seat
				s.LastPlay = nil
				s.Passes = 0
				s.Table = []Play{}
			} else {
				s.Turn = (p + 1) % s.Players
			}
			s.nextWindow(ctx.Now)
		} else if a.Type == "play" {
			used := map[int]bool{}
			for _, id := range a.Cards {
				if used[id] || !slices.Contains(s.Hands[p], id) {
					return m, errors.New("请选择自己手中不同的实体牌")
				}
				used[id] = true
			}
			combo, err := s.classify(a.Cards)
			if err != nil {
				return m, err
			}
			if s.LastPlay != nil && !beats(combo, s.LastPlay.Combination) {
				return m, errors.New("牌型、张数不匹配，或没有压过上一手")
			}
			if (combo.Kind == "bomb" || combo.Kind == "rocket") && !s.canBomb(p) {
				return m, errors.New("本局炸弹次数已用完，请选择其他牌型或不出")
			}
			a.Cards = s.ordered(a.Cards, combo)
			if s.LastPlay == nil {
				s.Table = []Play{}
			}
			s.Hands[p] = slices.DeleteFunc(s.Hands[p], func(id int) bool { return used[id] })
			s.Played = append(s.Played, a.Cards...)
			s.PlayCounts[p]++
			play := Play{Seat: p, Cards: append([]int{}, a.Cards...), Combination: combo}
			s.LastPlay = &play
			s.Table = append(slices.DeleteFunc(s.Table, func(v Play) bool { return v.Seat == p }), play)
			s.Passes = 0
			if s.isHuanle() && (combo.Kind == "rocket" || combo.Kind == "bomb") {
				s.BombCounts[p]++
			}
			if !s.isHuanle() && (combo.Kind == "rocket" || (combo.Kind == "bomb" && (s.Players == 3 || combo.Count >= 6))) {
				s.Multiplier *= 2
			}
			s.event("play", p, a.Cards, combo.Kind, ctx.Now)
			s.note("玩家 %d 出 %s（%d 张）", p+1, combinationName(combo.Kind), len(a.Cards))
			if len(s.Hands[p]) == 0 {
				s.finishRound(p, ctx.Now)
			} else {
				s.Turn = (p + 1) % s.Players
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
			s.HuangFan = false
			if e = s.deal(ctx.Now); e != nil {
				return m, e
			}
		}
	default:
		return m, errors.New("牌局已结束")
	}
	return s.envelope()
}
func combinationName(kind string) string {
	return map[string]string{"single": "单张", "pair": "对子", "triple": "三张", "trio_single": "三带一", "trio_pair": "三带对", "straight": "顺子", "pairs": "连对", "triples": "飞机", "plane_single": "飞机带单", "plane_pair": "飞机带对", "four_single": "四带两单", "four_pair": "四带两对", "bomb": "炸弹", "rocket": "王炸"}[kind]
}

type PlayerView struct {
	BombLimit int    `json:"bombLimit"`
	BombUsed  int    `json:"bombUsed"`
	Count     int    `json:"count"`
	Role      string `json:"role"`
	Score     int    `json:"score"`
	Bid       int    `json:"bid"`
	BidDone   bool   `json:"bidDone"`
	Double    int    `json:"double"`
	Ready     bool   `json:"ready"`
}
type Controls struct {
	CanChoose     bool    `json:"canChoose"`
	CanOpen       bool    `json:"canOpen"`
	Reports       [][]int `json:"reports"`
	BombRemaining int     `json:"bombRemaining"`
	CanAct        bool    `json:"canAct"`
	CanPass       bool    `json:"canPass"`
	CanBid        bool    `json:"canBid"`
	CanDouble     bool    `json:"canDouble"`
	CanReady      bool    `json:"canReady"`
	Bids          []int   `json:"bids"`
	Moves         []Move  `json:"moves"`
	Deadline      int64   `json:"deadline"`
}

func (s *State) controls(p int) Controls {
	c := Controls{CanAct: s.canAct(p), Bids: []int{}, Moves: []Move{}, Reports: [][]int{}, BombRemaining: -1}
	if !c.CanAct {
		return c
	}
	c.Deadline = s.Due[p]
	switch s.Phase {
	case "landlord_choice":
		c.CanChoose = true
		c.CanOpen = s.TouLiao
		c.Reports = s.reportCandidates()
	case "bid":
		c.CanBid = true
		c.Bids = append(c.Bids, 0)
		for i := s.Bid + 1; i <= 3; i++ {
			c.Bids = append(c.Bids, i)
		}
	case "double", "redouble":
		c.CanDouble = true
	case "round_end":
		c.CanReady = true
	case "play":
		c.CanPass = s.LastPlay != nil && s.LastPlay.Seat != p
		c.Moves = s.legalMoves(p)
		if s.bombLimit(p) >= 0 {
			c.BombRemaining = max(0, s.bombLimit(p)-s.bombsUsed(p))
		}
	}
	return c
}
func project(m gameapi.Match, p int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	hand := []int{}
	if p >= 0 && p < s.Players {
		hand = append(hand, s.Hands[p]...)
	}
	bottom := []int{}
	if s.Phase != "bid" && (!s.isHuanle() || p == s.Landlord) {
		bottom = append(bottom, s.Bottom...)
		sortHand(bottom)
	}
	players := make([]PlayerView, s.Players)
	for i := range players {
		role := "undecided"
		if s.Phase != "bid" {
			role = "farmer"
			if i == s.Landlord {
				role = "landlord"
			}
		}
		players[i] = PlayerView{BombLimit: s.bombLimit(i), BombUsed: s.bombsUsed(i), Count: len(s.Hands[i]), Role: role, Score: s.Scores[i], Bid: s.Bids[i], BidDone: s.BidDone[i], Double: s.DoubleFactors[i], Ready: s.Ready[i]}
	}
	landlordHand := []int{}
	if s.isHuanle() && s.OpenHand {
		landlordHand = append(landlordHand, s.Hands[s.Landlord]...)
		sortHand(landlordHand)
	}
	return map[string]any{"marker": s.Marker, "markerSeat": s.MarkerSeat, "huangFan": s.HuangFan, "touLiao": s.TouLiao, "openHand": s.OpenHand, "landlordHand": landlordHand, "reports": s.Reports, "reportPlay": s.ReportPlay, "options": s.Options, "ruleset": s.Ruleset, "phase": s.Phase, "turn": s.Turn, "window": s.Window, "round": s.Round, "dealSerial": s.DealSerial, "dealtAt": s.DealtAt, "hand": hand, "players": players, "bottom": bottom, "landlord": s.Landlord, "bid": s.Bid, "multiplier": s.Multiplier, "landlordFactor": s.LandlordFactor, "lastPlay": displayPlay(s.LastPlay, s.isHuanle()), "table": displayTable(s.Table, s.isHuanle()), "roundResults": s.RoundResults, "events": s.Events, "controls": s.controls(p)}, nil
}
