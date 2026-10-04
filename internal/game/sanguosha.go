package game

import (
	"errors"
	"fmt"
	"slices"
)

type SGPlayer struct {
	Hegemony      *SGHegemonyPlayer `json:"hegemony,omitempty"`
	Silenced      bool              `json:"silenced,omitempty"`
	HandSealed    bool              `json:"handSealed,omitempty"`
	Yiji          []int             `json:"yiji,omitempty"`
	Qianxun       []int             `json:"qianxun,omitempty"`
	PreviousHP    int               `json:"previousHP,omitempty"`
	PreviousHPSet bool              `json:"previousHPSet,omitempty"`
	JieLuoyi      bool              `json:"jieLuoyi,omitempty"`
	BaseKingdom   string            `json:"baseKingdom,omitempty"`
	Temporary     []string          `json:"temporary,omitempty"`
	Stars         []int             `json:"stars,omitempty"`
	Gale          []int             `json:"gale,omitempty"`
	Fog           []int             `json:"fog,omitempty"`
	ArmorOff      []int             `json:"armorOff,omitempty"`
	TurnKills     int               `json:"turnKills,omitempty"`
	Acquired      []string          `json:"acquired,omitempty"`
	SkillsLost    bool              `json:"skillsLost,omitempty"`
	Fields        []int             `json:"fields,omitempty"`
	Avatars       []string          `json:"avatars,omitempty"`
	Avatar        string            `json:"avatar,omitempty"`
	AvatarSkill   string            `json:"avatarSkill,omitempty"`
	AvatarKingdom string            `json:"avatarKingdom,omitempty"`
	Marks         map[string]int    `json:"marks,omitempty"`
	Flipped       bool              `json:"flipped,omitempty"`
	Buqu          []int             `json:"buqu,omitempty"`
	BuquActive    bool              `json:"buquActive,omitempty"`
	Chained       bool              `json:"chained,omitempty"`
	Drank         int               `json:"drank,omitempty"`
	Role          string            `json:"role"`
	General       string            `json:"general"`
	HP            int               `json:"hp"`
	MaxHP         int               `json:"maxHP"`
	Dead          bool              `json:"dead"`
	Hand          []int             `json:"hand"`
	Equip         []int             `json:"equip"`
	Judgment      []SGDelayed       `json:"judgment"`
	Used          map[string]int    `json:"used"`
	Choices       []string          `json:"choices"`
}
type SGDelayed struct {
	Card int    `json:"card"`
	Kind string `json:"kind"`
}

