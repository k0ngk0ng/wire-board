package game

// The classic 25-general identity ruleset. IDs deliberately exclude revised,
// boundary-break and expansion skills even when upstream art shares a name.
type SGGeneral struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Kingdom string   `json:"kingdom"`
	HP      int      `json:"hp"`
	Female  bool     `json:"female"`
	Skills  []string `json:"skills"`
}
type SGSkill struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

var SGSkills = map[string]SGSkill{
	"jianxiong": {"奸雄", "受到伤害后，可以获得造成此次伤害的牌。"},
	"hujia":     {"护驾", "主公技：需要闪时，可请魏势力角色依次替你打出闪。"},
	"fankui":    {"反馈", "受到伤害后，可以获得伤害来源的一张牌。"},
	"guicai":    {"鬼才", "判定牌生效前，可以打出一张手牌代替判定牌。"},
	"ganglie":   {"刚烈", "受到伤害后可判定：若不是红桃，伤害来源弃两张手牌或受到你造成的1点伤害。"},
	"tuxi":      {"突袭", "摸牌阶段可放弃摸牌，改为获得至多两名其他角色各一张手牌。"},
	"luoyi":     {"裸衣", "摸牌阶段可少摸一张牌，本回合杀和决斗造成的伤害+1。"},
	"tiandu":    {"天妒", "你的判定牌生效后，可以获得该牌。"},
	"yiji":      {"遗计", "每受到1点伤害，可观看牌堆顶两张牌，并任意分配给角色。"},
	"qingguo":   {"倾国", "可以将黑色手牌当闪使用或打出。"},
	"luoshen":   {"洛神", "准备阶段可判定：黑色则获得该牌，并可重复此过程；红色则结束。"},
	"rende":     {"仁德", "出牌阶段可将手牌交给其他角色；本阶段给牌首次达到两张时回复1点体力。"},
	"jijiang":   {"激将", "主公技：需要杀时，可请蜀势力角色依次替你打出杀。"},
	"wusheng":   {"武圣", "可以将红色牌当杀使用或打出。"},
	"paoxiao":   {"咆哮", "出牌阶段使用杀没有次数限制。"},
	"guanxing":  {"观星", "准备阶段可观看牌堆顶X张牌，任意排列置于牌堆顶或底（X为存活人数，最多5）。"},
	"kongcheng": {"空城", "没有手牌时，不能成为杀或决斗的目标。"},
	"longdan":   {"龙胆", "可以将杀当闪、闪当杀使用或打出。"},
	"mashu":     {"马术", "计算你与其他角色的距离时−1。"},
	"tieji":     {"铁骑", "使用杀指定目标后可判定：红色则该目标不能使用闪。"},
	"jizhi":     {"集智", "使用非延时锦囊时，可以摸一张牌。"},
	"qicai":     {"奇才", "使用锦囊牌没有距离限制。"},
	"zhiheng":   {"制衡", "出牌阶段限一次，可弃置任意数量的牌，摸等量的牌。"},
	"jiuyuan":   {"救援", "主公技：濒死时，其他吴势力角色对你使用桃额外回复1点体力。"},
	"qixi":      {"奇袭", "可以将黑色牌当过河拆桥使用。"},
	"keji":      {"克己", "本回合出牌阶段没有使用或打出杀，可以跳过弃牌阶段。"},
	"kurou":     {"苦肉", "出牌阶段可以失去1点体力，然后摸两张牌。"},
	"yingzi":    {"英姿", "摸牌阶段可以额外摸一张牌。"},
	"fanjian":   {"反间", "出牌阶段限一次：一名其他角色猜花色并随机获得你的一张手牌；猜错受到1点伤害。"},
	"guose":     {"国色", "可以将方块牌当乐不思蜀使用。"},
	"liuli":     {"流离", "成为杀的目标时，可弃一张牌，将杀转移给你攻击范围内另一名角色（不能是使用者）。"},
	"qianxun":   {"谦逊", "不能成为顺手牵羊和乐不思蜀的目标。"},
	"lianying":  {"连营", "失去最后的手牌后，可以摸一张牌。"},
	"jieyin":    {"结姻", "出牌阶段限一次：弃两张手牌，与一名已受伤的男性角色各回复1点体力。"},
	"xiaoji":    {"枭姬", "每失去一张装备区的牌后，可以摸两张牌。"},
	"qingnang":  {"青囊", "出牌阶段限一次：弃一张手牌，令一名已受伤角色回复1点体力。"},
	"jijiu":     {"急救", "你的回合外，可以将红色牌当桃使用。"},
	"wushuang":  {"无双", "杀需两张闪抵消；与你决斗的角色每次需打出两张杀。"},
	"lijian":    {"离间", "出牌阶段限一次：弃一张牌，指定两名男性角色，令后一名对前一名使用不可无懈的决斗。"},
	"biyue":     {"闭月", "结束阶段可以摸一张牌。"},
}
var SGGenerals = []SGGeneral{
	{"caocao", "曹操", "wei", 4, false, []string{"jianxiong", "hujia"}},
	{"simayi", "司马懿", "wei", 3, false, []string{"fankui", "guicai"}},
	{"xiahoudun", "夏侯惇", "wei", 4, false, []string{"ganglie"}},
	{"zhangliao", "张辽", "wei", 4, false, []string{"tuxi"}},
	{"xuchu", "许褚", "wei", 4, false, []string{"luoyi"}},
	{"guojia", "郭嘉", "wei", 3, false, []string{"tiandu", "yiji"}},
	{"zhenji", "甄姬", "wei", 3, true, []string{"qingguo", "luoshen"}},
	{"liubei", "刘备", "shu", 4, false, []string{"rende", "jijiang"}},
	{"guanyu", "关羽", "shu", 4, false, []string{"wusheng"}},
	{"zhangfei", "张飞", "shu", 4, false, []string{"paoxiao"}},
	{"zhugeliang", "诸葛亮", "shu", 3, false, []string{"guanxing", "kongcheng"}},
	{"zhaoyun", "赵云", "shu", 4, false, []string{"longdan"}},
	{"machao", "马超", "shu", 4, false, []string{"mashu", "tieji"}},
	{"huangyueying", "黄月英", "shu", 3, true, []string{"jizhi", "qicai"}},
	{"sunquan", "孙权", "wu", 4, false, []string{"zhiheng", "jiuyuan"}},
	{"ganning", "甘宁", "wu", 4, false, []string{"qixi"}},
	{"lvmeng", "吕蒙", "wu", 4, false, []string{"keji"}},
	{"huanggai", "黄盖", "wu", 4, false, []string{"kurou"}},
	{"zhouyu", "周瑜", "wu", 3, false, []string{"yingzi", "fanjian"}},
	{"daqiao", "大乔", "wu", 3, true, []string{"guose", "liuli"}},
	{"luxun", "陆逊", "wu", 3, false, []string{"qianxun", "lianying"}},
	{"sunshangxiang", "孙尚香", "wu", 3, true, []string{"jieyin", "xiaoji"}},
	{"huatuo", "华佗", "qun", 3, false, []string{"qingnang", "jijiu"}},
	{"lvbu", "吕布", "qun", 4, false, []string{"wushuang"}},
	{"diaochan", "貂蝉", "qun", 3, true, []string{"lijian", "biyue"}},
}

