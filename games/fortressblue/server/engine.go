package fortressblue

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	fbdata "github.com/k0ngk0ng/wire-board/games/fortressblue/data"
	fbmaps "github.com/k0ngk0ng/wire-board/games/fortressblue/maps"
	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

type Weapon struct {
	Name   string
	Damage float64
	Blast  float64
	Kind   string
	Ammo   bool
}

func strict(raw json.RawMessage, v any) error {
	if len(raw) == 0 || len(raw) > MaxAction || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
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

func configure(players int, raw json.RawMessage) (json.RawMessage, error) {
	if teamSize(players) == 0 {
		return nil, errors.New("疯狂坦克支持 2、4、6 或 8 人")
	}
	o := Options{Map: "stage-1", TurnSeconds: 30}
	if len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := strict(raw, &o); err != nil {
			return nil, errors.New("无效疯狂坦克设置")
		}
	}
	if o.Map != "random" {
		if _, ok := fbmaps.ByID(o.Map); !ok {
			return nil, errors.New("请选择有效地图")
		}
	}
	if o.TurnSeconds < 15 || o.TurnSeconds > 40 {
		return nil, errors.New("每回合时限需为 15～40 秒")
	}
	return json.Marshal(o)
}

func Module() gameapi.Module {
	return gameapi.Module{
		ID: "fortressblue", Name: "疯狂坦克", MinPlayers: MinPlayers, MaxPlayers: MaxPlayers,
		Configure: configure, Start: start, Apply: apply, View: view,
		BotAction: botAction, Reclaim: reclaim, Restore: restore,
		AcceptStale: acceptStale, OnTimer: onTimer, PreciseTimers: true,
	}
}

func start(ctx gameapi.Context, raw json.RawMessage) (gameapi.Match, error) {
	oRaw, err := configure(ctx.Players, raw)
	if err != nil {
		return gameapi.Match{}, err
	}
	var o Options
	_ = json.Unmarshal(oRaw, &o)
	seedHash := sha256.Sum256([]byte(fmt.Sprintf("%s/%d/%d", ctx.Now.Format(time.RFC3339Nano), ctx.Version, ctx.Actor)))
	seed := uint64(seedHash[0])<<56 | uint64(seedHash[1])<<48 | uint64(seedHash[2])<<40 | uint64(seedHash[3])<<32 | uint64(seedHash[4])<<24 | uint64(seedHash[5])<<16 | uint64(seedHash[6])<<8 | uint64(seedHash[7])
	mapID := o.Map
	if mapID == "random" {
		c, err := fbmaps.All()
		if err != nil {
			return gameapi.Match{}, err
		}
		mapID = c.Maps[int(seed%uint64(len(c.Maps)))].ID
	}
	m, ok := fbmaps.ByID(mapID)
	if !ok {
		return gameapi.Match{}, errors.New("地图不存在")
	}
	token := [12]byte{}
	if _, err = rand.Read(token[:]); err != nil {
		return gameapi.Match{}, err
	}
	t := ctx.Players / 2
	teamSpawns := [2][]fbmaps.Spawn{}
	for _, sp := range m.Spawns {
		teamSpawns[sp.Team] = append(teamSpawns[sp.Team], sp)
	}
	tanks := make([]Tank, ctx.Players)
	for seat := range tanks {
		team := teamOf(seat, ctx.Players)
		slot := seat % t
		spawn := teamSpawns[team][slot]
		facing := 1
		if team == TeamBlue {
			facing = -1
		}
		tanks[seat] = Tank{Seat: seat, Team: team, X: float64(spawn.X), Y: float64(spawn.Y), Facing: facing, Alive: true}
	}
	s := State{
		Options: o, Epoch: hex.EncodeToString(token[:]), Seed: seed, MapID: mapID,
		Phase: PhaseSelect, SelectUntil: ctx.Now.Add(SelectMS * time.Millisecond).UnixMilli(),
		Tanks: tanks, Order: turnOrder(ctx.Players), Events: []Event{}, Log: []string{},
		DamageTotal: make([]int, ctx.Players), Craters: []Crater{},
	}
	s.note(fmt.Sprintf("疯狂坦克：%s；选择坦克后由红蓝两队交替行动，注意风向与可破坏地形", m.Name))
	return s.envelope()
}

