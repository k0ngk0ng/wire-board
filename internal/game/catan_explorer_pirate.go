package game

import (
	"errors"
	"slices"
)

// E&P 2025 pp8,10–11. Separate from Seafarers' pirate and the moving fleet.
// Private controller: public scenario/Apply integration awaits the lair mission.
// The outer dispatcher must suspend ordinary actions while Pending != nil.
type catanExplorerPirate struct {
	Owner         int                         `json:"owner"`
	Tile          int                         `json:"tile"`
	Pending       *catanExplorerPiratePending `json:"pending,omitempty"`
	ChaseSequence uint64                      `json:"chaseSequence,omitempty"`
	Attempted     []int                       `json:"attempted,omitempty"`
	LastChase     *catanExplorerChase         `json:"lastChase,omitempty"`
}
type catanExplorerPiratePending struct {
	Player   int    `json:"player"`
	Sequence uint64 `json:"sequence"`
	Stage    string `json:"stage"`  // place, steal
	Resume   string `json:"resume"` // action after seven, movement after successful chase
}
type catanExplorerChase struct {
	Player   int    `json:"player"`
	Ship     int    `json:"ship"`
	Sequence uint64 `json:"sequence"`
	Die      int    `json:"die"`
	Success  bool   `json:"success"`
	Bonus    []int  `json:"bonus,omitempty"` // Farm die faces owned when this roll happened.
}

// Resource composition must only be shown to the two involved players. Public
// animation/logs may disclose Target and whether a card or coin changed hands.
type catanExplorerTheft struct {
	Target   int
	Resource int // -1 for coin/no theft
	Gold     bool
}

