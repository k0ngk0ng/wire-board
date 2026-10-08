package game

import (
	"errors"
	"slices"
)

// Helpers 2022 documents base/Seafarers only. Explorer's missing development
// deck, robber and played knight cards require explicitly labelled site rules.
const CatanExplorerHelpersRules = "wire-board-explorer-helpers-v1"

type catanExplorerHelpers struct {
	Rules      string                         `json:"rules"`
	Production *catanExplorerHelperProduction `json:"production,omitempty"`
}

type catanExplorerHelperProduction struct {
	RollID   int   `json:"rollId"`
	Received []int `json:"received"`
}

func catanExplorerHelperRules() []CatanHelper {
	rules := CatanHelpers()
	rules[2].Description = "非7点生产没有获得资源或商品时，可额外领取一张普通资源；金币、鱼筹码和引水渠不影响此资格。"
	rules[4].Description = "7点生产必须使用：资源与商品合计超过7张时免弃牌，否则领取一张普通资源。"
	rules[5].Title = "灵活造船（本站）"
	rules[5].Description = "本站探险适配：造一艘船时，可以用任意一种普通资源替换一张木材或羊毛；保留原造船位置、拆船及货物规则。"
	rules[7].Title = "人员参与建设（本站）"
	rules[7].Description = "本站探险适配：归还一名港口内或停靠己方港口的船上人员（初航用移民，其余任务用船员），用木砖各1建村，或粮食1矿石2升级港口／城市。不能撤回巢穴或农场船员。"
	rules[9].Title = "驱逐海盗（本站）"
	rules[9].Description = "本站探险适配：自己生产前或建设阶段，将海盗移出地图并领取1金币；没有海盗时也可领取，不偷牌。"
	rules[10].Title = "海上补给（本站）"
	rules[10].Description = "本站探险适配：建设阶段从银行领取一张自选普通资源；无需强盗或海盗在场。"
	rules[11].Title = "更换补给（本站）"
	rules[11].Description = "本站探险适配：建设阶段将一张普通资源归还银行，换取另一种有库存的普通资源。不能交换商品。"
	return rules
}

// EnableCatanExplorerHelpers configures the versioned adaptation before any play.
func (s *State) EnableCatanExplorerHelpers(all bool) error {
	return s.enableCatanExplorerHelpers(all)
}

func (s *State) enableCatanExplorerHelpers(all bool) error {
	if s == nil || s.Catan == nil || s.Catan.Explorer == nil || s.Finished {
		return errors.New("探险助手需要尚未开始行动的探索者牌桌")
	}
	if err := s.validateCatanExplorer(); err != nil {
		return err
	}
	g := s.Catan
	if g.RollID != 0 || g.Explorer.ActionID != 0 || g.Explorer.Helpers != nil || g.Explorer.Setup != nil && g.Explorer.Setup.Step != 0 {
		return errors.New("探险助手只能随新游戏初始化")
	}
	next := clone(*s)
	g = next.Catan
	g.Options = CatanOptions{Helpers: true, AllHelpers: all, Rules: CatanExpansionRules}
	g.Explorer.Helpers = &catanExplorerHelpers{Rules: CatanExplorerHelpersRules}
	pool := []int{}
	for id := len(g.Players) + 1; id <= 12; id++ {
		pool = append(pool, id)
	}
	shuffle(pool)
	if !all {
		pool = pool[:len(g.Players)]
	}
	g.HelperDisplay = pool
	for p := range g.Players {
		g.Players[p].Helper = &CatanHelperSeat{ID: (p-g.StartPlayer+len(g.Players))%len(g.Players) + 1}
	}
	next.Log = append(next.Log, "本站探险 Helpers：按先手顺序领取助手，开局完成后可用；发展卡／强盗／骑士卡能力改为造船、船员建设与补给，使用后仍须翻面或交换")
	if err := next.validateCatanExplorer(); err != nil {
		return err
	}
	*s = next
	return nil
}

