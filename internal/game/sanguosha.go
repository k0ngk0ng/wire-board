package game

import (
	"errors"
	"fmt"
	"slices"
)

type SGPlayer struct {
	Chained  bool           `json:"chained,omitempty"`
	Drank    int            `json:"drank,omitempty"`
	Role     string         `json:"role"`
	General  string         `json:"general"`
	HP       int            `json:"hp"`
	MaxHP    int            `json:"maxHP"`
	Dead     bool           `json:"dead"`
	Hand     []int          `json:"hand"`
	Equip    []int          `json:"equip"`
	Judgment []SGDelayed    `json:"judgment"`
	Used     map[string]int `json:"used"`
	Choices  []string       `json:"choices"`
}
type SGDelayed struct {
	Card int    `json:"card"`
	Kind string `json:"kind"`
}

// Every suspended effect is plain data, including nested rescue / counterspell
// windows. No goroutine, callback or browser state is required to resume play.
type SGEvent struct {
	Nature  string   `json:"nature,omitempty"`
	Chain   bool     `json:"chain,omitempty"`
	Next    *SGEvent `json:"next,omitempty"`
	Type    string   `json:"type"`
	Actor   int      `json:"actor"`
	Target  int      `json:"target"`
	Kind    string   `json:"kind,omitempty"`
	Cards   []int    `json:"cards,omitempty"`
	Targets []int    `json:"targets,omitempty"`
	Amount  int      `json:"amount,omitempty"`
	Step    int      `json:"step,omitempty"`
	Count   int      `json:"count,omitempty"`
	Aux     int      `json:"aux,omitempty"`
	Flag    bool     `json:"flag,omitempty"`
}
type SGPrompt struct {
	ID      int      `json:"id"`
	Player  int      `json:"player"`
	Kind    string   `json:"kind"`
	Message string   `json:"message"`
	Event   SGEvent  `json:"event"`
	Cards   []int    `json:"cards,omitempty"`
	Targets []int    `json:"targets,omitempty"`
	Choices []string `json:"choices,omitempty"`
}
type Sanguosha struct {
	Options   SGOptions  `json:"options,omitempty"`
	SkipDraw  bool       `json:"skipDraw,omitempty"`
	Revealed  []int      `json:"revealed,omitempty"`
	Players   []SGPlayer `json:"players"`
	Deck      []int      `json:"deck"`
	Discard   []int      `json:"discard"`
	Table     []int      `json:"table"`
	Queue     []SGEvent  `json:"queue"`
	Pending   *SGPrompt  `json:"pending,omitempty"`
	Sequence  int        `json:"sequence"`
	Grace     []int      `json:"grace"`
	Lord      int        `json:"lord"`
	Selecting bool       `json:"selecting"`
	Selected  int        `json:"selected"`
	InPlay    bool       `json:"inPlay"`
	SkipPlay  bool       `json:"skipPlay"`
}

