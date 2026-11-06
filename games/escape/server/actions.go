package escape

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func distance(x, y, tx, ty float64) float64 { return math.Hypot(x-tx, y-ty) }
func (s *State) stop(who int) {
	p := &s.Players[who]
	p.DX = 0
	p.DY = 0
	p.InputUntil = 0
	p.Queue = ""
	p.Mode = ""
	p.Goal = nil
	p.Task = ""
	p.Work = 0
	p.Progress = 0
}
func (s *State) queue(who int, target, mode string, point Point) {
	s.stop(who)
	p := &s.Players[who]
	p.Queue = target
	p.Mode = mode
	p.Goal = &point
	p.Ready = false
	p.Task = "前往目标"
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || len(raw) > 4096 {
		return m, errors.New("当前不能操作")
	}
	var a Action
	if strict(raw, &a) != nil || a.Epoch != s.Epoch {
		return m, errors.New("操作已过期，请重新连接")
	}
	p := &s.Players[ctx.Actor]
	if a.Seq <= p.Seq || a.Seq-p.Seq > 1024 {
		return m, errors.New("操作序号已过期")
	}
	if s.Phase != "playing" {
		return m, errors.New("本局已结束")
	}
	if a.Type == "pause" || a.Type == "resume" {
		want := a.Type == "pause"
		if s.Paused == want {
			return m, errors.New("暂停状态已变化")
		}
		s.Votes[ctx.Actor] = !s.Votes[ctx.Actor]
		all := true
		for i, vote := range s.Votes {
			automatic := i < len(ctx.Automated) && ctx.Automated[i]
			if !vote && !automatic {
				all = false
			}
		}
		if all {
			s.Paused = want
			s.ServerAt = ctx.Now.UnixMilli()
			for i := range s.Players {
				s.stop(i)
				s.Votes[i] = false
			}
			verb := "已恢复探索"
			if want {
				verb = "全队已暂停"
			}
			s.emit("pause", verb, p.Room, ctx.Actor)
		}
		s.Players[ctx.Actor].Seq = a.Seq
		s.changed()
		return s.match()
	}
	if s.Paused {
		return m, errors.New("请先投票恢复探索")
	}
	if math.IsNaN(a.DX) || math.IsNaN(a.DY) || math.Abs(a.DX) > 1 || math.Abs(a.DY) > 1 {
		return m, errors.New("移动方向无效")
	}
	if a.Type != "ping" && p.HP <= 0 {
		return m, errors.New("你已倒地，请标记位置等待队友救援")
	}
	if p.HP > 0 {
		now := ctx.Now.UnixMilli()
		movePlayer(p, min(int64(250), max(int64(0), now-max(p.MotionAt, s.ServerAt))), now)
		p.MotionAt = now
		p.AppliedSeq = p.Seq
	}
	switch a.Type {
	case "plan":
		label := ""
		for _, route := range s.routes() {
			if route.ID == a.Route {
				label = route.Label
			}
		}
		if label == "" {
			return m, errors.New("撤离路线无效")
		}
		if s.Stats.Route == a.Route {
			break
		}
		s.Stats.Route = a.Route
		for i := range s.Players {
			s.Players[i].Ready = false
		}
		s.emit("plan", "队伍计划改为「"+label+"」。", p.Room, ctx.Actor)
	case "input":
		if a.Target != "" || a.X != 0 || a.Y != 0 || a.Index != 0 || a.Route != "" {
			return m, errors.New("移动请求包含无关字段")
		}
		s.stop(ctx.Actor)
		p = &s.Players[ctx.Actor]
		p.DX = a.DX
		p.DY = a.DY
		p.InputUntil = ctx.Now.UnixMilli() + 500
		if a.DX != 0 || a.DY != 0 {
			p.Ready = false
		}
	case "walk":
		if math.IsNaN(a.X) || math.IsNaN(a.Y) || a.X < minX || a.X > maxX || a.Y < minY || a.Y > maxY {
			return m, errors.New("该位置不能通行")
		}
		s.queue(ctx.Actor, "walk", "walk", Point{a.X, a.Y})
	case "travel":
		r := s.room(p.Room)
		found := false
		for _, d := range r.Doors {
			if d.Room == a.Target {
				s.queue(ctx.Actor, d.Room, "travel", Point{d.X, d.Y})
				found = true
				break
			}
		}
		if !found {
			return m, errors.New("这里没有通向目标的门")
		}
	case "inspect", "interact":
		r, d := s.device(a.Target)
		if d == nil || r.ID != p.Room {
			return m, errors.New("目标不在当前房间")
		}
		s.queue(ctx.Actor, d.ID, a.Type, Point{d.X, d.Y})
		if distance(p.X, p.Y, d.X, d.Y) < 52 {
			if e = s.perform(ctx.Actor, d.ID, a.Type, a.Route); e != nil {
				return m, e
			}
		} else {
			s.Players[ctx.Actor].Mode = a.Type + ":" + a.Route
		}
	case "turn":
		r, d := s.device(a.Target)
		if d == nil || r.ID != p.Room || len(d.Dials) != 3 || a.Index < 0 || a.Index > 2 || d.Done || distance(p.X, p.Y, d.X, d.Y) > 82 {
			return m, errors.New("请靠近尚未完成的校准装置")
		}
		s.stop(ctx.Actor)
		d.Dials[a.Index] = (d.Dials[a.Index] + 1) % 4
		if (d.Kind == "pump" || d.Kind == "power" && s.Options.Scenario == "archive") && a.Index < 2 {
			d.Dials[a.Index+1] = (d.Dials[a.Index+1] + 1) % 4
		}
		p.Focus = d.ID
		s.changed()
	case "circuit":
		r, d := s.device(a.Target)
		if d == nil || r.ID != p.Room || d.Kind != "power" || distance(p.X, p.Y, d.X, d.Y) > 82 || !slices.Contains([]string{"lights", "pump", "lift"}, a.Circuit) {
			return m, errors.New("请在配电装置附近选择有效线路")
		}
		if i := slices.Index(s.Circuits, a.Circuit); i >= 0 {
			s.Circuits = slices.Delete(s.Circuits, i, i+1)
		} else {
			if len(s.Circuits) >= 2 {
				return m, errors.New("电源最多维持两条线路，请先断开一条")
			}
			s.Circuits = append(s.Circuits, a.Circuit)
		}
		s.emit("circuit", "配电线路已调整，请留意队友与撤离设备。", p.Room, ctx.Actor)
	case "med":
		if s.Supplies.Meds < 1 || p.HP >= 100 {
			return m, errors.New("没有可用急救包，或无需治疗")
		}
		s.Supplies.Meds--
		p.HP = math.Min(100, p.HP+50)
		s.emit("heal", "使用了共享急救包。", p.Room, ctx.Actor)
	case "cell":
		if s.Supplies.Cells < 1 || s.Power >= 99 {
			return m, errors.New("没有备用电池，或当前电力充足")
		}
		s.Supplies.Cells--
		s.Power = math.Min(100, s.Power+30)
		s.emit("power", "备用电池已接入。", p.Room, ctx.Actor)
	case "rescue":
		id, err := strconv.Atoi(strings.TrimPrefix(a.Target, "player-"))
		if err != nil || id < 0 || id >= len(s.Players) || id == ctx.Actor || s.Players[id].Room != p.Room || s.Players[id].HP > 0 {
			return m, errors.New("这里没有待救援的队友")
		}
		q := s.Players[id]
		s.queue(ctx.Actor, a.Target, "rescue", Point{q.X, q.Y})
	case "ping":
		s.Rally = &Rally{Room: p.Room, Until: s.Tick + 20000, Actor: ctx.Actor}
		s.emit("ping", "队友标记了位置，需要协助。", p.Room, ctx.Actor)
	default:
		return m, errors.New("未知密室操作")
	}
	s.Players[ctx.Actor].Seq = a.Seq
	return s.match()
}
func (s *State) perform(who int, id, mode, route string) error {
	r, d := s.device(id)
	if d == nil {
		return errors.New("目标已失效")
	}
	p := &s.Players[who]
	s.stop(who)
	p.Focus = id
	if mode == "inspect" && (len(d.Dials) > 0 || d.Kind == "relay" || d.Kind == "extract") {
		return nil
	}
	if d.Done && d.Kind != "relay" && d.Kind != "extract" {
		return nil
	}
	switch d.Kind {
	case "briefing":
		if s.clue("briefing") == nil {
			title, body := "最后一班", "供电、远端联锁可启动升降机；排水与信号校准可开启检修通道。全队在站台集合。"
			if s.Options.Scenario == "archive" {
				title = "守馆人遗言"
				body = "排水后，将两份密封档案送至水闸可乘艇离开；也可解开天文锁盘并联动观测台，启用穹顶吊笼。"
			}
			s.Journal = append(s.Journal, Clue{Key: "briefing", Title: title, Text: body, Room: r.ID})
			s.emit("clue", "已记录撤离方案。", r.ID, who)
		}
		d.Done = true
	case "supply":
		s.Supplies.Fuses += 4
		s.Supplies.Cells += 2
		s.Supplies.Meds++
		d.Done = true
		s.emit("supply", "已收集维修补给，资源由全队共享。", r.ID, who)
	case "medical":
		s.Supplies.Meds += 2
		d.Done = true
		s.emit("supply", "发现两份急救包。", r.ID, who)
	case "battery":
		s.Supplies.Cells++
		d.Done = true
		s.emit("supply", "找到一枚备用电池。", r.ID, who)
	case "clue-power", "clue-pump", "clue-signal":
		key := strings.TrimPrefix(d.Kind, "clue-")
		if s.clue(key) == nil {
			values := append([]int{}, s.Solutions[key]...)
			label := map[string]string{"power": "三路负载", "pump": "入水／旁路／排放", "signal": "三路信号"}[key]
			if key == "power" && s.Options.Scenario == "archive" {
				label = "内环／中环／外环"
			}
			s.Journal = append(s.Journal, Clue{Key: key, Title: d.Label, Text: fmt.Sprintf("%s：%d · %d · %d", label, values[0]+1, values[1]+1, values[2]+1), Values: values, Room: r.ID})
			s.emit("clue", "已记录「"+d.Label+"」，全队可查阅。", r.ID, who)
		}
		d.Done = true
	case "power", "pump", "signal":
		if d.Kind == "power" && s.Options.Scenario == "station" && s.Supplies.Fuses < 2 {
			return errors.New("需要两枚保险丝，请先寻找维修补给")
		}
		if !slices.Equal(d.Dials, s.Solutions[d.Kind]) {
			s.Power = math.Max(0, s.Power-4)
			s.Stats.Mistakes++
			s.emit("mistake", "校准失败，设备消耗了额外电力。", r.ID, who)
			return nil
		}
		if d.Kind == "power" {
			if s.Options.Scenario == "station" {
				s.Supplies.Fuses -= 2
			}
			s.Power = math.Min(100, s.Power+30)
		}
		s.Solved[d.Kind] = true
		d.Done = true
		s.emit("solved", "「"+d.Label+"」已恢复运转。", r.ID, who)
	case "document":
		if p.Carry != "" {
			return errors.New("每人只能携带一份档案，请先送至水闸")
		}
		p.Carry = d.ID
		d.Done = true
		s.emit("document", "取走「"+d.Label+"」，需要送至水闸。", r.ID, who)
	case "relay":
		if !s.Solved["power"] {
			return errors.New("请先恢复发电机或解开天文锁盘")
		}
		if s.Gate {
			return nil
		}
		d.HeldUntil = s.Tick + 12000
		s.emit("relay", "联锁已接通，请留在装置附近等待另一端。", r.ID, who)
		s.checkRelay()
	case "extract":
		if p.Carry != "" {
			p.Carry = ""
			s.Supplies.Documents++
			s.emit("deposit", "密封档案已安全送达水闸。", r.ID, who)
			return nil
		}
		routes := s.routes()
		if route == "" && s.Stats.Route != "" {
			route = s.Stats.Route
		}
		if route == "" {
			for _, v := range routes {
				if v.Available {
					route = v.ID
					break
				}
			}
		}
		available := false
		for _, v := range routes {
			if v.ID == route && v.Available {
				available = true
			}
		}
		if !available {
			return errors.New("撤离路线尚未就绪，请查看任务与笔记")
		}
		if s.Stats.Route != "" && s.Stats.Route != route {
			return errors.New("队伍正在集合，请使用已选撤离路线")
		}
		s.Stats.Route = route
		p.Ready = !p.Ready
		s.emit("ready", "一名队友已在撤离点就绪。", r.ID, who)
		s.checkVictory()
	}
	s.changed()
	return nil
}
func (s *State) checkRelay() {
	held := 0
	for _, r := range s.Rooms {
		for _, d := range r.Objects {
			if d.Kind != "relay" || d.HeldUntil <= s.Tick {
				continue
			}
			for _, p := range s.Players {
				if p.Room == r.ID && p.HP > 0 && distance(p.X, p.Y, d.X, d.Y) < 95 {
					held++
					break
				}
			}
		}
	}
	if held >= 2 && !s.Gate {
		s.Gate = true
		s.emit("solved", "双端联锁完成，升降路线永久开启。", "", -1)
	}
}
func (s *State) checkVictory() {
	for _, p := range s.Players {
		r := s.room(p.Room)
		if r == nil || r.Kind != "exit" || !p.Ready || p.HP <= 0 || p.Carry != "" {
			return
		}
	}
	s.Phase = "finished"
	s.Victory = true
	s.Reason = "全员撤离"
	s.Paused = false
	s.emit("victory", "所有队友已安全撤离。", "", -1)
}
