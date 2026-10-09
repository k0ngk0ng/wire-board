package game

import "errors"

// River layouts are independent of ordinary Seafarers layouts. Keeping a
// separate configuration prevents room changes from silently dropping rivers.
type CatanRiversSeafarersSetup struct {
	Scenario string `json:"scenario"`
	Layout   string `json:"layout"`
	Rules    string `json:"rules"`
}

func NormalizeCatanRiversSeafarersSetup(n int, setup CatanRiversSeafarersSetup) (CatanRiversSeafarersSetup, error) {
	if n < 2 || n > 6 {
		return setup, errors.New("河流航海家需要二至六人")
	}
	if setup.Rules != "" && setup.Rules != CatanRiversSeafarersRules {
		return setup, errors.New("河流航海家规则版本无效")
	}
	defaultLayout := "fixed"
	switch setup.Scenario {
	case "shores":
		if n > 4 {
			defaultLayout = catanRiversShoresExtendedLayout
		}
	case "fog":
		if n > 4 {
			defaultLayout = catanRiversFogExtendedLayout
		}
	case "desert":
		defaultLayout = "rivers-across"
	case "tribe":
		if n > 4 {
			return setup, errors.New("五六人河流部落尚未接通")
		}
		defaultLayout = "variable"
	case "new_world":
		defaultLayout = "default"
	default:
		return setup, errors.New("河流航海家地图无效")
	}
	if setup.Layout == "" {
		setup.Layout = defaultLayout
	}
	if setup.Layout != defaultLayout && !(setup.Scenario == "desert" && setup.Layout == "desert-belt") && !(setup.Scenario == "new_world" && setup.Layout == "prepared") {
		return setup, errors.New("河流航海家地图布局无效")
	}
	setup.Rules = CatanRiversSeafarersRules
	return setup, nil
}

// NewCatanRiversSeafarers is the common factory for room creation and restart.
// Extra modules must be enabled explicitly after construction and validated by
// their own rules; ordinary Seafarers options are never silently inherited.
func NewCatanRiversSeafarers(n int, setup CatanRiversSeafarersSetup, world *CatanRiversWorldMap) (*State, error) {
	setup, err := NormalizeCatanRiversSeafarersSetup(n, setup)
	if err != nil {
		return nil, err
	}
	if (world != nil) != (setup.Scenario == "new_world" && setup.Layout == "prepared") {
		return nil, errors.New("预备河流新世界必须提供地图，其他布局不能携带预备地图")
	}
	if setup.Scenario == "new_world" {
		if n == 2 {
			return newCatanTwoRiversWorld(world)
		}
		if world != nil {
			return newCatanRiversWorldWithMap(n, world)
		}
		return newCatanRiversWorld(n)
	}
	layout := ""
	if setup.Scenario == "desert" {
		layout = setup.Layout
	}
	if n == 2 {
		return newCatanTwoRiversSeafarers(setup.Scenario, layout)
	}
	switch setup.Scenario {
	case "desert":
		if n > 4 {
			return newCatanRiversDesertExtended(n, layout)
		}
	case "shores":
		return newCatanRiversShores(n)
	case "fog":
		if n > 4 {
			return newCatanRiversFogExtended(n)
		}
	case "tribe":
		return newCatanRiversTribe(n)
	}
	return newCatanRiversPrintedSeaLayout(n, setup.Scenario, layout)
}
