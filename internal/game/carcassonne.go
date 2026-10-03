package game

import (
	"errors"
	"fmt"
	"sort"
)

type CarPlayer struct {
	Score      int  `json:"score"`
	Meeples    int  `json:"meeples"`
	Eliminated bool `json:"eliminated,omitempty"`
}
type CarTile struct {
	X        int `json:"x"`
	Y        int `json:"y"`
	Kind     int `json:"kind"`
	Art      int `json:"art"`
	Rotation int `json:"rotation"`
	Owner    int `json:"owner"`
	Feature  int `json:"feature"`
}
type CarPlacement struct {
	X        int `json:"x"`
	Y        int `json:"y"`
	Rotation int `json:"rotation"`
}
type Carcassonne struct {
	Players   []CarPlayer `json:"players"`
	Tiles     []CarTile   `json:"tiles"`
	Deck      []int       `json:"deck"`
	Current   int         `json:"current"`
	Discarded []int       `json:"discarded"`
	Last      int         `json:"last"`
}

var carDX = [4]int{0, 1, 0, -1}
var carDY = [4]int{-1, 0, 1, 0}

func (s *State) initCarcassonne(n int) {
	g := &Carcassonne{Players: make([]CarPlayer, n), Current: -1, Last: -1, Discarded: []int{}}
	for i := range g.Players {
		g.Players[i].Meeples = 7
	}
	for a := 0; a < 72; a++ {
		if a != 36 {
			g.Deck = append(g.Deck, a)
		}
	}
	shuffle(g.Deck)
	g.Tiles = []CarTile{{Kind: carKind(36), Art: 36, Owner: -1, Feature: -1}}
	s.Carcassonne = g
	s.carDraw()
	s.Log = append(s.Log, "卡卡颂基础版：72 张地块，每人 7 名随从，包含农民")
}
func (g *Carcassonne) positions() map[[2]int]int {
	m := map[[2]int]int{}
	for i, t := range g.Tiles {
		m[[2]int{t.X, t.Y}] = i
	}
	return m
}
func carLegal(t CarTile, tiles []CarTile, positions map[[2]int]int) bool {
	if t.Rotation < 0 || t.Rotation > 3 {
		return false
	}
	if _, ok := positions[[2]int{t.X, t.Y}]; ok {
		return false
	}
	adjacent := false
	for d := 0; d < 4; d++ {
		i, ok := positions[[2]int{t.X + carDX[d], t.Y + carDY[d]}]
		if !ok {
			continue
		}
		adjacent = true
		f := carPortFeature(t, d*3+1)
		other := tiles[i]
		of := carPortFeature(other, ((d+2)%4)*3+1)
		if f < 0 || of < 0 || carDefinitions[t.Kind].Features[f].Kind != carDefinitions[other.Kind].Features[of].Kind {
			return false
		}
	}
	return adjacent
}
func (g *Carcassonne) placements(art int) []CarPlacement {
	result := []CarPlacement{}
	if art < 0 {
		return result
	}
	positions := g.positions()
	candidates := map[[2]int]bool{}
	for _, t := range g.Tiles {
		for d := 0; d < 4; d++ {
			p := [2]int{t.X + carDX[d], t.Y + carDY[d]}
			if _, ok := positions[p]; !ok {
				candidates[p] = true
			}
		}
	}
	for p := range candidates {
		for r := 0; r < 4; r++ {
			if carLegal(CarTile{X: p[0], Y: p[1], Kind: carKind(art), Rotation: r}, g.Tiles, positions) {
				result = append(result, CarPlacement{p[0], p[1], r})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return a.Rotation < b.Rotation
	})
	return result
}
func (s *State) carDraw() {
	g := s.Carcassonne
	g.Current = -1
	s.Phase = "car_tile"
	for len(g.Deck) > 0 {
		a := g.Deck[0]
		g.Deck = g.Deck[1:]
		if len(g.placements(a)) > 0 {
			g.Current = a
			return
		}
		g.Discarded = append(g.Discarded, a)
		s.Log = append(s.Log, fmt.Sprintf("地块 %s 无合法位置，移出本局并重抽", carDefinitions[carKind(a)].Name))
	}
	s.carScore(true)
	s.carFinish()
}
func (s *State) carFinish() {
	s.Finished = true
	s.Phase = "finished"
	s.Winners = []int{}
	best := -1
	for i, p := range s.Carcassonne.Players {
		if p.Eliminated {
			continue
		}
		if p.Score > best {
			best = p.Score
			s.Winners = []int{i}
		} else if p.Score == best {
			s.Winners = append(s.Winners, i)
		}
	}
	s.Log = append(s.Log, "本局结束：未完成建筑与田地已结算，同分共同获胜")
}
func (s *State) carNext() {
	g := s.Carcassonne
	for {
		s.Turn = (s.Turn + 1) % len(g.Players)
		if s.Turn == 0 {
			s.Round++
		}
		if !g.Players[s.Turn].Eliminated {
			break
		}
	}
	s.carDraw()
}
func (s *State) applyCarcassonne(player int, a Action) error {
	g := s.Carcassonne
	if player < 0 || player >= len(g.Players) || player != s.Turn || g.Players[player].Eliminated {
		return errors.New("还没有轮到你")
	}
	switch a.Type {
	case "car_place":
		if s.Phase != "car_tile" || g.Current < 0 {
			return errors.New("请先完成随从派遣")
		}
		t := CarTile{X: a.X, Y: a.Y, Rotation: a.Rotation, Kind: carKind(g.Current), Art: g.Current, Owner: -1, Feature: -1}
		if !carLegal(t, g.Tiles, g.positions()) {
			return errors.New("地块必须相邻，且所有接边地形一致")
		}
		g.Tiles = append(g.Tiles, t)
		g.Last = len(g.Tiles) - 1
		g.Current = -1
		s.Phase = "car_meeple"
		s.Log = append(s.Log, fmt.Sprintf("玩家 %d 在 (%d,%d) 放置地块 %s，旋转 %d°", player+1, t.X, t.Y, carDefinitions[t.Kind].Name, t.Rotation*90))
	case "car_meeple":
		if s.Phase != "car_meeple" {
			return errors.New("请先放置地块")
		}
		if a.Feature < -1 {
			return errors.New("无效随从位置")
		}
		if a.Feature >= 0 {
			legal := g.meepleChoices()
			found := false
			for _, f := range legal {
				if a.Feature == f {
					found = true
				}
			}
			if !found || g.Players[player].Meeples == 0 {
				return errors.New("该区域已被占领，或没有可用随从")
			}
			t := &g.Tiles[g.Last]
			t.Owner = player
			t.Feature = a.Feature
			g.Players[player].Meeples--
			kind := carDefinitions[t.Kind].Features[a.Feature].Kind
			s.Log = append(s.Log, fmt.Sprintf("玩家 %d 派遣%s至 (%d,%d) 的%s", player+1, map[bool]string{true: "农民", false: "随从"}[kind == "field"], t.X, t.Y, carNames[kind]))
		} else {
			s.Log = append(s.Log, fmt.Sprintf("玩家 %d 不派遣随从", player+1))
		}
		s.carScore(false)
		s.carNext()
	default:
		return errors.New("无效的卡卡颂行动")
	}
	return nil
}

// A graph node is one connected segment on one tile, never an entire tile.
type carNode struct{ Tile, Feature int }
type carComponent struct {
	Nodes          []carNode
	Kind           string
	Open           int
	Tiles          int
	Shields        int
	Followers      []int
	Occupied       []int
	AdjacentCities map[carNode]bool
}

func (g *Carcassonne) component(start carNode, positions map[[2]int]int) carComponent {
	result := carComponent{Followers: make([]int, len(g.Players)), AdjacentCities: map[carNode]bool{}}
	queue := []carNode{start}
	seen := map[carNode]bool{}
	tiles := map[int]bool{}
	for len(queue) > 0 {
		node := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if seen[node] {
			continue
		}
		seen[node] = true
		t := g.Tiles[node.Tile]
		f := carDefinitions[t.Kind].Features[node.Feature]
		result.Kind = f.Kind
		result.Nodes = append(result.Nodes, node)
		tiles[node.Tile] = true
		result.Shields += f.Shields
		if t.Owner >= 0 && t.Feature == node.Feature {
			result.Followers[t.Owner]++
			result.Occupied = append(result.Occupied, node.Tile)
		}
		for _, c := range f.Cities {
			result.AdjacentCities[carNode{node.Tile, c}] = true
		}
		if f.Kind == "monastery" {
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					if _, ok := positions[[2]int{t.X + dx, t.Y + dy}]; !ok {
						result.Open++
					}
				}
			}
			continue
		}
		for _, p := range f.Ports {
			p = (p + t.Rotation*3) % 12
			d := p / 3
			neighbor, ok := positions[[2]int{t.X + carDX[d], t.Y + carDY[d]}]
			if !ok {
				result.Open++
				continue
			}
			opposite := ((d+2)%4)*3 + 2 - p%3
			nf := carPortFeature(g.Tiles[neighbor], opposite)
			if nf < 0 {
				result.Open++
				continue
			}
			queue = append(queue, carNode{neighbor, nf})
		}
	}
	result.Tiles = len(tiles)
	return result
}
func (g *Carcassonne) meepleChoices() []int {
	result := []int{}
	if g.Last < 0 {
		return result
	}
	positions := g.positions()
	for f := range carDefinitions[g.Tiles[g.Last].Kind].Features {
		c := g.component(carNode{g.Last, f}, positions)
		if len(c.Occupied) == 0 {
			result = append(result, f)
		}
	}
	return result
}
func (g *Carcassonne) fieldCities(c carComponent, positions map[[2]int]int) int {
	seen := map[carNode]bool{}
	n := 0
	for node := range c.AdjacentCities {
		if seen[node] {
			continue
		}
		city := g.component(node, positions)
		for _, part := range city.Nodes {
			seen[part] = true
		}
		if city.Open == 0 {
			n++
		}
	}
	return n
}
func (s *State) carScore(final bool) {
	g := s.Carcassonne
	positions := g.positions()
	seen := map[carNode]bool{}
	for i, t := range g.Tiles {
		for f := range carDefinitions[t.Kind].Features {
			node := carNode{i, f}
			if seen[node] {
				continue
			}
			c := g.component(node, positions)
			for _, n := range c.Nodes {
				seen[n] = true
			}
			if len(c.Occupied) == 0 || (!final && (c.Open > 0 || c.Kind == "field")) {
				continue
			}
			points := c.Tiles
			switch c.Kind {
			case "city":
				points += c.Shields
				if c.Open == 0 {
					points *= 2
				}
			case "monastery":
				points = 9 - c.Open
			case "field":
				points = 3 * g.fieldCities(c, positions)
			}
			most := 0
			for p, n := range c.Followers {
				if !g.Players[p].Eliminated {
					most = max(most, n)
				}
			}
			for p, n := range c.Followers {
				if n > 0 && n == most && !g.Players[p].Eliminated {
					g.Players[p].Score += points
					detail := fmt.Sprintf("%d 块", c.Tiles)
					if c.Kind == "field" {
						detail = fmt.Sprintf("%d 座完整城市", points/3)
					} else if c.Kind == "city" {
						detail += fmt.Sprintf("、%d 盾徽", c.Shields)
					}
					s.Log = append(s.Log, fmt.Sprintf("玩家 %d 的%s%s（%s），获得 %d 分", p+1, carNames[c.Kind], map[bool]string{true: "终局结算", false: "完成"}[final], detail, points))
				}
			}
			for _, ti := range c.Occupied {
				tile := &g.Tiles[ti]
				g.Players[tile.Owner].Meeples++
				tile.Owner = -1
				tile.Feature = -1
			}
		}
	}
}
func (s *State) EliminateCarcassonne(player int) error {
	g := s.Carcassonne
	if g == nil || s.Finished || player < 0 || player >= len(g.Players) || player != s.Turn || g.Players[player].Eliminated {
		return errors.New("无法移出该玩家")
	}
	g.Players[player].Eliminated = true
	for i := range g.Tiles {
		if g.Tiles[i].Owner == player {
			g.Tiles[i].Owner = -1
			g.Tiles[i].Feature = -1
			g.Players[player].Meeples++
		}
	}
	s.Log = append(s.Log, fmt.Sprintf("玩家 %d 超时离场，随从收回，已放地块保留", player+1))
	active := 0
	for _, p := range g.Players {
		if !p.Eliminated {
			active++
		}
	}
	if g.Current >= 0 {
		g.Deck = append(g.Deck, g.Current)
		shuffle(g.Deck)
		g.Current = -1
	}
	s.carScore(false)
	if active == 1 {
		s.carScore(true)
		s.carFinish()
		return nil
	}
	s.carNext()
	return nil
}
func (s *State) carView(v map[string]any, player int) {
	g := s.Carcassonne
	c := v["carcassonne"].(map[string]any)
	delete(c, "deck")
	c["remaining"] = len(g.Deck)
	c["catalog"] = carDefinitions
	c["legal"] = []CarPlacement{}
	c["meepleChoices"] = []int{}
	if !s.Finished && player == s.Turn && !g.Players[player].Eliminated {
		if s.Phase == "car_tile" {
			c["legal"] = g.placements(g.Current)
		} else if g.Players[player].Meeples > 0 {
			c["meepleChoices"] = g.meepleChoices()
		}
	}
}
func (s *State) carBot(player int) (Action, error) {
	g := s.Carcassonne
	if player < 0 || player >= len(g.Players) || player != s.Turn || g.Players[player].Eliminated {
		return Action{}, errors.New("inactive bot seat")
	}
	if s.Phase == "car_tile" {
		choices := g.placements(g.Current)
		best := -1 << 30
		var selected CarPlacement
		// Public board only. Prefer closing our features and compact maps. Never inspect deck order.
		positions := g.positions()
		for _, p := range choices {
			score := -absCatan(p.X) - absCatan(p.Y)
			tile := CarTile{X: p.X, Y: p.Y, Kind: carKind(g.Current), Art: g.Current, Rotation: p.Rotation, Owner: -1, Feature: -1}
			trial := *g
			trial.Tiles = append(append([]CarTile{}, g.Tiles...), tile)
			ps := trial.positions()
			for d := 0; d < 4; d++ {
				if _, ok := positions[[2]int{p.X + carDX[d], p.Y + carDY[d]}]; ok {
					score += 3
				}
			}
			for f := range carDefinitions[tile.Kind].Features {
				c := trial.component(carNode{len(g.Tiles), f}, ps)
				if c.Kind == "field" {
					continue
				}
				if c.Followers[player] > 0 {
					score += 8
					if c.Open == 0 {
						score += c.Tiles * 8
					}
				} else if len(c.Occupied) == 0 && g.Players[player].Meeples > 0 {
					score += 2
					if c.Open == 0 {
						score += 10
					}
				}
			}
			if score > best {
				best = score
				selected = p
			}
		}
		if len(choices) == 0 {
			return Action{}, errors.New("no legal tile")
		}
		return Action{Type: "car_place", X: selected.X, Y: selected.Y, Rotation: selected.Rotation}, nil
	}
	best := 0
	choice := -1
	if g.Players[player].Meeples > 0 {
		positions := g.positions()
		for _, f := range g.meepleChoices() {
			c := g.component(carNode{g.Last, f}, positions)
			score := c.Tiles*2 - c.Open
			if c.Kind == "monastery" {
				score = 6 - c.Open/2
			}
			if c.Kind == "city" {
				score += 2
			}
			if c.Open == 0 {
				score += 20
			}
			if c.Kind == "field" {
				score = 3*g.fieldCities(c, positions) - 3
				if g.Players[player].Meeples <= 3 {
					score = -1
				}
			} else if g.Players[player].Meeples <= 2 && c.Open > 2 {
				score = -1
			}
			if score > best {
				best = score
				choice = f
			}
		}
	}
	return Action{Type: "car_meeple", Feature: choice}, nil
}
