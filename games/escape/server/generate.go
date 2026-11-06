package escape

import (
	"fmt"
	"hash/fnv"
	"slices"
)

func (s *State) random(n int) int {
	x := s.Random
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	s.Random = x
	return int(x % uint64(n))
}
func seedNumber(seed string) uint64 {
	h := fnv.New64a()
	h.Write([]byte("escape-generation-1/" + seed))
	return h.Sum64() | 1
}
func (s *State) changed() { s.Revision++ }
func (s *State) emit(kind, text, room string, actor int) {
	s.EventID++
	s.Events = append(s.Events, Event{s.EventID, kind, text, room, actor, s.Tick})
	if len(s.Events) > 16 {
		s.Events = s.Events[len(s.Events)-16:]
	}
	s.changed()
}
func (s *State) generate(n int) {
	s.Random = seedNumber(s.Seed)
	s.Circuits = []string{"lights", "lift"}
	if s.Options.Scenario == "archive" {
		s.Circuits = []string{"lights", "pump"}
	}
	s.Rooms = make([]Room, 9)
	s.Solutions = map[string][]int{}
	s.Solved = map[string]bool{}
	s.Journal = []Clue{}
	s.Events = []Event{}
	s.Votes = make([]bool, n)
	kinds := []string{"storage", "medical", "signal", "power", "pump", "relay", "records"}
	for i := len(kinds) - 1; i > 0; i-- {
		j := s.random(i + 1)
		kinds[i], kinds[j] = kinds[j], kinds[i]
	}
	cursor := 0
	for i := range s.Rooms {
		kind := ""
		if i == 4 {
			kind = "entry"
		} else if i == 8 {
			kind = "exit"
		} else {
			kind = kinds[cursor]
			cursor++
		}
		s.Rooms[i] = Room{ID: fmt.Sprint("r", i), X: i % 3, Y: i / 3, Kind: kind, Title: roomTitle(s.Options.Scenario, kind), Doors: []Door{}, Objects: []Device{}}
	}
	// Randomized spanning tree plus bounded loops. All prerequisites live in
	// reachable, unlocked rooms; routes are objective gates rather than door keys.
	type edge struct{ a, b int }
	edges := []edge{}
	for i := 0; i < 9; i++ {
		if i%3 < 2 {
			edges = append(edges, edge{i, i + 1})
		}
		if i < 6 {
			edges = append(edges, edge{i, i + 3})
		}
	}
	for i := len(edges) - 1; i > 0; i-- {
		j := s.random(i + 1)
		edges[i], edges[j] = edges[j], edges[i]
	}
	parents := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	var root func(int) int
	root = func(i int) int {
		if parents[i] != i {
			parents[i] = root(parents[i])
		}
		return parents[i]
	}
	extras := 0
	for _, e := range edges {
		a, b := root(e.a), root(e.b)
		if a != b {
			parents[a] = b
			s.connect(e.a, e.b)
		} else if extras < 2 {
			s.connect(e.a, e.b)
			extras++
		}
	}
	for _, key := range []string{"power", "pump", "signal"} {
		values := []int{s.random(4), s.random(4), s.random(4)}
		if slices.Equal(values, []int{0, 0, 0}) {
			values[0] = 1
		}
		s.Solutions[key] = values
	}
	for i := range s.Rooms {
		r := &s.Rooms[i]
		add := func(kind, label string, x, y float64, desc string) {
			r.Objects = append(r.Objects, Device{ID: r.ID + "-" + kind, Kind: kind, Label: label, X: x, Y: y, Description: desc})
			if kind == "power" || kind == "pump" || kind == "signal" {
				r.Objects[len(r.Objects)-1].Dials = []int{0, 0, 0}
			}
		}
		switch r.Kind {
		case "entry":
			add("briefing", "值守记录", 500, 350, "调查设施，交换线索，一起找到撤离路线。点击门口可前往相邻房间。")
			r.Objects[0].ID = "briefing"
		case "storage":
			add("supply", "维修补给", 340, 395, "共享补给：保险丝、电池和急救包。每箱仅能领取一次。")
			if s.Options.Scenario == "archive" {
				add("document", "潮汐档案", 670, 395, "密封档案一次只能携带一份；送到撤离室后可继续探索。")
			}
		case "medical":
			add("medical", "急救柜", 350, 395, "补充共享急救包。倒地队友可就近救援。")
			add("clue-pump", "水务手册", 670, 370, "保留下来的阀门设定记录。")
		case "signal":
			add("clue-power", "检修便笺", 365, 370, "设备校准顺序，记录后全队可在笔记中查看。")
			if s.Options.Scenario == "station" {
				add("signal", "信号校准台", 670, 410, "依照记录校准三路信号，排水后可开启检修通道。")
			} else {
				add("document", "星图原卷", 670, 395, "将星图原卷运至撤离室。")
			}
		case "power":
			label, desc := "备用发电机", "需要两枚保险丝，并按检修便笺调节三路负载。"
			if s.Options.Scenario == "archive" {
				label = "天文锁盘"
				desc = "按检修便笺转动三重星环，解开穹顶升降机。"
			}
			add("power", label, 470, 410, desc)
		case "pump":
			add("pump", "总排水阀", 470, 410, "按水务手册校准压力。入水阀会同时改变旁路读数，旁路阀会同时改变排放读数；接通排水供电后减缓进水。")
		case "relay":
			add("relay", "远端联锁", 500, 380, "供电恢复后，与撤离室的另一位队友同时操作。必须留在装置附近维持联锁。")
		case "records":
			add("clue-signal", "信号日志", 365, 370, "记录备用通道所需的信号顺序。")
			add("battery", "应急电源", 680, 400, "独立备用电池，可补充全队电力。")
		case "exit":
			add("relay", "撤离端联锁", 700, 365, "供电恢复后，与远端联锁同时保持；联锁完成后永久开启。")
			label := "撤离升降台"
			if s.Options.Scenario == "archive" {
				label = "水闸码头"
			}
			add("extract", label, 460, 420, "先交付携带物，再选择可用路线并集合。全队确认后共同撤离。")
		}
	}
	s.Players = make([]Player, n)
	for i := range s.Players {
		s.Players[i] = Player{Seat: i, Room: "r4", X: 440 + float64(i%2)*95, Y: 440 + float64(i/2)*65, Facing: "down", HP: 100}
	}
	s.reveal("r4")
	s.emit("arrival", "所有人已抵达，先查看值守记录。", "r4", -1)
}
func roomTitle(scene, kind string) string {
	station := map[string]string{"entry": "候车大厅", "storage": "工具库房", "medical": "值班医务室", "signal": "信号控制室", "power": "备用机房", "pump": "排水泵房", "relay": "中继控制室", "records": "调度档案室", "exit": "撤离站台"}
	archive := map[string]string{"entry": "月影阅览厅", "storage": "修复工坊", "medical": "守馆人休息室", "signal": "星图收藏室", "power": "穹顶钟室", "pump": "地下水务室", "relay": "西翼观测台", "records": "编目室", "exit": "旧馆水闸"}
	if scene == "archive" {
		return archive[kind]
	}
	return station[kind]
}
func (s *State) connect(a, b int) {
	dir, opp := "east", "west"
	if b == a+3 {
		dir, opp = "south", "north"
	}
	for _, v := range []struct {
		from, to int
		dir      string
	}{{a, b, dir}, {b, a, opp}} {
		p := doorPoint(v.dir)
		s.Rooms[v.from].Doors = append(s.Rooms[v.from].Doors, Door{s.Rooms[v.to].ID, v.dir, p.X, p.Y})
	}
}
func doorPoint(dir string) Point {
	switch dir {
	case "north":
		return Point{500, minY}
	case "south":
		return Point{500, maxY}
	case "west":
		return Point{minX, 420}
	default:
		return Point{maxX, 420}
	}
}
func (s *State) room(id string) *Room {
	for i := range s.Rooms {
		if s.Rooms[i].ID == id {
			return &s.Rooms[i]
		}
	}
	return nil
}
func (s *State) device(id string) (*Room, *Device) {
	for i := range s.Rooms {
		for j := range s.Rooms[i].Objects {
			if s.Rooms[i].Objects[j].ID == id {
				return &s.Rooms[i], &s.Rooms[i].Objects[j]
			}
		}
	}
	return nil, nil
}
func (s *State) reveal(id string) {
	r := s.room(id)
	if r != nil && !r.Known {
		r.Known = true
		s.Stats.Explored++
		s.changed()
	}
}
func (s *State) clue(key string) *Clue {
	for i := range s.Journal {
		if s.Journal[i].Key == key {
			return &s.Journal[i]
		}
	}
	return nil
}
