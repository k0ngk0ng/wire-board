package spf2t

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Input struct {
	Kind string `json:"kind"`
}

type Action struct {
	Type      string  `json:"type"`
	Epoch     string  `json:"epoch"`
	Seq       int64   `json:"seq,omitempty"`
	Character string  `json:"character,omitempty"`
	Events    []Input `json:"events,omitempty"`
}

func strict(raw json.RawMessage, v any) error {
	if len(raw) == 0 || len(raw) > MaxActionBytes {
		return errors.New("无效请求")
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(trimmed))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}

func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	if players != 2 {
		return nil, errors.New("街霸方块固定 2 人")
	}
	o := Options{Format: FormatBestOf3}
	if len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := strict(raw, &o); err != nil {
			return nil, errors.New("无效街霸方块设置")
		}
	}
	if o.Format == "" {
		o.Format = FormatBestOf3
	}
	if o.Format != FormatSingle && o.Format != FormatBestOf3 {
		return nil, errors.New("请选择单局决胜或三局两胜")
	}
	return json.Marshal(o)
}

func Module() gameapi.Module {
	return gameapi.Module{
		ID: "spf2t", Name: "街霸方块", MinPlayers: 2, MaxPlayers: 2,
		Realtime: true, Configure: configure, Start: start, Apply: apply,
		View: view, BotAction: botAction, Reclaim: reclaim,
		AcceptStale: acceptStale, Restore: restore, OnTimer: onTimer,
	}
}

func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	if ctx.Players != 2 {
		return gameapi.Match{}, errors.New("街霸方块固定 2 人")
	}
	oRaw, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var o Options
	if err = json.Unmarshal(oRaw, &o); err != nil {
		return gameapi.Match{}, err
	}
	token := make([]byte, 12)
	if _, err = rand.Read(token); err != nil {
		return gameapi.Match{}, err
	}
	seedBytes := make([]byte, 8)
	if _, err = rand.Read(seedBytes); err != nil {
		return gameapi.Match{}, err
	}
	seed := binary.LittleEndian.Uint64(seedBytes)
	now := ctx.Now.UnixMilli()
	players := make([]Player, 2)
	for i := range players {
		players[i] = newPlayer(i, seed)
	}
	s := &State{
		Options: o, Phase: PhaseSelect, Epoch: hex.EncodeToString(token), Seed: seed,
		ServerAt: now, SelectUntil: now + SelectMS, Round: 1, Players: players,
		Events: []Event{}, Log: []string{"街霸方块：双方选角，15 秒后自动选择"},
	}
	return s.envelope()
}

func parseAction(raw json.RawMessage) (Action, error) {
	var a Action
	if err := strict(raw, &a); err != nil {
		return a, errors.New("无效街霸方块操作")
	}
	if a.Epoch == "" {
		return a, errors.New("缺少对局标识")
	}
	switch a.Type {
	case "select":
		if a.Character == "" || a.Seq != 0 || len(a.Events) != 0 {
			return a, errors.New("无效选角操作")
		}
		if !validCharacter(a.Character) {
			return a, errors.New("未知角色")
		}
	case "inputs":
		if a.Character != "" || a.Seq <= 0 || a.Seq > MaxSeq || len(a.Events) == 0 || len(a.Events) > MaxBatchEvents {
			return a, errors.New("无效输入批次")
		}
		for _, event := range a.Events {
			switch event.Kind {
			case "left", "right", "rotateCW", "rotateCCW", "softDrop", "hardDrop":
			default:
				return a, errors.New("未知输入")
			}
		}
	default:
		return a, errors.New("未知操作")
	}
	return a, nil
}

func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s := &State{}
	if err := s.decode(m); err != nil {
		return m, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) {
		return m, errors.New("当前不能操作街霸方块")
	}
	a, err := parseAction(raw)
	if err != nil {
		return m, err
	}
	if a.Epoch != s.Epoch {
		return m, errors.New("对局已更新，请重新连接")
	}
	now := ctx.Now.UnixMilli()
	p := &s.Players[ctx.Actor]
	switch a.Type {
	case "select":
		if s.Phase != PhaseSelect || p.Selected {
			return m, errors.New("当前不能选角")
		}
		p.Character = a.Character
		p.Selected = true
		s.emitEvent(Event{Type: "select", Seat: ctx.Actor, At: now, Character: p.Character})
		s.ServerAt = now
		if s.allSelected() {
			s.moveToCountdown(now)
		}
		return s.envelope()
	case "inputs":
		if s.Phase != PhasePlaying || p.Lost || p.Resolve != nil || p.Active == nil {
			return m, errors.New("当前不能操作对子")
		}
		if a.Seq <= p.Seq || a.Seq > MaxSeq || a.Seq-p.Seq > MaxSeqJump {
			return m, errors.New("输入已过期或超出范围")
		}
		for _, event := range a.Events {
			if s.applyInput(p, event.Kind, now) {
				break
			}
			if p.Lost || p.Resolve != nil {
				break
			}
		}
		p.Seq = a.Seq
		p.IdleAt = now + IdleMS
		s.ServerAt = now
		return s.envelope()
	}
	return m, errors.New("未知操作")
}

func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s := &State{}
	if err := s.decode(m); err != nil {
		return false
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) {
		return false
	}
	a, err := parseAction(raw)
	if err != nil || a.Epoch != s.Epoch {
		return false
	}
	p := &s.Players[ctx.Actor]
	switch a.Type {
	case "select":
		return s.Phase == PhaseSelect && !p.Selected
	case "inputs":
		return s.Phase == PhasePlaying && !p.Lost && p.Resolve == nil && p.Active != nil &&
			a.Seq > p.Seq && a.Seq <= MaxSeq && a.Seq-p.Seq <= MaxSeqJump
	}
	return false
}

func view(m gameapi.Match, viewer int) (any, error) {
	s := &State{}
	if err := s.decode(m); err != nil {
		return nil, err
	}
	return s.view(viewer)
}

func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s := &State{}
	if err := s.decode(m); err != nil {
		return m, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) {
		return m, nil
	}
	if s.Phase != PhaseSelect && s.Phase != PhasePlaying {
		return m, nil
	}
	s.Players[ctx.Actor].IdleAt = ctx.Now.UnixMilli() + IdleMS
	s.ServerAt = ctx.Now.UnixMilli()
	return s.envelope()
}

func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s := &State{}
	if err := s.decode(m); err != nil {
		return m, err
	}
	if s.Result != nil {
		return m, nil
	}
	now := ctx.Now.UnixMilli()
	if m.SystemDeadline == 0 || m.SystemDeadline <= now {
		s.ServerAt = now
		switch s.Phase {
		case PhaseSelect:
			s.SelectUntil = maxInt64(s.SelectUntil, now+1)
		case PhaseCountdown:
			s.StartAt = maxInt64(s.StartAt, now+1)
		case PhaseRoundBreak:
			s.BreakUntil = maxInt64(s.BreakUntil, now+1)
		}
		return s.envelope()
	}
	return m, nil
}

func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s := &State{}
	if err := s.decode(m); err != nil {
		return m, err
	}
	if ctx.Actor != -1 {
		return m, errors.New("系统计时器不能冒用玩家席位")
	}
	if ctx.Players != len(s.Players) {
		return m, errors.New("人数不一致")
	}
	if s.Result != nil {
		return m, nil
	}
	if err := s.onTimer(ctx.Now.UnixMilli()); err != nil {
		return m, err
	}
	return s.envelope()
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
