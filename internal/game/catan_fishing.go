package game

import (
	"errors"
	"slices"
)

const CatanFishingRules = "catan-fishing-2025"

type CatanFishing struct {
	Attack     string                  `json:"attack,omitempty"`
	Caravans   string                  `json:"caravans,omitempty"`
	Rivers     string                  `json:"rivers,omitempty"`
	TwoKnights string                  `json:"twoKnights,omitempty"`
	TwoSea     string                  `json:"twoSea,omitempty"`
	Two        string                  `json:"two,omitempty"`
	SeaKnights string                  `json:"seaKnights,omitempty"`
	Helpers    string                  `json:"helpers,omitempty"`
	Explorer   string                  `json:"explorer,omitempty"`
	WorldSetup *CatanFishingWorldSetup `json:"worldSetup,omitempty"`
	Map        catanFishingMap         `json:"map"`
	Tokens     catanFishingTokens      `json:"tokens"`
	LastRollID int                     `json:"lastRollId"`
	Started    []bool                  `json:"started"`
	Pending    *CatanFishingPending    `json:"pending,omitempty"`
}

type CatanFishingPending struct {
	Resume   string `json:"resume"`
	Received []int  `json:"received,omitempty"`
	// Gold is computed once with ordinary production, then offered after all
	// fish responses. Never start two response queues at the same time.
	Gold []int `json:"gold,omitempty"`
}

