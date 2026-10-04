package game

import (
	"fmt"
	"slices"
)

const CatanSeafarersRules = "catan-seafarers-2025"

// Kept separate from base options until expansion-wide acceptance is complete.
// A prepared New World map is supplied separately and must never be re-rolled.
type CatanSeafarersSetup struct {
	Scenario string `json:"scenario"`
	Layout   string `json:"layout"`
	Rules    string `json:"rules"`
}

type CatanSeafarersScenario struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Layouts       []string `json:"layouts"`
	VictoryPoints int      `json:"victoryPoints"`
}

// This is a rules catalogue, not a list of released/public room choices.
// Layout order puts each scenario's printed/default recipe first.
func CatanSeafarersScenarios(n int) []CatanSeafarersScenario {
	if n < 3 || n > 6 {
		return nil
	}
	result := []CatanSeafarersScenario{
		{"shores", "驶向新海岸", []string{"fixed", "variable"}, 14},
		{"islands", "四岛", []string{"fixed", "variable"}, 13},
		{"fog", "迷雾群岛", []string{"fixed", "variable"}, 12},
		{"desert", "穿越沙漠", []string{"fixed", "variable"}, 14},
		{"tribe", "遗忘的部落", []string{"fixed", "variable"}, 13},
		{"cloth", "卡坦布匹", []string{"fixed", "variable"}, 14},
		{"pirate_islands", "海盗群岛", []string{"fixed"}, 10},
		{"wonders", "卡坦奇迹", []string{"fixed", "variable"}, 10},
		{"new_world", "新世界", []string{"prepared"}, 12},
	}
	if n > 4 {
		for i := range result {
			if result[i].ID == "new_world" {
				continue
			}
			result[i].Layouts = []string{"fixed"}
		}
		// The extension prescribes a variable mainland and fixed outer islands.
		// It has no independent all-fixed or unrestricted variable recipe.
		result[0].Layouts = []string{"variable"}
		result[1].Name = "六岛"
	}
	return result
}

func NormalizeCatanSeafarersSetup(n int, setup CatanSeafarersSetup) (CatanSeafarersSetup, error) {
	if n < 3 || n > 6 {
		return setup, fmt.Errorf("航海家需要3至6位玩家")
	}
	if setup.Rules != "" && setup.Rules != CatanSeafarersRules {
		return setup, fmt.Errorf("不支持的航海家规则版本")
	}
	for _, info := range CatanSeafarersScenarios(n) {
		if info.ID != setup.Scenario {
			continue
		}
		if setup.Layout == "" {
			setup.Layout = info.Layouts[0]
		}
		if !slices.Contains(info.Layouts, setup.Layout) {
			return setup, fmt.Errorf("%s不支持%d人使用该地图布局", info.Name, n)
		}
		setup.Rules = CatanSeafarersRules
		return setup, nil
	}
	return setup, fmt.Errorf("未知的航海家剧本")
}

// Internal unified entry point. Public room creation does not expose setup yet.
// All maps start with their scenario-specific pregame phases and inventories.
func NewCatanSeafarers(n int, options CatanOptions, setup CatanSeafarersSetup, world *CatanNewWorldMap) (*State, error) {
	setup, err := NormalizeCatanSeafarersSetup(n, setup)
	if err != nil {
		return nil, err
	}
	if setup.Scenario != "new_world" && world != nil {
		return nil, fmt.Errorf("只有新世界可以使用开局前确认的地图")
	}
	var s *State
	switch setup.Scenario {
	case "new_world":
		s, err = NewCatanNewWorldWithMap(n, options, world)
	case "wonders":
		s, err = NewCatanWonders(n, options)
	case "pirate_islands":
		s, err = NewCatanPirateIslands(n, options)
	default:
		s, err = NewCatan(n, options)
		if err != nil {
			return nil, err
		}
		g := s.Catan
		var three, four, six func() error
		switch setup.Scenario {
		case "shores":
			three, four, six = g.makeSeafarersShoresThree, g.makeSeafarersShoresFour, g.makeSeafarersShoresSix
		case "islands":
			three, four, six = g.makeSeafarersIslandsThree, g.makeSeafarersIslandsFour, g.makeSeafarersIslandsSix
		case "fog":
			three, four, six = g.makeSeafarersFogThree, g.makeSeafarersFogFour, g.makeSeafarersFogSix
		case "desert":
			three, four, six = g.makeSeafarersDesertThree, g.makeSeafarersDesertFour, g.makeSeafarersDesertSix
		case "tribe":
			three, four, six = g.makeSeafarersTribeFour, g.makeSeafarersTribeFour, g.makeSeafarersTribeSix
		case "cloth":
			three, four, six = g.makeSeafarersClothFour, g.makeSeafarersClothFour, g.makeSeafarersClothSix
		}
		switch {
		case n == 3:
			err = three()
		case n == 4:
			err = four()
		default:
			err = six()
		}
	}
	if err != nil {
		return nil, err
	}
	if setup.Layout == "variable" && n <= 4 {
		// State-level randomization includes the initial robber-choice phase.
		if err = s.randomizeCatanSeafarersMap(); err != nil {
			return nil, err
		}
	}
	s.Catan.Seafarers.Rules, s.Catan.Seafarers.Layout = setup.Rules, setup.Layout
	// Replace base-only setup wording (two passes / thirty land hexes) with
	// the chosen scenario's actual start. Keep paired-turn and helper guidance.
	s.Log = []string{}
	for _, info := range CatanSeafarersScenarios(n) {
		if info.ID == setup.Scenario {
			s.Log = append(s.Log, fmt.Sprintf("航海家 · %s · %d人开局", info.Name, n))
		}
	}
	switch setup.Scenario {
	case "cloth":
		s.Log = append(s.Log, "按顺序、逆序、顺序放置三组村庄与路线，只从第三座村庄领取资源")
	case "new_world":
		s.Log = append(s.Log, "按确认地图轮流放置随机港口，再开始两轮起始建设；地形与数字保持不变")
	case "wonders":
		s.Log = append(s.Log, "先手选择沙漠作为强盗起点，再开始两轮起始建设")
	case "pirate_islands":
		s.Log = append(s.Log, "每人已有一座村庄和一艘船，再按顺序、逆序放置两组起始村庄与路线")
	default:
		s.Log = append(s.Log, "按顺序放置村庄和道路或船只，再逆序放置第二组")
	}
	if options.FiveSix {
		s.Log = append(s.Log, "采用配对回合；第二位玩家不掷骰，不能与其他玩家自由交易")
	}
	if options.Helpers {
		s.Log = append(s.Log, "加入Helpers：完成第二组起始建设后领取助手，使用后翻面或交换")
	}
	s.catanScores()
	return s, nil
}
