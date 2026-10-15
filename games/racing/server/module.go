package racing

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"math"
)

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
	if n < 2 || n > 6 {
		return nil, errors.New("逐风赛车支持 2～6 人")
	}
	o := Options{Track: "woodland", Laps: 3}
	if len(raw) > 0 {
		if e := strict(raw, &o); e != nil {
			return nil, errors.New("无效赛车设置")
		}
	}
	if _, ok := trackByID(o.Track); !ok {
		return nil, errors.New("请选择有效赛道")
	}
	if o.Laps != 1 && o.Laps != 3 && o.Laps != 5 {
		return nil, errors.New("圈数可选 1、3 或 5 圈")
	}
	return json.Marshal(o)
}

type Action struct {
	Type     string  `json:"type"`
	Epoch    string  `json:"epoch"`
	Seq      int64   `json:"seq"`
	Steer    float64 `json:"steer"`
	Throttle float64 `json:"throttle"`
	Brake    bool    `json:"brake"`
	Drift    bool    `json:"drift"`
	Skin     int     `json:"skin"`
}

func Module() gameapi.Module {
	return gameapi.Module{ID: "racing", Name: "逐风赛车", MinPlayers: 2, MaxPlayers: 6, Realtime: true, Configure: configure, Start: start, Apply: apply, View: project, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer}
}
func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	oRaw, e := configure(ctx.Players, raw)
	if e != nil {
		return gameapi.Match{}, e
	}
	var o Options
	_ = json.Unmarshal(oRaw, &o)
	t, _ := trackByID(o.Track)
	token := [12]byte{}
	if _, e = rand.Read(token[:]); e != nil {
		return gameapi.Match{}, e
	}
	now := ctx.Now.UnixMilli()
	s := State{Options: o, Phase: "countdown", Epoch: hex.EncodeToString(token[:]), ServerAt: now, StartAt: now + 3000, MaxRaceAt: now + 483000, Track: t, Cars: make([]Car, ctx.Players), Events: []Event{}, Log: []string{"逐风赛车：3 秒后发车，首车完赛后保留 30 秒冲线时间"}}
	p, v := t.Points[0], tangent(t, 0)
	for i := range s.Cars {
		behind := 50. + float64(i/2)*55
		side := float64(i%2*2-1) * 22
		c := Car{Seat: i, X: p.X - v.X*behind - v.Y*side, Y: p.Y - v.Y*behind + v.X*side, Angle: math.Atan2(v.Y, v.X), Checkpoint: 0, Gate: -1, Progress: -behind, Rank: i/2 + 1, LastInputAt: now, IdleAt: now + idleMS, Skin: i % 5, Applied: Input{At: now, Brake: true}}
		s.Cars[i] = c
	}
	rank(&s)
	return s.match()
}
func validAction(ctx gameapi.Context, s State, raw json.RawMessage) (Action, error) {
	var a Action
	if strict(raw, &a) != nil {
		return a, errors.New("无效赛车操作")
	}
	if a.Epoch != s.Epoch {
		return a, errors.New("比赛已更新，请重新连接")
	}
	if s.Result != nil || ctx.Actor < 0 || ctx.Actor >= len(s.Cars) || s.Cars[ctx.Actor].FinishAt > 0 {
		return a, errors.New("当前不能操作赛车")
	}
	c := s.Cars[ctx.Actor]
	switch a.Type {
	case "drive":
		if a.Seq <= c.Seq || a.Seq <= 0 || a.Seq > maxInputSequence || c.Seq < 0 || c.Seq >= maxInputSequence || a.Seq-c.Seq > maxSequenceJump || !finite(a.Steer) || a.Steer < -1 || a.Steer > 1 || !finite(a.Throttle) || a.Throttle < 0 || a.Throttle > 1 {
			return a, errors.New("输入已过期或超出范围")
		}
	case "reset":
		if s.Phase == "countdown" || c.Speed > 55 || ctx.Now.UnixMilli() < c.ResetUntil || c.LastReset > 0 && ctx.Now.UnixMilli()-c.LastReset < 5000 {
			return a, errors.New("请减速后复位，复位冷却 5 秒")
		}
	case "skin":
		if a.Skin < 0 || a.Skin > 4 {
			return a, errors.New("请选择有效车身")
		}
	default:
		return a, errors.New("未知赛车操作")
	}
	return a, nil
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	a, e := validAction(ctx, s, raw)
	if e != nil {
		return m, e
	}
	c := &s.Cars[ctx.Actor]
	at := ctx.Now.UnixMilli()
	switch a.Type {
	case "drive":
		c.Seq = a.Seq
		c.Steer = a.Steer
		c.Throttle = a.Throttle
		c.Brake = a.Brake
		c.Drift = a.Drift
		c.LastInputAt = at
		c.IdleAt = at + idleMS
		in := Input{At: at, Steer: a.Steer, Throttle: a.Throttle, Brake: a.Brake, Drift: a.Drift}
		if len(c.Pending) > 0 && c.Pending[len(c.Pending)-1].At == at {
			c.Pending[len(c.Pending)-1] = in
		} else {
			c.Pending = append(c.Pending, in)
		}
		if len(c.Pending) > 64 {
			c.Pending = c.Pending[len(c.Pending)-64:]
		}
	case "skin":
		c.Skin = a.Skin
	case "reset":
		i := c.Gate
		if i < 0 {
			i = 0
		}
		p, v := s.Track.Points[i], tangent(s.Track, i)
		c.X = p.X - v.X*5
		c.Y = p.Y - v.Y*5
		c.Angle = math.Atan2(v.Y, v.X)
		c.VX = 0
		c.VY = 0
		c.Speed = 0
		c.LastReset = at
		c.ResetUntil = at + 2000
		c.Pending = nil
		c.Applied = Input{At: at, Brake: true}
		c.Steer = 0
		c.Throttle = 0
		c.Brake = true
		c.Drift = false
		c.LastInputAt = at
		c.IdleAt = at + idleMS
		s.emit("reset", ctx.Actor, at, 1)
	}
	return s.match()
}
func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, e := decode(m)
	if e != nil {
		return false
	}
	a, e := validAction(ctx, s, raw)
	return e == nil && (a.Type == "drive" || a.Type == "skin" || a.Type == "reset")
}
func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if s.Result != nil || ctx.Now.UnixMilli() < s.ServerAt+frameMS {
		return m, nil
	}
	repairSequences(&s)
	advance(&s, ctx.Now.UnixMilli())
	return s.match()
}
func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Cars) || s.Result != nil || s.Cars[ctx.Actor].FinishAt > 0 {
		return m, nil
	}
	c := &s.Cars[ctx.Actor]
	at := ctx.Now.UnixMilli()
	c.Steer = 0
	c.Throttle = 0
	c.Brake = true
	c.Drift = false
	c.Pending = nil
	c.Applied = Input{At: at, Brake: true}
	c.LastInputAt = at
	c.IdleAt = at + idleMS
	return s.match()
}
func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if !repairSequences(&s) {
		return m, nil
	}
	return s.match()
}
func project(m gameapi.Match, seat int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	v := View{Options: s.Options, Phase: s.Phase, Epoch: s.Epoch, ServerAt: s.ServerAt, StartAt: s.StartAt, FinishDeadline: s.FinishDeadline, MaxRaceAt: s.MaxRaceAt, Track: s.Track, Cars: s.Cars, Events: s.Events}
	for i := range v.Cars {
		v.Cars[i].Pending = nil
		v.Cars[i].Applied = Input{}
	}
	if seat >= 0 && seat < len(s.Cars) && s.Result == nil && s.Cars[seat].FinishAt == 0 {
		c := s.Cars[seat]
		v.Controls = Controls{CanDrive: true, CanReset: s.Phase != "countdown" && c.Speed <= 55 && s.ServerAt >= c.ResetUntil && (c.LastReset == 0 || s.ServerAt-c.LastReset >= 5000), CanSkin: true}
	}
	return v, nil
}

// This path only handles impossible/exhausted old snapshots, never ordinary
// packet loss. A fresh epoch prevents replay of old low sequence numbers. Keep
// gameplay and every deadline intact so one bad room cannot block startup.
func repairSequences(s *State) bool {
	repaired := false
	for i := range s.Cars {
		c := &s.Cars[i]
		if c.Seq >= 0 && c.Seq < maxInputSequence {
			continue
		}
		repaired = true
		c.Seq = 0
		c.Steer = 0
		c.Throttle = 0
		c.Brake = true
		c.Drift = false
		c.Pending = nil
		c.Applied = Input{At: s.ServerAt, Brake: true}
	}
	if repaired {
		digest := sha256.Sum256([]byte(s.Epoch + "\x00sequence-repair"))
		s.Epoch = hex.EncodeToString(digest[:12])
		s.Log = append(s.Log, "异常输入序号已恢复，赛程与操作期限保持不变")
	}
	return repaired
}
