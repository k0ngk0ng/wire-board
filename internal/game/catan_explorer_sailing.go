package game

import (
	"errors"
	"slices"
)

const catanExplorerRules = "catan-explorers-pirates-2025"

// Independent E&P ships: three stable slots per player. These are moving
// vessels, not Seafarers roads/ships stored in CatanEdge.Owner. This private
// kernel is not a public scenario constructor or an alternative Apply path.
type catanExplorerSailing struct {
	Rules     string                     `json:"rules"`
	Positions []int                      `json:"positions"` // -1 in supply; owner = slot / 3.
	Turn      *catanExplorerMovementTurn `json:"turn,omitempty"`
}
type catanExplorerMovementTurn struct {
	Player    int                     `json:"player"`
	Sequence  uint64                  `json:"sequence"`
	Open      bool                    `json:"open"`
	Current   int                     `json:"current"` // -1 until the first ship actually moves.
	Speed     int                     `json:"speed"`   // Zero, one or two Swift Voyage farms.
	Ships     []catanExplorerShipMove `json:"ships"`
	Exploring []int                   `json:"exploring"` // Mandatory discoveries; board controller reveals them.
}
type catanExplorerShipMove struct {
	Remaining int  `json:"remaining"`
	Spent     int  `json:"spent"`
	Wool      bool `json:"wool"`
	Tribute   bool `json:"tribute"`
	Closed    bool `json:"closed"`
}
type catanExplorerSailQuote struct {
	To        int   `json:"to"`
	Points    int   `json:"points"`
	Gold      int   `json:"gold"`
	Exploring []int `json:"exploring"`
}

func newCatanExplorerSailing(players int) (*catanExplorerSailing, error) {
	if players < 2 || players > 6 {
		return nil, errors.New("探险家与海盗需要2至6位玩家")
	}
	f := &catanExplorerSailing{Rules: catanExplorerRules, Positions: make([]int, players*3)}
	for i := range f.Positions {
		f.Positions[i] = -1
	}
	return f, nil
}

