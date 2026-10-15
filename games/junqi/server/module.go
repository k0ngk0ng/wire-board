package junqi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"time"
)

func Module() gameapi.Module {
	return gameapi.Module{ID: "junqi", Name: "军棋", MinPlayers: 2, MaxPlayers: 4, Configure: configure, Start: start, Apply: apply, View: view, BotAction: bot, Reclaim: reclaim, AcceptStale: acceptStale}
}
func acceptStale(c gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil || c.Actor < 0 || c.Actor >= s.Players {
		return false
	}
	var a Action
	if strict(raw, &a) != nil || a.Round != s.Round || a.Ply != s.Ply {
		return false
	}
	if s.Phase == "deploy" && (a.Type == "swap" || a.Type == "shuffle" || a.Type == "deploy_ready") {
		return a.Revision == s.Revisions[c.Actor]
	}
	return s.Phase == "playing" && a.Type == "vote_draw" && s.Offer != nil && a.OfferID == s.Offer.ID
}
func fields(a Action) bool {
	if a.Type != "move" && a.To != nil || a.Type != "deploy_ready" && a.Ready != nil || a.Type != "vote_draw" && (a.Accept != nil || a.OfferID != 0) || a.Type != "swap" && a.Other != "" || a.Type != "swap" && a.Type != "move" && a.Unit != "" {
		return false
	}
	return true
}
func apply(c gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(raw, &a) != nil || !fields(a) || c.Players != s.Players || c.Actor < 0 || c.Actor >= s.Players || !s.Alive[c.Actor] || s.Phase == "finished" {
		return m, errors.New("当前不能操作")
	}
	if a.Round != s.Round || a.Ply != s.Ply {
		return m, errors.New("局面已更新，请重新操作")
	}
	if s.Phase == "deploy" {
		if a.Revision != s.Revisions[c.Actor] {
			return m, errors.New("己方布阵已更新，请重新选择")
		}
		switch a.Type {
		case "swap":
			if s.Ready[c.Actor] {
				return m, errors.New("请先取消布阵确认")
			}
			first, second := s.unit(a.Unit), s.unit(a.Other)
			if first == nil || second == nil || first.ID == second.ID || first.Seat != c.Actor || second.Seat != c.Actor || !first.Alive || !second.Alive {
				return m, errors.New("只能交换两枚己方棋子")
			}
			from, to := first.Pos, second.Pos
			first.Pos, second.Pos = to, from
			if e = s.validateFormation(c.Actor); e != nil {
				return m, e
			}
			s.beginEvent(c, "swap")
			s.Last.Moves = []Move{{Unit: first.ID, From: from, To: to, Path: []int{from, to}}, {Unit: second.ID, From: to, To: from, Path: []int{to, from}}}
		case "shuffle":
			if s.Ready[c.Actor] {
				return m, errors.New("请先取消布阵确认")
			}
			if e = s.arrange(c.Actor); e != nil {
				return m, e
			}
			s.beginEvent(c, "shuffle")
		case "deploy_ready":
			if a.Ready == nil || *a.Ready == s.Ready[c.Actor] {
				return m, errors.New("请选择新的布阵确认状态")
			}
			if e = s.validateFormation(c.Actor); e != nil {
				return m, e
			}
			s.Ready[c.Actor] = *a.Ready
			s.note(fmt.Sprintf("%s%s", sideName(c.Actor), map[bool]string{true: "已完成布阵", false: "继续调整布阵"}[*a.Ready]))
			s.beginEvent(c, "ready")
		default:
			return m, errors.New("请先完成布阵")
		}
		s.Revisions[c.Actor]++
		all := true
		for _, ready := range s.Ready {
			all = all && ready
		}
		if all {
			s.Phase, s.Turn = "playing", 0
			s.beginEvent(c, "begin")
			for i := range s.Deadlines {
				s.Deadlines[i] = 0
			}
			s.note("所有玩家完成布阵，红方先行")
			s.advance(c.Now, true)
			s.remember()
		}
		return s.envelope()
	}
	switch a.Type {
	case "move":
		if c.Actor != s.Turn || a.Unit == "" || a.To == nil {
			return m, errors.New("请等待自己的回合并选择棋子与目标")
		}
		var chosen *Move
		for _, move := range legalMoves(makeGraph(s.Players), projected(s, c.Actor), c.Actor) {
			if move.Unit == a.Unit && move.To == *a.To {
				v := move
				chosen = &v
				break
			}
		}
		if chosen == nil {
			return m, errors.New("这条路线或目标不合法")
		}
		attacker := s.unit(a.Unit)
		var defender *Piece
		for i := range s.Pieces {
			if s.Pieces[i].Alive && s.Pieces[i].Pos == chosen.To {
				defender = &s.Pieces[i]
				break
			}
		}
		s.beginEvent(c, "move")
		s.Last.Moves = []Move{*chosen}
		s.Recent = append(s.Recent, *chosen)
		if len(s.Recent) > s.Players*4 {
			s.Recent = append([]Move(nil), s.Recent[len(s.Recent)-s.Players*4:]...)
		}
		s.Offer = nil
		s.Ply++
		s.NoCasualty++
		s.Deadlines[s.Turn] = 0
		if chosen.Turned {
			attacker.Revealed = true
			s.note(fmt.Sprintf("%s的工兵沿铁路转弯，身份公开", sideName(c.Actor)))
		}
		if defender == nil {
			attacker.Pos = chosen.To
			s.Last.Outcome = "move"
		} else {
			aLives, dLives := combat(attacker.Kind, defender.Kind)
			flag, defenderSeat := defender.Kind == "flag", defender.Seat
			if !aLives {
				s.remove(attacker, true)
			}
			if !dLives {
				s.remove(defender, true)
			}
			if aLives {
				attacker.Pos = chosen.To
			}
			s.Last.Outcome = "both"
			if aLives {
				s.Last.Outcome = "attacker"
			} else if dLives {
				s.Last.Outcome = "defender"
			}
			s.NoCasualty = 0
			s.Keys = nil
			if flag {
				s.Last.FlagCaptured = true
				s.eliminate(defenderSeat, "军旗被夺")
			}
			s.note(map[string]string{"both": "双方交战，同归于尽", "attacker": "双方交战，进攻方存活", "defender": "双方交战，防守方存活"}[s.Last.Outcome])
		}
		s.advance(c.Now, false)
		s.remember()
	case "resign":
		s.beginEvent(c, "resign")
		s.Offer = nil
		s.eliminate(c.Actor, "主动认输")
		if c.Actor == s.Turn {
			s.advance(c.Now, false)
		} else {
			s.checkWin()
		}
		s.remember()
	case "offer_draw":
		if c.Actor != s.Turn || s.Offer != nil {
			return m, errors.New("只能在自己的回合发起新的和棋提议")
		}
		s.OfferSerial++
		s.Offer = &Offer{ID: s.OfferSerial, By: c.Actor, Votes: make([]bool, s.Players)}
		s.Offer.Votes[c.Actor] = true
		s.note(fmt.Sprintf("%s提议和棋，计时继续", sideName(c.Actor)))
	case "vote_draw":
		if s.Offer == nil || a.OfferID != s.Offer.ID || a.Accept == nil || s.Offer.Votes[c.Actor] {
			return m, errors.New("没有需要你回应的当前和棋提议")
		}
		if !*a.Accept {
			s.Offer = nil
			s.note("和棋提议被拒绝，继续对局")
		} else {
			s.Offer.Votes[c.Actor] = true
			all := true
			for i, alive := range s.Alive {
				if alive && !s.Offer.Votes[i] {
					all = false
				}
			}
			if all {
				s.finish(-1, "agreement")
			}
		}
	default:
		return m, errors.New("未知军棋操作")
	}
	return s.envelope()
}
func reclaim(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Actor >= 0 && c.Actor < s.Players && s.Alive[c.Actor] && (s.Phase == "deploy" && !s.Ready[c.Actor] || s.Phase == "playing" && s.Turn == c.Actor) && s.Deadlines[c.Actor] <= c.Now.UnixMilli() {
		seconds := s.Options.TurnSeconds
		if s.Phase == "deploy" {
			seconds = s.Options.SetupSeconds
		}
		s.Deadlines[c.Actor] = c.Now.Add(time.Duration(seconds) * time.Second).UnixMilli()
		return s.envelope()
	}
	return m, nil
}
