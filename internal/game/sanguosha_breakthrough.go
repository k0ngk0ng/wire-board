package game

// Revised standard generals have permanent, independent IDs. A pregame option
// replaces the 21 corresponding classic candidates, never offering both versions.
var sgJieGenerals = []SGGeneral{
	{"jie_caocao", "界曹操", "wei", 4, false, []string{"jie_jianxiong", "hujia"}},
	{"jie_simayi", "界司马懿", "wei", 3, false, []string{"jie_fankui", "jie_guicai"}},
	{"jie_xiahoudun", "界夏侯惇", "wei", 4, false, []string{"jie_ganglie", "qingjian"}},
	{"jie_zhangliao", "界张辽", "wei", 4, false, []string{"jie_tuxi"}},
	{"jie_xuchu", "界许褚", "wei", 4, false, []string{"jie_luoyi"}},
	{"jie_guojia", "界郭嘉", "wei", 3, false, []string{"tiandu", "jie_yiji"}},
	{"jie_liubei", "界刘备", "shu", 4, false, []string{"jie_rende", "jijiang"}},
	{"jie_guanyu", "界关羽", "shu", 4, false, []string{"wusheng", "yijue"}},
	{"jie_zhangfei", "界张飞", "shu", 4, false, []string{"paoxiao", "tishen"}},
	{"jie_zhaoyun", "界赵云", "shu", 4, false, []string{"longdan", "yajiao"}},
	{"jie_machao", "界马超", "shu", 4, false, []string{"mashu", "jie_tieji"}},
	{"jie_huangyueying", "界黄月英", "shu", 3, true, []string{"jie_jizhi", "jie_qicai"}},
	{"jie_ganning", "界甘宁", "wu", 4, false, []string{"qixi", "fenwei"}},
	{"jie_lvmeng", "界吕蒙", "wu", 4, false, []string{"keji", "qinxue"}},
	{"jie_huanggai", "界黄盖", "wu", 4, false, []string{"jie_kurou", "zhaxiang"}},
	{"jie_zhouyu", "界周瑜", "wu", 3, false, []string{"jie_yingzi", "jie_fanjian"}},
	{"jie_daqiao", "界大乔", "wu", 3, true, []string{"jie_guose", "liuli"}},
	{"jie_luxun", "界陆逊", "wu", 3, false, []string{"jie_qianxun", "jie_lianying"}},
	{"jie_huatuo", "界华佗", "qun", 3, false, []string{"chuli", "jijiu"}},
	{"jie_lvbu", "界吕布", "qun", 5, false, []string{"wushuang", "liyu"}},
	{"jie_diaochan", "界貂蝉", "qun", 3, true, []string{"jie_lijian", "biyue"}},
}

func init() {
	for k, v := range map[string]SGSkill{
		"jie_jianxiong": {"奸雄·界", "受到伤害后，可选择获得造成伤害且仍全在处理区的牌，或摸一张牌。"},
		"jie_fankui":    {"反馈·界", "每受到1点伤害，可获得来源的一张手牌或装备。多点伤害依次选择，放弃后停止剩余次数。"},
		"jie_guicai":    {"鬼才·界", "判定生效前，可以用一张手牌或装备替换判定牌。"},
		"jie_ganglie":   {"刚烈·界", "每受到1点伤害，可判定：红色对来源造成1伤害；黑色弃来源一张手牌或装备。放弃后停止剩余次数。"},
		"qingjian":      {"清俭", "自己的摸牌阶段外获得手牌后，可将本次获得的牌分配给其他角色。初始发牌不触发。"},
		"jie_tuxi":      {"突袭·界", "正常摸牌时可少摸任意张牌，并选择等量有手牌且手牌数不少于自己的其他角色。先摸剩余牌，再获得各目标随机手牌一张。"},
		"jie_luoyi":     {"裸衣·界", "可跳过摸牌阶段，亮出顶三张，获得其中基本牌、武器和决斗。直到自己下次回合开始，杀和决斗的直接伤害+1，包括回合外。"},
		"jie_yiji":      {"遗计·界", "每受到1点伤害可摸两张，再选择至多两名其他角色，各扣置一至两张当前手牌到其私有遗计牌堆。其下次进入摸牌阶段时获得。"},
		"jie_rende":     {"仁德·界", "出牌阶段限一次连续分配：将发动时已有的手牌交给其他角色，可继续分配同批余牌；首次累计给出两张时回复1体力。"},
		"yijue":         {"义绝", "出牌阶段限一次，与另一角色拼点。胜出令其本回合不能使用或打出手牌、非锁定技能失效；未胜出可令其回复1体力。"},
		"tishen":        {"替身", "限定技：准备阶段，若体力低于自己上回合结束时，可回复至该值（不超过上限），并摸等于实际回复量的牌。"},
		"yajiao":        {"涯角", "回合外使用或打出手牌后可亮顶一张；与所用牌同类别，可交给任意角色；不同类别可弃置。未处理的牌放回顶。"},
		"jie_tieji":     {"铁骑·界", "使用杀指定目标后，可令其非锁定技能失效至回合结束并判定。目标弃同花色手牌或装备，否则不能使用闪响应该杀。"},
		"jie_jizhi":     {"集智·界", "使用任何锦囊时可亮顶一张；非基本直接获得，基本可用一张手牌置顶交换，否则弃置亮牌。"},
		"jie_qicai":     {"奇才·界", "锁定技：锦囊无距离限制；其他角色不能弃置你装备区的非坐骑牌，获得或移动仍允许。"},
		"fenwei":        {"奋威", "限定技：非延时锦囊指定至少两名目标时，可令其对所选目标无效。"},
		"qinxue":        {"勤学", "觉醒技：准备阶段手牌比体力多至少3张（初始七人及以上为2张），减1体力上限并获得攻心。"},
		"jie_kurou":     {"苦肉·界", "出牌阶段限一次：弃一张自己的手牌或装备，失去1体力。"},
		"zhaxiang":      {"诈降", "锁定技：每失去1体力摸三张。若在自己的出牌阶段，本回合杀次数+1，红杀无距离限制且不能被闪抵消。"},
		"jie_yingzi":    {"英姿·界", "锁定技：正常摸牌数+1，手牌上限固定为体力上限。"},
		"jie_fanjian":   {"反间·界", "出牌阶段限一次：给另一角色一张所选花色手牌；其选择亮全部手牌并弃该花色所有手牌和装备，或失去1体力。"},
		"jie_guose":     {"国色·界", "出牌阶段限一次：将方片手牌或装备当乐不思蜀，或弃方片移除场上一张乐不思蜀（可含自己）。完成后摸一张。"},
		"jie_qianxun":   {"谦逊·界", "成为别人非延时锦囊唯一目标或延时锦囊生效时，可把全部手牌扣为谦逊牌，当前回合结束时取回。"},
		"jie_lianying":  {"连营·界", "失去最后手牌后，可选择至多本次失去手牌数的不同存活角色，各摸一张，可选自己。"},
		"chuli":         {"除疠", "出牌阶段限一次：弃自己一张牌，选择任意名彼此势力不同的其他角色，分别弃其一张手牌或装备。全部弃完后，以此法弃黑桃的各牌主摸一张。"},
		"liyu":          {"利驭", "杀造成伤害后，受伤者可选择另一名合法决斗目标，让你获得其一张手牌或装备，然后你对所选目标使用虚拟决斗。"},
		"jie_lijian":    {"离间·界", "出牌阶段限一次：弃一张手牌或装备，令所选两名其他男性角色决斗；此决斗可以被无懈可击。"},
	} {
		SGSkills[k] = v
	}
}
