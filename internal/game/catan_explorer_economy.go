package game

import (
	"errors"
	"slices"
)

// 2025 English rulebook p4: 40 one-gold and 36 three-gold coins. Store
// denominations as fungible value; exchanging change cannot mint gold.
// Depleted-gold rules remain a separate rule-source gate.
const catanExplorerGoldSupply = 40 + 36*3

type catanExplorerEconomy struct {
	Gold     []int                     `json:"gold"`
	GoldBank int                       `json:"goldBank"`
	Turn     *catanExplorerEconomyTurn `json:"turn,omitempty"`
}

type catanExplorerEconomyTurn struct {
	Player   int    `json:"player"`
	Sequence uint64 `json:"sequence"`
	Phase    string `json:"phase"` // roll, discard, pirate, ready, abandoned (trusted platform removal only).
	Dice     [2]int `json:"dice"`
	Bought   int    `json:"bought"`
	Discard  []int  `json:"discard"`
}

type catanExplorerProduction struct {
	Resources [][]int `json:"resources"`
	Gold      []int   `json:"gold"`
	Discard   []int   `json:"discard"`
}

func newCatanExplorerEconomy(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo) (*catanExplorerEconomy, error) {
	if g == nil || c == nil || c.Turn != nil {
		return nil, errors.New("金币系统只能随探险组件开局初始化")
	}
	e := &catanExplorerEconomy{Gold: make([]int, len(g.Players)), GoldBank: catanExplorerStock(len(g.Players)).gold - 2*len(g.Players)}
	for p := range e.Gold {
		e.Gold[p] = 2
	}
	return e, e.validate(g, f, c)
}

func (e catanExplorerEconomy) validate(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo) error {
	if g == nil || c == nil || len(g.Players) < 2 || len(g.Players) > 6 || len(e.Gold) != len(g.Players) || e.GoldBank < 0 || e.GoldBank > catanExplorerStock(len(g.Players)).gold {
		return errors.New("探险经济人数或金币库存无效")
	}
	if err := c.validate(g, f); err != nil {
		return err
	}
	if !catanExplorerCanPay(g, 0, []int{0, 0, 0, 0, 0}) {
		return errors.New("探险资源银行与手牌不守恒")
	}
	total := e.GoldBank
	for _, gold := range e.Gold {
		if gold < 0 || gold > catanExplorerStock(len(g.Players)).gold {
			return errors.New("探险玩家金币数无效")
		}
		total += gold
	}
	if total != catanExplorerStock(len(g.Players)).gold {
		return errors.New("探险金币总值不守恒")
	}
	t := e.Turn
	if t == nil {
		if c.Turn != nil {
			return errors.New("探险经济回合缺失")
		}
		for _, gold := range e.Gold {
			if gold != 2 {
				return errors.New("尚未开始生产回合时每人应有2枚初始金币")
			}
		}
		return nil
	}
	if t.Player < 0 || t.Player >= len(g.Players) || g.Players[t.Player].Eliminated != (t.Phase == "abandoned") || t.Sequence == 0 || !slices.Contains([]string{"roll", "discard", "pirate", "ready", "abandoned"}, t.Phase) || t.Bought < 0 || t.Bought > 2 || t.Phase != "ready" && t.Phase != "abandoned" && t.Bought != 0 || len(t.Discard) != len(g.Players) {
		return errors.New("探险生产阶段、玩家或购买次数无效")
	}
	if t.Phase == "roll" || t.Phase == "abandoned" && t.Dice == [2]int{} {
		if t.Dice != [2]int{} {
			return errors.New("尚未掷骰不能已有生产点数")
		}
	} else if t.Dice[0] < 1 || t.Dice[0] > 6 || t.Dice[1] < 1 || t.Dice[1] > 6 {
		return errors.New("生产骰子点数无效")
	}
	seven := t.Dice[0]+t.Dice[1] == 7
	pending := false
	for p, amount := range t.Discard {
		if amount < 0 || amount > 0 && (g.Players[p].Eliminated || sum(g.Players[p].Resources) <= 7 || amount != sum(g.Players[p].Resources)/2) {
			return errors.New("七点待弃牌数无效")
		}
		pending = pending || amount > 0
	}
	if (t.Phase == "discard") != pending || (t.Phase == "discard" || t.Phase == "pirate") && !seven || t.Phase == "pirate" && !catanExplorerPirateScenario(c.Scenario) {
		return errors.New("探险七点响应与剧本不一致")
	}
	if t.Phase == "ready" || t.Phase == "abandoned" {
		if c.Turn == nil || c.Turn.Player != t.Player || c.Turn.Sequence != t.Sequence {
			return errors.New("经济与建设航行回合不一致")
		}
		if t.Phase == "abandoned" && c.Turn.Phase != "ended" {
			return errors.New("离场玩家的航行尚未结束")
		}
	} else {
		if c.Turn == nil && t.Sequence != 1 || c.Turn != nil && (c.Turn.Phase != "ended" || c.Turn.Sequence == ^uint64(0) || c.Turn.Sequence+1 != t.Sequence) {
			return errors.New("生产尚未完成，不能跳回合或开始建设航行")
		}
	}
	return nil
}

