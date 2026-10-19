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

	"github.com/k0ngk0ng/wire-board/internal/gameapi"
)

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
		return nil, errors.New("疯狂坦克支持 2、4、6、8 或 10 人")
	}
	o := Options{Map: "hills", TurnSeconds: 25}
	if len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if err := strict(raw, &o); err != nil {
			return nil, errors.New("无效疯狂坦克设置")
		}
	}
	if o.Map != "random" {
		if _, ok := mapByID(o.Map); !ok {
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
		ID: "fortressblue", Name: "疯狂坦克", MinPlayers: 2, MaxPlayers: 10,
		Configure: configure, Start: start, Apply: apply, View: view,
		BotAction: botAction, Reclaim: reclaim, Restore: restore,
		AcceptStale: acceptStale, OnTimer: onTimer,
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
		mapID = mapOrder[int(seed%uint64(len(mapOrder)))]
	}
	heights := generateHeights(seed, mapID)
	flattenSpawns(heights, ctx.Players)
	token := [12]byte{}
	if _, err = rand.Read(token[:]); err != nil {
		return gameapi.Match{}, err
	}
	positions := spawnYs(heights, ctx.Players)
	tanks := make([]Tank, ctx.Players)
	for seat := range tanks {
		team := teamOf(seat, ctx.Players)
		facing := 1
		if team == TeamBlue {
			facing = -1
		}
		tanks[seat] = Tank{Seat: seat, Team: team, X: positions[seat][0], Y: positions[seat][1], Facing: facing, Alive: true}
	}
	s := State{
		Options: o, Epoch: hex.EncodeToString(token[:]), Seed: seed, MapID: mapID,
		Phase: PhaseSelect, SelectUntil: ctx.Now.Add(SelectMS * time.Millisecond).UnixMilli(),
		Heights: heights, Tanks: tanks, Order: turnOrder(ctx.Players),
		Events: []Event{}, Log: []string{}, DamageTotal: make([]int, ctx.Players),
	}
	s.note("疯狂坦克：选择坦克，15 秒后由红蓝两队交替开火；注意风向与可破坏地形")
	return s.envelope()
}