// Every suspended effect is plain data, including nested rescue / counterspell
// windows. No goroutine, callback or browser state is required to resume play.
type SGEvent struct {
	HegFactionCounter bool     `json:"hegFactionCounter,omitempty"`
	CounterDepth      int      `json:"counterDepth,omitempty"`
	TrickID           int      `json:"trickID,omitempty"`
	OriginalTargets   []int    `json:"originalTargets,omitempty"`
	TrickChecked      bool     `json:"trickChecked,omitempty"`
	Foreseen          bool     `json:"foreseen,omitempty"`
	SavageSource      int      `json:"savageSource,omitempty"`
	Color             int      `json:"color,omitempty"`
	DamageStage       int      `json:"damageStage,omitempty"`
	Transfer          bool     `json:"transfer,omitempty"`
	Near              bool     `json:"near,omitempty"`
	Nature            string   `json:"nature,omitempty"`
	Chain             bool     `json:"chain,omitempty"`
	Next              *SGEvent `json:"next,omitempty"`
	Type              string   `json:"type"`
	Actor             int      `json:"actor"`
	Target            int      `json:"target"`
	Kind              string   `json:"kind,omitempty"`
	Cards             []int    `json:"cards,omitempty"`
	Targets           []int    `json:"targets,omitempty"`
	Amount            int      `json:"amount,omitempty"`
	Step              int      `json:"step,omitempty"`
	Count             int      `json:"count,omitempty"`
	Aux               int      `json:"aux,omitempty"`
	Flag              bool     `json:"flag,omitempty"`
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
	Hegemony       *SGHegemony    `json:"hegemony,omitempty"`
	TrickSequence  int            `json:"trickSequence,omitempty"`
	RulesVersion   int            `json:"rulesVersion,omitempty"`
	ActivePhase    string         `json:"activePhase,omitempty"`
	TableKinds     map[int]string `json:"tableKinds,omitempty"`
	DiscardedOwn   int            `json:"discardedOwn,omitempty"`
	TurnSequence   int            `json:"turnSequence,omitempty"`
	SkipDiscard    bool           `json:"skipDiscard,omitempty"`
	ExtraTurns     []int          `json:"extraTurns,omitempty"`
	ResumeTurns    []int          `json:"resumeTurns,omitempty"`
	DiscardPhase   bool           `json:"discardPhase,omitempty"`
	DiscardedHand  []int          `json:"discardedHand,omitempty"`
	DiscardedOther []int          `json:"discardedOther,omitempty"`
	TableSuits     map[int]int    `json:"tableSuits,omitempty"`
	Bluffs         []SGBluff      `json:"bluffs,omitempty"`
	Virtual        *SGVirtual     `json:"-"`
	SkipJudge      bool           `json:"skipJudge,omitempty"`
	Options        SGOptions      `json:"options,omitempty"`
	SkipDraw       bool           `json:"skipDraw,omitempty"`
	Revealed       []int          `json:"revealed,omitempty"`
	Players        []SGPlayer     `json:"players"`
	Deck           []int          `json:"deck"`
	Discard        []int          `json:"discard"`
	Table          []int          `json:"table"`
	Queue          []SGEvent      `json:"queue"`
	Pending        *SGPrompt      `json:"pending,omitempty"`
	Sequence       int            `json:"sequence"`
	Grace          []int          `json:"grace"`
	Lord           int            `json:"lord"`
	Selecting      bool           `json:"selecting"`
	Selected       int            `json:"selected"`
	InPlay         bool           `json:"inPlay"`
	SkipPlay       bool           `json:"skipPlay"`
}