// Sea edges include coastlines against the surrounding sea frame (official
// E&P FAQ "Sea Routes"). This does not invent an extra hex on which to place
// a pirate. Fish shoals and the Council use sea terrain plus mission metadata.
func catanExplorerSeaEdge(g *Catan, edge int) bool {
	if g == nil || edge < 0 || edge >= len(g.Edges) {
		return false
	}
	e := g.Edges[edge]
	if e.ID != edge || e.A < 0 || e.B < 0 || e.A == e.B || e.A >= len(g.Vertices) || e.B >= len(g.Vertices) || len(e.Tiles) < 1 || len(e.Tiles) > 2 {
		return false
	}
	sea := false
	for _, tile := range e.Tiles {
		if tile < 0 || tile >= len(g.Tiles) || g.Tiles[tile].ID != tile || !slices.Contains(g.Tiles[tile].Vertices, e.A) || !slices.Contains(g.Tiles[tile].Vertices, e.B) {
			return false
		}
		sea = sea || g.Tiles[tile].Resource == CatanSea
	}
	if len(e.Tiles) == 1 && g.Tiles[e.Tiles[0]].Resource != CatanFog {
		sea = true
	}
	return sea
}
func (f catanExplorerSailing) validate(g *Catan) error {
	if g == nil || len(g.Players) < 2 || len(g.Players) > 6 || f.Rules != catanExplorerRules || len(f.Positions) != 3*len(g.Players) {
		return errors.New("探险船只人数、规则或库存无效")
	}
	counts := map[int]int{}
	for _, edge := range f.Positions {
		if edge == -1 {
			continue
		}
		if !catanExplorerSeaEdge(g, edge) {
			return errors.New("探险船只不在海边")
		}
		counts[edge]++
		if counts[edge] > 2 {
			return errors.New("一条海边最多停靠两艘船")
		}
	}
	t := f.Turn
	if t == nil {
		return nil
	}
	if t.Player < 0 || t.Player >= len(g.Players) || t.Sequence == 0 || t.Speed < 0 || t.Speed > 2 || len(t.Ships) != len(f.Positions) || t.Current < -1 || t.Current >= len(f.Positions) || t.Current >= 0 && t.Current/3 != t.Player {
		return errors.New("探险航行回合无效")
	}
	for id, ship := range t.Ships {
		if id/3 != t.Player || f.Positions[id] < 0 {
			if ship != (catanExplorerShipMove{Closed: true}) {
				return errors.New("非行动船只不能持有移动点")
			}
			continue
		}
		budget := 4 + t.Speed
		if ship.Wool {
			budget += 2
		}
		if ship.Remaining < 0 || ship.Spent < 0 || ship.Spent > budget || ship.Closed && ship.Remaining != 0 || !ship.Closed && ship.Remaining+ship.Spent != budget || !t.Open && !ship.Closed || ship.Tribute && ship.Spent == 0 {
			return errors.New("探险船只移动点或贡金记录无效")
		}
		if ship.Spent > 0 && id != t.Current && !ship.Closed {
			return errors.New("不能交替移动多艘探险船")
		}
	}
	if t.Current >= 0 && t.Ships[t.Current].Spent == 0 {
		return errors.New("当前探险船没有实际移动记录")
	}
	for i, tile := range t.Exploring {
		if !t.Open || t.Current < 0 || tile < 0 || tile >= len(g.Tiles) || slices.Contains(t.Exploring[:i], tile) || !t.Ships[t.Current].Closed || t.Ships[t.Current].Spent == 0 || !catanExplorerTouches(g, f.Positions[t.Current], tile) {
			return errors.New("探险待揭示地块无效")
		}
	}
	return nil
}
func (f *catanExplorerSailing) begin(g *Catan, player int, sequence uint64, speed int) error {
	if err := f.validate(g); err != nil {
		return err
	}
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || sequence == 0 || speed < 0 || speed > 2 || f.Turn != nil && (f.Turn.Open || sequence <= f.Turn.Sequence) {
		return errors.New("不能重开或跳过尚未结束的航行阶段")
	}
	t := &catanExplorerMovementTurn{Player: player, Sequence: sequence, Open: true, Current: -1, Speed: speed, Ships: make([]catanExplorerShipMove, len(f.Positions)), Exploring: []int{}}
	for id, edge := range f.Positions {
		if id/3 == player && edge >= 0 {
			t.Ships[id].Remaining = 4 + speed
		} else {
			t.Ships[id].Closed = true
		}
	}
	f.Turn = t
	return nil
}
func (f catanExplorerSailing) allowed(g *Catan, player int, sequence uint64) error {
	if err := f.validate(g); err != nil {
		return err
	}
	if f.Turn == nil || !f.Turn.Open || f.Turn.Player != player || f.Turn.Sequence != sequence || g.Players[player].Eliminated || len(f.Turn.Exploring) != 0 {
		return errors.New("航行阶段、回应序号或待探索状态无效")
	}
	return nil
}
func catanExplorerTouches(g *Catan, edge, tile int) bool {
	if edge < 0 || edge >= len(g.Edges) || tile < 0 || tile >= len(g.Tiles) {
		return false
	}
	e := g.Edges[edge]
	return slices.Contains(g.Tiles[tile].Vertices, e.A) || slices.Contains(g.Tiles[tile].Vertices, e.B)
}
func catanExplorerAdjacentEdges(a, b CatanEdge) bool {
	return a.A == b.A || a.A == b.B || a.B == b.A || a.B == b.B
}

