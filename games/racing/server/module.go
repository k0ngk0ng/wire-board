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
		return nil, errors.New("极品飞车支持 2～6 人")
	}
	o := Options{Track: "france", Laps: 3, TimeOfDay: "day"}
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
	if o.TimeOfDay == "" {
		o.TimeOfDay = "day"
	}
	if o.TimeOfDay != "day" && o.TimeOfDay != "night" {
		return nil, errors.New("请选择白天或夜间")
	}
	if len(o.Vehicles) > 6 {
		return nil, errors.New("车型席位超出范围")
	}
	if len(o.Vehicles) > n {
		o.Vehicles = o.Vehicles[:n]
	}
	for len(o.Vehicles) < n {
		defaults := [5]string{"p911", "fm50", "ldia", "mcf1", "ccor"}
		o.Vehicles = append(o.Vehicles, defaults[len(o.Vehicles)%len(defaults)])
	}
	for _, id := range o.Vehicles {
		if !validClassicVehicleID(id) {
			return nil, errors.New("请选择有效车型")
		}
	}
	return json.Marshal(o)
}

func prepare(ctx gameapi.PreparationContext, rawOptions, rawAction json.RawMessage) (json.RawMessage, error) {
	if ctx.Actor < 0 || ctx.Actor >= ctx.Capacity {
		return nil, errors.New("当前不能选择车型")
	}
	var action struct {
		Type    string `json:"type"`
		Vehicle string `json:"vehicle"`
	}
	if err := strict(rawAction, &action); err != nil || action.Type != "vehicle" || !validClassicVehicleID(action.Vehicle) {
		return nil, errors.New("无效的车型选择")
	}
	configured, err := configure(ctx.Capacity, rawOptions)
	if err != nil {
		return nil, err
	}
	var options Options
	_ = json.Unmarshal(configured, &options)
	options.Vehicles[ctx.Actor] = action.Vehicle
	return configure(ctx.Capacity, mustClassicJSON(options))
}

func mustClassicJSON(value any) json.RawMessage { b, _ := json.Marshal(value); return b }

type Action struct {
	Horn        bool     `json:"horn,omitempty"`
	BrakeAmount *float64 `json:"brakeAmount,omitempty"`
	Type        string   `json:"type"`
	Epoch       string   `json:"epoch"`
	Seq         int64    `json:"seq"`
	Steer       float64  `json:"steer"`
	Throttle    float64  `json:"throttle"`
	Brake       bool     `json:"brake"`
	Drift       bool     `json:"drift"`
	Skin        int      `json:"skin"`
}

