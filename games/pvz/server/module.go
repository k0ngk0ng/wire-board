// Package pvz adapts the independent PvZ versus engine to platform rooms.
package pvz

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/games/pvz/engine"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

const maxSnapshotBytes = 512 * 1024
const maxApplied = 256

type Applied struct {
	Serial uint64        `json:"serial"`
	Tick   int64         `json:"tick"`
	Actor  int           `json:"actor"`
	Action engine.Action `json:"action"`
}
type state struct {
	Runtime   engine.State `json:"runtime"`
	ServerAt  int64        `json:"serverAt"`
	Remainder int64        `json:"remainder"`
	Revision  uint64       `json:"revision"`
	Applied   []Applied    `json:"applied"`
}
type View struct {
	Map         engine.MapInfo       `json:"map"`
	Visibility  engine.Visibility    `json:"visibility"`
	WireMode    string               `json:"wireMode"`
	Pickups     []engine.Pickup      `json:"pickups"`
	Graves      []engine.Grave       `json:"graves"`
	Craters     []engine.Crater      `json:"craters"`
	Terrain     []engine.Terrain     `json:"terrain"`
	Ladders     []engine.Ladder      `json:"ladders"`
	Protocol    int                  `json:"protocol"`
	Options     engine.Options       `json:"options"`
	Epoch       string               `json:"epoch"`
	Round       int                  `json:"round"`
	Phase       string               `json:"phase"`
	Tick        int64                `json:"tick"`
	ServerAt    int64                `json:"serverAt"`
	PhaseUntil  int64                `json:"phaseUntil"`
	Players     []engine.Player      `json:"players"`
	Plants      []engine.Plant       `json:"plants"`
	Zombies     []engine.Zombie      `json:"zombies"`
	Projectiles []engine.Projectile  `json:"projectiles"`
	Mowers      []engine.Mower       `json:"mowers"`
	Events      []engine.Event       `json:"events"`
	Rounds      []engine.RoundResult `json:"rounds"`
	WinnerTeam  int                  `json:"winnerTeam"`
	CanAct      bool                 `json:"canAct"`
	Catalog     engine.Catalog       `json:"catalog"`
	Snapshot    engine.State         `json:"snapshot"`
	Revision    uint64               `json:"revision"`
	Digest      string               `json:"digest"`
	Actor       int                  `json:"actor"`
}
type delta struct {
	Protocol   int                  `json:"protocol"`
	Epoch      string               `json:"epoch"`
	Round      int                  `json:"round"`
	Phase      string               `json:"phase"`
	Tick       int64                `json:"tick"`
	ServerAt   int64                `json:"serverAt"`
	PhaseUntil int64                `json:"phaseUntil"`
	Players    []engine.Player      `json:"players"`
	Rounds     []engine.RoundResult `json:"rounds"`
	WinnerTeam int                  `json:"winnerTeam"`
	CanAct     bool                 `json:"canAct"`
	Applied    []Applied            `json:"applied"`
	Revision   uint64               `json:"revision"`
	Digest     string               `json:"digest"`
}

// Override immutable fields in the embedded state so ordinary fog updates send
// only the necessary authority projection, without repeating map/catalog data.
type projectionPatch struct {
	engine.State
	Map           *engine.MapInfo `json:"map,omitempty"`
	Options       *engine.Options `json:"options,omitempty"`
	BaseEpoch     *string         `json:"baseEpoch,omitempty"`
	InitialRandom *uint32         `json:"initialRandom,omitempty"`
	Projection    *bool           `json:"projection,omitempty"`
}