type SGCard struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"`
	Suit int    `json:"suit"`
	Rank int    `json:"rank"`
}
type SGCardInfo struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Slot  string `json:"slot,omitempty"`
	Range int    `json:"range,omitempty"`
}

var SGCardTypes = map[string]SGCardInfo{
	"slash": {"杀", "出牌阶段通常限一次，对攻击范围内一名角色造成1点伤害，目标可用闪抵消。", "", 0},
	"jink":  {"闪", "响应杀，抵消一次杀。", "", 0}, "peach": {"桃", "出牌阶段回复自己1点体力；濒死求桃时可救助角色。", "", 0},
	"duel":          {"决斗", "目标先开始与你轮流打出杀，未能打出的一方受到对方造成的1点伤害。", "", 0},
	"snatch":        {"顺手牵羊", "获得距离1以内其他角色区域内的一张牌。", "", 0},
	"dismantlement": {"过河拆桥", "弃置一名其他角色区域内的一张牌。", "", 0},
	"ex_nihilo":     {"无中生有", "摸两张牌。", "", 0}, "amazing_grace": {"五谷丰登", "展示存活人数张牌，从你开始依次各选一张。", "", 0},
	"god_salvation":  {"桃园结义", "所有已受伤角色各回复1点体力。", "", 0},
	"savage_assault": {"南蛮入侵", "所有其他角色依次打出杀，否则受到1点伤害。", "", 0},
	"archery_attack": {"万箭齐发", "所有其他角色依次打出闪，否则受到1点伤害。", "", 0},
	"collateral":     {"借刀杀人", "指定有武器的角色和其攻击范围内的另一角色；前者须对后者使用杀，否则将武器交给你。", "", 0},
	"nullification":  {"无懈可击", "抵消锦囊对一名角色的效果，或抵消另一张无懈可击。", "", 0},
	"indulgence":     {"乐不思蜀", "判定不为红桃，跳过出牌阶段。", "", 0},
	"lightning":      {"闪电", "判定黑桃2至9，受到3点伤害；否则移至下一名可放置闪电的角色。", "", 0},
	"crossbow":       {"诸葛连弩", "出牌阶段使用杀没有次数限制。", "weapon", 1},
	"double_sword":   {"雌雄双股剑", "对异性使用杀时，可令其弃一张手牌或让你摸一张牌。", "weapon", 2},
	"qinggang_sword": {"青釭剑", "使用杀时忽略目标防具。", "weapon", 2},
	"blade":          {"青龙偃月刀", "杀被闪抵消后，可对相同目标再使用杀。", "weapon", 3},
	"spear":          {"丈八蛇矛", "可将两张手牌当杀使用或打出。", "weapon", 3},
	"axe":            {"贯石斧", "杀被闪抵消后，可弃两张牌（不含此斧）令杀仍造成伤害。", "weapon", 3},
	"halberd":        {"方天画戟", "使用最后一张手牌为杀时，可指定至多三名目标。", "weapon", 4},
	"kylin_bow":      {"麒麟弓", "杀造成伤害时，可弃置目标的一张坐骑。", "weapon", 5},
	"ice_sword":      {"寒冰剑", "杀造成伤害时，可防止伤害，依次弃置目标至多两张牌。", "weapon", 2},
	"eight_diagram":  {"八卦阵", "需要闪时可判定，红色视为使用或打出闪。", "armor", 0},
	"renwang_shield": {"仁王盾", "黑色杀对你无效。", "armor", 0},
	"jueying":        {"绝影", "其他角色计算与你的距离+1。", "defense", 0}, "dilu": {"的卢", "其他角色计算与你的距离+1。", "defense", 0}, "zhuahuangfeidian": {"爪黄飞电", "其他角色计算与你的距离+1。", "defense", 0},
	"chitu": {"赤兔", "你计算与其他角色的距离−1。", "offense", 0}, "dayuan": {"大宛", "你计算与其他角色的距离−1。", "offense", 0}, "zixing": {"紫骍", "你计算与其他角色的距离−1。", "offense", 0},
}

