package game

import (
	"errors"
	"slices"
)

type catanAttackCityBattle struct {
	Tile       int                      `json:"tile"`
	Barbarians int                      `json:"barbarians"`
	Knights    []catanAttackCityKnight  `json:"knights"`
	Strength   []int                    `json:"strength"`
	Prisoners  []int                    `json:"prisoners"`
	Gold       []int                    `json:"gold"`
	LossDie    int                      `json:"lossDie"`
	Lost       []catanAttackCityKnight  `json:"lost"`
	Downgraded []catanAttackCityKnight  `json:"downgraded"`
	Contests   []catanAttackContestRoll `json:"contests,omitempty"`
}

// No board mutation until every die and payout is validated. The end-phase
// controller will sequence these battles and run victory between coasts.
func (s *State) catanAttackCityBattle(tile int, die func() int) (*catanAttackCityBattle, error) {
	if s.Catan == nil || !s.Catan.attackKnights() || die == nil || !slices.Contains(s.Catan.Attack.Map.Coast, tile) {
		return nil, errors.New("无效蛮族城市骑士战场")
	}
	if err := s.Catan.Attack.City.validate(s.Catan); err != nil {
		return nil, err
	}
	next := clone(*s)
	g := next.Catan
	a := g.Attack
	c := a.City
	n := len(g.Players)
	count := a.Barbarians[tile]
	if count == 0 {
		return nil, nil
	}
	b := &catanAttackCityBattle{Tile: tile, Barbarians: count, Strength: make([]int, n), Prisoners: make([]int, n), Gold: make([]int, n), Knights: []catanAttackCityKnight{}, Lost: []catanAttackCityKnight{}, Downgraded: []catanAttackCityKnight{}}
	for _, k := range c.Knights {
		if k.Active && !g.Players[k.Owner].Eliminated && slices.Contains(g.Edges[k.Edge].Tiles, tile) {
			b.Knights = append(b.Knights, k)
			b.Strength[k.Owner] += k.Strength
		}
	}
	if sum(b.Strength) <= count {
		return nil, nil
	}
	allocation := catanAttackBattle{Barbarians: count, Prisoners: b.Prisoners, Gold: b.Gold}
	if err := allocation.distribute(b.Strength, die); err != nil {
		return nil, err
	}
	b.Contests = allocation.Contests
	a.Barbarians[tile] = 0
	for p := range n {
		a.Prisoners[p] += b.Prisoners[p]
	}
	next.catanScores()
	next.catanVictory()
	if next.Finished {
		if err := a.ensureGold(sum(b.Gold)); err != nil {
			return nil, err
		}
		for p := range n {
			a.Gold[p] += b.Gold[p]
			a.GoldBank -= b.Gold[p]
		}
		*s = next
		return b, nil
	}
	b.LossDie = die()
	orientation := catanAttackLossOrientation(b.LossDie)
	if orientation < 0 {
		return nil, errors.New("骑士损失骰子无效")
	}
	a.Barbarians[tile] = 0
	for _, k := range b.Knights {
		if a.Map.edgeOrientation(g, k.Edge) != orientation {
			continue
		}
		i := c.at(k.Edge)
		replacement := 0
		for rank := k.Strength - 1; rank >= 1; rank-- {
			if c.count(k.Owner, rank) < 2 {
				replacement = rank
				break
			}
		}
		if replacement == 0 {
			c.Knights = slices.Delete(c.Knights, i, i+1)
			b.Lost = append(b.Lost, k)
		} else {
			c.Knights[i].Strength = replacement
			b.Downgraded = append(b.Downgraded, c.Knights[i])
		}
		b.Gold[k.Owner] += 3
	}
	if err := a.ensureGold(sum(b.Gold)); err != nil {
		return nil, err
	}
	for p := range n {
		a.Gold[p] += b.Gold[p]
		a.GoldBank -= b.Gold[p]
	}
	if err := c.validate(g); err != nil {
		return nil, err
	}
	*s = next
	return b, nil
}

