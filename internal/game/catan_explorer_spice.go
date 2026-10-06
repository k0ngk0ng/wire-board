package game

import (
	"errors"
	"slices"
)

// Private mission rules; public room options remain gated on final acceptance.
// German 2025 p20: six spaces, unlike the seven-space fish/lair tracks.
var catanExplorerSpicePoints = [...]int{0, 1, 1, 2, 2, 3, 3}

type catanExplorerSpice struct {
	Deliveries []catanExplorerSpiceDelivery `json:"deliveries"`
	GoldUse    *catanExplorerSpiceGoldUse   `json:"goldUse,omitempty"`
}
type catanExplorerSpiceDelivery struct {
	Player   int    `json:"player"`
	Sequence uint64 `json:"sequence"`
	Sack     int    `json:"sack"`
}
type catanExplorerSpiceGoldUse struct {
	Player   int    `json:"player"`
	Sequence uint64 `json:"sequence"`
	Count    int    `json:"count"`
}
type catanExplorerSpiceView struct {
	Progress []int                      `json:"progress"`
	Scores   []int                      `json:"scores"`
	Leader   int                        `json:"leader"`
	GoldUse  *catanExplorerSpiceGoldUse `json:"goldUse,omitempty"`
}

func (m catanExplorerSpice) publicView(g *Catan) catanExplorerSpiceView {
	progress, arrival, scores := make([]int, len(g.Players)), make([]int, len(g.Players)), make([]int, len(g.Players))
	for i, d := range m.Deliveries {
		progress[d.Player]++
		arrival[d.Player] = i + 1
	}
	leader := -1
	for p, step := range progress {
		scores[p] = catanExplorerSpicePoints[step]
		if !g.Players[p].Eliminated && step > 0 && (leader < 0 || step > progress[leader] || step == progress[leader] && arrival[p] < arrival[leader]) {
			leader = p
		}
	}
	if leader >= 0 {
		scores[leader]++
	}
	return catanExplorerSpiceView{progress, scores, leader, clone(m.GoldUse)}
}
func (m catanExplorerSpice) validate(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy) error {
	if g == nil || b == nil || f == nil || c == nil || e == nil || b.Scenario != "spices-for-catan" || c.Scenario != b.Scenario {
		return errors.New("香料任务组件不匹配")
	}
	if err := b.validate(g); err != nil {
		return err
	}
	if err := e.validate(g, f, c); err != nil {
		return err
	}
	for _, h := range b.Hidden {
		if !h.Revealed || h.Farm == "" {
			continue
		}
		count := 0
		for _, s := range c.Spice {
			if s.Origin == h.Tile {
				count++
			}
		}
		if count != len(g.Players) {
			return errors.New("已探索农场缺少初始香料组件")
		}
	}
	if c.Turn != nil && c.Turn.Phase == "movement" && f.Turn.Speed != c.farmCount(b, c.Turn.Player, "swift") {
		return errors.New("快速航行移动点与已派驻农场不符")
	}
	seen := map[int]bool{}
	steps := make([]int, len(g.Players))
	var last uint64
	owner := -1
	for _, d := range m.Deliveries {
		if d.Player < 0 || d.Player >= len(g.Players) || d.Sack < 0 || d.Sack >= len(c.Spice) || seen[d.Sack] || d.Sequence == 0 || d.Sequence < last || d.Sequence == last && d.Player != owner || c.Turn == nil || d.Sequence > c.Turn.Sequence || d.Sequence == c.Turn.Sequence && (d.Player != c.Turn.Player || c.Turn.Phase == "action") {
			return errors.New("香料交付记录无效或重复")
		}
		s := c.Spice[d.Sack]
		if s.Owner != d.Player || s.Origin < 0 || s.At != (catanExplorerCargoLocation{"supply", -1}) {
			return errors.New("已交付香料必须归还供应区并保留来源")
		}
		steps[d.Player]++
		if steps[d.Player] > 6 {
			return errors.New("香料交付超过六座农场")
		}
		seen[d.Sack] = true
		last, owner = d.Sequence, d.Player
	}
	if u := m.GoldUse; u != nil {
		if u.Player < 0 || u.Player >= len(g.Players) || u.Sequence == 0 || c.Turn == nil || u.Sequence > c.Turn.Sequence || u.Sequence == c.Turn.Sequence && u.Player != c.Turn.Player || u.Count < 1 || u.Count > c.farmCount(b, u.Player, "gold") {
			return errors.New("快速金币使用次数或回合无效")
		}
	}
	return nil
}
func (m *catanExplorerSpice) apply(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, player int, sequence uint64, kind string, tile, ship, piece int) error {
	if err := m.validate(g, b, f, c, e); err != nil {
		return err
	}
	if kind == "gold" {
		if err := e.actionAllowed(g, f, c, player, sequence); err != nil {
			return err
		}
	} else {
		if err := e.productionAllowed(g, f, c, player, sequence, "ready"); err != nil {
			return err
		}
		if err := c.allowed(g, f, player, sequence, "movement"); err != nil {
			return err
		}
	}
	base := *g
	base.Explorer = nil
	ng, nf, nc, ne, nm := clone(base), clone(*f), clone(*c), clone(*e), clone(*m)
	if err := nm.applyUnchecked(&ng, b, &nf, &nc, &ne, player, sequence, kind, tile, ship, piece); err != nil {
		return err
	}
	if err := nm.validate(&ng, b, &nf, &nc, &ne); err != nil {
		return err
	}
	ng.Explorer = g.Explorer
	*g, *f, *c, *e, *m = ng, nf, nc, ne, nm
	return nil
}
func (m *catanExplorerSpice) applyUnchecked(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, e *catanExplorerEconomy, player int, sequence uint64, kind string, tile, ship, piece int) error {
	loc := catanExplorerCargoLocation{"ship", ship}
	switch kind {
	case "land":
		if !c.holder(g, f, player, loc) || piece < 0 || piece >= len(c.Units) || piece/11 != player || piece%11 < 2 || c.Units[piece] != loc || c.farmFriend(player, tile) || !catanExplorerTouches(g, f.Positions[ship], tile) {
			return errors.New("请用接触农场的己方船派驻一名船员，每座农场只能一次")
		}
		sack := -1
		for id, s := range c.Spice {
			if s.Origin == tile && s.Owner == -1 && s.At == (catanExplorerCargoLocation{"farm", tile}) {
				sack = id
				break
			}
		}
		if sack < 0 {
			return errors.New("此处没有可领取的香料")
		}
		c.Units[piece] = catanExplorerCargoLocation{"farm", tile}
		c.Spice[sack].Owner = player
		c.Spice[sack].At = loc
		speed := c.farmCount(b, player, "swift")
		if speed > f.Turn.Speed {
			if err := f.swiftVoyage(g, player, sequence, speed); err != nil {
				return err
			}
		}
	case "deliver":
		if !c.holder(g, f, player, loc) || piece < 0 || piece >= len(c.Spice) || c.Spice[piece].Owner != player || c.Spice[piece].At != loc {
			return errors.New("请选择己方船上的香料")
		}
		edge := g.Edges[f.Positions[ship]]
		if !slices.Contains(b.Council.Anchors, edge.A) && !slices.Contains(b.Council.Anchors, edge.B) {
			return errors.New("船端必须接触议会岛锚点才能交付香料")
		}
		c.Spice[piece].At = catanExplorerCargoLocation{"supply", -1}
		m.Deliveries = append(m.Deliveries, catanExplorerSpiceDelivery{player, sequence, piece})
	case "gold":
		used := 0
		if m.GoldUse != nil && m.GoldUse.Sequence == sequence {
			used = m.GoldUse.Count
		}
		if used >= c.farmCount(b, player, "gold") || piece < 0 || piece >= 5 || g.Players[player].Resources[piece] < 1 {
			return errors.New("每座已派驻的金币农场每行动阶段可用一张资源换一金币")
		}
		if e.GoldBank == 0 {
			return errors.New("金币供应耗尽的官方交易规则尚未核对")
		}
		g.Players[player].Resources[piece]--
		g.Bank[piece]++
		e.Gold[player]++
		e.GoldBank--
		m.GoldUse = &catanExplorerSpiceGoldUse{player, sequence, used + 1}
	default:
		return errors.New("未知香料任务行动")
	}
	return nil
}