func newCatanExplorerPirate() *catanExplorerPirate {
	return &catanExplorerPirate{Owner: -1, Tile: -1}
}
func catanExplorerPirateTile(g *Catan, b *catanExplorerBoard, tile int) bool {
	if tile < 0 || tile >= len(g.Tiles) || tile == b.FrameSea || g.Tiles[tile].Resource != CatanSea {
		return false
	}
	for _, edge := range g.Edges {
		if slices.Contains(edge.Tiles, tile) {
			for _, other := range edge.Tiles {
				if slices.Contains(b.Starting, other) {
					return false
				}
			}
		}
	}
	return true
}
func (p catanExplorerPirate) destinations(g *Catan, b *catanExplorerBoard) []int {
	result := []int{}
	for _, tile := range g.Tiles {
		if tile.ID != p.Tile && catanExplorerPirateTile(g, b, tile.ID) {
			result = append(result, tile.ID)
		}
	}
	return result
}
func (p catanExplorerPirate) victims(g *Catan, f *catanExplorerSailing, e *catanExplorerEconomy) []int {
	result := []int{}
	if p.Owner < 0 {
		return result
	}
	for ship, edge := range f.Positions {
		owner := ship / 3
		if edge >= 0 && owner != p.Owner && !g.Players[owner].Eliminated && (sum(g.Players[owner].Resources) > 0 || e.Gold[owner] > 0) && slices.Contains(g.Edges[edge].Tiles, p.Tile) && !slices.Contains(result, owner) {
			result = append(result, owner)
		}
	}
	return result
}
func (p catanExplorerPirate) validate(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy) error {
	if g == nil || b == nil || f == nil || c == nil || e == nil || !catanExplorerPirateScenario(b.Scenario) || c.Scenario != b.Scenario {
		return errors.New("海盗船需要探险家海盗任务组件")
	}
	if err := b.validate(g); err != nil {
		return err
	}
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	if p.Owner < -1 || p.Owner >= len(g.Players) || p.Owner == -1 && p.Tile != -1 || p.Owner >= 0 && (!catanExplorerPirateTile(g, b, p.Tile) || g.Players[p.Owner].Eliminated) {
		return errors.New("海盗船所有者或海格无效")
	}
	if p.ChaseSequence > 0 && (e.Turn == nil || p.ChaseSequence > e.Turn.Sequence) || p.ChaseSequence == 0 && len(p.Attempted) > 0 {
		return errors.New("驱赶记录的回合无效")
	}
	for i, ship := range p.Attempted {
		if ship < 0 || ship >= len(f.Positions) || slices.Contains(p.Attempted[:i], ship) || p.LastChase == nil || ship/3 != p.LastChase.Player {
			return errors.New("驱赶尝试船只无效")
		}
	}
	if last := p.LastChase; last != nil {
		if last.Sequence != p.ChaseSequence || e.Turn != nil && last.Sequence == e.Turn.Sequence && last.Player != e.Turn.Player || last.Player < 0 || last.Player >= len(g.Players) || last.Ship/3 != last.Player || !slices.Contains(p.Attempted, last.Ship) || last.Die < 1 || last.Die > 6 || last.Success != (last.Die == 6 || slices.Contains(last.Bonus, last.Die)) {
			return errors.New("驱赶骰子记录无效")
		}
		owned := c.pirateFarmDice(b, last.Player)
		for i, die := range last.Bonus {
			if !slices.Contains(owned, die) || slices.Contains(last.Bonus[:i], die) {
				return errors.New("驱赶奖励骰面缺少农场来源")
			}
		}
	} else if p.ChaseSequence != 0 {
		return errors.New("缺少驱赶骰子记录")
	}
	if q := p.Pending; q != nil {
		if e.Turn == nil || q.Player != e.Turn.Player || q.Sequence != e.Turn.Sequence || q.Stage != "place" && q.Stage != "steal" || q.Resume != "action" && q.Resume != "movement" {
			return errors.New("海盗回应阶段或序号无效")
		}
		if q.Resume == "action" && e.Turn.Phase != "pirate" || q.Resume == "movement" && (e.Turn.Phase != "ready" || c.Turn == nil || c.Turn.Phase != "movement" || p.LastChase == nil || !p.LastChase.Success || p.LastChase.Sequence != q.Sequence || p.LastChase.Player != q.Player) {
			return errors.New("海盗回应接续阶段无效")
		}
		if q.Stage == "place" && q.Resume == "movement" && (p.Owner < 0 || p.Owner == q.Player) {
			return errors.New("驱赶成功后的待替换海盗所有者无效")
		}
		if q.Stage == "steal" && (p.Owner != q.Player || len(p.victims(g, f, e)) == 0) {
			return errors.New("海盗偷取对象无效")
		}
		if q.Stage == "place" && len(p.destinations(g, b)) == 0 {
			return errors.New("没有可放置海盗的海格")
		}
	}
	return nil
}