// Inventory-only checks also run inside cargo transactions. They must not
// inspect State.Phase, which is synchronized after those transactions commit.
func (g *Catan) validateExplorerHelperInventory() error {
	var x *catanExplorerHelpers
	if g.Explorer != nil {
		x = g.Explorer.Helpers
	}
	if x == nil {
		if g.Options != (CatanOptions{}) || len(g.HelperDisplay)+len(g.HelperExile) != 0 || g.HelperPending != nil {
			return errors.New("未启用探险助手却包含助手选项或回应")
		}
		for _, p := range g.Players {
			if p.Helper != nil {
				return errors.New("未启用探险助手却已分配助手")
			}
		}
		return nil
	}
	if x.Rules != CatanExplorerHelpersRules || !g.Options.Helpers || g.Options.FiveSix || g.Options.Rules != CatanExpansionRules || len(g.HelperExile) != 0 {
		return errors.New("探险助手规则版本或组件无效")
	}
	seen := map[int]bool{}
	add := func(id int) bool {
		if id < 1 || id > 12 || seen[id] {
			return false
		}
		seen[id] = true
		return true
	}
	for _, id := range g.HelperDisplay {
		if !add(id) {
			return errors.New("探险助手展示区重复或无效")
		}
	}
	for _, p := range g.Players {
		h := p.Helper
		if p.Eliminated {
			if h != nil {
				return errors.New("离场探险玩家必须归还助手")
			}
			continue
		}
		if h == nil || !add(h.ID) || h.AcquiredTurn > g.TurnSerial || h.UsedTurn > g.TurnSerial {
			return errors.New("探险助手归属或使用回合无效")
		}
	}
	count := 2 * len(g.Players)
	if g.Options.AllHelpers {
		count = 12
	}
	if len(seen) != count {
		return errors.New("探险助手数量不守恒")
	}
	return nil
}

func (s *State) validateExplorerHelpers() error {
	g, x := s.Catan, s.Catan.Explorer
	if err := g.validateExplorerHelperInventory(); err != nil {
		return err
	}
	if x.Helpers == nil {
		return nil
	}
	q := g.HelperPending
	if q == nil {
		if x.Helpers.Production != nil || !s.Finished && s.Phase == "catan_helper" {
			return errors.New("探险助手生产或回应记录缺失")
		}
		return nil
	}
	if s.Finished || x.Setup != nil || s.Phase != "catan_helper" || q.Player < 0 || q.Player >= len(g.Players) || g.Players[q.Player].Eliminated || g.Trade != nil || g.FreeRoads != 0 || g.Fishing != nil && g.Fishing.Pending != nil || g.CitiesKnights != nil && (g.CitiesKnights.Event != nil || g.CitiesKnights.Pending != nil) || x.Pirate != nil && x.Pirate.Pending != nil {
		return errors.New("探险助手与其他回应冲突")
	}
	h := g.Players[q.Player].Helper
	if q.Kind != "resource" && q.Optional || len(q.Cards) != 0 {
		return errors.New("探险助手不能包含发展卡候选")
	}
	if production := x.Helpers.Production; production != nil {
		if q.Resume != "explorer_production" || production.RollID != g.RollID || production.RollID < 1 || len(production.Received) != len(g.Players) || x.Economy.Turn.NoProduction || !slices.Contains([]string{"ready", "aqueduct", "discard", "pirate"}, x.Economy.Turn.Phase) || (q.Kind != "resource" && q.Kind != "exchange") {
			return errors.New("探险助手生产接续无效")
		}
		for _, n := range production.Received {
			if n < 0 || n > len(g.Bank)*catanExplorerStock(len(g.Players)).resources {
				return errors.New("探险助手生产所得无效")
			}
		}
		seven := x.Economy.Turn.productionNumber() == 7
		if h.ID != 3 && h.ID != 5 || seven != (h.ID == 5) || !seven && production.Received[q.Player] != 0 || q.Kind == "resource" && q.Optional != !seven {
			return errors.New("探险助手补偿与生产点数或所得不符")
		}
	} else if q.Player != s.Turn || q.Resume != "catan_turn" && !(q.Resume == "catan_roll" && h.ID == 10) || q.Resume != s.catanExplorerPhaseWithoutHelper() || q.Kind != "exchange" && q.Kind != "leader" {
		return errors.New("探险助手行动后续阶段无效")
	}
	if q.Kind == "exchange" {
		if h.UsedTurn != g.TurnSerial || h.AcquiredTurn >= g.TurnSerial {
			return errors.New("尚未使用探险助手不能翻面或交换")
		}
	} else if !g.helperReady(q.Player, h.ID) {
		return errors.New("探险助手尚不可用或本回合已使用")
	}
	if q.Kind == "resource" && sum(g.Bank[:5]) == 0 {
		return errors.New("银行没有助手可领取的普通资源")
	}
	if q.Kind == "leader" && (h.ID != 7 || q.Target < 0 || q.Target >= len(g.Players) || q.Target == q.Player || g.Players[q.Target].Eliminated || g.Players[q.Target].Score <= g.Players[q.Player].Score || sum(g.Players[q.Target].Resources) == 0) {
		return errors.New("探险助手查看手牌目标无效")
	}
	return nil
}

