package starfort

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
	"io"
	"slices"
)

type Action struct {
	Type        string `json:"type"`
	Choice      string `json:"choice,omitempty"`
	ObjectiveID string `json:"objectiveId,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Epoch       int    `json:"epoch"`
	Pad         int    `json:"pad,omitempty"`
	Tower       string `json:"tower,omitempty"`
	TowerID     int    `json:"towerId,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Target      int    `json:"target,omitempty"`
	Amount      int    `json:"amount,omitempty"`
}

func strict(raw json.RawMessage, v any) error {
	if len(raw) > 4096 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func validOptions(o Options) bool {
	return slices.Contains([]string{"campaign", "endless"}, o.Mode) && slices.Contains([]int{10, 20}, o.Waves) && slices.Contains([]string{"easy", "normal", "hard"}, o.Difficulty) && slices.Contains([]int{20, 30, 45}, o.PrepSeconds)
}
func configure(n int, raw json.RawMessage) (json.RawMessage, error) {
	if n < 1 || n > 4 {
		return nil, errors.New("星垒需要 1～4 人")
	}
	o := Options{Mode: "campaign", Waves: 10, Difficulty: "normal", PrepSeconds: 30}
	if len(raw) > 0 {
		if err := strict(raw, &o); err != nil {
			return nil, errors.New("无效星垒设置")
		}
	}
	if !validOptions(o) {
		return nil, errors.New("无效模式、波数、难度或准备时间")
	}
	if o.Mode == "endless" {
		o.Waves = 10
	}
	return json.Marshal(o)
}
func Module() gameapi.Module {
	return gameapi.Module{
		ID: "starfort", Name: "星垒", MinPlayers: 1, MaxPlayers: 4, Configure: configure,
		Start: func(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
			normalized, err := configure(ctx.Players, raw)
			if err != nil {
				return gameapi.Match{}, err
			}
			var o Options
			_ = json.Unmarshal(normalized, &o)
			s := &State{Options: o, Players: make([]Player, ctx.Players), Phase: "prepare", Epoch: 1, Wave: 1, BaseHP: 30, BaseMaxHP: 30, ServerAt: ctx.Now.UnixMilli(), NextAt: ctx.Now.UnixMilli() + int64(o.PrepSeconds)*1000, Towers: []Tower{}, Enemies: []Enemy{}, Events: []Event{}}
			for i := range s.Players {
				s.Players[i].Gold = 300
			}
			s.describeWave()
			s.initCentralWorld(ctx.Now.UnixNano()%2147483647 + int64(ctx.Players)*101)
			s.note("星核已点亮，共同守住防线")
			return s.Envelope()
		},
		Apply: apply, View: view, BotAction: botAction,
		AcceptStale: func(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
			s, err := decode(m)
			if err != nil || ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) || s.finished() {
				return false
			}
			var a Action
			return strict(raw, &a) == nil && a.Epoch == s.Epoch
		},
		Reclaim: func(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
			s, err := decode(m)
			if err != nil {
				return m, err
			}
			if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) || s.finished() {
				return m, errors.New("当前不能接管")
			}
			return s.Envelope()
		},
		OnTimer: timer,
	}
}
func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	var a Action
	if err = strict(raw, &a); err != nil {
		return m, errors.New("无效游戏行动")
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Players) || ctx.Players != len(s.Players) || s.finished() || a.Epoch != s.Epoch {
		return m, errors.New("当前不能行动或波次已经改变")
	}
	p := ctx.Actor
	if s.Phase == "paused" && a.Type != "resume" && a.Type != "wait" {
		return m, errors.New("请等待全体恢复对局")
	}
	switch a.Type {
	case "build":
		c, ok := spec(a.Tower)
		if !ok || a.Pad < 0 || a.Pad >= len(s.World.Map.Pads) {
			return m, errors.New("请选择有效建造点和防御塔")
		}
		for _, t := range s.Towers {
			if t.Pad == a.Pad {
				return m, errors.New("建造点已被占用")
			}
		}
		if s.Players[p].Gold < c.Cost {
			return m, errors.New("金币不足")
		}
		s.Players[p].Gold -= c.Cost
		s.TowerSerial++
		s.Towers = append(s.Towers, Tower{ID: s.TowerSerial, Pad: a.Pad, Owner: p, Kind: a.Tower, Level: 1, Spent: c.Cost, Priority: "first"})
		pos := s.World.Map.Pads[a.Pad]
		s.event(Event{Type: "build", X: pos.X, Y: pos.Y, Kind: a.Tower, Owner: p})
		s.note("玩家 %d 建造了%s", p+1, c.Name)
	case "upgrade", "sell":
		index := -1
		for i, t := range s.Towers {
			if t.ID == a.TowerID {
				index = i
				break
			}
		}
		if index < 0 || s.Towers[index].Owner != p {
			return m, errors.New("只能管理自己的防御塔")
		}
		t := &s.Towers[index]
		pos := s.World.Map.Pads[t.Pad]
		if a.Type == "sell" {
			amount := refund(*t)
			s.Players[p].Gold = min(maxValue, s.Players[p].Gold+amount)
			s.event(Event{Type: "sell", X: pos.X, Y: pos.Y, Kind: t.Kind, Owner: p, Amount: float64(amount)})
			s.Towers = slices.Delete(s.Towers, index, index+1)
		} else {
			if (t.Level == 3 && (a.Branch != "power" && a.Branch != "speed")) || (t.Level != 3 && a.Branch != "") {
				return m, errors.New("第 4 级请选择威力或迅捷专精，其他等级无需选择")
			}
			cost := upgradeCost(*t)
			if s.Players[p].Gold < cost || t.Level >= maxValue || t.Spent > maxValue-cost {
				return m, errors.New("金币不足或强化已达资源上限")
			}
			s.Players[p].Gold -= cost
			t.Spent += cost
			t.Level++
			if a.Branch != "" {
				t.Branch = a.Branch
			}
			s.event(Event{Type: "upgrade", X: pos.X, Y: pos.Y, Kind: t.Kind, Owner: p, Amount: float64(t.Level)})
		}
	case "route_vote", "boon_vote":
		kind := "route"
		if a.Type == "boon_vote" {
			kind = "boon"
		}
		if err := s.vote(kind, a.Choice, p, ctx.Now.UnixMilli()); err != nil {
			return m, err
		}
	case "objective":
		if err := s.acceptObjective(a.ObjectiveID, p); err != nil {
			return m, err
		}
	case "target":
		if !slices.Contains([]string{"first", "strong", "support"}, a.Priority) {
			return m, errors.New("无效集火优先级")
		}
		found := false
		for i := range s.Towers {
			if s.Towers[i].ID == a.TowerID && s.Towers[i].Owner == p {
				s.Towers[i].Priority = a.Priority
				found = true
			}
		}
		if !found {
			return m, errors.New("只能管理自己的防御塔")
		}
	case "gift":
		if a.Target < 0 || a.Target >= len(s.Players) || a.Target == p || a.Amount < 1 || a.Amount > s.Players[p].Gold || s.Players[a.Target].Gold > maxValue-a.Amount {
			return m, errors.New("无效赠予金额或队友")
		}
		s.Players[p].Gold -= a.Amount
		s.Players[a.Target].Gold += a.Amount
		s.event(Event{Type: "gift", Owner: p, Amount: float64(a.Amount)})
	case "ready":
		if s.Phase != "prepare" || s.World.Choice != nil || s.World.BoonChoice != nil {
			return m, errors.New("请先完成波间选择")
		}
		s.Players[p].Ready = !s.Players[p].Ready
		s.Players[p].PauseVote = false
		all := true
		for _, v := range s.Players {
			all = all && v.Ready
		}
		if all {
			s.startWave(ctx.Now.UnixMilli())
		}
	case "pause":
		if s.Phase != "prepare" {
			return m, errors.New("只能在波次之间暂停")
		}
		s.Players[p].PauseVote = !s.Players[p].PauseVote
		s.Players[p].Ready = false
		all := true
		for _, v := range s.Players {
			all = all && v.PauseVote
		}
		if all {
			s.Phase = "paused"
			s.Epoch++
			s.NextAt = 0
			s.ServerAt = ctx.Now.UnixMilli()
			s.event(Event{Type: "pause", Owner: p})
			s.note("全体玩家同意暂停")
		}
	case "resume":
		if s.Phase != "paused" {
			return m, errors.New("当前没有暂停")
		}
		s.Players[p].ResumeVote = !s.Players[p].ResumeVote
		all := true
		for _, v := range s.Players {
			all = all && v.ResumeVote
		}
		if all {
			s.prepare(ctx.Now.UnixMilli())
			s.event(Event{Type: "resume", Owner: p})
			s.note("全体玩家同意恢复")
		}
	case "wait":
	default:
		return m, errors.New("未知星垒行动")
	}
	s.Cursor = (p + 1) % len(s.Players)
	next, err := s.Envelope()
	if err != nil {
		return m, err
	}
	return next, nil
}
func timer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	now := ctx.Now.UnixMilli()
	if ctx.Actor != -1 || ctx.Players != len(s.Players) || m.SystemDeadline == 0 || now < m.SystemDeadline {
		return m, errors.New("系统事件尚未到期")
	}
	if s.Phase == "prepare" {
		s.resolveChoices(now)
		if s.World.Choice == nil && s.World.BoonChoice == nil && now >= s.NextAt {
			s.startWave(now)
		}
	} else if s.Phase == "battle" {
		steps := min(int64(1000), max(int64(0), now-s.ServerAt)) / StepMS
		s.ServerAt = now
		for i := int64(0); i < steps && s.Phase == "battle"; i++ {
			s.step(now)
		}
	}
	next, err := s.Envelope()
	if err != nil {
		return m, err
	}
	return next, nil
}

