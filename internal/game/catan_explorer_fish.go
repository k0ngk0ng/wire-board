package game

import (
	"errors"
	"slices"
)

// Private fish-mission kernel. Its board is official; room creation stays
// closed until full State/Apply, pirate/lair integration and UI acceptance.
type catanExplorerFish struct {
	LastRoll   *catanExplorerFishRoll      `json:"lastRoll,omitempty"`
	Deliveries []catanExplorerFishDelivery `json:"deliveries"`
}
type catanExplorerFishRoll struct {
	Player   int    `json:"player"`
	Sequence uint64 `json:"sequence"`
	Die      int    `json:"die"`
	Spawned  int    `json:"spawned"` // Fish ID, -1 if nothing appeared.
}
type catanExplorerFishDelivery struct {
	Player   int    `json:"player"`
	Sequence uint64 `json:"sequence"`
	Fish     int    `json:"fish"`
}
type catanExplorerFishView struct {
	LastRoll *catanExplorerFishRoll `json:"lastRoll,omitempty"`
	Progress []int                  `json:"progress"`
	Scores   []int                  `json:"scores"`
	Leader   int                    `json:"leader"`
}

func (m catanExplorerFish) publicView(players int) catanExplorerFishView {
	progress, arrival := make([]int, players), make([]int, players)
	for i, d := range m.Deliveries {
		// Same printed seven spaces as Pirate Lairs, p4. Last-space overflow
		// remains a rule-source gate shared by both missions; never invent a VP.
		if progress[d.Player] < 7 {
			progress[d.Player]++
			arrival[d.Player] = i + 1
		}
	}
	leader := -1
	scores := make([]int, players)
	for p, step := range progress {
		scores[p] = catanExplorerLairPoints[step]
		if step > 0 && (leader < 0 || step > progress[leader] || step == progress[leader] && arrival[p] < arrival[leader]) {
			leader = p
		}
	}
	if leader >= 0 {
		scores[leader]++
	}
	return catanExplorerFishView{clone(m.LastRoll), progress, scores, leader}
}
func (m catanExplorerFish) validate(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo) error {
	if g == nil || b == nil || f == nil || c == nil || b.Scenario != "fish-for-catan" || c.Scenario != b.Scenario {
		return errors.New("鱼群任务组件缺失")
	}
	if err := b.validate(g); err != nil {
		return err
	}
	if err := c.validate(g, f); err != nil {
		return err
	}
	for _, loc := range c.Fish {
		if loc.Kind == "shoal" && !slices.ContainsFunc(b.Hidden, func(h catanExplorerHidden) bool { return h.Tile == loc.Index && h.Revealed && h.Fish > 0 }) {
			return errors.New("鱼群只能出现在已探索渔场")
		}
	}
	if r := m.LastRoll; r != nil {
		if r.Player < 0 || r.Player >= len(g.Players) || r.Sequence == 0 || r.Die < 1 || r.Die > 6 || r.Spawned < -1 || r.Spawned >= len(c.Fish) || c.Turn == nil || r.Sequence > c.Turn.Sequence || r.Sequence == c.Turn.Sequence && (r.Player != c.Turn.Player || c.Turn.Phase == "action") {
			return errors.New("捕捞骰子或回合记录无效")
		}
	}
	last := uint64(0)
	owner := -1
	for _, d := range m.Deliveries {
		if d.Player < 0 || d.Player >= len(g.Players) || d.Fish < 0 || d.Fish >= len(c.Fish) || d.Sequence == 0 || d.Sequence < last || d.Sequence == last && d.Player != owner || c.Turn == nil || d.Sequence > c.Turn.Sequence || d.Sequence == c.Turn.Sequence && (d.Player != c.Turn.Player || c.Turn.Phase == "action") {
			return errors.New("鱼群交付记录无效")
		}
		last, owner = d.Sequence, d.Player
	}
	return nil
}

// The authoritative dispatcher supplies the actual pirate tile and die source.
// No action changes MPs; even an exhausted but docked ship can load fish (FAQ).
func (m *catanExplorerFish) apply(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64, kind string, ship, fish, pirateTile int, randN func(int) int) error {
	if err := m.validate(g, b, f, c); err != nil {
		return err
	}
	if err := c.allowed(g, f, player, sequence, "movement"); err != nil {
		return err
	}
	nm, nc := clone(*m), clone(*c)
	if err := nm.applyUnchecked(g, b, f, &nc, player, sequence, kind, ship, fish, pirateTile, randN); err != nil {
		return err
	}
	if err := nm.validate(g, b, f, &nc); err != nil {
		return err
	}
	*m, *c = nm, nc
	return nil
}
func (m *catanExplorerFish) applyUnchecked(g *Catan, b *catanExplorerBoard, f *catanExplorerSailing, c *catanExplorerCargo, player int, sequence uint64, kind string, ship, fish, pirateTile int, randN func(int) int) error {
	switch kind {
	case "roll":
		if randN == nil || m.LastRoll != nil && m.LastRoll.Sequence == sequence {
			return errors.New("每个移动阶段只能掷一次捕捞骰")
		}
		die := randN(6) + 1
		if die < 1 || die > 6 {
			return errors.New("捕捞骰子无效")
		}
		r := &catanExplorerFishRoll{Player: player, Sequence: sequence, Die: die, Spawned: -1}
		for _, h := range b.Hidden {
			if !h.Revealed || h.Fish != die || h.Tile == pirateTile {
				continue
			}
			loc := catanExplorerCargoLocation{"shoal", h.Tile}
			if c.used(loc) > 0 {
				break
			}
			for id, at := range c.Fish {
				if at == (catanExplorerCargoLocation{"supply", -1}) {
					c.Fish[id] = loc
					r.Spawned = id
					break
				}
			}
			break
		}
		m.LastRoll = r
	case "load", "deliver":
		loc := catanExplorerCargoLocation{"ship", ship}
		if !c.holder(g, f, player, loc) || fish < 0 || fish >= len(c.Fish) {
			return errors.New("请选择己方船与有效鱼群")
		}
		if kind == "load" {
			from := c.Fish[fish]
			if from.Kind != "shoal" || c.used(loc) != 0 || !catanExplorerTouches(g, f.Positions[ship], from.Index) {
				return errors.New("空船接触有鱼群的渔场才能装载")
			}
			c.Fish[fish] = loc
		} else {
			edge := g.Edges[f.Positions[ship]]
			if c.Fish[fish] != loc || !slices.Contains(b.Council.Anchors, edge.A) && !slices.Contains(b.Council.Anchors, edge.B) {
				return errors.New("载鱼船的一端必须接触议会岛锚点")
			}
			c.Fish[fish] = catanExplorerCargoLocation{"supply", -1}
			m.Deliveries = append(m.Deliveries, catanExplorerFishDelivery{player, sequence, fish})
		}
	default:
		return errors.New("未知鱼群任务动作")
	}
	return nil
}

// Call inside the authoritative pirate-placement transaction. Fish aboard a
// ship/harbor are untouched: only a shoal's uncollected haul is removed.
func (c *catanExplorerCargo) removeShoalFish(tile int) {
	for id, loc := range c.Fish {
		if loc == (catanExplorerCargoLocation{"shoal", tile}) {
			c.Fish[id] = catanExplorerCargoLocation{"supply", -1}
		}
	}
}