// Trusted round controller chooses player/sequence and supplies actual dice.
// Neither this method nor resolveProduction is a client action dispatcher.
func (e *catanExplorerEconomy) beginProduction(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64) error {
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	previous := uint64(0)
	if e.Turn != nil {
		if e.Turn.Phase != "ready" && e.Turn.Phase != "abandoned" || c.Turn.Phase != "ended" {
			return errors.New("上一回合生产、响应或航行尚未完成")
		}
		previous = e.Turn.Sequence
	}
	if player < 0 || player >= len(g.Players) || g.Players[player].Eliminated || sequence == 0 || sequence != previous+1 {
		return errors.New("探险生产回合序号或玩家无效")
	}
	e.Turn = &catanExplorerEconomyTurn{Player: player, Sequence: sequence, Phase: "roll", Discard: make([]int, len(g.Players))}
	return nil
}

func (e catanExplorerEconomy) productionAllowed(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64, phase string) error {
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	if e.Turn == nil || e.Turn.Player != player || e.Turn.Sequence != sequence || e.Turn.Phase != phase {
		return errors.New("不是当前探险生产玩家、阶段或回应序号")
	}
	return nil
}

func (e *catanExplorerEconomy) resolveProduction(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64, dice [2]int) (catanExplorerProduction, error) {
	result := catanExplorerProduction{}
	if err := e.productionAllowed(g, f, c, player, sequence, "roll"); err != nil {
		return result, err
	}
	if dice[0] < 1 || dice[0] > 6 || dice[1] < 1 || dice[1] > 6 {
		return result, errors.New("请选择有效的生产骰子")
	}
	number, n := dice[0]+dice[1], len(g.Players)
	result = catanExplorerProduction{Resources: make([][]int, n), Gold: make([]int, n), Discard: make([]int, n)}
	for p := range result.Resources {
		result.Resources[p] = make([]int, 5)
	}
	phase := "ready"
	if number == 7 {
		if catanExplorerPirateScenario(c.Scenario) {
			phase = "pirate" // Activation/stealing controller still to be installed.
		}
		for p, hand := range g.Players {
			if !hand.Eliminated && sum(hand.Resources) > 7 {
				result.Discard[p] = sum(hand.Resources) / 2
				phase = "discard"
			}
		}
	} else {
		for _, tile := range g.Tiles {
			if tile.Number != number || tile.Resource < 0 || tile.Resource > CatanGold || tile.Resource >= CatanDesert && tile.Resource != CatanGold {
				continue
			}
			for _, vertex := range tile.Vertices {
				if vertex < 0 || vertex >= len(g.Vertices) {
					return catanExplorerProduction{}, errors.New("生产地块建筑位置无效")
				}
				v := g.Vertices[vertex]
				if v.Owner < 0 || v.Level == 0 || g.Players[v.Owner].Eliminated {
					continue
				}
				if tile.Resource == CatanGold {
					result.Gold[v.Owner] += 2
				} else {
					result.Resources[v.Owner][tile.Resource]++ // Harbor is not a resource-doubling city.
				}
			}
		}
		// Base 2025 scarcity rule: a sole recipient takes what remains; when
		// multiple players share an insufficient resource, nobody gets that type.
		for resource, bank := range g.Bank {
			total, recipients := 0, 0
			for _, hand := range result.Resources {
				total += hand[resource]
				if hand[resource] > 0 {
					recipients++
				}
			}
			if total > bank {
				for p := range result.Resources {
					if result.Resources[p][resource] > 0 {
						result.Resources[p][resource] = 0
						if recipients == 1 {
							result.Resources[p][resource] = bank
						}
					}
				}
			}
		}
		for p := range g.Players {
			if !g.Players[p].Eliminated && sum(result.Resources[p]) == 0 {
				result.Gold[p]++ // Gold-field income is not a resource; FAQ confirms compensation too.
			}
		}
		if sum(result.Gold) > e.GoldBank {
			return catanExplorerProduction{}, errors.New("金币供应不足的官方结算规则尚未核对，不能部分结算生产")
		}
	}
	// All costs/inventory have been checked. Entering action is the only
	// remaining fallible operation, so perform it before modifying payouts.
	if phase == "ready" {
		if err := c.beginAction(g, f, player, sequence); err != nil {
			return catanExplorerProduction{}, err
		}
	}
	for p, resources := range result.Resources {
		for resource, amount := range resources {
			g.Players[p].Resources[resource] += amount
			g.Bank[resource] -= amount
		}
		e.Gold[p] += result.Gold[p]
		e.GoldBank -= result.Gold[p]
	}
	e.Turn.Dice, e.Turn.Phase = dice, phase
	e.Turn.Discard = slices.Clone(result.Discard)
	return result, nil
}

