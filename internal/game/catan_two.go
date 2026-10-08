package game

import (
	"errors"
	"fmt"
	"slices"
)

const CatanTwoRules = "catan-for-two-2025"

type CatanTwo struct {
	Helpers         string           `json:"helpers,omitempty"`
	AfterHelper     string           `json:"afterHelper,omitempty"`
	Variants        string           `json:"variants,omitempty"`
	Knights         string           `json:"knights,omitempty"`
	Rules           string           `json:"rules"`
	Rolls           []int            `json:"rolls"`
	Sequence        int              `json:"sequence"`
	Pending         *CatanTwoPending `json:"pending,omitempty"`
	Tokens          []int            `json:"tokens"`
	Bank            int              `json:"bank"`
	TokensIssued    int              `json:"tokensIssued,omitempty"`
	Spent           bool             `json:"spent"`
	KnightExchanged bool             `json:"knightExchanged"`
	Trade           *CatanTwoTrade   `json:"trade,omitempty"`
}

// Construct the 2025 two-player variant with two neutral building colors.
func NewCatanTwo(n int, options CatanOptions) (*State, error) {
	o, err := NormalizeCatanOptions(options)
	if err != nil {
		return nil, err
	}
	if n != 2 || !CatanTwoHelpersOptions("", o) {
		return nil, errors.New("双人卡坦需要两位玩家；组合规则尚未开放")
	}
	s, err := newCatanTwoCore()
	if err != nil {
		return nil, err
	}
	return s, s.enableTwoHelpers(o)
}

type CatanTwoPending struct {
	Remaining []string `json:"remaining,omitempty"`
	Kind      string   `json:"kind"`
	Resume    string   `json:"resume"`
}

// Core construction is shared with the internal server acceptance entrypoint.
// UI, supply exhaustion and combinations still gate public two-player play.
func newCatanTwoCore() (*State, error) {
	return newCatanTwoBoard("")
}

// Two-player Rivers includes separate gold and trade-token ledgers.
func NewCatanTwoRivers(n int, options CatanOptions) (*State, error) {
	o, err := NormalizeCatanOptions(options)
	if err != nil || n != 2 || o != (CatanOptions{}) {
		return nil, errors.New("双人河流需要两位玩家，其他组合尚未开放")
	}
	return newCatanTwoBoard("rivers")
}

// Two-player Caravans uses the verified two-wagon bidding controller.
func NewCatanTwoCaravans(n int, options CatanOptions) (*State, error) {
	o, err := NormalizeCatanOptions(options)
	if err != nil || n != 2 || o != (CatanOptions{}) {
		return nil, errors.New("双人商队需要两位玩家，其他组合尚未开放")
	}
	return newCatanTwoBoard("caravans")
}

func newCatanTwoBoard(scenario string) (*State, error) {
	s := &State{Kind: "catan", Round: 1}
	s.initCatan(2)
	g := s.Catan
	g.Two = &CatanTwo{Rules: CatanTwoRules, Rolls: []int{}, Tokens: []int{5, 5}, Bank: 10}
	if scenario == "rivers" {
		m, err := g.makeRiversMap()
		if err != nil {
			return nil, err
		}
		g.Rivers = &CatanRivers{Rules: CatanRiversRules, Map: m, Gold: make([]int, 2), Bank: 100}
		s.Phase = "catan_rivers_start"
	}
	if scenario == "caravans" {
		m, err := g.makeCaravansMap()
		if err != nil {
			return nil, err
		}
		g.Caravans = &catanCaravans{Rules: CatanCaravansRules, Map: m, Wagons: []catanCaravanWagon{}}
	}
	if err := g.prepareTwoNeutrals(); err != nil {
		return nil, err
	}
	g.StartPlayer = catanRandom(2)
	s.Turn = g.StartPlayer
	s.catanScores()
	if err := g.validateRivers(); err != nil {
		return nil, err
	}
	if err := s.validateCaravans(); err != nil {
		return nil, err
	}
	return s, s.validateCatanTwo()
}

