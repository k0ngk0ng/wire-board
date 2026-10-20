package bubbleparty

import (
	"crypto/rand"
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
	Move  string `json:"move"`
	Bomb  bool   `json:"bomb"`
}

func Module() gameapi.Module {
	return gameapi.Module{
		ID: "bubbleparty", Name: "泡泡堂", MinPlayers: 2, MaxPlayers: 8,
		Realtime: true, Configure: configure, Start: start, Apply: apply,
		View: project, BotAction: botAction, Reclaim: reclaim, Restore: restore,
		AcceptStale: acceptStale, OnTimer: onTimer,
	}
}

func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	var seedBytes [8]byte
	if _, err := rand.Read(seedBytes[:]); err != nil {
		return gameapi.Match{}, err
	}
	var seed uint64
	for i, b := range seedBytes {
		seed |= uint64(b) << (8 * i)
	}
	return startWithSeed(ctx, raw, seed)
}

func startWithSeed(ctx gameapi.Context, raw json.RawMessage, seed uint64) (gameapi.Match, error) {
	oRaw, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var o Options
	if err := json.Unmarshal(oRaw, &o); err != nil {
		return gameapi.Match{}, err
	}
	if o.Map == "random" {
		o.Map = randomMap(seed)
	}
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		return gameapi.Match{}, err
	}
	now := ctx.Now.UnixMilli()
	s := &State{
		RulesVersion: ClassicRulesVersion, Schema: 1, Options: o, Phase: PhaseCountdown, Epoch: hex.EncodeToString(token[:]),
		Seed: seed, ServerAt: now, StartAt: now + Countdown, MapID: o.Map,
		Players: make([]Player, ctx.Players), TeamSize: ctx.Players / 2,
		Cells: mapCells(o.Map, seed), Events: []Event{}, Bombs: []Bomb{}, Items: []Item{},
		Log: []string{"3 秒后开始水泡大战"},
	}
	seconds := int64(o.RoundSeconds)
	if seconds <= 0 {
		seconds = 300
	}
	s.MatchEndsAt = s.StartAt + seconds*1000
	s.SuddenAt = 0
	for team := 0; team < 2; team++ {
		spawns := mapSpawnsFor(s.MapID, s.TeamSize, team)
		for i, sp := range spawns {
			if s.MapID == "community10" {
				clearDistrictSpawnExit(s.Cells, sp[0], sp[1])
			}
			seat := team*s.TeamSize + i
			s.Players[seat] = Player{
				Seat: seat, Team: team, X: sp[0], Y: sp[1], Dir: map[int]string{0: "right", 1: "left"}[team],
				Alive: true, SpeedMS: BaseMoveMS, MaxBombs: 1, Range: BaseBombRange,
				LastInputAt: now, IdleAt: s.StartAt + IdleMS, NextMoveAt: s.StartAt, NextBombAt: s.StartAt,
				Applied: Input{At: now, Move: "none"},
			}
		}
	}
	s.emit("countdown", -1, 0, 0, "", int(Countdown), now)
	return s.match()
}

func validAction(ctx gameapi.Context, s *State, raw json.RawMessage) (Action, error) {
	var a Action
	if len(raw) > MaxActionSize || strict(raw, &a) != nil {
		return a, errors.New("无效水泡操作")
	}
	if a.Epoch != s.Epoch {
		return a, errors.New("对局已更新，请重新连接")
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) {
		return a, errors.New("当前不能操作角色")
	}
	p := &s.Players[ctx.Actor]
	if a.Type != "input" {
		return a, errors.New("未知水泡操作")
	}
	if s.Phase != PhasePlaying || s.Result != nil || !p.Alive {
		return a, errors.New("当前不能操作角色")
	}
	if a.Seq <= p.Seq || a.Seq <= 0 || a.Seq > MaxSeq || p.Seq < 0 || p.Seq >= MaxSeq || a.Seq-p.Seq > MaxSeqJump {
		return a, errors.New("输入已过期或超出范围")
	}
	if _, ok := directionNames[a.Move]; !ok {
		return a, errors.New("无效移动方向")
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
	if s.RulesVersion == ClassicRulesVersion {
		s.advance(ctx.Now.UnixMilli())
	}
	p := &s.Players[ctx.Actor]
	if s.Result != nil || !p.Alive {
		return s.match()
	}
	s.applyInput(p, a, ctx.Now.UnixMilli())
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
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || s.Result != nil {
		return m, nil
	}
	p := &s.Players[ctx.Actor]
	now := ctx.Now.UnixMilli()
	p.Pending = nil
	p.Applied = Input{At: now, Move: "none"}
	p.LastInputAt = now
	p.IdleAt = now + IdleMS
	p.BombReq = false
	return s.match()
}

func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	repaired := false
	// Old saves may have lost bomb occupancy when the owner moved away.
	for i, c := range s.Cells {
		if c == CellBomb {
			s.Cells[i] = CellEmpty
		}
	}
	for _, b := range s.Bombs {
		if b.X < 0 || b.X >= Cols || b.Y < 0 || b.Y >= Rows {
			return m, errors.New("水泡位置无效")
		}
		s.setCell(b.X, b.Y, CellBomb)
	}
	for i := range s.Players {
		p := &s.Players[i]
		if p.Seq < 0 || p.Seq >= MaxSeq {
			p.Seq = 0
			p.Pending = nil
			p.Applied = Input{At: s.ServerAt, Move: "none"}
			p.BombReq = false
			repaired = true
		}
		if p.SpeedMS == 0 {
			p.SpeedMS = BaseMoveMS
			repaired = true
		}
		if p.MaxBombs == 0 {
			p.MaxBombs = 1
			repaired = true
		}
		if p.Range == 0 {
			p.Range = BaseBombRange
			repaired = true
		}
		if p.IdleAt <= 0 {
			p.IdleAt = s.ServerAt + IdleMS
			repaired = true
		}
	}
	if repaired {
		sum := sha256.Sum256([]byte(s.Epoch + "\x00bubbleparty-restore"))
		s.Epoch = hex.EncodeToString(sum[:12])
		s.log("异常输入状态已恢复，对局与期限保持不变")
	}
	return s.match()
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
