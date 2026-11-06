package engine

// Original IDs, plant prices and recharge counters are bound to the sampled
// PC 1.0.0.1051 tables. Zombie purchase prices are explicit versus rules.
type Spec struct {
	Kind         string   `json:"kind"`
	Name         string   `json:"name"`
	Cost         int      `json:"cost"`
	Cooldown     int      `json:"cooldown"`
	HP           int      `json:"hp"`
	Armor        int      `json:"armor,omitempty"`
	Description  string   `json:"description"`
	OriginalID   int      `json:"originalId"`
	Target       string   `json:"target"`
	Layer        string   `json:"layer,omitempty"`
	UpgradeFrom  string   `json:"upgradeFrom,omitempty"`
	WaterOnly    bool     `json:"waterOnly,omitempty"`
	WaterAllowed bool     `json:"waterAllowed,omitempty"`
	RoofOnly     bool     `json:"roofOnly,omitempty"`
	Scenes       []string `json:"scenes,omitempty"`
	Mushroom     bool     `json:"mushroom,omitempty"`
	LaunchMS     int      `json:"launchMS,omitempty"`
	RequiresIce  bool     `json:"requiresIce,omitempty"`
}
type Catalog struct {
	Plants  []Spec `json:"plants"`
	Zombies []Spec `json:"zombies"`
}

