package game

import "errors"

const CatanExpansionRules = "catan-2025-helpers-2022"

type CatanOptions struct {
	FiveSix    bool   `json:"fiveSix,omitempty"`
	Rules      string `json:"rules,omitempty"`
	Helpers    bool   `json:"helpers,omitempty"`
	AllHelpers bool   `json:"allHelpers,omitempty"`
}

func NormalizeCatanOptions(o CatanOptions) (CatanOptions, error) {
	if o.Rules != "" && o.Rules != CatanExpansionRules {
		return o, errors.New("不支持的卡坦岛规则版本")
	}
	if o.AllHelpers && !o.Helpers {
		return o, errors.New("使用全部助手需要启用 Helpers 扩展")
	}
	if o.Helpers || o.FiveSix {
		o.Rules = CatanExpansionRules
	} else {
		o.Rules = ""
	}
	return o, nil
}

func NewCatan(n int, options CatanOptions) (*State, error) {
	o, err := NormalizeCatanOptions(options)
	if err != nil {
		return nil, err
	}
	if o.FiveSix && (n < 5 || n > 6) {
		return nil, errors.New("五至六人扩充需要 5–6 位玩家")
	}
	if !o.FiveSix && (n < 3 || n > 4) {
		return nil, errors.New("基础卡坦岛需要 3–4 位玩家；5–6 人请启用扩充")
	}
	s := &State{Kind: "catan", Phase: "turn", Round: 1, Log: []string{}}
	s.initCatan(n)
	s.Catan.Options = o
	s.initCatanHelpers()
	return s, nil
}

func (s *State) initCatanHelpers() {
	if s.Catan.Options.Helpers {
		pool := []int{}
		for id := len(s.Catan.Players) + 1; id <= 12; id++ {
			pool = append(pool, id)
		}
		shuffle(pool)
		if !s.Catan.Options.AllHelpers {
			pool = pool[:len(s.Catan.Players)]
		}
		s.Catan.HelperDisplay = pool
		s.Log = append(s.Log, "加入 Helpers：每人完成第二组起始建设后领取助手，使用后翻面或交换")
	}
}