func beginPlaying(s *State, at time.Time) error {
	if s.Phase != PhaseSelect {
		return nil
	}
	for i := range s.Tanks {
		t := &s.Tanks[i]
		id := t.TankID
		if _, ok := tankSpec(id); !ok || !t.Selected {
			ids := fbdata.IDs()
			if len(ids) == 0 {
				return nil
			}
			id = ids[(i+int(s.Seed%uint64(len(ids))))%len(ids)]
		}
		t.TankID, t.Selected = id, true
		spec, _ := tankSpec(id)
		t.MaxHP, t.HP = spec.HP, spec.HP
		t.Fuel, t.Special = float64(spec.Speed), -1
		t.SpecialUnlimited = true
		t.PendingShotEffect = ""
		t.Damage, t.Kills, t.Deaths, t.Alive, t.Statuses = 0, 0, 0, true, nil
		if len(t.Items) == 0 {
			t.Items = startingItems(s.Epoch, i)
		}
	}
	m, _ := fbmaps.ByID(s.MapID)
	if err := placeSafeSpawns(s, m, maskFor(m, s.Craters)); err != nil {
		return err
	}
	s.Phase = PhasePlaying
	s.StartAt = at.UnixMilli()
	s.SuddenAt = s.StartAt + SuddenAfterMS
	s.MatchEndsAt = s.StartAt + MatchLimitMS
	s.Turn = 1
	s.Current = s.Order[0]
	s.Wind = windFor(s.Seed, s.Turn, false)
	s.Deadline = at.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
	s.emit("start", -1, -1, 0, 0, 0, "", at.UnixMilli())
	s.note("双方坦克已就位；红队先手，风力和每回合时限已生效")
	return nil
}

func nextAlive(s *State, from int) int {
	if len(s.Order) == 0 {
		return -1
	}
	pos := -1
	for i, seat := range s.Order {
		if seat == from {
			pos = i
			break
		}
	}
	for n := 1; n <= len(s.Order); n++ {
		seat := s.Order[(pos+n+len(s.Order))%len(s.Order)]
		if s.Tanks[seat].Alive {
			return seat
		}
	}
	return -1
}

func advanceTurn(s *State, at time.Time) {
	if s.checkTeamOutcome(at.UnixMilli()) {
		return
	}
	next := -1
	from := s.Current
	for attempt := 0; attempt < len(s.Tanks); attempt++ {
		next = nextAlive(s, from)
		if next < 0 {
			break
		}
		// Project semantics: a status cycle belongs to the affected tank's next
		// action opportunity, not to every other player's action or flying shell.
		tickStatuses(s, next, at.UnixMilli())
		if s.checkTeamOutcome(at.UnixMilli()) {
			return
		}
		if s.Tanks[next].Alive {
			break
		}
		from = next
	}
	if next < 0 {
		if !s.Finished {
			s.checkTeamOutcome(at.UnixMilli())
			if !s.Finished {
				s.finish(true, -1, "没有可行动的坦克", at.UnixMilli())
			}
		}
		return
	}
	s.Current = next
	s.Turn++
	s.Wind = windFor(s.Seed, s.Turn, s.Sudden)
	if spec, ok := tankSpec(s.Tanks[next].TankID); ok {
		s.Tanks[next].Fuel = float64(spec.Speed)
	}
	s.Deadline = at.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
	s.emit("turn", next, -1, 0, 0, 0, "", at.UnixMilli())
	s.note(fmt.Sprintf("轮到 %d 号位行动", next+1))
}