func (c *catanAttackCity) distances(g *Catan, from, steps int) map[int]int {
	out := map[int]int{}
	if from < 0 || from >= len(g.Edges) || steps < 1 || steps > len(g.Edges) {
		return out
	}
	out[from] = 0
	queue := []int{from}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		d := out[id]
		if d == steps {
			continue
		}
		e := g.Edges[id]
		for _, next := range g.Edges {
			if next.A != e.A && next.A != e.B && next.B != e.A && next.B != e.B {
				continue
			}
			if _, seen := out[next.ID]; seen {
				continue
			}
			out[next.ID] = d + 1
			queue = append(queue, next.ID)
		}
	}
	return out
}
func (c *catanAttackCity) destinations(g *Catan, edge int) []int {
	out := []int{}
	i := c.at(edge)
	if i < 0 {
		return out
	}
	steps := 3
	if c.Knights[i].Active {
		steps = 5
	}
	for to, d := range c.distances(g, edge, steps) {
		if d > 0 && c.at(to) < 0 && !g.Attack.castleEdge(g, to) {
			out = append(out, to)
		}
	}
	slices.Sort(out)
	return out
}
func (c *catanAttackCity) move(g *Catan, player, from, to int) error {
	i := c.at(from)
	if i < 0 || c.Knights[i].Owner != player || !slices.Contains(c.destinations(g, from), to) {
		return errors.New("道路骑士目的地无效")
	}
	c.Knights[i].Edge = to
	c.Knights[i].Active = false
	return nil
}

// Official FAQ: the displaced knight's owner selects the next free path.
// Retain all equally near empty paths for that owner's private response.
func (c *catanAttackCity) displacementTargets(g *Catan, from int) []int {
	i := c.at(from)
	out := []int{}
	if i < 0 || !c.Knights[i].Active || c.Knights[i].ActivatedAt == g.CitiesKnights.ActionSerial {
		return out
	}
	for to, d := range c.distances(g, from, 3) {
		other := c.at(to)
		if d > 0 && other >= 0 && c.Knights[other].Owner != c.Knights[i].Owner && c.Knights[other].Strength < c.Knights[i].Strength {
			out = append(out, to)
		}
	}
	slices.Sort(out)
	return out
}
func (c *catanAttackCity) retreatEdges(g *Catan, from int) []int {
	// The printed FAQ says nearest free edge, not along the owner's roads.
	// Passing occupied paths is legal; future response excludes castle edges.
	for steps := 1; steps <= len(g.Edges); steps++ {
		out := []int{}
		for to, d := range c.distances(g, from, steps) {
			if d > 0 && c.at(to) < 0 && !g.Attack.castleEdge(g, to) {
				out = append(out, to)
			}
		}
		if len(out) > 0 {
			slices.Sort(out)
			return out
		}
	}
	return []int{}
}

type catanAttackCityRetreat struct {
	Player   int                   `json:"player"`
	Knight   catanAttackCityKnight `json:"knight"`
	Attacker int                   `json:"attacker"`
	Targets  []int                 `json:"targets"`
}

func (c *catanAttackCity) beginDisplacement(g *Catan, player, from, to int) (*catanAttackCityRetreat, error) {
	i := c.at(from)
	if i < 0 || c.Knights[i].Owner != player || !slices.Contains(c.displacementTargets(g, from), to) {
		return nil, errors.New("请选择三步内等级较低的对手道路骑士")
	}
	j := c.at(to)
	displaced := c.Knights[j]
	displaced.Active = false
	c.Knights[i].Edge = to
	c.Knights[i].Active = false
	c.Knights = slices.Delete(c.Knights, j, j+1)
	targets := c.retreatEdges(g, to)
	if len(targets) == 0 {
		// Finite knight supply cannot fill the map, but retain a deterministic
		// fallback for a future smaller board: return the displaced piece.
		return nil, nil
	}
	return &catanAttackCityRetreat{Player: displaced.Owner, Knight: displaced, Attacker: player, Targets: targets}, nil
}
func (c *catanAttackCity) completeDisplacement(g *Catan, q *catanAttackCityRetreat, player, to int) error {
	if q == nil || player != q.Player || q.Knight.Owner != player || q.Knight.Active || !slices.Contains(q.Targets, to) || !slices.Contains(c.retreatEdges(g, q.Knight.Edge), to) {
		return errors.New("请由被驱逐骑士的主人选择最近空边")
	}
	attacker := c.at(q.Knight.Edge)
	if attacker < 0 || c.Knights[attacker].Owner != q.Attacker || c.Knights[attacker].Active || c.Knights[attacker].Strength <= q.Knight.Strength || c.count(player, q.Knight.Strength) >= 2 {
		return errors.New("道路骑士驱逐记录与棋盘不符")
	}
	knight := q.Knight
	knight.Edge = to
	c.Knights = append(c.Knights, knight)
	return nil
}