func (s *State) initSanguosha(n int) {
	roles := map[int][]string{4: {"lord", "loyalist", "rebel", "renegade"}, 5: {"lord", "loyalist", "rebel", "rebel", "renegade"}, 6: {"lord", "loyalist", "rebel", "rebel", "rebel", "renegade"}, 7: {"lord", "loyalist", "loyalist", "rebel", "rebel", "rebel", "renegade"}, 8: {"lord", "loyalist", "loyalist", "rebel", "rebel", "rebel", "rebel", "renegade"}}[n]
	roles = clone(roles)
	shuffle(roles)
	g := &Sanguosha{RulesVersion: 1, ActivePhase: "setup", Players: make([]SGPlayer, n), Selecting: true, Deck: []int{}, Discard: []int{}, Table: []int{}, Queue: []SGEvent{}}
	s.Sanguosha = g
	s.Phase = "sg_select"
	for _, c := range sgCards {
		g.Deck = append(g.Deck, c.ID)
	}
	shuffle(g.Deck)
	for i := range g.Players {
		g.Players[i] = SGPlayer{Role: roles[i], Hand: []int{}, Equip: []int{}, Judgment: []SGDelayed{}, Used: map[string]int{}}
		if roles[i] == "lord" {
			g.Lord = i
		}
	}
	s.sgLordChoices()
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
	if s.sgHegemony() {
		return s.sgHegName(i)
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
	if p.Dead || p.Silenced && !sgCompulsory(skill) {
		return false
	}
	if s.sgHegemony() {
		return slices.Contains(s.sgHegSkills(i, true), skill)
	}
	if sgLordSkill(skill) && p.Role != "lord" {
		return false
	}
	return !p.SkillsLost && (skill != "" && p.AvatarSkill == skill || slices.Contains(p.Acquired, skill) || slices.Contains(p.Temporary, skill) || slices.Contains(sgGeneral(p.General).Skills, skill))
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
	if kind == "general" || kind == "heg_generals" {
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
	s.sgGain(i, ids)
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
		if len(p.Buqu) > 0 {
			s.sgPush(SGEvent{Type: "buqu_trim", Actor: i})
		}
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
	if v := s.Sanguosha.Virtual; v != nil && v.Paid && v.Player == i && v.Card == id && slices.Contains(s.Sanguosha.Table, id) {
		return true
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
	if s.sgHegemony() && a == s.Turn && s.Sanguosha.ActivePhase != "inactive" && s.Sanguosha.Players[a].Used["heg_fenxun"] == b+1 {
		return 1
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
	if s.sgHas(a, "tuntian") {
		d -= len(s.Sanguosha.Players[a].Fields)
	}
	if s.sgEquip(b, "defense") != 0 {
		d++
	}
	if s.sgHas(b, "feiying") {
		d++
	}
	return max(1, d)
}
func (s *State) sgRange(i int) int {
	n := max(1, SGCardTypes[s.sgWeapon(i)].Range)
	if s.sgHegemony() {
		for _, who := range s.sgOrder(0) {
			if who != i && s.sgHegFriend(i, who) && s.sgWeapon(who) == "six_swords" {
				return n + 1 // Multiple allied swords do not stack.
			}
		}
	}
	return n
}
func (s *State) sgCanTarget(a, b int, kind string) bool {
	return s.sgCanTargetRange(a, b, kind, false)
}
func (s *State) sgCanTargetRange(a, b int, kind string, ignoreRange bool) bool {
	if !s.sgAlive(a) || !s.sgAlive(b) {
		return false
	}
	p := s.Sanguosha.Players[b]
	if s.sgHegemony() {
		if kind == "befriend_attacking" && (!s.sgHegShown(a) || !s.sgHegShown(b) || s.sgHegFriend(a, b)) {
			return false
		}
		if kind == "known_both" && len(p.Hand) == 0 && p.Hegemony.Shown[0] && p.Hegemony.Shown[1] {
			return false
		}
	}
	if a == b && kind != "peach" && kind != "analeptic" && kind != "lightning" && kind != "ex_nihilo" && kind != "iron_chain" && kind != "fire_attack" {
		return false
	}
	if (sgIsSlash(kind) || kind == "duel") && s.sgHas(b, "kongcheng") && len(p.Hand) == 0 {
		return false
	}
	if (kind == "snatch" || kind == "indulgence") && s.sgHas(b, "qianxun") {
		return false
	}
	if sgIsSlash(kind) && !ignoreRange && s.sgTianyi(a) != 1 && s.sgDistance(a, b) > s.sgRange(a) {
		return false
	}
	limit := 1
	if kind == "supply_shortage" && s.sgHas(a, "duanliang") {
		limit = 2
	}
	if (kind == "snatch" || kind == "supply_shortage") && !s.sgHas(a, "qicai") && !s.sgHas(a, "jie_qicai") && s.sgDistance(a, b) > limit {
		return false
	}
	if (kind == "snatch" || kind == "dismantlement") && len(p.Hand)+len(p.Equip)+len(p.Judgment) == 0 {
		return false
	}
	if kind == "dismantlement" && len(s.sgDiscardable(a, b, true)) == 0 {
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
	lost := false
	for _, id := range ids {
		lost = lost || slices.Contains(p.Hand, id) || slices.Contains(p.Equip, id)
		if slices.Contains(p.Equip, id) {
			equip++
			if sgCard(id).Kind == "silver_lion" && !s.sgGodArmorOff(i) {
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
	s.sgTuntianLoss(i, lost)
	s.sgEmptyHand(i, before-len(p.Hand))
	for range equip {
		if s.sgHas(i, "xiaoji") {
			s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "xiaoji", Amount: 2})
		}
	}
}
func (s *State) sgDiscard(i int, ids []int) {
	s.sgRecordDiscard(i, ids)
	s.sgSpent(i, ids)
}

// Used/responded cards and replaced equipment are not discard costs. Keeping
// the movement reason distinct prevents Guzheng from reclaiming responses.
func (s *State) sgSpent(i int, ids []int) {
	if v := s.Sanguosha.Virtual; v != nil && v.Paid && v.Player == i && len(ids) == 1 && ids[0] == v.Card {
		s.sgFinishCards(ids)
		return
	}
	s.sgLose(i, ids)
	s.Sanguosha.Discard = append(s.Sanguosha.Discard, ids...)
}
func (s *State) sgPay(i int, ids []int) {
	if v := s.Sanguosha.Virtual; v != nil && v.Paid && len(ids) == 1 && ids[0] == v.Card && slices.Contains(s.Sanguosha.Table, v.Card) {
		return
	}
	s.sgPlaceTable(i, ids)
	s.sgLose(i, ids)
}
func (s *State) sgPlaceTable(i int, ids []int) {
	g := s.Sanguosha
	if g.TableSuits == nil {
		g.TableSuits = map[int]int{}
	}
	if g.TableKinds == nil {
		g.TableKinds = map[int]string{}
	}
	for _, id := range ids {
		g.TableSuits[id] = s.sgCardFor(i, id).Suit
		g.TableKinds[id] = s.sgCardFor(i, id).Kind
	}
	g.Table = append(g.Table, ids...)
}
func (s *State) sgTakeTable(id int) {
	g := s.Sanguosha
	g.Table = sgRemove(g.Table, id)
	delete(g.TableSuits, id)
	delete(g.TableKinds, id)
}
func (s *State) sgFinishCards(ids []int) {
	g := s.Sanguosha
	for _, id := range ids {
		if slices.Contains(g.Table, id) {
			s.sgTakeTable(id)
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
	if !s.sgHandUsable(i, ids) {
		return "", errors.New("义绝：本回合不能使用或打出手牌")
	}
	if err := s.sgValidateCards(i, ids, -1, false); err != nil {
		return "", err
	}
	if skill == "longhun" {
		return s.sgGodLonghun(i, ids, desired)
	}
	if skill == "guhuo" {
		v := s.Sanguosha.Virtual
		if v == nil || v.Player != i || len(ids) != 1 || ids[0] != v.Card {
			return "", errors.New("蛊惑必须先经过质疑结算")
		}
		if desired != "" && desired != v.Kind && !(desired == "slash" && sgIsSlash(v.Kind)) {
			return "", errors.New("声明的牌型不能用于此响应")
		}
		return v.Kind, nil
	}
	if skill == "luanji" {
		if !s.sgHas(i, skill) || s.sgValidateCards(i, ids, 2, true) != nil || s.sgCardFor(i, ids[0]).Suit != s.sgCardFor(i, ids[1]).Suit || desired != "" && desired != "archery_attack" {
			return "", errors.New("乱击需要两张同花色手牌")
		}
		return "archery_attack", nil
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
	c := s.sgCardFor(i, ids[0])
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
		case "huoji", "kanpo", "lianhuan", "shuangxiong":
			if !s.sgOwn(i, c.ID, true) {
				return "", errors.New("此技能需要手牌")
			}
			switch skill {
			case "huoji":
				if !red {
					return "", errors.New("火计需要红色手牌")
				}
				kind = "fire_attack"
			case "kanpo":
				if red {
					return "", errors.New("看破需要黑色手牌")
				}
				kind = "nullification"
			case "lianhuan":
				if c.Suit != 2 {
					return "", errors.New("连环需要梅花手牌")
				}
				kind = "iron_chain"
			case "shuangxiong":
				mark := s.Sanguosha.Players[i].Used["shuangxiong"]
				if i != s.Turn || mark == 0 || (mark == 1) == red {
					return "", errors.New("双雄需要本回合与判定颜色不同的手牌")
				}
				kind = "duel"
			}
		case "duanliang":
			if red || sgIsTrick(kind) && SGCardTypes[kind].Slot == "" {
				return "", errors.New("断粮需要黑色基本牌或装备牌")
			}
			kind = "supply_shortage"
		case "jiuchi":
			if c.Suit != 0 || !s.sgOwn(i, c.ID, true) {
				return "", errors.New("酒池需要黑桃手牌")
			}
			kind = "analeptic"
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
		case "heg_duoshi":
			if !s.sgHegemony() || !red || !s.sgOwn(i, c.ID, true) || s.Sanguosha.Players[i].Used[skill] >= 4 {
				return "", errors.New("度势需要红色手牌，每阶段至多四次")
			}
			kind = "await_exhausted"
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
	if desired != "" && desired != kind && !(desired == "slash" && sgIsSlash(kind)) && !(s.sgHegemony() && desired == "nullification" && kind == "heg_nullification") {
		return "", errors.New("响应的牌型不正确")
	}
	return kind, nil
}
func (s *State) applySanguosha(i int, a Action) error {
	if !s.sgAlive(i) && !s.sgGodDeathResponse(i) {
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
		if a.Skill == "guhuo" && g.Virtual == nil && g.Pending.Kind != "huashen" {
			return s.sgStartGuhuo(i, a)
		}
		return s.sgRespond(i, a)
	}
	if g.Selecting || s.Turn != i || s.Phase != "sg_play" {
		return errors.New("尚未轮到你出牌")
	}
	if a.Type == "sg_end" {
		g.InPlay = false
		s.sgPush(s.sgEndPhase(i)...)
		return nil
	}
	if a.Type == "sg_skill" {
		return s.sgSkill(i, a)
	}
	if a.Type != "sg_play" {
		return errors.New("未知三国杀操作")
	}
	if a.Skill == "guhuo" && g.Virtual == nil {
		return s.sgStartGuhuo(i, a)
	}
	kind, err := s.sgAs(i, a.Cards, a.Skill, "")
	if err != nil {
		return err
	}
	if a.Skill == "heg_duoshi" {
		g.Players[i].Used[a.Skill]++
	}
	return s.sgUse(i, kind, a.Cards, a.Targets, false)
}
func (s *State) sgUse(i int, kind string, ids, targets []int, forced bool) error {
	g := s.Sanguosha
	p := &g.Players[i]
	info := SGCardTypes[kind]
	legality := clone(*s)
	legality.sgLose(i, ids)
	if sgIsSlash(kind) && s.sgTianyi(i) == -1 {
		return errors.New("天义拼点未赢，本回合不能使用杀")
	}
	if kind == "jink" || kind == "nullification" || kind == "heg_nullification" {
		return errors.New("此牌只能响应时使用")
	}
	if info.Slot != "" {
		if len(targets) != 0 {
			return errors.New("装备无需选择目标")
		}
		s.sgPush(SGEvent{Type: "equip", Actor: i, Kind: kind, Cards: ids})
		s.sgJieCardUsed(i, kind, ids)
		s.sgPay(i, ids)
		s.sgLog("%s 装备「%s」", s.sgName(i), info.Name)
		return nil
	}
	if (kind == "iron_chain" || kind == "known_both") && len(targets) == 0 {
		s.sgPush(SGEvent{Type: "draw", Actor: i, Amount: 1})
		s.sgSpent(i, ids)
		s.sgLog("%s 重铸%s，摸一张牌", s.sgName(i), info.Name)
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
	case "await_exhausted":
		if !s.sgHegemony() {
			return errors.New("此牌仅用于国战")
		}
		targets = nil
		for _, who := range s.sgOrder(i) {
			if s.sgHegFriend(i, who) {
				targets = append(targets, who)
			}
		}
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
		if sgIsSlash(kind) && legality.sgWeapon(i) == "halberd" && len(ids) == 1 && ((len(p.Hand) == 1 && p.Hand[0] == ids[0]) || (g.Virtual != nil && g.Virtual.Paid && len(p.Hand) == 0 && g.Virtual.Card == ids[0])) {
			maxTargets = 3
		}
		if sgIsSlash(kind) && s.sgTianyi(i) == 1 {
			maxTargets++
		}
		if len(targets) < 1 || len(targets) > maxTargets {
			return errors.New("请选择正确数量的目标")
		}
		seen := map[int]bool{}
		for _, t := range targets {
			if seen[t] || !legality.sgCanTargetRange(i, t, kind, (s.sgGodWushenRange(i, kind, ids) || i == s.Turn && p.Used["zhaxiang"] > 0 && s.sgCardColor(i, ids) == 1)) || s.sgWeimu(i, t, kind, ids) {
				return errors.New("目标不符合此牌条件")
			}
			seen[t] = true
		}
	}
	// Global tricks omit prohibited targets, while explicit targets reject above.
	if slices.Contains([]string{"amazing_grace", "god_salvation", "savage_assault", "archery_attack", "await_exhausted"}, kind) {
		targets = slices.DeleteFunc(targets, func(t int) bool { return s.sgWeimu(i, t, kind, ids) })
	}
	if kind == "lightning" && s.sgWeimu(i, i, kind, ids) {
		return errors.New("帷幕不能成为黑色锦囊目标")
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
	if sgIsSlash(kind) && !forced && p.Used["slash"] >= 1+max(0, s.sgTianyi(i))+p.Used["zhaxiang"] && !s.sgHas(i, "paoxiao") && legality.sgWeapon(i) != "crossbow" {
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
	original := clone(targets)
	if kind == "collateral" {
		original = original[:1]
	}
	trickID := 0
	if sgIsTrick(kind) {
		g.TrickSequence++
		trickID = g.TrickSequence
		if !sgIsDelayed(kind) && len(original) >= 2 {
			events = append(events, SGEvent{Type: "fenwei", Actor: i, Kind: kind, Targets: clone(original), TrickID: trickID})
		}
	}
	if kind == "duel" || sgIsSlash(kind) && s.sgCardColor(i, ids) == 1 {
		events = append(events, SGEvent{Type: "optional_draw", Actor: i, Kind: "jiang", Amount: 1})
		if kind == "duel" {
			for _, target := range targets {
				events = append(events, SGEvent{Type: "optional_draw", Actor: target, Kind: "jiang", Amount: 1})
			}
		}
	}
	if kind == "amazing_grace" {
		events = append(events, SGEvent{Type: "grace_reveal", Actor: i})
	}
	for _, t := range targets {
		e := SGEvent{TrickID: trickID, OriginalTargets: clone(original), Type: "effect", Actor: i, Target: t, Kind: kind, Cards: ids, Amount: 1, Aux: -1, Nature: sgNature(kind), Color: s.sgCardColor(i, ids)}
		if kind == "savage_assault" {
			for _, who := range s.sgOrder(s.Turn) {
				if who != i && s.sgHas(who, "huoshou") {
					e.SavageSource = who + 1
					break
				}
			}
		}
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
	if kind == "await_exhausted" {
		for _, target := range targets {
			events = append(events, SGEvent{Type: "heg_await_discard", Target: target, TrickID: trickID})
		}
		events = append(events, SGEvent{Type: "heg_await_cleanup", TrickID: trickID})
	}
	events = append(events, SGEvent{Type: "cleanup", Actor: i, Kind: kind, Cards: ids})
	if kind == "amazing_grace" {
		events = append(events, SGEvent{Type: "grace_cleanup"})
	}
	s.sgPush(events...)
	s.sgJieCardUsed(i, kind, ids)
	s.sgPay(i, ids)
	if sgIsTrick(kind) && !sgIsDelayed(kind) {
		s.sgPush(SGEvent{Type: "optional_draw", Actor: i, Kind: "jizhi", Amount: 1})
	}
	s.sgGodTrick(i, kind)
	s.sgHegJizhi(i, kind, ids)
	targetNames := ""
	for _, t := range targets {
		targetNames += " " + s.sgName(t)
	}
	s.sgLog("%s 使用「%s」→%s", s.sgName(i), info.Name, targetNames)
	return nil
}
