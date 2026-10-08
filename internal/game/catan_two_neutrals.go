package game

import (
	"errors"
	"slices"
)

// Neutral colors occupy the real board without becoming resource-owning seats.
// -1 remains the only empty road owner. Shared by base and river variants.
var catanTwoNeutralOwners = [2]int{-2, -3}

func (g *Catan) twoBoardDimensions() bool {
	if g.Transport != nil {
		return len(g.Vertices) == 57 && len(g.Edges) == 84
	}
	return len(g.Vertices) == 54 && len(g.Edges) == 72
}

func (g *Catan) prepareTwoNeutrals() error {
	if len(g.Players) != 2 || len(g.Tiles) != 19 || !g.twoBoardDimensions() || g.SetupStep != 0 || g.BaseSetup != nil || g.Seafarers != nil || g.CitiesKnights != nil || g.Fishing != nil && !g.twoFishing() || g.Caravans != nil && g.Rivers != nil || g.Options != (CatanOptions{}) || g.FriendlyRobber != nil || g.Harbors != nil || g.CardEvent != nil || g.RevealedEvent != nil {
		return errors.New("双人中立布局目前仅用于未开始的基础、渔夫、河流、商队或运输地图")
	}
	for _, v := range g.Vertices {
		if v.Level != 0 || v.Owner != -1 {
			return errors.New("中立布局不能覆盖已有建筑")
		}
	}
	for _, e := range g.Edges {
		if e.Owner != -1 {
			return errors.New("中立布局不能覆盖已有道路")
		}
	}
	// Official 2025 T&B p7: south corner of the top-middle hex and north
	// corner of the bottom-middle hex. No initial neutral roads.
	for i, tileCorner := range [][2]int{{1, 1}, {17, 4}} {
		id := g.Tiles[tileCorner[0]].Vertices[tileCorner[1]]
		g.Vertices[id].Owner, g.Vertices[id].Level = catanTwoNeutralOwners[i], 1
	}
	return nil
}

type catanTwoNeutralChoice struct {
	Owner  int `json:"owner"`
	Vertex int `json:"vertex"` // -1 for a road
	Edge   int `json:"edge"`   // -1 for a settlement
}

// A replacement road is allowed only if neither neutral color can place the
// requested settlement, including its five-piece limit. No city is built.
func (g *Catan) twoNeutralChoices(kind string) []catanTwoNeutralChoice {
	choices := []catanTwoNeutralChoice{}
	if kind == "knight" || kind == "knight_promote" {
		choices = g.twoKnightChoices(kind)
		if len(choices) > 0 || kind == "knight_promote" {
			return choices
		}
		kind = "road"
	}
	if kind == "ship" && g.twoSeafarers() {
		choices = g.twoNeutralShipChoices()
		if len(choices) > 0 {
			return choices
		}
		kind = "road"
	}
	if kind == "settlement" {
		for _, owner := range catanTwoNeutralOwners {
			_, villages, _ := g.pieces(owner)
			if villages >= 5 {
				continue
			}
			for _, v := range g.Vertices {
				if g.canSettlement(owner, v.ID, false) {
					choices = append(choices, catanTwoNeutralChoice{owner, v.ID, -1})
				}
			}
		}
		if len(choices) > 0 {
			return choices
		}
	} else if kind == "bridge" && g.Rivers != nil {
		for _, owner := range catanTwoNeutralOwners {
			for _, edge := range g.Rivers.Map.Bridges {
				if g.canBridge(owner, edge) {
					choices = append(choices, catanTwoNeutralChoice{owner, -1, edge})
				}
			}
		}
		if len(choices) > 0 {
			return choices
		}
	} else if kind != "road" {
		return choices
	}
	for _, owner := range catanTwoNeutralOwners {
		roads, _, _ := g.pieces(owner)
		if roads >= 15 {
			continue
		}
		for _, e := range g.Edges {
			if g.canRoad(owner, e.ID) {
				choices = append(choices, catanTwoNeutralChoice{owner, -1, e.ID})
			}
		}
	}
	return choices
}

