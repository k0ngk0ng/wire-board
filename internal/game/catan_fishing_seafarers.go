package game

import "errors"

// Internal entry point: the public room catalogue stays gated until complete
// expansion acceptance. Additional scenarios need their own board recipes and
// production continuations; never apply Four Islands to the six-island board.
func NewCatanFishingSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, placements []CatanFishingGroundPlacement) (*State, error) {
	if options.Helpers || options.AllHelpers {
		return nil, errors.New("捕鱼与助手组合尚未接入")
	}
	if setup.Scenario != "islands" || n < 3 || n > 4 {
		return nil, errors.New("此捕鱼航海家剧本或人数尚未接入")
	}
	s, err := NewCatanSeafarers(n, options, setup, nil)
	if err != nil {
		return nil, err
	}
	g := s.Catan
	m, err := g.makeFishingFourIslands(placements)
	if err != nil {
		return nil, err
	}
	tokens, err := newCatanFishingTokens(n)
	if err != nil {
		return nil, err
	}
	g.Fishing = &CatanFishing{Map: *m, Tokens: *tokens, LastRollID: -1, Started: make([]bool, n)}
	// The 2025 combination explicitly uses scenario setup except for the
	// listed lake/grounds changes: retain Four Islands' robber and pirate.
	s.Log = append(s.Log, "四岛捕鱼：不放湖泊，保留四岛起始强盗与海盗；2鱼可驱离海盗，5鱼可造船；13分获胜，持旧靴子需14分")
	return s, nil
}

func (g *Catan) fishingSeaSupported() bool {
	return g.Seafarers != nil && g.Seafarers.Scenario == "islands" &&
		len(g.Players) >= 3 && len(g.Players) <= 4 && g.CitiesKnights == nil
}

func (g *Catan) fishCanRemovePirate(player int) bool {
	return g.Seafarers != nil && g.Seafarers.Pirate >= 0 && g.pirateAllowed(player)
}
