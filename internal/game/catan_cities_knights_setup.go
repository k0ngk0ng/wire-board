package game

import "fmt"

type CatanCitiesKnightsSetup struct {
	Layout string `json:"layout"`
	Rules  string `json:"rules"`
}

const CatanCitiesKnightsFiveSixRules = "catan-knights-5-6-2025"

func catanCitiesKnightsRules(n int) string {
	if n > 4 {
		return CatanCitiesKnightsFiveSixRules
	}
	return CatanCitiesKnightsRules
}
func NormalizeCatanCitiesKnightsSetup(n int, setup CatanCitiesKnightsSetup) (CatanCitiesKnightsSetup, error) {
	if n < 3 || n > 6 {
		return setup, fmt.Errorf("城市与骑士需要3至6位玩家")
	}
	rules := catanCitiesKnightsRules(n)
	if setup.Rules != "" && setup.Rules != rules {
		return setup, fmt.Errorf("城市与骑士规则版本与人数不一致")
	}
	if setup.Layout == "" {
		setup.Layout = "variable"
	}
	if setup.Layout != "variable" {
		return setup, fmt.Errorf("该城市与骑士布局尚未接入")
	}
	setup.Rules = rules
	return setup, nil
}
func NewCatanCitiesKnightsConfigured(n int, options CatanOptions, setup CatanCitiesKnightsSetup) (*State, error) {
	normalized, err := NormalizeCatanCitiesKnightsSetup(n, setup)
	if err != nil {
		return nil, err
	}
	s, err := NewCatanCitiesKnights(n, options)
	if err != nil {
		return nil, err
	}
	s.Catan.CitiesKnights.Layout = normalized.Layout
	return s, nil
}

// Old internal saves have the base rules ID even in five/six-player games, and
// predate the layout field. Resolve from the actual game, never room drafts.
func (g *Catan) CitiesKnightsSetup() CatanCitiesKnightsSetup {
	if g.CitiesKnights == nil {
		return CatanCitiesKnightsSetup{}
	}
	setup := CatanCitiesKnightsSetup{Rules: g.CitiesKnights.Rules, Layout: g.CitiesKnights.Layout}
	if setup.Layout == "" {
		setup.Layout = "variable"
	}
	if setup.Rules == "" || setup.Rules == CatanCitiesKnightsRules {
		setup.Rules = catanCitiesKnightsRules(len(g.Players))
	}
	return setup
}
