package game

import (
	"errors"
	"reflect"
	"slices"
)

const CatanAttackSeafarersRules = "catan-attack-seafarers-2025"

// Official combination p2: install the ordinary Barbarian Attack island
// into the four-player New Shores frame, preserving the printed outer islands.
func newCatanAttackShoresBoard(n int) (*Catan, *catanAttackMap, error) {
	if n == 3 {
		return newCatanAttackShoresThreeBoard()
	}
	if n < 4 || n > 6 {
		return nil, nil, errors.New("此蛮族新海岸配方需要四人")
	}
	layout := "fixed"
	if n > 4 {
		layout = "variable"
	}
	s, err := NewCatanSeafarers(n, CatanOptions{FiveSix: n > 4}, CatanSeafarersSetup{Scenario: "shores", Layout: layout}, nil)
	if err != nil {
		return nil, nil, err
	}
	base, m, err := newCatanAttackBoard(n)
	if err != nil {
		return nil, nil, err
	}
	g := s.Catan
	ids := []int{12, 13, 14, 18, 19, 20, 21, 24, 25, 26, 27, 28, 31, 32, 33, 34, 37, 38, 39}
	if n > 4 {
		ids = riverShoresMainlandIDs()
	}
	for i, id := range ids {
		g.Tiles[id].Resource, g.Tiles[id].Number = base.Tiles[i].Resource, base.Tiles[i].Number
	}
	remap := func(values []int) []int {
		out := make([]int, len(values))
		for i, id := range values {
			out[i] = ids[id]
		}
		return out
	}
	mapped := &catanAttackMap{Sea: CatanAttackSeafarersRules, Castles: remap(m.Castles), Coast: remap(m.Coast), Barbarians: m.Barbarians, Gold: m.Gold}
	// Transfer printed attack ports by hex-side identity, not vertex numbering.
	g.Ports = nil
	for _, p := range base.Ports {
		found := false
		for _, tile := range base.Edges[p.Edge].Tiles {
			for side := 0; side < 6; side++ {
				if catanFishingSide(base, tile, side) == p.Edge {
					edge := catanFishingSide(g, ids[tile], side)
					if edge < 0 {
						return nil, nil, errors.New("蛮族海岸港口映射失败")
					}
					g.Ports = append(g.Ports, CatanPort{Edge: edge, Resource: p.Resource})
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return nil, nil, errors.New("蛮族海岸原港口缺失")
		}
	}
	g.Robber, g.Seafarers.Pirate = -1, -1
	g.Seafarers.Islands = g.findIslands()
	g.Seafarers.StartIslands = []int{g.Seafarers.Islands[ids[len(ids)/2]]}
	g.Seafarers.VictoryPoints = 14
	return g, mapped, nil
}

func (g *Catan) attackSea() bool {
	return g.Seafarers != nil && g.Attack != nil && g.Attack.Map != nil && g.Attack.Map.Sea == CatanAttackSeafarersRules
}
func (g *Catan) attackSeaKnightEdge(edge int) bool {
	if g.Seafarers == nil {
		return true
	}
	if edge < 0 || edge >= len(g.Edges) {
		return false
	}
	for _, id := range g.Edges[edge].Tiles {
		if g.Tiles[id].Resource != CatanSea && g.Seafarers.Islands[id] == g.Seafarers.StartIslands[0] {
			return true
		}
	}
	return false
}
func NewCatanAttackShores(n int) (*State, error) {
	if n == 2 {
		return newCatanTwoAttackShores()
	}
	return newCatanAttackShores(n)
}

func newCatanAttackShores(n int) (*State, error) {
	board, m, err := newCatanAttackShoresBoard(n)
	if err != nil {
		return nil, err
	}
	s, err := NewCatan(n, CatanOptions{FiveSix: n > 4})
	if err != nil {
		return nil, err
	}
	g := s.Catan
	g.Tiles, g.Vertices, g.Edges, g.Ports, g.HexSize, g.Seafarers = board.Tiles, board.Vertices, board.Edges, board.Ports, board.HexSize, board.Seafarers
	g.Robber = -1
	g.DevDeck, g.DevDiscard = []int{}, []int{}
	g.Attack, err = newCatanAttackPieces(g, m)
	if err != nil {
		return nil, err
	}
	s.Log = []string{"蛮族新海岸：先建村庄再逆序建城市；蛮族与骑士仅在主岛活动，不使用强盗和海盗，14分获胜"}
	if n > 4 {
		s.Log = append(s.Log, "本站五六人蛮族新海岸：扩大蛮族30格主岛嵌入扩大海图，保留外岛，48名蛮族、152金币、双城堡和配对回合；14分获胜。")
	}
	s.catanScores()
	return s, s.validateCatanAttack()
}
func (m catanAttackMap) validateSea(g *Catan) error {
	if g.Seafarers != nil && g.Seafarers.Scenario == "pirate_islands" {
		return m.validatePirateSea(g)
	}
	if g.Seafarers != nil && g.Seafarers.Scenario == "wonders" {
		return m.validateWondersSea(g)
	}
	if g.Seafarers != nil && g.Seafarers.Scenario == "tribe" {
		return m.validateTribeSea(g)
	}
	if m.Sea != CatanAttackSeafarersRules || m.Caravans != "" || m.Rivers != "" || m.Transport != "" || (len(g.Players) < 2 || len(g.Players) > 6) || g.Seafarers == nil {
		return errors.New("蛮族海图配置无效")
	}
	recipeSeats := len(g.Players)
	if recipeSeats == 2 {
		recipeSeats = 4
	}
	ref, want, err := newCatanAttackShoresBoard(recipeSeats)
	desert := g.Seafarers.Scenario == "desert"
	if desert {
		if len(g.Players) < 2 || len(g.Players) > 6 {
			return errors.New("蛮族沙漠人数未接通")
		}
		if len(g.Players) == 3 {
			ref, want, err = newCatanAttackDesertThreeBoard()
		} else {
			ref, want, err = newCatanAttackDesertFourBoard()
			if len(g.Players) > 4 {
				ref, want, err = newCatanAttackDesertExtendedBoard(len(g.Players))
			}
		}
	}
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(m, *want) || len(g.Tiles) != len(ref.Tiles) || len(g.Edges) != len(ref.Edges) || len(g.Vertices) != len(ref.Vertices) || len(g.Ports) != len(ref.Ports) {
		return errors.New("蛮族海图组件无效")
	}
	gotPorts, wantPorts := []int{}, []int{}
	for i, p := range g.Ports {
		if p.Edge != ref.Ports[i].Edge {
			return errors.New("蛮族海图港口位置无效")
		}
		gotPorts = append(gotPorts, p.Resource)
		wantPorts = append(wantPorts, ref.Ports[i].Resource)
	}
	slices.Sort(gotPorts)
	slices.Sort(wantPorts)
	if !slices.Equal(gotPorts, wantPorts) {
		return errors.New("蛮族海图港口库存无效")
	}
	ids := []int{12, 13, 14, 18, 19, 20, 21, 24, 25, 26, 27, 28, 31, 32, 33, 34, 37, 38, 39}
	if len(g.Players) == 3 {
		ids = []int{10, 11, 15, 16, 17, 20, 21, 22, 23, 26, 27, 28, 31, 32}
	}
	if len(g.Players) > 4 {
		ids = riverShoresMainlandIDs()
	}
	if desert {
		ids = nil
	}
	counts := [2][5]int{}
	for i, tile := range g.Tiles {
		w := ref.Tiles[i]
		if slices.Contains(ids, i) && tile.Resource >= 0 && tile.Resource < 5 {
			region := 1
			if slices.Contains(m.Coast, i) {
				region = 0
			}
			counts[region][tile.Resource]++
			tile.Resource = w.Resource
		}
		if !reflect.DeepEqual(tile, w) {
			return errors.New("蛮族海图地形数字无效")
		}
	}
	recipe := catanAttackBoardRecipe(len(g.Players) > 4)
	if len(g.Players) == 3 {
		recipe.coastalResources = [5]int{2, 2, 2, 2, 1}
		recipe.innerResources = [5]int{1, 0, 1, 1, 1}
	}
	if !desert && (counts[0] != recipe.coastalResources || counts[1] != recipe.innerResources) {
		return errors.New("蛮族海图地形库存无效")
	}
	for i, v := range g.Vertices {
		w := ref.Vertices[i]
		v.Owner, v.Level = w.Owner, w.Level
		if !reflect.DeepEqual(v, w) {
			return errors.New("蛮族海图交点无效")
		}
	}
	for i, e := range g.Edges {
		w := ref.Edges[i]
		e.Owner, e.Ship, e.Bridge, e.Damaged, e.Warship = w.Owner, w.Ship, w.Bridge, w.Damaged, w.Warship
		if !reflect.DeepEqual(e, w) {
			return errors.New("蛮族海图边无效")
		}
	}
	sea, w := g.Seafarers, ref.Seafarers
	if sea.Scenario != w.Scenario || sea.Rules != w.Rules || sea.Layout != w.Layout || sea.Variable != w.Variable || sea.Fog != nil || sea.Tribe != nil || sea.Cloth != nil || sea.Wonders != nil || sea.PirateIslands != nil || sea.NewWorld != nil || sea.NumberRecipe != w.NumberRecipe || sea.Pirate != -1 || sea.VictoryPoints != g.attackSeaVictoryPoints(14) || sea.IslandBonus != 2 || len(sea.Seats) != len(g.Players) || !slices.Equal(sea.Islands, w.Islands) || !slices.Equal(sea.StartIslands, w.StartIslands) {
		return errors.New("蛮族海图分区无效")
	}
	return nil
}
