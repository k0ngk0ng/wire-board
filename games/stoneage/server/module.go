package stoneage

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

func strict(b []byte, v any) error {
	if len(b) == 0 || len(b) > 128<<10 || bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		return errors.New("无效请求")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("请求包含多余内容")
	}
	return nil
}
func configure(n int, b json.RawMessage) (json.RawMessage, error) {
	if n < 2 || n > 10 || n%2 != 0 {
		return nil, errors.New("石器竞技场支持 1v1 至 5v5，需要偶数席位")
	}
	o := Options{TurnSeconds: 25, PreparationSeconds: 180, MaxRounds: 25, Teams: make([]int, n)}
	for i := range o.Teams {
		o.Teams[i] = i % 2
	}
	if len(b) > 0 {
		if e := strict(b, &o); e != nil {
			return nil, errors.New("无效竞技场设置")
		}
	}
	if len(o.Teams) == 0 {
		o.Teams = make([]int, n)
		for i := range o.Teams {
			o.Teams[i] = i % 2
		}
	}
	if !slices.Contains([]int{20, 25, 30, 45}, o.TurnSeconds) || o.PreparationSeconds < 60 || o.PreparationSeconds > 300 || o.MaxRounds < 10 || o.MaxRounds > 40 || len(o.Teams) != n {
		return nil, errors.New("人数、选配时间或回合设置不合法")
	}
	for _, team := range o.Teams {
		if team < 0 || team > 1 {
			return nil, errors.New("队伍只能是红队或蓝队")
		}
	}
	return json.Marshal(o)
}
func Module() gameapi.Module {
	return gameapi.Module{
		ID: "stoneage", Name: "石器竞技场", MinPlayers: 2, MaxPlayers: 10,
		PreciseTimers: true,
		Configure:     configure, Prepare: prepare, Start: start, Apply: apply, View: makeView,
		BotAction: botAction, Reclaim: reclaim, ReclaimOnCancel: true, Restore: restore, AcceptStale: acceptStale, OnTimer: onTimer,
	}
}
func prepare(c gameapi.PreparationContext, b, a json.RawMessage) (json.RawMessage, error) {
	var o Options
	if e := strict(b, &o); e != nil {
		return nil, e
	}
	var change struct {
		Type string `json:"type"`
		Seat int    `json:"seat"`
		Team int    `json:"team"`
	}
	if strict(a, &change) != nil || change.Type != "team" || change.Seat < 0 || change.Seat >= len(c.Bots) || change.Team < 0 || change.Team > 1 || (!c.Host && change.Seat != c.Actor) {
		return nil, errors.New("只能选择自己的队伍，房主可调整全部席位")
	}
	o.Teams[change.Seat] = change.Team
	return json.Marshal(o)
}
func start(c gameapi.Context, b json.RawMessage) (gameapi.Match, error) {
	normalized, e := configure(c.Players, b)
	if e != nil {
		return gameapi.Match{}, e
	}
	var o Options
	json.Unmarshal(normalized, &o)
	red := 0
	for _, team := range o.Teams {
		if team == 0 {
			red++
		}
	}
	if red != c.Players/2 {
		return gameapi.Match{}, errors.New("双方人数必须相同，请先调整队伍")
	}
	var entropy [32]byte
	if _, e := rand.Read(entropy[:]); e != nil {
		return gameapi.Match{}, errors.New("无法生成本局随机配置")
	}
	seed := sha256.Sum256(append(entropy[:], []byte(fmt.Sprintf("stoneage/pool/%s/%d/%d", c.Now.Format(time.RFC3339Nano), c.Version, c.Players))...))
	s := State{Rules: rulesVersion, Session: fmt.Sprintf("%x", seed[:16]), Window: 1, Phase: "selection", Options: o, Random: binary.LittleEndian.Uint64(seed[:8]), Plans: make([]*Plan, c.Players), Players: make([]Player, c.Players), Events: []Event{}, Log: []string{"双方共用本局候选；每人选择一只出战宠和一只骑宠。"}}
	s.makePool()
	// Separate the pool and combat streams. Neither state is sent to clients.
	combat := sha256.Sum256(append([]byte("stoneage/combat/"), seed[:]...))
	s.Random = binary.LittleEndian.Uint64(combat[:8])
	for i := range s.Players {
		s.Players[i] = Player{Seat: i, Team: o.Teams[i], Deadline: c.Now.Add(time.Duration(o.PreparationSeconds) * time.Second).UnixMilli()}
	}
	return s.envelope()
}
func decode(m gameapi.Match) (*State, error) {
	if m.Schema != schema || len(m.Data) == 0 || len(m.Data) > 8<<20 {
		return nil, errors.New("不支持的石器竞技场存档")
	}
	var s State
	if e := json.Unmarshal(m.Data, &s); e != nil {
		return nil, e
	}
	if s.Rules == "stoneage-arena-v1" {
		// Legacy matches obeyed every order. Do not reroll their pool or RNG.
		for i := range s.Pool.Pets {
			s.Pool.Pets[i].Loyalty = 100
		}
		for i := range s.Pool.Rides {
			s.Pool.Rides[i].Loyalty = 100
		}
		for i := range s.Players {
			s.Players[i].Pet.Loyalty = 100
			s.Players[i].Ride.Loyalty = 100
		}
		migrate := func(players []Player) {
			for i := range players {
				players[i].Pet.Loyalty = 100
				players[i].Ride.Loyalty = 100
			}
		}
		if s.Playback != nil {
			migrate(s.Playback.Before)
		}
		for i := range s.Events {
			migrate(s.Events[i].Players)
		}
		s.Rules = rulesVersion
	}
	previewStats(s.Pool.Pets, 640)
	previewStats(s.Pool.Rides, 400)
	for i := range s.Pool.Skills {
		if s.Pool.Skills[i].ID == 30 || s.Pool.Skills[i].ID == 31 {
			for _, skill := range allSkills {
				if skill.ID == s.Pool.Skills[i].ID {
					s.Pool.Skills[i] = skill
					break
				}
			}
		}
	}
	n := len(s.Players)
	if s.Rules != rulesVersion || s.Session == "" || s.Window < 1 || !slices.Contains([]string{"selection", "arming", "planning", "resolving", "finished"}, s.Phase) || len(s.Plans) != n || s.Round < 0 || s.Round > s.Options.MaxRounds || s.EventID < 0 {
		return nil, errors.New("竞技场存档结构不合法")
	}
	if _, e := configure(n, mustJSON(s.Options)); e != nil {
		return nil, e
	}
	if len(s.Pool.Pets) != 20 || (len(s.Pool.Rides) != 6 && len(s.Pool.Rides) != 4) || len(s.Pool.Skills) != 14 || len(s.Pool.Weapons) != 6 || len(s.Pool.Hats) != 4 || len(s.Pool.Clothes) != 4 || len(s.Pool.Accessories) != 6 {
		return nil, errors.New("竞技场候选池不完整")
	}
	if len(s.Events) > 256 || len(s.Log) > 100 {
		return nil, errors.New("竞技场事件超出范围")
	}
	for _, rows := range [][]PetDefinition{s.Pool.Pets, s.Pool.Rides} {
		for _, p := range rows {
			if p.Loyalty < 0 || p.Loyalty > 100 {
				return nil, errors.New("宠物忠诚度存档不合法")
			}
		}
	}
	for _, ride := range s.Pool.Rides {
		if ride.Loyalty != 100 {
			return nil, errors.New("骑宠忠诚度必须为 100")
		}
	}
	for i, p := range s.Players {
		if p.Seat != i || p.Team != s.Options.Teams[i] || p.Deadline < 0 || len(p.Inventory) > 8 {
			return nil, errors.New("竞技场席位存档不合法")
		}
		if p.Loadout != nil {
			if e := s.validateLoadout(*p.Loadout); e != nil {
				return nil, e
			}
		}
		if s.Phase != "selection" {
			if p.Loadout == nil {
				return nil, errors.New("参战配置缺失")
			}
			pet, _ := petByID(s.Pool.Pets, p.Loadout.Pet)
			ride, _ := petByID(s.Pool.Rides, p.Loadout.Ride)
			if p.Pet.Loyalty != pet.Loyalty || p.Ride.Loyalty != ride.Loyalty || p.Body.Fled || p.Ride.Fled {
				return nil, errors.New("参战宠物忠诚度与候选不一致")
			}
			for _, u := range []Unit{p.Body, p.Pet, p.Ride} {
				if u.MaxHP <= 0 || u.MaxHP > 100000 || u.HP < 0 || u.HP > u.MaxHP || u.MP < 0 || u.MP > u.MaxMP || u.Speed <= 0 || u.Attack <= 0 || u.Defense <= 0 {
					return nil, errors.New("竞技场战斗属性不合法")
				}
			}
		}
	}
	if (s.Phase == "resolving") != (s.Playback != nil && s.ResolveAt > 0) {
		return nil, errors.New("战斗播放存档不完整")
	}
	return &s, nil
}
func mustJSON(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func (s *State) envelope() (gameapi.Match, error) {
	actors, owed := []int{}, []int{}
	deadline := int64(0)
	if s.Phase == "selection" || s.Phase == "arming" || s.Phase == "planning" {
		for i, p := range s.Players {
			if s.Phase == "planning" && (p.Body.HP <= 0 || p.Surrendered) {
				continue
			}
			actors = append(actors, i)
			needs := s.Phase == "selection" && !p.Locked || s.Phase == "arming" && !p.BattleReady || s.Phase == "planning" && !p.Submitted
			if needs {
				owed = append(owed, i)
				if deadline == 0 || p.Deadline < deadline {
					deadline = p.Deadline
				}
			}
		}
	}
	b, e := json.Marshal(s)
	return gameapi.Match{Schema: schema, Data: b, Actors: actors, Attention: append([]int{}, owed...), TimeoutActors: owed, Deadline: deadline, SystemDeadline: s.ResolveAt, Result: s.Result, Log: append([]string(nil), s.Log...)}, e
}
func (s *State) validateLoadout(l Loadout) error {
	if l.Elements != nil {
		sum := 0
		if len(l.Elements) != 4 {
			return errors.New("地水火风必须恰好四项")
		}
		for _, v := range l.Elements {
			if v < 0 || v > 10 {
				return errors.New("元素属性超出范围")
			}
			sum += v
		}
		if l.Elements[0]*l.Elements[2] > 0 || l.Elements[1]*l.Elements[3] > 0 {
			return errors.New("地与火、水与风不能同时分配")
		}
		if sum != 10 {
			return errors.New("地水火风合计必须为 10")
		}
	}
	if _, ok := petByID(s.Pool.Pets, l.Pet); !ok {
		return errors.New("请选择本局提供的出战宠")
	}
	ride, ok := petByID(s.Pool.Rides, l.Ride)
	if !ok {
		return errors.New("请选择本局提供的一只骑宠")
	}
	if !slices.ContainsFunc(ride.Riders, func(r Rider) bool { return r.Avatar == l.Avatar }) {
		return errors.New("人物外观与骑宠动画不匹配")
	}
	if len(l.Points) != 4 {
		return errors.New("必须分配四项属性")
	}
	total := 0
	for _, n := range l.Points {
		if n < 0 || n > pointBudget {
			return errors.New("属性点分配超出范围")
		}
		total += n
	}
	if total != pointBudget {
		return fmt.Errorf("每人共有 %d 点，请完成体力、腕力、耐力和速度的分配", pointBudget)
	}
	if len(l.Skills) < 2 || len(l.Skills) > 7 || !hasSkill(l.Skills, 1) || !hasSkill(l.Skills, 2) || familyUsed(l.Skills, s.Pool.Skills) {
		return errors.New("宠物最多七项技能，必须保留攻击与防御，同族技能只能选择一档")
	}
	for _, entry := range []struct {
		rows []Equipment
		id   int
	}{{s.Pool.Hats, l.Hat}, {s.Pool.Weapons, l.Weapon}, {s.Pool.Clothes, l.Clothes}} {
		if _, ok := gearByID(entry.rows, entry.id); !ok {
			return errors.New("帽子、武器和衣服必须从对应候选中各选一件")
		}
	}
	if len(l.Accessories) > 2 || !unique(l.Accessories) {
		return errors.New("最多携带两件不同饰品")
	}
	for _, id := range l.Accessories {
		if _, ok := gearByID(s.Pool.Accessories, id); !ok {
			return errors.New("饰品不在本局候选中")
		}
	}
	for _, entry := range []struct {
		rows []Item
		id   int
	}{{s.Pool.HPItems, l.HPItem}, {s.Pool.MPItems, l.MPItem}, {s.Pool.CureItems, l.CureItem}} {
		if _, ok := itemByID(entry.rows, entry.id); !ok {
			return errors.New("回血、回气与解除异常补给都必须选择")
		}
	}
	if len(l.Tactics) > 2 || !unique(l.Tactics) {
		return errors.New("最多携带两种不同战术道具")
	}
	for _, id := range l.Tactics {
		if _, ok := itemByID(s.Pool.Tactics, id); !ok {
			return errors.New("道具不在本局候选中")
		}
	}
	return nil
}
func apply(c gameapi.Context, m gameapi.Match, b json.RawMessage) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	var a Action
	if strict(b, &a) != nil {
		return m, errors.New("无效竞技场行动")
	}
	if c.Actor < 0 || c.Actor >= len(s.Players) || a.Session != s.Session || a.Window != s.Window || s.Phase == "finished" || s.Phase == "resolving" {
		return m, errors.New("操作窗口已经结束")
	}
	p := &s.Players[c.Actor]
	if a.Type == "surrender" && s.Phase == "planning" {
		if p.Body.HP <= 0 {
			return m, errors.New("已经退场")
		}
		p.Body.HP = 0
		p.Pet.HP = 0
		p.Riding = false
		p.Surrendered = true
		p.Submitted = true
		if r := s.outcome(); r != nil {
			s.Result = r
			s.Phase = "finished"
			s.Window++
			s.Reason = "surrender"
		} else if s.allSubmitted() {
			s.resolve(c.Now)
		}
		return s.envelope()
	}
	automated := c.Actor < len(c.Automated) && c.Automated[c.Actor]
	if c.Now.UnixMilli() >= p.Deadline && !automated {
		return m, errors.New("操作已超时，请使用平台托管或接管入口")
	}
	switch a.Type {
	case "battle_ready":
		if s.Phase != "arming" || p.BattleReady || a.Plan != nil || a.Loadout != nil {
			return m, errors.New("当前无需确认战斗资源")
		}
		p.BattleReady = true
		all := true
		for _, q := range s.Players {
			all = all && q.BattleReady
		}
		if all {
			s.Phase = "planning"
			s.Window++
			for i := range s.Players {
				s.Players[i].Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
			}
		}
	case "loadout":
		if s.Phase != "selection" || p.Locked || a.Loadout == nil || a.Plan != nil {
			return m, errors.New("当前不能修改参战配置")
		}
		if e = s.validateLoadout(*a.Loadout); e != nil {
			return m, e
		}
		p.Loadout = a.Loadout
	case "lock":
		if s.Phase != "selection" || p.Locked || a.Plan != nil {
			return m, errors.New("请先完成参战配置")
		}
		if a.Loadout != nil {
			if e = s.validateLoadout(*a.Loadout); e != nil {
				return m, e
			}
			p.Loadout = a.Loadout
		}
		if p.Loadout == nil {
			return m, errors.New("请先完成参战配置")
		}
		p.Locked = true
		all := true
		for _, q := range s.Players {
			all = all && q.Locked
		}
		if all {
			s.beginCombat(c.Now)
		}
	case "submit":
		if s.Phase != "planning" || p.Submitted || p.Body.HP <= 0 || a.Plan == nil || a.Loadout != nil {
			return m, errors.New("本回合已经提交或不能行动")
		}
		if e = s.validatePlan(c.Actor, *a.Plan); e != nil {
			return m, e
		}
		s.Plans[c.Actor] = a.Plan
		p.Submitted = true
		if s.allSubmitted() {
			s.resolve(c.Now)
		}
	default:
		return m, errors.New("竞技场不支持这项操作")
	}
	return s.envelope()
}
func (s *State) allSubmitted() bool {
	for _, p := range s.Players {
		if p.Body.HP > 0 && !p.Submitted && !p.Surrendered {
			return false
		}
	}
	return true
}
func (s *State) beginCombat(now time.Time) {
	s.Phase = "arming"
	s.Round = 1
	s.Window++
	s.Log = append(s.Log, "配置已锁定；每轮先选择指令，再按敏捷顺序逐个行动。")
	for i := range s.Players {
		s.initializePlayer(i)
		s.Players[i].Deadline = now.Add(90 * time.Second).UnixMilli()
	}
}
func onTimer(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Actor != -1 || s.Phase != "resolving" || c.Now.UnixMilli() < s.ResolveAt {
		return m, errors.New("战斗播放尚未结束")
	}
	s.ResolveAt = 0
	s.Playback = nil
	s.Window++
	if s.PendingResult != nil {
		s.Result = s.PendingResult
		s.PendingResult = nil
		s.Phase = "finished"
		s.Log = append(s.Log, "本场竞技已结束。")
	} else {
		s.Phase = "planning"
		s.Round++
		s.Plans = make([]*Plan, len(s.Players))
		for i := range s.Players {
			s.Players[i].Submitted = false
			s.Players[i].Deadline = c.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		}
	}
	return s.envelope()
}
func restore(_ gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	return s.envelope()
}
func reclaim(c gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, e := decode(m)
	if e != nil {
		return m, e
	}
	if c.Actor < 0 || c.Actor >= len(s.Players) {
		return m, errors.New("无效席位")
	}
	p := &s.Players[c.Actor]
	if s.Phase != "selection" && s.Phase != "planning" && s.Phase != "arming" || p.Locked && s.Phase == "selection" || p.Submitted || p.Deadline > c.Now.UnixMilli() {
		return m, nil
	}
	seconds := s.Options.TurnSeconds
	if s.Phase == "selection" {
		seconds = s.Options.PreparationSeconds
	}
	p.Deadline = c.Now.Add(time.Duration(seconds) * time.Second).UnixMilli()
	return s.envelope()
}
func acceptStale(c gameapi.Context, m gameapi.Match, b json.RawMessage) bool {
	s, e := decode(m)
	if e != nil {
		return false
	}
	var a Action
	if strict(b, &a) != nil || a.Session != s.Session || a.Window != s.Window || c.Actor < 0 || c.Actor >= len(s.Players) {
		return false
	}
	p := s.Players[c.Actor]
	return c.Now.UnixMilli() < p.Deadline && (s.Phase == "selection" && !p.Locked && (a.Type == "loadout" || a.Type == "lock") || s.Phase == "arming" && !p.BattleReady && a.Type == "battle_ready" || s.Phase == "planning" && !p.Submitted && a.Type == "submit")
}
func publicPlayer(p Player, reveal bool) Player {
	if p.Pet.Charge != nil {
		charge := *p.Pet.Charge
		charge.Target = -1
		p.Pet.Charge = &charge
	}
	p.Inventory = nil
	p.Magic = nil
	if !reveal {
		p.Loadout = nil
	}
	return p
}
func makeView(m gameapi.Match, viewer int) (any, error) {
	s, e := decode(m)
	if e != nil {
		return nil, e
	}
	if viewer < -1 || viewer >= len(s.Players) {
		return nil, errors.New("无效观看席位")
	}
	v := View{Rules: s.Rules, Session: s.Session, Window: s.Window, Phase: s.Phase, Round: s.Round, Options: s.Options, Pool: s.Pool, Players: make([]Player, len(s.Players)), Viewer: viewer, Events: append([]Event{}, s.Events...), Playback: s.Playback, Reason: s.Reason}
	v.PointBudget = pointBudget
	for i, p := range s.Players {
		v.Players[i] = publicPlayer(p, s.Phase != "selection")
		if i == viewer {
			v.Self = &Self{Loadout: p.Loadout, Inventory: p.Inventory, Magic: p.Magic, Plan: s.Plans[i]}
		}
	}
	for i := range v.Events {
		v.Events[i].Players = append([]Player(nil), v.Events[i].Players...)
		for j, p := range v.Events[i].Players {
			v.Events[i].Players[j] = publicPlayer(p, true)
		}
	}
	if v.Playback != nil {
		copy := *v.Playback
		copy.Before = append([]Player(nil), copy.Before...)
		for i, p := range copy.Before {
			copy.Before[i] = publicPlayer(p, true)
		}
		v.Playback = &copy
	}
	return v, nil
}