func Module() gameapi.Module {
	return gameapi.Module{ID: "racing", Name: "极品飞车", MinPlayers: 2, MaxPlayers: 6, Realtime: true, Configure: configure, Prepare: prepare, Start: start, Apply: apply, View: project, BotAction: botAction, Reclaim: reclaim, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer}
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
	s := State{Options: o, Phase: "countdown", Epoch: hex.EncodeToString(token[:]), ServerAt: now, StartAt: now + 3000, MaxRaceAt: now + 1203000, Track: t, Cars: make([]Car, ctx.Players), Events: []Event{}, Log: []string{"极品飞车：3 秒后发车，首车完赛后保留 30 秒冲线时间"}}
	if t.Version == 3 {
		s.Physics = 3
	}
	p, v := t.Points[0], tangent(t, 0)
	for i := range s.Cars {
		behind := 50. + float64(i/2)*55
		side := float64(i%2*2-1) * 22
		c := Car{Seat: i, X: p.X - v.X*behind - v.Y*side, Y: p.Y - v.Y*behind + v.X*side, Angle: math.Atan2(v.Y, v.X), Checkpoint: 0, Gate: -1, Progress: -behind, Rank: i/2 + 1, LastInputAt: now, IdleAt: now + idleMS, Skin: i % 6, Applied: Input{At: now, Brake: true}}
		if s.Physics == 3 {
			c.Vehicle = o.Vehicles[i]
			c.Height = t.Road[0].Height
			classicPose(&c, t)
			motion := classicMotionFromCar(c)
			motion.Brake = 1
			classicMotionToCar(&c, motion)
			c.Applied.Protect = true
		}
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
		if a.BrakeAmount != nil && (!finite(*a.BrakeAmount) || *a.BrakeAmount < 0 || *a.BrakeAmount > 1 || a.Brake != (*a.BrakeAmount > 0)) {
			return a, errors.New("刹车力度超出范围或状态不一致")
		}
		if a.Seq <= c.Seq || a.Seq <= 0 || a.Seq > maxInputSequence || c.Seq < 0 || c.Seq >= maxInputSequence || a.Seq-c.Seq > maxSequenceJump || !finite(a.Steer) || a.Steer < -1 || a.Steer > 1 || !finite(a.Throttle) || a.Throttle < 0 || a.Throttle > 1 {
			return a, errors.New("输入已过期或超出范围")
		}
	case "reset":
		if s.Phase == "countdown" || c.Speed > 55 || ctx.Now.UnixMilli() < c.ResetUntil || c.LastReset > 0 && ctx.Now.UnixMilli()-c.LastReset < 5000 {
			return a, errors.New("请减速后复位，复位冷却 5 秒")
		}
	case "skin":
		if a.Skin < 0 || a.Skin > 5 {
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
		c.BrakeAmount = a.BrakeAmount
		c.Drift = a.Drift
		c.LastInputAt = at
		c.IdleAt = at + idleMS
		in := Input{At: at, Steer: a.Steer, Throttle: a.Throttle, Brake: a.Brake, BrakeAmount: a.BrakeAmount, Drift: a.Drift, Horn: a.Horn}
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
		c.Horn = false
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
		c.Applied = Input{At: at, Brake: true, Protect: true}
		c.Steer = 0
		c.Throttle = 0
		c.Brake = true
		c.BrakeAmount = nil
		c.Drift = false
		c.LastInputAt = at
		c.IdleAt = at + idleMS
		if s.Physics == 3 {
			c.RouteIndex = i
			c.Height = s.Track.Road[i].Height
			c.Motion = nil
			c.Driver = nil
			motion := classicMotionFromCar(*c)
			motion.Brake = 1
			classicMotionToCar(c, motion)
			classicPose(c, s.Track)
		}
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
	c.Horn = false
	c.Steer = 0
	c.Throttle = 0
	c.Brake = true
	c.BrakeAmount = nil
	c.Drift = false
	c.Pending = nil
	c.Applied = Input{At: at, Brake: true, Protect: true}
	c.LastInputAt = at
	c.IdleAt = at + idleMS
	if c.Motion != nil {
		c.Motion.Throttle = 0
		c.Motion.Brake = 1
		c.Motion.ReverseTimer = 0
	}
	c.Driver = nil
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
	v := View{Physics: s.Physics, Options: s.Options, Phase: s.Phase, Epoch: s.Epoch, ServerAt: s.ServerAt, StartAt: s.StartAt, FinishDeadline: s.FinishDeadline, MaxRaceAt: s.MaxRaceAt, Track: s.Track, Cars: s.Cars, Events: s.Events}
	if s.Physics == 3 {
		v.Track = compactTrack(v.Track)
		v.Events = append([]Event{}, v.Events...)
		for i := range v.Events {
			e := &v.Events[i]
			e.X = math.Round(e.X*1000) / 1000
			e.Y = math.Round(e.Y*1000) / 1000
			e.Intensity = math.Round(e.Intensity*1000) / 1000
		}
	}
	for i := range v.Cars {
		c := &v.Cars[i]
		c.Horn = s.Phase != "finished" && c.Applied.Horn && !c.Applied.Protect && s.ServerAt-c.Applied.At <= lostInputMS && s.ServerAt >= c.ResetUntil && c.FinishAt == 0
		if s.Physics == 3 {
			c := &v.Cars[i]
			c.Protected = c.Applied.Protect || s.ServerAt-c.Applied.At > lostInputMS || s.ServerAt < c.ResetUntil
		}
		v.Cars[i].Pending = nil
		v.Cars[i].Applied = Input{}
		v.Cars[i].Driver = nil
		if s.Physics == 3 {
			compactClassicCar(&v.Cars[i])
		}
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
		c.Horn = false
		c.Seq = 0
		c.Steer = 0
		c.Throttle = 0
		c.Brake = true
		c.BrakeAmount = nil
		c.Drift = false
		c.Pending = nil
		c.Applied = Input{At: s.ServerAt, Brake: true, Protect: true}
		if c.Motion != nil {
			c.Motion.Throttle = 0
			c.Motion.Brake = 1
			c.Motion.ReverseTimer = 0
		}
		c.Driver = nil
	}
	if repaired {
		digest := sha256.Sum256([]byte(s.Epoch + "\x00sequence-repair"))
		s.Epoch = hex.EncodeToString(digest[:12])
		s.Log = append(s.Log, "异常输入序号已恢复，赛程与操作期限保持不变")
	}
	return repaired
}
