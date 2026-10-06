package game

import (
	"errors"
	"fmt"
	"slices"
)

type CatanRivers struct {
	Map    *catanRiversMap `json:"map"`
	Gold   []int           `json:"gold"`
	Bank   int             `json:"bank"`
	Bought int             `json:"bought"` // Current action phase; reset once at catanNext.
}

// Internal scenario constructor. Public room selection remains unavailable
// pending complete rules, original artwork, UI and full-game acceptance.
func NewCatanRivers(n int, options CatanOptions) (*State, error) {
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	m, err := s.Catan.makeRiversMap()
	if err != nil {
		return nil, err
	}
	supply := 100
	if n > 4 {
		supply = 152
	}
	s.Catan.Rivers = &CatanRivers{Map: m, Gold: make([]int, n), Bank: supply}
	s.Phase = "catan_rivers_start"
	s.catanScores()
	return s, nil
}

func (g *Catan) validateRivers() error {
	r := g.Rivers
	if r == nil {
		return nil
	}
	if r.Map == nil || len(r.Gold) != len(g.Players) || len(g.Players) == 2 && g.Two == nil || r.Bought < 0 || r.Bought > 2 || g.BaseSetup != nil || g.Seafarers != nil || g.Fishing != nil || g.CitiesKnights != nil || g.Options.Helpers || g.Harbors != nil || g.FriendlyRobber != nil || g.CardEvent != nil || g.RevealedEvent != nil || (len(g.Players) > 4) != g.Options.FiveSix || (len(g.Players) > 4) != (g.Paired != nil) {
		return errors.New("河流状态或尚未核对的组合无效")
	}
	board := *g
	board.Robber = -1
	if err := r.Map.validate(&board); err != nil {
		return err
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) {
		return errors.New("河流强盗位置无效")
	}
	supply := 100
	if len(g.Players) > 4 {
		supply = 152
	}
	total := r.Bank
	if total < 0 || total > supply {
		return errors.New("金币库存无效")
	}
	for _, held := range r.Gold {
		if held < 0 || held > supply {
			return errors.New("玩家金币无效")
		}
		total += held
	}
	if total != supply {
		return errors.New("金币不守恒")
	}
	for _, e := range g.Edges {
		neutral := g.Two != nil && slices.Contains(catanTwoNeutralOwners[:], e.Owner)
		if e.Owner < -1 && !neutral || e.Owner >= len(g.Players) || e.Bridge && (e.Owner == -1 || !slices.Contains(r.Map.Bridges, e.ID) || e.Ship || e.Damaged) || e.Owner != -1 && slices.Contains(r.Map.Bridges, e.ID) && !e.Bridge {
			return errors.New("桥梁位置或棋子无效")
		}
	}
	for p := range g.Players {
		if g.bridgeCount(p) > 3 {
			return errors.New("桥梁数量超出库存")
		}
	}
	if g.Two != nil {
		for _, owner := range catanTwoNeutralOwners {
			if g.bridgeCount(owner) > 3 {
				return errors.New("中立桥梁数量超出库存")
			}
		}
	}
	return nil
}

func (s *State) catanRiversStart(a Action) error {
	g := s.Catan
	if g.Rivers == nil || a.Type != "catan_rivers_start" || g.SetupStep != 0 || !slices.Contains(g.Rivers.Map.Swamps, a.Tile) {
		return errors.New("请选择一处沼泽作为强盗起点")
	}
	g.Robber = a.Tile
	s.Phase = "catan_setup_settlement"
	s.catanLog(s.Turn, "选择沼泽 #%d 为强盗起点", a.Tile+1)
	return nil
}

func (g *Catan) riverVertex(v int) bool {
	if g.Rivers == nil {
		return false
	}
	for _, channel := range g.Rivers.Map.Channels {
		for _, id := range channel.Tiles {
			if slices.Contains(g.Tiles[id].Vertices, v) {
				return true
			}
		}
	}
	return false
}
func (g *Catan) riverEdge(id int) bool {
	if g.Rivers == nil || id < 0 || id >= len(g.Edges) {
		return false
	}
	for _, channel := range g.Rivers.Map.Channels {
		for _, tile := range channel.Tiles {
			if slices.Contains(g.edgeTiles(id), tile) {
				return true
			}
		}
	}
	return false
}
func (g *Catan) bridgeCount(p int) int {
	n := 0
	for _, e := range g.Edges {
		if e.Owner == p && e.Bridge {
			n++
		}
	}
	return n
}
func (g *Catan) canBridge(p, id int) bool {
	neutral := g.Two != nil && slices.Contains(catanTwoNeutralOwners[:], p)
	if g.Rivers == nil || !neutral && (p < 0 || p >= len(g.Players) || g.Players[p].Eliminated) || id < 0 || id >= len(g.Edges) || g.Edges[id].Owner != -1 || !slices.Contains(g.Rivers.Map.Bridges, id) || g.bridgeCount(p) >= 3 {
		return false
	}
	e := g.Edges[id]
	for _, v := range []int{e.A, e.B} {
		if g.opponentPiece(p, v) {
			continue
		}
		if g.Vertices[v].Level > 0 {
			if g.Vertices[v].Owner == p {
				return true
			}
			continue
		}
		for _, edge := range g.touching(v) {
			other := g.Edges[edge]
			if other.Owner == p && !other.Ship && !other.Damaged {
				return true
			}
		}
	}
	return false
}

