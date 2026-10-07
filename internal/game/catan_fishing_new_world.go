package game

import (
	"errors"
	"fmt"
	"slices"
)

type CatanFishingWorldSetup struct {
	Numbers []int `json:"numbers"` // saved shuffled stack; only current is public
	Index   int   `json:"index"`
}

// July 2025 Fishing + Seafarers p.2: no lake; after all ports, place the
// randomly drawn grounds in turn. Always retain the pre-game approved map.
// Public three/four-player rooms validate space when selecting the combination.
func NewCatanFishingNewWorld(n int, options CatanOptions, layout *CatanNewWorldMap) (*State, error) {
	if n < 3 || n > 6 || options.Helpers || options.AllHelpers {
		return nil, errors.New("新世界捕鱼需要三至六人，目前不使用助手")
	}
	s, err := NewCatanSeafarers(n, options, CatanSeafarersSetup{Scenario: "new_world", Layout: "prepared"}, layout)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	numbers := catanFishingGroundNumbers(n)
	if !g.fishingWorldCanFinish(len(g.newWorld().Ports), len(numbers)) {
		return nil, fmt.Errorf("此地图无法同时放下%d个港口和%d个渔场，请在开局前调整海岸布局", len(g.newWorld().Ports), len(numbers))
	}
	tokens, err := newCatanFishingTokens(n)
	if err != nil {
		return nil, err
	}
	shuffle(numbers)
	g.Fishing = &CatanFishing{
		Map:    catanFishingMap{Lakes: []catanFishingLake{}, Grounds: []catanFishingGround{}},
		Tokens: *tokens, LastRollID: -1, Started: make([]bool, n),
		WorldSetup: &CatanFishingWorldSetup{Numbers: numbers},
	}
	s.Log = append(s.Log, "新世界捕鱼：先轮流放港口，再轮流安放随机渔场；不放湖泊，12分获胜，持旧靴子需13分")
	return s, nil
}

// GenerateCatanFishingNewWorldMap is for an explicitly requested fresh map.
// Enabling Fishing on an existing approved map must validate it, never reroll it.
func GenerateCatanFishingNewWorldMap(n int) (*CatanNewWorldMap, error) {
	if n < 3 || n > 6 {
		return nil, errors.New("新世界捕鱼需要三至六人")
	}
	for range 64 {
		layout, err := GenerateCatanNewWorldMap(n)
		if err != nil {
			return nil, err
		}
		if _, err = NewCatanFishingNewWorld(n, CatanOptions{FiveSix: n > 4}, layout); err == nil {
			return layout, nil
		}
	}
	return nil, errors.New("暂未生成可放满港口与渔场的地图，请重新生成")
}

func (g *Catan) fishingWorldSetup() *CatanFishingWorldSetup {
	if g.Fishing == nil {
		return nil
	}
	return g.Fishing.WorldSetup
}

func (f catanFishingMap) validateNewWorld(g *Catan) error {
	w, q := g.newWorld(), g.fishingWorldSetup()
	n := len(g.Players)
	wantNumbers, ports := catanFishingGroundNumbers(n), 10
	if n > 4 {
		ports = 11
	}
	grounds := len(wantNumbers)
	if w == nil || q == nil || g.Seafarers.Scenario != "new_world" || n < 3 || n > 6 || g.Options.FiveSix != (n > 4) || (g.Paired != nil) != (n > 4) ||
		len(w.Ports) != ports || w.Index < 0 || w.Index > ports || len(g.Ports) != w.Index ||
		q.Index < 0 || q.Index > grounds || len(q.Numbers) != grounds || len(f.Grounds) != q.Index ||
		len(f.Lakes) != 0 || len(f.ExtraNumbers) != 0 || w.Index < ports && q.Index != 0 || g.SetupStep > 0 && q.Index != grounds {
		return errors.New("新世界捕鱼布局阶段或组件数量不符")
	}
	numbers := slices.Clone(q.Numbers)
	slices.Sort(numbers)
	if !slices.Equal(numbers, wantNumbers) {
		return errors.New("新世界渔场点数不符")
	}
	for _, tile := range g.Tiles {
		if tile.Resource == catanLake || tile.Resource == CatanFog {
			return errors.New("新世界捕鱼不使用湖泊或迷雾")
		}
	}
	coasts := map[[2]int]CatanFishingCoast{}
	for _, c := range g.fishingCoasts(g.findIslands()) {
		coasts[c.Edges] = c
	}
	for i, ground := range f.Grounds {
		c, ok := coasts[ground.Edges]
		if !ok || ground.Number != q.Numbers[i] || ground.Vertices != c.Vertices ||
			(ground.SeaTile == nil) != (c.SeaTile < 0) || ground.SeaTile != nil && *ground.SeaTile != c.SeaTile {
			return errors.New("新世界渔场与已抽取点数或海岸位置不符")
		}
	}
	if !g.fishingWorldCanFinish(ports-w.Index, grounds-q.Index) {
		return errors.New("新世界剩余港口和渔场没有足够的互不重叠位置")
	}
	if g.Robber < -1 || g.Robber >= len(g.Tiles) || g.Robber >= 0 && g.Tiles[g.Robber].Resource == CatanSea {
		return errors.New("新世界强盗位置无效")
	}
	return nil
}

