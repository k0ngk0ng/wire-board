package escape

import (
	"math"
	"slices"
)

func (s State) powered(key string) bool { return slices.Contains(s.Circuits, key) && s.Power > 0 }

func (s State) routes() []Route {
	if s.Options.Scenario == "archive" {
		return []Route{{"boat", "水闸救生艇", s.Solved["pump"] && s.powered("pump") && s.Supplies.Documents >= 2, "排水供电 · 两份档案送达"}, {"lift", "穹顶吊笼", s.Solved["power"] && s.Gate && s.powered("lift"), "天文锁盘 · 联锁与升降供电"}}
	}
	return []Route{{"lift", "货运升降机", s.Solved["power"] && s.Gate && s.powered("lift"), "备用供电 · 联锁与升降供电"}, {"passage", "检修通道", s.Solved["pump"] && s.powered("pump") && s.Solved["signal"], "排水供电 · 信号校准"}}
}
func (s State) view(who int) View {
	players := append([]Player{}, s.Players...)
	for i := range players {
		players[i].Queue = ""
		players[i].Mode = ""
		players[i].Work = 0
		players[i].InputUntil = 0
		players[i].MotionAt = 0
	}
	rooms := make([]Room, len(s.Rooms))
	water := make([]float64, len(s.Rooms))
	for i, r := range s.Rooms {
		if r.Known {
			water[i] = math.Round(r.Flood*10) / 10
			rooms[i] = r
			rooms[i].Flood = math.Round(r.Flood*10) / 10
		} else {
			water[i] = -1
			rooms[i] = Room{ID: r.ID, X: r.X, Y: r.Y, Known: false}
		}
	}
	powerLabel := "修复备用供电"
	if s.Options.Scenario == "archive" {
		powerLabel = "解开天文锁盘"
	}
	objectives := []Objective{{"power", powerLabel, s.Solved["power"], ""}, {"pump", "恢复排水", s.Solved["pump"] && s.powered("pump"), ""}}
	if s.Options.Scenario == "station" {
		objectives = append(objectives, Objective{"relay", "两端联锁同步", s.Gate, ""}, Objective{"signal", "校准备用信号", s.Solved["signal"], ""})
	} else {
		objectives = append(objectives, Objective{"documents", "运回两份档案", s.Supplies.Documents >= 2, fmtCount(s.Supplies.Documents, 2)}, Objective{"relay", "启动穹顶联锁", s.Gate, ""})
	}
	v := View{Water: water, Rally: s.Rally, Circuits: s.Circuits, Protocol: 1, Options: s.Options, Epoch: s.Epoch, Phase: s.Phase, Tick: s.Tick, ServerAt: s.ServerAt, Remaining: max(0, s.Duration-s.Tick), Power: math.Round(s.Power*10) / 10, Players: players, Rooms: rooms, Journal: append([]Clue{}, s.Journal...), Supplies: s.Supplies, Objectives: objectives, Routes: s.routes(), Incident: s.Incident, Events: append([]Event{}, s.Events...), Revision: s.Revision, Paused: s.Paused, Votes: append([]bool{}, s.Votes...), CanAct: who >= 0 && who < len(s.Players) && s.Phase == "playing" && !s.Paused, You: who, Stats: s.Stats, Reason: s.Reason}
	if s.Phase == "finished" {
		v.Seed = s.Seed
	}
	return v
}
func fmtCount(a, b int) string { return string(rune('0'+a)) + " / " + string(rune('0'+b)) }