func (s *State) catanRiverReward(p, amount int) error {
	if s.Catan.Rivers == nil || amount == 0 || p < 0 {
		return nil
	}
	r := s.Catan.Rivers
	// Guard against inventing coins while the official depleted-supply rule
	// remains unverified. This unresolved boundary blocks public release.
	if amount < 0 || r.Bank < amount {
		return errors.New("金币供应不足：此边界规则尚待核实，不能透支")
	}
	r.Bank -= amount
	r.Gold[p] += amount
	s.catanLog(p, "河流建设获得 金币×%d", amount)
	return nil
}
func (s *State) catanBuildBridge(p int, a Action) error {
	g := s.Catan
	if s.Phase != "catan_turn" || a.Skill != "" || !g.canBridge(p, a.Edge) || !catanHas(g.Players[p].Resources, catanPrices["catan_bridge"]) {
		return errors.New("桥梁须支付木材×1、砖块×2，连接己方道路或建筑，且最多3座；不能用免费道路建桥")
	}
	catanMove(g.Players[p].Resources, g.Bank, catanPrices["catan_bridge"])
	g.Edges[a.Edge].Owner, g.Edges[a.Edge].Bridge = p, true
	if err := s.catanRiverReward(p, 3); err != nil {
		return err
	}
	s.catanLog(p, "修建桥梁 #%d，支付 木材×1、砖块×2", a.Edge+1)
	return s.catanAfterRoute(CatanRouteCompletion{Player: p, Edge: a.Edge})
}

// Wealth has no incumbent tie privilege. All tied poorest players lose two
// points, and tied richest players leave the single bonus in the supply.
func (g *Catan) riverWealth() (richest int, poor []int) {
	richest = -1
	if g.Rivers == nil {
		return
	}
	high, low, ties := -1, int(^uint(0)>>1), 0
	for p, amount := range g.Rivers.Gold {
		if g.Players[p].Eliminated {
			continue
		}
		if amount > high {
			high, richest, ties = amount, p, 1
		} else if amount == high {
			ties++
		}
		if amount < low {
			low = amount
			poor = []int{p}
		} else if amount == low {
			poor = append(poor, p)
		}
	}
	if ties != 1 {
		richest = -1
	}
	return
}
func (g *Catan) riverPoints(p int) int {
	richest, poor := g.riverWealth()
	points := 0
	if richest == p {
		points++
	}
	if slices.Contains(poor, p) {
		points -= 2
	}
	return points
}
func (s *State) catanCoins(p int, a Action) error {
	g := s.Catan
	r := g.Rivers
	if r == nil || s.Phase != "catan_turn" || a.Color < 0 || a.Color >= 5 {
		return errors.New("金币交易仅可在自己掷骰后的行动阶段进行")
	}
	c := a.Color
	switch a.Type {
	case "catan_coin_buy":
		if r.Bought >= 2 || r.Gold[p] < 2 || g.Bank[c] <= 0 {
			return errors.New("每次行动最多用金币买2张资源，每张2金币，银行必须有库存")
		}
		r.Gold[p] -= 2
		r.Bank += 2
		r.Bought++
		g.Bank[c]--
		g.Players[p].Resources[c]++
		s.catanLog(p, "支付 金币×2，购买 %s×1（本次行动 %d/2）", CatanResources[c], r.Bought)
	case "catan_coin_sell":
		rate := g.rates(p)[c]
		if g.Players[p].Resources[c] < rate || r.Bank < 1 {
			return errors.New("资源或金币库存不足")
		}
		g.Players[p].Resources[c] -= rate
		g.Bank[c] += rate
		r.Bank--
		r.Gold[p]++
		s.catanLog(p, "支付 %s×%d，兑换 金币×1", CatanResources[c], rate)
	default:
		return errors.New("未知金币交易")
	}
	g.Trade = nil
	s.catanScores()
	s.catanVictory()
	return nil
}
func (g *Catan) validTradeGold(amount int) bool {
	return amount >= 0 && amount <= 152 && (g.Rivers != nil || amount == 0)
}
func (g *Catan) hasTradeGold(p, amount int) bool {
	return g.validTradeGold(amount) && (amount == 0 || g.Rivers.Gold[p] >= amount)
}
func catanTradeText(cards []int, gold int) string {
	text := catanText(cards)
	if gold == 0 {
		return text
	}
	if sum(cards) == 0 {
		return fmt.Sprintf("金币×%d", gold)
	}
	return fmt.Sprintf("%s、金币×%d", text, gold)
}

func (g *Catan) riverBotChoices(p int) []botChoice {
	if g.Rivers == nil {
		return nil
	}
	choices := []botChoice{}
	for _, edge := range g.Rivers.Map.Bridges {
		if g.canBridge(p, edge) {
			choices = append(choices, botChoice{Action{Type: "catan_bridge", Edge: edge}, 220})
		}
	}
	// Sell only when the public wealth change immediately improves our score.
	// Buying resources is handled alongside ordinary build-directed trades.
	before := g.riverPoints(p)
	copy := *g
	river := *g.Rivers
	copy.Rivers = &river
	river.Gold = slices.Clone(river.Gold)
	river.Gold[p]++
	if g.Rivers.Bank > 0 && copy.riverPoints(p) > before {
		for color, rate := range g.rates(p) {
			if g.Players[p].Resources[color] >= rate {
				choices = append(choices, botChoice{Action{Type: "catan_coin_sell", Color: color}, 310 + (copy.riverPoints(p)-before)*30})
			}
		}
	}
	return choices
}
