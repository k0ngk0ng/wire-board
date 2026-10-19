package battlecity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Action struct {
	Type  string `json:"type"`
	Epoch string `json:"epoch"`
	Seq   int64  `json:"seq"`
	Tank  string `json:"tank"`
	Move  string `json:"move"`
	Fire  bool   `json:"fire"`
}

func Module() gameapi.Module {
	return gameapi.Module{
		ID: "battlecity", Name: "坦克大战", MinPlayers: 2, MaxPlayers: 6,
		Realtime: true, Configure: configure, Start: start, Apply: apply,
		View: project, BotAction: botAction, Reclaim: reclaim, Restore: restore,
		AcceptStale: acceptStale, OnTimer: onTimer,
	}
}

func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	oRaw, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var o Options
	if err := json.Unmarshal(oRaw, &o); err != nil {
		return gameapi.Match{}, err
	}
	s, err := newState(ctx, o)
	if err != nil {
		return gameapi.Match{}, err
	}
	return s.match()
}

func validAction(ctx gameapi.Context, s State, raw json.RawMessage) (Action, error) {
	var a Action
	if len(raw) > MaxActionSize {
		return a, errors.New("操作过大")
	}
	if strict(raw, &a) != nil {
		return a, errors.New("无效坦克操作")
	}
	if a.Epoch != s.Epoch {
		return a, errors.New("对局已更新，请重新连接")
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Tanks) {
		return a, errors.New("当前不能操作坦克")
	}
	t := &s.Tanks[ctx.Actor]
	switch a.Type {
	case "select":
		if s.Phase != PhaseSelect || s.Result != nil {
			return a, errors.New("当前不能选择坦克")
		}
		if t.Selected {
			return a, errors.New("已经选择过坦克")
		}
		if !ValidTank(a.Tank) {
			return a, errors.New("请选择有效坦克")
		}
	case "input":
		if s.Phase != PhasePlaying || s.Result != nil {
			return a, errors.New("当前不能操作坦克")
		}
		if a.Seq <= t.Seq || a.Seq <= 0 || a.Seq > MaxSeq || t.Seq < 0 || t.Seq >= MaxSeq || a.Seq-t.Seq > MaxSeqJump {
			return a, errors.New("输入已过期或超出范围")
		}
		if !s.validMove(a.Move) {
			return a, errors.New("无效移动方向")
		}
	default:
		return a, errors.New("未知坦克操作")
	}
	return a, nil
}

func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	a, err := validAction(ctx, s, raw)
	if err != nil {
		return m, err
	}
	now := ctx.Now.UnixMilli()
	t := &s.Tanks[ctx.Actor]
	switch a.Type {
	case "select":
		t.Class = a.Tank
		t.Selected = true
		t.MaxHP = s.maxHP(t)
		t.HP = t.MaxHP
		t.IdleAt = now + IdleMS
		s.emit("select", t.Seat, t.Team, now, t.X, t.Y, 0, t.Class, 0)
		if s.allSelected() {
			s.beginCountdown(now)
		}
	case "input":
		t.Seq = a.Seq
		s.applyInput(t, a.Move, a.Fire, now)
	}
	return s.match()
}

func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, err := decode(m)
	if err != nil {
		return false
	}
	_, err = validAction(ctx, s, raw)
	return err == nil
}

func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Tanks) || s.Result != nil {
		return m, nil
	}
	t := &s.Tanks[ctx.Actor]
	now := ctx.Now.UnixMilli()
	t.Pending = nil
	t.Applied = s.neutralInput(now)
	t.LastInputAt = now
	t.IdleAt = now + IdleMS
	t.SlideUntil = 0
	return s.match()
}

func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	repaired := false
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if t.Seq < 0 || t.Seq >= MaxSeq {
			t.Seq = 0
			t.Pending = nil
			t.Applied = s.neutralInput(s.ServerAt)
			repaired = true
		}
		if s.Phase != PhaseSelect && t.Class == "" {
			t.Class = "balanced"
			t.Selected = true
			repaired = true
		}
		if t.MaxHP <= 0 {
			t.MaxHP = s.maxHP(t)
			if t.HP > t.MaxHP {
				t.HP = t.MaxHP
			}
			repaired = true
		}
	}
	if repaired {
		s.Epoch = repairEpoch(s.Epoch)
		s.log("异常输入序号已恢复，对局与期限保持不变")
	}
	return s.match()
}

func repairEpoch(epoch string) string {
	sum := sha256.Sum256([]byte(epoch + "\x00sequence-repair"))
	return hex.EncodeToString(sum[:12])
}

func project(m gameapi.Match, seat int) (any, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	return s.project(seat), nil
}

func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	s.onTimer(ctx.Now.UnixMilli())
	return s.match()
}