func decode(m gameapi.Match) (*State, error) {
	if m.Schema != 1 || len(m.Data) == 0 {
		return nil, errors.New("无效疯狂坦克存档")
	}
	var s State
	if err := json.Unmarshal(m.Data, &s); err != nil {
		return nil, errors.New("无效疯狂坦克存档")
	}
	players := len(s.Tanks)
	if teamSize(players) == 0 || len(s.DamageTotal) != players || s.Epoch == "" || s.Seed == 0 {
		return nil, errors.New("无效疯狂坦克玩家")
	}
	if _, err := configure(players, mustJSON(s.Options)); err != nil {
		return nil, errors.New("无效疯狂坦克设置")
	}
	mapSpec, ok := fbmaps.ByID(s.MapID)
	if !ok {
		return nil, errors.New("无效疯狂坦克地图")
	}
	if s.Phase != PhaseSelect && s.Phase != PhasePlaying && s.Phase != PhaseFinished {
		return nil, errors.New("无效疯狂坦克阶段")
	}
	if len(s.Order) != players {
		return nil, errors.New("无效疯狂坦克行动顺序")
	}
	seen := map[int]bool{}
	for _, seat := range s.Order {
		if seat < 0 || seat >= players || seen[seat] {
			return nil, errors.New("无效疯狂坦克行动顺序")
		}
		seen[seat] = true
	}
	if len(s.Craters) > MaxCraters {
		return nil, errors.New("弹坑过多")
	}
	for _, c := range s.Craters {
		if math.IsNaN(c.X) || math.IsNaN(c.Y) || c.X < 0 || c.X > float64(mapSpec.Width) || c.Y < 0 || c.Y > float64(mapSpec.Height)+80 || c.R < 0 || c.R > 120 {
			return nil, errors.New("无效弹坑")
		}
	}
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if t.Seat != i || t.Team != teamOf(i, players) || math.IsNaN(t.X) || math.IsNaN(t.Y) || t.X < 0 || t.X > float64(mapSpec.Width) || t.Y < -100 || t.Y > float64(mapSpec.Height)+300 || t.HP < 0 || t.MaxHP < 0 || t.HP > t.MaxHP || t.Fuel < 0 || t.Fuel > 200 || (!t.SpecialUnlimited && (t.Special < 0 || t.Special > MaxSpecial)) || (t.SpecialUnlimited && t.Special != -1) || (t.PendingShotEffect != "" && t.PendingShotEffect != "heal") || t.Damage < 0 || t.Kills < 0 || len(t.Statuses) > MaxStatuses {
			return nil, errors.New("无效疯狂坦克席位")
		}
		if t.Selected {
			spec, ok := tankSpec(t.TankID)
			// Published v2 used a 990-HP fallback for tank 13. Preserve live
			// legacy health instead of rejecting or healing that saved match.
			legacyHP := t.TankID == 13 && t.MaxHP == 990 && !t.SpecialUnlimited
			if !ok || (s.Phase != PhaseSelect && t.MaxHP != spec.HP && !legacyHP) {
				return nil, errors.New("无效疯狂坦克类型")
			}
		}
		if len(t.Items) > MaxItems || t.ItemTurn < 0 || t.ItemTurn > s.Turn+1 {
			return nil, errors.New("无效疯狂坦克道具")
		}
		for _, itemID := range t.Items {
			if _, ok := fbdata.ItemByID(itemID); !ok {
				return nil, errors.New("无效疯狂坦克道具")
			}
		}
	}
	if s.Wind < -20 || s.Wind > 20 || s.Turn < 0 {
		return nil, errors.New("无效疯狂坦克风力或回合")
	}
	if s.Phase == PhasePlaying {
		if s.Current < 0 || s.Current >= players || (s.Resolution == nil && (!s.Tanks[s.Current].Alive || s.Deadline <= 0)) || s.StartAt <= 0 || s.MatchEndsAt <= 0 {
			return nil, errors.New("无效疯狂坦克对战状态")
		}
		for _, t := range s.Tanks {
			if !t.Selected {
				return nil, errors.New("对战中有未选择坦克")
			}
		}
	}
	if s.Phase == PhaseSelect && s.SelectUntil <= 0 {
		return nil, errors.New("无效疯狂坦克选择期限")
	}
	if s.LastShot != nil {
		if len(s.LastShot.Arcs) == 0 || len(s.LastShot.Arcs) > 8 || len(s.LastShot.Blasts) > 16 {
			return nil, errors.New("无效疯狂坦克弹道")
		}
		total := 0
		for _, arc := range s.LastShot.Arcs {
			total += len(arc.Path)
			if len(arc.PathTimes) > 0 {
				if len(arc.PathTimes) != len(arc.Path) {
					return nil, errors.New("弹道时间采样不完整")
				}
				prev := -1.0
				for _, stamp := range arc.PathTimes {
					if stamp < prev || stamp < 0 || stamp > 30000 {
						return nil, errors.New("无效弹道时间")
					}
					prev = stamp
				}
			}
			if len(arc.Path) > MaxPath || len(arc.Path) == 0 {
				return nil, errors.New("无效疯狂坦克弹道")
			}
		}
		if total > MaxPath {
			return nil, errors.New("弹道点数超出上限")
		}
	}
	if s.Resolution != nil {
		r := s.Resolution
		if s.Phase != PhasePlaying || s.Deadline != 0 || r.At <= 0 || r.ImpactAt < r.At || r.SettleAt < r.ImpactAt || (r.Stage != "flying" && r.Stage != "settling") || len(r.Falls) > players {
			return nil, errors.New("无效疯狂坦克动作时间轴")
		}
		if r.ShotID > 0 && (s.LastShot == nil || s.LastShot.ID != r.ShotID || s.LastShot.ImpactAt != r.ImpactAt || s.LastShot.SettleAt != r.SettleAt) {
			return nil, errors.New("炮弹时间轴不一致")
		}
		if r.Stage == "flying" && (r.ShotID <= 0 || s.PendingShot == nil || len(r.Falls) > 0) {
			return nil, errors.New("无效飞行状态")
		}
		if r.Stage == "settling" && s.PendingShot != nil {
			return nil, errors.New("无效命中状态")
		}
		if r.ShotID == 0 && (r.Stage != "settling" || s.MoveRemainingMS <= 0) {
			return nil, errors.New("无效移动落地状态")
		}
		fallen := map[int]bool{}
		for _, f := range r.Falls {
			if f.Seat < 0 || f.Seat >= players || fallen[f.Seat] || f.At < r.ImpactAt || (f.EndAt < f.At || (f.EndAt == f.At && !f.Landed)) || f.EndAt > r.SettleAt || f.FromX < 0 || f.FromX > float64(mapSpec.Width) || f.ToX < 0 || f.ToX > float64(mapSpec.Width) || f.FromY < -100 || f.ToY < f.FromY || f.ToY > float64(mapSpec.Height)+300 {
				return nil, errors.New("无效落地轨迹")
			}
			fallen[f.Seat] = true
		}
	} else if s.PendingShot != nil {
		return nil, errors.New("孤立的待结算炮弹")
	}
	if len(s.Hazards) > 64 {
		return nil, errors.New("地面效果过多")
	}
	hazardIDs := map[int64]bool{}
	for _, h := range s.Hazards {
		if h.ID <= 0 || hazardIDs[h.ID] || (h.Kind != "fire" && h.Kind != "poison") || h.X < 0 || h.X >= float64(mapSpec.Width) || h.Y < 0 || h.Y > float64(mapSpec.Height) || h.R <= 0 || h.R > 120 || h.At <= 0 || h.EndAt <= h.At || h.EndAt-h.At > 60000 || h.NextTickAt <= h.At || h.Source < 0 || h.Source >= players {
			return nil, errors.New("无效地面效果")
		}
		hazardIDs[h.ID] = true
	}
	if s.Resolution != nil && s.LastShot != nil && s.PendingShot != nil && s.LastShot.LaunchAt == 0 {
		s.PendingShot.Launched = true
	}
	if len(s.Events) > MaxEvents || len(s.Log) > MaxLog {
		return nil, errors.New("疯狂坦克事件过多")
	}
	if s.Finished != (s.Phase == PhaseFinished) || (s.Finished && !s.Draw && s.WinnerTeam != TeamRed && s.WinnerTeam != TeamBlue) {
		return nil, errors.New("无效疯狂坦克结果")
	}
	return &s, nil
}

func mustJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func apply(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	var a Action
	if err = strict(raw, &a); err != nil {
		return m, errors.New("无效疯狂坦克操作")
	}
	if a.Epoch != s.Epoch {
		return m, errors.New("对局已更新，请重新连接")
	}
	if ctx.Actor < 0 || ctx.Actor >= len(s.Tanks) {
		return m, errors.New("你不是本局玩家")
	}
	switch a.Type {
	case "select":
		if s.Phase != PhaseSelect || s.Finished || s.Tanks[ctx.Actor].Selected || a.Turn != 0 || a.Dir != 0 || a.Angle != 0 || a.Power != 0 || a.Facing != 0 || a.Weapon != "" || a.Slot != 0 {
			return m, errors.New("当前不能选择坦克")
		}
		if _, ok := tankSpec(a.Tank); !ok {
			return m, errors.New("请选择有效坦克")
		}
		t := &s.Tanks[ctx.Actor]
		t.TankID, t.Selected = a.Tank, true
		spec, _ := tankSpec(a.Tank)
		s.note(fmt.Sprintf("%d 号位选择了%s", ctx.Actor+1, spec.Name))
		all := true
		for _, tank := range s.Tanks {
			if !tank.Selected {
				all = false
				break
			}
		}
		if all {
			if err = beginPlaying(s, ctx.Now); err != nil {
				return m, err
			}
		}
	case "move":
		if a.Tank != 0 || a.Angle != 0 || a.Power != 0 || a.Facing != 0 || a.Weapon != "" || a.Slot != 0 {
			return m, errors.New("无效移动操作")
		}
		if !s.canAct(ctx.Actor) || a.Turn != s.Turn {
			return m, errors.New("还没轮到你移动")
		}
		if !moveTank(s, ctx.Actor, a.Dir, ctx.Now.UnixMilli()) {
			return m, errors.New("移动力不足或前方不可通行")
		}
	case "item":
		if a.Tank != 0 || a.Dir != 0 || a.Angle != 0 || a.Power != 0 || a.Facing != 0 || a.Weapon != "" {
			return m, errors.New("无效道具操作")
		}
		if a.Turn != s.Turn {
			return m, errors.New("还没轮到你使用道具")
		}
		if err := useItem(s, ctx.Actor, a.Slot, ctx.Now.UnixMilli()); err != nil {
			return m, err
		}
	case "fire":
		if a.Tank != 0 || a.Dir != 0 || a.Slot != 0 || (a.Facing != -1 && a.Facing != 1) || (a.Weapon != "primary" && a.Weapon != "special") {
			return m, errors.New("无效开火参数")
		}
		spec, ok := tankSpec(s.Tanks[ctx.Actor].TankID)
		if !ok || a.Angle < float64(spec.MinAngle) || a.Angle > float64(spec.Angle) || a.Power < 0 || a.Power > 100 {
			return m, errors.New("无效开火参数（角度须在坦克射界内，力度 0–100）")
		}
		if !s.canAct(ctx.Actor) || a.Turn != s.Turn {
			return m, errors.New("还没轮到你开火")
		}
		weapon := Weapon{Name: "主炮", Damage: float64(spec.Primary), Blast: 14}
		kind := ""
		if s.Tanks[ctx.Actor].PendingShotEffect == "heal" {
			weapon = Weapon{Name: "healing", Blast: 14}
			kind = "heal"
			s.Tanks[ctx.Actor].PendingShotEffect = ""
		} else if a.Weapon == "special" {
			if !s.Tanks[ctx.Actor].SpecialUnlimited && s.Tanks[ctx.Actor].Special <= 0 {
				return m, errors.New("特殊炮弹已用完")
			}
			if !s.Tanks[ctx.Actor].SpecialUnlimited {
				s.Tanks[ctx.Actor].Special--
			}
			weapon = Weapon{Name: "特殊弹", Damage: float64(spec.Special), Blast: 22}
			if len(spec.Skills) > 0 {
				switch spec.Skills[0] {
				case 1:
					kind, weapon.Blast = "explosion", weapon.Blast*1.25
					weapon.Damage *= 1.1
				case 2:
					kind = "poison"
				case 3:
					kind, weapon.Blast = "fire", weapon.Blast*1.15
				case 4:
					kind = "lightning"
				}
			}
		}
		resolveShot(s, ctx.Actor, weapon, kind, a.Angle, a.Power, a.Facing, ctx.Now)
	default:
		return m, errors.New("未知疯狂坦克操作")
	}
	return s.envelope()
}

