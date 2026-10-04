package game

import (
	"errors"
	"slices"
)

// The top port is face up; the remaining shuffled ports are not public.
// Index is independent of settlement setup so placing ports grants no pieces,
// starting resources, Helpers, or paired action phases.
type CatanNewWorld struct {
	Ports []int `json:"ports"`
	Index int   `json:"index"`
}

// Internal constructor; the room picker remains closed until all expansion
// configuration and UI acceptance are complete. Uses the printed inventories.
func NewCatanNewWorld(n int, options CatanOptions) (*State, error) {
	s, err := NewCatan(n, options)
	if err != nil {
		return nil, err
	}
	if err = s.Catan.makeNewWorldMap(); err != nil {
		return nil, err
	}
	s.Phase = "catan_world_ports"
	s.Log = append(s.Log, "新世界：轮流放置随机港口，再开始两轮起始建设；12分获胜")
	return s, nil
}

func (g *Catan) newWorld() *CatanNewWorld {
	if g.Seafarers == nil {
		return nil
	}
	return g.Seafarers.NewWorld
}

func (g *Catan) makeNewWorldMap() error {
	if len(g.Players) < 3 || len(g.Players) > 6 {
		return errors.New("新世界需要3至6位玩家")
	}
	terrainCounts := []int{5, 4, 5, 5, 4, 0, 19, 0}
	numberCounts := []int{0, 0, 1, 3, 3, 3, 2, 0, 2, 3, 3, 2, 1}
	ports := []int{-1, -1, -1, -1, -1, 0, 1, 2, 3, 4}
	if len(g.Players) > 4 {
		terrainCounts = []int{7, 7, 7, 7, 7, 3, 21, 4}
		numberCounts = []int{0, 0, 2, 3, 4, 5, 5, 0, 5, 5, 4, 4, 2}
		ports = append(ports, 2) // The sixth special port in the extension is wool.
	}
	terrain, numbers := []int{}, []int{}
	for resource, count := range terrainCounts {
		for range count {
			terrain = append(terrain, resource)
		}
	}
	for number, count := range numberCounts {
		for range count {
			numbers = append(numbers, number)
		}
	}
	shuffle(terrain)
	shuffle(numbers)
	shuffle(ports)
	specs := newWorldFrame(len(g.Players))
	for i := range specs {
		specs[i].Resource = terrain[i]
	}
	if err := g.makeScenarioMap(specs); err != nil {
		return err
	}
	g.Seafarers = &CatanSeafarers{NewWorld: &CatanNewWorld{Ports: ports}, Scenario: "new_world", Variable: true, VictoryPoints: 12, IslandBonus: 1, Pirate: -1, Seats: make([]CatanSeafarerSeat, len(g.Players))}
	// Both pieces start on the frame in the official diagrams, including 5–6.
	g.Robber = -1
	g.Ports = []CatanPort{}
	g.Seafarers.Islands = g.findIslands()
	return g.newWorldNumbers(numbers)
}

func (g *Catan) newWorldNumbers(numbers []int) error {
	red, black, land := []int{}, []int{}, []int{}
	for _, n := range numbers {
		if n == 6 || n == 8 {
			red = append(red, n)
		} else {
			black = append(black, n)
		}
	}
	for _, t := range g.Tiles {
		if t.Resource < CatanDesert || t.Resource == CatanGold {
			land = append(land, t.ID)
		}
	}
	if len(land) != len(numbers) {
		return errors.New("新世界数字牌数量不符")
	}
	shuffle(land)
	adjacent := make([][]int, len(g.Tiles))
	for _, e := range g.Edges {
		if len(e.Tiles) == 2 {
			a, b := e.Tiles[0], e.Tiles[1]
			adjacent[a] = append(adjacent[a], b)
			adjacent[b] = append(adjacent[b], a)
		}
	}
	var place func(int, int) bool
	place = func(start, at int) bool {
		if at == len(red) {
			return true
		}
		for i := start; i <= len(land)-(len(red)-at); i++ {
			id := land[i]
			if g.Tiles[id].Resource == CatanGold {
				continue
			}
			ok := true
			for _, other := range adjacent[id] {
				if g.Tiles[other].Number == 6 || g.Tiles[other].Number == 8 {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			g.Tiles[id].Number = red[at]
			if place(i+1, at+1) {
				return true
			}
			g.Tiles[id].Number = 0
		}
		return false
	}
	if !place(0, 0) {
		return errors.New("无法按新世界规则分开红色数字牌")
	}
	for _, id := range land {
		if g.Tiles[id].Number == 0 {
			g.Tiles[id].Number, black = black[0], black[1:]
		}
	}
	return nil
}

func (g *Catan) worldPortEdges() []int {
	result := []int{}
	w := g.newWorld()
	if w == nil || w.Index >= len(w.Ports) {
		return result
	}
	blocked := map[int]bool{}
	for _, p := range g.Ports {
		e := g.Edges[p.Edge]
		blocked[e.A], blocked[e.B] = true, true
	}
	for _, e := range g.Edges {
		if !blocked[e.A] && !blocked[e.B] && g.edgeTerrain(e.ID, true) && g.edgeTerrain(e.ID, false) {
			result = append(result, e.ID)
		}
	}
	return result
}

func (s *State) catanWorldPort(player int, a Action) error {
	g := s.Catan
	w := g.newWorld()
	if w == nil || s.Phase != "catan_world_ports" || player != s.Turn || g.SetupStep != 0 || a.Type != "catan_world_port" || !slices.Contains(g.worldPortEdges(), a.Edge) {
		return errors.New("请在空闲海岸放置当前港口，与其他港口至少隔开一条边")
	}
	resource := w.Ports[w.Index]
	g.Ports = append(g.Ports, CatanPort{Edge: a.Edge, Resource: resource})
	w.Index++
	s.catanLog(player, "在海岸 #%d 放置%s（%d/%d）", a.Edge+1, catanPortName(resource), w.Index, len(w.Ports))
	if w.Index == len(w.Ports) {
		s.Turn = g.StartPlayer
		s.Phase = "catan_setup_settlement"
	} else {
		s.Turn = (s.Turn + 1) % len(g.Players)
	}
	return nil
}

func (s *State) catanWorldPortBot(player int) (Action, error) {
	g := s.Catan
	if s.Phase != "catan_world_ports" || s.Turn != player {
		return Action{}, errors.New("当前不能放置新世界港口")
	}
	edges := g.worldPortEdges()
	if len(edges) == 0 {
		return Action{}, errors.New("新世界没有合法港口位置")
	}
	// No unrevealed port identities or private resources participate in placement.
	return Action{Type: "catan_world_port", Edge: edges[catanRandom(len(edges))]}, nil
}
