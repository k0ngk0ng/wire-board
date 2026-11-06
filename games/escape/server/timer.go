package escape

import (
	"math"
	"strconv"
	"strings"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if s.Paused || s.Phase != "playing" {
		return m, nil
	}
	now := ctx.Now.UnixMilli()
	dt := min(int64(250), max(int64(0), now-s.ServerAt))
	if dt == 0 {
		return m, nil
	}
	prior := s.ServerAt
	s.ServerAt = now
	oldSecond := s.Tick / 1000
	s.Tick += dt
	for i := range s.Players {
		p := &s.Players[i]
		if p.HP <= 0 {
			s.stop(i)
			continue
		}
		movementMS := min(dt, max(int64(0), now-max(p.MotionAt, prior)))
		p.MotionAt = now
		p.AppliedSeq = p.Seq
		movePlayer(p, movementMS, now)
		if p.Goal == nil || distance(p.X, p.Y, p.Goal.X, p.Goal.Y) > 3 {
			continue
		}
		queue, mode := p.Queue, p.Mode
		switch {
		case mode == "travel":
			old := s.room(p.Room)
			var dir string
			for _, d := range old.Doors {
				if d.Room == queue {
					dir = d.Direction
				}
			}
			s.stop(i)
			p.Room = queue
			switch dir {
			case "north":
				p.Y = maxY - 40
				p.X = 500
			case "south":
				p.Y = minY + 40
				p.X = 500
			case "east":
				p.X = minX + 40
				p.Y = 420
			case "west":
				p.X = maxX - 40
				p.Y = 420
			}
			p.Focus = ""
			p.Ready = false
			s.reveal(queue)
			s.changed()
		case mode == "walk":
			s.stop(i)
		case mode == "rescue":
			id, err := strconv.Atoi(strings.TrimPrefix(queue, "player-"))
			if err != nil || id < 0 || id >= len(s.Players) || s.Players[id].HP > 0 || s.Players[id].Room != p.Room {
				s.stop(i)
				continue
			}
			p.Work += dt
			p.Progress = float64(p.Work) / 3000
			p.Task = "救援队友"
			if p.Work >= 3000 {
				hp := 30.0
				if s.Supplies.Meds > 0 {
					s.Supplies.Meds--
					hp = 60
				}
				s.Players[id].HP = hp
				s.Stats.Rescues++
				s.stop(i)
				s.emit("rescue", "队友已获救。", p.Room, i)
			}
		default:
			parts := strings.SplitN(mode, ":", 2)
			route := ""
			if len(parts) > 1 {
				route = parts[1]
			}
			if err := s.perform(i, queue, parts[0], route); err != nil {
				s.stop(i)
				s.emit("notice", err.Error(), p.Room, i)
			}
		}
	}
	if s.Tick/1000 != oldSecond {
		s.survival()
	}
	s.checkRelay()
	if s.Tick >= s.Duration && s.Phase == "playing" {
		s.finishLoss("设施已封闭，未能及时撤离")
	}
	living := 0
	for _, p := range s.Players {
		if p.HP > 0 {
			living++
		}
	}
	if living == 0 && s.Phase == "playing" {
		s.finishLoss("全队倒地，救援信号中断")
	}
	return s.match()
}
func movePlayer(p *Player, dt, now int64) {
	dx, dy := p.DX, p.DY
	if now > p.InputUntil {
		dx, dy = 0, 0
		p.DX = 0
		p.DY = 0
	}
	if p.Goal != nil {
		d := distance(p.X, p.Y, p.Goal.X, p.Goal.Y)
		if d > 2 {
			dx = (p.Goal.X - p.X) / d
			dy = (p.Goal.Y - p.Y) / d
		} else {
			dx, dy = 0, 0
		}
	}
	length := math.Hypot(dx, dy)
	if length > 1 {
		dx /= length
		dy /= length
	}
	rate := speed
	if p.Carry != "" {
		rate *= 0.72
	}
	if p.HP < 35 {
		rate *= 0.8
	}
	step := rate * float64(dt) / 1000
	if p.Goal != nil {
		d := distance(p.X, p.Y, p.Goal.X, p.Goal.Y)
		step = math.Min(step, d)
	}
	p.X = math.Max(minX, math.Min(maxX, p.X+dx*step))
	p.Y = math.Max(minY, math.Min(maxY, p.Y+dy*step))
	if math.Abs(dx) > math.Abs(dy) {
		if dx > 0 {
			p.Facing = "right"
		} else {
			p.Facing = "left"
		}
	} else if dy != 0 {
		if dy > 0 {
			p.Facing = "down"
		} else {
			p.Facing = "up"
		}
	}
}

func (s *State) finishLoss(reason string) {
	s.Phase = "finished"
	s.Victory = false
	s.Reason = reason
	s.Paused = false
	s.emit("defeat", reason, "", -1)
}
func (s *State) survival() {
	difficulty := 1.0
	if s.Options.Difficulty == "easy" {
		difficulty = .6
	}
	if s.Options.Difficulty == "hard" {
		difficulty = 1.4
	}
	drain := .18 * difficulty
	if s.Solved["power"] {
		drain *= .35
	}
	s.Power = math.Max(0, s.Power-drain)
	if s.Power < 25 && s.Tick%30000 < 1000 {
		s.emit("warning", "电力不足，可使用共享备用电池。", "", -1)
	}
	for i := range s.Rooms {
		r := &s.Rooms[i]
		water := float64(s.Tick) / 1000 * .045 * difficulty
		if s.Options.Scenario == "archive" {
			water *= 1.4
		}
		if s.Solved["pump"] && s.powered("pump") {
			water *= .18
		}
		r.Flood = math.Min(95, math.Max(0, water+float64(r.Y)*3-6))
	}
	if s.Incident != nil && s.Tick >= s.Incident.Ends {
		s.Incident = nil
		s.changed()
	}
	if s.Rally != nil && s.Tick >= s.Rally.Until {
		s.Rally = nil
		s.changed()
	}
	if s.Incident == nil && s.Tick >= s.NextIncident {
		id := s.random(9)
		kind := "steam"
		if s.Options.Scenario == "archive" {
			kind = "surge"
		}
		s.Incident = &Incident{Room: s.Rooms[id].ID, Kind: kind, Begins: s.Tick + 12000, Ends: s.Tick + 29000}
		s.NextIncident = s.Tick + int64(70000/difficulty)
		s.emit("warning", "设施出现异常，标记区域将在 12 秒后发生险情。", s.Rooms[id].ID, -1)
	}
	for i := range s.Players {
		p := &s.Players[i]
		if p.HP <= 0 {
			continue
		}
		damage := 0.0
		r := s.room(p.Room)
		if r.Flood > 75 {
			damage += 2 * difficulty
		}
		if s.Incident != nil && s.Incident.Room == p.Room && s.Tick >= s.Incident.Begins && s.Tick < s.Incident.Ends {
			damage += 3 * difficulty
		}
		if s.Power <= 0 {
			damage += .5 * difficulty
		}
		if damage > 0 {
			p.HP = math.Max(0, p.HP-damage)
			if p.HP == 0 {
				p.Ready = false
				s.stop(i)
				s.emit("down", "一名队友倒地，请靠近救援。", p.Room, i)
			}
		}
	}
}
