package tetris

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
	Type   string  `json:"type"`
	Epoch  string  `json:"epoch"`
	Seq    int64   `json:"seq"`
	Events []Input `json:"events"`
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
	if players < 1 || players > 4 {
		return nil, errors.New("俄罗斯方块支持 1～4 人")
	}
	o := Options{Mode: ModeSurvival}
	if len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := strict(raw, &o); err != nil {
			return nil, errors.New("无效俄罗斯方块设置")
		}
	}
	if o.Mode == "" {
		o.Mode = ModeSurvival
	}
	if o.Mode != ModeSurvival && o.Mode != ModeSprint {
		return nil, errors.New("请选择生存赛或 40 行冲刺")
	}
	return json.Marshal(o)
}

func Module() gameapi.Module {
	return gameapi.Module{
		ID: "tetris", Name: "俄罗斯方块", MinPlayers: 1, MaxPlayers: 4,
		Realtime: true, Configure: configure, Start: start, Apply: apply,
		View: view, BotAction: botAction, Reclaim: reclaim,
		AcceptStale: acceptStale, Restore: restore, OnTimer: onTimer,
	}
}

func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
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
	startAt := now + CountdownMS
	players := make([]Player, ctx.Players)
	for i := range players {
		players[i] = newPlayer(i, seed)
		players[i].IdleAt = startAt + IdleMS
	}
	s := &State{
		Options: o, Phase: "countdown", Epoch: hex.EncodeToString(token),
		Seed: seed, ServerAt: now, StartAt: startAt, MatchEndsAt: startAt + MatchCapMS,
		Players: players, Events: []Event{},
		Log: []string{"俄罗斯方块：3 秒后开始；生存赛最后存活者获胜，冲刺赛先清 40 行获胜"},
	}
	normalizeTargets(s)
	return s.envelope()
}

func parseAction(raw json.RawMessage) (Action, error) {
	var a Action
	if err := strict(raw, &a); err != nil {
		return a, errors.New("无效俄罗斯方块操作")
	}
	if a.Type != "inputs" || len(a.Events) == 0 || len(a.Events) > MaxBatchEvents {
		return a, errors.New("无效输入批次")
	}
	for _, event := range a.Events {
		switch event.Kind {
		case "left", "right", "rotateCW", "rotateCCW", "softDrop", "hardDrop", "hold":
		default:
			return a, errors.New("未知输入")
		}
	}
	return a, nil
}

func validateBatch(s *State, p *Player, a Action) error {
	if a.Epoch != s.Epoch {
		return errors.New("对局已更新，请重新连接")
	}
	if a.Seq <= p.Seq || a.Seq <= 0 || a.Seq > MaxSeq || a.Seq-p.Seq > MaxSeqJump {
		return errors.New("输入已过期或超出范围")
	}
	return nil
}

func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) || s.Phase != "playing" || !s.Players[ctx.Actor].Alive {
		return m, errors.New("当前不能操作俄罗斯方块")
	}
	a, err := parseAction(raw)
	if err != nil {
		return m, err
	}
	p := &s.Players[ctx.Actor]
	if err = validateBatch(s, p, a); err != nil {
		return m, err
	}
	now := ctx.Now.UnixMilli()
	for _, event := range a.Events {
		if s.Result != nil {
			break
		}
		applyEvent(s, p, event.Kind, now)
	}
	p.Seq = a.Seq
	p.IdleAt = now + IdleMS
	p.LastInputAt = now
	return s.envelope()
}

func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, err := decode(m)
	if err != nil || ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) || s.Phase != "playing" || !s.Players[ctx.Actor].Alive {
		return false
	}
	a, err := parseAction(raw)
	if err != nil {
		return false
	}
	return validateBatch(s, &s.Players[ctx.Actor], a) == nil
}

func pieceView(p *Piece, board [BoardRows]string, own bool) *PieceView {
	if p == nil {
		return nil
	}
	pv := &PieceView{ID: p.ID, Kind: p.Kind, Rot: p.Rot, X: p.X, Y: p.Y}
	for _, pt := range shapeCells(p.Kind, p.Rot) {
		pv.Cells = append(pv.Cells, Point{X: p.X + pt.X, Y: p.Y + pt.Y})
	}
	if own {
		y := dropY(board, p.Kind, p.Rot, p.X, p.Y)
		pv.GhostY = &y
	}
	return pv
}

func view(m gameapi.Match, viewer int) (any, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	v := View{
		Options: s.Options, Phase: s.Phase, Epoch: s.Epoch,
		ServerAt: s.ServerAt, StartAt: s.StartAt, MatchEndsAt: s.MatchEndsAt,
		Players: make([]PlayerView, 0, len(s.Players)),
		Events:  append([]Event{}, s.Events...),
	}
	own := viewer >= 0 && viewer < len(s.Players) && s.Players[viewer].Alive && s.Phase == "playing"
	for i := range s.Players {
		p := &s.Players[i]
		pv := PlayerView{
			Seat: i, Alive: p.Alive, EliminatedAt: p.EliminatedAt,
			Board: visibleBoard(p.Board), Lines: p.Lines, Score: p.Score,
			Level: p.Level, PendingCount: len(p.Pending), TargetSeat: p.TargetSeat,
		}
		if p.Active != nil {
			pv.Active = pieceView(p.Active, p.Board, viewer == i)
		}
		if own && viewer == i {
			pv.Queue = append([]Kind{}, p.Queue[:3]...)
			pv.Hold = p.Hold
			pv.CanHold = !p.HoldUsed
			pv.Seq = p.Seq
		}
		v.Players = append(v.Players, pv)
	}
	if own {
		v.Controls.CanPlay = true
		v.Controls.CanHold = !s.Players[viewer].HoldUsed
	}
	return v, nil
}

func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || s.Phase != "playing" || !s.Players[ctx.Actor].Alive {
		return m, nil
	}
	s.Players[ctx.Actor].IdleAt = ctx.Now.UnixMilli() + IdleMS
	return s.envelope()
}

func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	if _, err := decode(m); err != nil {
		return m, err
	}
	if m.Result == nil && (m.SystemDeadline == 0 || m.SystemDeadline <= ctx.Now.UnixMilli()) {
		m.SystemDeadline = ctx.Now.UnixMilli() + FrameMS
	}
	return m, nil
}

func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
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
	now := ctx.Now.UnixMilli()
	switch s.Phase {
	case "countdown":
		if now < s.StartAt {
			return m, nil
		}
		s.Phase = "playing"
		s.ServerAt = now
		for i := range s.Players {
			p := &s.Players[i]
			p.IdleAt = now + IdleMS
			if p.Alive && !spawn(p, now) {
				eliminatePlayer(s, p, now, "spawn_blocked")
			}
		}
		if s.Result == nil {
			s.emitEvent(Event{Type: "start", Seat: -1, At: now})
			s.log("俄罗斯方块开始")
		}
	case "playing":
		if now >= s.MatchEndsAt {
			finish(s, now, "time_limit")
			break
		}
		for i := range s.Players {
			if s.Result != nil {
				break
			}
			if s.Players[i].Alive && advancePlayer(s, &s.Players[i], now) {
				break
			}
		}
		s.ServerAt = now
	default:
		return m, nil
	}
	return s.envelope()
}
