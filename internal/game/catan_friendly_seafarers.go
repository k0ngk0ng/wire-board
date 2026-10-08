package game

import "errors"

// New sea games disclose a versioned site fallback for maps without a legal
// desert. Scenario restrictions still take precedence over retreat to desert.
func CatanFriendlySeafarersSupported(n int, scenario string) bool {
	if n < 3 || n > 6 {
		return false
	}
	switch scenario {
	case "shores", "islands", "fog", "tribe", "new_world", "desert", "wonders", "cloth", "pirate_islands":
		return true
	case "six_islands":
		return n > 4
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
	var world *CatanNewWorldMap
	var err error
	if setup.Scenario == "new_world" {
		world, err = GenerateCatanNewWorldMap(n)
		if err != nil {
			return nil, err
		}
	}
	s, err := NewCatanSeafarers(n, options, setup, world)
	if err != nil {
		return nil, err
	}
	if err = s.ConfigureCatanFriendlyRobber(CatanFriendlyRobberSetup{Enabled: true}); err != nil {
		return nil, err
	}
	return s, nil
}
