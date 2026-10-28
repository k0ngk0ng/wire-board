// Package starfort owns the cooperative 星垒 simulation and saved state.
package starfort

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"math"
	"slices"
)

const StepMS = 200

// All summons and split children share this simultaneous-entity budget.
const MaxEnemies = 64
const MaxWaveEnemies = 44
const MaxEvents = 1024
const maxSnapshotBytes = 2 * 1024 * 1024
const maxValue = 1e12

type Options struct {
	Mode        string `json:"mode"`
	Waves       int    `json:"waves"`
	Difficulty  string `json:"difficulty"`
	PrepSeconds int    `json:"prepSeconds"`
}
type Player struct {
	Gold       int     `json:"gold"`
	Kills      int     `json:"kills"`
	Damage     float64 `json:"damage"`
	Ready      bool    `json:"ready"`
	PauseVote  bool    `json:"pauseVote"`
	ResumeVote bool    `json:"resumeVote"`
}
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Pad struct {
	ID      int     `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Terrain string  `json:"terrain"`
	Sector  int     `json:"sector"`
}
type Map struct {
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Path     []Point   `json:"path"`
	Pads     []Pad     `json:"pads"`
	Base     Point     `json:"base"`
	Routes   []Route   `json:"routes"`
	Regions  []Region  `json:"regions"`
	Features []Feature `json:"features"`
}

var board = Map{Width: 960, Height: 640, Path: []Point{{-30, 120}, {250, 120}, {250, 300}, {700, 300}, {700, 500}, {1000, 500}}, Pads: []Pad{{ID: 0, X: 150, Y: 210, Terrain: "plain"}, {ID: 1, X: 340, Y: 210, Terrain: "plain"}, {ID: 2, X: 470, Y: 210, Terrain: "plain"}, {ID: 3, X: 590, Y: 210, Terrain: "plain"}, {ID: 4, X: 790, Y: 390, Terrain: "plain"}, {ID: 5, X: 610, Y: 400, Terrain: "plain"}, {ID: 6, X: 470, Y: 390, Terrain: "plain"}, {ID: 7, X: 340, Y: 400, Terrain: "plain"}, {ID: 8, X: 160, Y: 350, Terrain: "plain"}, {ID: 9, X: 810, Y: 570, Terrain: "plain"}, {ID: 10, X: 890, Y: 400, Terrain: "plain"}, {ID: 11, X: 610, Y: 570, Terrain: "plain"}, {ID: 12, X: 140, Y: 40, Terrain: "plain"}, {ID: 13, X: 340, Y: 70, Terrain: "plain"}, {ID: 14, X: 790, Y: 210, Terrain: "plain"}, {ID: 15, X: 900, Y: 570, Terrain: "plain"}}}

type TowerSpec struct {
	Kind        string  `json:"kind"`
	Name        string  `json:"name"`
	Cost        int     `json:"cost"`
	Range       float64 `json:"range"`
	Damage      float64 `json:"damage"`
	Interval    int     `json:"interval"`
	Description string  `json:"description"`
}

var catalog = []TowerSpec{
	{"arrow", "风羽箭塔", 90, 190, 22, 600, "快速单体攻击；适合追击快速敌人"},
	{"cannon", "星火炮塔", 140, 210, 48, 1600, "爆炸伤害覆盖周围敌人；适合密集编队"},
	{"frost", "霜晶塔", 110, 175, 10, 1000, "命中后减速 45%，持续 1.6 秒"},
	{"arcane", "辉光法师塔", 130, 195, 48, 1200, "魔法忽略护甲，对护盾造成额外伤害"},
}

type Tower struct {
	PassiveReadyAt int64   `json:"passiveReadyAt,omitempty"`
	FocusTarget    int     `json:"focusTarget,omitempty"`
	FocusHits      int     `json:"focusHits,omitempty"`
	ID             int     `json:"id"`
	Pad            int     `json:"pad"`
	Owner          int     `json:"owner"`
	Kind           string  `json:"kind"`
	Level          int     `json:"level"`
	Branch         string  `json:"branch"`
	Angle          float64 `json:"angle"`
	Cooldown       int     `json:"cooldown"`
	Spent          int     `json:"spent"`
	Priority       string  `json:"priority"`
}
type Enemy struct {
	Level         int     `json:"level,omitempty"`
	Frostbound    bool    `json:"frostbound,omitempty"`
	Repairing     int     `json:"repairing,omitempty"`
	RepairSinceMS int64   `json:"repairSinceMS,omitempty"`
	ID            int     `json:"id"`
	Kind          string  `json:"kind"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
	Progress      float64 `json:"progress"`
	HP            float64 `json:"hp"`
	MaxHP         float64 `json:"maxHP"`
	Shield        float64 `json:"shield"`
	Speed         float64 `json:"speed"`
	SlowUntil     int64   `json:"slowUntil"`
	Armor         float64 `json:"armor"`
	Reward        int     `json:"reward"`
	Leak          int     `json:"leak"`
	Route         string  `json:"route"`
	Phase         int     `json:"phase"`
	AbilityTick   int64   `json:"abilityTick"`
}
type WaveInfo struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Rule        string `json:"rule"`
	Total       int    `json:"total"`
	Spawned     int    `json:"spawned"`
	Boss        bool   `json:"boss"`
}

