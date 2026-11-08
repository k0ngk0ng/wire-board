package stoneage

import (
	_ "embed"
	"encoding/json"
	"slices"
	"sync"
)

// Rule data and URL metadata only; no client image or audio payloads.
//
//go:embed catalog_source.json
var sourceJSON []byte
var catalogOnce sync.Once
var sourceCatalog struct {
	Pets      []PetDefinition `json:"pets"`
	Rides     []PetDefinition `json:"rides"`
	Equipment []Equipment     `json:"equipment"`
}

var allSkills = []Skill{
	{1, "攻击", "attack", "普通攻击一个目标。", 100, 1, "", 0},
	{2, "防御", "guard", "降低本回合受到的物理伤害。", 0, 0, "", 0},
	{3, "破除防御", "break", "对防御目标造成额外伤害，对未防御目标威力降低。", 65, 1, "", 0},
	{10, "连续攻击", "combo", "连续攻击两次，每次威力降低。", 60, 2, "", 0},
	{11, "三段攻击", "combo", "连续攻击三次，每次威力降低。", 45, 3, "", 0},
	{20, "忠犬", "guardian", "守护主人，代替主人承受一次攻击，并攻击敌人。", 80, 1, "", 0},
	{30, "突击", "charge", "提升攻击威力，出招后防御降低。", 190, 1, "", 0},
	{31, "双重突击", "charge", "连续突击两次，出招后防御降低。", 110, 2, "", 0},
	{40, "一击必杀", "mighty", "两倍威力攻击，命中率降低。", 200, 1, "", 0},
	{50, "背水之战其之１", "balance", "本回合攻击提升 25%，防御降低 35%。", 125, 1, "", 0},
	{51, "背水之战其之２", "balance", "本回合攻击提升 45%，防御降低 55%。", 145, 1, "", 0},
	{60, "毒攻击", "poison", "降低威力，命中后有机会使目标中毒。", 70, 1, "poison", 45},
	{61, "猛毒攻击", "poison", "降低威力，命中后有机会使目标持续中毒。", 50, 1, "poison", 70},
	{80, "石化攻击", "stone", "降低威力，命中后有机会使目标石化。", 70, 1, "stone", 35},
	{90, "混乱攻击", "confusion", "降低威力，命中后有机会使目标混乱。", 70, 1, "confusion", 40},
	{100, "泥醉攻击", "drunk", "降低威力，命中后有机会使目标敏捷下降。", 70, 1, "drunk", 50},
	{110, "催眠攻击", "sleep", "降低威力，命中后有机会使目标睡眠；受攻击苏醒。", 70, 1, "sleep", 40},
	{150, "不防守战法", "counter", "放弃防御，提升回避和反击概率。", 100, 1, "", 0},
	{210, "落马术", "dismount", "降低威力，命中骑乘人物后有机会使其落马。", 70, 1, "", 50},
}
var allMagic = []Magic{
	{10, "治愈的精灵", "heal", 18, 180, false, ""},
	{20, "滋润的精灵", "heal", 32, 90, true, ""},
	{30, "净化的精灵", "cure", 16, 0, false, ""},
	{40, "守护的精灵", "shield", 22, 2, false, ""},
	{50, "疾风的精灵", "haste", 22, 2, false, ""},
	{60, "火焰的精灵", "damage", 28, 120, false, "fire"},
	{70, "冰冻的精灵", "damage", 28, 120, false, "water"},
	{80, "大地的精灵", "damage", 28, 120, false, "earth"},
	{90, "风暴的精灵", "damage", 28, 120, false, "wind"},
}

func catalog() {
	catalogOnce.Do(func() {
		if e := json.Unmarshal(sourceJSON, &sourceCatalog); e != nil {
			panic(e)
		}
		for i := range sourceCatalog.Equipment {
			g := &sourceCatalog.Equipment[i]
			v := g.Variant
			switch g.Slot {
			case "weapon":
				g.Attack = 35 + v*5
				g.Speed = 5 - v*5
				switch g.WeaponType {
				case 1:
					g.Attack += 20
					g.Speed -= 15
				case 3:
					g.Attack += 8
				case 0:
					g.Attack -= 15
				case 4, 17, 19:
					g.Attack -= 15
				case 18:
					g.Attack += 5
				}
				magic := allMagic[(g.WeaponType+v+9)%len(allMagic)]
				g.Magic = &magic
			case "hat":
				g.Defense = 18 + v%4*4
				g.HP = 35 + (3-v%4)*15
				g.Speed = v % 3 * 3
				if v%3 == 0 {
					g.Resist = []string{"poison", "stone", "sleep"}[v/3%3]
				}
			case "clothes":
				g.Defense = 30 + v%4*7
				g.Speed = 12 - v%4*7
				g.HP = 40 + v%2*30
				magic := allMagic[v%len(allMagic)]
				g.Magic = &magic
			case "accessory":
				switch v % 6 {
				case 0:
					g.Attack = 12
					g.Defense = -4
				case 1:
					g.Defense = 12
					g.Speed = -4
				case 2:
					g.Speed = 12
					g.Attack = -4
				case 3:
					g.HP = 70
				case 4:
					g.MP = 45
				case 5:
					g.Resist = []string{"poison", "stone", "confusion"}[v/6%3]
				}
				if v%3 == 0 {
					magic := allMagic[(v/3+2)%len(allMagic)]
					g.Magic = &magic
				}
			}
		}
	})
}