type BuildControl struct {
	Pad     int    `json:"pad"`
	Kind    string `json:"kind"`
	Cost    int    `json:"cost"`
	Enabled bool   `json:"enabled"`
}
type UpgradeControl struct {
	TowerID     int  `json:"towerId"`
	Cost        int  `json:"cost"`
	Enabled     bool `json:"enabled"`
	NeedsBranch bool `json:"needsBranch"`
}
type SellControl struct {
	TowerID int  `json:"towerId"`
	Refund  int  `json:"refund"`
	Enabled bool `json:"enabled"`
}
type ObjectiveControl struct {
	ObjectiveID string `json:"objectiveId"`
	Cost        int    `json:"cost"`
	Enabled     bool   `json:"enabled"`
}
type TargetControl struct {
	TowerID int  `json:"towerId"`
	Enabled bool `json:"enabled"`
}
type Controls struct {
	CanRouteVote bool               `json:"canRouteVote"`
	CanBoonVote  bool               `json:"canBoonVote"`
	Objectives   []ObjectiveControl `json:"objectives"`
	Targets      []TargetControl    `json:"targets"`
	CanAct       bool               `json:"canAct"`
	CanReady     bool               `json:"canReady"`
	CanPause     bool               `json:"canPause"`
	CanResume    bool               `json:"canResume"`
	Builds       []BuildControl     `json:"builds"`
	Upgrades     []UpgradeControl   `json:"upgrades"`
	Sells        []SellControl      `json:"sells"`
}