func (s *State) initSanguosha(n int) {
	roles := map[int][]string{4: {"lord", "loyalist", "rebel", "renegade"}, 5: {"lord", "loyalist", "rebel", "rebel", "renegade"}, 6: {"lord", "loyalist", "rebel", "rebel", "rebel", "renegade"}, 7: {"lord", "loyalist", "loyalist", "rebel", "rebel", "rebel", "renegade"}, 8: {"lord", "loyalist", "loyalist", "rebel", "rebel", "rebel", "rebel", "renegade"}}[n]
	roles = clone(roles)
	shuffle(roles)
	g := &Sanguosha{Players: make([]SGPlayer, n), Selecting: true, Deck: []int{}, Discard: []int{}, Table: []int{}, Queue: []SGEvent{}}
	s.Sanguosha = g
	s.Phase = "sg_select"
	for _, c := range sgCards {
		g.Deck = append(g.Deck, c.ID)
	}
	shuffle(g.Deck)
	ids := []string{}
	for _, x := range SGGenerals {
		ids = append(ids, x.ID)
	}
	shuffle(ids)
	for i := range g.Players {
		g.Players[i] = SGPlayer{Role: roles[i], Hand: []int{}, Equip: []int{}, Judgment: []SGDelayed{}, Used: map[string]int{}}
		if roles[i] == "lord" {
			g.Lord = i
		}
	}
	// Monarch gets the three monarchs plus two random non-monarch generals.
	lordChoices := []string{"caocao", "liubei", "sunquan"}
	for _, id := range ids {
		if !slices.Contains(lordChoices, id) && len(lordChoices) < 5 {
			lordChoices = append(lordChoices, id)
		}
	}
	g.Players[g.Lord].Choices = lordChoices
	s.Turn = g.Lord
	s.sgAsk(g.Lord, "general", "选择你的武将", SGEvent{})
}
func (s *State) sgLog(format string, args ...any) {
	s.Log = append(s.Log, fmt.Sprintf(format, args...))
	if len(s.Log) > 80 {
		s.Log = s.Log[len(s.Log)-80:]
	}
}
func (s *State) sgName(i int) string {
	if i < 0 || i >= len(s.Sanguosha.Players) {
		return "天灾"
	}
	p := s.Sanguosha.Players[i]
	name := sgGeneral(p.General).Name
	if name == "" {
		name = "待选武将"
	}
	return fmt.Sprintf("玩家 %d（%s）", i+1, name)
}
func (s *State) sgHas(i int, skill string) bool {
	if i < 0 || i >= len(s.Sanguosha.Players) {
		return false
	}
	p := s.Sanguosha.Players[i]
	if p.Dead {
		return false
	}
	if (skill == "hujia" || skill == "jijiang" || skill == "jiuyuan") && p.Role != "lord" {
		return false
	}
	return slices.Contains(sgGeneral(p.General).Skills, skill)
}
func (s *State) sgAlive(i int) bool {
	return i >= 0 && i < len(s.Sanguosha.Players) && !s.Sanguosha.Players[i].Dead
}
func (s *State) sgNext(i int) int {
	for range s.Sanguosha.Players {
		i = (i + 1) % len(s.Sanguosha.Players)
		if s.sgAlive(i) {
			return i
		}
	}
	return i
}
func (s *State) sgOrder(start int) []int {
	out := []int{}
	for j := range s.Sanguosha.Players {
		i := (start + j) % len(s.Sanguosha.Players)
		if s.sgAlive(i) {
			out = append(out, i)
		}
	}
	return out
}
func (s *State) sgPush(events ...SGEvent) { s.Sanguosha.Queue = append(events, s.Sanguosha.Queue...) }
func (s *State) sgAsk(player int, kind, msg string, e SGEvent) {
	g := s.Sanguosha
	g.Sequence++
	g.Pending = &SGPrompt{ID: g.Sequence, Player: player, Kind: kind, Message: msg, Event: e}
	s.Phase = "sg_response"
	if kind == "general" {
		s.Phase = "sg_select"
	}
}
func (s *State) sgOptional(player int, skill string, e SGEvent) {
	e.Kind = skill
	if s.sgHas(player, skill) {
		s.sgAsk(player, "invoke", "是否发动「"+SGSkills[skill].Name+"」？", e)
		s.Sanguosha.Pending.Choices = []string{"yes", "pass"}
	}
}
func sgRemove(v []int, id int) []int {
	if i := slices.Index(v, id); i >= 0 {
		return append(v[:i], v[i+1:]...)
	}
	return v
}
func (s *State) sgDrawIDs(n int) []int {
	g := s.Sanguosha
	out := []int{}
	for range n {
		if len(g.Deck) == 0 {
			g.Deck = g.Discard
			g.Discard = []int{}
			shuffle(g.Deck)
		}
		if len(g.Deck) == 0 {
			break
		}
		out = append(out, g.Deck[0])
		g.Deck = g.Deck[1:]
	}
	return out
}
func (s *State) sgDraw(i, n int) {
	if !s.sgAlive(i) {
		return
	}
	ids := s.sgDrawIDs(n)
	s.Sanguosha.Players[i].Hand = append(s.Sanguosha.Players[i].Hand, ids...)
	s.sgLog("%s 摸了 %d 张牌", s.sgName(i), len(ids))
}
func (s *State) sgHeal(i, n int) {
	if !s.sgAlive(i) {
		return
	}
	p := &s.Sanguosha.Players[i]
	before := p.HP
	p.HP = min(p.MaxHP, p.HP+n)
	if p.HP > before {
		s.sgLog("%s 回复 %d 点体力", s.sgName(i), p.HP-before)
	}
}
func (s *State) sgEquip(i int, slot string) int {
	if !s.sgAlive(i) {
		return 0
	}
	for _, id := range s.Sanguosha.Players[i].Equip {
		if SGCardTypes[sgCard(id).Kind].Slot == slot {
			return id
		}
	}
	return 0
}
func (s *State) sgWeapon(i int) string { return sgCard(s.sgEquip(i, "weapon")).Kind }
func (s *State) sgOwn(i, id int, handOnly bool) bool {
	if !s.sgAlive(i) {
		return false
	}
	p := s.Sanguosha.Players[i]
	return slices.Contains(p.Hand, id) || (!handOnly && slices.Contains(p.Equip, id))
}
func (s *State) sgDistance(a, b int) int {
	alive := s.sgOrder(0)
	x, y := slices.Index(alive, a), slices.Index(alive, b)
	if x < 0 || y < 0 {
		return 999
	}
	d := x - y
	if d < 0 {
		d = -d
	}
	d = min(d, len(alive)-d)
	if s.sgEquip(a, "offense") != 0 {
		d--
	}
	if s.sgHas(a, "mashu") {
		d--
	}
	if s.sgEquip(b, "defense") != 0 {
		d++
	}
	return max(1, d)
}
func (s *State) sgRange(i int) int { return max(1, SGCardTypes[s.sgWeapon(i)].Range) }
func (s *State) sgCanTarget(a, b int, kind string) bool {
	if !s.sgAlive(a) || !s.sgAlive(b) {
		return false
	}
	p := s.Sanguosha.Players[b]
	if a == b && kind != "peach" && kind != "analeptic" && kind != "lightning" && kind != "ex_nihilo" && kind != "iron_chain" && kind != "fire_attack" {
		return false
	}
	if (sgIsSlash(kind) || kind == "duel") && s.sgHas(b, "kongcheng") && len(p.Hand) == 0 {
		return false
	}
	if (kind == "snatch" || kind == "indulgence") && s.sgHas(b, "qianxun") {
		return false
	}
	if sgIsSlash(kind) && s.sgDistance(a, b) > s.sgRange(a) {
		return false
	}
	if (kind == "snatch" || kind == "supply_shortage") && !s.sgHas(a, "qicai") && s.sgDistance(a, b) > 1 {
		return false
	}
	if (kind == "snatch" || kind == "dismantlement") && len(p.Hand)+len(p.Equip)+len(p.Judgment) == 0 {
		return false
	}
	if kind == "fire_attack" && len(p.Hand) == 0 {
		return false
	}
	if sgIsDelayed(kind) {
		for _, d := range p.Judgment {
			if d.Kind == kind {
				return false
			}
		}
	}
	return true
}