// Returns whether a helper interrupted production. There is at most one Hilda
// or Thorolf in the shared pool. Fish replacement finishes first; Aqueduct is
// determined from original production, not from this extra helper resource.
func (s *State) catanExplorerProductionHelper(received []int) bool {
	g, x := s.Catan, s.Catan.Explorer
	if x.Helpers == nil {
		return false
	}
	seven := x.Economy.Turn.productionNumber() == 7
	for p := range g.Players {
		id := 3
		if seven {
			id = 5
		}
		if !g.helperReady(p, id) || !seven && received[p] != 0 {
			continue
		}
		if !seven && sum(g.Bank[:5]) == 0 {
			continue
		}
		x.Helpers.Production = &catanExplorerHelperProduction{RollID: g.RollID, Received: slices.Clone(received)}
		g.Trade = nil
		if seven && (sum(g.Players[p].Resources) > 7 || sum(g.Bank[:5]) == 0) {
			s.catanLog(p, "托罗夫七点庇护：免弃牌；不领取资源")
			s.catanHelperComplete(p, "explorer_production")
		} else {
			s.catanHelperAsk(CatanHelperPending{Player: p, Kind: "resource", Resume: "explorer_production", Optional: !seven})
		}
		g.DiscardDue = slices.Clone(x.Economy.Turn.Discard)
		return true
	}
	return false
}

func (s *State) catanExplorerHelperRespond(player int, a Action) error {
	g := s.Catan
	if a.Prompt < 1 || uint64(a.Prompt) != g.TurnSerial || a.Skill != "" {
		return errors.New("探险助手回应序号已过期")
	}
	if q := g.HelperPending; q != nil && q.Player == player && q.Kind == "leader" && a.Type == "catan_helper_choice" && a.Choice == "skip" && sum(g.Players[q.Target].Resources[:5]) == 0 {
		s.catanLog(player, "所查看的对手没有可领取的普通资源，完成助手使用")
		s.catanHelperComplete(player, q.Resume)
		return nil
	}
	if err := s.catanHelperRespond(player, a); err != nil {
		return err
	}
	if g.HelperPending == nil {
		if production := g.Explorer.Helpers.Production; production != nil {
			g.Explorer.Helpers.Production = nil
			return s.catanExplorerFinishProductionBonuses(production.Received)
		}
		s.catanExplorerSyncPhase()
	}
	return nil
}

func (s *State) catanExplorerHelperView(v map[string]any, viewer int) {
	g, x := s.Catan, s.Catan.Explorer
	if x.Helpers == nil {
		return
	}
	v["helperRules"] = catanExplorerHelperRules()
	v["explorer"].(map[string]any)["helperRules"] = x.Helpers.Rules
	for p, raw := range v["players"].([]any) {
		if h := g.Players[p].Helper; h != nil {
			raw.(map[string]any)["helperReady"] = x.Setup == nil && !s.Finished && g.helperReady(p, h.ID)
		}
	}
	if q := g.HelperPending; q != nil {
		public := v["helperPending"].(map[string]any)
		delete(public, "cards")
		if q.Kind == "leader" && q.Player == viewer {
			public["resources"] = slices.Clone(g.Players[q.Target].Resources[:5])
		}
	}
}
