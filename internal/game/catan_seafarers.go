package game

import (
	"errors"
	"slices"
)

const (
	CatanDesert = 5
	CatanSea    = 6
	CatanGold   = 7
	CatanFog    = 8
)

// These limits are per action phase, including the second paired player.
// Keeping them in the save prevents refresh/restart from moving a second ship.
type CatanSeafarerSeat struct {
	HomeIslands    []int `json:"homeIslands,omitempty"`
	SettledIslands []int `json:"settledIslands,omitempty"`
	IslandPoints   int   `json:"islandPoints,omitempty"`
}

type CatanSeafarers struct {
	NumberRecipe  string              `json:"numberRecipe,omitempty"`
	Rules         string              `json:"rules,omitempty"`
	Layout        string              `json:"layout,omitempty"`
	NewWorld      *CatanNewWorld      `json:"newWorld,omitempty"`
	Wonders       *CatanWonders       `json:"wonders,omitempty"`
	PirateIslands *CatanPirateIslands `json:"pirateIslands,omitempty"`
	Cloth         *CatanClothState    `json:"cloth,omitempty"`
	Tribe         *CatanTribeState    `json:"tribe,omitempty"`
	Fog           *CatanFogState      `json:"fog,omitempty"`
	Scenario      string              `json:"scenario,omitempty"`
	Variable      bool                `json:"variable,omitempty"`
	VictoryPoints int                 `json:"victoryPoints,omitempty"`
	IslandBonus   int                 `json:"islandBonus,omitempty"`
	Islands       []int               `json:"islands,omitempty"`
	StartIslands  []int               `json:"startIslands,omitempty"`
	Seats         []CatanSeafarerSeat `json:"seats,omitempty"`
	Pirate        int                 `json:"pirate"` // -1 means the frame, where no edges are blocked.
	BuiltShips    []int               `json:"builtShips,omitempty"`
	MovedShip     bool                `json:"movedShip,omitempty"`
}

