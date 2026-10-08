package game

import "errors"

const CatanFriendlyRobberRules = "catan-friendly-robber-2025"
const CatanFriendlyKnightsRules = "wire-board-friendly-knights-v1"
const CatanFriendlySeaFallbackRules = "wire-board-friendly-sea-fallback-v1"

const catanFriendlySeaNotice = "本站补充规则：没有合法陆地且没有符合剧本限制的沙漠时，强盗退到场外且不偷牌；海盗已在外海且没有合法海洋时可留在外海"

type CatanFriendlyRobber struct {
	Rules    string `json:"rules"`
	Fallback string `json:"fallback,omitempty"`
	Knights  string `json:"knights,omitempty"`
}

// Internal base variant constructor. Special sea fallback/fleet interactions
// and other expansion configuration remain subject to combination acceptance.
func NewCatanFriendlyRobber(n int, options CatanOptions, setup CatanBaseConfiguration) (*State, error) {
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
	if s.Catan.CitiesKnights != nil {
		s.Catan.FriendlyRobber.Knights = CatanFriendlyKnightsRules
		s.Log = append(s.Log, "本站组合规则：首次蛮族入侵前强盗与海盗仍休眠；征税和骑士驱逐遵守友善保护，城市失守后按最新公开分数重新判断保护")
	}
	if s.Catan.Seafarers != nil {
		s.Catan.FriendlyRobber.Fallback = CatanFriendlySeaFallbackRules
		s.Log = append(s.Log, catanFriendlySeaNotice)
	}
}

func (s *State) validateCatanFriendlyFallback() error {
	g := s.Catan
	if g != nil && g.FriendlyRobber != nil && g.FriendlyRobber.Knights != "" && (g.FriendlyRobber.Knights != CatanFriendlyKnightsRules || g.CitiesKnights == nil || g.FriendlyRobber.Rules != CatanFriendlyRobberRules) {
		return errors.New("友善强盗的城市骑士组合配置无效")
	}
	if g == nil || g.FriendlyRobber == nil || g.FriendlyRobber.Fallback == "" {
		return nil // Preserve older saves and the base variant.
	}
	if g.FriendlyRobber.Fallback != CatanFriendlySeaFallbackRules || g.FriendlyRobber.Rules != CatanFriendlyRobberRules || g.Seafarers == nil {
		return errors.New("友善强盗的航海退路配置无效")
	}
	return nil
}

func (g *Catan) friendlySeaFallback() bool {
	return g.Seafarers != nil && g.FriendlyRobber != nil && g.FriendlyRobber.Fallback == CatanFriendlySeaFallbackRules
}

func (g *Catan) friendlyRobberOutsideAllowed() bool {
	if !g.friendlySeaFallback() || g.Attack != nil || g.pirateIslands() != nil || (g.CitiesKnights != nil && (g.CitiesKnights.Invasions == 0 || g.CitiesKnights.Chase == "pirate")) {
		return false
	}
	for _, t := range g.Tiles {
		if g.robberLandAllowed(t.ID) && (t.Resource == CatanDesert || (t.ID != g.Robber && !g.friendlyRobberBlocks(t.ID))) {
			return false
		}
	}
	return true
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
	if !g.pirateAllowed(player) || tile < -1 || tile >= len(g.Tiles) {
		return false
	}
	if tile == g.Seafarers.Pirate {
		if tile != -1 || !g.friendlySeaFallback() {
			return false
		}
		for _, t := range g.Tiles {
			if t.Resource == CatanSea && !g.friendlyPirateBlocks(t.ID) {
				return false
			}
		}
		return true
	}
	return tile < 0 || g.Tiles[tile].Resource == CatanSea && !g.friendlyPirateBlocks(tile)
}