func resolveShot(s *State, seat int, weapon Weapon, kind string, angle, power float64, facing int, at time.Time) {
	shooter := &s.Tanks[seat]
	wind := s.Wind
	// Old snapshots may contain the former no-wind modifier; consume it without
	// removing ordinary wind, which the original piercing item does not negate.
	consumeStatus(shooter, "nowind")
	if kind != "heal" {
		powered := consumeStatus(shooter, "power")
		piercing := consumeStatus(shooter, "piercing")
		if powered || piercing {
			weapon.Damage *= 1.5
		}
	}
	m, _ := fbmaps.ByID(s.MapID)
	mask := bulletMaskFor(m, s.Craters)
	shotID := int64(1)
	if s.LastShot != nil {
		shotID = s.LastShot.ID + 1
	}
	shot := &Shot{ID: shotID, Seat: seat, Weapon: weapon.Name, At: at.UnixMilli(), Blasts: []Blast{}}
	spec, _ := tankSpec(shooter.TankID)
	arc := simulateArc(m, mask, s.Tanks, seat, shooter.X, shooter.Y, angle, power, facing, float64(spec.Range)/50, wind, MaxPath)
	flight := arc.PathTimes[len(arc.PathTimes)-1]
	duration := math.Max(180, flight)
	if flight > 0 {
		for i := range arc.PathTimes {
			arc.PathTimes[i] *= duration / flight
		}
	}
	shot.LaunchAt = shot.At + 334
	for i := range arc.PathTimes {
		arc.PathTimes[i] += 334
	}
	arc.ImpactAt = shot.LaunchAt + int64(math.Ceil(duration))
	shot.ImpactAt, shot.SettleAt = arc.ImpactAt, arc.ImpactAt+450
	shot.Arcs = []Arc{arc}
	shooter.Facing = facing
	s.LastShot = shot
	s.PendingShot = &PendingShot{Weapon: weapon, Kind: kind}
	s.Resolution = &Resolution{Stage: "flying", LaunchAt: shot.LaunchAt, ShotID: shotID, At: shot.At, ImpactAt: shot.ImpactAt, SettleAt: shot.SettleAt}
	s.Deadline = 0
	s.emit("fire_ready", seat, -1, shooter.X, shooter.Y, 0, weapon.Name, shot.At)

}