func (g *Catan) placeTwoNeutral(kind string, choice catanTwoNeutralChoice) error {
	if !slices.Contains(g.twoNeutralChoices(kind), choice) {
		return errors.New("请选择中立势力的合法建设位置")
	}
	if g.twoAttackKnights() && (kind == "knight" || kind == "knight_promote") && slices.Contains(g.twoAttackKnightChoices(kind), choice) {
		c := g.Attack.City
		if kind == "knight" {
			c.Knights = append(c.Knights, catanAttackCityKnight{Owner: choice.Owner, Edge: choice.Edge, Strength: 1})
		} else {
			i := c.at(choice.Edge)
			c.Knights[i].Strength = 2
			c.Knights[i].PromotedAt = g.CitiesKnights.ActionSerial
		}
		return nil
	}
	if choice.Vertex >= 0 {
		if kind == "knight" {
			g.CitiesKnights.Knights = append(g.CitiesKnights.Knights, CatanKnight{Owner: choice.Owner, Vertex: choice.Vertex, Strength: 1})
		} else if kind == "knight_promote" {
			n := g.knightAt(choice.Vertex)
			n.Strength = 2
			n.PromotedAt = g.CitiesKnights.ActionSerial
		} else {
			g.Vertices[choice.Vertex].Owner, g.Vertices[choice.Vertex].Level = choice.Owner, 1
		}
	} else {
		ship := kind == "ship" && len(g.twoNeutralShipChoices()) > 0
		g.Edges[choice.Edge].Owner = choice.Owner
		g.Edges[choice.Edge].Ship = ship
		g.Edges[choice.Edge].Bridge = kind == "bridge" && g.Rivers != nil && slices.Contains(g.Rivers.Map.Bridges, choice.Edge)
	}
	return nil
}

// The ordinary tie rule also applies when a neutral color holds the award.
// Negative neutral owners must not be confused with the unclaimed value -1.
func (g *Catan) twoLongestOwner(old int) int {
	best := 4
	leaders := []int{}
	for _, owner := range []int{0, 1, -2, -3} {
		if owner >= 0 && (owner >= len(g.Players) || g.Players[owner].Eliminated) {
			continue
		}
		length := g.roadLength(owner)
		if length > best {
			best, leaders = length, []int{owner}
		} else if length == best && length >= 5 {
			leaders = append(leaders, owner)
		}
	}
	if slices.Contains(leaders, old) {
		return old
	}
	if len(leaders) == 1 {
		return leaders[0]
	}
	return -1
}

// FAQ 29 confirms that desert and coastal rewards add up to three, once per
// settlement, not per incident coastal edge. Neutral colors receive no tokens.
func (g *Catan) twoSettlementTokens(owner, vertex int) int {
	if owner < 0 || owner >= len(g.Players) || vertex < 0 || vertex >= len(g.Vertices) {
		return 0
	}
	reward := 0
	for _, tile := range g.Tiles {
		if (tile.Resource == CatanDesert || g.Rivers != nil && tile.Resource == catanSwamp) && slices.Contains(tile.Vertices, vertex) {
			reward = 2
			break
		}
	}
	// Commodity centers grant one token per settlement, including setup.
	// The initial city is not a settlement and earns no such token.
	if g.Transport != nil {
		for _, site := range g.Transport.Map.Sites {
			if slices.Contains(g.Tiles[site.Tile].Vertices, vertex) {
				reward++
				break
			}
		}
	}
	for _, edge := range g.touching(vertex) {
		interior := false
		if g.Transport != nil {
			for _, site := range g.Transport.Map.Sites {
				interior = interior || slices.Contains(site.Paths, edge)
			}
		}
		if !interior && ((g.twoSeafarers() && g.edgeTerrain(edge, true) && g.edgeTerrain(edge, false)) || len(g.edgeTiles(edge)) == 1) {
			return reward + 1
		}
	}
	return reward
}