// Loss triggers are queued after the complete movement, never exposing a partly
// paid action. Cards remain on the table until the enclosing use finishes.
func (s *State) sgLose(i int, ids []int) {
	p := &s.Sanguosha.Players[i]
	before := len(p.Hand)
	equip := 0
	for _, id := range ids {
		if slices.Contains(p.Equip, id) {
			equip++
			if sgCard(id).Kind == "silver_lion" {
				s.sgPush(SGEvent{Type: "heal", Actor: i, Amount: 1})
			}
		}
		p.Hand = sgRemove(p.Hand, id)
		p.Equip = sgRemove(p.Equip, id)
		for j, d := range p.Judgment {
			if d.Card == id {
				p.Judgment = append(p.Judgment[:j], p.Judgment[j+1:]...)
				break
			}
		}
	}
	if before > 0 && len(p.Hand) == 0 && s.sgHas(i, "lianying") {
		s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "lianying", Amount: 1})
	}
	for range equip {
		if s.sgHas(i, "xiaoji") {
			s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "xiaoji", Amount: 2})
		}
	}
}
func (s *State) sgDiscard(i int, ids []int) {
	s.sgLose(i, ids)
	s.Sanguosha.Discard = append(s.Sanguosha.Discard, ids...)
}
func (s *State) sgPay(i int, ids []int) {
	s.sgLose(i, ids)
	s.Sanguosha.Table = append(s.Sanguosha.Table, ids...)
}
func (s *State) sgFinishCards(ids []int) {
	g := s.Sanguosha
	for _, id := range ids {
		if slices.Contains(g.Table, id) {
			g.Table = sgRemove(g.Table, id)
			g.Discard = append(g.Discard, id)
		}
	}
}
func (s *State) sgValidateCards(i int, ids []int, n int, hand bool) error {
	if n >= 0 && len(ids) != n {
		return errors.New("选择的牌数量不正确")
	}
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] || !s.sgOwn(i, id, hand) {
			return errors.New("请选择自己可使用的牌")
		}
		seen[id] = true
	}
	return nil
}
func (s *State) sgAs(i int, ids []int, skill, desired string) (string, error) {
	if err := s.sgValidateCards(i, ids, -1, false); err != nil {
		return "", err
	}
	if skill == "spear" {
		if (desired != "" && desired != "slash") || s.sgWeapon(i) != "spear" || s.sgValidateCards(i, ids, 2, true) != nil {
			return "", errors.New("丈八蛇矛需要两张手牌")
		}
		return "slash", nil
	}
	if len(ids) != 1 {
		return "", errors.New("请选择一张牌")
	}
	c := sgCard(ids[0])
	kind := c.Kind
	red := c.Suit == 1 || c.Suit == 3
	if skill == "fan" {
		if s.sgWeapon(i) != "fan" || kind != "slash" || !s.sgOwn(i, c.ID, true) {
			return "", errors.New("朱雀羽扇需要一张普通杀")
		}
		kind = "fire_slash"
	} else if skill != "" {
		if !s.sgHas(i, skill) {
			return "", errors.New("没有此技能")
		}
		switch skill {
		case "wusheng":
			if !red {
				return "", errors.New("武圣需要红色牌")
			}
			kind = "slash"
		case "qingguo":
			if red || !s.sgOwn(i, c.ID, true) {
				return "", errors.New("倾国需要黑色手牌")
			}
			kind = "jink"
		case "longdan":
			if sgIsSlash(kind) {
				kind = "jink"
			} else if kind == "jink" {
				kind = "slash"
			} else {
				return "", errors.New("龙胆需要杀或闪")
			}
		case "qixi":
			if red {
				return "", errors.New("奇袭需要黑色牌")
			}
			kind = "dismantlement"
		case "guose":
			if c.Suit != 3 {
				return "", errors.New("国色需要方块牌")
			}
			kind = "indulgence"
		case "jijiu":
			if !red || i == s.Turn {
				return "", errors.New("急救只能在回合外使用红色牌")
			}
			kind = "peach"
		default:
			return "", errors.New("此技能不能转化卡牌")
		}
	} else if !s.sgOwn(i, c.ID, true) {
		return "", errors.New("装备区的牌需通过技能转化")
	}
	if desired != "" && desired != kind && !(desired == "slash" && sgIsSlash(kind)) {
		return "", errors.New("响应的牌型不正确")
	}
	return kind, nil
}
func (s *State) applySanguosha(i int, a Action) error {
	if !s.sgAlive(i) {
		return errors.New("该角色无法行动")
	}
	// Validation errors are transactional even when checking costs needs to inspect
	// post-payment range (e.g. discarding a horse to redirect a slash).
	trial := clone(*s)
	if err := trial.sgApply(i, a); err != nil {
		return err
	}
	trial.sgRun()
	*s = trial
	return nil
}
func (s *State) sgApply(i int, a Action) error {
	g := s.Sanguosha
	if g.Pending != nil {
		if (i != g.Pending.Player && !(g.Pending.Kind == "nullification" && !slices.Contains(g.Pending.Event.Targets, i))) || a.Prompt != g.Pending.ID {
			return errors.New("响应已变化，请刷新后重试")
		}
		return s.sgRespond(i, a)
	}
	if g.Selecting || s.Turn != i || s.Phase != "sg_play" {
		return errors.New("尚未轮到你出牌")
	}
	if a.Type == "sg_end" {
		g.InPlay = false
		s.sgPush(SGEvent{Type: "discard_phase", Actor: i}, SGEvent{Type: "optional_draw", Actor: i, Kind: "biyue", Amount: 1}, SGEvent{Type: "next", Actor: i})
		return nil
	}
	if a.Type == "sg_skill" {
		return s.sgSkill(i, a)
	}
	if a.Type != "sg_play" {
		return errors.New("未知三国杀操作")
	}
	kind, err := s.sgAs(i, a.Cards, a.Skill, "")
	if err != nil {
		return err
	}
	return s.sgUse(i, kind, a.Cards, a.Targets, false)
}
func (s *State) sgUse(i int, kind string, ids, targets []int, forced bool) error {
	g := s.Sanguosha
	p := &g.Players[i]
	info := SGCardTypes[kind]
	legality := clone(*s)
	legality.sgLose(i, ids)
	if kind == "jink" || kind == "nullification" {
		return errors.New("此牌只能响应时使用")
	}
	if info.Slot != "" {
		if len(targets) != 0 {
			return errors.New("装备无需选择目标")
		}
		s.sgPush(SGEvent{Type: "equip", Actor: i, Kind: kind, Cards: ids})
		s.sgPay(i, ids)
		s.sgLog("%s 装备「%s」", s.sgName(i), info.Name)
		return nil
	}
	if kind == "iron_chain" && len(targets) == 0 {
		s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: 1})
		s.sgDiscard(i, ids)
		s.sgLog("%s 重铸铁索连环，摸一张牌", s.sgName(i))
		return nil
	}
	switch kind {
	case "analeptic":
		if p.Used["analeptic"] > 0 {
			return errors.New("本回合已经使用过酒")
		}
		targets = []int{i}
	case "peach":
		if p.HP >= p.MaxHP {
			return errors.New("体力已满")
		}
		targets = []int{i}
	case "ex_nihilo", "lightning":
		targets = []int{i}
	case "amazing_grace", "god_salvation":
		targets = s.sgOrder(i)
	case "savage_assault", "archery_attack":
		targets = s.sgOrder(s.sgNext(i))
		targets = sgRemove(targets, i)
	case "collateral":
		if len(targets) != 2 || s.sgEquip(targets[0], "weapon") == 0 || !s.sgCanTarget(targets[0], targets[1], "slash") || targets[0] == i {
			return errors.New("选择持武器角色及其范围内的目标")
		}
	default:
		maxTargets := 1
		if kind == "iron_chain" {
			maxTargets = 2
		}
		if sgIsSlash(kind) && legality.sgWeapon(i) == "halberd" && len(ids) == 1 && len(p.Hand) == 1 && p.Hand[0] == ids[0] {
			maxTargets = 3
		}
		if len(targets) < 1 || len(targets) > maxTargets {
			return errors.New("请选择正确数量的目标")
		}
		seen := map[int]bool{}
		for _, t := range targets {
			if seen[t] || !legality.sgCanTarget(i, t, kind) {
				return errors.New("目标不符合此牌条件")
			}
			seen[t] = true
		}
	}
	// Multiple targets of the same card resolve in current-turn seat order.
	if kind != "collateral" && len(targets) > 1 {
		ordered := []int{}
		for _, target := range s.sgOrder(s.Turn) {
			if slices.Contains(targets, target) {
				ordered = append(ordered, target)
			}
		}
		targets = ordered
	}
	if kind == "lightning" && !s.sgCanTarget(i, i, kind) {
		return errors.New("判定区已有闪电")
	}
	if sgIsSlash(kind) && !forced && p.Used["slash"] > 0 && !s.sgHas(i, "paoxiao") && legality.sgWeapon(i) != "crossbow" {
		return errors.New("本阶段已经使用过杀")
	}
	if sgIsSlash(kind) {
		if !forced {
			p.Used["slash"]++
		}
		if i == s.Turn && g.InPlay {
			p.Used["keji_slash"]++
		}
	}
	if kind == "analeptic" {
		p.Used["analeptic"]++
	}
	drank := 0
	if sgIsSlash(kind) {
		drank = p.Drank
		p.Drank = 0
	}
	events := []SGEvent{}
	if kind == "amazing_grace" {
		events = append(events, SGEvent{Type: "grace_reveal", Actor: i})
	}
	for _, t := range targets {
		e := SGEvent{Type: "effect", Actor: i, Target: t, Kind: kind, Cards: ids, Amount: 1, Aux: -1, Nature: sgNature(kind)}
		if sgIsSlash(kind) {
			e.Type = "slash_start"
			e.Amount += drank
		} else if sgIsTrick(kind) && !sgIsDelayed(kind) {
			e.Type = "null_window"
			e.Step = i
		}
		if kind == "collateral" {
			e.Aux = targets[1]
			events = append(events, e)
			break
		}
		events = append(events, e)
	}
	events = append(events, SGEvent{Type: "cleanup", Cards: ids})
	if kind == "amazing_grace" {
		events = append(events, SGEvent{Type: "grace_cleanup"})
	}
	s.sgPush(events...)
	s.sgPay(i, ids)
	if sgIsTrick(kind) && !sgIsDelayed(kind) {
		s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "jizhi", Amount: 1})
	}
	targetNames := ""
	for _, t := range targets {
		targetNames += " " + s.sgName(t)
	}
	s.sgLog("%s 使用「%s」→%s", s.sgName(i), info.Name, targetNames)
	return nil
}