// DamageDelta is an authoritative display delta for additional AoE victims.
// Grouping one blast avoids repeating a complete Enemy object per victim.
type DamageDelta struct {
	ID     int     `json:"id"`
	Amount float64 `json:"amount"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
}
type DamageGroup struct {
	Amount  float64 `json:"amount"`
	Targets []int   `json:"targets"`
}

type Event struct {
	DamageGroups []DamageGroup `json:"damageGroups,omitempty"`
	IceWallID    int           `json:"iceWallId,omitempty"`
	IceWall      *ViewIceWall  `json:"iceWall,omitempty"`
	SlowTargets  []int         `json:"slowTargets,omitempty"`
	SlowUntil    int64         `json:"slowUntil,omitempty"`
	Damage       []DamageDelta `json:"damage,omitempty"`
	Skill        string        `json:"skill,omitempty"`
	CraterID     int           `json:"craterId,omitempty"`
	Crater       *Crater       `json:"crater,omitempty"`
	// AtMS is optional for schema-1/2 saves created before the combat clock.
	// A pointer preserves the difference between an old event and time zero.
	AtMS     *int64     `json:"atMS,omitempty"`
	TowerID  int        `json:"towerId,omitempty"`
	Target   *ViewEnemy `json:"target,omitempty"`
	TargetID int        `json:"targetId,omitempty"`
	ID       int        `json:"id"`
	Type     string     `json:"type"`
	X        float64    `json:"x"`
	Y        float64    `json:"y"`
	ToX      float64    `json:"toX,omitempty"`
	ToY      float64    `json:"toY,omitempty"`
	Kind     string     `json:"kind,omitempty"`
	Owner    int        `json:"owner"`
	Amount   float64    `json:"amount,omitempty"`
	Tick     int64      `json:"tick"`
}
type State struct {
	SkillRules     int       `json:"skillRules,omitempty"`
	SkillRNG       uint64    `json:"skillRng,omitempty"`
	IceWalls       []IceWall `json:"iceWalls,omitempty"`
	IceWallSerial  int       `json:"iceWallSerial,omitempty"`
	Craters        []Crater  `json:"craters,omitempty"`
	CraterSerial   int       `json:"craterSerial,omitempty"`
	Options        Options   `json:"options"`
	World          World     `json:"world"`
	Players        []Player  `json:"players"`
	Phase          string    `json:"phase"`
	Epoch          int       `json:"epoch"`
	Wave           int       `json:"wave"`
	CompletedWaves int       `json:"completedWaves"`
	BaseHP         int       `json:"baseHP"`
	BaseMaxHP      int       `json:"baseMaxHP"`
	Kills          int       `json:"kills"`
	Tick           int64     `json:"tick"`
	ServerAt       int64     `json:"serverAt"`
	NextAt         int64     `json:"nextAt"`
	ElapsedMS      int64     `json:"elapsedMS"`
	Towers         []Tower   `json:"towers"`
	Enemies        []Enemy   `json:"enemies"`
	WaveInfo       WaveInfo  `json:"waveInfo"`
	Events         []Event   `json:"events"`
	SpawnClock     int       `json:"spawnClock"`
	TowerSerial    int       `json:"towerSerial"`
	EnemySerial    int       `json:"enemySerial"`
	EventSerial    int       `json:"eventSerial"`
	Cursor         int       `json:"cursor"`
	Log            []string  `json:"log"`
	eventAtMS      *int64
}

func spec(kind string) (TowerSpec, bool) {
	for _, c := range catalog {
		if c.Kind == kind {
			return c, true
		}
	}
	return TowerSpec{}, false
}
func (s *State) note(format string, args ...any) {
	s.Log = append(s.Log, fmt.Sprintf(format, args...))
	if len(s.Log) > 40 {
		s.Log = s.Log[len(s.Log)-40:]
	}
}
func (s *State) event(e Event) {
	atMS := s.ElapsedMS
	if s.eventAtMS != nil {
		atMS = *s.eventAtMS
	}
	if e.Type == "hit" && e.TowerID > 0 {
		// A blast is simultaneous. Death events can sit between its victims; stop
		// looking when another shot or logical time starts so ordering stays exact.
		for i := len(s.Events) - 1; i >= 0; i-- {
			prior := &s.Events[i]
			if prior.AtMS == nil || *prior.AtMS != atMS || prior.Type == "shot" {
				break
			}
			if prior.Type == "hit" && prior.TowerID == e.TowerID {
				count := len(prior.Damage)
				for _, group := range prior.DamageGroups {
					count += len(group.Targets)
				}
				if count >= MaxEnemies-1 {
					break
				}
				for j := range prior.DamageGroups {
					if prior.DamageGroups[j].Amount == e.Amount {
						prior.DamageGroups[j].Targets = append(prior.DamageGroups[j].Targets, e.TargetID)
						return
					}
				}
				prior.DamageGroups = append(prior.DamageGroups, DamageGroup{Amount: e.Amount, Targets: []int{e.TargetID}})
				return
			}
		}
	}
	s.EventSerial++
	e.ID = s.EventSerial
	e.Tick = s.Tick
	e.AtMS = &atMS
	if e.TargetID > 0 && e.Target == nil && e.Type != "frostbound" {
		for _, enemy := range s.Enemies {
			if enemy.ID == e.TargetID {
				target := viewEnemy(enemy)
				e.Target = &target
				break
			}
		}
	}
	s.Events = append(s.Events, e)
	if len(s.Events) > MaxEvents {
		s.Events = s.Events[len(s.Events)-MaxEvents:]
	}
}

func (s *State) finished() bool { return s.Phase == "won" || s.Phase == "lost" }
func (s *State) actors() []int {
	a := []int{}
	if !s.finished() {
		for i := range s.Players {
			a = append(a, (i+s.Cursor)%len(s.Players))
		}
	}
	return a
}
func (s *State) valid() error {
	if err := s.validNewSkills(); err != nil {
		return err
	}
	if err := s.validCraters(); err != nil {
		return err
	}
	if err := s.validWorld(); err != nil {
		return err
	}
	if len(s.Players) < 1 || len(s.Players) > 4 || !validOptions(s.Options) || !slices.Contains([]string{"prepare", "battle", "paused", "won", "lost"}, s.Phase) || s.Epoch < 1 || s.Wave < 1 || s.CompletedWaves < 0 || s.CompletedWaves > s.Wave || s.BaseMaxHP != 30 || s.BaseHP < 0 || s.BaseHP > s.BaseMaxHP || s.Cursor < 0 || s.Cursor >= len(s.Players) || s.Tick < 0 || s.ElapsedMS != s.Tick*StepMS || s.ServerAt < 0 || s.NextAt < 0 {
		return errors.New("无效星垒存档")
	}
	if (s.Phase == "lost") != (s.BaseHP == 0) || (s.Phase == "won" && (s.Options.Mode != "campaign" || s.CompletedWaves != s.Options.Waves)) || len(s.Towers) > len(s.World.Map.Pads) || len(s.Enemies) > MaxEnemies || len(s.Events) > MaxEvents || len(s.Log) > 40 {
		return errors.New("无效战场状态")
	}
	for _, p := range s.Players {
		if p.Gold < 0 || p.Gold > maxValue || p.Kills < 0 || !finite(p.Damage) || p.Damage < 0 {
			return errors.New("无效玩家资源")
		}
	}
	ids, pads := map[int]bool{}, map[int]bool{}
	for _, t := range s.Towers {
		_, ok := spec(t.Kind)
		if t.PassiveReadyAt < 0 || t.FocusHits < 0 || t.FocusHits > 2 || t.FocusTarget < 0 || !ok || !slices.Contains([]string{"first", "strong", "support"}, t.Priority) || t.ID < 1 || t.ID > s.TowerSerial || ids[t.ID] || t.Pad < 0 || t.Pad >= len(s.World.Map.Pads) || pads[t.Pad] || t.Owner < 0 || t.Owner >= len(s.Players) || t.Level < 1 || t.Level > maxValue || t.Spent < 1 || t.Spent > maxValue || !finite(t.Angle) || t.Cooldown < 0 || t.Cooldown > 10000 || !s.validBranch(t) {
			return errors.New("无效防御塔")
		}
		ids[t.ID] = true
		pads[t.Pad] = true
	}
	ids = map[int]bool{}
	for _, e := range s.Enemies {
		if e.Level < 0 || e.Level > maxValue || e.ID < 1 || e.ID > s.EnemySerial || ids[e.ID] || !slices.Contains([]string{"slime", "runner", "armored", "swarm", "shield", "elite", "boss", "healer", "banner", "splitter", "engineer"}, e.Kind) || !finite(e.HP) || !finite(e.MaxHP) || !finite(e.Shield) || !finite(e.X) || !finite(e.Y) || !finite(e.Progress) || !finite(e.Speed) || !finite(e.Armor) || e.HP <= 0 || e.MaxHP <= 0 || e.HP > e.MaxHP || e.Shield < 0 || e.Speed <= 0 || e.Speed > 500 || e.Progress < 0 || e.Progress > pathDistance(s.routePath(e.Route)) || len(s.routePath(e.Route)) < 2 || e.Phase < 0 || e.Phase > 2 || e.AbilityTick < 0 || e.Armor < 0 || e.Armor > 1 || e.Reward < 0 || e.Leak < 1 {
			return errors.New("无效敌人")
		}
		ids[e.ID] = true
	}
	if s.WaveInfo.Total < 1 || s.WaveInfo.Total > 64 || s.WaveInfo.Spawned < 0 || s.WaveInfo.Spawned > s.WaveInfo.Total || s.SpawnClock < 0 || s.SpawnClock > 10000 {
		return errors.New("无效波次状态")
	}
	last := 0
	for _, e := range s.Events {
		if e.IceWallID < 0 || e.IceWallID > s.IceWallSerial || e.ID <= last || e.ID > s.EventSerial || !finite(e.X) || !finite(e.Y) || !finite(e.ToX) || !finite(e.ToY) || !finite(e.Amount) || e.Tick < 0 || e.Tick > s.Tick {
			return errors.New("无效战场事件")
		}
		if w := e.IceWall; w != nil && (w.ID < 1 || w.ID > s.IceWallSerial || !finite(w.X) || !finite(w.Y) || !finite(w.Angle) || w.CreatedMS < 0 || w.ExpiresMS-w.CreatedMS != 6000) {
			return errors.New("无效冰墙事件")
		}
		if e.AtMS != nil && (*e.AtMS < max(int64(0), (e.Tick-1)*StepMS) || *e.AtMS > e.Tick*StepMS) {
			return errors.New("无效战场事件时间")
		}
		if e.Type == "frostbound" && (s.SkillRules != 2 || e.TargetID < 1 || e.TargetID > s.EnemySerial || e.IceWallID < 1) {
			return errors.New("无效永久减速事件")
		}
		if e.Target != nil && (e.Target.ID != e.TargetID || e.Target.ID < 1 || e.Target.ID > s.EnemySerial || !finite(e.Target.Health) || !finite(e.Target.MaxHealth) || !finite(e.Target.HP) || !finite(e.Target.Shield) || e.Target.HP < 0 || e.Target.Shield < 0 || e.Target.Health != e.Target.HP+e.Target.Shield || e.Target.MaxHealth < e.Target.Health) {
			return errors.New("无效战场事件目标")
		}
		if len(e.SlowTargets) > MaxEnemies || e.SlowUntil < 0 {
			return errors.New("霜环数据无效")
		}
		slowIDs := map[int]bool{}
		for _, id := range e.SlowTargets {
			if id < 1 || id > s.EnemySerial || slowIDs[id] {
				return errors.New("霜环目标无效")
			}
			slowIDs[id] = true
		}
		if len(e.Damage) >= MaxEnemies {
			return errors.New("范围命中数量无效")
		}
		victims := map[int]bool{e.TargetID: true}
		for _, hit := range e.Damage {
			if hit.ID < 1 || hit.ID > s.EnemySerial || victims[hit.ID] || !finite(hit.Amount) || hit.Amount < 0 || !finite(hit.X) || !finite(hit.Y) {
				return errors.New("范围命中数据无效")
			}
			victims[hit.ID] = true
		}
		count := len(e.Damage)
		for _, group := range e.DamageGroups {
			count += len(group.Targets)
			if count >= MaxEnemies || len(group.Targets) == 0 || !finite(group.Amount) || group.Amount < 0 {
				return errors.New("范围命中分组无效")
			}
			for _, id := range group.Targets {
				if id < 1 || id > s.EnemySerial || victims[id] {
					return errors.New("范围命中分组目标无效")
				}
				victims[id] = true
			}
		}
		last = e.ID
	}
	return nil
}
func finite(n float64) bool { return !math.IsInf(n, 0) && !math.IsNaN(n) }
func (s *State) Envelope() (gameapi.Match, error) {
	if err := s.valid(); err != nil {
		return gameapi.Match{}, err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return gameapi.Match{}, err
	}
	schema := 2
	if s.World.Layout == "central" {
		schema = 3
	}
	if s.SkillRules == 2 {
		schema = 4
		if s.World.Layout != "central" {
			schema = 5
		}
	}
	m := gameapi.Match{Schema: schema, Data: raw, Actors: s.actors(), Attention: []int{}, TimeoutActors: []int{}, Log: append([]string{}, s.Log...)}
	if s.finished() {
		r := &gameapi.Result{Cooperative: true, Ranks: make([]int, len(s.Players)), Winners: []int{}, Scores: make([]int, len(s.Players))}
		for i := range s.Players {
			r.Ranks[i] = 1
			r.Scores[i] = s.CompletedWaves*1000 + s.Kills
			if s.Phase == "won" {
				r.Winners = append(r.Winners, i)
			}
		}
		r.Details, _ = json.Marshal(map[string]any{"mode": s.Options.Mode, "difficulty": s.Options.Difficulty, "completedWaves": s.CompletedWaves, "kills": s.Kills, "baseHP": s.BaseHP, "elapsedMS": s.ElapsedMS, "outcome": s.Phase})
		m.Result = r
	} else if s.Phase == "prepare" {
		m.SystemDeadline = s.NextAt
		for i, p := range s.Players {
			if !p.Ready {
				m.Attention = append(m.Attention, i)
			}
		}
	} else if s.Phase == "battle" {
		m.SystemDeadline = s.ServerAt + 1000
	}
	return m, m.Validate(len(s.Players))
}
func decode(m gameapi.Match) (*State, error) {
	if (m.Schema != 1 && m.Schema != 2 && m.Schema != 3 && m.Schema != 4 && m.Schema != 5) || len(m.Data) > maxSnapshotBytes {
		return nil, errors.New("不支持的星垒存档")
	}
	var s State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return nil, err
	}
	if m.Schema == 1 {
		s.migrateWorld()
	}
	if (m.Schema == 3 || m.Schema == 4) && s.World.Layout != "central" {
		return nil, errors.New("中央地图存档缺少布局标识")
	}
	if (m.Schema == 4 || m.Schema == 5) != (s.SkillRules == 2) {
		return nil, errors.New("技能规则与存档版本不一致")
	}
	if m.Schema == 5 && s.World.Layout != "" {
		return nil, errors.New("合作扩展地图存档布局不一致")
	}
	if err := s.valid(); err != nil {
		return nil, err
	}
	return &s, nil
}
func upgradeCost(t Tower) int {
	c, _ := spec(t.Kind)
	return int(math.Min(maxValue, math.Round(float64(c.Cost)*(.7+.38*float64(t.Level-1)))))
}
func refund(t Tower) int { return t.Spent * 7 / 10 }
func stats(t Tower) (damage, rangePX float64, interval int) {
	c, _ := spec(t.Kind)
	damage = math.Min(maxValue, c.Damage*(1+.65*float64(t.Level-1)))
	rangePX = c.Range + math.Min(60, float64(t.Level-1)*6)
	rate := 1 + .04*float64(t.Level-1)
	if t.Branch == "power" {
		damage *= 1.5
		rate *= .9
	}
	if t.Branch == "speed" {
		damage *= .9
		rate *= 1.65
	}
	interval = max(StepMS, int(float64(c.Interval)/rate))
	return
}
