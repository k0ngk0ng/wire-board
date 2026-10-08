package game

import "errors"

// The site's explicit catalogue uses independently cross-checked legacy faces
// with 2025 event effects and New Year lifecycle. It is not a claim that the
// current physical 2025 production/event pairing has been verified.
const CatanEventCatalogue = "wire-board-events-v1"
const catanEventCatalogueNotice = "本站事件牌组：采用已交叉核对的旧版36张点数／事件配比，按2025事件效果执行；不宣称等同2025实体牌表"

// EnableCatanEvents is an atomic creation-only step after the selected map is
// initialized. Public room validation independently limits supported recipes.
func (s *State) EnableCatanEvents(catalogue string) error {
	if catalogue != CatanEventCatalogue || s.Kind != "catan" || s.Catan == nil || s.Finished || s.Catan.RollID != 0 || s.Catan.EventDeck != nil {
		return errors.New("只能在新卡坦对局开始前启用本站事件牌组")
	}
	next := clone(*s)
	next.Catan.EventDeck = &catanEventSession{Catalogue: catalogue, Deck: newCatanEventDeck()}
	if err := next.validateCatanEventSession(); err != nil {
		return err
	}
	next.Log = append(next.Log, catanEventCatalogueNotice)
	*s = next
	return nil
}