func (s *State) controls(p int) Controls {
	c := Controls{Builds: []BuildControl{}, Upgrades: []UpgradeControl{}, Sells: []SellControl{}}
	player := p >= 0 && p < len(s.Players) && !s.finished()
	c.CanResume = player && s.Phase == "paused"
	c.CanAct = player && s.Phase != "paused"
	c.CanPause = c.CanAct && s.Phase == "prepare"
	c.CanRouteVote = c.CanPause && s.World.Choice != nil
	c.CanBoonVote = c.CanPause && s.World.BoonChoice != nil
	c.CanReady = c.CanPause && s.World.Choice == nil && s.World.BoonChoice == nil
	c.Objectives = []ObjectiveControl{}
	c.Targets = []TargetControl{}
	for _, o := range s.World.Objectives {
		cost := o.Cost
		afford := player && s.Players[p].Gold >= cost
		c.Objectives = append(c.Objectives, ObjectiveControl{o.ID, cost, c.CanReady && o.Status == "available" && afford})
	}
	if !player {
		return c
	}
	occupied := map[int]bool{}
	for _, t := range s.Towers {
		occupied[t.Pad] = true
		if t.Owner == p {
			c.Targets = append(c.Targets, TargetControl{t.ID, c.CanAct})
			cost := upgradeCost(t)
			c.Upgrades = append(c.Upgrades, UpgradeControl{t.ID, cost, c.CanAct && s.Players[p].Gold >= cost && t.Level < maxValue && t.Spent <= maxValue-cost, t.Level == 3})
			c.Sells = append(c.Sells, SellControl{t.ID, refund(t), c.CanAct})
		}
	}
	for _, pad := range s.World.Map.Pads {
		for _, tower := range catalog {
			c.Builds = append(c.Builds, BuildControl{pad.ID, tower.Kind, tower.Cost, c.CanAct && !occupied[pad.ID] && s.Players[p].Gold >= tower.Cost})
		}
	}
	return c
}