func (e *catanExplorerEconomy) discard(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64, cards []int) error {
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	if player < 0 || player >= len(g.Players) || e.Turn == nil || e.Turn.Phase != "discard" || e.Turn.Sequence != sequence || e.Turn.Discard[player] == 0 || !catanBundle(cards) || sum(cards) != e.Turn.Discard[player] || !catanHas(g.Players[player].Resources, cards) {
		return errors.New("请选择本次七点应归还的资源，金币不计入弃牌")
	}
	phase := "discard"
	if sum(e.Turn.Discard) == e.Turn.Discard[player] {
		phase = "pirate"
		if c.Scenario == "land-ho" {
			if err := c.beginAction(g, f, e.Turn.Player, sequence); err != nil {
				return err
			}
			phase = "ready"
		}
	}
	for r, amount := range cards {
		g.Players[player].Resources[r] -= amount
		g.Bank[r] += amount
	}
	e.Turn.Discard[player], e.Turn.Phase = 0, phase
	return nil
}

func (e catanExplorerEconomy) actionAllowed(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64) error {
	if err := e.productionAllowed(g, f, c, player, sequence, "ready"); err != nil {
		return err
	}
	return c.allowed(g, f, player, sequence, "action")
}

// Resource indices 0–4, gold=-1. Each action is one exact official exchange:
// three equal resources for one other resource/gold, or two gold for a resource.
func (e *catanExplorerEconomy) bankTrade(g *Catan, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64, give, receive int) error {
	if err := e.actionAllowed(g, f, c, player, sequence); err != nil {
		return err
	}
	if give < -1 || give >= 5 || receive < -1 || receive >= 5 || give == receive {
		return errors.New("请选择不同种类的资源或金币")
	}
	if give == -1 {
		if e.Turn.Bought >= 2 || e.Gold[player] < 2 || g.Bank[receive] == 0 {
			return errors.New("每回合只能两次用2金币购买银行现有资源")
		}
		e.Gold[player] -= 2
		e.GoldBank += 2
		g.Bank[receive]--
		g.Players[player].Resources[receive]++
		e.Turn.Bought++
	} else {
		if g.Players[player].Resources[give] < 3 || receive >= 0 && g.Bank[receive] == 0 {
			return errors.New("须支付同类3资源，且银行有目标资源")
		}
		if receive == -1 && e.GoldBank == 0 {
			return errors.New("金币供应耗尽的官方交易规则尚未核对")
		}
		g.Players[player].Resources[give] -= 3
		g.Bank[give] += 3
		if receive == -1 {
			e.Gold[player]++
			e.GoldBank--
		} else {
			g.Players[player].Resources[receive]++
			g.Bank[receive]--
		}
	}
	return nil
}

type catanExplorerEconomyView struct {
	Gold     []int `json:"gold"`
	GoldBank int   `json:"goldBank"`
	Bought   int   `json:"bought"`
}

func (e catanExplorerEconomy) publicView() catanExplorerEconomyView {
	v := catanExplorerEconomyView{Gold: slices.Clone(e.Gold), GoldBank: e.GoldBank}
	if e.Turn != nil {
		v.Bought = e.Turn.Bought
	}
	return v
}