// Standalone Fishing supports three to six players. Sea and combined recipes
// retain their own configuration and component acceptance gates.
func NewCatanFishing(n int, options CatanOptions) (*State, error) {
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	m, err := s.Catan.makeFishingMap()
	if err != nil {
		return nil, err
	}
	tokens, err := newCatanFishingTokens(n)
	if err != nil {
		return nil, err
	}
	s.Catan.Fishing = &CatanFishing{Map: *m, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	s.enableFishingHelpers()
	if m.NumberRecipe != "" {
		s.Log = append(s.Log, "本站数字配置：五六人沿逆时针螺旋使用固定数列，跳过湖泊；数字数量不变，不宣称对应 2025 实体字母背面")
	}
	s.Log = append(s.Log, "捕鱼：海岸渔场和湖泊产鱼，强盗从场外开始；鱼筹码面值仅本人可见")
	return s, nil
}

func (g *Catan) validateFishing() error {
	f := g.Fishing
	if f == nil {
		return nil
	}
	if g.Explorer != nil {
		return g.Explorer.validateFishing(g)
	}
	if (f.Two != "" && !g.twoFishing()) || (g.Two != nil && !g.twoFishing()) || (len(g.Players) == 2 && !g.twoFishing()) {
		return errors.New("双人渔夫规则标记无效")
	}
	if (f.TwoSea != "" || g.Two != nil && g.Seafarers != nil) && !g.twoFishingSeafarers() {
		return errors.New("双人捕鱼海图规则标记无效")
	}
	if (f.TwoKnights != "" || g.Two != nil && g.CitiesKnights != nil) && !g.twoFishingKnights() {
		return errors.New("双人渔夫骑士规则标记无效")
	}
	if f.Explorer != "" {
		return errors.New("非探险存档不能含探险鱼筹码")
	}
	if err := g.validateFishingSeaKnights(); err != nil {
		return err
	}
	if g.Seafarers != nil && !g.fishingSeaSupported() {
		return errors.New("此捕鱼扩展组合尚未接入")
	}
	if (g.Options.Helpers && (!g.fishingHelpers() || g.CitiesKnights != nil && !g.cityHelpers())) || (!g.Options.Helpers && f.Helpers != "") {
		return errors.New("渔夫助手规则与配置不符")
	}
	if len(f.Started) != len(g.Players) || len(f.Tokens.Hands) != len(g.Players) || f.LastRollID < -1 || f.LastRollID > g.RollID {
		return errors.New("invalid fishing turn state")
	}
	if f.WorldSetup != nil && (g.Seafarers == nil || g.Seafarers.Scenario != "new_world") {
		return errors.New("非新世界不能包含渔场轮流放置状态")
	}
	if err := f.Map.validate(g); err != nil {
		return err
	}
	if err := f.Tokens.validate(); err != nil {
		return err
	}
	if (f.Pending != nil) != (len(f.Tokens.Pending) > 0) {
		return errors.New("fishing continuation and token queue differ")
	}
	if q := f.Pending; q != nil {
		if q.Resume != "catan_turn" && q.Resume != "catan_setup_road" || q.Resume == "catan_turn" && len(q.Received) != len(g.Players) || q.Resume == "catan_setup_road" && q.Received != nil {
			return errors.New("invalid fishing continuation")
		}
		if g.GoldPending != nil || q.Gold != nil && (len(q.Gold) != len(g.Players) || g.Seafarers == nil) {
			return errors.New("invalid fishing gold continuation")
		}
		for _, count := range append(slices.Clone(q.Received), q.Gold...) {
			if count < 0 {
				return errors.New("negative fishing production continuation")
			}
		}
	}
	return nil
}

func (s *State) catanStartFishing(due, received []int, resume string) error {
	return s.catanStartFishingGold(due, received, nil, resume)
}

func (s *State) catanStartFishingGold(due, received, gold []int, resume string) error {
	f := s.Catan.Fishing
	before := f.Tokens.copy()
	if err := f.Tokens.beginDraw(s.Turn, due); err != nil {
		return err
	}
	s.catanLogFishDraws(before)
	if len(f.Tokens.Pending) > 0 {
		f.Pending = &CatanFishingPending{Resume: resume, Received: slices.Clone(received)}
		if sum(gold) > 0 {
			f.Pending.Gold = slices.Clone(gold)
		}
		s.Phase = "catan_fish_replace"
	} else {
		s.catanFinishFishing(received, gold, resume)
	}
	return nil
}

func (s *State) catanFinishFishing(received, gold []int, resume string) {
	s.Phase = resume
	if sum(gold) > 0 {
		s.catanStartGold(gold, received, resume)
	} else if received != nil {
		s.catanAfterProduction(received)
	}
}

func (s *State) catanStartingFish(player, vertex int, gold []int) error {
	g, f := s.Catan, s.Catan.Fishing
	if f == nil {
		s.catanFinishFishing(nil, gold, "catan_setup_road")
		return nil
	}
	if f.Started[player] {
		return errors.New("起始捕鱼已经结算")
	}
	if g.twoFishing() {
		f.Started[player] = true
		s.catanFinishFishing(nil, gold, "catan_setup_road")
		return nil
	}
	due := make([]int, len(g.Players))
	// The printed setup instruction awards "a random fish token" if the
	// second settlement adjoins a lake or ground (not city production).
	for _, lake := range f.Map.Lakes {
		if slices.Contains(g.Tiles[lake.Tile].Vertices, vertex) {
			due[player] = 1
		}
	}
	for _, ground := range f.Map.Grounds {
		if slices.Contains(ground.Vertices[:], vertex) {
			due[player] = 1
		}
	}
	if err := s.catanStartFishingGold(due, nil, gold, "catan_setup_road"); err != nil {
		return err
	}
	f.Started[player] = true
	return nil
}

func (s *State) catanLogFishDraws(before catanFishingTokens) {
	f := s.Catan.Fishing
	for p, hand := range f.Tokens.Hands {
		if count := len(hand) - len(before.Hands[p]); count > 0 {
			s.catanLog(p, "领取 %d 枚鱼筹码（面值保密）", count)
		}
	}
	if f.Tokens.BootOwner != before.BootOwner && f.Tokens.BootOwner >= 0 {
		s.catanLog(f.Tokens.BootOwner, "抽到旧靴子，获胜需要额外 1 分")
	}
}

func (s *State) catanReplaceFish(player int, a Action) error {
	f := s.Catan.Fishing
	if f == nil || f.Pending == nil || s.Phase != "catan_fish_replace" || s.CatanPendingActor() != player {
		return errors.New("请等待对应玩家选择是否更换鱼筹码")
	}
	var id *int
	switch a.Type {
	case "catan_fish_replace":
		id = &a.Card
	case "catan_fish_keep":
	default:
		return errors.New("请选择更换一枚鱼筹码或保留现有筹码")
	}
	before := f.Tokens.copy()
	if err := f.Tokens.replace(player, id); err != nil {
		return err
	}
	if id == nil {
		s.catanLog(player, "保留现有鱼筹码，放弃本次其余领取")
	} else {
		s.catanLog(player, "弃置一枚 %d 鱼筹码并盲抽替换，停止本次领取", catanFishValue(*id))
	}
	s.catanLogFishDraws(before)
	if len(f.Tokens.Pending) == 0 {
		q := f.Pending
		f.Pending = nil
		s.catanFinishFishing(q.Received, q.Gold, q.Resume)
	}
	return nil
}

func (s *State) catanFishBot(player int) (Action, error) {
	f := s.Catan.Fishing
	if f == nil || s.Phase != "catan_fish_replace" || s.CatanPendingActor() != player {
		return Action{}, errors.New("inactive fishing responder")
	}
	// Only own faces: replace a one-fish token, retain a hand of 2s/3s.
	for _, id := range f.Tokens.Hands[player] {
		if catanFishValue(id) == 1 {
			return Action{Type: "catan_fish_replace", Card: id}, nil
		}
	}
	return Action{Type: "catan_fish_keep"}, nil
}

func (g *Catan) victoryTargetFor(player int) int {
	goal := g.victoryTarget()
	if g.Fishing != nil && g.Fishing.Tokens.BootOwner == player {
		goal++
	}
	return goal
}

// Platform removal is outside printed game rules: return fish faceup and
// shuffle the boot back into the supply, so it cannot stay on an absent seat.
func (s *State) catanReturnFishing(player int) {
	f := s.Catan.Fishing
	if f == nil {
		return
	}
	f.Tokens.Discard = append(f.Tokens.Discard, f.Tokens.Hands[player]...)
	f.Tokens.Hands[player] = []int{}
	if f.Tokens.BootOwner == player {
		f.Tokens.BootOwner = -1
		f.Tokens.DrawPile = append(f.Tokens.DrawPile, catanFishBoot)
		shuffle(f.Tokens.DrawPile)
	}
	s.catanLog(player, "离场鱼筹码归还供应，旧靴子如有则重新洗入")
}

func (s *State) catanFishingView(v map[string]any, player int) {
	delete(v, "fishing") // Never fall back to the save representation.
	f := s.Catan.Fishing
	if f == nil {
		return
	}
	public, err := f.Tokens.view(player)
	if err != nil {
		return
	}
	targets := make([]int, len(s.Catan.Players))
	for p := range targets {
		targets[p] = s.Catan.victoryTargetFor(p)
	}
	v["fishing"] = map[string]any{"map": clone(f.Map), "tokens": public, "victoryTargets": targets,
		"legal": s.catanFishLegal(player), "canReplace": !s.Finished && s.Phase == "catan_fish_replace" && s.CatanPendingActor() == player}
	if f.Attack != "" {
		v["fishing"].(map[string]any)["attack"] = f.Attack
	}
	if f.Caravans != "" {
		v["fishing"].(map[string]any)["caravans"] = f.Caravans
	}
	if f.Rivers != "" {
		v["fishing"].(map[string]any)["rivers"] = f.Rivers
	}
	if f.Two != "" {
		v["fishing"].(map[string]any)["two"] = f.Two
	}
	if f.TwoKnights != "" {
		v["fishing"].(map[string]any)["twoKnights"] = f.TwoKnights
	}
	if f.TwoSea != "" {
		v["fishing"].(map[string]any)["twoSea"] = f.TwoSea
	}
	if f.SeaKnights != "" {
		v["fishing"].(map[string]any)["seaKnights"] = f.SeaKnights
	}
	if f.Helpers != "" {
		v["fishing"].(map[string]any)["helpers"] = f.Helpers
	}
	if q := f.WorldSetup; q != nil {
		setup := map[string]any{"index": q.Index, "total": len(q.Numbers), "remaining": len(q.Numbers) - q.Index}
		if s.Phase == "catan_world_fish" && q.Index >= 0 && q.Index < len(q.Numbers) {
			setup["current"] = q.Numbers[q.Index]
		}
		v["fishing"].(map[string]any)["worldSetup"] = setup
	}
}
