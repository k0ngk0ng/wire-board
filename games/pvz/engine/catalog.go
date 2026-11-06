package engine

// Plant costs, refresh counters and HP are grounded in the original PC binary
// PlantDefinition table / PlantInitialize (REA evidence in research report).
// Zombie purchase prices, spawn cooldowns and team economy are versus additions.
type Spec struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Cost        int    `json:"cost"`
	Cooldown    int    `json:"cooldown"`
	HP          int    `json:"hp"`
	Armor       int    `json:"armor,omitempty"`
	Description string `json:"description"`
}
type Catalog struct {
	Plants  []Spec `json:"plants"`
	Zombies []Spec `json:"zombies"`
}

var plantSpecs = []Spec{
	{"sunflower", "向日葵", 50, 7500, 300, 0, "每隔一段时间生产 25 阳光，归种植者所有"},
	{"peashooter", "豌豆射手", 100, 7500, 300, 0, "向本行前方发射豌豆"},
	{"repeater", "双发射手", 200, 7500, 300, 0, "连续发射两颗豌豆"},
	{"snowpea", "寒冰射手", 175, 7500, 300, 0, "冰豌豆伤害并减慢僵尸"},
	{"wallnut", "坚果墙", 50, 30000, 4000, 0, "高生命的防线，撑杆僵尸可以越过"},
	{"potatomine", "土豆地雷", 25, 30000, 300, 0, "埋设后需要准备，成熟后引爆同格僵尸"},
	{"cherrybomb", "樱桃炸弹", 150, 50000, 300, 0, "短暂引信后炸伤附近三行的僵尸"},
	{"tallnut", "高坚果", 125, 30000, 8000, 0, "坚固的高墙，可以阻挡撑杆跳跃"},
}
var zombieSpecs = []Spec{
	{"zombie", "普通僵尸", 50, 3000, 270, 0, "稳定前进，接触植物后啃咬"},
	{"conehead", "路障僵尸", 100, 6000, 270, 370, "路障先吸收伤害"},
	{"buckethead", "铁桶僵尸", 175, 12000, 270, 1100, "铁桶提供厚重护甲"},
	{"newspaper", "读报僵尸", 100, 8000, 270, 150, "报纸被打坏后加速"},
	{"polevaulter", "撑杆僵尸", 125, 10000, 500, 0, "可以越过一株普通植物，高坚果能阻挡"},
	{"football", "橄榄球僵尸", 225, 18000, 270, 1400, "移动快，头盔坚固"},
}

func GetCatalog() Catalog {
	return Catalog{Plants: append([]Spec{}, plantSpecs...), Zombies: append([]Spec{}, zombieSpecs...)}
}
func PlantSpec(kind string) (Spec, bool) {
	for _, c := range plantSpecs {
		if c.Kind == kind {
			return c, true
		}
	}
	return Spec{}, false
}
func ZombieSpec(kind string) (Spec, bool) {
	for _, c := range zombieSpecs {
		if c.Kind == kind {
			return c, true
		}
	}
	return Spec{}, false
}