var plantSpecs = []Spec{
	{Kind: "peashooter", Name: "豌豆射手", Description: "向本行前方发射豌豆，持续攻击迎面的僵尸。", Cost: 100, Cooldown: 7500, HP: 300, OriginalID: 0, Target: "cell", Layer: "body", LaunchMS: 1500},
	{Kind: "sunflower", Name: "向日葵", Description: "定期产出阳光，收益归种植者所有。", Cost: 50, Cooldown: 7500, HP: 300, OriginalID: 1, Target: "cell", Layer: "body", LaunchMS: 25000},
	{Kind: "cherrybomb", Name: "樱桃炸弹", Description: "短暂引信后炸伤附近三行的僵尸。", Cost: 150, Cooldown: 50000, HP: 300, OriginalID: 2, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "wallnut", Name: "坚果墙", Description: "用厚实的身体阻挡啃咬，为后排争取输出时间。", Cost: 50, Cooldown: 30000, HP: 4000, OriginalID: 3, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "potatomine", Name: "土豆雷", Description: "埋设后需要准备，成熟后炸伤靠近的地面僵尸。", Cost: 25, Cooldown: 30000, HP: 300, OriginalID: 4, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "snowpea", Name: "寒冰射手", Description: "冰豌豆造成伤害并减慢目标的移动和啃咬。", Cost: 175, Cooldown: 7500, HP: 300, OriginalID: 5, Target: "cell", Layer: "body", LaunchMS: 1500},
	{Kind: "chomper", Name: "大嘴花", Description: "吞下靠近的普通僵尸，消化期间无法再次进食。", Cost: 150, Cooldown: 7500, HP: 300, OriginalID: 6, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "repeater", Name: "双发射手", Description: "每轮连续发射两颗豌豆，集中攻击本行目标。", Cost: 200, Cooldown: 7500, HP: 300, OriginalID: 7, Target: "cell", Layer: "body", LaunchMS: 1500},
	{Kind: "puffshroom", Name: "小喷菇", Description: "喷射短程孢子，适合靠前布置；白天会睡眠。", Cost: 0, Cooldown: 7500, HP: 300, OriginalID: 8, Target: "cell", Layer: "body", LaunchMS: 1500, Mushroom: true},
	{Kind: "sunshroom", Name: "阳光菇", Description: "幼小时产出少量阳光，长大后每次产出增加。", Cost: 25, Cooldown: 7500, HP: 300, OriginalID: 9, Target: "cell", Layer: "body", LaunchMS: 25000, Mushroom: true},
	{Kind: "fumeshroom", Name: "大喷菇", Description: "向前喷出穿透气体，可同时伤害直线上的多个目标。", Cost: 75, Cooldown: 7500, HP: 300, OriginalID: 10, Target: "cell", Layer: "body", LaunchMS: 1500, Mushroom: true},
	{Kind: "gravebuster", Name: "墓碑吞噬者", Description: "吞噬目标墓碑，清出可种植的格子。", Cost: 75, Cooldown: 7500, HP: 300, OriginalID: 11, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "hypnoshroom", Name: "魅惑菇", Description: "被吃掉后令可魅惑僵尸转身反攻，巨人与载具不受影响。", Cost: 75, Cooldown: 30000, HP: 300, OriginalID: 12, Target: "cell", Layer: "body", LaunchMS: 0, Mushroom: true},
	{Kind: "scaredyshroom", Name: "胆小菇", Description: "远程喷射孢子，但僵尸靠近时会躲起来停止攻击。", Cost: 25, Cooldown: 7500, HP: 300, OriginalID: 13, Target: "cell", Layer: "body", LaunchMS: 1500, Mushroom: true},
	{Kind: "iceshroom", Name: "寒冰菇", Description: "冻结全场僵尸并留下减速，还能熄灭僵王火球。", Cost: 75, Cooldown: 50000, HP: 300, OriginalID: 14, Target: "cell", Layer: "body", LaunchMS: 0, Mushroom: true},
	{Kind: "doomshroom", Name: "毁灭菇", Description: "大范围爆炸后留下弹坑，该格暂时无法种植。", Cost: 125, Cooldown: 50000, HP: 300, OriginalID: 15, Target: "cell", Layer: "body", LaunchMS: 0, Mushroom: true},
	{Kind: "lilypad", Name: "睡莲", Description: "承托水面上的陆生植物，移除后上方陆生植物会落水。", Cost: 25, Cooldown: 7500, HP: 300, OriginalID: 16, Target: "cell", Layer: "support", LaunchMS: 0, WaterOnly: true},
	{Kind: "squash", Name: "窝瓜", Description: "扑向靠近的僵尸，压伤落点附近的目标。", Cost: 50, Cooldown: 30000, HP: 300, OriginalID: 17, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "threepeater", Name: "三线射手", Description: "同时向本行及相邻两行发射豌豆。", Cost: 325, Cooldown: 7500, HP: 300, OriginalID: 18, Target: "cell", Layer: "body", LaunchMS: 1500},
	{Kind: "tanglekelp", Name: "缠绕海草", Description: "抓住靠近的水路僵尸，将其拖入水中后消失。", Cost: 25, Cooldown: 30000, HP: 300, OriginalID: 19, Target: "cell", Layer: "body", LaunchMS: 0, WaterOnly: true},
	{Kind: "jalapeno", Name: "火爆辣椒", Description: "焚烧整行，可清除该行的冰道、梯子和冰球。", Cost: 125, Cooldown: 50000, HP: 300, OriginalID: 20, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "spikeweed", Name: "地刺", Description: "伤害踩过的地面僵尸，并刺破车辆轮胎。", Cost: 100, Cooldown: 7500, HP: 300, OriginalID: 21, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "torchwood", Name: "火炬树桩", Description: "点燃穿过的普通豌豆；冰豌豆经过时先融化。", Cost: 175, Cooldown: 7500, HP: 300, OriginalID: 22, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "tallnut", Name: "高坚果", Description: "阻挡前进，并拦住撑杆、海豚和跳跳僵尸的越障。", Cost: 125, Cooldown: 30000, HP: 8000, OriginalID: 23, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "seashroom", Name: "海蘑菇", Description: "从水面发射短程孢子，适合贴近来敌布置。", Cost: 0, Cooldown: 30000, HP: 300, OriginalID: 24, Target: "cell", Layer: "body", LaunchMS: 1500, WaterOnly: true, Mushroom: true},
	{Kind: "plantern", Name: "路灯花", Description: "照亮附近浓雾，揭示隐藏的僵尸与地形。", Cost: 25, Cooldown: 30000, HP: 300, OriginalID: 25, Target: "cell", Layer: "body", LaunchMS: 25000},
	{Kind: "cactus", Name: "仙人掌", Description: "发射针刺，遇到气球时升高攻击并将气球刺破。", Cost: 125, Cooldown: 7500, HP: 300, OriginalID: 26, Target: "cell", Layer: "body", LaunchMS: 1500},
	{Kind: "blover", Name: "三叶草", Description: "吹走气球僵尸，并暂时驱散整片浓雾。", Cost: 100, Cooldown: 7500, HP: 300, OriginalID: 27, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "splitpea", Name: "裂荚射手", Description: "前方单发、后方双发，可应对绕到背后的敌人。", Cost: 125, Cooldown: 7500, HP: 300, OriginalID: 28, Target: "cell", Layer: "body", LaunchMS: 1500},
	{Kind: "starfruit", Name: "杨桃", Description: "向五个方向发射星星，覆盖侧面和背后的目标。", Cost: 125, Cooldown: 7500, HP: 300, OriginalID: 29, Target: "cell", Layer: "body", LaunchMS: 1500},
	{Kind: "pumpkin", Name: "南瓜头", Description: "作为外壳吸收伤害，保护同格的植物。", Cost: 125, Cooldown: 30000, HP: 4000, OriginalID: 30, Target: "cell", Layer: "shell", LaunchMS: 0},
	{Kind: "magnetshroom", Name: "磁力菇", Description: "吸走附近僵尸的金属装备，也能移除已经架好的梯子。", Cost: 100, Cooldown: 7500, HP: 300, OriginalID: 31, Target: "cell", Layer: "body", LaunchMS: 0, Mushroom: true},
	{Kind: "cabbagepult", Name: "卷心菜投手", Description: "抛出卷心菜越过地形与正面盾牌，打击本行目标。", Cost: 100, Cooldown: 7500, HP: 300, OriginalID: 32, Target: "cell", Layer: "body", LaunchMS: 3000},
	{Kind: "flowerpot", Name: "花盆", Description: "为屋顶上的植物提供基座，移除会使上方植物失去支撑。", Cost: 25, Cooldown: 7500, HP: 300, OriginalID: 33, Target: "cell", Layer: "support", LaunchMS: 0},
	{Kind: "kernelpult", Name: "玉米投手", Description: "投出玉米粒，有时投出黄油使目标短暂停顿。", Cost: 100, Cooldown: 7500, HP: 300, OriginalID: 34, Target: "cell", Layer: "body", LaunchMS: 3000},
	{Kind: "coffeebean", Name: "咖啡豆", Description: "唤醒同格睡眠的蘑菇，自身不永久占用格位。", Cost: 75, Cooldown: 7500, HP: 300, OriginalID: 35, Target: "cell", Layer: "instant", LaunchMS: 0},
	{Kind: "garlic", Name: "大蒜", Description: "被啃咬时使僵尸改走相邻的可通行行。", Cost: 50, Cooldown: 7500, HP: 400, OriginalID: 36, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "umbrellaleaf", Name: "叶子保护伞", Description: "弹开附近的蹦极僵尸，并挡住投石车篮球。", Cost: 100, Cooldown: 7500, HP: 300, OriginalID: 37, Target: "cell", Layer: "body", LaunchMS: 0},
	{Kind: "marigold", Name: "金盏花", Description: "产出归本人所有的金币，收取后补充个人资源。", Cost: 50, Cooldown: 30000, HP: 300, OriginalID: 38, Target: "cell", Layer: "body", LaunchMS: 25000},
	{Kind: "melonpult", Name: "西瓜投手", Description: "投掷西瓜，伤害命中目标及邻行附近的僵尸。", Cost: 300, Cooldown: 7500, HP: 300, OriginalID: 39, Target: "cell", Layer: "body", LaunchMS: 3000},
	{Kind: "gatlingpea", Name: "机枪射手", Description: "每轮连续发射四颗豌豆，集中削弱本行目标。", Cost: 250, Cooldown: 50000, HP: 300, OriginalID: 40, Target: "cell", Layer: "body", LaunchMS: 1500, UpgradeFrom: "repeater"},
	{Kind: "twinsunflower", Name: "双子向日葵", Description: "每次产出双份阳光，收益仍归种植者所有。", Cost: 150, Cooldown: 50000, HP: 300, OriginalID: 41, Target: "cell", Layer: "body", LaunchMS: 25000, UpgradeFrom: "sunflower"},
	{Kind: "gloomshroom", Name: "忧郁菇", Description: "连续向周围喷出穿透气体，覆盖近处相邻行。", Cost: 150, Cooldown: 50000, HP: 300, OriginalID: 42, Target: "cell", Layer: "body", LaunchMS: 2000, UpgradeFrom: "fumeshroom", Mushroom: true},
	{Kind: "cattail", Name: "香蒲", Description: "追踪全场目标，优先用针刺击落气球。", Cost: 225, Cooldown: 50000, HP: 300, OriginalID: 43, Target: "cell", Layer: "body", LaunchMS: 1500, UpgradeFrom: "lilypad", WaterOnly: true},
	{Kind: "wintermelon", Name: "冰西瓜", Description: "投掷冰西瓜，对命中区域造成范围伤害与减速。", Cost: 200, Cooldown: 50000, HP: 300, OriginalID: 44, Target: "cell", Layer: "body", LaunchMS: 3000, UpgradeFrom: "melonpult"},
	{Kind: "goldmagnet", Name: "金磁铁", Description: "自动收取本人的多枚金币，不会拿走队友的收益。", Cost: 50, Cooldown: 50000, HP: 300, OriginalID: 45, Target: "cell", Layer: "body", LaunchMS: 0, UpgradeFrom: "magnetshroom"},
	{Kind: "spikerock", Name: "地刺王", Description: "连续刺伤经过的僵尸，并能承受多次碾压或重击。", Cost: 125, Cooldown: 50000, HP: 450, OriginalID: 46, Target: "cell", Layer: "body", LaunchMS: 0, UpgradeFrom: "spikeweed"},
	{Kind: "cobcannon", Name: "玉米加农炮", Description: "装填完成后点击炮身，再选择目标区域发射重炮。", Cost: 500, Cooldown: 50000, HP: 300, OriginalID: 47, Target: "cell", Layer: "body", LaunchMS: 6000, UpgradeFrom: "kernelpult"},
	{Kind: "imitater", Name: "模仿者", Description: "延迟变成所选普通植物，可与原卡分别使用冷却。", Cost: 0, Cooldown: 7500, HP: 300, OriginalID: 48, Target: "cell", Layer: "body", LaunchMS: 0},
}
var zombieSpecs = []Spec{
	{Kind: "zombie", Name: "普通僵尸", Description: "沿本行前进，接触植物后持续啃咬。", Cost: 50, Cooldown: 3000, HP: 270, Armor: 0, OriginalID: 0, Target: "row"},
	{Kind: "flag", Name: "旗帜僵尸", Description: "带旗快速推进，可作为同路进攻的先头。", Cost: 60, Cooldown: 4000, HP: 270, Armor: 0, OriginalID: 1, Target: "row"},
	{Kind: "conehead", Name: "路障僵尸", Description: "路障先承受伤害，脱落后身体继续推进。", Cost: 100, Cooldown: 6000, HP: 270, Armor: 370, OriginalID: 2, Target: "row"},
	{Kind: "polevaulter", Name: "撑杆僵尸", Description: "持杆时跃过首个可跨越目标，高坚果能拦住。", Cost: 125, Cooldown: 10000, HP: 500, Armor: 0, OriginalID: 3, Target: "row"},
	{Kind: "buckethead", Name: "铁桶僵尸", Description: "铁桶吸收大量伤害，磁力菇可以夺走铁桶。", Cost: 175, Cooldown: 12000, HP: 270, Armor: 1100, OriginalID: 4, Target: "row"},
	{Kind: "newspaper", Name: "读报僵尸", Description: "报纸破碎后进入狂暴，加快推进。", Cost: 100, Cooldown: 8000, HP: 270, Armor: 150, OriginalID: 5, Target: "row"},
	{Kind: "screendoor", Name: "铁栅门僵尸", Description: "铁栅门抵挡正面直射，喷雾和抛物线可以绕过。", Cost: 175, Cooldown: 12000, HP: 270, Armor: 1100, OriginalID: 6, Target: "row"},
	{Kind: "football", Name: "橄榄球僵尸", Description: "高速推进且有金属头盔，适合压迫薄弱防线。", Cost: 225, Cooldown: 18000, HP: 270, Armor: 1400, OriginalID: 7, Target: "row"},
	{Kind: "dancing", Name: "舞王僵尸", Description: "召唤周围伴舞，补充被消灭的伴舞位置。", Cost: 300, Cooldown: 25000, HP: 500, Armor: 0, OriginalID: 8, Target: "row"},
	{Kind: "backupdancer", Name: "伴舞僵尸", Description: "随舞王协同行进，也能独立推进和啃咬。", Cost: 60, Cooldown: 5000, HP: 270, Armor: 0, OriginalID: 9, Target: "row"},
	{Kind: "duckytube", Name: "鸭子救生圈僵尸", Description: "借助救生圈前进，接触植物后停下啃咬。", Cost: 75, Cooldown: 4000, HP: 270, Armor: 0, OriginalID: 10, Target: "row", WaterOnly: true},
	{Kind: "snorkel", Name: "潜水僵尸", Description: "潜水时躲过普通直射，靠近植物时浮出啃咬。", Cost: 150, Cooldown: 10000, HP: 270, Armor: 0, OriginalID: 11, Target: "row", WaterOnly: true},
	{Kind: "zomboni", Name: "冰车僵尸", Description: "碾压沿途植物并留下冰道，地刺能刺破轮胎。", Cost: 350, Cooldown: 30000, HP: 1350, Armor: 0, OriginalID: 12, Target: "row"},
	{Kind: "bobsled", Name: "雪橇僵尸小队", Description: "沿冰道快速滑行，雪橇损坏后队员改为步行。", Cost: 250, Cooldown: 20000, HP: 270, Armor: 300, OriginalID: 13, Target: "row", RequiresIce: true},
	{Kind: "dolphin", Name: "海豚骑士僵尸", Description: "骑海豚跃过首株植物，高坚果可以阻止跳跃。", Cost: 200, Cooldown: 18000, HP: 500, Armor: 0, OriginalID: 14, Target: "row", WaterOnly: true},
	{Kind: "jackinthebox", Name: "玩偶匣僵尸", Description: "行进一段时间后开盒爆炸，毁坏附近植物。", Cost: 150, Cooldown: 18000, HP: 500, Armor: 0, OriginalID: 15, Target: "row"},
	{Kind: "balloon", Name: "气球僵尸", Description: "飞越地面防线，针刺与三叶草可以应对。", Cost: 175, Cooldown: 12000, HP: 270, Armor: 0, OriginalID: 16, Target: "row", WaterAllowed: true},
	{Kind: "digger", Name: "矿工僵尸", Description: "潜地穿过防线，从左侧出土后回头攻击；磁力可令其提前出土。", Cost: 200, Cooldown: 18000, HP: 270, Armor: 100, OriginalID: 17, Target: "row"},
	{Kind: "pogo", Name: "跳跳僵尸", Description: "连续跳过普通植物，可被高坚果阻挡或磁力夺杆。", Cost: 200, Cooldown: 18000, HP: 500, Armor: 0, OriginalID: 18, Target: "row"},
	{Kind: "yeti", Name: "雪人僵尸", Description: "短暂向前推进后转身逃走，难以持续压线。", Cost: 300, Cooldown: 25000, HP: 1350, Armor: 0, OriginalID: 19, Target: "row"},
	{Kind: "bungee", Name: "蹦极僵尸", Description: "选择目标格空降偷取植物，附近保护伞可将其弹开。", Cost: 125, Cooldown: 15000, HP: 450, Armor: 0, OriginalID: 20, Target: "cell", WaterAllowed: true},
	{Kind: "ladder", Name: "扶梯僵尸", Description: "扛梯抵挡正面攻击，架梯后同伴可翻越该格。", Cost: 200, Cooldown: 18000, HP: 500, Armor: 500, OriginalID: 21, Target: "row"},
	{Kind: "catapult", Name: "投石车僵尸", Description: "远处投篮打击后排，弹药耗尽后向前碾压。", Cost: 350, Cooldown: 30000, HP: 850, Armor: 0, OriginalID: 22, Target: "row"},
	{Kind: "gargantuar", Name: "伽刚特尔", Description: "重击砸毁植物，受伤后向防线后方投掷小鬼。", Cost: 600, Cooldown: 60000, HP: 3000, Armor: 0, OriginalID: 23, Target: "row"},
	{Kind: "imp", Name: "小鬼僵尸", Description: "直接推进啃咬，巨人也会将小鬼抛到防线后方。", Cost: 50, Cooldown: 4000, HP: 270, Armor: 0, OriginalID: 24, Target: "row"},
	{Kind: "zomboss", Name: "僵王博士", Description: "轮换召唤、踩踏、砸车和冰火球，低头时可被集中攻击。", Cost: 2000, Cooldown: 120000, HP: 40000, Armor: 0, OriginalID: 25, Target: "row", Scenes: []string{"roofnight"}},
	{Kind: "peahead", Name: "豌豆僵尸", Description: "一边推进一边发射豌豆，攻击远处的植物。", Cost: 200, Cooldown: 18000, HP: 270, Armor: 0, OriginalID: 26, Target: "row"},
	{Kind: "wallnuthead", Name: "坚果僵尸", Description: "坚果头先吸收伤害，破碎后仍会继续啃咬。", Cost: 175, Cooldown: 15000, HP: 270, Armor: 1100, OriginalID: 27, Target: "row"},
	{Kind: "jalapenohead", Name: "火爆辣椒僵尸", Description: "延时引爆整行植物，自身也会消失。", Cost: 300, Cooldown: 30000, HP: 500, Armor: 0, OriginalID: 28, Target: "row"},
	{Kind: "gatlinghead", Name: "机枪僵尸", Description: "连续喷出四颗豌豆，远程压制同一行植物。", Cost: 400, Cooldown: 30000, HP: 270, Armor: 0, OriginalID: 29, Target: "row"},
	{Kind: "squashhead", Name: "窝瓜僵尸", Description: "靠近植物后起跳压毁目标，自身随之消失。", Cost: 175, Cooldown: 18000, HP: 270, Armor: 0, OriginalID: 30, Target: "row"},
	{Kind: "tallnuthead", Name: "高坚果僵尸", Description: "高坚果头提供厚重保护，破碎后继续向前。", Cost: 250, Cooldown: 20000, HP: 270, Armor: 2200, OriginalID: 31, Target: "row"},
	{Kind: "redeye", Name: "红眼伽刚特尔", Description: "承受更多伤害的巨人，同样会重击并投掷小鬼。", Cost: 900, Cooldown: 90000, HP: 6000, Armor: 0, OriginalID: 32, Target: "row"},
}

func GetCatalog() Catalog {
	return Catalog{Plants: append([]Spec{}, plantSpecs...), Zombies: append([]Spec{}, zombieSpecs...)}
}
func PlantSpec(kind string) (Spec, bool) {
	for _, s := range plantSpecs {
		if s.Kind == kind {
			return s, true
		}
	}
	return Spec{}, false
}
func ZombieSpec(kind string) (Spec, bool) {
	for _, s := range zombieSpecs {
		if s.Kind == kind {
			return s, true
		}
	}
	return Spec{}, false
}