func nearestEnemy(s *State, seat, except int, center Point, radius float64) int {
	best, bestDist := -1, math.Inf(1)
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if i == except || !t.Alive || t.Team == s.Tanks[seat].Team {
			continue
		}
		c := tankCenter(*t)
		d := math.Hypot(c.X-center.X, c.Y-center.Y)
		if d <= radius && d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

func applyDirectDamage(s *State, seat, victim, damage int, kind string, at int64) {
	t := &s.Tanks[victim]
	if !t.Alive {
		return
	}
	damage = min(t.HP, shieldedDamage(t, damage))
	if damage == 0 {
		s.emit("shield", seat, victim, t.X, t.Y, 0, kind, at)
		return
	}
	t.HP -= damage
	if seat != victim {
		s.Tanks[seat].Damage += damage
		s.DamageTotal[seat] += damage
	}
	s.emit("hit", seat, victim, t.X, t.Y, damage, kind, at)
	if t.HP <= 0 {
		t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
		s.Tanks[seat].Kills++
		s.emit("death", seat, victim, t.X, t.Y, 0, "", at)
	}
}

func onTimer(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if s.Finished {
		return s.envelope()
	}
	now := ctx.Now
	if s.Phase == PhaseSelect {
		if now.UnixMilli() >= s.SelectUntil {
			if err = beginPlaying(s, now); err != nil {
				return m, err
			}
			return s.envelope()
		}
		return s.envelope()
	}
	if s.Phase != PhasePlaying {
		return s.envelope()
	}
	processWorldTimeline(s, now)
	if s.Resolution != nil || s.Finished {
		return s.envelope()
	}
	if now.UnixMilli() >= s.MatchEndsAt {
		s.timeOutcome(now.UnixMilli())
		return s.envelope()
	}
	if !s.Sudden && now.UnixMilli() >= s.SuddenAt {
		s.Sudden = true
		s.Wind = windFor(s.Seed, s.Turn, true)
		s.emit("sudden", -1, -1, 0, 0, 0, "", now.UnixMilli())
		s.note("突然死亡：伤害翻倍，风力增强")
	}
	if now.UnixMilli() >= s.Deadline {
		s.note(fmt.Sprintf("%d 号位行动超时", s.Current+1))
		s.Deadline = now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
		advanceTurn(s, now)
	}
	return s.envelope()
}

func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	// Reclaim changes ownership only. Stage and action deadlines are authoritative.
	return s.envelope()
}

func restore(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if s.Phase == PhaseSelect {
		all := true
		for _, t := range s.Tanks {
			if !t.Selected {
				all = false
			}
		}
		if all {
			if err = beginPlaying(s, ctx.Now); err != nil {
				return m, err
			}
		}
	}
	return s.envelope()
}

func acceptStale(ctx gameapi.Context, m gameapi.Match, raw json.RawMessage) bool {
	s, err := decode(m)
	if err != nil {
		return false
	}
	var a Action
	if strict(raw, &a) != nil || a.Epoch != s.Epoch || ctx.Actor < 0 || ctx.Actor >= len(s.Tanks) {
		return false
	}
	if s.Phase == PhaseSelect {
		return !s.Tanks[ctx.Actor].Selected && a.Type == "select"
	}
	return s.canAct(ctx.Actor) && a.Type != "select"
}

func view(m gameapi.Match, seat int) (any, error) {
	s, err := decode(m)
	if err != nil {
		return nil, err
	}
	return s.project(seat), nil
}
