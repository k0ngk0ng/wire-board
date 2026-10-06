package game

import "errors"

// Roads only need the incident hexes explored. A third hex touching an
// endpoint need not be explored until a building is placed at that vertex.
func catanExplorerLandEdge(g *Catan, edge int) bool {
	if g == nil || edge < 0 || edge >= len(g.Edges) {
		return false
	}
	land := false
	for _, tile := range g.Edges[edge].Tiles {
		if tile < 0 || tile >= len(g.Tiles) {
			return false
		}
		h := g.Tiles[tile]
		if h.Resource < 0 || h.Resource > CatanGold || h.Resource == CatanFog || h.Resource == CatanGold && h.Number == 0 {
			return false
		}
		land = land || h.Resource < CatanSea || h.Resource == CatanGold
	}
	return land
}

func (c *catanExplorerCargo) buildRoad(g *Catan, f *catanExplorerSailing, player int, sequence uint64, edge int) error {
	if err := c.allowed(g, f, player, sequence, "action"); err != nil {
		return err
	}
	if !catanExplorerLandEdge(g, edge) || g.Edges[edge].Owner != -1 {
		return errors.New("道路必须建在已探索、未占用的可建设陆地边")
	}
	count := 0
	for _, road := range g.Edges {
		if road.Owner == player {
			count++
		}
	}
	connected := false
	target := g.Edges[edge]
	for _, at := range []int{target.A, target.B} {
		v := g.Vertices[at]
		if v.Owner == player && v.Level > 0 {
			connected = true
		} else if v.Level == 0 && v.Owner == -1 {
			for _, road := range g.Edges {
				if road.Owner == player && (road.A == at || road.B == at) {
					connected = true
				}
			}
		}
	}
	cost := []int{1, 1, 0, 0, 0}
	if !connected || count >= 15 || !catanExplorerCanPay(g, player, cost) {
		return errors.New("修路需要连接己方道路或建筑、剩余道路棋子及1木1砖；不能穿过对手或中立建筑")
	}
	catanExplorerPay(g, player, cost)
	g.Edges[edge].Owner = player
	return nil
}

func (c *catanExplorerCargo) buildSettlement(g *Catan, f *catanExplorerSailing, player int, sequence uint64, vertex int) error {
	if err := c.allowed(g, f, player, sequence, "action"); err != nil {
		return err
	}
	if !catanExplorerLandVertex(g, vertex) || g.Vertices[vertex].Level != 0 {
		return errors.New("村庄必须建在已探索的空陆地点，不能接触迷雾或未解放金矿")
	}
	connected, count := false, 0
	for _, v := range g.Vertices {
		if v.Owner == player && v.Level == 1 {
			count++
		}
	}
	for _, edge := range g.Edges {
		other := -1
		if edge.A == vertex {
			other = edge.B
		} else if edge.B == vertex {
			other = edge.A
		}
		if other < 0 {
			continue
		}
		if g.Vertices[other].Level != 0 {
			return errors.New("新村庄必须与所有建筑至少相隔两条边")
		}
		connected = connected || edge.Owner == player
	}
	cost := []int{1, 1, 1, 1, 0}
	if !connected || count >= 5 || !catanExplorerCanPay(g, player, cost) {
		return errors.New("建村需要连接己方道路、剩余村庄棋子及木砖羊粮各1")
	}
	catanExplorerPay(g, player, cost)
	g.Vertices[vertex].Owner, g.Vertices[vertex].Level = player, 1
	g.Players[player].Score++
	return nil
}
