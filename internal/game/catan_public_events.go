package game

import "errors"

// The site's explicit catalogue uses independently cross-checked legacy faces
// with 2025 event effects and New Year lifecycle. It is not a claim that the
// current physical 2025 production/event pairing has been verified.
const CatanEventCatalogue = "wire-board-events-v1"

// Trade Advantage may steal a commodity, matching the mixed concealed hand.
// This interpretation is explicitly recorded as a site combination rule.
const CatanEventKnightsRules = "wire-board-events-knights-v1"

const CatanEventClothFallbackRules = "wire-board-events-cloth-fallback-v1"
const catanEventCatalogueNotice = "本站事件牌组：采用已交叉核对的旧版36张点数／事件配比，按2025事件效果执行；不宣称等同2025实体牌表"

// EnableCatanEvents is an atomic creation-only step after the selected map is
// initialized. Public room validation independently limits supported recipes.
func (s *State) EnableCatanEvents(catalogue string) error {
	if catalogue != CatanEventCatalogue || s.Kind != "catan" || s.Catan == nil || s.Finished || s.Catan.RollID != 0 || s.Catan.EventDeck != nil {
		return errors.New("只能在新卡坦对局开始前启用本站事件牌组")
	}
	next := clone(*s)
	next.Catan.EventDeck = &catanEventSession{Catalogue: catalogue, Deck: newCatanEventDeck()}
	if next.Catan.Explorer != nil {
		next.Catan.EventDeck.Explorer = CatanEventExplorerRules
	}
	if next.Catan.CitiesKnights != nil {
		next.Catan.EventDeck.Knights = CatanEventKnightsRules
	}
	if next.Catan.cloth() != nil {
		next.Catan.EventDeck.ClothFallback = CatanEventClothFallbackRules
	}
	if next.Catan.pirateIslands() != nil && !next.Catan.attackSeaKnights() {
		next.Catan.EventDeck.FleetRules = CatanEventFleetRules
	}
	if err := next.validateCatanEventSession(); err != nil {
		return err
	}
	if next.Catan.Explorer != nil {
		next.Log = append(next.Log, "探险事件牌：使用本站36张牌组的生产点数，按2025官方组合说明忽略所有事件文字；双人同样每回合一张，五六人仅第一位抽牌。金币补偿、7点海盗与独立鱼群骰照常；城市骑士使用独立红骰／事件骰，炼金术替代抽牌。")
	} else {
		next.Log = append(next.Log, catanEventCatalogueNotice)
	}
	if next.Catan.Transport != nil {
		next.Log = append(next.Log, "运输事件：地震损坏道路花2移动点，7点移动蛮族，强盗逃跑无效果；抽到2或12仍执行事件，不重抽。金币与货物不参与资源事件。")
	}
	if next.Catan.CitiesKnights != nil && next.Catan.Explorer == nil {
		next.Log = append(next.Log, "事件牌与城市骑士：先事件文字，再独立红骰／事件骰，最后生产；炼金术替代抽牌。本站补充规则：贸易优势随机偷取资源或商品。")
	}
	if next.Catan.cloth() != nil {
		next.Log = append(next.Log, "本站补充规则：布匹地图的强盗逃跑只能进入大岛沙漠；没有合法沙漠则移到场外，不偷牌、不移动海盗。")
	}
	if next.Catan.pirateIslands() != nil {
		next.Log = append(next.Log, "本站补充规则：事件牌只决定生产；额外独立掷两颗舰队骰，取较小值移动和攻击。先事件、再舰队奖励、最后生产；强盗逃跑没有效果。")
	}
	*s = next
	return nil
}
