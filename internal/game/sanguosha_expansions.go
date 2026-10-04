package game

import (
	"errors"
	"slices"
)

// Saved with the game; omitted fields in pre-expansion snapshots mean classic.
type SGOptions struct {
	StandardVersion string   `json:"standardVersion,omitempty"`
	Mode            string   `json:"mode,omitempty"`
	Deck            string   `json:"deck,omitempty"`
	Packs           []string `json:"packs,omitempty"`
}

func NormalizeSGOptions(o SGOptions) (SGOptions, error) {
	if o.Mode == "hegemony" {
		if o.StandardVersion != "" || o.Deck != "" && o.Deck != "hegemony" {
			return o, errors.New("国战使用独立武将与牌堆，不能混入身份局版本")
		}
		for _, pack := range o.Packs {
			if pack != "hegemony" {
				return o, errors.New("国战使用60名基础国战武将，不能混入身份局武将包")
			}
		}
		return SGOptions{Mode: "hegemony", Deck: "hegemony", Packs: []string{"hegemony"}}, nil
	}
	if o.StandardVersion == "" {
		o.StandardVersion = "classic"
	}
	if o.StandardVersion != "classic" && o.StandardVersion != "breakthrough" {
		return o, errors.New("未知标准武将版本")
	}
	if o.Mode == "" {
		o.Mode = "identity"
	}
	if o.Deck == "" {
		o.Deck = "standard"
	}
	if len(o.Packs) == 0 {
		o.Packs = []string{"standard"}
	}
	if o.Mode != "identity" {
		return o, errors.New("未知三国杀模式")
	}
	if o.Deck != "standard" && o.Deck != "military" {
		return o, errors.New("未知三国杀牌堆")
	}
	for _, pack := range o.Packs {
		if pack != "standard" && pack != "wind" && pack != "fire" && pack != "thicket" && pack != "mountain" && pack != "god" {
			return o, errors.New("该武将包尚未开放")
		}
	}
	requested := o.Packs
	o.Packs = []string{"standard"}
	for _, pack := range []string{"wind", "fire", "thicket", "mountain", "god"} {
		if slices.Contains(requested, pack) {
			o.Packs = append(o.Packs, pack)
		}
	}
	return o, nil
}

func NewSanguosha(n int, options SGOptions) (*State, error) {
	o, err := NormalizeSGOptions(options)
	if err != nil {
		return nil, err
	}
	s, err := New("sanguosha", n)
	if err != nil {
		return nil, err
	}
	if o.Mode == "hegemony" {
		s.initSGHegemony(n)
		return s, nil
	}
	s.Sanguosha.Options = o
	s.sgLordChoices()
	if o.Deck == "military" {
		for _, c := range sgMilitaryCards {
			s.Sanguosha.Deck = append(s.Sanguosha.Deck, c.ID)
		}
		shuffle(s.Sanguosha.Deck)
	}
	return s, nil
}

func (g *Sanguosha) cardCatalog() []SGCard {
	if g.Hegemony != nil {
		return sgHegemonyCards
	}
	if g.Options.Deck != "military" {
		return sgCards
	}
	return append(append([]SGCard{}, sgCards...), sgMilitaryCards...)
}

func sgIsSlash(kind string) bool {
	return kind == "slash" || kind == "fire_slash" || kind == "thunder_slash"
}
func sgIsDelayed(kind string) bool {
	return kind == "indulgence" || kind == "lightning" || kind == "supply_shortage"
}
func sgIsTrick(kind string) bool {
	return SGCardTypes[kind].Slot == "" && !sgIsSlash(kind) &&
		!slices.Contains([]string{"jink", "peach", "analeptic"}, kind)
}
func sgNature(kind string) string {
	switch kind {
	case "fire_slash", "fire_attack":
		return "fire"
	case "thunder_slash", "lightning":
		return "thunder"
	}
	return ""
}

// The first 108 IDs never change: running classic matches retain their cards.
// Source: pinned QSanguosha-v2 ManeuveringPackage (52 physical cards).
var sgMilitaryCards = func() []SGCard {
	rows := [][]string{
		{"guding_blade", "vine", "analeptic", "thunder_slash", "thunder_slash", "thunder_slash", "thunder_slash", "thunder_slash", "analeptic", "supply_shortage", "iron_chain", "iron_chain", "nullification"},
		{"nullification", "fire_attack", "fire_attack", "fire_slash", "peach", "peach", "fire_slash", "jink", "jink", "fire_slash", "jink", "jink", "nullification"},
		{"silver_lion", "vine", "analeptic", "supply_shortage", "thunder_slash", "thunder_slash", "thunder_slash", "thunder_slash", "analeptic", "iron_chain", "iron_chain", "iron_chain", "iron_chain"},
		{"fan", "peach", "peach", "fire_slash", "fire_slash", "jink", "jink", "jink", "analeptic", "jink", "jink", "fire_attack", "hualiu"},
	}
	var cards []SGCard
	for suit, row := range rows {
		for rank, kind := range row {
			cards = append(cards, SGCard{ID: 109 + len(cards), Kind: kind, Suit: suit, Rank: rank + 1})
		}
	}
	return cards
}()