// Suits: spade, heart, club, diamond. Each pair is a real physical card.
func sgDeck() []SGCard {
	rows := [][]string{
		{"lightning", "duel", "eight_diagram", "double_sword", "dismantlement", "snatch", "dismantlement", "snatch", "blade", "jueying", "indulgence", "qinggang_sword", "savage_assault", "slash", "slash", "slash", "slash", "slash", "slash", "slash", "nullification", "snatch", "spear", "dismantlement", "dayuan", "savage_assault"},
		{"god_salvation", "archery_attack", "jink", "jink", "peach", "amazing_grace", "peach", "amazing_grace", "kylin_bow", "chitu", "peach", "indulgence", "peach", "ex_nihilo", "peach", "ex_nihilo", "peach", "ex_nihilo", "slash", "slash", "slash", "ex_nihilo", "peach", "dismantlement", "zhuahuangfeidian", "jink"},
		{"crossbow", "duel", "eight_diagram", "slash", "dismantlement", "slash", "dismantlement", "slash", "dilu", "slash", "indulgence", "slash", "savage_assault", "slash", "slash", "slash", "slash", "slash", "slash", "slash", "slash", "slash", "collateral", "nullification", "collateral", "nullification"},
		{"crossbow", "duel", "jink", "jink", "jink", "snatch", "jink", "snatch", "jink", "axe", "jink", "slash", "jink", "slash", "jink", "slash", "jink", "slash", "jink", "slash", "jink", "jink", "peach", "halberd", "slash", "zixing"},
	}
	cards := []SGCard{}
	for suit, row := range rows {
		for j, kind := range row {
			cards = append(cards, SGCard{ID: len(cards) + 1, Kind: kind, Suit: suit, Rank: j/2 + 1})
		}
	}
	for _, c := range []SGCard{{Kind: "ice_sword", Suit: 0, Rank: 2}, {Kind: "renwang_shield", Suit: 2, Rank: 2}, {Kind: "lightning", Suit: 1, Rank: 12}, {Kind: "nullification", Suit: 3, Rank: 12}} {
		c.ID = len(cards) + 1
		cards = append(cards, c)
	}
	return cards
}

var sgCards = sgDeck()

func sgCard(id int) SGCard {
	if id > len(sgCards) && id <= len(sgCards)+len(sgMilitaryCards) {
		return sgMilitaryCards[id-len(sgCards)-1]
	}
	if id < 1 || id > len(sgCards) {
		return SGCard{}
	}
	return sgCards[id-1]
}
func sgGeneral(id string) SGGeneral {
	for _, g := range SGGenerals {
		if g.ID == id {
			return g
		}
	}
	return SGGeneral{}
}
