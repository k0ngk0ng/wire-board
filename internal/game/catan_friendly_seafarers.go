package game

import "errors"

// The remaining sea combinations stay in the acceptance backlog: maps with
// no desert and the Forgotten Tribe's numbered-land restriction need a
// separately verified interpretation of the friendly robber fallback.
func CatanFriendlySeafarersSupported(n int, scenario string) bool {
	if n < 3 || n > 6 {
		return false
	}
	switch scenario {
	case "shores":
		return n >= 4
	case "desert", "wonders", "cloth", "pirate_islands":
		return true
	}
	return false
}

// Internal constructor only. Normal scenario setup, scoring and pirate rules
// still apply. Cloth and Pirate Islands begin play at three building points;
// without Cities & Knights/Helpers those points never decrease.
func NewCatanFriendlySeafarers(n int, options CatanOptions, setup CatanSeafarersSetup) (*State, error) {
	if !CatanFriendlySeafarersSupported(n, setup.Scenario) {
		return nil, errors.New("该航海家剧本的友善强盗组合尚未核验")
	}
	if options.Helpers || options.AllHelpers {
		return nil, errors.New("友善强盗与助手的组合尚未核验")
	}
	s, err := NewCatanSeafarers(n, options, setup, nil)
	if err != nil {
		return nil, err
	}
	if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
		return nil, err
	}
	return s, nil
}