func (s *State) validateCatanTwo() error {
	g := s.Catan
	q := g.Two
	if q == nil {
		return nil
	}
	if q.Rules != CatanTwoRules {
		return errors.New("双人规则版本无效")
	}
	if err := s.validateTwoHelpers(); err != nil {
		return err
	}
	if err := s.validateTwoKnights(); err != nil {
		return err
	}
	if err := s.validateCatanTwoVariants(); err != nil {
		return err
	}
	if err := s.validateCatanTwoTokens(); err != nil {
		return err
	}
	if len(g.Players) != 2 || len(g.Tiles) != 19 || !g.twoBoardDimensions() || g.Seafarers != nil || g.CitiesKnights != nil && !g.twoKnights() || g.Caravans != nil && g.Rivers != nil || g.Fishing != nil && !g.twoFishing() || g.BaseSetup != nil || g.Paired != nil || g.EventDeck == nil && (g.CardEvent != nil || g.RevealedEvent != nil) || g.GoldPending != nil || s.Turn < 0 || s.Turn >= 2 || g.StartPlayer < 0 || g.StartPlayer >= 2 || len(q.Rolls) > 2 || q.Sequence < 0 {
		return errors.New("双人状态或尚未接入的组合无效")
	}
	for _, n := range q.Rolls {
		if n < 2 || n > 12 || g.Transport != nil && g.EventDeck == nil && (n == 2 || n == 12) {
			return errors.New("双人生产点数无效")
		}
	}
	if g.EventDeck == nil && len(q.Rolls) == 2 && q.Rolls[0] == q.Rolls[1] {
		return errors.New("双人两次生产点数必须不同")
	}
	if g.setup() && (len(q.Rolls) != 0 || q.Pending != nil || q.Sequence != 0) {
		return errors.New("双人起始状态无效")
	}
	if !s.Finished && (s.Phase == "catan_turn" || s.Phase == "catan_transport_move") && len(q.Rolls) != 2 {
		return errors.New("请先完成两次生产")
	}
	if s.Phase == "catan_roll" && len(q.Rolls) == 2 {
		return errors.New("本回合已经完成两次生产")
	}
	if q.Pending != nil {
		if len(q.Pending.Remaining) > 1 || len(q.Pending.Remaining) > 0 && (!g.twoKnights() || q.Pending.Kind != "knight_promote" || q.Pending.Remaining[0] != "knight_promote") || (q.Pending.Kind == "knight" || q.Pending.Kind == "knight_promote") && (!g.twoKnights() || q.Pending.Resume != "catan_turn") {
			return errors.New("中立骑士回应无效")
		}
		if (q.Pending.Kind == "settlement" || q.Pending.Kind == "bridge") && q.Pending.Resume != "catan_turn" || q.Pending.Kind == "bridge" && g.Rivers == nil || q.Pending.Resume == "catan_roll" && len(q.Rolls) >= 2 {
			return errors.New("中立建设返回阶段无效")
		}
		if s.Finished || g.setup() || s.Phase != "catan_two_build" || q.Sequence == 0 || !slices.Contains([]string{"road", "settlement", "bridge", "knight", "knight_promote"}, q.Pending.Kind) || !slices.Contains([]string{"catan_roll", "catan_turn", "catan_roads"}, q.Pending.Resume) || len(g.twoNeutralChoices(q.Pending.Kind)) == 0 || g.Trade != nil || q.Pending.Resume == "catan_turn" && len(q.Rolls) != 2 || q.Pending.Resume == "catan_roads" && g.FreeRoads <= 0 {
			return errors.New("双人中立建设响应无效")
		}
	} else if s.Phase == "catan_two_build" {
		return errors.New("中立建设响应缺失")
	}
	for _, v := range g.Vertices {
		if v.Owner < -3 || v.Owner > 1 || v.Level < 0 || v.Level > 2 || v.Level > 0 && v.Owner == -1 || v.Level == 0 && v.Owner != -1 || v.Owner < -1 && v.Level != 1 {
			return errors.New("双人建筑所有者无效")
		}
	}
	for _, e := range g.Edges {
		if e.Owner < -3 || e.Owner > 1 || e.Ship || e.Bridge && g.Rivers == nil || e.Damaged && (g.EventDeck == nil || e.Owner < 0) {
			return errors.New("双人道路状态无效")
		}
	}
	for _, owner := range []int{0, 1, -2, -3} {
		r, v, c := g.pieces(owner)
		if r > 15 || v-len(g.fallenCities(owner)) > 5 || c+len(g.fallenCities(owner)) > 4 || owner < 0 && (v < 1 || c != 0) {
			return errors.New("双人棋子库存无效")
		}
	}
	cards := 5
	if g.twoKnights() {
		cards = 8
	}
	if len(g.Bank) != cards {
		return errors.New("双人银行无效")
	}
	for _, p := range g.Players {
		if !g.cardBundle(p.Resources) {
			return errors.New("双人手牌无效")
		}
	}
	for color, total := range g.Bank {
		if total < 0 {
			return errors.New("双人资源供应不足")
		}
		for _, p := range g.Players {
			total += p.Resources[color]
		}
		if g.Caravans != nil && g.Caravans.Pending != nil {
			for _, bid := range g.Caravans.Pending.Bids {
				if len(bid) == 5 {
					total += bid[color]
				}
			}
		}
		want := 19
		if color >= 5 {
			want = 12
		}
		if total != want {
			return errors.New("双人资源总量不守恒")
		}
	}
	return nil
}

func (s *State) catanTwoRoll(a, b int) error {
	g := s.Catan
	q := g.Two
	if q == nil || g.CitiesKnights != nil || g.EventDeck != nil || s.Phase != "catan_roll" || q.Pending != nil || len(q.Rolls) >= 2 || a < 1 || a > 6 || b < 1 || b > 6 || len(q.Rolls) == 1 && a+b == q.Rolls[0] {
		return errors.New("请选择与第一次总点数不同的第二次生产")
	}
	g.Dice = []int{a, b}
	g.RollID++
	q.Rolls = append(q.Rolls, a+b)
	return s.catanRoll(a + b)
}

