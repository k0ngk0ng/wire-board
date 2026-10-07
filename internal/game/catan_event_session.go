package game

import "errors"

// This is a LEGACY REFERENCE catalogue for internal integration, not a verified
// 2025 physical deck. No public option or client action can enable it.
// Facts transcribed from docs/research/catan-event-cards-legacy-reference.csv;
// deliberately omit the old printed red die (2025 C&K needs independent dice).
const catanEventReferenceCatalogue = "legacy-reference-v1"

var catanEventReferenceFaces = [...]struct {
	Production int
	Kind       string
}{
	{2, "plentiful_year"},
	{3, "conflict"},
	{3, "beautiful_day"},
	{4, "beautiful_day"},
	{4, "robber_flees"},
	{4, "robber_flees"},
	{5, "beautiful_day"},
	{5, "beautiful_day"},
	{5, "tournament"},
	{5, "trade_advantage"},
	{6, "epidemic"},
	{6, "earthquake"},
	{6, "good_neighbors"},
	{6, "beautiful_day"},
	{6, "beautiful_day"},
	{7, "robber_attacks"},
	{7, "robber_attacks"},
	{7, "robber_attacks"},
	{7, "robber_attacks"},
	{7, "robber_attacks"},
	{7, "robber_attacks"},
	{8, "epidemic"},
	{8, "beautiful_day"},
	{8, "beautiful_day"},
	{8, "beautiful_day"},
	{8, "beautiful_day"},
	{9, "beautiful_day"},
	{9, "beautiful_day"},
	{9, "beautiful_day"},
	{9, "calm_seas"},
	{10, "helpful_neighbor"},
	{10, "beautiful_day"},
	{10, "beautiful_day"},
	{11, "helpful_neighbor"},
	{11, "beautiful_day"},
	{12, "calm_seas"},
}

type catanEventSession struct {
	Catalogue string         `json:"catalogue"`
	Deck      catanEventDeck `json:"deck"`
}

// Private research entry. Additional module combinations need their own
// acceptance before enabling; neither constructor is a public room option.
func newCatanReferenceEvents(n int) (*State, error) {
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		return nil, err
	}
	s.Catan.EventDeck = &catanEventSession{Catalogue: catanEventReferenceCatalogue, Deck: newCatanEventDeck()}
	s.Log = append(s.Log, "内部测试：使用旧版参考事件牌表，尚非已核验的2025正式牌表")
	return s, s.validateCatanEventSession()
}

func newCatanAttackReferenceEvents(n int) (*State, error) {
	s, err := newCatanAttackState(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		return nil, err
	}
	s.Catan.EventDeck = &catanEventSession{Catalogue: catanEventReferenceCatalogue, Deck: newCatanEventDeck()}
	s.Log = append(s.Log, "内部测试：使用旧版参考事件牌表，尚非已核验的2025正式牌表")
	return s, s.validateCatanEventSession()
}

func (s *State) validateCatanEventSession() error {
	g := s.Catan
	if g == nil || g.EventDeck == nil {
		return nil
	}
	session := g.EventDeck
	if session.Catalogue != catanEventReferenceCatalogue {
		return errors.New("不支持的事件牌参考表版本")
	}
	if g.Explorer != nil || g.Transport != nil || g.Two != nil || g.Caravans != nil || g.Rivers != nil || g.Fishing != nil || g.CitiesKnights != nil || g.Seafarers != nil || g.Harbors != nil || g.FriendlyRobber != nil || g.Options.Helpers || g.Options.AllHelpers {
		return errors.New("该组合尚未接入完整事件牌抽取")
	}
	if len(g.Players) < 3 || len(g.Players) > 6 || g.Options.FiveSix != (len(g.Players) > 4) {
		return errors.New("事件牌玩家数量与规则不一致")
	}
	d := session.Deck
	if err := d.validate(); err != nil {
		return err
	}
	// The scenario independently checks resource/piece supply, conquered
	// buildings, private gifts, event responders and end-of-turn battles.
	if err := s.validateCatanAttack(); err != nil {
		return err
	}
	// Compare using division rather than multiplying a potentially corrupt cycle.
	// Every cycle reveals 31 normal cards; five remain hidden below New Year.
	const drawsPerCycle = catanEventNormalCards - catanEventBottom
	if g.RollID < len(d.Discard) || (g.RollID-len(d.Discard))%drawsPerCycle != 0 || uint64((g.RollID-len(d.Discard))/drawsPerCycle) != d.Cycle-1 || d.Cycle > 1 && len(d.Discard) == 0 {
		return errors.New("事件牌抽取次数与存档回合不一致")
	}
	if g.RollID == 0 {
		if g.CardEvent != nil || g.RevealedEvent != nil {
			return errors.New("尚未抽牌却有事件结算")
		}
		return nil
	}
	if g.setup() || len(d.Discard) == 0 {
		return errors.New("无效事件牌开局状态")
	}
	face := catanEventReferenceFaces[d.Discard[len(d.Discard)-1]]
	revealed := g.RevealedEvent
	if revealed == nil || revealed.RollID != g.RollID || revealed.Kind != face.Kind || revealed.Production != face.Production || revealed.Red != 0 || revealed.Face != 0 {
		return errors.New("已揭示事件与实际抽牌不一致")
	}
	if q := g.CardEvent; q != nil {
		if s.Phase != "catan_card_event" || s.Finished || revealed.ProductionStarted || q.Kind != face.Kind || q.Production != face.Production || q.Red != 0 || q.Face != 0 || len(q.Players) == 0 {
			return errors.New("事件牌回应与抽牌记录不一致")
		}
		seen := map[int]bool{}
		for _, player := range q.Players {
			if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || seen[player] {
				return errors.New("无效事件回应队列")
			}
			seen[player] = true
		}
	} else if !revealed.ProductionStarted || s.Phase == "catan_card_event" {
		return errors.New("事件牌生产后续缺失")
	}
	return nil
}

// Called inside applyCatan's atomic clone, after seat/phase authorization.
func (s *State) catanDrawEvent() error {
	if err := s.validateCatanEventSession(); err != nil {
		return err
	}
	g := s.Catan
	if g.EventDeck == nil || s.Phase != "catan_roll" || g.setup() || g.Paired != nil && g.Paired.Second {
		return errors.New("当前不能抽取事件牌")
	}
	if g.RollID == int(^uint(0)>>1) {
		return errors.New("事件抽取次数溢出")
	}
	next := clone(*s)
	draw, err := next.Catan.EventDeck.Deck.next()
	if err != nil {
		return err
	}
	if draw.NewYear {
		next.Log = append(next.Log, "新年：重新混洗全部事件牌，保留底部五张，再揭示下一张")
	}
	face := catanEventReferenceFaces[draw.Card]
	if err := next.catanBeginCardEvent(face.Kind, face.Production, 0, 0); err != nil {
		return err
	}
	if err := next.validateCatanEventSession(); err != nil {
		return err
	}
	*s = next
	return nil
}