// ViewTower adds computed combat stats to the public projection. Persisting
// these values as well would risk stale ranges after upgrading or tuning.
type ViewTower struct {
	Passive *PassiveView `json:"passive,omitempty"`
	Tower
	EffectiveRange    float64 `json:"effectiveRange"`
	EffectiveDamage   float64 `json:"effectiveDamage"`
	EffectiveInterval int     `json:"effectiveInterval"`
}

// Health is a single presentation value. Damage and armor rules remain in State.
type ViewEnemy struct {
	Enemy
	Health    float64 `json:"health"`
	MaxHealth float64 `json:"maxHealth"`
}

func viewEnemy(enemy Enemy) ViewEnemy {
	capacity := enemy.MaxHP
	if enemy.Kind == "shield" {
		capacity += enemy.MaxHP * shieldRatio
	}
	// Also handle imported snapshots whose shield exceeds the usual capacity.
	health := enemy.HP + enemy.Shield
	return ViewEnemy{enemy, health, max(capacity, health)}
}

type ViewTowerSpec struct {
	TowerSpec
	Passive *PassiveView `json:"passive,omitempty"`
}

// ViewWorld carries dynamic world state; the map is sent once at the top level.
type ViewWorld struct {
	World
	Map *Map `json:"map,omitempty"`
}

func view(m gameapi.Match, p int) (any, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	// Send recent complete batches, not the entire persisted event ring on
	// every update. Keep the idle cursor so the next batch remains contiguous.
	cutoff := s.ElapsedMS - 3000
	first := 0
	for first < len(s.Events)-1 {
		event := s.Events[first]
		at := event.Tick * StepMS
		if event.AtMS != nil {
			at = *event.AtMS
		}
		if at >= cutoff {
			break
		}
		first++
	}
	s.Events = s.Events[first:]
	towers := make([]ViewTower, 0, len(s.Towers))
	for _, tower := range s.Towers {
		damage, radius, interval := s.towerStats(tower)
		towers = append(towers, ViewTower{Tower: tower, EffectiveRange: radius, EffectiveDamage: damage, EffectiveInterval: interval, Passive: s.passiveView(tower)})
	}
	enemies := make([]ViewEnemy, 0, len(s.Enemies))
	for _, enemy := range s.Enemies {
		enemies = append(enemies, viewEnemy(enemy))
	}
	specs := make([]ViewTowerSpec, 0, len(catalog))
	for _, spec := range catalog {
		specs = append(specs, ViewTowerSpec{spec, s.passiveView(Tower{Kind: spec.Kind})})
	}
	return struct {
		Options        Options         `json:"options"`
		World          ViewWorld       `json:"world"`
		Players        []Player        `json:"players"`
		Phase          string          `json:"phase"`
		Epoch          int             `json:"epoch"`
		Wave           int             `json:"wave"`
		CompletedWaves int             `json:"completedWaves"`
		BaseHP         int             `json:"baseHP"`
		BaseMaxHP      int             `json:"baseMaxHP"`
		Kills          int             `json:"kills"`
		Tick           int64           `json:"tick"`
		ServerAt       int64           `json:"serverAt"`
		NextAt         int64           `json:"nextAt"`
		ElapsedMS      int64           `json:"elapsedMS"`
		WaveInfo       WaveInfo        `json:"waveInfo"`
		Events         []Event         `json:"events"`
		Craters        []Crater        `json:"craters,omitempty"`
		Enemies        []ViewEnemy     `json:"enemies"`
		Towers         []ViewTower     `json:"towers"`
		Map            Map             `json:"map"`
		Catalog        []ViewTowerSpec `json:"catalog"`
		Controls       Controls        `json:"controls"`
	}{
		Options:        s.Options,
		World:          ViewWorld{World: s.World},
		Players:        s.Players,
		Phase:          s.Phase,
		Epoch:          s.Epoch,
		Wave:           s.Wave,
		CompletedWaves: s.CompletedWaves,
		BaseHP:         s.BaseHP,
		BaseMaxHP:      s.BaseMaxHP,
		Kills:          s.Kills,
		Tick:           s.Tick,
		ServerAt:       s.ServerAt,
		NextAt:         s.NextAt,
		ElapsedMS:      s.ElapsedMS,
		WaveInfo:       s.WaveInfo,
		Events:         s.Events,
		Craters:        s.Craters,
		Enemies:        enemies,
		Towers:         towers,
		Map:            s.mapData(),
		Catalog:        specs,
		Controls:       s.controls(p),
	}, nil
}
