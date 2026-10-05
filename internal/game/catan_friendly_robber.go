package game

import "errors"

const CatanFriendlyRobberRules = "catan-friendly-robber-2025"

type CatanFriendlyRobber struct {
	Rules string `json:"rules"`
}

// Internal base variant constructor. Special sea fallback/fleet interactions
// and other expansion configuration remain subject to combination acceptance.
func NewCatanFriendlyRobber(n int, options CatanOptions, setup CatanBaseConfiguration) (*State, error) {
	if options.Helpers || options.AllHelpers {
		return nil, errors.New("友善强盗与助手的组合尚未核验")
	}
	s, err := NewCatanConfigured(n, options, setup)
	if err != nil {
		return nil, err
	}
	s.enableCatanFriendlyRobber()
	return s, nil
}
func (s *State) enableCatanFriendlyRobber() {
	if s.Catan.FriendlyRobber != nil {
		return
	}
	s.Catan.FriendlyRobber = &CatanFriendlyRobber{Rules: CatanFriendlyRobberRules}
	s.Log = append(s.Log, "加入友善强盗：公开分数不足3分的玩家受到保护，隐藏胜利点不计；无合法陆地时强盗返回沙漠")
}

// Official T&B FAQ 54–55: use visible points, including the active player's
// buildings. Neither a hidden VP card nor the size of a hand removes protection.
func (g *Catan) friendlyProtected(player int) bool {
	return g.FriendlyRobber != nil && player >= 0 && player < len(g.Players) && !g.Players[player].Eliminated && g.Players[player].Score-g.hiddenVictoryPoints(player) < 3
}
func (g *Catan) friendlyRobberBlocks(tile int) bool {
	if g.FriendlyRobber == nil {
		return false
	}
	for _, id := range g.Tiles[tile].Vertices {
		v := g.Vertices[id]
		if v.Level > 0 && g.friendlyProtected(v.Owner) {
			return true
		}
	}
	return false
}
func (g *Catan) friendlyPirateBlocks(tile int) bool {
	if g.FriendlyRobber == nil || tile < 0 {
		return false
	}
	for _, e := range g.Edges {
		if !e.Ship || !g.friendlyProtected(e.Owner) {
			continue
		}
		for _, id := range g.edgeTiles(e.ID) {
			if id == tile {
				return true
			}
		}
	}
	return false
}

// Shared by actions, legal hints and bots. The sea rim has no adjacent ships.
func (g *Catan) pirateDestinationAllowed(player, tile int) bool {
	if !g.pirateAllowed(player) || tile < -1 || tile >= len(g.Tiles) || tile == g.Seafarers.Pirate {
		return false
	}
	return tile < 0 || g.Tiles[tile].Resource == CatanSea && !g.friendlyPirateBlocks(tile)
}