// Quote a whole path atomically. Passing an edge with two ships is legal;
// stopping there (including a mandatory discovery stop) is not. Pirate tolls
// apply to the hex's edges, while discovery uses either endpoint of the ship.
func (f catanExplorerSailing) quote(g *Catan, player int, sequence uint64, ship int, path []int, pirateOwner, pirateTile int) (catanExplorerSailQuote, error) {
	result := catanExplorerSailQuote{To: -1, Exploring: []int{}}
	if err := f.allowed(g, player, sequence); err != nil {
		return result, err
	}
	if ship < 0 || ship >= len(f.Positions) || ship/3 != player || f.Positions[ship] < 0 || f.Turn.Ships[ship].Closed || len(path) == 0 || len(path) > f.Turn.Ships[ship].Remaining {
		return result, errors.New("请选择可移动的己方船只及足够移动点内的路径")
	}
	if pirateOwner < -1 || pirateOwner >= len(g.Players) || pirateOwner == -1 && pirateTile != -1 || pirateOwner >= 0 && (pirateTile < 0 || pirateTile >= len(g.Tiles) || g.Tiles[pirateTile].Resource != CatanSea) {
		return result, errors.New("海盗位置或所有者无效")
	}
	previous := f.Positions[ship]
	for i, edge := range path {
		if edge == previous || !catanExplorerSeaEdge(g, edge) || !catanExplorerAdjacentEdges(g.Edges[previous], g.Edges[edge]) {
			return result, errors.New("船只只能沿连续海边航行")
		}
		if pirateOwner >= 0 && pirateOwner != player && !f.Turn.Ships[ship].Tribute && (slices.Contains(g.Edges[previous].Tiles, pirateTile) || slices.Contains(g.Edges[edge].Tiles, pirateTile)) {
			result.Gold = 1
		}
		for _, tile := range g.Tiles {
			if tile.Resource == CatanFog && catanExplorerTouches(g, edge, tile.ID) {
				result.Exploring = append(result.Exploring, tile.ID)
			}
		}
		if len(result.Exploring) > 0 && i != len(path)-1 {
			return result, errors.New("发现新地块后必须立即结束该船本回合的移动")
		}
		previous = edge
	}
	others := 0
	for id, edge := range f.Positions {
		if id != ship && edge == previous {
			others++
		}
	}
	if others >= 2 {
		return result, errors.New("目的海边已有两艘船，不能在此停靠")
	}
	result.To, result.Points = previous, len(path)
	return result, nil
}
func (f *catanExplorerSailing) sail(g *Catan, player int, sequence uint64, ship int, path []int, pirateOwner, pirateTile int, gold []int, goldBank *int) (catanExplorerSailQuote, error) {
	result, err := f.quote(g, player, sequence, ship, path, pirateOwner, pirateTile)
	if err != nil {
		return result, err
	}
	if len(gold) != len(g.Players) || goldBank == nil || *goldBank < 0 || slices.ContainsFunc(gold, func(n int) bool { return n < 0 }) || gold[player] < result.Gold || result.Gold > 0 && *goldBank == int(^uint(0)>>1) {
		return result, errors.New("金币不足以支付贡金，或金币库存无效")
	}
	t := f.Turn
	if t.Current >= 0 && t.Current != ship {
		t.Ships[t.Current].Closed, t.Ships[t.Current].Remaining = true, 0
	}
	t.Current = ship
	move := &t.Ships[ship]
	move.Remaining -= result.Points
	move.Spent += result.Points
	move.Tribute = move.Tribute || result.Gold > 0
	f.Positions[ship] = result.To
	gold[player] -= result.Gold
	*goldBank += result.Gold
	if len(result.Exploring) > 0 {
		move.Closed, move.Remaining = true, 0
		t.Exploring = slices.Clone(result.Exploring)
	}
	return result, nil
}
func (f *catanExplorerSailing) wool(g *Catan, player int, sequence uint64, ship int) error {
	if err := f.allowed(g, player, sequence); err != nil {
		return err
	}
	if ship < 0 || ship >= len(f.Positions) || ship/3 != player || f.Positions[ship] < 0 || f.Turn.Ships[ship].Wool || f.Turn.Ships[ship].Closed || !catanBundle(g.Bank) || !catanBundle(g.Players[player].Resources) || g.Players[player].Resources[2] < 1 {
		return errors.New("每艘未结束移动的船每回合只能付一次羊毛增加2移动点")
	}
	stock := 19
	if len(g.Players) > 4 {
		stock = 24
	}
	if g.Bank[2] >= stock {
		return errors.New("羊毛银行库存无效")
	}
	g.Players[player].Resources[2]--
	g.Bank[2]++
	f.Turn.Ships[ship].Wool = true
	f.Turn.Ships[ship].Remaining += 2
	return nil
}

// Internal mission hook: acquiring a Swift Voyage farm benefits every ship
// immediately, but does not reopen a ship already completed or stopped by fog.
func (f *catanExplorerSailing) swiftVoyage(g *Catan, player int, sequence uint64, total int) error {
	if err := f.allowed(g, player, sequence); err != nil {
		return err
	}
	if total <= f.Turn.Speed || total > 2 {
		return errors.New("快速航行奖励必须来自新增的香料农场")
	}
	for id := range f.Turn.Ships {
		if id/3 == player && !f.Turn.Ships[id].Closed {
			f.Turn.Ships[id].Remaining += total - f.Turn.Speed
		}
	}
	f.Turn.Speed = total
	return nil
}

// Called only after the board/mission controller has actually revealed all
// mandatory hexes and resolved their rewards. Never a client-facing action.
func (f *catanExplorerSailing) discovered(g *Catan, player int, sequence uint64) error {
	if err := f.validate(g); err != nil {
		return err
	}
	t := f.Turn
	if t == nil || !t.Open || t.Player != player || t.Sequence != sequence || len(t.Exploring) == 0 {
		return errors.New("没有需要完成的探索")
	}
	for _, tile := range t.Exploring {
		if g.Tiles[tile].Resource == CatanFog {
			return errors.New("必须先揭示全部新地块")
		}
	}
	t.Exploring = []int{}
	return nil
}
func (f *catanExplorerSailing) end(g *Catan, player int, sequence uint64) error {
	if err := f.allowed(g, player, sequence); err != nil {
		return err
	}
	for id := range f.Turn.Ships {
		f.Turn.Ships[id].Closed, f.Turn.Ships[id].Remaining = true, 0
	}
	f.Turn.Open = false
	return nil
}