// SplitMix64: the complete stream state is persisted and never exposed to a
// client or bot. Rejected transactions cannot advance the original snapshot.
func (s *State) randomN(n int) int {
	s.Random += 0x9e3779b97f4a7c15
	z := s.Random
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	if n <= 1 {
		return 0
	}
	return int(z % uint64(n))
}
func sample[T any](s *State, values []T, n int) []T {
	out := append([]T(nil), values...)
	for i := len(out) - 1; i > 0; i-- {
		j := s.randomN(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	if n < len(out) {
		out = out[:n]
	}
	return out
}
func (s *State) makePool() {
	catalog()
	s.Pool.Pets = sample(s, sourceCatalog.Pets, 20)
	s.Pool.Rides = sample(s, sourceCatalog.Rides, 6)
	for i := range s.Pool.Pets {
		s.Pool.Pets[i].Loyalty = s.randomN(101)
	}
	for i := range s.Pool.Rides {
		s.Pool.Rides[i].Loyalty = 100
	}
	// Attack, guard, break and dismount are guaranteed; draw ten additional
	// skills without replacement, keeping both control and direct damage.
	s.Pool.Skills = []Skill{allSkills[0], allSkills[1], allSkills[2], allSkills[len(allSkills)-1]}
	rest := append([]Skill(nil), allSkills[3:len(allSkills)-1]...)
	s.Pool.Skills = append(s.Pool.Skills, sample(s, rest, 10)...)
	for _, slot := range []string{"hat", "weapon", "clothes", "accessory"} {
		rows := []Equipment{}
		for _, g := range sourceCatalog.Equipment {
			if g.Slot == slot {
				rows = append(rows, g)
			}
		}
		n := 4
		if slot == "accessory" {
			n = 6
		}
		switch slot {
		case "hat":
			s.Pool.Hats = sample(s, rows, n)
		case "clothes":
			s.Pool.Clothes = sample(s, rows, n)
		case "accessory":
			s.Pool.Accessories = sample(s, rows, n)
		case "weapon":
			// Six different weapon families, never six versions of an axe.
			kinds := sample(s, []int{0, 1, 2, 3, 4, 17, 18, 19}, 6)
			for _, k := range kinds {
				group := []Equipment{}
				for _, g := range rows {
					if g.WeaponType == k {
						group = append(group, g)
					}
				}
				s.Pool.Weapons = append(s.Pool.Weapons, group[s.randomN(len(group))])
			}
		}
	}
	icon := "bitmaps/bitmap_9136.png"
	// These are arena rations with explicit effects and finite quantities.
	// Original food icons are shared; effects are not inferred from image IDs.
	s.Pool.HPItems = sample(s, []Item{{10001, "鲜肉料理", "hp", "回复单体 220 点生命。", icon, 220, false}, {10002, "烤鱼料理", "hp", "回复单体 260 点生命。", icon, 260, false}, {10003, "营养餐", "hp", "回复单体 200 点生命。", icon, 200, false}}, 2)
	s.Pool.MPItems = sample(s, []Item{{10011, "清泉水", "mp", "回复单体 60 点气力。", icon, 60, false}, {10012, "果汁", "mp", "回复单体 75 点气力。", icon, 75, false}, {10013, "椰子汁", "mp", "回复单体 65 点气力。", icon, 65, false}}, 2)
	s.Pool.CureItems = sample(s, []Item{{10021, "净化药", "cure", "解除单体中毒、石化、睡眠、混乱与酒醉。", icon, 0, false}, {10022, "清醒药", "cure", "解除单体所有异常状态。", icon, 0, false}, {10023, "草药包", "cure", "解除单体所有异常状态并回复 30 点生命。", icon, 30, false}}, 2)
	s.Pool.Tactics = sample(s, []Item{{10031, "守护石", "shield", "使一名队友两回合受到的伤害降低。", icon, 2, false}, {10032, "疾风果", "haste", "使一名队友两回合敏捷提升。", icon, 2, false}, {10033, "勇气果", "rage", "使一名队友两回合攻击提升。", icon, 2, false}, {10034, "驱散粉", "dispel", "清除一名对手的临时增益。", icon, 0, false}, {10035, "抗性药", "immune", "使一名队友两回合免疫新的异常状态。", icon, 2, false}, {10036, "急救餐", "hp", "回复单体 150 点生命。", icon, 150, false}}, 4)
}
func petByID(rows []PetDefinition, id int) (PetDefinition, bool) {
	for _, p := range rows {
		if p.ID == id {
			return p, true
		}
	}
	return PetDefinition{}, false
}
func gearByID(rows []Equipment, id int) (Equipment, bool) {
	for _, g := range rows {
		if g.ID == id {
			return g, true
		}
	}
	return Equipment{}, false
}
func itemByID(rows []Item, id int) (Item, bool) {
	for _, g := range rows {
		if g.ID == id {
			return g, true
		}
	}
	return Item{}, false
}
func skillByID(rows []Skill, id int) (Skill, bool) {
	for _, g := range rows {
		if g.ID == id {
			return g, true
		}
	}
	return Skill{}, false
}
func (p Pool) gear(l Loadout) []Equipment {
	out := []Equipment{}
	for _, entry := range []struct {
		rows []Equipment
		id   int
	}{{p.Hats, l.Hat}, {p.Weapons, l.Weapon}, {p.Clothes, l.Clothes}} {
		g, _ := gearByID(entry.rows, entry.id)
		out = append(out, g)
	}
	for _, id := range l.Accessories {
		g, _ := gearByID(p.Accessories, id)
		out = append(out, g)
	}
	return out
}
func familyUsed(ids []int, rows []Skill) bool {
	seen := map[string]bool{}
	for _, id := range ids {
		g, ok := skillByID(rows, id)
		if !ok || seen[g.Family] {
			return true
		}
		seen[g.Family] = true
	}
	return false
}
func unique(ids []int) bool {
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}
func hasSkill(ids []int, id int) bool { return slices.Contains(ids, id) }