func (s *State) catanTwoAfterAction(before *State, a Action) error {
	g, q := s.Catan, s.Catan.Two
	if q == nil {
		return nil
	}
	// Includes initial settlements. Use the previous actor because completing
	// setup can advance Turn; upgrades and neutral villages earn no tokens.
	if a.Type == "catan_settlement" && !g.twoFishing() && g.Vertices[a.Vertex].Level == 1 {
		if err := s.catanTwoEarn(before.Turn, g.twoSettlementTokens(before.Turn, a.Vertex)); err != nil {
			return err
		}
	}
	if s.Finished {
		q.AfterHelper = ""
		return nil
	}
	if q.AfterHelper != "" && g.HelperPending == nil {
		kind := q.AfterHelper
		q.AfterHelper = ""
		return s.catanTwoStartBuild(kind)
	}
	// The first seven must finish discards, robber movement and theft before
	// returning here. Saving at any intervening phase keeps the first total.
	if len(q.Rolls) == 1 && s.Phase == "catan_turn" {
		s.Phase = "catan_roll"
	}
	if s.catanTwoKnightAfterAction(before, a) {
		return nil
	}
	if before.Catan.setup() || (a.Type != "catan_road" && a.Type != "catan_settlement" && a.Type != "catan_bridge" && a.Type != "catan_fish_road") {
		return nil
	}
	kind := "road"
	if a.Type == "catan_settlement" {
		kind = "settlement"
	}
	if a.Type == "catan_bridge" {
		kind = "bridge"
	}
	if g.HelperPending != nil {
		q.AfterHelper = kind
		return nil
	}
	return s.catanTwoStartBuild(kind)
}

func (s *State) catanTwoStartBuild(kind string) error {
	g, q := s.Catan, s.Catan.Two
	if len(g.twoNeutralChoices(kind)) == 0 {
		s.Log = append(s.Log, "两家中立势力均无合法建设位置，本次无需额外建设")
		return nil
	}
	q.Sequence++
	q.Pending = &CatanTwoPending{Kind: kind, Resume: s.Phase}
	g.Trade = nil
	s.Phase = "catan_two_build"
	s.catanLog(s.Turn, "请为一家中立势力完成额外建设")
	return nil
}

func (s *State) catanTwoBuild(player int, a Action) error {
	g, q := s.Catan, s.Catan.Two
	if q == nil || q.Pending == nil || s.Phase != "catan_two_build" || player != s.Turn || a.Type != "catan_two_build" || a.Target < 0 || a.Target > 1 {
		return errors.New("请由当前玩家完成中立建设")
	}
	choice := catanTwoNeutralChoice{Owner: catanTwoNeutralOwners[a.Target], Vertex: a.Vertex, Edge: a.Edge}
	if err := g.placeTwoNeutral(q.Pending.Kind, choice); err != nil {
		return err
	}
	resume, remaining := q.Pending.Resume, slices.Clone(q.Pending.Remaining)
	kind := q.Pending.Kind
	s.Phase = resume
	q.Pending = nil
	what, at := "道路", choice.Edge
	if choice.Vertex >= 0 {
		what, at = "村庄", choice.Vertex
		if kind == "knight" {
			what = "一级骑士"
		}
		if kind == "knight_promote" {
			what = "二级骑士"
		}
	} else if g.Edges[choice.Edge].Bridge {
		what = "桥梁"
	}
	s.catanLog(player, "为中立势力 %d 建造%s #%d", a.Target+1, what, at+1)
	s.catanTwoQueueBuild(remaining, resume)
	s.catanScores()
	if q.Pending == nil {
		s.catanVictory()
	}
	return nil
}

func (s *State) catanTwoBot(player int) (Action, error) {
	g := s.Catan
	if g.Two == nil || g.Two.Pending == nil || player != s.Turn {
		return Action{}, errors.New("inactive neutral builder")
	}
	choices := g.twoNeutralChoices(g.Two.Pending.Kind)
	if len(choices) == 0 {
		return Action{}, errors.New("no neutral placement")
	}
	best, score := choices[0], -1<<30
	for _, choice := range choices {
		trial := clone(*g)
		if err := trial.placeTwoNeutral(g.Two.Pending.Kind, choice); err != nil {
			return Action{}, err
		}
		value := trial.roadLength(player)*8 - trial.roadLength(1-player)*6
		for _, v := range trial.Vertices {
			if v.Level == 0 && g.canSettlement(player, v.ID, false) && !trial.canSettlement(player, v.ID, false) {
				value -= 40
			}
			if v.Level == 0 && g.canSettlement(1-player, v.ID, false) && !trial.canSettlement(1-player, v.ID, false) {
				value += 30
			}
		}
		if value > score {
			best, score = choice, value
		}
	}
	return Action{Type: "catan_two_build", Target: -best.Owner - 2, Vertex: best.Vertex, Edge: best.Edge}, nil
}

func catanTwoOwnerName(owner int) string { return fmt.Sprintf("中立势力 %d", -owner-1) }
