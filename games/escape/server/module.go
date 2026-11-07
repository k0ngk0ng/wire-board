package escape

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func Module() gameapi.Module {
	return gameapi.Module{ID: "escape", Name: "密室逃脱", MinPlayers: 2, MaxPlayers: 4, Realtime: true, Configure: configure, Start: start, Apply: apply, View: project, SyncSnapshot: project, StreamView: stream, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer, IsInputRelease: isInputRelease}
}
func strict(raw json.RawMessage, out any) error {
	if len(raw) == 0 || len(raw) > 128*1024 || string(raw) == "null" {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}

var validSeed = regexp.MustCompile(`^[a-zA-Z0-9_-]{0,32}$`)

func configure(n int, raw json.RawMessage) (json.RawMessage, error) {
	if n < 2 || n > 4 {
		return nil, errors.New("密室逃脱需要 2～4 个角色，可添加电脑伙伴")
	}
	o := Options{Scenario: "station", Difficulty: "normal"}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效密室设置")
		}
	}
	if o.Scenario != "station" && o.Scenario != "archive" || o.Difficulty != "easy" && o.Difficulty != "normal" && o.Difficulty != "hard" || !validSeed.MatchString(o.Seed) {
		return nil, errors.New("请选择有效密室、难度与地图编号")
	}
	return json.Marshal(o)
}
func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	raw, e := configure(ctx.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var o Options
	json.Unmarshal(raw, &o)
	var random [24]byte
	if _, e = rand.Read(random[:]); e != nil {
		return gameapi.Match{}, e
	}
	epoch := hex.EncodeToString(random[:12])
	seed := o.Seed
	if seed == "" {
		seed = hex.EncodeToString(random[12:])
	}
	duration := int64(20 * 60 * 1000)
	if o.Difficulty == "easy" {
		duration = 25 * 60 * 1000
	}
	if o.Difficulty == "hard" {
		duration = 15 * 60 * 1000
	}
	s := State{Options: o, Epoch: epoch, Seed: seed, Phase: "playing", Duration: duration, Power: 100, ServerAt: ctx.Now.UnixMilli(), NextIncident: 60000}
	s.generate(ctx.Players)
	return s.match()
}
func decode(m gameapi.Match) (State, error) {
	var s State
	if m.Schema != 1 || strict(m.Data, &s) != nil || len(s.Rooms) != 9 || len(s.Players) < 2 || len(s.Players) > 4 || s.Epoch == "" || s.Random == 0 || s.Tick < 0 || s.Duration < 60000 || s.Power < 0 || s.Power > 100 || len(s.Events) > 16 || len(s.Votes) != len(s.Players) {
		return s, errors.New("无法读取密室存档")
	}
	if s.Phase != "playing" && s.Phase != "finished" {
		return s, errors.New("密室阶段无效")
	}
	for i, p := range s.Players {
		if p.Seat != i || s.room(p.Room) == nil || math.IsNaN(p.X) || math.IsNaN(p.Y) || p.X < minX || p.X > maxX || p.Y < minY || p.Y > maxY || p.HP < 0 || p.HP > 100 {
			return s, errors.New("密室角色状态无效")
		}
	}
	for i, r := range s.Rooms {
		if r.ID != fmt.Sprint("r", i) || len(r.Objects) > 4 || len(r.Doors) > 4 {
			return s, errors.New("密室结构无效")
		}
		for _, d := range r.Doors {
			if s.room(d.Room) == nil {
				return s, errors.New("密室通道无效")
			}
		}
	}
	return s, nil
}
func (s State) match() (gameapi.Match, error) {
	raw, e := json.Marshal(s)
	if e != nil {
		return gameapi.Match{}, e
	}
	m := gameapi.Match{Schema: 1, Data: raw, Actors: []int{}, Attention: []int{}, TimeoutActors: []int{}, Paused: s.Paused, CheckpointKey: fmt.Sprintf("%s/%s/%d", s.Epoch, s.Phase, s.Revision), Log: []string{}}
	for _, e := range s.Events {
		m.Log = append(m.Log, e.Text)
	}
	if s.Phase == "finished" {
		r := &gameapi.Result{Cooperative: true, Ranks: make([]int, len(s.Players)), Winners: []int{}, Scores: make([]int, len(s.Players))}
		for i := range s.Players {
			r.Ranks[i] = 1
			if s.Victory {
				r.Winners = append(r.Winners, i)
				r.Scores[i] = int(s.Duration-s.Tick)/1000 + 100*s.Supplies.Documents + 50*s.Stats.Rescues + 20*s.Stats.Explored - 20*s.Stats.Mistakes
			}
		}
		r.Details, _ = json.Marshal(map[string]any{"scenario": s.Options.Scenario, "seed": s.Seed, "elapsed": s.Tick, "stats": s.Stats, "reason": s.Reason, "documents": s.Supplies.Documents})
		m.Result = r
		return m, nil
	}
	for i := range s.Players {
		m.Actors = append(m.Actors, i)
	}
	if !s.Paused {
		m.SystemDeadline = s.ServerAt + tickMS
	}
	return m, nil
}
func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) {
		return m, errors.New("当前不能接管")
	}
	s.stop(ctx.Actor)
	return s.match()
}
func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Players != len(s.Players) {
		return m, errors.New("恢复人数与存档不符")
	}
	s.ServerAt = ctx.Now.UnixMilli()
	for i := range s.Players {
		s.Players[i].DX = 0
		s.Players[i].DY = 0
		s.Players[i].InputUntil = 0
		s.Players[i].MotionAt = s.ServerAt
	}
	return s.match()
}
func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil || ctx.Actor < 0 || ctx.Actor >= len(s.Players) || len(raw) > 4096 {
		return false
	}
	var a Action
	return strict(raw, &a) == nil && a.Epoch == s.Epoch && a.Seq > s.Players[ctx.Actor].Seq && a.Seq-s.Players[ctx.Actor].Seq <= 1024
}
func isInputRelease(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	var a Action
	return acceptStale(ctx, m, raw) && strict(raw, &a) == nil && a.Type == "input" && a.DX == 0 && a.DY == 0 && a.Target == "" && a.X == 0 && a.Y == 0 && a.Route == "" && a.Index == 0 && a.Circuit == ""
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
	v := s.view(viewer)
	key := s.Epoch + "|" + strconv.FormatUint(s.Revision, 10)
	if cursor == key && strings.HasPrefix(cursor, s.Epoch+"|") {
		v.Partial = true
		v.Rooms = nil
		v.Journal = nil
		v.Objectives = nil
		v.Routes = nil
		v.Events = nil
	}
	return gameapi.StreamViewResult{Data: v, Cursor: key}, nil
}