func beginPlaying(s *State, at time.Time) {
	if s.Phase != PhaseSelect {
		return
	}
	for i := range s.Tanks {
		t := &s.Tanks[i]
		class := t.Class
		if _, ok := classByID(class); !ok {
			class = classOrder[(i+int(s.Seed%uint64(len(classOrder))))%len(classOrder)]
		}
		spec, _ := classByID(class)
		t.Class, t.Selected = class, true
		t.MaxHP, t.HP = spec.MaxHP, spec.MaxHP
		t.Fuel, t.Special, t.Shield = spec.Move, spec.SpecialAmmo, 0
		t.Damage, t.Kills, t.Deaths = 0, 0, 0
		t.Alive = true
		t.Y = groundAt(s.Heights, t.X)
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
	next := nextAlive(s, s.Current)
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
	spec, _ := classByID(s.Tanks[next].Class)
	s.Tanks[next].Fuel = spec.Move
	s.Deadline = at.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
	s.emit("turn", next, -1, 0, 0, 0, "", at.UnixMilli())
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
	if _, ok := mapByID(s.MapID); !ok {
		return nil, errors.New("无效疯狂坦克地图")
	}
	if !heightsValid(s.Heights) || (s.Phase == PhaseSelect && !heightsSymmetric(s.Heights)) {
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
	for i := range s.Tanks {
		t := &s.Tanks[i]
		if t.Seat != i || t.Team != teamOf(i, players) || math.IsNaN(t.X) || math.IsNaN(t.Y) || t.X < 0 || t.X > FieldWidth || t.Y < 0 || t.Y > FieldHeight || t.HP < 0 || t.MaxHP < 0 || t.HP > t.MaxHP || t.Fuel < 0 || t.Fuel > 200 || t.Special < 0 || t.Damage < 0 || t.Kills < 0 {
			return nil, errors.New("无效疯狂坦克席位")
		}
		if t.Selected {
			spec, ok := classByID(t.Class)
			if !ok {
				return nil, errors.New("无效疯狂坦克类型")
			}
			if s.Phase != PhaseSelect && (t.MaxHP != spec.MaxHP || t.Special > spec.SpecialAmmo) {
				return nil, errors.New("无效疯狂坦克类型")
			}
		}
	}
	if s.Wind < -20 || s.Wind > 20 || s.Turn < 0 {
		return nil, errors.New("无效疯狂坦克风力或回合")
	}
	if s.Phase == PhasePlaying {
		if s.Current < 0 || s.Current >= players || !s.Tanks[s.Current].Alive || s.Deadline <= 0 || s.StartAt <= 0 || s.MatchEndsAt <= 0 {
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
		if len(s.LastShot.Arcs) == 0 || len(s.LastShot.Arcs) > 4 || len(s.LastShot.Blasts) > 32 {
			return nil, errors.New("无效疯狂坦克弹道")
		}
		total := 0
		for _, arc := range s.LastShot.Arcs {
			total += len(arc.Path)
			if len(arc.Path) > MaxPath || len(arc.Path) == 0 {
				return nil, errors.New("无效疯狂坦克弹道")
			}
		}
		if total > MaxPath {
			return nil, errors.New("弹道点数超出上限")
		}
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
		if s.Phase != PhaseSelect || s.Finished || s.Tanks[ctx.Actor].Selected || a.Turn != 0 || a.Dir != 0 || a.Angle != 0 || a.Power != 0 || a.Facing != 0 || a.Weapon != "" {
			return m, errors.New("当前不能选择坦克")
		}
		if _, ok := classByID(a.Class); !ok {
			return m, errors.New("请选择有效坦克")
		}
		t := &s.Tanks[ctx.Actor]
		t.Class, t.Selected = a.Class, true
		s.note(fmt.Sprintf("%d 号位选择了%s", ctx.Actor+1, classes[a.Class].Name))
		all := true
		for _, tank := range s.Tanks {
			if !tank.Selected {
				all = false
				break
			}
		}
		if all {
			beginPlaying(s, ctx.Now)
		}
	case "move":
		if a.Class != "" || a.Angle != 0 || a.Power != 0 || a.Facing != 0 || a.Weapon != "" || (a.Dir != -1 && a.Dir != 1) {
			return m, errors.New("无效移动操作")
		}
		if !s.canAct(ctx.Actor) || a.Turn != s.Turn {
			return m, errors.New("还没轮到你移动")
		}
		if !moveTank(s, ctx.Actor, a.Dir, ctx.Now.UnixMilli()) {
			return m, errors.New("燃料不足或前方不可通行")
		}
	case "fire":
		if a.Class != "" || a.Dir != 0 || a.Angle < 5 || a.Angle > 85 || a.Power < 20 || a.Power > 100 || (a.Facing != -1 && a.Facing != 1) || (a.Weapon != "primary" && a.Weapon != "special") {
			return m, errors.New("无效开火参数（角度 5–85，力度 20–100）")
		}
		if !s.canAct(ctx.Actor) || a.Turn != s.Turn {
			return m, errors.New("还没轮到你开火")
		}
		spec, ok := classByID(s.Tanks[ctx.Actor].Class)
		if !ok {
			return m, errors.New("坦克类型无效")
		}
		weapon := spec.Primary
		if a.Weapon == "special" {
			if s.Tanks[ctx.Actor].Special <= 0 {
				return m, errors.New("特殊炮弹已用完")
			}
			weapon = spec.Special
			s.Tanks[ctx.Actor].Special--
		}
		resolveShot(s, ctx.Actor, weapon, a.Angle, a.Power, a.Facing, ctx.Now)
	default:
		return m, errors.New("未知疯狂坦克操作")
	}
	return s.envelope()
}

func resolveShot(s *State, seat int, weapon Weapon, angle, power float64, facing int, at time.Time) {
	shooter := &s.Tanks[seat]
	shotID := int64(1)
	if s.LastShot != nil {
		shotID = s.LastShot.ID + 1
	}
	shot := &Shot{ID: shotID, Seat: seat, Weapon: weapon.Name, At: at.UnixMilli()}
	shots := weapon.Shots
	if shots < 1 {
		shots = 1
	}
	limit := max(32, MaxPath/shots)
	for i := 0; i < shots; i++ {
		offset := (float64(i) - float64(shots-1)/2) * weapon.Spread
		arc := simulateArc(s.Heights, s.Tanks, seat, shooter.X, shooter.Y, angle+offset, power, facing, s.Wind, limit)
		shot.Arcs = append(shot.Arcs, arc)
	}
	s.emit("shot", seat, -1, shooter.X, shooter.Y, int(weapon.Damage), weapon.Name, at.UnixMilli())
	total := 0
	for _, arc := range shot.Arcs {
		s.emit("explode", seat, -1, arc.Impact.X, arc.Impact.Y, 0, "", at.UnixMilli())
		applyBlast(s, seat, weapon, arc.Impact, &shot.Blasts, at, &total)
	}
	s.LastShot = shot
	settleTanks(s, nil, at.UnixMilli())
	if !s.checkTeamOutcome(at.UnixMilli()) {
		advanceTurn(s, at)
	}
	if total > 0 {
		s.note(fmt.Sprintf("%d 号位开火，合计造成 %d 点效果", seat+1, total))
	} else {
		s.note(fmt.Sprintf("%d 号位开火", seat+1))
	}
}

func applyBlast(s *State, seat int, weapon Weapon, center Point, blasts *[]Blast, at time.Time, total *int) {
	radius := weapon.Blast
	if radius < 4 {
		radius = 4
	}
	blast := Blast{X: center.X, Y: center.Y, R: radius, Seat: seat}
	shooter := &s.Tanks[seat]
	if weapon.Heal {
		for i := range s.Tanks {
			t := &s.Tanks[i]
			if !t.Alive || t.Team != shooter.Team {
				continue
			}
			dist := math.Hypot(center.X-t.X, center.Y-(t.Y-7))
			if dist > radius {
				continue
			}
			heal := int(30 * (1 - 0.4*dist/radius))
			if heal <= 0 {
				continue
			}
			if t.HP+heal > t.MaxHP {
				heal = t.MaxHP - t.HP
			}
			if heal > 0 {
				t.HP += heal
				blast.Heal += heal
				*total += heal
				s.emit("heal", seat, i, t.X, t.Y, heal, "", at.UnixMilli())
			}
		}
	} else {
		for i := range s.Tanks {
			t := &s.Tanks[i]
			if !t.Alive || (t.Team == shooter.Team && i != seat) {
				continue
			}
			dist := math.Hypot(center.X-t.X, center.Y-(t.Y-7))
			if dist > radius {
				continue
			}
			damage := int(math.Round(weapon.Damage * (1 - 0.55*dist/radius)))
			if s.Sudden {
				damage *= 2
			}
			if damage < 1 {
				damage = 1
			}
			t.HP -= damage
			t.Damage += damage
			shooter.Damage += damage
			s.DamageTotal[seat] += damage
			blast.Victim, blast.Damage = i, damage
			*total += damage
			s.emit("hit", seat, i, t.X, t.Y, damage, "", at.UnixMilli())
			if t.HP <= 0 && t.Alive {
				t.HP, t.Alive, t.Deaths = 0, false, t.Deaths+1
				if i != seat {
					shooter.Kills++
				}
				s.emit("death", seat, i, t.X, t.Y, 0, "", at.UnixMilli())
			}
		}
		if carveTerrain(s.Heights, center.X, center.Y, radius) {
			blast.Terrain = true
		}
	}
	*blasts = append(*blasts, blast)
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
			beginPlaying(s, now)
			return s.envelope()
		}
		return s.envelope()
	}
	if s.Phase != PhasePlaying {
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
	return s.envelope()
}

func reclaim(ctx gameapi.Context, m gameapi.Match) (gameapi.Match, error) {
	s, err := decode(m)
	if err != nil {
		return m, err
	}
	if s.Phase == PhasePlaying && ctx.Actor == s.Current && s.Deadline <= ctx.Now.UnixMilli() {
		s.Deadline = ctx.Now.Add(time.Duration(s.Options.TurnSeconds) * time.Second).UnixMilli()
	}
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
			beginPlaying(s, ctx.Now)
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