// This removes only prefixes which cannot possibly form a complete legal
// layout; every complete placement arrangement remains reachable. It does not
// preselect, move, or reserve any particular future ground or reveal its face.
func (g *Catan) fishingWorldPortFits(edge int) bool {
	q := g.fishingWorldSetup()
	if q == nil {
		return true
	}
	w := g.newWorld()
	if w == nil || w.Index >= len(w.Ports) || q.Index != 0 {
		return false
	}
	next := *g
	next.Ports = append(slices.Clone(g.Ports), CatanPort{Edge: edge, Resource: w.Ports[w.Index]})
	return next.fishingWorldCanFinish(len(w.Ports)-w.Index-1, len(q.Numbers))
}

func (g *Catan) worldFishCoasts() []CatanFishingCoast {
	result := []CatanFishingCoast{}
	w, q := g.newWorld(), g.fishingWorldSetup()
	if w == nil || q == nil || w.Index != len(w.Ports) || q.Index >= len(q.Numbers) {
		return result
	}
	for _, coast := range g.fishingCoasts(g.findIslands()) {
		next, fish := *g, *g.Fishing
		fish.Map.Grounds = append(slices.Clone(fish.Map.Grounds), fishingSeaGround(q.Numbers[q.Index], coast))
		next.Fishing = &fish
		if next.fishingWorldCanFinish(0, len(q.Numbers)-q.Index-1) {
			result = append(result, coast)
		}
	}
	return result
}

func (s *State) catanWorldFish(player int, a Action) error {
	g := s.Catan
	q := g.fishingWorldSetup()
	if q == nil || s.Phase != "catan_world_fish" || player != s.Turn || g.SetupStep != 0 || a.Type != "catan_world_fish" {
		return errors.New("请等待轮到自己安放当前渔场")
	}
	for _, coast := range g.worldFishCoasts() {
		if coast.Vertices[1] != a.Vertex {
			continue
		}
		number := q.Numbers[q.Index]
		g.Fishing.Map.Grounds = append(g.Fishing.Map.Grounds, fishingSeaGround(number, coast))
		q.Index++
		s.catanLog(player, "在海岸交点 #%d 安放%d点渔场（%d/%d）", a.Vertex+1, number, q.Index, len(q.Numbers))
		if q.Index == len(q.Numbers) {
			s.Turn, s.Phase = g.StartPlayer, "catan_setup_settlement"
		} else {
			s.Turn = (s.Turn + 1) % len(g.Players)
		}
		return nil
	}
	return errors.New("请选择亮起的海岸凹角，避开港口及已有渔场，并保留剩余渔场的位置")
}

func (s *State) catanWorldFishBot(player int) (Action, error) {
	if s.Phase != "catan_world_fish" || s.Turn != player {
		return Action{}, errors.New("当前不能安放新世界渔场")
	}
	coasts := s.Catan.worldFishCoasts()
	if len(coasts) == 0 {
		return Action{}, errors.New("新世界没有合法渔场位置")
	}
	return Action{Type: "catan_world_fish", Vertex: coasts[catanRandom(len(coasts))].Vertices[1]}, nil
}