func (g *Catan) edgeTiles(edge int) []int {
	result := []int{}
	if edge < 0 || edge >= len(g.Edges) {
		return result
	}
	e := g.Edges[edge]
	if len(e.Tiles) > 0 {
		return e.Tiles
	}
	for _, t := range g.Tiles {
		if slices.Contains(t.Vertices, e.A) && slices.Contains(t.Vertices, e.B) {
			result = append(result, t.ID)
		}
	}
	return result
}
func (g *Catan) edgeTerrain(edge int, ship bool) bool {
	if g.riversSea() && slices.Contains(g.Rivers.Map.Bridges, edge) {
		return false
	}
	if g.Seafarers == nil {
		return !ship
	}
	tiles := g.edgeTiles(edge)
	// A coastal edge along the sea frame also accepts a ship. The frame
	// itself has no extra intersections and is not a pirate-blocked hex.
	if ship && len(tiles) == 1 && g.Tiles[tiles[0]].Resource != CatanFog {
		return true
	}
	for _, id := range tiles {
		resource := g.Tiles[id].Resource
		if ship && resource == CatanSea {
			return true
		}
		if !ship && resource != CatanSea && resource != CatanFog {
			return true
		}
	}
	return false
}
func (g *Catan) landVertex(v int) bool {
	if g.Seafarers == nil {
		return true
	}
	for _, t := range g.Tiles {
		if t.Resource != CatanSea && t.Resource != CatanFog && g.tribeLand(t.ID) && g.clothLand(t.ID) && slices.Contains(t.Vertices, v) {
			return true
		}
	}
	return false
}
func (g *Catan) pirateBlocks(edge int) bool {
	return g.Seafarers != nil && g.Seafarers.Pirate >= 0 && slices.Contains(g.edgeTiles(edge), g.Seafarers.Pirate)
}
func (g *Catan) canRoute(p, id int, ship bool) bool {
	if !ship && g.hasDamagedRoad(p) {
		return false
	}
	if id < 0 || id >= len(g.Edges) || g.Edges[id].Owner != -1 || !g.edgeTerrain(id, ship) || (ship && g.pirateBlocks(id)) {
		return false
	}
	e := g.Edges[id]
	if ship && g.pirateIslands() != nil {
		if _, _, ok := g.pirateShipPlan(p, id); !ok {
			return false
		}
	}
	for _, v := range []int{e.A, e.B} {
		if g.opponentPiece(p, v) {
			continue
		}
		vertex := g.Vertices[v]
		if vertex.Level > 0 {
			if vertex.Owner == p {
				return true
			}
			continue
		}
		for _, next := range g.touching(v) {
			route := g.Edges[next]
			if route.Owner == p && route.Ship == ship {
				return true
			}
		}
	}
	return false
}
func (g *Catan) canShip(p, id int) bool { return g.Seafarers != nil && g.canRoute(p, id, true) }
func (g *Catan) shipCount(p int) int {
	n := 0
	for _, e := range g.Edges {
		if e.Owner == p && e.Ship {
			n++
		}
	}
	return n
}
func (g *Catan) hasRoute(p int) bool {
	if g.hasRoad(p) {
		return true
	}
	if g.shipCount(p) >= 15 {
		return false
	}
	for _, e := range g.Edges {
		if g.canShip(p, e.ID) {
			return true
		}
	}
	return false
}
func (g *Catan) setupRoute(p, id int, ship bool) bool {
	if id < 0 || id >= len(g.Edges) || g.Edges[id].Owner != -1 || !g.edgeTerrain(id, ship) || (ship && g.pirateBlocks(id)) {
		return false
	}
	e := g.Edges[id]
	return (e.A == g.SetupVertex || e.B == g.SetupVertex) && ((!ship && g.canRoad(p, id)) || (ship && g.canShip(p, id)))
}
func (g *Catan) movableShip(p, id int) bool {
	if g.riversSea() && g.riverEdge(id) && (p < 0 || p >= len(g.Players) || g.Rivers.Gold[p] < 1) {
		return false
	}
	if g.Seafarers == nil || g.Seafarers.MovedShip || id < 0 || id >= len(g.Edges) || g.Edges[id].Owner != p || !g.Edges[id].Ship || g.pirateBlocks(id) || slices.Contains(g.Seafarers.BuiltShips, id) {
		return false
	}
	return g.openRoute(p, id) && g.preservesKnightConnections(p, id)
}
func (g *Catan) shipDestinations(p, from int) []int {
	result := []int{}
	if !g.movableShip(p, from) {
		return result
	}
	temp := *g
	temp.Edges = append([]CatanEdge{}, g.Edges...)
	if g.pirateIslands() != nil {
		sea := *g.Seafarers
		pirates := *g.pirateIslands()
		pirates.Fortresses = append([]CatanPirateFortress{}, pirates.Fortresses...)
		sea.PirateIslands = &pirates
		temp.Seafarers = &sea
		if !temp.pirateRemoveRouteTail(p, from) {
			return result
		}
	}
	temp.Edges[from].Owner = -1
	temp.Edges[from].Ship = false
	temp.Edges[from].Warship = false
	for _, e := range temp.Edges {
		if e.ID != from && temp.canShip(p, e.ID) {
			result = append(result, e.ID)
		}
	}
	return result
}
func (s *State) catanMoveShip(player int, a Action) error {
	g := s.Catan
	if s.Phase != "catan_turn" || !slices.Contains(g.shipDestinations(player, a.Edge), a.Target) {
		return errors.New("只能移动本阶段尚未移动过、非本阶段新造且处于开放末端的船；海盗附近不能移入或移出")
	}
	if g.riversSea() && g.riverEdge(a.Edge) {
		g.Rivers.Gold[player]--
		g.Rivers.Bank++
		s.catanLog(player, "移走河岸船只，退还金币×1")
	}
	warship := g.Edges[a.Edge].Warship
	if !g.pirateRemoveRouteTail(player, a.Edge) {
		return errors.New("远征航线只能移动末端船只")
	}
	g.Edges[a.Edge].Owner = -1
	g.Edges[a.Edge].Ship = false
	g.Edges[a.Edge].Warship = false
	g.Edges[a.Target].Owner = player
	g.Edges[a.Target].Ship = true
	g.Edges[a.Target].Warship = warship
	g.Seafarers.MovedShip = true
	g.Trade = nil
	s.catanLog(player, "将船只从 #%d 移至 #%d", a.Edge+1, a.Target+1)
	return s.catanAfterRoute(CatanRouteCompletion{Player: player, Edge: a.Target})
}
func (s *State) catanMovePirate(player, tile int) error {
	g := s.Catan
	if !g.pirateAllowed(player) {
		return errors.New("必须先与布匹村落建立贸易，才能移动海盗")
	}
	if s.Phase != "catan_robber" || !g.pirateDestinationAllowed(player, tile) {
		if g.FriendlyRobber != nil {
			return errors.New("请选择合法的海盗位置；友善海盗不能影响公开分数不足3分的船主")
		}
		return errors.New("请将海盗移至另一块海洋或地图外框")
	}
	g.Seafarers.Pirate = tile
	if g.CitiesKnights != nil {
		g.CitiesKnights.Chase = ""
	}
	g.Victims = []int{}
	for _, e := range g.Edges {
		if e.Ship && e.Owner >= 0 && e.Owner != player && !g.Players[e.Owner].Eliminated && !g.friendlyProtected(e.Owner) && (sum(g.Players[e.Owner].Resources) > 0 || (g.cloth() != nil && g.cloth().Held[e.Owner] > 0)) && g.pirateBlocks(e.ID) && !slices.Contains(g.Victims, e.Owner) {
			g.Victims = append(g.Victims, e.Owner)
		}
	}
	if tile < 0 {
		s.catanLog(player, "将海盗移至地图外框")
	} else {
		s.catanLog(player, "将海盗移至海洋 #%d", tile+1)
	}
	if len(g.Victims) == 0 {
		s.Phase = g.ResumePhase
		return nil
	}
	if g.cloth() != nil {
		s.Phase = "catan_cloth_steal"
		return nil
	}
	s.Phase = "catan_steal"
	if len(g.Victims) == 1 {
		return s.catanSteal(player, g.Victims[0])
	}
	return nil
}