func Module() gameapi.Module {
	return gameapi.Module{ID: "pvz", Name: "植物大战僵尸", MinPlayers: 2, MaxPlayers: 4, Realtime: true, Configure: configure, Start: start, Apply: apply, View: project, StreamView: stream, SyncSnapshot: project, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer}
}
func strict(raw json.RawMessage, out any) error {
	if len(raw) == 0 || len(raw) > maxSnapshotBytes || string(raw) == "null" {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(n int, raw json.RawMessage) (json.RawMessage, error) {
	o := engine.Options{RoundSeconds: 360, PrepSeconds: 20}
	if len(raw) > 0 {
		if strict(raw, &o) != nil {
			return nil, errors.New("无效植物大战僵尸设置")
		}
	}
	if o.RoundSeconds == 0 || o.PrepSeconds == 0 {
		return nil, errors.New("请选择有效的攻防与布阵时间")
	}
	o, e := engine.NormalizeOptions(o, n)
	if e != nil {
		return nil, e
	}
	return json.Marshal(o)
}
func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	raw, e := configure(ctx.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var o engine.Options
	_ = json.Unmarshal(raw, &o)
	var token [12]byte
	if _, e = rand.Read(token[:]); e != nil {
		return gameapi.Match{}, e
	}
	runtime, e := engine.New(o, ctx.Players, hex.EncodeToString(token[:]))
	if e != nil {
		return gameapi.Match{}, e
	}
	engine.EnableProgression(&runtime)
	s := state{Runtime: runtime, ServerAt: ctx.Now.UnixMilli(), Applied: []Applied{}}
	return s.match()
}
func decode(m gameapi.Match) (state, error) {
	var s state
	if m.Schema != 2 || strict(m.Data, &s) != nil || s.Runtime.Projection || s.ServerAt < 0 || s.Remainder < 0 || s.Remainder >= engine.TickMS || len(s.Applied) > maxApplied {
		return s, errors.New("无法读取植物大战僵尸存档")
	}
	if err := engine.Validate(s.Runtime); err != nil {
		return s, err
	}
	var prev uint64
	for i, a := range s.Applied {
		if a.Serial == 0 || a.Serial > s.Revision || i > 0 && a.Serial != prev+1 || a.Tick < 0 || a.Tick > s.Runtime.Tick || a.Actor < 0 || a.Actor >= len(s.Runtime.Players) || a.Action.Epoch != s.Runtime.Epoch {
			return s, errors.New("无效同步记录")
		}
		prev = a.Serial
	}
	if len(s.Applied) > 0 && s.Applied[len(s.Applied)-1].Serial != s.Revision {
		return s, errors.New("同步记录缺少最新操作")
	}
	return s, nil
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a engine.Action
	if len(raw) > 2048 || strict(raw, &a) != nil {
		return m, errors.New("无效植物大战僵尸操作")
	}
	if ctx.Players != len(s.Runtime.Players) {
		return m, errors.New("房间人数与存档不符")
	}
	if e = engine.Apply(&s.Runtime, ctx.Actor, a); e != nil {
		return m, e
	}
	s.Revision++
	s.Applied = append(s.Applied, Applied{Serial: s.Revision, Tick: s.Runtime.Tick, Actor: ctx.Actor, Action: a})
	if len(s.Applied) > maxApplied {
		s.Applied = s.Applied[len(s.Applied)-maxApplied:]
	}
	return s.match()
}
func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Actor != -1 {
		return m, errors.New("只有系统可以推进时钟")
	}
	if s.Runtime.Phase == "finished" {
		return m, nil
	}
	now := ctx.Now.UnixMilli()
	elapsed := max(int64(0), now-s.ServerAt)
	s.ServerAt = max(s.ServerAt, now)
	s.Remainder += min(elapsed, 1000)
	steps := int(s.Remainder / engine.TickMS)
	s.Remainder %= engine.TickMS
	oldEpoch := s.Runtime.Epoch
	if e = engine.Step(&s.Runtime, steps); e != nil {
		return m, e
	}
	if oldEpoch != s.Runtime.Epoch {
		s.Applied = []Applied{}
	}
	return s.match()
}
func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Players != len(s.Runtime.Players) {
		return m, errors.New("恢复人数与存档不符")
	}
	s.ServerAt = ctx.Now.UnixMilli()
	s.Remainder = 0
	return s.match()
}
func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Runtime.Players) {
		return m, errors.New("当前不能接管")
	}
	return m, nil
}
func botAction(ctx gameapi.Context, m gameapi.Match) (json.RawMessage, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	visible := engine.ProjectState(s.Runtime, ctx.Actor)
	a := engine.BotAction(&visible, ctx.Actor)
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}
func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil || ctx.Actor < 0 || ctx.Actor >= len(s.Runtime.Players) {
		return false
	}
	var a engine.Action
	if len(raw) > 2048 || strict(raw, &a) != nil || a.Epoch != s.Runtime.Epoch {
		return false
	}
	return a.Seq > s.Runtime.Players[ctx.Actor].Seq && a.Seq-s.Runtime.Players[ctx.Actor].Seq <= 1024
}
func (s state) phaseUntil() int64 {
	if s.Runtime.Phase == "finished" {
		return 0
	}
	return s.ServerAt + (s.Runtime.PhaseEndTick-s.Runtime.Tick)*engine.TickMS - s.Remainder
}
func (s state) canAct(viewer int) bool {
	return viewer >= 0 && viewer < len(s.Runtime.Players) && (s.Runtime.Phase == "prepare" || s.Runtime.Phase == "battle")
}
func (s state) view(viewer int) View {
	r := engine.ProjectState(s.Runtime, viewer)
	wire := "replay"
	if r.Map.Fog {
		wire = "projection"
	}
	return View{Protocol: 2, Map: r.Map, Visibility: engine.VisibilityFor(s.Runtime, viewer), WireMode: wire, Pickups: r.Pickups, Graves: r.Graves, Craters: r.Craters, Terrain: r.Terrain, Ladders: r.Ladders, Options: r.Options, Epoch: r.Epoch, Round: r.Round, Phase: r.Phase, Tick: r.Tick, ServerAt: s.ServerAt, PhaseUntil: s.phaseUntil(), Players: r.Players, Plants: r.Plants, Zombies: r.Zombies, Projectiles: r.Projectiles, Mowers: r.Mowers, Events: r.Events, Rounds: r.Rounds, WinnerTeam: r.WinnerTeam, CanAct: s.canAct(viewer), Catalog: engine.CatalogFor(r), Snapshot: r, Revision: s.Revision, Digest: engine.Digest(r), Actor: viewer}
}

