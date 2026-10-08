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
	if next.Catan.CitiesKnights != nil {
		next.Catan.EventDeck.Knights = CatanEventKnightsRules
	}
	if next.Catan.cloth() != nil {
		next.Catan.EventDeck.ClothFallback = CatanEventClothFallbackRules
	}
	if err := next.validateCatanEventSession(); err != nil {
		return err
	}
	next.Log = append(next.Log, catanEventCatalogueNotice)
	if next.Catan.CitiesKnights != nil {
		next.Log = append(next.Log, "事件牌与城市骑士：先事件文字，再独立红骰／事件骰，最后生产；炼金术替代抽牌。本站补充规则：贸易优势随机偷取资源或商品。")
	}
	if next.Catan.cloth() != nil {
		next.Log = append(next.Log, "本站补充规则：布匹地图的强盗逃跑只能进入大岛沙漠；没有合法沙漠则移到场外，不偷牌、不移动海盗。")
	}
	*s = next
	return nil
}