func (g *Catan) findIslands() []int { return g.findLandRegions(false) }

func (g *Catan) findLandRegions(splitDeserts bool) []int {
	isLand := func(resource int) bool {
		return resource != CatanSea && resource != CatanFog && (!splitDeserts || resource != CatanDesert)
	}
	islands := make([]int, len(g.Tiles))
	for i := range islands {
		islands[i] = -1
	}
	nextIsland := 0
	for _, tile := range g.Tiles {
		if islands[tile.ID] >= 0 || !isLand(tile.Resource) {
			continue
		}
		queue := []int{tile.ID}
		islands[tile.ID] = nextIsland
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			for _, e := range g.Edges {
				tiles := g.edgeTiles(e.ID)
				if !slices.Contains(tiles, id) {
					continue
				}
				for _, neighbor := range tiles {
					if islands[neighbor] < 0 && isLand(g.Tiles[neighbor].Resource) {
						islands[neighbor] = nextIsland
						queue = append(queue, neighbor)
					}
				}
			}
		}
		nextIsland++
	}
	return islands
}
func (g *Catan) islandAt(v int) int {
	if g.Seafarers == nil {
		return -1
	}
	for _, t := range g.Tiles {
		if t.ID < len(g.Seafarers.Islands) && g.Seafarers.Islands[t.ID] >= 0 && slices.Contains(t.Vertices, v) {
			return g.Seafarers.Islands[t.ID]
		}
	}
	return -1
}
func (g *Catan) seaSetupAllowed(v int) bool {
	if w := g.wonders(); w != nil && slices.Contains(w.SetupBlocked, v) {
		return false
	}
	if g.Seafarers != nil && g.Seafarers.Fog != nil {
		for _, id := range g.Seafarers.Fog.StartTiles {
			if id >= 0 && id < len(g.Tiles) && slices.Contains(g.Tiles[id].Vertices, v) {
				return true
			}
		}
		return false
	}
	return g.Seafarers == nil || len(g.Seafarers.StartIslands) == 0 || slices.Contains(g.Seafarers.StartIslands, g.islandAt(v))
}
func (s *State) catanSettleIsland(player, v int, setup bool) {
	g := s.Catan
	if g.Seafarers == nil || g.Seafarers.Fog != nil || len(g.Seafarers.Islands) == 0 {
		return
	}
	island := g.islandAt(v)
	if island < 0 {
		return
	}
	if len(g.Seafarers.Seats) < len(g.Players) {
		g.Seafarers.Seats = append(g.Seafarers.Seats, make([]CatanSeafarerSeat, len(g.Players)-len(g.Seafarers.Seats))...)
	}
	seat := &g.Seafarers.Seats[player]
	if setup && !slices.Contains(seat.HomeIslands, island) {
		seat.HomeIslands = append(seat.HomeIslands, island)
	}
	if !slices.Contains(seat.SettledIslands, island) {
		seat.SettledIslands = append(seat.SettledIslands, island)
		if !setup && !slices.Contains(seat.HomeIslands, island) && g.Seafarers.IslandBonus > 0 {
			seat.IslandPoints += g.Seafarers.IslandBonus
			region := "岛屿"
			if g.Seafarers.Scenario == "desert" {
				region = "区域"
			}
			s.catanLog(player, "首次在新的%s建造村庄，额外获得 %d 分", region, g.Seafarers.IslandBonus)
		}
	}
}