func project(m gameapi.Match, viewer int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	return s.view(viewer), nil
}
func stream(m gameapi.Match, viewer int, cursor string) (gameapi.StreamViewResult, error) {
	s, e := decode(m)
	if e != nil {
		return gameapi.StreamViewResult{}, e
	}
	next := s.Runtime.Epoch + "|" + strconv.FormatUint(s.Revision, 10)
	full := func() gameapi.StreamViewResult { return gameapi.StreamViewResult{Data: s.view(viewer), Cursor: next} }
	sep := strings.LastIndexByte(cursor, '|')
	if sep < 0 || cursor[:sep] != s.Runtime.Epoch {
		return full(), nil
	}
	if s.Runtime.Map.Fog {
		v := s.view(viewer)
		data := map[string]any{"protocol": 2, "wireMode": "projection", "epoch": v.Epoch, "round": v.Round, "phase": v.Phase, "tick": v.Tick, "serverAt": v.ServerAt, "phaseUntil": v.PhaseUntil, "players": v.Players, "rounds": v.Rounds, "winnerTeam": v.WinnerTeam, "canAct": v.CanAct, "visibility": v.Visibility, "projected": projectionPatch{State: v.Snapshot}, "revision": v.Revision, "digest": v.Digest}
		return gameapi.StreamViewResult{Data: data, Cursor: next}, nil
	}
	seq, e := strconv.ParseUint(cursor[sep+1:], 10, 64)
	if e != nil || seq > s.Revision {
		return full(), nil
	}
	if seq < s.Revision && (len(s.Applied) == 0 || seq+1 < s.Applied[0].Serial) {
		return full(), nil
	}
	applied := []Applied{}
	for _, a := range s.Applied {
		if a.Serial > seq {
			applied = append(applied, a)
		}
	}
	r := s.Runtime
	return gameapi.StreamViewResult{Data: delta{Protocol: 1, Epoch: r.Epoch, Round: r.Round, Phase: r.Phase, Tick: r.Tick, ServerAt: s.ServerAt, PhaseUntil: s.phaseUntil(), Players: r.Players, Rounds: r.Rounds, WinnerTeam: r.WinnerTeam, CanAct: s.canAct(viewer), Applied: applied, Revision: s.Revision, Digest: engine.Digest(r)}, Cursor: next}, nil
}
func (s state) match() (gameapi.Match, error) {
	raw, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	r := s.Runtime
	m := gameapi.Match{Schema: 2, Data: raw, Actors: []int{}, Attention: []int{}, TimeoutActors: []int{}, CheckpointKey: r.Epoch + "/" + r.Phase, Log: []string{"共享草坪，独立资源；完成两个回合后比较攻破结果与用时"}}
	if r.Phase == "prepare" || r.Phase == "battle" {
		for p := range r.Players {
			m.Actors = append(m.Actors, p)
		}
	}
	if result := engine.Result(r); result != nil {
		out := &gameapi.Result{Draw: result.Draw, Team: len(r.Players) == 4, Ranks: make([]int, len(r.Players)), Scores: make([]int, len(r.Players)), Winners: []int{}}
		for p, player := range r.Players {
			out.Ranks[p] = 1
			if !result.Draw {
				if player.Team == result.WinnerTeam {
					out.Winners = append(out.Winners, p)
					out.Scores[p] = 1
				} else {
					out.Ranks[p] = len(r.Players)/2 + 1
				}
			}
		}
		out.Details, _ = json.Marshal(result)
		m.Result = out
		m.Log = append(m.Log, "两回合结束")
	} else {
		m.SystemDeadline = s.ServerAt + engine.TickMS - s.Remainder
	}
	if e = m.Validate(len(r.Players)); e != nil {
		return gameapi.Match{}, fmt.Errorf("植物大战僵尸状态错误: %w", e)
	}
	return m, nil
}