func init() {
	for k, v := range map[string]SGCardInfo{
		"fire_slash":      {"火杀", "杀：造成1点火焰伤害。与普通杀、雷杀共用每回合使用次数。", "", 0},
		"thunder_slash":   {"雷杀", "杀：造成1点雷电伤害。与普通杀、火杀共用每回合使用次数。", "", 0},
		"analeptic":       {"酒", "出牌阶段限一次：本回合下一张杀伤害+1；自己濒死时可使用酒回复1点体力，救命不计次数。", "", 0},
		"fire_attack":     {"火攻", "一名有手牌的角色展示一张手牌；你可弃置一张相同花色的手牌，对其造成1点火焰伤害。", "", 0},
		"iron_chain":      {"铁索连环", "选择一至两名角色，分别横置或重置。横置角色受到属性伤害后，将伤害传导给其他横置角色并重置。也可不选目标重铸：弃置此牌并摸一张牌。", "", 0},
		"supply_shortage": {"兵粮寸断", "置于距离1以内其他角色的判定区；判定不为梅花，跳过其摸牌阶段。", "", 0},
		"fan":             {"朱雀羽扇", "可以将一张普通杀当火杀使用。", "weapon", 4},
		"guding_blade":    {"古锭刀", "杀对没有手牌的目标造成的伤害+1（不重复增加连环传导的伤害）。", "weapon", 2},
		"vine":            {"藤甲", "普通杀、南蛮入侵和万箭齐发对你无效；受到的火焰伤害+1。", "armor", 0},
		"silver_lion":     {"白银狮子", "每次受到的伤害至多1点；失去装备区里的此牌后，回复1点体力。", "armor", 0},
		"hualiu":          {"骅骝", "其他角色计算与你的距离+1。", "defense", 0},
	} {
		SGCardTypes[k] = v
	}
}

func (s *State) sgIgnoreArmor(e SGEvent) bool {
	return s.sgGodArmorOff(e.Target) || sgIsSlash(e.Kind) && !e.Chain && !e.Transfer && s.sgWeapon(e.Actor) == "qinggang_sword"
}
func (s *State) sgSlashImmune(e SGEvent) bool {
	if !sgIsSlash(e.Kind) || s.sgIgnoreArmor(e) {
		return false
	}
	armor := s.sgArmor(e.Target)
	if armor == "vine" && e.Nature == "" && e.Kind == "slash" {
		s.sgLog("%s 的藤甲令普通杀无效", s.sgName(e.Target))
		return true
	}
	if armor == "renwang_shield" && e.Color == 2 {
		s.sgLog("%s 的仁王盾令黑色杀无效", s.sgName(e.Target))
		return true
	}
	if armor == "renwang_shield" && e.Color == 0 && len(e.Cards) > 0 {
		for _, id := range e.Cards {
			if c := sgCard(id); c.Suit == 1 || c.Suit == 3 {
				return false
			}
		}
		s.sgLog("%s 的仁王盾令黑色杀无效", s.sgName(e.Target))
		return true
	}
	return false
}

func (s *State) sgDamage(e SGEvent) {
	g := s.Sanguosha
	if !s.sgAlive(e.Target) || e.Amount <= 0 {
		return
	}
	if e.SavageSource > 0 {
		e.Actor = e.SavageSource - 1
		if !s.sgAlive(e.Actor) {
			e.Actor = -1
		}
	}
	if e.Nature == "" {
		e.Nature = sgNature(e.Kind)
	}
	amount := e.Amount
	if e.DamageStage == 0 && !e.Chain && !e.Transfer && s.sgAlive(e.Actor) {
		if (e.Actor == s.Turn && g.Players[e.Actor].Used["luoyi"] > 0 || g.Players[e.Actor].JieLuoyi) && (sgIsSlash(e.Kind) || e.Kind == "duel") {
			amount++
		}
		if sgIsSlash(e.Kind) && s.sgWeapon(e.Actor) == "guding_blade" && len(g.Players[e.Target].Hand) == 0 {
			amount++
		}
	}
	e.Amount = amount
	if e.DamageStage == 0 {
		e.DamageStage = 1
	}
	if !e.Foreseen {
		e.Foreseen = true
		if s.sgGodForeseen(&e) {
			return
		}
		amount = e.Amount
	}
	if s.sgWindDamage(e) {
		return
	}
	if s.sgHegDamage(&e) {
		return
	}
	amount = e.Amount
	if !s.sgIgnoreArmor(e) {
		switch s.sgArmor(e.Target) {
		case "vine":
			if e.Nature == "fire" {
				amount++
			}
		case "silver_lion":
			amount = min(1, amount)
		}
	}
	p := &g.Players[e.Target]
	e.Near = s.sgAlive(e.Actor) && s.sgDistance(e.Actor, e.Target) <= 1
	s.sgGodBeforeDamage(e.Target, e.Actor, amount)
	p.HP -= amount
	e.Amount = amount
	e.Type = "hurt"
	dealt := e
	dealt.Type = "damage_dealt"
	es := []SGEvent{dealt, e}
	if e.Transfer {
		es = append(es, SGEvent{Type: "tianxiang_draw", Target: e.Target})
	}
	if p.Chained && e.Nature != "" {
		p.Chained = false
		if !e.Chain {
			for _, target := range s.sgOrder(s.Turn) {
				if target != e.Target && g.Players[target].Chained {
					spread := e
					spread.Type = "chain_damage"
					spread.Target = target
					spread.Chain = true
					spread.Transfer = false
					spread.DamageStage = 1
					spread.Foreseen = false
					spread.HegDamageChecked = false
					es = append(es, spread)
				}
			}
		}
	}
	label := SGCardTypes[e.Kind].Name
	if label == "" {
		label = SGSkills[e.Kind].Name
	}
	if label == "" {
		label = "技能"
	}
	nature := map[string]string{"": "", "fire": "火焰", "thunder": "雷电"}[e.Nature]
	if e.Chain {
		label += "·连环传导"
	}
	s.sgLog("%s 对 %s 造成 %d 点%s伤害（%s）", s.sgName(e.Actor), s.sgName(e.Target), amount, nature, label)
	s.sgPush(es...)
	if p.HP <= 0 {
		s.sgEnterDying(SGEvent{Actor: e.Actor, Target: e.Target, Step: s.Turn})
	}
}