// All methods are transactions over board/economy/pirate state. Rejected
// choices cannot leave a replaced pirate, consumed roll, stolen card or clock
// transition behind. randN is supplied only by the trusted controller/tests.
func (p *catanExplorerPirate) apply(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, player int, sequence uint64, kind string, target int, declineGold bool, randN func(int) int) (catanExplorerTheft, error) {
	none := catanExplorerTheft{Target: -1, Resource: -1}
	if err := p.validate(g, b, f, c, e); err != nil {
		return none, err
	}
	if e.Turn == nil || e.Turn.Player != player || e.Turn.Sequence != sequence || player < 0 || player >= len(g.Players) || g.Players[player].Eliminated {
		return none, errors.New("不是海盗行动玩家或回应序号")
	}
	base := *g
	base.Explorer = nil
	nextG, nextF, nextC, nextE, nextP := clone(base), clone(*f), clone(*c), clone(*e), clone(*p)
	result, err := nextP.applyUnchecked(&nextG, b, &nextF, &nextC, &nextE, player, sequence, kind, target, declineGold, randN)
	if err != nil {
		return none, err
	}
	if err = nextP.validate(&nextG, b, &nextF, &nextC, &nextE); err != nil {
		return none, err
	}
	// Preserve the aggregate identity when this controller is integrated later.
	nextG.Explorer = g.Explorer
	*g, *f, *c, *e, *p = nextG, nextF, nextC, nextE, nextP
	return result, nil
}
func (p *catanExplorerPirate) finish(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy) error {
	q := p.Pending
	if q.Resume == "action" {
		if err := c.beginAction(g, f, q.Player, q.Sequence); err != nil {
			return err
		}
		e.Turn.Phase = "ready"
	}
	p.Pending = nil
	return nil
}
func (p catanExplorerPirate) battleReady(g *Catan, f *catanExplorerSailing, player int, sequence uint64, ship int) bool {
	return p.Pending == nil && p.Owner >= 0 && p.Owner != player && f.Turn != nil && f.Turn.Open && f.Turn.Player == player && f.Turn.Sequence == sequence && ship >= 0 && ship < len(f.Positions) && ship/3 == player && f.Positions[ship] >= 0 && f.Turn.Ships[ship].Spent == 0 && (p.ChaseSequence != sequence || !slices.Contains(p.Attempted, ship)) && catanExplorerTouches(g, f.Positions[ship], p.Tile)
}
func (p *catanExplorerPirate) applyUnchecked(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, player int, sequence uint64, kind string, target int, declineGold bool, randN func(int) int) (catanExplorerTheft, error) {
	result := catanExplorerTheft{Target: -1, Resource: -1}
	switch kind {
	case "seven":
		if p.Pending != nil || e.Turn.Phase != "pirate" {
			return result, errors.New("尚未完成七点弃牌或已开始海盗回应")
		}
		p.Pending = &catanExplorerPiratePending{Player: player, Sequence: sequence, Stage: "place", Resume: "action"}
	case "place":
		if p.Pending == nil || p.Pending.Stage != "place" || !slices.Contains(p.destinations(g, b), target) {
			return result, errors.New("请选择不同于原位置且不邻起始岛的海格")
		}
		p.Owner, p.Tile = player, target
		c.removeShoalFish(target)
		if len(p.victims(g, f, e)) == 0 {
			return result, p.finish(g, f, c, e)
		}
		p.Pending.Stage = "steal"
	case "steal":
		if p.Pending == nil || p.Pending.Stage != "steal" || !slices.Contains(p.victims(g, f, e), target) {
			return result, errors.New("请选择在海盗格边上有船的对手")
		}
		hand := g.Players[target].Resources
		if sum(hand) > 0 {
			if declineGold || randN == nil {
				return result, errors.New("有资源的对手须随机偷取一张资源")
			}
			card := randN(sum(hand))
			if card < 0 || card >= sum(hand) {
				return result, errors.New("随机偷取结果无效")
			}
			for color, n := range hand {
				if card < n {
					hand[color]--
					g.Players[player].Resources[color]++
					result = catanExplorerTheft{Target: target, Resource: color}
					break
				}
				card -= n
			}
		} else if !declineGold {
			e.Gold[target]--
			e.Gold[player]++
			result = catanExplorerTheft{Target: target, Resource: -1, Gold: true}
		}
		return result, p.finish(g, f, c, e)
	case "chase":
		if e.Turn.Phase != "ready" || c.Turn == nil || c.Turn.Phase != "movement" || !p.battleReady(g, f, player, sequence, target) || randN == nil {
			return result, errors.New("此船本回合不能驱赶海盗")
		}
		die := randN(6) + 1
		if die < 1 || die > 6 {
			return result, errors.New("驱赶骰子无效")
		}
		if p.ChaseSequence != sequence {
			p.Attempted = nil
			p.ChaseSequence = sequence
		}
		p.Attempted = append(p.Attempted, target)
		bonus := c.pirateFarmDice(b, player)
		p.LastChase = &catanExplorerChase{Player: player, Ship: target, Sequence: sequence, Die: die, Success: die == 6 || slices.Contains(bonus, die), Bonus: bonus}
		if p.LastChase.Success {
			p.Pending = &catanExplorerPiratePending{Player: player, Sequence: sequence, Stage: "place", Resume: "movement"}
		}
	default:
		return result, errors.New("未知的探险海盗行动")
	}
	return result, nil
}
